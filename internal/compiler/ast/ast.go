package ast

import "fmt"

// Position represents a source location.
type Position struct {
	Line   int // 1-indexed
	Column int // 1-indexed
	Offset int // byte offset in source
}

// ---------------------------------------------------------------------------
// Body Segments
// ---------------------------------------------------------------------------

// BodySegment represents a segment inside a body block.
type BodySegment interface {
	bodySegment()
	SegmentPos() Position
}

// TextSegment is raw text inside a body.
type TextSegment struct {
	Content string
	Pos     Position
}

func (*TextSegment) bodySegment()           {}
func (s *TextSegment) SegmentPos() Position { return s.Pos }

// UseRefSegment represents [use X] inside a body.
type UseRefSegment struct {
	Name       string // "life", "handler"
	AppendCode bool   // +code modifier
	AppendDoc  bool   // +doc modifier
	Pos        Position
}

func (*UseRefSegment) bodySegment()           {}
func (s *UseRefSegment) SegmentPos() Position { return s.Pos }

// InjectRefSegment represents [inject X] inside a body.
type InjectRefSegment struct {
	Path string // "greet", "handler"
	Pos  Position
}

func (*InjectRefSegment) bodySegment()           {}
func (s *InjectRefSegment) SegmentPos() Position { return s.Pos }

// TargetRefSegment represents [target "path"] inside a body.
type TargetRefSegment struct {
	Name string // "life.c", "life_handler"
	Pos  Position
}

func (*TargetRefSegment) bodySegment()           {}
func (s *TargetRefSegment) SegmentPos() Position { return s.Pos }

// ReferenceRefSegment represents [reference "path"] inside a body.
// Like TargetRefSegment but for symbol resolution only (not emitted in status output).
type ReferenceRefSegment struct {
	Name string // "lib.rs", "utils.py"
	Pos  Position
}

func (*ReferenceRefSegment) bodySegment()           {}
func (s *ReferenceRefSegment) SegmentPos() Position { return s.Pos }

// MatchSegment represents [match field] { [case "val"] { body } ... } inside a body.
type MatchSegment struct {
	Field string        // "user.language"
	Cases []*CaseClause // Ordered list of cases
	Pos   Position
}

func (*MatchSegment) bodySegment()           {}
func (s *MatchSegment) SegmentPos() Position { return s.Pos }

// CaseClause represents one [case "value"] { body } inside a match block.
// Value "_" represents the default/wildcard case.
type CaseClause struct {
	Value string        // "en", "es", "_"
	Body  []BodySegment // Body segments for this case
	Pos   Position
}

// ---------------------------------------------------------------------------
// Node & Declaration interfaces
// ---------------------------------------------------------------------------

// Node is the interface for all AST nodes.
type Node interface {
	node()
}

// Declaration is implemented by all top-level declarations.
type Declaration interface {
	Node
	DeclName() string
	GetDirectives() []Directive
}

// ---------------------------------------------------------------------------
// ConstraintDecl
// ---------------------------------------------------------------------------

// ConstraintDecl represents a constraint block with a name and free text body.
// Syntax: constraint name { free text }
type ConstraintDecl struct {
	Name string
	Body []BodySegment
	Pos  Position
}

func (*ConstraintDecl) node()                        {}
func (c *ConstraintDecl) DeclName() string             { return c.Name }
func (*ConstraintDecl) GetDirectives() []Directive      { return nil }

// ---------------------------------------------------------------------------
// PromptDecl
// ---------------------------------------------------------------------------

// PromptDecl represents a reusable prompt definition.
// Syntax: prompt name { body }
type PromptDecl struct {
	Name       string
	Body       []BodySegment
	References []string // reference "path" declarations
	IsPrivate  bool
	Pos        Position
}

func (*PromptDecl) node()                        {}
func (p *PromptDecl) DeclName() string             { return p.Name }
func (*PromptDecl) GetDirectives() []Directive      { return nil }

// ---------------------------------------------------------------------------
// InjectDecl
// ---------------------------------------------------------------------------

// InjectDecl represents a top-level inject statement (print/execute).
// Syntax: inject name
type InjectDecl struct {
	Name string
	Pos  Position
}

func (*InjectDecl) node()                        {}
func (i *InjectDecl) DeclName() string             { return "_inject_" + i.Name }
func (*InjectDecl) GetDirectives() []Directive      { return nil }

// ---------------------------------------------------------------------------
// SpecDecl
// ---------------------------------------------------------------------------

// SpecDecl represents a spec block with free text body.
// Only valid inside a plan block.
// Syntax: spec { free text }
type SpecDecl struct {
	Body []BodySegment
	Pos  Position
}

func (*SpecDecl) node()                        {}
func (s *SpecDecl) DeclName() string             { return "_spec" }
func (*SpecDecl) GetDirectives() []Directive      { return nil }

// ---------------------------------------------------------------------------
// ImplDecl
// ---------------------------------------------------------------------------

// ImplDecl represents an impl block inside a plan.
// Syntax: impl name { body }
type ImplDecl struct {
	Name string // symbol name to implement (e.g. "main", "add")
	Body []BodySegment
	Pos  Position
}

func (*ImplDecl) node()                        {}
func (i *ImplDecl) DeclName() string             { return i.Name }
func (*ImplDecl) GetDirectives() []Directive      { return nil }

// ---------------------------------------------------------------------------
// PlanDecl
// ---------------------------------------------------------------------------

