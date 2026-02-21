package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vailang/vai/internal/coder"
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/composer"
	"github.com/vailang/vai/internal/compiler/lexer"
	"github.com/vailang/vai/internal/compiler/parser"
	"github.com/vailang/vai/internal/compiler/reader"
)

// compiler implements the Compiler interface.
type compiler struct{}

// New creates a new Compiler.
func New() Compiler {
	return &compiler{}
}

// Parse reads .vai/.plan files from path (file or directory) and compiles
// them through the full pipeline: read → lex → parse → compose → program.
func (c *compiler) Parse(path string) (Program, []error) {
	sources, err := reader.ReadPaths(path)
	if err != nil {
		return &program{}, []error{err}
	}
	if len(sources) == 0 {
		return &program{}, []error{fmt.Errorf("no .vai or .plan files found in %s", path)}
	}

	return c.parseSources(sources)
}

// Eval reads .vai files from path, appends eval source, compiles, and renders.
func (c *compiler) Eval(path string, eval string) (string, error) {
	sources, err := reader.ReadPaths(path)
	if err != nil {
		return "", err
	}
	if len(sources) == 0 {
		return "", fmt.Errorf("no .vai or .plan files found in %s", path)
	}

	// Determine the base directory from real sources for the eval virtual file.
	absPath, _ := filepath.Abs(path)
	info, _ := os.Stat(absPath)
	evalDir := absPath
	if info != nil && !info.IsDir() {
		evalDir = filepath.Dir(absPath)
	}
	evalPath := filepath.Join(evalDir, "<eval>.vai")

	// Append the eval string as a virtual source file in the same directory.
	sources[evalPath] = eval

	prog, errs := c.parseSources(sources)
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return "", fmt.Errorf("%s", strings.Join(msgs, "\n"))
	}

	return prog.Render(), nil
}

// parseSources compiles multiple vai sources into a single program.
func (c *compiler) parseSources(sources map[string]string) (Program, []error) {
	// Sort paths for deterministic ordering.
	paths := make([]string, 0, len(sources))
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Lex and parse each file individually.
	var files []*ast.File
	var errs []error
	for _, filePath := range paths {
		cs := reader.NewVaiSource(sources[filePath])
		scanner := lexer.NewScanner(cs)
		p := parser.New(scanner)
		file, parseErrs := p.ParseFile()
		for _, pe := range parseErrs {
			errs = append(errs, pe)
		}
		file.SourcePath = filePath
		files = append(files, file)
	}
	if len(errs) > 0 {
		return &program{}, errs
	}

	// Create a target resolver that lazily loads coders on demand.
	tr := newTargetResolver()

	astSrc := &fileSource{files: files}
	comp := composer.New(astSrc, nil, tr, nil)
	compErrs := comp.Validate()
	for _, ce := range compErrs {
		errs = append(errs, ce)
	}
	if len(errs) > 0 {
		tr.Close()
		return &program{}, errs
	}

	// Merge all files for program execution.
	merged, mergeErr := ast.MergeFiles(files)
	if mergeErr != nil {
		tr.Close()
		return &program{}, []error{mergeErr}
	}

	requests := comp.Requests()
	return &program{file: merged, requests: requests, targetResolver: tr}, nil
}

// ---------------------------------------------------------------------------
// targetResolverImpl — lazily loads coders with caching
// ---------------------------------------------------------------------------

type targetResolverImpl struct {
	coders map[string]*coder.Coder // absolute path → loaded coder
}

func newTargetResolver() *targetResolverImpl {
	return &targetResolverImpl{coders: make(map[string]*coder.Coder)}
}

func (r *targetResolverImpl) getOrCreate(path string) (*coder.Coder, error) {
	if cod, ok := r.coders[path]; ok {
		return cod, nil
	}

	lang, err := coder.DetectLanguage(path)
	if err != nil {
		return nil, fmt.Errorf("unsupported target language for %s: %w", path, err)
	}

	cod, err := coder.New(lang, path)
	if err != nil {
		return nil, err
	}

	src, err := os.ReadFile(path)
	if err != nil {
		// Target file doesn't exist yet — coder with no symbols.
		r.coders[path] = cod
		return cod, nil
	}

	if err := cod.Load(src); err != nil {
		cod.Close()
		return nil, fmt.Errorf("loading symbols from %s: %w", path, err)
	}

	r.coders[path] = cod
	return cod, nil
}

