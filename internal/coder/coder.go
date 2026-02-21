package coder

import (
	"fmt"

	"github.com/vailang/vai/internal/coder/api"
	"github.com/vailang/vai/internal/coder/clang"
	"github.com/vailang/vai/internal/coder/golang"
	"github.com/vailang/vai/internal/coder/python"
	"github.com/vailang/vai/internal/coder/rust"
	"github.com/vailang/vai/internal/coder/typescript"
)

// Re-export types from api so consumers only need to import coder.
type (
	Language       = api.Language
	Symbol         = api.Symbol
	Method         = api.Method
	SymbolKind     = api.SymbolKind
	ImportZone     = api.ImportZone
	LangQuery      = api.LangQuery
	HeaderInfo     = api.HeaderInfo
	ResolvedSymbol = api.ResolvedSymbol
)

// Re-export constants.
const (
	Go         = api.Go
	Rust       = api.Rust
	Python     = api.Python
	TypeScript = api.TypeScript
	C          = api.C

	SymbolFunction  = api.SymbolFunction
	SymbolStruct    = api.SymbolStruct
	SymbolClass     = api.SymbolClass
	SymbolInterface = api.SymbolInterface
	SymbolTrait     = api.SymbolTrait
)

// Re-export functions from api.
var (
	DetectLanguage    = api.DetectLanguage
	IsTSX             = api.IsTSX
	IsValid           = api.IsValid
	CommentPrefix     = api.CommentPrefix
	BlockCommentStart = api.BlockCommentStart
	BlockCommentEnd   = api.BlockCommentEnd
	GetCode           = api.GetCode
	GetDoc            = api.GetDoc
	ComputeHash       = api.ComputeHash
	LoadCache         = api.LoadCache
	SaveCache         = api.SaveCache
	CacheDir          = api.CacheDir
)

// OverrideDir allows tests to override the cache directory.
// Re-exported from api — set api.OverrideDir directly for tests.

// ReaderCode groups read-only operations: parse, extract, analyze.
type ReaderCode interface {
	Load(source []byte) error
	Resolve(name string) (api.ResolvedSymbol, bool)
	ParseSymbols(source []byte) ([]Symbol, error)
	FindImportZone(source []byte) (*ImportZone, error)
	StripLeadingImports(code string) (string, []string)
	IsImportLine(line string) bool
	Language() Language
	Close()
}

// WriterCode groups write/modify operations: insert, build.
type WriterCode interface {
	InsertImports(targetPath string, newImports []string) error
	BuildImportBlock(imports []string, comment string) string
	Language() Language
	Close()
}

// Coder implements both ReaderCode and WriterCode.
// All tree-sitter usage is encapsulated in the LangQuery sub-packages.
type Coder struct {
	lang     Language
	filePath string
	query    LangQuery

	// Loaded state — populated by Load().
	source  []byte
	symbols []Symbol
	loaded  bool
}

// New creates a Coder for the given language and file path.
func New(lang Language, filePath string) (*Coder, error) {
	q, err := newQuery(lang, filePath)
	if err != nil {
		return nil, err
	}
	return &Coder{lang: lang, filePath: filePath, query: q}, nil
}

// Close releases tree-sitter resources.
func (c *Coder) Close() {
	if c.query != nil {
		c.query.Close()
	}
}

// Language returns the language this coder was created for.
func (c *Coder) Language() Language { return c.lang }

func newQuery(lang Language, filePath string) (LangQuery, error) {
	switch lang {
	case Go:
		return golang.New()
	case TypeScript:
		return typescript.New(IsTSX(filePath))
	case Python:
		return python.New()
	case Rust:
		return rust.New()
	case C:
		return clang.New()
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}
