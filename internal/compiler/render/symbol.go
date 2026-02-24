package render

import (
	"strings"

	"github.com/vailang/vai/internal/compiler/ast"
)

// UseRef resolves a [use X] reference to formatted output.
// Structs/classes render as code fences with full definition.
// Interfaces/traits render as code fences with documentation.
// Functions render as inline backtick signatures.
func UseRef(
	ref *ast.UseRefSegment,
	symbols map[string]ast.SymbolKind,
	sigs map[string]string,
	target TargetInfo,
	targetPaths []string,
) string {
	kind := symbols[ref.Name]

	switch kind {
	case ast.SymbolStruct, ast.SymbolClass:
		// Find the code from the target files.
		if target != nil {
			for _, path := range targetPaths {
				if code, ok := target.GetCode(path, ref.Name); ok {
					lang := LangTag(path)
					return "```" + lang + "\n" + code + "\n```"
				}
			}
		}
		// Fallback to signature.
		if sig, ok := sigs[ref.Name]; ok {
			return "`" + sig + "`"
		}

	case ast.SymbolInterface, ast.SymbolTrait:
		// Interfaces/traits: show full definition with comments.
		if target != nil {
			for _, path := range targetPaths {
				if code, ok := target.GetCode(path, ref.Name); ok {
					lang := LangTag(path)
					var result strings.Builder
					if doc, ok := target.GetDoc(path, ref.Name); ok && doc != "" {
						result.WriteString(doc + "\n")
					}
					result.WriteString("```" + lang + "\n" + code + "\n```")
					return result.String()
				}
			}
		}
		if sig, ok := sigs[ref.Name]; ok {
			return "`" + sig + "`"
		}

	default:
		// Functions and others → inline signature.
		if sig, ok := sigs[ref.Name]; ok {
			return "`" + sig + "`"
		}
	}

	// Unresolved reference — render as empty (warning already emitted by composer).
	return ""
}

// resolveAllTargets loads symbols from all known target and reference paths
// across all plans and prompts in the file.
func resolveAllTargets(
	file *ast.File,
	target TargetInfo,
	baseDir string,
) (map[string]ast.SymbolKind, map[string]string, []string) {
	symbols := map[string]ast.SymbolKind{}
	sigs := map[string]string{}
	if target == nil {
		return symbols, sigs, nil
	}

	var allPaths []string
	seen := map[string]bool{}

	addPaths := func(paths []string) {
		for _, p := range paths {
			abs := absPath(p, baseDir)
			if !seen[abs] {
				seen[abs] = true
				allPaths = append(allPaths, abs)
			}
		}
	}

	// Collect all target and reference paths from all plans and prompts.
	for _, decl := range file.Declarations {
		switch d := decl.(type) {
		case *ast.PlanDecl:
			addPaths(d.Targets)
			addPaths(d.References)
		case *ast.PromptDecl:
			addPaths(d.References)
		}
	}

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

	return symbols, sigs, allPaths
}
