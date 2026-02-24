package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vailang/vai/internal/coder"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/runner/diagnostic"
	"github.com/vailang/vai/internal/runner/provider"
	"github.com/vailang/vai/internal/runner/tools"
)

// debug runs the compile-check loop until success or max attempts.
func (r *Runner) debug(ctx context.Context) error {
	// Collect unique languages from target files.
	langTargets := r.collectLangTargets()
	if len(langTargets) == 0 {
		return nil
	}

	maxAttempts := r.cfg.Debug.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	for lang, targets := range langTargets {
		langCfg, ok := r.cfg.Debug.Languages[lang]
		if !ok {
			continue // no debug config for this language
		}
		if langCfg.CompileCheck == "" {
			continue
		}

		// Use structured parser when format is "json".
		var parser diagnostic.Parser
		if langCfg.Format == "json" {
			parser = diagnostic.ForLanguage(lang)
		}

		if err := r.debugLang(ctx, lang, targets, langCfg, parser, maxAttempts); err != nil {
			return err
		}
	}
	return nil
}

// debugLang runs the debug loop for a single language.
func (r *Runner) debugLang(ctx context.Context, lang string, targets []string, langCfg config.DebugLangConfig, parser diagnostic.Parser, maxAttempts int) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Run compile check.
		cmd := r.expandCmd(langCfg.CompileCheck, targets)
		r.emit(Event{Kind: EventInfo, Step: "debug", Message: fmt.Sprintf("[%s] attempt %d/%d: %s", lang, attempt, maxAttempts, cmd)})

		output, err := r.runCmd(ctx, cmd)
		if err == nil {
			// Compilation succeeded — run extra tools.
			allOk := true
			for _, tool := range langCfg.Tools {
				toolCmd := r.expandCmd(tool, targets)
				if _, toolErr := r.runCmd(ctx, toolCmd); toolErr != nil {
					allOk = false
					break
				}
			}
			if allOk {
				r.emit(Event{Kind: EventInfo, Step: "debug", Message: fmt.Sprintf("[%s]: passed", lang)})
				return nil
			}
		}

		// Compilation failed — try to fix.
		r.emit(Event{Kind: EventInfo, Step: "debug", Message: fmt.Sprintf("[%s]: compilation failed, attempting fix...", lang)})
		fixed, fixErr := r.fixCompileErrors(ctx, targets, output, parser)
		if fixErr != nil {
			return fmt.Errorf("fix attempt %d: %w", attempt, fixErr)
		}
		if !fixed {
			if attempt == maxAttempts {
				return fmt.Errorf("compilation failed after %d attempts for %s:\n%s", maxAttempts, lang, output)
			}
			continue
		}

		// Flush fixes to disk for next compile check.
		if err := r.fm.Flush(); err != nil {
			return fmt.Errorf("flushing fixes: %w", err)
		}
	}
	return fmt.Errorf("compilation failed after %d attempts for %s", maxAttempts, lang)
}

// fixCompileErrors attempts to fix compilation errors using the executor LLM.
// When a diagnostic parser is available, errors are mapped to specific symbols
// and only the failing function code is sent to the LLM. Otherwise falls back
// to sending the full file.
func (r *Runner) fixCompileErrors(ctx context.Context, targets []string, errorOutput string, parser diagnostic.Parser) (bool, error) {
	// Try structured diagnostics first.
	if parser != nil {
		diags, err := parser.Parse([]byte(errorOutput))
		if err == nil && len(diags) > 0 {
			fixed, err := r.fixTargeted(ctx, targets, diags)
			if err != nil {
				return false, err
			}
			if fixed {
				return true, nil
			}
			// Fall through to legacy if targeted fix didn't help.
		}
	}
	return r.fixLegacy(ctx, targets, errorOutput)
}

