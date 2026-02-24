package runner

import (
	"fmt"
	"strings"

	"github.com/vailang/vai/internal/coder/api"
	"github.com/vailang/vai/internal/runner/tools"
)

// diff applies the architect's skeleton declarations and imports to in-memory target files.
func (r *Runner) diff(skeletons []planSkeletonResult) error {
	for _, skel := range skeletons {
		if len(skel.targetPaths) == 0 {
			continue
		}

		primaryTarget := skel.targetPaths[0]
		absTarget := r.resolvePath(primaryTarget)

		// Apply imports first (architect's responsibility).
		if len(skel.skeleton.Imports) > 0 {
			if err := r.fm.Load(absTarget); err != nil {
				return fmt.Errorf("loading %s: %w", primaryTarget, err)
			}
			if err := r.fm.ModifyFile(absTarget, func(content []byte) ([]byte, error) {
				return r.insertImportsInMemory(content, absTarget, skel.skeleton.Imports)
			}); err != nil {
				return fmt.Errorf("applying imports to %s: %w", primaryTarget, err)
			}
		}

		// Apply declarations.
		for _, decl := range skel.skeleton.Declarations {
			if decl.Action == "keep" {
				continue
			}

			targetPath := primaryTarget
			if decl.Target != "" {
				targetPath = decl.Target
			}
			absPath := r.resolvePath(targetPath)

			if err := r.fm.Load(absPath); err != nil {
				return fmt.Errorf("loading %s: %w", targetPath, err)
			}

			if err := r.applyDecl(absPath, decl); err != nil {
				return fmt.Errorf("applying %s %s to %s: %w", decl.Action, decl.Name, targetPath, err)
			}
		}
	}
	return nil
}

// applyDecl applies a single skeleton declaration to an in-memory file.
func (r *Runner) applyDecl(absPath string, decl SkeletonDecl) error {
	return r.fm.ModifyFile(absPath, func(content []byte) ([]byte, error) {
		switch decl.Action {
		case "add":
			return r.declAdd(content, absPath, decl)
		case "modify":
			return r.declModify(content, absPath, decl)
		case "remove":
			return r.declRemove(content, absPath, decl)
		default:
			return content, nil
		}
	})
}

// declAdd appends a new declaration to the file.
func (r *Runner) declAdd(content []byte, absPath string, decl SkeletonDecl) ([]byte, error) {
	if decl.Code == "" {
		return content, nil
	}
	// Append to end of file with a blank line separator.
	var b strings.Builder
	b.Write(content)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(decl.Code)
	if !strings.HasSuffix(decl.Code, "\n") {
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// declModify replaces an existing declaration found by name using tree-sitter.
func (r *Runner) declModify(content []byte, absPath string, decl SkeletonDecl) ([]byte, error) {
	if decl.Code == "" {
		return content, nil
	}

	c, err := r.pool.Get(absPath, content)
	if err != nil {
		return nil, fmt.Errorf("loading coder for %s: %w", absPath, err)
	}

	resolved, ok := c.Resolve(decl.Name)
	if !ok {
		// Symbol not found — fall back to add.
		return r.declAdd(content, absPath, decl)
	}

	replacement := api.BodyReplacement{
		StartByte: resolved.StartByte,
		EndByte:   resolved.EndByte,
		Stub:      decl.Code,
	}
	return []byte(api.ApplyReplacements(content, []api.BodyReplacement{replacement})), nil
}

// declRemove removes a declaration found by name using tree-sitter.
func (r *Runner) declRemove(content []byte, absPath string, decl SkeletonDecl) ([]byte, error) {
	c, err := r.pool.Get(absPath, content)
	if err != nil {
		return nil, fmt.Errorf("loading coder for %s: %w", absPath, err)
	}

	resolved, ok := c.Resolve(decl.Name)
	if !ok {
		// Symbol not found — nothing to remove.
		return content, nil
	}

	// Remove the byte range.
	replacement := api.BodyReplacement{
		StartByte: resolved.StartByte,
		EndByte:   resolved.EndByte,
		Stub:      "",
	}
	return []byte(api.ApplyReplacements(content, []api.BodyReplacement{replacement})), nil
}

// SkeletonDecl is re-exported from tools for internal use.
type SkeletonDecl = tools.SkeletonDecl