func (r *targetResolverImpl) ResolveTarget(path string) (map[string]ast.SymbolKind, map[string]string, error) {
	cod, err := r.getOrCreate(path)
	if err != nil {
		return nil, nil, err
	}

	raw := cod.Symbols()
	symbols := make(map[string]ast.SymbolKind, len(raw))
	sigs := make(map[string]string, len(raw))
	for name, kind := range raw {
		symbols[name] = ast.SymbolKind(kind)
		if resolved, ok := cod.Resolve(name); ok {
			sigs[name] = resolved.Signature
		}
	}
	return symbols, sigs, nil
}

func (r *targetResolverImpl) GetCode(path, name string) (string, bool) {
	cod, ok := r.coders[path]
	if !ok {
		return "", false
	}
	if resolved, ok := cod.Resolve(name); ok {
		return resolved.Code, resolved.Code != ""
	}
	return "", false
}

func (r *targetResolverImpl) GetSkeleton(path string) (string, bool) {
	cod, err := r.getOrCreate(path)
	if err != nil {
		return "", false
	}
	skeleton, err := cod.Skeleton()
	if err != nil {
		return "", false
	}
	return skeleton, skeleton != ""
}

func (r *targetResolverImpl) GetDoc(path, name string) (string, bool) {
	cod, ok := r.coders[path]
	if !ok {
		return "", false
	}
	if resolved, ok := cod.Resolve(name); ok {
		return resolved.Doc, resolved.Doc != ""
	}
	return "", false
}

func (r *targetResolverImpl) Close() {
	for _, cod := range r.coders {
		cod.Close()
	}
}

// ---------------------------------------------------------------------------
// fileSource — implements composer.ASTSource
// ---------------------------------------------------------------------------

type fileSource struct {
	files []*ast.File
}

func (f *fileSource) Files() []*ast.File {
	return f.files
}

// ---------------------------------------------------------------------------
// program — implements Program
// ---------------------------------------------------------------------------

type program struct {
	file           *ast.File
	requests       []composer.Request
	targetResolver *targetResolverImpl
}

func (p *program) Tasks() int {
	return len(p.requests)
}

// Exec resolves inject declarations by looking up the referenced prompts
// or plans and assembling their body text.
func (p *program) Exec() (string, error) {
	if p.file == nil {
		return "", nil
	}

	prompts := p.indexPrompts()
	plans := p.indexPlans()
	baseDir := filepath.Dir(p.file.SourcePath)
	var parts []string
	for _, decl := range p.file.Declarations {
		inj, ok := decl.(*ast.InjectDecl)
		if !ok {
			continue
		}
		if pd, found := prompts[inj.Name]; found {
			parts = append(parts, p.renderBodyText(pd.Body))
		} else if plan, found := plans[inj.Name]; found {
			parts = append(parts, p.renderPlanResolved(plan, baseDir))
		}
	}

	return strings.Join(parts, "\n"), nil
}

