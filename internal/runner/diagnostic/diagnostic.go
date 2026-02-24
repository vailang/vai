package diagnostic

// Diagnostic represents a single compiler error or warning.
type Diagnostic struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	Level   string `json:"level"` // "error", "warning"
}

// Parser converts raw compiler output into structured diagnostics.
type Parser interface {
	Parse(output []byte) ([]Diagnostic, error)
}

// ForLanguage returns the diagnostic parser for a language.
// Returns nil if the language has no structured output support.
func ForLanguage(lang string) Parser {
	switch lang {
	case "go":
		return &GoParser{}
	case "rust":
		return &RustParser{}
	case "python":
		return &PythonParser{}
	case "c":
		return &ClangParser{}
	case "typescript":
		return &TypeScriptParser{}
	default:
		return nil
	}
}
