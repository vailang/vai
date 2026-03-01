package render

import (
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// implStyle controls heading levels and formatting for different render modes.
type implStyle struct {
	titlePrefix string // e.g. "### Implementation Guide for: " or "# "
	codeHeading string // e.g. "#### Actual state..." or "## Actual state..."
	refHeading  string // e.g. "\n### Reference\n" or "\n## Reference\n"
	textPrefix  string // extra newline before text parts (ImplInjected adds "\n")
}

var (
	atomicStyle = implStyle{
		titlePrefix: "### Implementation Guide for: ",
		codeHeading: "#### Actual state of implementation\n",
		refHeading:  "\n### Reference\n",
	}
	injectedStyle = implStyle{
		titlePrefix: "# ",
		codeHeading: "## Actual state of implementation\n",
		refHeading:  "\n## Reference\n",
		textPrefix:  "\n",
	}
)

// ImplAtomic renders a single impl as a self-contained string.
// Each impl includes its resolved signature, body text, and a Reference
// section with all [use X] dependencies (signatures only, no code).
func ImplAtomic(
	impl *ast.ImplDecl,
	prompts map[string]*ast.PromptDecl,
	plans map[string]*ast.PlanDecl,
	target TargetInfo,
	baseDir string,
	planTargets ...string,
) string {
	file := implParentFile(impl, plans)
	return renderImpl(impl, file, prompts, target, baseDir, atomicStyle, planTargets...)
}

// ImplInjected renders a single impl for `inject plan.impl` usage.
// It shows: the impl name heading, the code block from the target file,
// body text, and a Reference section with all [use] refs as signatures.
func ImplInjected(
	impl *ast.ImplDecl,
	plan *ast.PlanDecl,
	prompts map[string]*ast.PromptDecl,
	target TargetInfo,
	baseDir string,
) string {
	file := implParentFile(impl, map[string]*ast.PlanDecl{plan.Name: plan})
	return renderImpl(impl, file, prompts, target, baseDir, injectedStyle, plan.Targets...)
}

// renderImpl is the shared rendering logic for both ImplAtomic and ImplInjected.
func renderImpl(
	impl *ast.ImplDecl,
	file *ast.File,
	prompts map[string]*ast.PromptDecl,
	target TargetInfo,
	baseDir string,
	style implStyle,
	planTargets ...string,
) string {
	symbols, sigs, allPaths := resolveAllTargets(file, target, baseDir)

	var buf strings.Builder
	buf.WriteString(style.titlePrefix + impl.Name + "\n")

	// Show existing code from the impl's [target] file.
	if implTarget := ImplTargetPath(impl, baseDir, planTargets...); implTarget != "" {
		if target != nil {
			if code, ok := target.GetCode(implTarget, impl.Name); ok {
				buf.WriteString(style.codeHeading)
				lang := LangTag(implTarget)
				buf.WriteString("```" + lang + "\n")
				buf.WriteString(code)
				if !strings.HasSuffix(code, "\n") {
					buf.WriteString("\n")
				}
				buf.WriteString("```\n")
			}
		}
	}

	// Render body text (excluding [use] refs which go into Reference section).
	textParts, refs := splitBodyTextAndRefs(impl.Body, prompts)

	if len(textParts) > 0 {
		buf.WriteString(style.textPrefix)
		buf.WriteString(strings.Join(textParts, "\n"))
		buf.WriteString("\n")
	}

	// Reference section with resolved [use] dependencies.
	// Structs/classes/interfaces/traits render as full code blocks; others as signatures.
	if len(refs) > 0 {
		buf.WriteString(style.refHeading)
		for _, ref := range refs {
			buf.WriteString(renderRefItem(ref, symbols, sigs, target, allPaths))
		}
	}

	return buf.String()
}

// ImplTargetPath extracts the [target "path"] from an impl's body segments.
// Returns the absolute path, or empty string if no target found.
// planTargets provides fallback targets when the impl has no explicit [target]
// (auto-inherit for single-target plans).
func ImplTargetPath(impl *ast.ImplDecl, baseDir string, planTargets ...string) string {
	for _, seg := range impl.Body {
		if tr, ok := seg.(*ast.TargetRefSegment); ok {
			return absPath(tr.Name, baseDir)
		}
	}
	// Auto-inherit: if plan has exactly one target and impl has none, use it.
	if len(planTargets) == 1 && planTargets[0] != "" {
		return absPath(planTargets[0], baseDir)
	}
	return ""
}

// splitBodyTextAndRefs separates body segments into text parts and [use] refs.
// [inject] refs are resolved inline into text parts.
func splitBodyTextAndRefs(
	body []ast.BodySegment,
	prompts map[string]*ast.PromptDecl,
) (textParts []string, refs []*ast.UseRefSegment) {
	for _, seg := range body {
		switch s := seg.(type) {
		case *ast.TextSegment:
			text := strings.TrimSpace(s.Content)
			if text != "" {
				textParts = append(textParts, text)
			}
		case *ast.UseRefSegment:
			refs = append(refs, s)
		case *ast.InjectRefSegment:
			if pd, found := prompts[s.Path]; found {
				text := BodyText(pd.Body)
				if text != "" {
					textParts = append(textParts, text)
				}
			}
		}
	}
	return
}

// renderRefItem renders a single [use] reference for the Reference section.
// Structs/classes/interfaces/traits are rendered as full code fences.
// Functions and other symbols are rendered as inline backtick signatures.
func renderRefItem(
	ref *ast.UseRefSegment,
	symbols map[string]ast.SymbolKind,
	sigs map[string]string,
	target TargetInfo,
	allPaths []string,
) string {
	// Structs/classes/interfaces/traits: try full code fence first.
	kind := symbols[ref.Name]
	switch kind {
	case ast.SymbolStruct, ast.SymbolClass, ast.SymbolInterface, ast.SymbolTrait:
		if target != nil {
			for _, path := range allPaths {
				if code, ok := target.GetCode(path, ref.Name); ok {
					return "```" + LangTag(path) + "\n" + code + "\n```\n"
				}
			}
		}
	}
	// All symbols fall back to signature, then bare name.
	if sig, ok := sigs[ref.Name]; ok {
		return "- `" + sig + "`\n"
	}
	return "- `" + ref.Name + "`\n"
}

// ImplLight renders an impl as a lightweight summary for the planner.
// No code block (the skeleton is already shown in Target File Status).
// All [use] references are rendered as signature-only (no code fences for types).
func ImplLight(
	impl *ast.ImplDecl,
	prompts map[string]*ast.PromptDecl,
	sigs map[string]string,
) string {
	var buf strings.Builder
	buf.WriteString("### " + impl.Name + "\n")

	textParts, refs := splitBodyTextAndRefs(impl.Body, prompts)

	if len(textParts) > 0 {
		buf.WriteString(strings.Join(textParts, "\n"))
		buf.WriteString("\n")
	}

	if len(refs) > 0 {
		buf.WriteString("\nDependencies:\n")
		for _, ref := range refs {
			if sig, ok := sigs[ref.Name]; ok {
				buf.WriteString("- `" + sig + "`\n")
			} else {
				buf.WriteString("- `" + ref.Name + "`\n")
			}
		}
	}

	return buf.String()
}

// implParentFile creates a minimal ast.File containing the plan declarations
// so that resolveAllTargets can find target/reference paths.
// This is needed because render functions receive plans as maps, not as a file.
func implParentFile(impl *ast.ImplDecl, plans map[string]*ast.PlanDecl) *ast.File {
	file := &ast.File{}
	for _, plan := range plans {
		file.Declarations = append(file.Declarations, plan)
	}
	return file
}
