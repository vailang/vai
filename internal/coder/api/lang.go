package api

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Language represents a supported host language.
type Language string

const (
	Go         Language = "go"
	Rust       Language = "rust"
	Python     Language = "python"
	TypeScript Language = "typescript"
	C          Language = "c"
)

// DetectLanguage returns the language for a file path based on extension.
func DetectLanguage(filePath string) (Language, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return Go, nil
	case ".ts", ".tsx":
		return TypeScript, nil
	case ".py":
		return Python, nil
	case ".rs":
		return Rust, nil
	case ".c", ".h":
		return C, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// IsTSX returns true if the file path ends with .tsx.
func IsTSX(filePath string) bool {
	return strings.ToLower(filepath.Ext(filePath)) == ".tsx"
}

// IsValid reports whether lang is a recognized language.
func IsValid(lang Language) bool {
	switch lang {
	case Go, Rust, Python, TypeScript, C:
		return true
	default:
		return false
	}
}

// CommentPrefix returns the line comment prefix for a language.
func CommentPrefix(lang Language) string {
	switch Normalize(lang) {
	case Python:
		return "#"
	default:
		return "//"
	}
}

// BlockCommentStart returns the block comment opening delimiter.
func BlockCommentStart(lang Language) string {
	switch Normalize(lang) {
	case Python:
		return `"""`
	default:
		return "/*"
	}
}

// BlockCommentEnd returns the block comment closing delimiter.
func BlockCommentEnd(lang Language) string {
	switch Normalize(lang) {
	case Python:
		return `"""`
	default:
		return "*/"
	}
}

// Normalize maps language aliases to their canonical form.
func Normalize(lang Language) Language {
	switch lang {
	case "py":
		return Python
	case "ts":
		return TypeScript
	case "rs":
		return Rust
	default:
		return lang
	}
}
