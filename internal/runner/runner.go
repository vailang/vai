package runner

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/vailang/vai/internal/compiler"
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/composer"
	"github.com/vailang/vai/internal/compiler/render"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/locker"
	"github.com/vailang/vai/internal/runner/filemanager"
	"github.com/vailang/vai/internal/runner/provider"
	"github.com/vailang/vai/internal/runner/tools"
)

// RequestLocker checks whether a request should be skipped (locked)
// and records hashes after successful execution.
type RequestLocker interface {
	IsLocked(key, hash string) bool
	Lock(key, hash string)
	Save() error
}

// StepStatus represents the completion status of a pipeline step.
type StepStatus string

const (
	StatusPending  StepStatus = "pending"
	StatusComplete StepStatus = "complete"
	StatusFailed   StepStatus = "failed"
	StatusSkipped  StepStatus = "skipped"
)

// ImplStat holds token usage for a single impl execution.
type ImplStat struct {
	Name      string     `json:"name"`
	TokensIn  int        `json:"tokens_in"`
	TokensOut int        `json:"tokens_out"`
	Status    StepStatus `json:"status"`
}

// RunStats tracks token usage and status across all pipeline steps.
type RunStats struct {
	ArchitectTokensIn  int        `json:"architect_tokens_in"`
	ArchitectTokensOut int        `json:"architect_tokens_out"`
	ArchitectCycles    int        `json:"architect_cycles"`
	ArchitectStatus    StepStatus `json:"architect_status"`
	ImplStats          []ImplStat `json:"impl_stats,omitempty"`
	DebugTokensIn      int        `json:"debug_tokens_in"`
	DebugTokensOut     int        `json:"debug_tokens_out"`
	DebugCalls         int        `json:"debug_calls"`
	DebugStatus        StepStatus `json:"debug_status"`
	CachedPlans        int        `json:"cached_plans"`
	CachedImpls        int        `json:"cached_impls"`
	SavedTokensEstimate string   `json:"saved_tokens_estimate,omitempty"`
}

// EstimateTokensSaved returns an approximate token count saved by cache hits.
// Uses the heuristic of ~4 characters per token, prefixed with ~ to indicate approximation.
func EstimateTokensSaved(textLen int) string {
	if textLen == 0 {
		return ""
	}
	return fmt.Sprintf("~%d", textLen/4)
}

// Runner orchestrates the 5-step pipeline:
// Plan (compiler) → Architect (LLM) → Diff (tree-sitter) → Executor (LLM) → Debug (loop).
type Runner struct {
	prog       compiler.Program
	cfg        *config.Config
	planner    provider.Provider
	executor   provider.Provider
	fm         *filemanager.FileManager
	pool       *coderPool
	baseDir    string // working directory for command execution
	planFilter string // if set, only process plans matching this name
	locker     RequestLocker
	stats      RunStats
	events     chan Event
}

// SetPlanFilter restricts the runner to only process the named plan.
func (r *Runner) SetPlanFilter(name string) {
	r.planFilter = name
}

// New creates a Runner from a compiled Program and configuration.
// If locker is nil, no lock checking is performed.
func New(prog compiler.Program, cfg *config.Config, baseDir string, locker RequestLocker) (*Runner, error) {
	planner, err := provider.New(cfg.Planner)
	if err != nil {
		return nil, fmt.Errorf("creating planner provider: %w", err)
	}
	executor, err := provider.New(cfg.Executor)
	if err != nil {
		return nil, fmt.Errorf("creating executor provider: %w", err)
	}
	return &Runner{
		prog:     prog,
		cfg:      cfg,
		planner:  planner,
		executor: executor,
		fm:       filemanager.New(),
		pool:     newCoderPool(),
		baseDir:  baseDir,
		locker:   locker,
	}, nil
}

// emit sends an event to the event channel if one is set.
func (r *Runner) emit(e Event) {
	if r.events != nil {
		r.events <- e
	}
}

// startAsync runs fn in a goroutine, returning event and error channels.
func (r *Runner) startAsync(fn func() error) (<-chan Event, <-chan error) {
	events := make(chan Event, 64)
	errc := make(chan error, 1)
	r.events = events
	go func() {
		defer close(events)
		defer r.pool.Close()
		errc <- fn()
	}()
	return events, errc
}

