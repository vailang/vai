package runner

import (
	"fmt"
	"os"
	"strings"

	"github.com/vailang/vai/internal/planfile"
)

// saveSkeleton writes the architect's skeleton result back to the original plan file.
// It preserves original target/spec/constraint blocks and adds the architect's impls.
func (r *Runner) saveSkeleton(skeletons []planSkeletonResult) error {
	for _, skel := range skeletons {
		if skel.sourcePath == "" {
			continue
		}

		content, err := os.ReadFile(skel.sourcePath)
		if err != nil {
			return fmt.Errorf("reading plan file %s: %w", skel.sourcePath, err)
		}

		updated, err := rewritePlanBlock(string(content), skel.planName, skel)
		if err != nil {
			return fmt.Errorf("rewriting plan %s: %w", skel.planName, err)
		}

		if err := os.WriteFile(skel.sourcePath, []byte(updated), 0644); err != nil {
			return fmt.Errorf("writing plan file %s: %w", skel.sourcePath, err)
		}

		r.emit(Event{Kind: EventInfo, Step: "save", Name: skel.sourcePath, Message: fmt.Sprintf("saved skeleton to %s", skel.sourcePath)})
	}
	return nil
}

// rewritePlanBlock finds the plan block by name and replaces it with updated content.
// Preserves original target, spec, and constraint blocks; replaces impls with skeleton output.
func rewritePlanBlock(source, planName string, skel planSkeletonResult) (string, error) {
	planStart := planfile.FindPlanStart(source, planName)
	if planStart < 0 {
		return "", fmt.Errorf("plan %q not found in source", planName)
	}

	planEnd := planfile.FindMatchingBrace(source, planStart)
	if planEnd < 0 {
		return "", fmt.Errorf("no matching brace for plan %q", planName)
	}

	// Extract the original plan body to preserve target/spec/constraint blocks.
	braceStart := strings.Index(source[planStart:], "{")
	if braceStart < 0 {
		return "", fmt.Errorf("no opening brace for plan %q", planName)
	}
	braceStart += planStart
	originalBody := source[braceStart+1 : planEnd]

	// Extract preserved blocks (target, spec, constraint, reference).
	preserved := extractPreservedBlocks(originalBody)

	// Build the new plan block.
	newBlock := buildPlanBlock(planName, preserved, skel)

	var b strings.Builder
	b.WriteString(source[:planStart])
	b.WriteString(newBlock)
	b.WriteString(source[planEnd+1:]) // skip the closing brace
	return b.String(), nil
}

// extractPreservedBlocks extracts target, spec, constraint, and reference lines from a plan body.
// Returns them as a single string ready to be re-inserted.
func extractPreservedBlocks(body string) string {
	var preserved strings.Builder
	lines := strings.Split(body, "\n")

	depth := 0
	inPreserved := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track braces for nested blocks.
		if inPreserved {
			preserved.WriteString(line)
			preserved.WriteByte('\n')
			for _, ch := range line {
				switch ch {
				case '{':
					depth++
				case '}':
					depth--
				}
			}
			if depth == 0 {
				inPreserved = false
			}
			continue
		}

		// Check if this line starts a preserved block.
		if strings.HasPrefix(trimmed, "target ") ||
			strings.HasPrefix(trimmed, "reference ") {
			preserved.WriteString(line)
			preserved.WriteByte('\n')
			continue
		}

		if strings.HasPrefix(trimmed, "spec ") || trimmed == "spec{" ||
			strings.HasPrefix(trimmed, "constraint ") || trimmed == "constraint{" {
			preserved.WriteString(line)
			preserved.WriteByte('\n')
			for _, ch := range line {
				switch ch {
				case '{':
					depth++
				case '}':
					depth--
				}
			}
			if depth > 0 {
				inPreserved = true
			}
			continue
		}
	}

	return preserved.String()
}

// buildPlanBlock constructs the new plan block with preserved blocks + architect's impls.
func buildPlanBlock(name, preserved string, skel planSkeletonResult) string {
	var b strings.Builder

	b.WriteString("plan ")
	b.WriteString(name)
	b.WriteString(" {\n")

	// Write preserved blocks (target, spec, constraint).
	if preserved != "" {
		b.WriteString(preserved)
	}

	// Write impls from the skeleton.
	for _, impl := range skel.skeleton.Impls {
		b.WriteString("\n    impl ")
		b.WriteString(impl.Name)
		b.WriteString(" {\n")
		// Emit [target] directive — use per-impl target or fall back to plan's first target.
		target := impl.Target
		if target == "" && len(skel.targetPaths) > 0 {
			target = skel.targetPaths[0]
		}
		if target != "" {
			b.WriteString("        [target \"")
			b.WriteString(target)
			b.WriteString("\"]\n")
		}
		if impl.Instruction != "" {
			for _, line := range strings.Split(impl.Instruction, "\n") {
				b.WriteString("        ")
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		for _, u := range impl.Uses {
			b.WriteString("        [use ")
			b.WriteString(u)
			b.WriteString("]\n")
		}
		b.WriteString("    }\n")
	}

	b.WriteString("}\n")
	return b.String()
}