// Render produces the fully resolved structured text of the program.
func (p *program) Render() string {
	if p.file == nil {
		return ""
	}

	prompts := p.indexPrompts()
	plans := p.indexPlans()
	baseDir := filepath.Dir(p.file.SourcePath)
	var buf strings.Builder

	// 1. Render inject declarations — prompts or plans.
	hasPlanInject := false
	for _, decl := range p.file.Declarations {
		inj, ok := decl.(*ast.InjectDecl)
		if !ok {
			continue
		}
		if pd, found := prompts[inj.Name]; found {
			text := p.renderBodyResolved(pd.Body, baseDir)
			if text != "" {
				buf.WriteString(text)
				buf.WriteString("\n\n")
			}
		} else if plan, found := plans[inj.Name]; found {
			hasPlanInject = true
			buf.WriteString(p.renderPlanResolved(plan, baseDir))
			buf.WriteString("\n")
		}
	}

	// 2. Render constraints (only when no plan was injected, since plan rendering includes them).
	if !hasPlanInject {
		constraints := p.collectConstraints()
		if len(constraints) > 0 {
			buf.WriteString("## Global Constraint\n")
			for _, c := range constraints {
				if c.Name != "" {
					buf.WriteString("**" + c.Name + "**")
				} else {
					buf.WriteString("-")
				}
				body := p.renderBodyResolved(c.Body, baseDir)
				if body != "" {
					buf.WriteString(" " + body)
				}
				buf.WriteString("\n")
			}
		}
	}

	return buf.String()
}

// renderPlanResolved renders a plan as structured, self-contained output.
func (p *program) renderPlanResolved(plan *ast.PlanDecl, baseDir string) string {
	var buf strings.Builder

	// # Plan Name
	buf.WriteString("# " + plan.Name + "\n\n")

	// ## Specification
	if len(plan.Specs) > 0 {
		buf.WriteString("## Specification\n")
		for _, spec := range plan.Specs {
			text := p.renderBodyResolved(spec.Body, baseDir)
			if text != "" {
				buf.WriteString(text + "\n")
			}
		}
		buf.WriteString("\n---\n\n")
	}

	// ## Target File Status
	if len(plan.Targets) > 0 {
		buf.WriteString("## Target File Status\n")
		for _, target := range plan.Targets {
			absPath := target
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(baseDir, absPath)
			}
			// Try skeleton view first — full file structure with empty bodies.
			if skeleton, ok := p.targetResolver.GetSkeleton(absPath); ok {
				lang := langTag(absPath)
				buf.WriteString("### " + target + "\n")
				buf.WriteString("```" + lang + "\n")
				buf.WriteString(skeleton)
				if !strings.HasSuffix(skeleton, "\n") {
					buf.WriteString("\n")
				}
				buf.WriteString("```\n\n")
			} else {
				// Fallback to symbol listing if skeleton fails.
				symbols, sigs, err := p.targetResolver.ResolveTarget(absPath)
				if err != nil {
					continue
				}
				buf.WriteString("### " + target + "\n")
				for name := range symbols {
					if sig, ok := sigs[name]; ok {
						buf.WriteString("- `" + sig + "`\n")
					} else {
						buf.WriteString("- " + name + "\n")
					}
				}
				buf.WriteString("\n")
			}
		}
		buf.WriteString("---\n\n")
	}

	// ## Implementation Order — each impl is atomic
	if len(plan.Impls) > 0 {
		buf.WriteString("## Implementation Order\n")
		for _, impl := range plan.Impls {
			buf.WriteString(p.renderImplAtomic(impl, baseDir))
			buf.WriteString("\n")
		}
		buf.WriteString("---\n\n")
	}

	// ## Global Constraints
	globalConstraints := p.collectGlobalConstraints()
	if len(globalConstraints) > 0 {
		buf.WriteString("## Global Constraints\n")
		for _, c := range globalConstraints {
			p.renderConstraintEntry(&buf, c, baseDir)
		}
		buf.WriteString("\n---\n\n")
	}

	// ## Plan Constraints
	if len(plan.Constraints) > 0 {
		buf.WriteString("## Plan Constraints\n")
		for _, c := range plan.Constraints {
			p.renderConstraintEntry(&buf, c, baseDir)
		}
	}

	return buf.String()
}

// renderConstraintEntry renders a single constraint.
// Named constraints use a #### heading; anonymous ones render as list items.
func (p *program) renderConstraintEntry(buf *strings.Builder, c *ast.ConstraintDecl, baseDir string) {
	body := p.renderBodyResolved(c.Body, baseDir)
	if c.Name != "" {
		buf.WriteString("#### " + c.Name + "\n")
		if body != "" {
			buf.WriteString(body + "\n")
		}
		buf.WriteString("\n")
	} else if body != "" {
		buf.WriteString("- " + body + "\n")
	}
}

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

