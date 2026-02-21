package composer

import (
	"strings"
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

// testSource implements ASTSource for testing.
type testSource struct {
	files []*ast.File
}

func (s *testSource) Files() []*ast.File { return s.files }

func singleFile(decls ...ast.Declaration) ASTSource {
	return &testSource{files: []*ast.File{{Declarations: decls}}}
}

func singleFileWithInject(decls []ast.Declaration, injects []*ast.InjectPromptDecl) ASTSource {
	return &testSource{files: []*ast.File{{Declarations: decls, InjectPrompts: injects}}}
}

// testResolver implements SymbolResolver for testing.
type testResolver struct {
	symbols    map[string]ast.SymbolKind
	signatures map[string]string
	code       map[string]string
	docs       map[string]string
	generated  map[string]bool
}

func (r *testResolver) Symbols() map[string]ast.SymbolKind { return r.symbols }
func (r *testResolver) Signatures() map[string]string       { return r.signatures }
func (r *testResolver) GetCode(name string) (string, bool) {
	if r.code == nil {
		return "", false
	}
	c, ok := r.code[name]
	return c, ok
}
func (r *testResolver) GetDoc(name string) (string, bool) {
	if r.docs == nil {
		return "", false
	}
	d, ok := r.docs[name]
	return d, ok
}
func (r *testResolver) IsGenerated(name string) bool {
	if r.generated == nil {
		return false
	}
	return r.generated[name]
}

func symbolsOnly(symbols map[string]ast.SymbolKind) *testResolver {
	return &testResolver{symbols: symbols}
}

// ---------------------------------------------------------------------------
// Validate — [use X] checks
// ---------------------------------------------------------------------------

func TestUseRefToPromptIsError(t *testing.T) {
	src := singleFile(
		&ast.PromptDecl{
			Name: "Rules",
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Be safe"}},
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.TextSegment{Content: "Hello"},
				&ast.UseRefSegment{Name: "Rules", Pos: ast.Position{Line: 5, Column: 3}},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for [use prompt]")
	}
	if !strings.Contains(errs[0].Msg, "cannot use prompt") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestUseRefToSignatureOnlyFuncIsError(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "helper",
			Kind: ast.BodyNone, // signature only
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "helper", Pos: ast.Position{Line: 3, Column: 1}},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for [use signature-only func]")
	}
	if !strings.Contains(errs[0].Msg, "has no body") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestUseRefToFuncWithBodyIsOK(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "helper",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Do stuff"}},
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "helper"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestUseRefToUndeclaredIsError(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "Ghost", Pos: ast.Position{Line: 2, Column: 3}},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for undeclared reference")
	}
	if !strings.Contains(errs[0].Msg, "is not declared") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestUseRefToExternalSymbolIsOK(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "add_numbers"},
			},
		},
	)

	resolver := symbolsOnly(map[string]ast.SymbolKind{
		"add_numbers": ast.SymbolFunction,
	})

	c := New(src, resolver, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Validate — +code/+doc on AST declarations is error
// ---------------------------------------------------------------------------

func TestUseRefWithCodeModifierOnASTDeclIsError(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "helper",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Do stuff"}},
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "helper", AppendCode: true, Pos: ast.Position{Line: 5, Column: 3}},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for +code on AST declaration")
	}
	if !strings.Contains(errs[0].Msg, "+code") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestUseRefWithCodeModifierOnExternalIsOK(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "add_numbers", AppendCode: true},
			},
		},
	)

	resolver := symbolsOnly(map[string]ast.SymbolKind{
		"add_numbers": ast.SymbolFunction,
	})

	c := New(src, resolver, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Validate — [inject X] checks
// ---------------------------------------------------------------------------

func TestInjectRefToPromptIsOK(t *testing.T) {
	src := singleFile(
		&ast.PromptDecl{
			Name: "Rules",
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Be safe"}},
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.InjectRefSegment{Path: "Rules"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestInjectRefToFuncIsError(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "helper",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Do stuff"}},
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.InjectRefSegment{Path: "helper", Pos: ast.Position{Line: 4, Column: 3}},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for [inject func]")
	}
	if !strings.Contains(errs[0].Msg, "must point to a prompt") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestInjectRefQualifiedPathSkipped(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.InjectRefSegment{Path: "module.Rules"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors for qualified inject path: %v", errs)
	}
}

func TestInjectRefStdPromptExistsIsOK(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.InjectRefSegment{Path: "std.executor_role"},
			},
		},
	)

	known := map[string]bool{"std.executor_role": true}
	c := New(src, nil, nil, known)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestInjectRefStdPromptMisspelledIsError(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.InjectRefSegment{Path: "std.executor_interfacee", Pos: ast.Position{Line: 3, Column: 5}},
			},
		},
	)

	known := map[string]bool{"std.executor_interface": true}
	c := New(src, nil, nil, known)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for misspelled std prompt")
	}
	if !strings.Contains(errs[0].Msg, "does not exist") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestInjectRefNonStdQualifiedPathSkippedEvenWithKnown(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.InjectRefSegment{Path: "module.Foo"},
			},
		},
	)

	known := map[string]bool{"std.executor_role": true}
	c := New(src, nil, nil, known)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors for non-std qualified path: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Validate — [use X] for params and type aliases
