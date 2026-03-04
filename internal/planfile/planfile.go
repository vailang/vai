package planfile

import "strings"

// FindPlanStart finds the byte offset of "plan <name>" followed by '{' in source.
func FindPlanStart(source, name string) int {
	target := "plan " + name
	idx := 0
	for {
		pos := strings.Index(source[idx:], target)
		if pos < 0 {
			return -1
		}
		pos += idx

		// Must be at start of line or start of file.
		if pos > 0 && source[pos-1] != '\n' && source[pos-1] != '\r' {
			idx = pos + len(target)
			continue
		}

		// Next non-space after name must be '{'.
		rest := strings.TrimSpace(source[pos+len(target):])
		if len(rest) > 0 && rest[0] == '{' {
			return pos
		}

		idx = pos + len(target)
	}
}

// FindMatchingBrace finds the matching closing brace starting from planStart.
func FindMatchingBrace(source string, planStart int) int {
	braceStart := strings.Index(source[planStart:], "{")
	if braceStart < 0 {
		return -1
	}
	braceStart += planStart

	depth := 0
	for i := braceStart; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
