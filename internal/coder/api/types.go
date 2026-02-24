package api

// SymbolKind represents the kind of symbol extracted from host files.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolStruct    SymbolKind = "struct"
	SymbolClass     SymbolKind = "class"
	SymbolInterface SymbolKind = "interface"
	SymbolTrait     SymbolKind = "trait"
	SymbolConst     SymbolKind = "const"
	SymbolEnum      SymbolKind = "enum"
)

// Symbol represents a top-level declaration extracted from a host language file.
type Symbol struct {
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	Signature string     `json:"signature"`
	Doc       string     `json:"doc,omitempty"`
	Methods   []Method   `json:"methods,omitempty"`
	StartByte int        `json:"start_byte"`
	EndByte   int        `json:"end_byte"`
}

// Method represents a method within a struct, class, interface, or trait.
type Method struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Doc       string `json:"doc,omitempty"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
}

// ImportZone describes the byte range and content of existing imports in a host file.
type ImportZone struct {
	StartByte int
	EndByte   int
	Existing  []string
}

// ResolvedSymbol is the coder's response when resolving a symbol by name.
// Contains all extracted information: kind, signature, full code block, and documentation.
type ResolvedSymbol struct {
	Kind      string // "function", "struct", "interface", "enum", "trait"
	Signature string // rendered signature, e.g. "int my_func(int a, int b)"
	Code      string // full code block from source
	Doc       string // documentation comment
	StartByte int
	EndByte   int
}
