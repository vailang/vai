package render

import (
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// BodyText concatenates text segments only, without symbol resolution.
func BodyText(segments []ast.BodySegment) string {
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

// BodyResolved renders body segments with full symbol resolution.
// Target and reference files are loaded on demand; [use X] references
// are resolved to code fences (structs) or inline signatures (functions).
func BodyResolved(
	segments []ast.BodySegment,
	prompts map[string]*ast.PromptDecl,
	plans map[string]*ast.PlanDecl,
	target TargetInfo,
	baseDir string,
) string {
	// Collect target and reference files referenced in this body.
	targetPaths, refPaths := collectBodyFilePaths(segments, plans, baseDir)
	allPaths := append(targetPaths, refPaths...)

	// Build symbol lookup from those files.
	symbols := map[string]ast.SymbolKind{}
	sigs := map[string]string{}
	if target != nil {
		for _, path := range allPaths {
			s, si, err := target.ResolveTarget(path)
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
			parts = append(parts, UseRef(s, symbols, sigs, target, allPaths))
		case *ast.TargetRefSegment:
			// Context only — no output.
		case *ast.ReferenceRefSegment:
			// Context only — symbols loaded above.
		case *ast.InjectRefSegment:
			if pd, found := prompts[s.Path]; found {
				text := BodyText(pd.Body)
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

// collectBodyFilePaths finds all [target "path"] and [reference "path"|plan_name]
// directives in body segments and resolves them to absolute paths.
func collectBodyFilePaths(
	segments []ast.BodySegment,
	plans map[string]*ast.PlanDecl,
	baseDir string,
) (targetPaths, refPaths []string) {
	seen := map[string]bool{}
	add := func(p string) string {
		ap := absPath(p, baseDir)
		if seen[ap] {
			return ""
		}
		seen[ap] = true
		return ap
	}
	for _, seg := range segments {
		switch s := seg.(type) {
		case *ast.TargetRefSegment:
			if ap := add(s.Name); ap != "" {
				targetPaths = append(targetPaths, ap)
			}
		case *ast.ReferenceRefSegment:
			// Check if this is a plan name reference — import its targets.
			if plan, ok := plans[s.Name]; ok {
				for _, t := range plan.Targets {
					if ap := add(t); ap != "" {
						refPaths = append(refPaths, ap)
					}
				}
				continue
			}
			if ap := add(s.Name); ap != "" {
				refPaths = append(refPaths, ap)
			}
		}
	}
	return
}
