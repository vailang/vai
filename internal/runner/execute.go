package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/vailang/vai/internal/coder"
	"github.com/vailang/vai/internal/coder/api"
	"github.com/vailang/vai/internal/locker"
	"github.com/vailang/vai/internal/runner/provider"
	"github.com/vailang/vai/internal/runner/tools"
)

// execute runs all impl tasks in parallel using the executor LLM.
func (r *Runner) execute(ctx context.Context, skeletons []planSkeletonResult) error {
	// Collect all impls that need execution.
	type implTask struct {
		planName   string
		targetPath string // absolute path
		impl       tools.SkeletonImpl
	}

	var tasks []implTask
	for _, skel := range skeletons {
		if len(skel.targetPaths) == 0 {
			continue
		}
		defaultTarget := r.resolvePath(skel.targetPaths[0])
		for _, impl := range skel.skeleton.Impls {
			if impl.Action == "add" || impl.Action == "update" {
				target := defaultTarget
				if impl.Target != "" {
					target = r.resolvePath(impl.Target)
				}
				tasks = append(tasks, implTask{
					planName:   skel.planName,
					targetPath: target,
					impl:       impl,
				})
			}
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	// Get system prompt once (shared by all executors).
	system, err := r.prog.Eval("inject std.developer")
	if err != nil {
		return fmt.Errorf("eval developer prompt: %w", err)
	}

	type implResult struct {
		name      string
		tokensIn  int
		tokensOut int
		err       error
		cached    bool
		textLen   int // length of instruction text (for token estimate)
	}

	var wg sync.WaitGroup
	resultCh := make(chan implResult, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t implTask) {
			defer wg.Done()
			qualName := t.planName + "." + t.impl.Name

			// Check lock before calling the executor.
			if r.locker != nil {
				implHash := locker.HashImpl(t.impl.Name, t.impl.Instruction, t.targetPath)
				if r.locker.IsLocked("impl:"+qualName, implHash) {
					r.emit(Event{Kind: EventInfo, Step: "executor", Name: qualName, Message: fmt.Sprintf("impl %q is locked, skipping", qualName)})
					resultCh <- implResult{name: qualName, cached: true, textLen: len(t.impl.Instruction)}
					return
				}
			}

			tokIn, tokOut, err := r.executeImpl(ctx, system, t.planName, t.targetPath, t.impl)

			// Record successful impl hash in the lock.
			if err == nil && r.locker != nil {
				implHash := locker.HashImpl(t.impl.Name, t.impl.Instruction, t.targetPath)
				r.locker.Lock("impl:"+qualName, implHash)
			}

			resultCh <- implResult{
				name:      qualName,
				tokensIn:  tokIn,
				tokensOut: tokOut,
				err:       err,
			}
		}(task)
	}

	wg.Wait()
	close(resultCh)

	var savedTextLen int
	var errs []error
	for res := range resultCh {
		if res.cached {
			r.stats.CachedImpls++
			savedTextLen += res.textLen
			r.stats.ImplStats = append(r.stats.ImplStats, ImplStat{
				Name:   res.name,
				Status: StatusSkipped,
			})
			continue
		}
		status := StatusComplete
		if res.err != nil {
			status = StatusFailed
			errs = append(errs, fmt.Errorf("impl %s: %w", res.name, res.err))
		}
		r.stats.ImplStats = append(r.stats.ImplStats, ImplStat{
			Name:      res.name,
			TokensIn:  res.tokensIn,
			TokensOut: res.tokensOut,
			Status:    status,
		})
	}
	if savedTextLen > 0 {
		r.stats.SavedTokensEstimate = EstimateTokensSaved(savedTextLen)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d impl(s) failed: %v", len(errs), errs[0])
	}
	return nil
}

// executeImpl handles a single impl execution. Returns token counts and error.
func (r *Runner) executeImpl(ctx context.Context, system, planName, absTargetPath string, impl tools.SkeletonImpl) (tokensIn, tokensOut int, err error) {
	qualName := planName + "." + impl.Name

	// Build the user prompt with skeleton context.
	user := r.buildExecutorPrompt(absTargetPath, impl)

	r.emit(Event{Kind: EventImplStart, Step: "executor", Name: qualName})
	resp, err := r.executor.Call(ctx, provider.Request{
		System:    system,
		Messages:  []provider.Message{{Role: "user", Content: user}},
		Tools:     []provider.ToolDefinition{tools.WriteCodeTool()},
		Model:     r.cfg.Executor.Model,
		MaxTokens: r.cfg.Executor.MaxTokens,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("executor call: %w", err)
	}
	tokensIn = resp.TokensIn
	tokensOut = resp.TokensOut

	for _, tc := range resp.ToolCalls {
		if tc.Name == "write_code" {
			result, err := tools.ParseWriteCode(tc.Input)
			if err != nil {
				return tokensIn, tokensOut, fmt.Errorf("parsing write_code: %w", err)
			}
			if result.NoChange {
				return tokensIn, tokensOut, nil
			}
			return tokensIn, tokensOut, r.applyWriteCode(absTargetPath, impl.Name, result)
		}
	}

	// If no tool call, check if content contains code (some providers return text).
	if resp.Content != "" {
		return tokensIn, tokensOut, r.applyWriteCode(absTargetPath, impl.Name, &tools.WriteCodeInput{
			Code: resp.Content,
		})
	}

	return tokensIn, tokensOut, fmt.Errorf("no write_code tool call in response for %s", impl.Name)
}

// buildExecutorPrompt constructs the user prompt for an impl execution.
// Includes the current skeleton stub code and the instruction.
func (r *Runner) buildExecutorPrompt(absTargetPath string, impl tools.SkeletonImpl) string {
	var b strings.Builder

	b.WriteString("## Function: ")
	b.WriteString(impl.Name)
	b.WriteString("\n\n")

	// Load coder once for all symbol resolutions.
	content, ok := r.fm.Read(absTargetPath)
	var c *coder.Coder
	if ok && len(content) > 0 {
		c, _ = r.pool.Get(absTargetPath, content)
	}

	// Include the current skeleton stub.
	if c != nil {
		if resolved, found := c.Resolve(impl.Name); found {
			b.WriteString("Current skeleton code:\n```\n")
			b.WriteString(resolved.Code)
			b.WriteString("\n```\n\n")
		}
	}

	if impl.Instruction != "" {
		b.WriteString("### Instructions\n\n")
		b.WriteString(impl.Instruction)
		b.WriteString("\n\n")
	}

	if len(impl.Uses) > 0 {
		b.WriteString("### Dependencies\n\n")
		b.WriteString("These symbols are available:\n")
		for _, u := range impl.Uses {
			b.WriteString("- `")
			b.WriteString(u)
			b.WriteString("`\n")
		}
		b.WriteString("\n")

		// Resolve dependency signatures.
		if c != nil {
			for _, u := range impl.Uses {
				if resolved, found := c.Resolve(u); found && resolved.Signature != "" {
					b.WriteString("```\n")
					b.WriteString(resolved.Signature)
					b.WriteString("\n```\n")
				}
			}
		}
	}

	b.WriteString("Write the complete function with the real implementation. Include the full signature and body.\n")

	return b.String()
}


// applyWriteCode modifies the in-memory file with the executor's output.
func (r *Runner) applyWriteCode(absPath, symbolName string, wc *tools.WriteCodeInput) error {
	return r.fm.ModifyFile(absPath, func(content []byte) ([]byte, error) {
		c, err := r.pool.Get(absPath, content)
		if err != nil {
			return nil, fmt.Errorf("loading coder for %s: %w", absPath, err)
		}

		newCode := wc.Code
		resolved, ok := c.Resolve(symbolName)
		var result []byte

		if ok {
			// Replace the existing symbol with the new implementation.
			replacement := api.BodyReplacement{
				StartByte: resolved.StartByte,
				EndByte:   resolved.EndByte,
				Stub:      newCode,
			}
			result = []byte(api.ApplyReplacements(content, []api.BodyReplacement{replacement}))
		} else {
			// Symbol not found — append to end of file.
			r.emit(Event{Kind: EventWarning, Step: "executor", Name: symbolName, Message: fmt.Sprintf("symbol %q not found, appending", symbolName)})
			var sb strings.Builder
			sb.Write(content)
			if len(content) > 0 && content[len(content)-1] != '\n' {
				sb.WriteByte('\n')
			}
			sb.WriteByte('\n')
			sb.WriteString(newCode)
			if !strings.HasSuffix(newCode, "\n") {
				sb.WriteByte('\n')
			}
			result = []byte(sb.String())
		}

		return result, nil
	})
}

// insertImportsInMemory adds imports to file content without writing to disk.
func (r *Runner) insertImportsInMemory(content []byte, absPath string, imports []string) ([]byte, error) {
	c, err := r.pool.Get(absPath, content)
	if err != nil {
		return nil, err
	}

	zone, err := c.FindImportZone(content)
	if err != nil {
		return nil, err
	}

	lang, err := coder.DetectLanguage(absPath)
	if err != nil {
		return nil, err
	}
	comment := string(coder.CommentPrefix(lang)) + " from vailang compiler"
	block := c.BuildImportBlock(imports, comment)

	var result []byte
	if zone != nil {
		result = append(result, content[:zone.EndByte]...)
		result = append(result, '\n')
		result = append(result, []byte(block)...)
		result = append(result, content[zone.EndByte:]...)
	} else {
		result = append(result, []byte(block)...)
		result = append(result, '\n')
		result = append(result, content...)
	}
	return result, nil
}
