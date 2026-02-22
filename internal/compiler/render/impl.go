package render

import (
	"path/filepath"
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
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
) string {
	_, sigs, _ := resolveAllTargets(implParentFile(impl, plans), target, baseDir)

	var buf strings.Builder
	buf.WriteString("### Implementation Guide for: " + impl.Name + "\n")

	// Show existing code from the impl's [target] file.
	if implTarget := ImplTargetPath(impl, baseDir); implTarget != "" {
		if target != nil {
			if code, ok := target.GetCode(implTarget, impl.Name); ok {
				buf.WriteString("#### Actual state of implementation\n")
				buf.WriteString("```" + LangTag(implTarget) + "\n" + code + "\n```\n")
			}
		}
	}

	// Render body text (excluding [use] refs which go into Reference section).
	textParts, refs := splitBodyTextAndRefs(impl.Body, prompts)

	if len(textParts) > 0 {
		buf.WriteString(strings.Join(textParts, "\n"))
		buf.WriteString("\n")
	}

	// Reference section with resolved [use] dependencies (signatures only).
	if len(refs) > 0 {
		buf.WriteString("\n### Reference\n")
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
	_, sigs, _ := resolveAllTargets(implParentFile(impl, map[string]*ast.PlanDecl{plan.Name: plan}), target, baseDir)

	var buf strings.Builder
	buf.WriteString("# " + impl.Name + "\n")

	// Show the code block from the impl's [target] file.
	if implTarget := ImplTargetPath(impl, baseDir); implTarget != "" {
		if target != nil {
			if code, ok := target.GetCode(implTarget, impl.Name); ok {
				buf.WriteString("## Actual state of implementation\n")
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

	// Render body text.
	textParts, refs := splitBodyTextAndRefs(impl.Body, prompts)

	if len(textParts) > 0 {
		buf.WriteString("\n")
		buf.WriteString(strings.Join(textParts, "\n"))
		buf.WriteString("\n")
	}

	// Reference section — signatures only, no code.
	if len(refs) > 0 {
		buf.WriteString("\n## Reference\n")
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

// ImplTargetPath extracts the [target "path"] from an impl's body segments.
// Returns the absolute path, or empty string if no target found.
func ImplTargetPath(impl *ast.ImplDecl, baseDir string) string {
	for _, seg := range impl.Body {
		if tr, ok := seg.(*ast.TargetRefSegment); ok {
			absPath := tr.Name
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(baseDir, absPath)
			}
			return absPath
		}
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
