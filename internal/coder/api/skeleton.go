package api

import (
	"sort"
	"strings"
)

// BodyReplacement describes a byte range in source to replace with a stub string.
type BodyReplacement struct {
	StartByte int
	EndByte   int
	Stub      string
}

// ApplyReplacements builds a new string from source, replacing the given byte ranges with stubs.
// Replacements are sorted by StartByte; non-overlapping is assumed.
func ApplyReplacements(source []byte, replacements []BodyReplacement) string {
	if len(replacements) == 0 {
		return string(source)
	}

	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].StartByte < replacements[j].StartByte
	})

	var buf strings.Builder
	pos := 0
	for _, r := range replacements {
		if r.StartByte > pos {
			buf.Write(source[pos:r.StartByte])
		}
		buf.WriteString(r.Stub)
		pos = r.EndByte
	}
	if pos < len(source) {
		buf.Write(source[pos:])
	}
	return buf.String()
}