// PlanDecl represents a plan declaration containing structured declarations.
// Syntax: plan Name { declarations... }
type PlanDecl struct {
	Name         string
	Declarations []Declaration
	Constraints  []*ConstraintDecl
	Specs        []*SpecDecl
	Impls        []*ImplDecl
	Targets      []string // target "path" declarations
	References   []string // reference "path" declarations (symbol source, not in status)
	Pos          Position
}

func (*PlanDecl) node()                        {}
func (pl *PlanDecl) DeclName() string            { return pl.Name }
func (*PlanDecl) GetDirectives() []Directive      { return nil }

// ---------------------------------------------------------------------------
// File (root node)
// ---------------------------------------------------------------------------

// File represents a complete .vai source file.
type File struct {
	SourcePath    string               // Absolute path of the .vai source file
	Declarations  []Declaration        // All top-level declarations in source order
	TargetPath    string               // Target file path (from vai: header or plan)
	InjectPrompts []*InjectPromptDecl  // Inject prompt blocks prepended to all tasks
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// MergeFiles combines multiple files into a single File.
func MergeFiles(files []*File) (*File, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files to merge")
	}
	if len(files) == 1 {
		return files[0], nil
	}

	merged := &File{}
	for _, f := range files {
		merged.Declarations = append(merged.Declarations, f.Declarations...)
		// Preserve the first non-empty source path as the base.
		if merged.SourcePath == "" && f.SourcePath != "" {
			merged.SourcePath = f.SourcePath
		}
	}
	return merged, nil
}

// ---------------------------------------------------------------------------
// Declaration type checks
// ---------------------------------------------------------------------------

// IsPrompt reports whether d is a *PromptDecl.
func IsPrompt(d Declaration) bool {
	_, ok := d.(*PromptDecl)
	return ok
}

// IsPlan reports whether d is a *PlanDecl.
func IsPlan(d Declaration) bool {
	_, ok := d.(*PlanDecl)
	return ok
}

// IsConstraint reports whether d is a *ConstraintDecl.
func IsConstraint(d Declaration) bool {
	_, ok := d.(*ConstraintDecl)
	return ok
}

// IsSpec reports whether d is a *SpecDecl.
func IsSpec(d Declaration) bool {
	_, ok := d.(*SpecDecl)
	return ok
}

// IsImpl reports whether d is a *ImplDecl.
func IsImpl(d Declaration) bool {
	_, ok := d.(*ImplDecl)
	return ok
}

// IsInject reports whether d is a *InjectDecl.
func IsInject(d Declaration) bool {
	_, ok := d.(*InjectDecl)
	return ok
}

// KindName returns a human-readable name for a declaration's type.
func KindName(d Declaration) string {
	switch d.(type) {
	case *PromptDecl:
		return "prompt"
	case *PlanDecl:
		return "plan"
	case *ConstraintDecl:
		return "constraint"
	case *SpecDecl:
		return "spec"
	case *ImplDecl:
		return "impl"
	case *InjectDecl:
		return "inject"
	case *FuncDecl:
		return "func"
	case *StructDecl:
		return "struct"
	case *InterfaceDecl:
		return "interface"
	}
	return "unknown"
}

// HasBody reports whether d has a non-empty body.
func HasBody(d Declaration) bool {
	switch v := d.(type) {
	case *PromptDecl:
		return len(v.Body) > 0
	case *ConstraintDecl:
		return len(v.Body) > 0
	case *SpecDecl:
		return len(v.Body) > 0
	case *ImplDecl:
		return len(v.Body) > 0
	case *PlanDecl:
		return len(v.Declarations) > 0 || len(v.Impls) > 0 || len(v.Specs) > 0 || len(v.Constraints) > 0
	case *FuncDecl:
		return v.Kind != BodyNone && len(v.Body) > 0
	case *StructDecl:
		return v.Kind != BodyNone && (len(v.Body) > 0 || len(v.Methods) > 0)
	case *InterfaceDecl:
		return v.Kind != BodyNone && (len(v.Body) > 0 || len(v.Methods) > 0)
	}
	return false
}

// walkSegments visits body segments, recursing into MatchSegment cases.
func walkSegments(segs []BodySegment, fn func(BodySegment)) {
	for _, seg := range segs {
		fn(seg)
		if m, ok := seg.(*MatchSegment); ok {
			for _, c := range m.Cases {
				walkSegments(c.Body, fn)
			}
		}
	}
}

// WalkBodySegments visits all body segments in a declaration, including
// match/case nested segments.
func WalkBodySegments(decl Declaration, fn func(BodySegment)) {
	switch d := decl.(type) {
	case *PromptDecl:
		walkSegments(d.Body, fn)
	case *ConstraintDecl:
		walkSegments(d.Body, fn)
	case *SpecDecl:
		walkSegments(d.Body, fn)
	case *ImplDecl:
		walkSegments(d.Body, fn)
	case *PlanDecl:
		for _, inner := range d.Declarations {
			WalkBodySegments(inner, fn)
		}
		for _, c := range d.Constraints {
			WalkBodySegments(c, fn)
		}
		for _, s := range d.Specs {
			WalkBodySegments(s, fn)
		}
		for _, i := range d.Impls {
			WalkBodySegments(i, fn)
		}
	case *FuncDecl:
		walkSegments(d.Body, fn)
	case *StructDecl:
		walkSegments(d.Body, fn)
		for _, m := range d.Methods {
			walkSegments(m.Body, fn)
		}
	case *InterfaceDecl:
		walkSegments(d.Body, fn)
		for _, m := range d.Methods {
			walkSegments(m.Body, fn)
		}
	case *InjectPromptDecl:
		walkSegments(d.Body, fn)
	}
}
