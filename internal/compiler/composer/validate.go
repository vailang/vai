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

// resolveTargets discovers all target and reference files from the AST and loads their symbols.
// Targets are found in: File.TargetPath, PlanDecl.Targets, and [target "path"]
// directives inside any body segment. References are found in: PlanDecl.References,
// PromptDecl.References, and [reference "path"] directives.
// Each file is loaded once via the TargetResolver (which caches internally).
// Reference symbols are merged into the same lookup maps but tracked separately
// so they are NOT included in status output.
func (c *Composer) resolveTargets() {
	if c.targetResolver == nil {
		return
	}

	c.targetSymbols = map[string]ast.SymbolKind{}
	c.targetSigs = map[string]string{}
	c.referencePaths = nil

	seen := map[string]bool{}
	refSeen := map[string]bool{}
	for _, file := range c.src.Files() {
		fileDir := filepath.Dir(file.SourcePath)
		resolve := func(target string) string {
			if target == "" {
				return ""
			}
			if filepath.IsAbs(target) {
				return target
			}
			if c.baseDir != "" {
				return filepath.Join(c.baseDir, target)
			}
			return filepath.Join(fileDir, target)
		}

		// File-level target.
		if file.TargetPath != "" {
			abs := resolve(file.TargetPath)
			if !seen[abs] {
				seen[abs] = true
				c.targetPaths = append(c.targetPaths, abs)
			}
		}

		// Plan-level targets + references, body-level [target] and [reference] directives.
		for _, decl := range file.Declarations {
			if pd, ok := decl.(*ast.PlanDecl); ok {
				for _, target := range pd.Targets {
					abs := resolve(target)
					if !seen[abs] {
						seen[abs] = true
						c.targetPaths = append(c.targetPaths, abs)
					}
				}
				for _, ref := range pd.References {
					abs := resolve(ref)
					if !refSeen[abs] {
						refSeen[abs] = true
						c.referencePaths = append(c.referencePaths, abs)
					}
				}
			}
			// Collect references from prompt/constraint declarations.
			if pd, ok := decl.(*ast.PromptDecl); ok {
				for _, ref := range pd.References {
					abs := resolve(ref)
					if !refSeen[abs] {
						refSeen[abs] = true
						c.referencePaths = append(c.referencePaths, abs)
					}
				}
			}
			// Walk all body segments to find [target] and [reference] directives.
			ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
				switch s := seg.(type) {
				case *ast.TargetRefSegment:
					abs := resolve(s.Name)
					if !seen[abs] {
						seen[abs] = true
						c.targetPaths = append(c.targetPaths, abs)
					}
				case *ast.ReferenceRefSegment:
					// Check if this is a plan name reference — import its targets as references.
					if planDecl, found := c.declared[s.Name]; found {
						if pd, isPlan := planDecl.(*ast.PlanDecl); isPlan {
							for _, target := range pd.Targets {
								abs := resolve(target)
								if !refSeen[abs] {
									refSeen[abs] = true
									c.referencePaths = append(c.referencePaths, abs)
								}
							}
							return
						}
					}
					// Otherwise treat as a file path.
					abs := resolve(s.Name)
					if !refSeen[abs] {
						refSeen[abs] = true
						c.referencePaths = append(c.referencePaths, abs)
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

	// Resolve each reference file (symbols merged into same maps for [use] resolution).
	for _, path := range c.referencePaths {
		symbols, sigs, err := c.targetResolver.ResolveTarget(path)
		if err != nil {
			c.errorf(ast.Position{}, "reference %q: %s", path, err)
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
		Msg:      fmt.Sprintf(format, args...),
		Pos:      pos,
		Severity: SeverityError,
	})
}

// warnf records a semantic warning (does not block compilation).
func (c *Composer) warnf(pos ast.Position, format string, args ...any) {
	c.errors = append(c.errors, Error{
		Msg:      fmt.Sprintf(format, args...),
		Pos:      pos,
		Severity: SeverityWarning,
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
		// Build set of plan targets for impl validation.
		planTargets := map[string]bool{}
		for _, t := range d.Targets {
			planTargets[t] = true
		}

		seenImpls := map[string]ast.Position{}
		for _, impl := range d.Impls {
			c.validateSegments(impl.Body, nil)
			if prev, dup := seenImpls[impl.Name]; dup {
				c.errorf(impl.Pos, "duplicate impl %q in plan %q (first at %d:%d)", impl.Name, d.Name, prev.Line, prev.Column)
			} else {
				seenImpls[impl.Name] = impl.Pos
			}

			// Validate impl has exactly one [target] that is in the plan's targets.
			var implTargets []*ast.TargetRefSegment
			for _, seg := range impl.Body {
				if tr, ok := seg.(*ast.TargetRefSegment); ok {
					implTargets = append(implTargets, tr)
				}
			}
			if len(implTargets) == 0 {
				if len(d.Targets) == 0 {
					c.errorf(impl.Pos, "impl %q: [target] is required (plan has no targets)", impl.Name)
				} else if len(d.Targets) > 1 {
					c.errorf(impl.Pos, "impl %q: [target] is required when plan has multiple targets", impl.Name)
				}
				// single target → auto-inherit, no error
			} else if len(implTargets) > 1 {
				c.errorf(implTargets[1].Pos, "impl %q: only one [target] allowed", impl.Name)
			} else if !planTargets[implTargets[0].Name] {
				c.errorf(implTargets[0].Pos, "impl %q: target %q is not declared in plan %q", impl.Name, implTargets[0].Name, d.Name)
			}
		}

	case *ast.PromptDecl:
		// Forbid [target] in prompts — use [reference] instead.
		ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
			if tr, ok := seg.(*ast.TargetRefSegment); ok {
				c.errorf(tr.Pos, "[target] is not allowed in prompt declarations; use [reference] instead")
				return
			}
			c.validateSegment(seg, nil)
		})

	case *ast.ConstraintDecl:
		// Forbid [target] in constraints — use [reference] instead.
		ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
			if tr, ok := seg.(*ast.TargetRefSegment); ok {
				c.errorf(tr.Pos, "[target] is not allowed in constraint declarations; use [reference] instead")
				return
			}
			c.validateSegment(seg, nil)
		})

	default:
		// SpecDecl, InjectPromptDecl — no local names.
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
		c.warnf(ref.Pos, "[use %s]: '%s' is not declared", ref.Name, ref.Name)
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

// validateInjectRef checks that an [inject X] reference points to a prompt, plan, or plan.impl.
func (c *Composer) validateInjectRef(ref *ast.InjectRefSegment) {
	// Qualified paths (e.g. "module.PromptName" or "plan.implName").
	if strings.Contains(ref.Path, ".") {
		// Standard library prompts.
		if c.knownPrompts != nil && strings.HasPrefix(ref.Path, "std.") {
			if !c.knownPrompts[ref.Path] {
				c.errorf(ref.Pos, "[inject %s]: prompt '%s' does not exist", ref.Path, ref.Path)
			}
			return
		}
		// Check for plan.impl pattern — only validate if the first part is a known declaration.
		parts := strings.SplitN(ref.Path, ".", 2)
		if len(parts) == 2 {
			planDecl, found := c.declared[parts[0]]
			if !found {
				// Not a known declaration — might be an external module, skip validation.
				return
			}
			plan, ok := planDecl.(*ast.PlanDecl)
			if !ok {
				c.errorf(ref.Pos, "[inject %s]: '%s' is not a plan declaration", ref.Path, parts[0])
				return
			}
			// Check that the plan has an impl with that name.
			implFound := false
			for _, impl := range plan.Impls {
				if impl.Name == parts[1] {
					implFound = true
					break
				}
			}
			if !implFound {
				c.errorf(ref.Pos, "[inject %s]: plan '%s' has no impl '%s'", ref.Path, parts[0], parts[1])
			}
		}
		return
	}

	// Local path — must resolve to a PromptDecl or PlanDecl.
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