// Run executes the full pipeline (skeleton + plan + code).
// Returns an event channel and an error channel.
// The event channel is closed when the pipeline completes.
// The error channel receives exactly one value (nil on success).
func (r *Runner) Run(ctx context.Context) (<-chan Event, <-chan error) {
	return r.startAsync(func() error {
		if err := r.loadTargets(); err != nil {
			return err
		}
		skeletons, err := r.runArchitectStep(ctx)
		if err != nil {
			return err
		}
		r.mergeImpls(skeletons)
		if err := r.runDiffAndSave(ctx, skeletons); err != nil {
			return err
		}
		if err := r.runCodeStep(ctx, skeletons); err != nil {
			return err
		}
		if err := r.saveLock(); err != nil {
			return err
		}
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		r.emit(Event{Kind: EventDone})
		return nil
	})
}

// RunSkeleton runs architect + diff + flush (writes stubs to target files, does NOT save impls to .vai file).
func (r *Runner) RunSkeleton(ctx context.Context) (<-chan Event, <-chan error) {
	return r.startAsync(func() error {
		if err := r.loadTargets(); err != nil {
			return err
		}
		skeletons, err := r.runArchitectStep(ctx)
		if err != nil {
			return err
		}
		r.mergeImpls(skeletons)
		// Apply skeleton declarations to in-memory target files.
		r.emit(Event{Kind: EventStepStart, Step: "diff", Message: "applying skeleton to target files..."})
		if err := r.diff(skeletons); err != nil {
			r.emit(Event{Kind: EventStepFailed, Step: "diff", Message: err.Error()})
			return fmt.Errorf("diff step: %w", err)
		}
		r.emit(Event{Kind: EventStepComplete, Step: "diff"})
		// Emit skeleton summary.
		for _, skel := range skeletons {
			r.emit(Event{Kind: EventSkeleton, Name: skel.planName, Data: SkeletonData{
				ImportCount: len(skel.skeleton.Imports),
				DeclCount:   len(skel.skeleton.Declarations),
				ImplCount:   len(skel.skeleton.Impls),
			}})
		}
		// Flush target files to disk.
		if err := r.fm.Flush(); err != nil {
			return fmt.Errorf("flush: %w", err)
		}
		if err := r.saveLock(); err != nil {
			return err
		}
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		r.emit(Event{Kind: EventDone})
		return nil
	})
}

// RunPlan runs architect + diff + save + flush (everything before executor).
func (r *Runner) RunPlan(ctx context.Context) (<-chan Event, <-chan error) {
	return r.startAsync(func() error {
		if err := r.loadTargets(); err != nil {
			return err
		}
		skeletons, err := r.runArchitectStep(ctx)
		if err != nil {
			return err
		}
		r.mergeImpls(skeletons)
		if err := r.runDiffAndSave(ctx, skeletons); err != nil {
			return err
		}
		// Flush target files to disk.
		if err := r.fm.Flush(); err != nil {
			return fmt.Errorf("flush: %w", err)
		}
		if err := r.saveLock(); err != nil {
			return err
		}
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		r.emit(Event{Kind: EventDone})
		return nil
	})
}

// RunCode runs executor + debug (requires skeleton already saved in .vai file).
func (r *Runner) RunCode(ctx context.Context) (<-chan Event, <-chan error) {
	return r.startAsync(func() error {
		if err := r.loadTargets(); err != nil {
			return err
		}
		// Build skeleton results from the already-compiled program's impls.
		skeletons := r.buildSkeletonsFromProgram()
		if err := r.runCodeStep(ctx, skeletons); err != nil {
			return err
		}
		if err := r.saveLock(); err != nil {
			return err
		}
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		r.emit(Event{Kind: EventDone})
		return nil
	})
}

// loadTargets loads all target files from planner requests into memory.
func (r *Runner) loadTargets() error {
	targetPaths := r.collectTargetPaths()
	for _, path := range targetPaths {
		absPath := r.resolvePath(path)
		if err := r.fm.Load(absPath); err != nil {
			return fmt.Errorf("loading target %s: %w", path, err)
		}
	}
	return nil
}

