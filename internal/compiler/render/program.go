package render

import (
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// Render produces the fully resolved structured text of a compiled program.
// It renders inject declarations (prompts, plans, plan.impl), then global
// constraints if no plan was injected.
func Render(
	file *ast.File,
	prompts map[string]*ast.PromptDecl,
	plans map[string]*ast.PlanDecl,
	target TargetInfo,
	baseDir string,
) string {
	if file == nil {
		return ""
	}

	var buf strings.Builder
	globalConstraints := collectGlobalConstraints(file)

	// 1. Render inject declarations — prompts, plans, or plan.impl.
	hasPlanInject := false
	for _, decl := range file.Declarations {
		inj, ok := decl.(*ast.InjectDecl)
		if !ok {
			continue
		}
		// Check for plan.impl qualified inject.
		if strings.Contains(inj.Name, ".") {
			parts := strings.SplitN(inj.Name, ".", 2)
			if plan, found := plans[parts[0]]; found {
				for _, impl := range plan.Impls {
					if impl.Name == parts[1] {
						buf.WriteString(ImplInjected(impl, plan, prompts, target, baseDir))
						buf.WriteString("\n")
						break
					}
				}
				continue
			}
		}
		if pd, found := prompts[inj.Name]; found {
			text := BodyResolved(pd.Body, prompts, plans, target, baseDir)
			if text != "" {
				buf.WriteString(text)
				buf.WriteString("\n\n")
			}
		} else if plan, found := plans[inj.Name]; found {
			hasPlanInject = true
			buf.WriteString(Plan(plan, prompts, plans, target, globalConstraints, baseDir))
			buf.WriteString("\n")
		}
	}

	// 2. Render constraints (only when no plan was injected, since plan rendering includes them).
	if !hasPlanInject && len(globalConstraints) > 0 {
		buf.WriteString("## Global Constraint\n")
		for _, c := range collectAllConstraints(file) {
			if c.Name != "" {
				buf.WriteString("**" + c.Name + "**")
			} else {
				buf.WriteString("-")
			}
			body := BodyResolved(c.Body, prompts, plans, target, baseDir)
			if body != "" {
				buf.WriteString(" " + body)
			}
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// Exec resolves inject declarations by looking up the referenced prompts
// or plans and assembling their body text. Used for execution output.
func Exec(
	file *ast.File,
	prompts map[string]*ast.PromptDecl,
	plans map[string]*ast.PlanDecl,
	target TargetInfo,
	baseDir string,
) string {
	if file == nil {
		return ""
	}

	var parts []string
	for _, decl := range file.Declarations {
		inj, ok := decl.(*ast.InjectDecl)
		if !ok {
			continue
		}
		// Check for plan.impl qualified inject.
		if strings.Contains(inj.Name, ".") {
			splitParts := strings.SplitN(inj.Name, ".", 2)
			if plan, found := plans[splitParts[0]]; found {
				for _, impl := range plan.Impls {
					if impl.Name == splitParts[1] {
						parts = append(parts, ImplInjected(impl, plan, prompts, target, baseDir))
						break
					}
				}
				continue
			}
		}
		if pd, found := prompts[inj.Name]; found {
			parts = append(parts, BodyText(pd.Body))
		} else if plan, found := plans[inj.Name]; found {
			globalConstraints := collectGlobalConstraints(file)
			parts = append(parts, Plan(plan, prompts, plans, target, globalConstraints, baseDir))
		}
	}

	return strings.Join(parts, "\n")
}

// collectAllConstraints returns top-level and plan-nested constraints.
func collectAllConstraints(file *ast.File) []*ast.ConstraintDecl {
	if file == nil {
		return nil
	}
	var constraints []*ast.ConstraintDecl
	for _, decl := range file.Declarations {
		switch d := decl.(type) {
		case *ast.ConstraintDecl:
			constraints = append(constraints, d)
		case *ast.PlanDecl:
			constraints = append(constraints, d.Constraints...)
		}
	}
	return constraints
}
