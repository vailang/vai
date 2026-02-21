package composer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// paramNames returns a set of parameter names for local [use X] resolution.
func paramNames(params []ast.Param) map[string]bool {
	if len(params) == 0 {
		return nil
	}
	m := make(map[string]bool, len(params))
	for _, p := range params {
		m[p.Name] = true
	}
	return m
}

// typeAliasNames returns a set of type alias names for local [use X] resolution.
func typeAliasNames(types []*ast.TypeAlias) map[string]bool {
	if len(types) == 0 {
		return nil
	}
	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t.Name] = true
	}
	return m
}

// mergeNames combines two name sets.
func mergeNames(a, b map[string]bool) map[string]bool {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	m := make(map[string]bool, len(a)+len(b))
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	return m
}

// Validate runs all semantic checks and returns collected errors.
func (c *Composer) Validate() []Error {
	c.buildDeclaredMap()
	c.resolveTargets()
	for _, file := range c.src.Files() {
		c.validateFile(file)
	}
	return c.errors
}

// resolveTargets discovers all target files from the AST and loads their symbols.
// Targets are found in: File.TargetPath, PlanDecl.Targets, and [target "path"]
// directives inside any body segment. Each file is loaded once via the
// TargetResolver (which caches internally).
func (c *Composer) resolveTargets() {
	if c.targetResolver == nil {
		return
	}

	c.targetSymbols = map[string]ast.SymbolKind{}
	c.targetSigs = map[string]string{}

	seen := map[string]bool{}
	for _, file := range c.src.Files() {
		baseDir := filepath.Dir(file.SourcePath)
		resolve := func(target string) string {
			if target == "" {
				return ""
			}
			if filepath.IsAbs(target) {
				return target
			}
			return filepath.Join(baseDir, target)
		}

		// File-level target.
		if file.TargetPath != "" {
			abs := resolve(file.TargetPath)
			if !seen[abs] {
				seen[abs] = true
				c.targetPaths = append(c.targetPaths, abs)
			}
		}

		// Plan-level targets + body-level [target "path"] directives.
		for _, decl := range file.Declarations {
			if pd, ok := decl.(*ast.PlanDecl); ok {
				for _, target := range pd.Targets {
					abs := resolve(target)
					if !seen[abs] {
						seen[abs] = true
						c.targetPaths = append(c.targetPaths, abs)
					}
				}
			}
			// Walk all body segments to find [target] directives.
			ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
				if tr, ok := seg.(*ast.TargetRefSegment); ok {
					abs := resolve(tr.Name)
					if !seen[abs] {
						seen[abs] = true
						c.targetPaths = append(c.targetPaths, abs)
					}
				}
			})
		}
	}

	// Resolve each target file.
	for _, path := range c.targetPaths {
		symbols, sigs, err := c.targetResolver.ResolveTarget(path)
		if err != nil {
			c.errorf(ast.Position{}, "target %q: %s", path, err)
			continue
		}
		for name, kind := range symbols {
			c.targetSymbols[name] = kind
		}
		for name, sig := range sigs {
			c.targetSigs[name] = sig
		}
	}
}

// errorf records a semantic error.
func (c *Composer) errorf(pos ast.Position, format string, args ...any) {
	c.errors = append(c.errors, Error{
		Msg: fmt.Sprintf(format, args...),
		Pos: pos,
	})
}

// validateFile checks all declarations and inject prompts in a file.
func (c *Composer) validateFile(file *ast.File) {
	for _, decl := range file.Declarations {
		c.validateDeclBody(decl)
	}
	for _, ip := range file.InjectPrompts {
		c.validateDeclBody(ip)
	}
}

// validateDeclBody walks all body segments in a declaration and validates references.
// Local names (params, type aliases) are resolved per-context to allow [use X] for params.
func (c *Composer) validateDeclBody(decl ast.Declaration) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		locals := paramNames(d.Params)
		c.validateSegments(d.Body, locals)

	case *ast.StructDecl:
		aliases := typeAliasNames(d.Types)
		c.validateSegments(d.Body, aliases)
		for _, m := range d.Methods {
			locals := mergeNames(aliases, paramNames(m.Params))
			c.validateSegments(m.Body, locals)
		}

	case *ast.InterfaceDecl:
		aliases := typeAliasNames(d.Types)
		c.validateSegments(d.Body, aliases)
		for _, m := range d.Methods {
			locals := mergeNames(aliases, paramNames(m.Params))
			c.validateSegments(m.Body, locals)
		}

	case *ast.PlanDecl:
		for _, inner := range d.Declarations {
			c.validateDeclBody(inner)
		}
		for _, con := range d.Constraints {
			c.validateSegments(con.Body, nil)
		}
		for _, spec := range d.Specs {
			c.validateSegments(spec.Body, nil)
		}
		seenImpls := map[string]ast.Position{}
		for _, impl := range d.Impls {
			c.validateSegments(impl.Body, nil)
			if prev, dup := seenImpls[impl.Signature]; dup {
				c.errorf(impl.Pos, "duplicate impl %q in plan %q (first at %d:%d)", impl.Signature, d.Name, prev.Line, prev.Column)
			} else {
				seenImpls[impl.Signature] = impl.Pos
			}
		}

	default:
		// PromptDecl, ConstraintDecl, SpecDecl, InjectPromptDecl — no local names.
		ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
			c.validateSegment(seg, nil)
		})
	}
}

