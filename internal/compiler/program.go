package compiler

import (
	"path/filepath"
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/composer"
	"github.com/vailang/vai/internal/compiler/lexer"
	"github.com/vailang/vai/internal/compiler/parser"
	"github.com/vailang/vai/internal/compiler/reader"
	"github.com/vailang/vai/internal/compiler/render"
)

// program implements the Program interface.
type program struct {
	file           *ast.File
	requests       []composer.Request
	targetResolver *targetResolverImpl
	stdPrompts     map[string]*ast.PromptDecl // "std.name" → PromptDecl
	projectDir     string                     // project root (vai.toml dir); empty = use source file dir
	warnings       []error                    // non-fatal warnings from compilation
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

// Tasks returns the number of compiled requests (planner + executor).
func (p *program) Tasks() int {
	return len(p.requests)
}

// Exec resolves all inject declarations and returns the assembled body text.
func (p *program) Exec() (string, error) {
	if p.file == nil {
		return "", nil
	}
	result := render.Exec(p.file, p.indexPrompts(), p.indexPlans(), p.targetResolver, p.baseDir())
	return result, nil
}

// Eval parses source as vai code and resolves it within this program's context.
// Example: prog.Eval("inject my_plan") renders the plan from the compiled program.
func (p *program) Eval(source string) (string, error) {
	if p.file == nil {
		return "", nil
	}

	// Lex + parse the eval source.
	cs := reader.NewVaiSource(source)
	scanner := lexer.NewScanner(cs)
	par := parser.New(scanner)
	par.SetEvalMode(true)
	evalFile, errs := par.ParseFile()
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return "", &evalError{msg: strings.Join(msgs, "\n")}
	}

	// Merge eval declarations into the existing program context.
	combined, err := ast.MergeFiles([]*ast.File{p.file, evalFile})
	if err != nil {
		return "", err
	}

	tmp := &program{file: combined, requests: p.requests, targetResolver: p.targetResolver, stdPrompts: p.stdPrompts, projectDir: p.projectDir}
	result := render.Exec(combined, tmp.indexPrompts(), tmp.indexPlans(), p.targetResolver, tmp.baseDir())
	return result, nil
}

// Render produces the fully resolved structured markdown output.
func (p *program) Render() string {
	if p.file == nil {
		return ""
	}
	return render.Render(p.file, p.indexPrompts(), p.indexPlans(), p.targetResolver, p.baseDir())
}

// Warnings returns non-fatal diagnostics from compilation.
func (p *program) Warnings() []error {
	return p.warnings
}

// ---------------------------------------------------------------------------
// Inspection
// ---------------------------------------------------------------------------

// File returns the merged AST.
func (p *program) File() *ast.File {
	return p.file
}

// Requests returns the compiled requests.
func (p *program) Requests() []composer.Request {
	return p.requests
}

// HasPrompt reports whether a prompt with the given name exists.
func (p *program) HasPrompt(name string) bool {
	if p.file == nil {
		return false
	}
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PromptDecl); ok && pd.Name == name {
			return true
		}
	}
	return false
}

// HasPlan reports whether a plan with the given name exists.
func (p *program) HasPlan(name string) bool {
	if p.file == nil {
		return false
	}
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PlanDecl); ok && pd.Name == name {
			return true
		}
	}
	return false
}

// ListPrompts returns the names of all prompts in the program.
func (p *program) ListPrompts() []string {
	if p.file == nil {
		return nil
	}
	var names []string
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PromptDecl); ok {
			names = append(names, pd.Name)
		}
	}
	return names
}

// ListConstraints returns the names of all constraints (top-level and plan-scoped).
func (p *program) ListConstraints() []string {
	if p.file == nil {
		return nil
	}
	var names []string
	for _, decl := range p.file.Declarations {
		switch d := decl.(type) {
		case *ast.ConstraintDecl:
			names = append(names, d.Name)
		case *ast.PlanDecl:
			for _, c := range d.Constraints {
				names = append(names, c.Name)
			}
		}
	}
	return names
}

// GetPlanSpec returns the rendered specification for a named plan.
func (p *program) GetPlanSpec(name string) string {
	plan := p.findPlan(name)
	if plan == nil {
		return ""
	}
	prompts := p.indexPrompts()
	plans := p.indexPlans()
	baseDir := p.baseDir()
	var parts []string
	for _, spec := range plan.Specs {
		text := render.BodyResolved(spec.Body, prompts, plans, p.targetResolver, baseDir)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// GetPlanImpl returns the rendered implementations for a named plan.
func (p *program) GetPlanImpl(name string) []string {
	plan := p.findPlan(name)
	if plan == nil {
		return nil
	}
	prompts := p.indexPrompts()
	plans := p.indexPlans()
	baseDir := p.baseDir()

	var results []string
	for _, impl := range plan.Impls {
		results = append(results, render.ImplAtomic(impl, prompts, plans, p.targetResolver, baseDir))
	}
	return results
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// baseDir returns the project root directory for path resolution.
// Uses projectDir (vai.toml dir) when set, otherwise falls back to source file dir.
func (p *program) baseDir() string {
	if p.projectDir != "" {
		return p.projectDir
	}
	if p.file == nil {
		return ""
	}
	return filepath.Dir(p.file.SourcePath)
}

// findPlan finds a plan by name.
func (p *program) findPlan(name string) *ast.PlanDecl {
	if p.file == nil {
		return nil
	}
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PlanDecl); ok && pd.Name == name {
			return pd
		}
	}
	return nil
}

// indexPlans builds a name → PlanDecl map from the merged AST.
func (p *program) indexPlans() map[string]*ast.PlanDecl {
	plans := map[string]*ast.PlanDecl{}
	if p.file == nil {
		return plans
	}
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PlanDecl); ok {
			plans[pd.Name] = pd
		}
	}
	return plans
}

// indexPrompts builds a name → PromptDecl map from the merged AST,
// including standard library prompts with "std." prefix.
func (p *program) indexPrompts() map[string]*ast.PromptDecl {
	prompts := map[string]*ast.PromptDecl{}
	// Add std prompts first so user prompts can shadow them.
	for name, pd := range p.stdPrompts {
		prompts[name] = pd
	}
	if p.file == nil {
		return prompts
	}
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PromptDecl); ok {
			prompts[pd.Name] = pd
		}
	}
	return prompts
}

// evalError wraps parse errors from Eval.
type evalError struct {
	msg string
}

func (e *evalError) Error() string {
	return e.msg
}
