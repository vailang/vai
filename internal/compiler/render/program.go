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

	// 1. Render inject/inspect declarations.
	hasPlanInject := false
	for _, decl := range file.Declarations {
		switch d := decl.(type) {
		case *ast.InjectDecl:
			// Check for plan.impl qualified inject.
			if strings.Contains(d.Name, ".") {
				parts := strings.SplitN(d.Name, ".", 2)
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
			if pd, found := prompts[d.Name]; found {
				text := BodyResolved(pd.Body, prompts, plans, target, baseDir)
				if text != "" {
					buf.WriteString(text)
					buf.WriteString("\n\n")
				}
			} else if plan, found := plans[d.Name]; found {
				hasPlanInject = true
				buf.WriteString(Plan(plan, prompts, plans, target, globalConstraints, baseDir))
				buf.WriteString("\n")
			}

		case *ast.InspectDecl:
			if text := inspectPlan(d.Name, plans); text != "" {
				buf.WriteString(text)
				buf.WriteString("\n")
			}
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

// Exec resolves inject/inspect declarations by looking up the referenced prompts
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
		switch d := decl.(type) {
		case *ast.InjectDecl:
			// Check for plan.impl qualified inject.
			if strings.Contains(d.Name, ".") {
				splitParts := strings.SplitN(d.Name, ".", 2)
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
			if pd, found := prompts[d.Name]; found {
				parts = append(parts, BodyText(pd.Body))
			} else if plan, found := plans[d.Name]; found {
				globalConstraints := collectGlobalConstraints(file)
				parts = append(parts, Plan(plan, prompts, plans, target, globalConstraints, baseDir))
			}

		case *ast.InspectDecl:
			if text := inspectPlan(d.Name, plans); text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, "\n")
}

// inspectPlan returns raw unrendered plan content for an inspect expression.
// Supported forms:
//   - "planName"          → spec text + impl name list
//   - "planName.spec"     → spec text only
//   - "planName.implName" → impl body text only
func inspectPlan(name string, plans map[string]*ast.PlanDecl) string {
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		plan, found := plans[parts[0]]
		if !found {
			return ""
		}
		if parts[1] == "spec" {
			return inspectSpec(plan)
		}
		// Look up a specific impl.
		for _, impl := range plan.Impls {
			if impl.Name == parts[1] {
				return BodyText(impl.Body)
			}
		}
		return ""
	}

	// Bare plan name: spec + impl list.
	plan, found := plans[name]
	if !found {
		return ""
	}
	var buf strings.Builder
	spec := inspectSpec(plan)
	if spec != "" {
		buf.WriteString(spec)
	}
	if len(plan.Impls) > 0 {
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		names := make([]string, len(plan.Impls))
		for i, impl := range plan.Impls {
			names[i] = impl.Name
		}
		buf.WriteString("impls: ")
		buf.WriteString(strings.Join(names, ", "))
	}
	return buf.String()
}

// inspectSpec returns the raw text of all spec blocks in a plan.
func inspectSpec(plan *ast.PlanDecl) string {
	var parts []string
	for _, spec := range plan.Specs {
		text := BodyText(spec.Body)
		if text != "" {
			parts = append(parts, text)
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