func (p *program) GetPlanSpec(name string) string {
	plan := p.findPlan(name)
	if plan == nil {
		return ""
	}
	baseDir := filepath.Dir(p.file.SourcePath)
	var parts []string
	for _, spec := range plan.Specs {
		text := p.renderBodyResolved(spec.Body, baseDir)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func (p *program) GetPlanImpl(name string) []string {
	plan := p.findPlan(name)
	if plan == nil {
		return nil
	}
	baseDir := filepath.Dir(p.file.SourcePath)

	var results []string
	for _, impl := range plan.Impls {
		results = append(results, p.renderImplAtomic(impl, baseDir))
	}
	return results
}

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

// ---------------------------------------------------------------------------
// program helpers
// ---------------------------------------------------------------------------

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

// renderImplAtomic renders a single impl as a self-contained string.
// Each impl includes its signature, body text, and a ## Reference section
// with all resolved [use X] dependencies.
func (p *program) renderImplAtomic(impl *ast.ImplDecl, baseDir string) string {
	// Resolve target symbols for this plan's context.
	symbols, sigs, targetPaths := p.resolveAllTargets(baseDir)

	var buf strings.Builder
	buf.WriteString("### impl \"" + impl.Signature + "\"\n")

	// Render body text (excluding [use] refs which go into Reference section).
	var textParts []string
	var refs []*ast.UseRefSegment
	for _, seg := range impl.Body {
		switch s := seg.(type) {
		case *ast.TextSegment:
			text := strings.TrimSpace(s.Content)
			if text != "" {
				textParts = append(textParts, text)
			}
		case *ast.UseRefSegment:
			refs = append(refs, s)
		case *ast.InjectRefSegment:
			prompts := p.indexPrompts()
			if pd, found := prompts[s.Path]; found {
				text := p.renderBodyText(pd.Body)
				if text != "" {
					textParts = append(textParts, text)
				}
			}
		}
	}

	if len(textParts) > 0 {
		buf.WriteString(strings.Join(textParts, "\n"))
		buf.WriteString("\n")
	}

	// Reference section with resolved [use] dependencies.
	if len(refs) > 0 {
		buf.WriteString("\n### Reference\n")
		for _, ref := range refs {
			resolved := p.renderUseRef(ref, symbols, sigs, targetPaths)
			buf.WriteString("- **" + ref.Name + "**: " + resolved + "\n")
		}
	}

	return buf.String()
}

// resolveAllTargets loads symbols from all known target paths.
func (p *program) resolveAllTargets(baseDir string) (map[string]ast.SymbolKind, map[string]string, []string) {
	symbols := map[string]ast.SymbolKind{}
	sigs := map[string]string{}
	if p.targetResolver == nil {
		return symbols, sigs, nil
	}

	var targetPaths []string
	seen := map[string]bool{}

	// Collect all target paths from all plans.
	for _, decl := range p.file.Declarations {
		if pd, ok := decl.(*ast.PlanDecl); ok {
			for _, target := range pd.Targets {
				absPath := target
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(baseDir, absPath)
				}
				if !seen[absPath] {
					seen[absPath] = true
					targetPaths = append(targetPaths, absPath)
				}
			}
		}
	}

	for _, path := range targetPaths {
		s, si, err := p.targetResolver.ResolveTarget(path)
		if err != nil {
			continue
		}
		for k, v := range s {
			symbols[k] = v
		}
		for k, v := range si {
			sigs[k] = v
		}
	}

	return symbols, sigs, targetPaths
}

// collectGlobalConstraints returns only top-level constraints (not inside plans).
func (p *program) collectGlobalConstraints() []*ast.ConstraintDecl {
	if p.file == nil {
		return nil
	}
	var constraints []*ast.ConstraintDecl
	for _, decl := range p.file.Declarations {
		if c, ok := decl.(*ast.ConstraintDecl); ok {
			constraints = append(constraints, c)
		}
	}
	return constraints
}

func (p *program) indexPrompts() map[string]*ast.PromptDecl {
	prompts := map[string]*ast.PromptDecl{}
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

func (p *program) collectConstraints() []*ast.ConstraintDecl {
	if p.file == nil {
		return nil
	}
	var constraints []*ast.ConstraintDecl
	for _, decl := range p.file.Declarations {
		switch d := decl.(type) {
		case *ast.ConstraintDecl:
			constraints = append(constraints, d)
		case *ast.PlanDecl:
			constraints = append(constraints, d.Constraints...)
		}
	}
	return constraints
}

// renderBodyText concatenates text segments only (no symbol resolution).
func (p *program) renderBodyText(segments []ast.BodySegment) string {
	var parts []string
	for _, seg := range segments {
		if ts, ok := seg.(*ast.TextSegment); ok {
			text := strings.TrimSpace(ts.Content)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// renderBodyResolved renders body segments with full symbol resolution.
// Target files are loaded on demand; [use X] references are resolved to
// code fences (structs) or inline signatures (functions).
func (p *program) renderBodyResolved(segments []ast.BodySegment, baseDir string) string {
	// Collect target files referenced in this body.
	targetPaths := p.collectBodyTargetPaths(segments, baseDir)

	// Build symbol lookup from those targets.
	symbols := map[string]ast.SymbolKind{}
	sigs := map[string]string{}
	for _, path := range targetPaths {
		s, si, err := p.targetResolver.ResolveTarget(path)
		if err != nil {
			continue
		}
		for k, v := range s {
			symbols[k] = v
		}
		for k, v := range si {
			sigs[k] = v
		}
	}

	var parts []string
	for _, seg := range segments {
		switch s := seg.(type) {
		case *ast.TextSegment:
			text := strings.TrimSpace(s.Content)
			if text != "" {
				parts = append(parts, text)
			}
		case *ast.UseRefSegment:
			parts = append(parts, p.renderUseRef(s, symbols, sigs, targetPaths))
		case *ast.TargetRefSegment:
			// Context only — no output.
		case *ast.InjectRefSegment:
			prompts := p.indexPrompts()
			if pd, found := prompts[s.Path]; found {
				text := p.renderBodyText(pd.Body)
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

// collectBodyTargetPaths finds all [target "path"] directives in body segments
// and resolves them to absolute paths.
func (p *program) collectBodyTargetPaths(segments []ast.BodySegment, baseDir string) []string {
	var paths []string
	seen := map[string]bool{}
	for _, seg := range segments {
		tr, ok := seg.(*ast.TargetRefSegment)
		if !ok {
			continue
		}
		absPath := tr.Name
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(baseDir, absPath)
		}
		if !seen[absPath] {
			seen[absPath] = true
			paths = append(paths, absPath)
		}
	}
	return paths
}

// renderUseRef resolves a [use X] reference to formatted output.
// Structs/classes → code fence with full definition.
// Functions → inline backtick signature.
func (p *program) renderUseRef(ref *ast.UseRefSegment, symbols map[string]ast.SymbolKind, sigs map[string]string, targetPaths []string) string {
	kind := symbols[ref.Name]

	switch kind {
	case ast.SymbolStruct, ast.SymbolClass:
		// Find the code from the target files.
		for _, path := range targetPaths {
			if code, ok := p.targetResolver.GetCode(path, ref.Name); ok {
				lang := langTag(path)
				return "```" + lang + "\n" + code + "\n```"
			}
		}
		// Fallback to signature.
		if sig, ok := sigs[ref.Name]; ok {
			return "`" + sig + "`"
		}
	default:
		// Functions, interfaces, etc. → inline signature.
		if sig, ok := sigs[ref.Name]; ok {
			return "`" + sig + "`"
		}
	}

	return "[use " + ref.Name + "]"
}

// langTag returns the code fence language tag for a file path.
func langTag(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".c", ".h":
		return "c"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	default:
		return ""
	}
}