// fixTargeted maps diagnostics to specific symbols and sends only the
// failing function code to the LLM for each one.
func (r *Runner) fixTargeted(ctx context.Context, targets []string, diags []diagnostic.Diagnostic) (bool, error) {
	system, err := r.prog.Eval("inject std.developer")
	if err != nil {
		return false, fmt.Errorf("eval developer prompt: %w", err)
	}

	anyFixed := false
	for _, target := range targets {
		absPath := r.resolvePath(target)
		content, ok := r.fm.Read(absPath)
		if !ok {
			continue
		}

		c, err := r.pool.Get(absPath, content)
		if err != nil {
			continue
		}

		symDiags := mapDiagnosticsToSymbols(diags, c, content, target)
		for _, sd := range symDiags {
			if sd.name == "_unmatched" || sd.code == "" {
				continue
			}

			// Build targeted prompt with only this function and its errors.
			var prompt strings.Builder
			prompt.WriteString("## Fix Compilation Errors in `" + sd.name + "`\n\n")
			prompt.WriteString("```\n" + sd.code + "\n```\n\n")
			prompt.WriteString("## Errors\n\n")
			for _, d := range sd.diagnostics {
				relLine := d.Line - sd.startLine + 1
				prompt.WriteString(fmt.Sprintf("- Line %d: %s\n", relLine, d.Message))
			}
			prompt.WriteString("\nFix only this function. Return the corrected function definition.\n")

			resp, err := r.executor.Call(ctx, provider.Request{
				System:    system,
				Messages:  []provider.Message{{Role: "user", Content: prompt.String()}},
				Tools:     []provider.ToolDefinition{tools.WriteCodeTool()},
				Model:     r.cfg.Executor.Model,
				MaxTokens: r.cfg.Executor.MaxTokens,
			})
			if err != nil {
				continue
			}
			r.stats.DebugTokensIn += resp.TokensIn
			r.stats.DebugTokensOut += resp.TokensOut
			r.stats.DebugCalls++

			for _, tc := range resp.ToolCalls {
				if tc.Name == "write_code" {
					result, err := tools.ParseWriteCode(tc.Input)
					if err != nil || result.NoChange {
						continue
					}
					if err := r.applyWriteCode(absPath, sd.name, result); err == nil {
						anyFixed = true
						r.emit(Event{Kind: EventInfo, Step: "debug", Name: sd.name, Message: fmt.Sprintf("fixed %s in %s", sd.name, target)})
					}
				}
			}
		}
	}
	return anyFixed, nil
}

// fixLegacy sends the full file and all errors to the LLM. Used as fallback
// when structured diagnostics are not available.
func (r *Runner) fixLegacy(ctx context.Context, targets []string, errorOutput string) (bool, error) {
	system, err := r.prog.Eval("inject std.developer")
	if err != nil {
		return false, fmt.Errorf("eval developer prompt: %w", err)
	}

	anyFixed := false
	for _, target := range targets {
		absPath := r.resolvePath(target)
		content, ok := r.fm.Read(absPath)
		if !ok {
			continue
		}

		userPrompt := fmt.Sprintf(
			"## Fix Compilation Error\n\nFile: %s\n\n```\n%s\n```\n\n## Error Output\n\n```\n%s\n```\n\nFix the compilation errors in this file. Provide the corrected full file content.",
			target, string(content), errorOutput,
		)

		resp, err := r.executor.Call(ctx, provider.Request{
			System:    system,
			Messages:  []provider.Message{{Role: "user", Content: userPrompt}},
			Tools:     []provider.ToolDefinition{tools.ReportFixTool()},
			Model:     r.cfg.Executor.Model,
			MaxTokens: r.cfg.Executor.MaxTokens,
		})
		if err != nil {
			continue
		}
		r.stats.DebugTokensIn += resp.TokensIn
		r.stats.DebugTokensOut += resp.TokensOut
		r.stats.DebugCalls++

		for _, tc := range resp.ToolCalls {
			if tc.Name == "report_fix" {
				fix, err := tools.ParseReportFix(tc.Input)
				if err != nil {
					continue
				}
				if !fix.Fixed {
					r.emit(Event{Kind: EventWarning, Step: "debug", Message: fmt.Sprintf("unable to fix %s: %s", target, fix.Reason)})
					continue
				}
				if fix.Code != "" {
					r.fm.Write(absPath, []byte(fix.Code))
					anyFixed = true
				}
			}
		}

		// If no tool call but content returned, use it as the fix.
		if !anyFixed && resp.Content != "" {
			r.fm.Write(absPath, []byte(resp.Content))
			anyFixed = true
		}
	}
	return anyFixed, nil
}

