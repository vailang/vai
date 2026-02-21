package api

// LangQuery encapsulates all tree-sitter operations for a language.
// Implemented by per-language sub-packages. The parent coder package
// never imports tree-sitter directly.
type LangQuery interface {
	// ReadSymbols extracts all top-level symbols from source code.
	ReadSymbols(source []byte) ([]Symbol, error)
	// ReadImportZone finds the import block's byte range and content.
	ReadImportZone(source []byte) (*ImportZone, error)
	// ReadSkeleton returns the file source with implementation bodies replaced by stubs.
	ReadSkeleton(source []byte) (string, error)
	// Close releases tree-sitter resources.
	Close()
}