// runArchitectStep calls the planner LLM for each plan and returns skeleton results.
func (r *Runner) runArchitectStep(ctx context.Context) ([]planSkeletonResult, error) {
	r.emit(Event{Kind: EventStepStart, Step: "architect", Message: "planning skeleton..."})
	skeletons, err := r.architect(ctx)
	if err != nil {
		r.stats.ArchitectStatus = StatusFailed
		r.emit(Event{Kind: EventStepFailed, Step: "architect", Message: err.Error()})
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		return nil, fmt.Errorf("architect step: %w", err)
	}

	// Fill in skeletons for plans that were locked (skipped by architect).
	skeletons = r.fillLockedSkeletons(skeletons)

	r.stats.ArchitectStatus = StatusComplete
	r.emit(Event{Kind: EventStepComplete, Step: "architect"})

	// Load any additional target files from per-decl targets.
	for _, skel := range skeletons {
		for _, decl := range skel.skeleton.Declarations {
			if decl.Target != "" {
				absPath := r.resolvePath(decl.Target)
				if err := r.fm.Load(absPath); err != nil {
					return nil, fmt.Errorf("loading decl target %s: %w", decl.Target, err)
				}
			}
		}
	}
	return skeletons, nil
}

// runDiffAndSave applies the skeleton to target files and saves back to the plan file.
func (r *Runner) runDiffAndSave(_ context.Context, skeletons []planSkeletonResult) error {
	// Diff — apply skeleton declarations to in-memory files.
	r.emit(Event{Kind: EventStepStart, Step: "diff", Message: "applying skeleton to target files..."})
	if err := r.diff(skeletons); err != nil {
		r.emit(Event{Kind: EventStepFailed, Step: "diff", Message: err.Error()})
		return fmt.Errorf("diff step: %w", err)
	}
	r.emit(Event{Kind: EventStepComplete, Step: "diff"})

	// Emit skeleton summary.
	for _, skel := range skeletons {
		r.emit(Event{Kind: EventSkeleton, Name: skel.planName, Data: SkeletonData{
			ImportCount: len(skel.skeleton.Imports),
			DeclCount:   len(skel.skeleton.Declarations),
			ImplCount:   len(skel.skeleton.Impls),
		}})
	}

	// Save skeleton back to the original plan file.
	r.emit(Event{Kind: EventStepStart, Step: "save", Message: "persisting skeleton to plan file..."})
	if err := r.saveSkeleton(skeletons); err != nil {
		r.emit(Event{Kind: EventStepFailed, Step: "save", Message: err.Error()})
		return fmt.Errorf("save skeleton: %w", err)
	}
	r.emit(Event{Kind: EventStepComplete, Step: "save"})
	return nil
}

// runCodeStep runs the executor and debug steps.
func (r *Runner) runCodeStep(ctx context.Context, skeletons []planSkeletonResult) error {
	// Executor — fill function bodies in parallel.
	r.emit(Event{Kind: EventStepStart, Step: "executor", Message: "generating implementations..."})
	if err := r.execute(ctx, skeletons); err != nil {
		_ = r.fm.Flush()
		r.emit(Event{Kind: EventStepFailed, Step: "executor", Message: err.Error()})
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		return fmt.Errorf("executor step: %w", err)
	}
	r.emit(Event{Kind: EventStepComplete, Step: "executor"})

	// Flush to disk before debug step.
	if err := r.fm.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	// Debug — compile check loop.
	r.emit(Event{Kind: EventStepStart, Step: "debug", Message: "checking compilation..."})
	if err := r.debug(ctx); err != nil {
		r.stats.DebugStatus = StatusFailed
		r.emit(Event{Kind: EventStepFailed, Step: "debug", Message: err.Error()})
		r.emit(Event{Kind: EventSummary, Data: r.stats})
		return fmt.Errorf("debug step: %w", err)
	}
	r.stats.DebugStatus = StatusComplete
	r.emit(Event{Kind: EventStepComplete, Step: "debug"})
	return nil
}