// symbolDiag groups diagnostics for a single symbol.
type symbolDiag struct {
	name        string
	code        string // full function source code
	startLine   int
	diagnostics []diagnostic.Diagnostic
}

// mapDiagnosticsToSymbols assigns each diagnostic to the symbol whose
// byte range contains the error line. Unmatched diagnostics go into a
// special "_unmatched" group.
func mapDiagnosticsToSymbols(
	diags []diagnostic.Diagnostic,
	c *coder.Coder,
	content []byte,
	targetFile string,
) []symbolDiag {
	lineOffsets := buildLineOffsets(content)

	// Collect all symbols with their byte ranges.
	type symRange struct {
		name      string
		startByte int
		endByte   int
		startLine int
	}
	var ranges []symRange
	for name := range c.Symbols() {
		if resolved, ok := c.Resolve(name); ok {
			line := byteToLine(lineOffsets, resolved.StartByte)
			ranges = append(ranges, symRange{
				name: name, startByte: resolved.StartByte,
				endByte: resolved.EndByte, startLine: line,
			})
		}
	}
	// Sort by start byte for consistent ordering.
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].startByte < ranges[j].startByte })

	groups := map[string]*symbolDiag{}
	for _, d := range diags {
		if d.File != targetFile {
			continue
		}
		byteOff := lineToByteOffset(lineOffsets, d.Line)
		matched := false
		for _, sr := range ranges {
			if byteOff >= sr.startByte && byteOff < sr.endByte {
				if _, ok := groups[sr.name]; !ok {
					resolved, _ := c.Resolve(sr.name)
					groups[sr.name] = &symbolDiag{
						name: sr.name, code: resolved.Code,
						startLine: sr.startLine,
					}
				}
				groups[sr.name].diagnostics = append(groups[sr.name].diagnostics, d)
				matched = true
				break
			}
		}
		if !matched {
			if _, ok := groups["_unmatched"]; !ok {
				groups["_unmatched"] = &symbolDiag{name: "_unmatched"}
			}
			groups["_unmatched"].diagnostics = append(groups["_unmatched"].diagnostics, d)
		}
	}

	var result []symbolDiag
	for _, sd := range groups {
		result = append(result, *sd)
	}
	return result
}

// buildLineOffsets returns byte offsets for each line (1-indexed).
// offsets[0] = byte offset of line 1, offsets[1] = byte offset of line 2, etc.
func buildLineOffsets(content []byte) []int {
	offsets := []int{0} // line 1 starts at byte 0
	for i, b := range content {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

func lineToByteOffset(offsets []int, line int) int {
	if line <= 0 || line > len(offsets) {
		return 0
	}
	return offsets[line-1]
}

func byteToLine(offsets []int, byteOff int) int {
	for i := len(offsets) - 1; i >= 0; i-- {
		if byteOff >= offsets[i] {
			return i + 1
		}
	}
	return 1
}

// collectLangTargets groups target file paths by language name.
func (r *Runner) collectLangTargets() map[string][]string {
	result := map[string][]string{}
	seen := map[string]bool{}

	for _, req := range r.prog.Requests() {
		for _, tp := range req.TargetPaths {
			if seen[tp] {
				continue
			}
			seen[tp] = true
			ext := filepath.Ext(tp)
			lang := langFromExt(ext)
			if lang != "" {
				result[lang] = append(result[lang], tp)
			}
		}
	}
	return result
}

// langFromExt maps file extension to language name for debug config lookup.
func langFromExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".c", ".h":
		return "c"
	default:
		return ""
	}
}

// expandCmd replaces {target} placeholder with the actual file paths.
func (r *Runner) expandCmd(cmd string, targets []string) string {
	if len(targets) > 0 {
		absTargets := make([]string, len(targets))
		for i, t := range targets {
			absTargets[i] = r.resolvePath(t)
		}
		return strings.ReplaceAll(cmd, "{target}", strings.Join(absTargets, " "))
	}
	return cmd
}

// runCmd executes a shell command in the base directory.
func (r *Runner) runCmd(ctx context.Context, cmdStr string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = r.baseDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.String() + stderr.String()
	return output, err
}
