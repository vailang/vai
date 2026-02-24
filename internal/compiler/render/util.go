package render

import (
	"path/filepath"
	"strings"
)

// LangTag returns the code fence language tag for a target language file.
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

// absPath resolves a possibly relative path against baseDir.
func absPath(p, baseDir string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

// ExtTag returns a code fence language tag for any file extension,
// including non-code config files (toml, json, yaml, etc.).
func ExtTag(path string) string {
	if tag := LangTag(path); tag != "" {
		return tag
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		return "toml"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml":
		return "xml"
	case ".md":
		return "markdown"
	case ".mod":
		return "go"
	default:
		return ""
	}
}
