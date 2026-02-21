package ast

// ---------------------------------------------------------------------------
// BodyKind — classifies how a declaration body should be processed
// ---------------------------------------------------------------------------

type BodyKind int

const (
	BodyNone    BodyKind = iota // signature only, no body
	BodyTask                    // LLM task body (free text + references)
	BodyContext                 // single code fence → direct output
)

// ---------------------------------------------------------------------------
// SymbolKind — kind of symbol (mirrors coder/api but independent)
// ---------------------------------------------------------------------------

type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolStruct    SymbolKind = "struct"
	SymbolClass     SymbolKind = "class"
	SymbolInterface SymbolKind = "interface"
	SymbolTrait     SymbolKind = "trait"
)

// ---------------------------------------------------------------------------
// Param, TypeAlias — supporting types
// ---------------------------------------------------------------------------

// Param represents a function parameter.
type Param struct {
	Name string
	Type string
}

// TypeAlias represents a type alias inside a struct or interface.
type TypeAlias struct {
	Name string
	Type string
}

// ---------------------------------------------------------------------------
// Directive — metadata annotations on declarations
// ---------------------------------------------------------------------------

type DirectiveKind int

// Directive is a metadata annotation attached to a declaration.
type Directive struct {
	Kind  DirectiveKind
	Value string
}

// ---------------------------------------------------------------------------
// FuncDecl
// ---------------------------------------------------------------------------

// FuncDecl represents a function declaration.
type FuncDecl struct {
	Name       string
	Parent     string // non-empty for struct/interface methods
	Params     []Param
	ReturnType string
	Kind       BodyKind
	Body       []BodySegment
	Directives []Directive
	Pos        Position
}

func (*FuncDecl) node()                        {}
func (f *FuncDecl) DeclName() string           { return f.QualifiedName() }
func (f *FuncDecl) GetDirectives() []Directive { return f.Directives }

// QualifiedName returns "Parent.Name" if Parent is set, otherwise just Name.
func (f *FuncDecl) QualifiedName() string {
	if f.Parent != "" {
		return f.Parent + "." + f.Name
	}
	return f.Name
}

// ---------------------------------------------------------------------------
// StructDecl
// ---------------------------------------------------------------------------

// StructDecl represents a struct declaration with optional methods and type aliases.
type StructDecl struct {
	Name       string
	Types      []*TypeAlias
	Methods    []*FuncDecl
	Kind       BodyKind
	Body       []BodySegment
	Directives []Directive
	Pos        Position
}

func (*StructDecl) node()                        {}
func (s *StructDecl) DeclName() string           { return s.Name }
func (s *StructDecl) GetDirectives() []Directive { return s.Directives }

// ---------------------------------------------------------------------------
// InterfaceDecl
// ---------------------------------------------------------------------------

// InterfaceDecl represents an interface declaration with optional methods and type aliases.
type InterfaceDecl struct {
	Name       string
	Types      []*TypeAlias
	Methods    []*FuncDecl
	Kind       BodyKind
	Body       []BodySegment
	Directives []Directive
	Pos        Position
}

func (*InterfaceDecl) node()                        {}
func (i *InterfaceDecl) DeclName() string           { return i.Name }
func (i *InterfaceDecl) GetDirectives() []Directive { return i.Directives }

// ---------------------------------------------------------------------------
// InjectPromptDecl
// ---------------------------------------------------------------------------

// InjectPromptDecl represents an inject prompt block (prepended to all tasks).
type InjectPromptDecl struct {
	Body []BodySegment
	Kind BodyKind
}

func (*InjectPromptDecl) node()                      {}
func (*InjectPromptDecl) DeclName() string           { return "_inject_prompt" }
func (*InjectPromptDecl) GetDirectives() []Directive { return nil }

// ---------------------------------------------------------------------------
// ExternalSymbol — resolved symbol from a target/host file
// ---------------------------------------------------------------------------

// ExternalSymbol describes a host file symbol resolved by the coder.
// Contains all extracted data: kind, signature, full code block, documentation, and position.
type ExternalSymbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`      // "function", "struct", "interface", "enum", "trait"
	Signature string `json:"signature"` // e.g. "int my_func(int a, int b)"
	Code      string `json:"code"`      // full code block from source
	Doc       string `json:"doc"`       // documentation comment
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}
