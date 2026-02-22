package render

import (
	"path/filepath"
	"strings"
)

// LangTag returns the code fence language tag for a file path.
func LangTag(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".c", ".h":
		return "c"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	default:
		return ""
	}
}
