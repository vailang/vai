package coder

import "strings"

// stripOutputZones removes content between vai:output_start and vai:output_end
// markers, replacing it with whitespace of the same length to preserve byte offsets.
// This ensures tree-sitter only sees host file imports, not generated code.
func stripOutputZones(source []byte) []byte {
	s := string(source)
	result := make([]byte, len(source))
	copy(result, source)

	for {
		startIdx := strings.Index(s, "vai:output_start")
		if startIdx < 0 {
			break
		}
		// Find the line start.
		lineStart := strings.LastIndex(s[:startIdx], "\n")
		if lineStart < 0 {
			lineStart = 0
		} else {
			lineStart++ // skip the newline itself
		}

		endIdx := strings.Index(s[startIdx:], "vai:output_end")
		if endIdx < 0 {
			break
		}
		endIdx += startIdx
		// Find the line end after vai:output_end.
		lineEnd := strings.Index(s[endIdx:], "\n")
		if lineEnd < 0 {
			lineEnd = len(s)
		} else {
			lineEnd += endIdx + 1
		}

		// Replace the zone content with spaces (preserve byte positions).
		for i := lineStart; i < lineEnd && i < len(result); i++ {
			if result[i] != '\n' {
				result[i] = ' '
			}
		}

		// Advance past this zone.
		s = s[:lineStart] + string(result[lineStart:lineEnd]) + s[lineEnd:]
	}

	return result
}

// deduplicateImports returns only the imports from newOnes that are not already
// present in existing. Comparison is exact string match after trimming.
func deduplicateImports(existing, newOnes []string) []string {
	set := map[string]bool{}
	for _, e := range existing {
		set[strings.TrimSpace(e)] = true
	}
	var result []string
	for _, n := range newOnes {
		if !set[strings.TrimSpace(n)] {
			result = append(result, n)
		}
	}
	return result
}