// validateSegments validates a slice of body segments with local name context.
func (c *Composer) validateSegments(segs []ast.BodySegment, locals map[string]bool) {
	for _, seg := range segs {
		c.validateSegment(seg, locals)
	}
}

// validateSegment dispatches validation for a single body segment.
func (c *Composer) validateSegment(seg ast.BodySegment, locals map[string]bool) {
	switch s := seg.(type) {
	case *ast.UseRefSegment:
		c.validateUseRef(s, locals)
	case *ast.InjectRefSegment:
		c.validateInjectRef(s)
	case *ast.MatchSegment:
		for _, clause := range s.Cases {
			c.validateSegments(clause.Body, locals)
		}
	}
}

// validateUseRef checks that a [use X] reference points to a valid, usable declaration.
// locals contains param and type-alias names visible in the current scope.
func (c *Composer) validateUseRef(ref *ast.UseRefSegment, locals map[string]bool) {
	// Check local names (params, type aliases) first.
	if locals[ref.Name] {
		return
	}

	decl, found := c.declared[ref.Name]
	if !found {
		// Not in AST — check target symbols.
		if c.targetSymbols != nil {
			if _, ok := c.targetSymbols[ref.Name]; ok {
				return // valid target symbol (+code/+doc are valid on target symbols)
			}
		}
		// Check external (host file) symbols.
		if c.symbols != nil {
			if _, ok := c.symbols[ref.Name]; ok {
				return // valid external symbol
			}
		}
		c.errorf(ref.Pos, "[use %s]: '%s' is not declared", ref.Name, ref.Name)
		return
	}

	// Found in AST — +code/+doc are only valid on external (host/target) symbols.
	if ref.AppendCode || ref.AppendDoc {
		mod := modifierString(ref.AppendCode, ref.AppendDoc)
		c.errorf(ref.Pos, "[use %s%s]: %s is only valid for external (host file) references", ref.Name, mod, mod)
		return
	}

	// Prompts are metadata, not tasks — cannot be used as dependencies.
	// But if the same name exists as an external/target symbol, prefer that.
	if ast.IsPrompt(decl) {
		if c.targetSymbols != nil {
			if _, ok := c.targetSymbols[ref.Name]; ok {
				return
			}
		}
		if c.symbols != nil {
			if _, ok := c.symbols[ref.Name]; ok {
				return
			}
		}
		c.errorf(ref.Pos, "[use %s]: cannot use prompt as a dependency; prompts are metadata, not tasks", ref.Name)
		return
	}

	// Declarations without a body produce no output — cannot be depended on.
	if !ast.HasBody(decl) {
		c.errorf(ref.Pos, "[use %s]: cannot use '%s' as a dependency; it has no body", ref.Name, ref.Name)
		return
	}
}

// validateInjectRef checks that an [inject X] reference points to a prompt.
func (c *Composer) validateInjectRef(ref *ast.InjectRefSegment) {
	// Qualified paths (e.g. "module.PromptName") — validate against known prompts if available.
	if strings.Contains(ref.Path, ".") {
		if c.knownPrompts != nil && strings.HasPrefix(ref.Path, "std.") {
			if !c.knownPrompts[ref.Path] {
				c.errorf(ref.Pos, "[inject %s]: prompt '%s' does not exist", ref.Path, ref.Path)
			}
		}
		return
	}

	// Local path — must resolve to a PromptDecl.
	decl, found := c.declared[ref.Path]
	if !found {
		c.errorf(ref.Pos, "[inject %s]: '%s' is not declared", ref.Path, ref.Path)
		return
	}

	if !ast.IsPrompt(decl) && !ast.IsPlan(decl) {
		c.errorf(ref.Pos, "[inject %s]: must point to a prompt or plan declaration, not %s", ref.Path, ast.KindName(decl))
	}
}

func modifierString(appendCode, appendDoc bool) string {
	switch {
	case appendCode && appendDoc:
		return "+code+doc"
	case appendCode:
		return "+code"
	default:
		return "+doc"
	}
}
