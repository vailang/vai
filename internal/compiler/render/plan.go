package render

import (
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// Plan renders a plan as structured, self-contained markdown output.
// Sections: Specification, Target File Status, Implementation Order,
// Global Constraints, Plan Constraints.
func Plan(
	plan *ast.PlanDecl,
	prompts map[string]*ast.PromptDecl,
	plans map[string]*ast.PlanDecl,
	target TargetInfo,
	globalConstraints []*ast.ConstraintDecl,
	baseDir string,
) string {
	var buf strings.Builder

	// # Plan Name
	buf.WriteString("# " + plan.Name + "\n\n")

	// ## Specification
	if len(plan.Specs) > 0 {
		buf.WriteString("## Specification\n")
		for _, spec := range plan.Specs {
			text := BodyResolved(spec.Body, prompts, plans, target, baseDir)
			if text != "" {
				buf.WriteString(text + "\n")
			}
		}
		buf.WriteString("\n---\n\n")
	}

	// ## Target File Status
	if len(plan.Targets) > 0 {
		buf.WriteString("## Target File Status\n")
		for _, t := range plan.Targets {
			ap := absPath(t, baseDir)
			// Try skeleton view first — full file structure with empty bodies.
			if target != nil {
				if skeleton, ok := target.GetSkeleton(ap); ok {
					lang := LangTag(ap)
					buf.WriteString("### " + t + "\n")
					buf.WriteString("```" + lang + "\n")
					buf.WriteString(skeleton)
					if !strings.HasSuffix(skeleton, "\n") {
						buf.WriteString("\n")
					}
					buf.WriteString("```\n\n")
				} else {
					// Fallback to symbol listing if skeleton fails.
					symbols, sigs, err := target.ResolveTarget(ap)
					if err != nil {
						continue
					}
					buf.WriteString("### " + t + "\n")
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
		}
		buf.WriteString("---\n\n")
	}

	// ## Reference Files — raw content of non-code reference files (e.g. go.mod, Cargo.toml).
	if len(plan.References) > 0 && target != nil {
		var refBuf strings.Builder
		for _, ref := range plan.References {
			ap := absPath(ref, baseDir)
			if content, ok := target.GetRawContent(ap); ok {
				if refBuf.Len() == 0 {
					refBuf.WriteString("## Reference Files\n")
				}
				refBuf.WriteString("### " + ref + "\n")
				refBuf.WriteString("```" + ExtTag(ref) + "\n")
				refBuf.WriteString(content)
				if !strings.HasSuffix(content, "\n") {
					refBuf.WriteString("\n")
				}
				refBuf.WriteString("```\n\n")
			}
		}
		if refBuf.Len() > 0 {
			buf.WriteString(refBuf.String())
			buf.WriteString("---\n\n")
		}
	}

	// ## Implementation Order — each impl is atomic.
	if len(plan.Impls) > 0 {
		buf.WriteString("## Implementation Order\n")
		for _, impl := range plan.Impls {
			if len(plan.Targets) == 0 {
				buf.WriteString("### impl " + impl.Name + " (target not used)\n")
			} else {
				buf.WriteString(ImplAtomic(impl, prompts, plans, target, baseDir, plan.Targets...))
			}
			buf.WriteString("\n")
		}
		buf.WriteString("---\n\n")
	}

	// ## Global Constraints
	if len(globalConstraints) > 0 {
		buf.WriteString("## Global Constraints\n")
		for _, c := range globalConstraints {
			Constraint(&buf, c, prompts, target, baseDir)
		}
		buf.WriteString("\n---\n\n")
	}

	// ## Plan Constraints
	if len(plan.Constraints) > 0 {
		buf.WriteString("## Plan Constraints\n")
		for _, c := range plan.Constraints {
			Constraint(&buf, c, prompts, target, baseDir)
		}
	}

	return buf.String()
}

// Constraint renders a single constraint entry.
// Named constraints use a #### heading; anonymous ones render as list items.
func Constraint(
	buf *strings.Builder,
	c *ast.ConstraintDecl,
	prompts map[string]*ast.PromptDecl,
	target TargetInfo,
	baseDir string,
) {
	body := BodyResolved(c.Body, prompts, nil, target, baseDir)
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

// collectGlobalConstraints returns only top-level constraints (not inside plans).
func collectGlobalConstraints(file *ast.File) []*ast.ConstraintDecl {
	if file == nil {
		return nil
	}
	var constraints []*ast.ConstraintDecl
	for _, decl := range file.Declarations {
		if c, ok := decl.(*ast.ConstraintDecl); ok {
			constraints = append(constraints, c)
		}
	}
	return constraints
}
