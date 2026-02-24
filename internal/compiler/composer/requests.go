package composer

import (
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// Requests produces the resolved request list from all files.
// Must be called after Validate returns no errors.
func (c *Composer) Requests() []Request {
	var reqs []Request
	for _, file := range c.src.Files() {
		reqs = append(reqs, c.fileRequests(file)...)
	}
	return reqs
}

// fileRequests builds requests from a single file's declarations.
func (c *Composer) fileRequests(file *ast.File) []Request {
	var reqs []Request

	for _, decl := range file.Declarations {
		declReqs := c.declRequests(decl)
		for i := range declReqs {
			declReqs[i].SourcePath = file.SourcePath
		}
		reqs = append(reqs, declReqs...)
	}

	return reqs
}

// declRequests builds requests for a single declaration, recursing into PlanDecl.
func (c *Composer) declRequests(decl ast.Declaration) []Request {
	// Skip declarations without bodies, prompts, constraints, and specs (metadata only).
	if !ast.HasBody(decl) || ast.IsPrompt(decl) || ast.IsConstraint(decl) || ast.IsSpec(decl) {
		return nil
	}

	// PlanDecl: produce requests for inner declarations + the plan itself.
	if plan, ok := decl.(*ast.PlanDecl); ok {
		var reqs []Request
		for _, inner := range plan.Declarations {
			innerReqs := c.declRequests(inner)
			for i := range innerReqs {
				innerReqs[i].PlanName = plan.Name
				innerReqs[i].TargetPaths = plan.Targets
			}
			reqs = append(reqs, innerReqs...)
		}
		// The plan itself is a PlannerAgent request.
		reqs = append(reqs, Request{
			Name:        plan.Name,
			TargetPaths: plan.Targets,
			Task:        buildTask(plan),
			Type:        PlannerAgent,
		})
		return reqs
	}

	req := Request{
		Name: decl.DeclName(),
		Task: buildTask(decl),
		Type: requestType(decl),
	}

	// Collect resolved references (skip local names like params/type aliases).
	ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
		ref, ok := seg.(*ast.UseRefSegment)
		if !ok {
			return
		}

		// Skip local references (params, type aliases) — not task dependencies.
		if _, isDeclared := c.declared[ref.Name]; !isDeclared {
			if _, isTarget := c.targetSymbols[ref.Name]; !isTarget {
				if _, isExternal := c.symbols[ref.Name]; !isExternal {
					return
				}
			}
		}

		r := Reference{
			Name:       ref.Name,
			AppendCode: ref.AppendCode,
			AppendDoc:  ref.AppendDoc,
		}

		// Check target symbols first, then external (host) symbols.
		// Search both target and reference paths for code/doc resolution.
		if kind, isTarget := c.targetSymbols[ref.Name]; isTarget {
			r.IsExternal = true
			r.Kind = kind
			r.Signature = c.targetSigs[ref.Name]
			allPaths := append(c.targetPaths, c.referencePaths...)
			if c.targetResolver != nil {
				if ref.AppendCode {
					for _, path := range allPaths {
						if code, ok := c.targetResolver.GetCode(path, ref.Name); ok {
							r.ResolvedCode = code
							break
						}
					}
				}
				if ref.AppendDoc {
					for _, path := range allPaths {
						if doc, ok := c.targetResolver.GetDoc(path, ref.Name); ok {
							r.ResolvedDoc = doc
							break
						}
					}
				}
			}
		} else if kind, isExt := c.symbols[ref.Name]; isExt {
			r.IsExternal = true
			r.Kind = kind
			// Resolve signature, code, doc, and generated status from the resolver.
			if c.resolver != nil {
				if sigs := c.resolver.Signatures(); sigs != nil {
					r.Signature = sigs[ref.Name]
				}
				r.IsGenerated = c.resolver.IsGenerated(ref.Name)
				if ref.AppendCode {
					if code, ok := c.resolver.GetCode(ref.Name); ok {
						r.ResolvedCode = code
					}
				}
				if ref.AppendDoc {
					if doc, ok := c.resolver.GetDoc(ref.Name); ok {
						r.ResolvedDoc = doc
					}
				}
			}
		}

		req.References = append(req.References, r)
	})

	return []Request{req}
}

// buildTask renders the signature + body text for a declaration.
func buildTask(decl ast.Declaration) string {
	var parts []string

	if sig := renderSignature(decl); sig != "" {
		parts = append(parts, sig)
	}

	if body := assembleBody(decl); body != "" {
		parts = append(parts, body)
	}

	return strings.Join(parts, "\n")
}

// requestType determines the request type from the declaration kind.
func requestType(decl ast.Declaration) RequestType {
	if ast.IsPlan(decl) {
		return PlannerAgent
	}
	return ExecutorAgent
}

// renderSignature returns a human-readable signature string for a declaration.
func renderSignature(decl ast.Declaration) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return renderFuncSignature(d)
	case *ast.StructDecl:
		return "struct " + d.Name
	case *ast.InterfaceDecl:
		return "interface " + d.Name
	case *ast.PlanDecl:
		return "plan " + d.Name
	}
	return ""
}

func renderFuncSignature(f *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if f.Parent != "" {
		b.WriteString(f.Parent + ".")
	}
	b.WriteString(f.Name)
	b.WriteString("(")
	for i, param := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(param.Name)
		if param.Type != "" {
			b.WriteString(" " + param.Type)
		}
	}
	b.WriteString(")")
	if f.ReturnType != "" {
		b.WriteString(" -> " + f.ReturnType)
	}
	return b.String()
}

// assembleBody renders body segments as text.
func assembleBody(decl ast.Declaration) string {
	var parts []string
	ast.WalkBodySegments(decl, func(seg ast.BodySegment) {
		switch s := seg.(type) {
		case *ast.TextSegment:
			text := strings.TrimSpace(s.Content)
			if text != "" {
				parts = append(parts, text)
			}
		case *ast.UseRefSegment:
			parts = append(parts, "[use "+s.Name+"]")
		case *ast.InjectRefSegment:
			parts = append(parts, "[inject "+s.Path+"]")
		}
	})
	return strings.Join(parts, "\n")
}
