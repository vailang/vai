package compiler

import (
	"strings"
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

// buildProgram creates a program from a hand-built AST file.
func buildProgram(file *ast.File) *program {
	return &program{file: file}
}

func TestProgramNilFile(t *testing.T) {
	p := &program{}
	if p.Tasks() != 0 {
		t.Errorf("Tasks() = %d, want 0", p.Tasks())
	}
	if got, _ := p.Exec(); got != "" {
		t.Errorf("Exec() = %q, want empty", got)
	}
	if got := p.Render(); got != "" {
		t.Errorf("Render() = %q, want empty", got)
	}
	if p.HasPrompt("x") {
		t.Error("HasPrompt should be false on nil file")
	}
	if p.HasPlan("x") {
		t.Error("HasPlan should be false on nil file")
	}
	if got := p.ListPrompts(); got != nil {
		t.Errorf("ListPrompts() = %v, want nil", got)
	}
	if got := p.ListConstraints(); got != nil {
		t.Errorf("ListConstraints() = %v, want nil", got)
	}
	if got := p.File(); got != nil {
		t.Error("File() should be nil")
	}
	if got := p.Requests(); got != nil {
		t.Error("Requests() should be nil")
	}
}

func TestProgramHasPrompt(t *testing.T) {
	file := &ast.File{
		Declarations: []ast.Declaration{
			&ast.PromptDecl{Name: "greet"},
			&ast.PromptDecl{Name: "farewell"},
		},
	}
	p := buildProgram(file)

	if !p.HasPrompt("greet") {
		t.Error("HasPrompt(greet) should be true")
	}
	if !p.HasPrompt("farewell") {
		t.Error("HasPrompt(farewell) should be true")
	}
	if p.HasPrompt("missing") {
		t.Error("HasPrompt(missing) should be false")
	}
}

func TestProgramHasPlan(t *testing.T) {
	file := &ast.File{
		Declarations: []ast.Declaration{
			&ast.PlanDecl{Name: "rust"},
		},
	}
	p := buildProgram(file)

	if !p.HasPlan("rust") {
		t.Error("HasPlan(rust) should be true")
	}
	if p.HasPlan("go") {
		t.Error("HasPlan(go) should be false")
	}
}

func TestProgramListPrompts(t *testing.T) {
	file := &ast.File{
		Declarations: []ast.Declaration{
			&ast.PromptDecl{Name: "a"},
			&ast.PlanDecl{Name: "plan1"},
			&ast.PromptDecl{Name: "b"},
		},
	}
	p := buildProgram(file)

	names := p.ListPrompts()
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("ListPrompts() = %v, want [a b]", names)
	}
}

func TestProgramListConstraints(t *testing.T) {
	file := &ast.File{
		Declarations: []ast.Declaration{
			&ast.ConstraintDecl{Name: "global"},
			&ast.PlanDecl{
				Name: "myplan",
				Constraints: []*ast.ConstraintDecl{
					{Name: "local"},
				},
			},
		},
	}
	p := buildProgram(file)

	names := p.ListConstraints()
	if len(names) != 2 || names[0] != "global" || names[1] != "local" {
		t.Errorf("ListConstraints() = %v, want [global local]", names)
	}
}

func TestProgramFileAccessor(t *testing.T) {
	file := &ast.File{SourcePath: "/test/main.vai"}
	p := buildProgram(file)

	if p.File() != file {
		t.Error("File() should return the same file")
	}
}

func TestProgramEval(t *testing.T) {
	// Compile a base program with a prompt, then eval inject against it.
	prog := compileSource(t, `prompt greet {
	Hello from eval!
	}`)

	result, err := prog.Eval("inject greet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Hello from eval!") {
		t.Errorf("Eval result should contain prompt text, got %q", result)
	}
}

func TestProgramEvalParseError(t *testing.T) {
	prog := compileSource(t, `prompt greet { hi }`)

	_, err := prog.Eval("func {")
	if err == nil {
		t.Fatal("expected parse error from Eval")
	}
}