// ---------------------------------------------------------------------------

func TestUseRefToParamIsOK(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "load",
			Kind: ast.BodyTask,
			Params: []ast.Param{
				{Name: "path", Type: "&std::path::Path"},
			},
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "path"},
				&ast.TextSegment{Content: "Load from path"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestUseRefToStructTypeAliasIsOK(t *testing.T) {
	src := singleFile(
		&ast.StructDecl{
			Name: "TodoList",
			Kind: ast.BodyTask,
			Types: []*ast.TypeAlias{
				{Name: "items", Type: "Vec<TodoItem>"},
			},
			Body: []ast.BodySegment{
				&ast.TextSegment{Content: "A list of todos"},
			},
			Methods: []*ast.FuncDecl{
				{
					Name:   "add",
					Parent: "TodoList",
					Kind:   ast.BodyTask,
					Params: []ast.Param{
						{Name: "&mut self"},
						{Name: "text", Type: "String"},
					},
					Body: []ast.BodySegment{
						&ast.UseRefSegment{Name: "text"},
						&ast.UseRefSegment{Name: "items"},
						&ast.TextSegment{Content: "Add item"},
					},
				},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestUseRefToParamNotAddedAsReference(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "load",
			Kind: ast.BodyTask,
			Params: []ast.Param{
				{Name: "path", Type: "&str"},
			},
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "path"},
				&ast.TextSegment{Content: "Load data"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	c.Validate()
	reqs := c.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if len(reqs[0].References) != 0 {
		t.Errorf("expected 0 references (param is local), got %d", len(reqs[0].References))
	}
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

func TestRequestsFromFuncDecl(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Params: []ast.Param{
				{Name: "name", Type: "string"},
			},
			ReturnType: "string",
			Body: []ast.BodySegment{
				&ast.TextSegment{Content: "Say hello to the user"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	reqs := c.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	req := reqs[0]
	if req.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", req.Name)
	}
	if req.Type != ExecutorAgent {
		t.Errorf("expected ExecutorAgent, got %q", req.Type)
	}
	if !strings.Contains(req.Task, "func greet(name string) -> string") {
		t.Errorf("expected signature in task, got %q", req.Task)
	}
	if !strings.Contains(req.Task, "Say hello to the user") {
		t.Errorf("expected body text in task, got %q", req.Task)
	}
}

func TestRequestsFromPlanDecl(t *testing.T) {
	src := singleFile(
		&ast.PlanDecl{
			Name: "BuildAPI",
			Declarations: []ast.Declaration{
				&ast.FuncDecl{
					Name: "createUser",
					Kind: ast.BodyTask,
					Body: []ast.BodySegment{
						&ast.TextSegment{Content: "Create a user"},
					},
				},
			},
		},
	)

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	reqs := c.Requests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests (1 inner func + 1 plan), got %d", len(reqs))
	}
	// Inner func should be ExecutorAgent
	if reqs[0].Type != ExecutorAgent {
		t.Errorf("expected ExecutorAgent for inner func, got %q", reqs[0].Type)
	}
	// Plan itself should be PlannerAgent
	if reqs[1].Type != PlannerAgent {
		t.Errorf("expected PlannerAgent for plan, got %q", reqs[1].Type)
	}
}

func TestRequestsSkipPromptAndSignatureOnly(t *testing.T) {
	src := singleFile(
		&ast.PromptDecl{
			Name: "Rules",
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Be safe"}},
		},
		&ast.FuncDecl{
			Name: "signatureOnly",
			Kind: ast.BodyNone,
		},
		&ast.FuncDecl{
			Name: "actual",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Work"}},
		},
	)

	c := New(src, nil, nil, nil)
	c.Validate()

	reqs := c.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request (only 'actual'), got %d", len(reqs))
	}
	if reqs[0].Name != "actual" {
		t.Errorf("expected 'actual', got %q", reqs[0].Name)
	}
}

func TestRequestsWithExternalReference(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.TextSegment{Content: "Hello"},
				&ast.UseRefSegment{Name: "add_numbers", AppendCode: true},
			},
		},
	)

	resolver := &testResolver{
		symbols: map[string]ast.SymbolKind{
			"add_numbers": ast.SymbolFunction,
		},
		signatures: map[string]string{
			"add_numbers": "int add_numbers(int a, int b)",
		},
		code: map[string]string{
			"add_numbers": "int add_numbers(int a, int b) { return a + b; }",
		},
	}

	c := New(src, resolver, nil, nil)
	c.Validate()

	reqs := c.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if len(reqs[0].References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(reqs[0].References))
	}

	ref := reqs[0].References[0]
	if ref.Name != "add_numbers" {
		t.Errorf("expected 'add_numbers', got %q", ref.Name)
	}
	if !ref.IsExternal {
		t.Error("expected IsExternal=true")
	}
	if ref.Kind != ast.SymbolFunction {
		t.Errorf("expected SymbolFunction, got %q", ref.Kind)
	}
	if !ref.AppendCode {
		t.Error("expected AppendCode=true")
	}
	if ref.Signature != "int add_numbers(int a, int b)" {
		t.Errorf("expected resolved signature, got %q", ref.Signature)
	}
	if ref.ResolvedCode != "int add_numbers(int a, int b) { return a + b; }" {
		t.Errorf("expected resolved code, got %q", ref.ResolvedCode)
	}
}

func TestRequestsWithInternalReference(t *testing.T) {
	src := singleFile(
		&ast.FuncDecl{
			Name: "helper",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{&ast.TextSegment{Content: "Help"}},
		},
		&ast.FuncDecl{
			Name: "greet",
			Kind: ast.BodyTask,
			Body: []ast.BodySegment{
				&ast.UseRefSegment{Name: "helper"},
			},
		},
	)

	c := New(src, nil, nil, nil)
	c.Validate()

	reqs := c.Requests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}

	// "greet" should have one internal reference.
	greetReq := reqs[1]
	if greetReq.Name != "greet" {
		t.Fatalf("expected 'greet', got %q", greetReq.Name)
	}
	if len(greetReq.References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(greetReq.References))
	}

	ref := greetReq.References[0]
	if ref.IsExternal {
		t.Error("expected IsExternal=false for internal reference")
	}
	if ref.Kind != "" {
		t.Errorf("expected empty Kind for internal ref, got %q", ref.Kind)
	}
}

