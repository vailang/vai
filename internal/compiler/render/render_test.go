package render

import (
	"strings"
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

// mockTargetInfo implements TargetInfo for testing.
type mockTargetInfo struct {
	symbols   map[string]ast.SymbolKind     // name → kind
	sigs      map[string]string             // name → signature
	code      map[string]map[string]string  // path → name → code
	skeletons map[string]string             // path → skeleton
	docs      map[string]map[string]string  // path → name → doc
}

func (m *mockTargetInfo) ResolveTarget(path string) (map[string]ast.SymbolKind, map[string]string, error) {
	return m.symbols, m.sigs, nil
}

func (m *mockTargetInfo) GetCode(path, name string) (string, bool) {
	if m.code == nil {
		return "", false
	}
	if pathCodes, ok := m.code[path]; ok {
		code, ok := pathCodes[name]
		return code, ok
	}
	return "", false
}

func (m *mockTargetInfo) GetSkeleton(path string) (string, bool) {
	if m.skeletons == nil {
		return "", false
	}
	s, ok := m.skeletons[path]
	return s, ok
}

func (m *mockTargetInfo) GetDoc(path, name string) (string, bool) {
	if m.docs == nil {
		return "", false
	}
	if pathDocs, ok := m.docs[path]; ok {
		doc, ok := pathDocs[name]
		return doc, ok
	}
	return "", false
}

func TestBodyText(t *testing.T) {
	segments := []ast.BodySegment{
		&ast.TextSegment{Content: "Hello"},
		&ast.TextSegment{Content: "  World  "},
		&ast.UseRefSegment{Name: "ignored"},
	}
	got := BodyText(segments)
	want := "Hello\nWorld"
	if got != want {
		t.Errorf("BodyText() = %q, want %q", got, want)
	}
}

func TestBodyTextEmpty(t *testing.T) {
	got := BodyText(nil)
	if got != "" {
		t.Errorf("BodyText(nil) = %q, want empty", got)
	}
}

func TestUseRefFunction(t *testing.T) {
	ref := &ast.UseRefSegment{Name: "add"}
	symbols := map[string]ast.SymbolKind{"add": ast.SymbolFunction}
	sigs := map[string]string{"add": "fn add(a: i32, b: i32) -> i32"}

	got := UseRef(ref, symbols, sigs, nil, nil)
	want := "`fn add(a: i32, b: i32) -> i32`"
	if got != want {
		t.Errorf("UseRef(func) = %q, want %q", got, want)
	}
}

func TestUseRefStruct(t *testing.T) {
	ref := &ast.UseRefSegment{Name: "Todo"}
	symbols := map[string]ast.SymbolKind{"Todo": ast.SymbolStruct}
	sigs := map[string]string{}

	target := &mockTargetInfo{
		code: map[string]map[string]string{
			"/src/main.rs": {"Todo": "struct Todo {\n    title: String,\n}"},
		},
	}

	got := UseRef(ref, symbols, sigs, target, []string{"/src/main.rs"})
	if !strings.Contains(got, "```rust") {
		t.Errorf("UseRef(struct) should contain rust code fence, got %q", got)
	}
	if !strings.Contains(got, "struct Todo") {
		t.Errorf("UseRef(struct) should contain struct code, got %q", got)
	}
}

func TestUseRefInterface(t *testing.T) {
	ref := &ast.UseRefSegment{Name: "Reader"}
	symbols := map[string]ast.SymbolKind{"Reader": ast.SymbolInterface}
	sigs := map[string]string{}

	target := &mockTargetInfo{
		code: map[string]map[string]string{
			"/src/lib.go": {"Reader": "type Reader interface {\n\tRead(p []byte) (int, error)\n}"},
		},
		docs: map[string]map[string]string{
			"/src/lib.go": {"Reader": "// Reader reads bytes."},
		},
	}

	got := UseRef(ref, symbols, sigs, target, []string{"/src/lib.go"})
	if !strings.Contains(got, "// Reader reads bytes.") {
		t.Errorf("UseRef(interface) should contain doc, got %q", got)
	}
	if !strings.Contains(got, "```go") {
		t.Errorf("UseRef(interface) should contain go code fence, got %q", got)
	}
}

func TestUseRefUnresolved(t *testing.T) {
	ref := &ast.UseRefSegment{Name: "missing"}
	got := UseRef(ref, nil, nil, nil, nil)
	want := "[use missing]"
	if got != want {
		t.Errorf("UseRef(missing) = %q, want %q", got, want)
	}
}

func TestLangTag(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"lib.rs", "rust"},
		{"app.py", "python"},
		{"index.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"main.c", "c"},
		{"main.h", "c"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		got := LangTag(tt.path)
		if got != tt.want {
			t.Errorf("LangTag(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestImplTargetPath(t *testing.T) {
	impl := &ast.ImplDecl{
		Name: "add",
		Body: []ast.BodySegment{
			&ast.TargetRefSegment{Name: "src/main.rs"},
			&ast.TextSegment{Content: "implement add"},
		},
	}
	got := ImplTargetPath(impl, "/project")
	want := "/project/src/main.rs"
	if got != want {
		t.Errorf("ImplTargetPath() = %q, want %q", got, want)
	}
}

func TestImplTargetPathAbsolute(t *testing.T) {
	impl := &ast.ImplDecl{
		Name: "add",
		Body: []ast.BodySegment{
			&ast.TargetRefSegment{Name: "/abs/path/main.rs"},
		},
	}
	got := ImplTargetPath(impl, "/project")
	want := "/abs/path/main.rs"
	if got != want {
		t.Errorf("ImplTargetPath() = %q, want %q", got, want)
	}
}

func TestImplTargetPathNone(t *testing.T) {
	impl := &ast.ImplDecl{
		Name: "add",
		Body: []ast.BodySegment{
			&ast.TextSegment{Content: "no target"},
		},
	}
	got := ImplTargetPath(impl, "/project")
	if got != "" {
		t.Errorf("ImplTargetPath() = %q, want empty", got)
	}
}

func TestExec(t *testing.T) {
	file := &ast.File{
		Declarations: []ast.Declaration{
			&ast.PromptDecl{
				Name: "greet",
				Body: []ast.BodySegment{
					&ast.TextSegment{Content: "Hello!"},
				},
			},
			&ast.InjectDecl{Name: "greet"},
		},
	}

	prompts := map[string]*ast.PromptDecl{
		"greet": file.Declarations[0].(*ast.PromptDecl),
	}
	plans := map[string]*ast.PlanDecl{}

	got := Exec(file, prompts, plans, nil, "/tmp")
	if got != "Hello!" {
		t.Errorf("Exec() = %q, want %q", got, "Hello!")
	}
}

func TestExecNilFile(t *testing.T) {
	got := Exec(nil, nil, nil, nil, "")
	if got != "" {
		t.Errorf("Exec(nil) = %q, want empty", got)
	}
}

func TestRenderNilFile(t *testing.T) {
	got := Render(nil, nil, nil, nil, "")
	if got != "" {
		t.Errorf("Render(nil) = %q, want empty", got)
	}
}