// buildSkeletonsFromProgram constructs skeleton results from the compiled program's
// existing impl declarations. Used by RunCode when the skeleton is already saved.
func (r *Runner) buildSkeletonsFromProgram() []planSkeletonResult {
	// Index plans from the AST.
	planIndex := map[string]*ast.PlanDecl{}
	if f := r.prog.File(); f != nil {
		for _, decl := range f.Declarations {
			if pd, ok := decl.(*ast.PlanDecl); ok {
				planIndex[pd.Name] = pd
			}
		}
	}

	var results []planSkeletonResult
	for _, req := range r.prog.Requests() {
		if req.Type != composer.PlannerAgent {
			continue
		}
		if r.planFilter != "" && req.Name != r.planFilter {
			continue
		}
		plan, ok := planIndex[req.Name]
		if !ok {
			continue
		}
		var impls []tools.SkeletonImpl
		for _, impl := range plan.Impls {
			impls = append(impls, astImplToSkeleton(impl))
		}
		results = append(results, planSkeletonResult{
			planName:    req.Name,
			targetPaths: req.TargetPaths,
			sourcePath:  req.SourcePath,
			skeleton:    tools.PlanSkeletonInput{Impls: impls},
		})
	}
	return results
}

// collectTargetPaths gathers unique target paths from all plan requests.
func (r *Runner) collectTargetPaths() []string {
	seen := map[string]bool{}
	var paths []string
	for _, req := range r.prog.Requests() {
		if req.Type == composer.PlannerAgent {
			if r.planFilter != "" && req.Name != r.planFilter {
				continue
			}
			for _, tp := range req.TargetPaths {
				if !seen[tp] {
					seen[tp] = true
					paths = append(paths, tp)
				}
			}
		}
	}
	return paths
}

// resolvePath resolves a relative path against the base directory.
func (r *Runner) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.baseDir, path)
}

// planSkeletonResult holds a skeleton response with its associated plan metadata.
type planSkeletonResult struct {
	planName    string
	targetPaths []string
	sourcePath  string // absolute path of the .vai/.plan source file
	skeleton    tools.PlanSkeletonInput
}