// ---------------------------------------------------------------------------
// Validate — inject prompt body
// ---------------------------------------------------------------------------

func TestInjectPromptBodyValidated(t *testing.T) {
	ip := &ast.InjectPromptDecl{
		Body: []ast.BodySegment{
			&ast.UseRefSegment{Name: "Ghost", Pos: ast.Position{Line: 2, Column: 3}},
		},
		Kind: ast.BodyTask,
	}

	src := singleFileWithInject(nil, []*ast.InjectPromptDecl{ip})

	c := New(src, nil, nil, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for undeclared ref in inject prompt")
	}
	if !strings.Contains(errs[0].Msg, "is not declared") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

// ---------------------------------------------------------------------------
// Validate — [use X] against target file symbols
// ---------------------------------------------------------------------------

// testTargetResolver implements TargetResolver for testing.
type testTargetResolver struct {
	symbols map[string]map[string]ast.SymbolKind // path → name → kind
	sigs    map[string]map[string]string         // path → name → signature
	code    map[string]map[string]string         // path → name → code
	docs    map[string]map[string]string         // path → name → doc
}

func (r *testTargetResolver) ResolveTarget(path string) (map[string]ast.SymbolKind, map[string]string, error) {
	return r.symbols[path], r.sigs[path], nil
}

func (r *testTargetResolver) GetCode(path, name string) (string, bool) {
	if r.code == nil || r.code[path] == nil {
		return "", false
	}
	v, ok := r.code[path][name]
	return v, ok
}

func (r *testTargetResolver) GetDoc(path, name string) (string, bool) {
	if r.docs == nil || r.docs[path] == nil {
		return "", false
	}
	d, ok := r.docs[path][name]
	return d, ok
}

func TestUseRefToTargetSymbolIsOK(t *testing.T) {
	src := &testSource{files: []*ast.File{{
		Declarations: []ast.Declaration{
			&ast.PlanDecl{
				Name:    "Build",
				Targets: []string{"life.c"},
				Declarations: []ast.Declaration{
					&ast.FuncDecl{
						Name: "improve_life",
						Kind: ast.BodyTask,
						Body: []ast.BodySegment{
							&ast.UseRefSegment{Name: "init_grid"},
							&ast.TextSegment{Content: "Improve the init_grid function"},
						},
					},
				},
			},
		},
	}}}

	tr := &testTargetResolver{
		symbols: map[string]map[string]ast.SymbolKind{
			"life.c": {"init_grid": ast.SymbolFunction},
		},
		sigs: map[string]map[string]string{
			"life.c": {"init_grid": "void init_grid(int rows, int cols)"},
		},
	}

	c := New(src, nil, tr, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestUseRefToMissingTargetSymbolIsError(t *testing.T) {
	src := &testSource{files: []*ast.File{{
		Declarations: []ast.Declaration{
			&ast.PlanDecl{
				Name:    "Build",
				Targets: []string{"life.c"},
				Declarations: []ast.Declaration{
					&ast.FuncDecl{
						Name: "improve_life",
						Kind: ast.BodyTask,
						Body: []ast.BodySegment{
							&ast.UseRefSegment{Name: "nonexistent", Pos: ast.Position{Line: 5, Column: 3}},
						},
					},
				},
			},
		},
	}}}

	tr := &testTargetResolver{
		symbols: map[string]map[string]ast.SymbolKind{
			"life.c": {"init_grid": ast.SymbolFunction},
		},
	}

	c := New(src, nil, tr, nil)
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for [use nonexistent]")
	}
	if !strings.Contains(errs[0].Msg, "is not declared") {
		t.Errorf("unexpected error: %s", errs[0].Msg)
	}
}

func TestUseRefToTargetSymbolWithCodeModifier(t *testing.T) {
	src := &testSource{files: []*ast.File{{
		TargetPath: "life.c",
		Declarations: []ast.Declaration{
			&ast.FuncDecl{
				Name: "improve_life",
				Kind: ast.BodyTask,
				Body: []ast.BodySegment{
					&ast.UseRefSegment{Name: "init_grid", AppendCode: true},
				},
			},
		},
	}}}

	tr := &testTargetResolver{
		symbols: map[string]map[string]ast.SymbolKind{
			"life.c": {"init_grid": ast.SymbolFunction},
		},
		sigs: map[string]map[string]string{
			"life.c": {"init_grid": "void init_grid(int rows, int cols)"},
		},
		code: map[string]map[string]string{
			"life.c": {"init_grid": "void init_grid(int rows, int cols) { /* ... */ }"},
		},
	}

	c := New(src, nil, tr, nil)
	errs := c.Validate()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	reqs := c.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if len(reqs[0].References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(reqs[0].References))
	}

	ref := reqs[0].References[0]
	if !ref.IsExternal {
		t.Error("expected IsExternal=true for target symbol")
	}
	if ref.Signature != "void init_grid(int rows, int cols)" {
		t.Errorf("expected resolved signature, got %q", ref.Signature)
	}
	if ref.ResolvedCode != "void init_grid(int rows, int cols) { /* ... */ }" {
		t.Errorf("expected resolved code, got %q", ref.ResolvedCode)
	}
}