// architect sends each plan to the planner LLM and collects skeleton responses.
func (r *Runner) architect(ctx context.Context) ([]planSkeletonResult, error) {
	var results []planSkeletonResult

	for _, req := range r.prog.Requests() {
		if req.Type != composer.PlannerAgent {
			continue
		}
		if r.planFilter != "" && req.Name != r.planFilter {
			continue
		}

		// Check lock before calling the planner.
		if r.locker != nil {
			planHash := r.computePlanHash(req.Name)
			if planHash != "" && r.locker.IsLocked("plan:"+req.Name, planHash) {
				r.emit(Event{Kind: EventInfo, Step: "architect", Name: req.Name, Message: fmt.Sprintf("plan %q is locked, skipping", req.Name)})
				r.stats.CachedPlans++
				continue
			}
		}

		// System prompt via Eval.
		system, err := r.prog.Eval("inject std.vai_system")
		if err != nil {
			return nil, fmt.Errorf("eval system prompt: %w", err)
		}

		// User prompt: the full plan rendered by the language.
		user, err := r.prog.Eval("inject " + req.Name)
		if err != nil {
			return nil, fmt.Errorf("eval plan %s: %w", req.Name, err)
		}

		r.emit(Event{Kind: EventInfo, Step: "architect", Name: req.Name, Message: fmt.Sprintf("calling planner for plan %q...", req.Name)})
		resp, err := r.planner.Call(ctx, provider.Request{
			System:    system,
			Messages:  []provider.Message{{Role: "user", Content: user}},
			Tools:     []provider.ToolDefinition{tools.PlanSkeletonTool()},
			Model:     r.cfg.Planner.Model,
			MaxTokens: r.cfg.Planner.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("planner call for %s: %w", req.Name, err)
		}

		r.emit(Event{Kind: EventInfo, Step: "architect", Name: req.Name, Message: fmt.Sprintf("planner tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)})
		r.stats.ArchitectTokensIn += resp.TokensIn
		r.stats.ArchitectTokensOut += resp.TokensOut
		r.stats.ArchitectCycles++

		for _, tc := range resp.ToolCalls {
			if tc.Name == "plan_skeleton" {
				skeleton, err := tools.ParsePlanSkeleton(tc.Input)
				if err != nil {
					return nil, fmt.Errorf("parsing skeleton for %s: %w", req.Name, err)
				}
				results = append(results, planSkeletonResult{
					planName:    req.Name,
					targetPaths: req.TargetPaths,
					sourcePath:  req.SourcePath,
					skeleton:    *skeleton,
				})

				// Record successful plan hash in the lock.
				if r.locker != nil {
					planHash := r.computePlanHash(req.Name)
					if planHash != "" {
						r.locker.Lock("plan:"+req.Name, planHash)
					}
				}
			}
		}
	}
	return results, nil
}

// astImplToSkeleton converts an AST ImplDecl to a SkeletonImpl,
// extracting instruction text and [use]/[target] directives from body segments.
func astImplToSkeleton(impl *ast.ImplDecl) tools.SkeletonImpl {
	si := tools.SkeletonImpl{
		Name:   impl.Name,
		Action: "update",
	}
	si.Instruction = render.BodyText(impl.Body)
	for _, seg := range impl.Body {
		switch s := seg.(type) {
		case *ast.UseRefSegment:
			si.Uses = append(si.Uses, s.Name)
		case *ast.TargetRefSegment:
			si.Target = s.Name
		}
	}
	return si
}

// saveLock persists the lock file to disk if a locker is configured.
func (r *Runner) saveLock() error {
	if r.locker != nil {
		return r.locker.Save()
	}
	return nil
}

// computePlanHash extracts source-level text from the plan's AST and hashes it.
func (r *Runner) computePlanHash(planName string) string {
	f := r.prog.File()
	if f == nil {
		return ""
	}
	for _, decl := range f.Declarations {
		pd, ok := decl.(*ast.PlanDecl)
		if !ok || pd.Name != planName {
			continue
		}
		var specTexts []string
		for _, spec := range pd.Specs {
			specTexts = append(specTexts, render.BodyText(spec.Body))
		}
		var constraintTexts []string
		for _, c := range pd.Constraints {
			constraintTexts = append(constraintTexts, render.BodyText(c.Body))
		}
		var implEntries []locker.ImplEntry
		for _, impl := range pd.Impls {
			implEntries = append(implEntries, locker.ImplEntry{
				Name:     impl.Name,
				BodyText: render.BodyText(impl.Body),
			})
		}
		return locker.HashPlan(pd.Name, pd.Targets, specTexts, constraintTexts, implEntries)
	}
	return ""
}

// fillLockedSkeletons adds skeleton results for plans that were skipped by the
// architect (locked). The skeletons are built from the existing AST impls,
// which were saved from a previous run.
func (r *Runner) fillLockedSkeletons(archSkeletons []planSkeletonResult) []planSkeletonResult {
	handled := map[string]bool{}
	for _, s := range archSkeletons {
		handled[s.planName] = true
	}
	programSkeletons := r.buildSkeletonsFromProgram()
	for _, ps := range programSkeletons {
		if !handled[ps.planName] {
			archSkeletons = append(archSkeletons, ps)
		}
	}
	return archSkeletons
}

// mergeImpls merges architect-generated impls with the user's AST impls.
// Architect impls are the base; user AST impls provide [use] overrides.
// Impls with action "remove" are filtered out.
func (r *Runner) mergeImpls(skeletons []planSkeletonResult) {
	planIndex := map[string]*ast.PlanDecl{}
	if f := r.prog.File(); f != nil {
		for _, decl := range f.Declarations {
			if pd, ok := decl.(*ast.PlanDecl); ok {
				planIndex[pd.Name] = pd
			}
		}
	}
	for i, skel := range skeletons {
		plan := planIndex[skel.planName]

		// Index user's AST impls by name.
		astIndex := map[string]*ast.ImplDecl{}
		if plan != nil {
			for _, impl := range plan.Impls {
				astIndex[impl.Name] = impl
			}
		}

		// Filter and merge architect impls with user overrides.
		var merged []tools.SkeletonImpl
		for _, archImpl := range skel.skeleton.Impls {
			if archImpl.Action == "remove" {
				continue
			}
			// If user has a matching AST impl, preserve user's [use] directives.
			if userImpl, ok := astIndex[archImpl.Name]; ok {
				astSkel := astImplToSkeleton(userImpl)
				if len(astSkel.Uses) > 0 {
					archImpl.Uses = astSkel.Uses
				}
				if astSkel.Target != "" {
					archImpl.Target = astSkel.Target
				}
			}
			merged = append(merged, archImpl)
		}
		skeletons[i].skeleton.Impls = merged
	}
}
