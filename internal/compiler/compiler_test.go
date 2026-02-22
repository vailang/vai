package compiler

import (
	"strings"
	"testing"
)

// helper: compile source string without target resolution.
func compileSource(t *testing.T, source string) Program {
	t.Helper()
	c := &compiler{}
	prog, errs := c.ParseSources(map[string]string{"test.vai": source})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	return prog
}

func TestCompilerPrompt(t *testing.T) {
	source := `prompt greet {
	Hello, World!
	}
	`

	prog := compileSource(t, source)
	result, err := prog.Eval("inject greet")
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("unexpected result: got %q, want %q", result, "Hello, World!")
	}
}

func TestCompilerNilCoder(t *testing.T) {
	source := `prompt base {
	You are a helpful assistant.
	}
	`

	prog := compileSource(t, source)
	if prog.Tasks() != 0 {
		t.Errorf("expected 0 tasks for prompt only, got %d", prog.Tasks())
	}
}

func TestCompilerMultipleInjects(t *testing.T) {
	source := `prompt greeting {
	Hello!
	}
	prompt farewell {
	Goodbye!
	}
	`

	prog := compileSource(t, source)
	result, err := prog.Eval("inject greeting\ninject farewell")
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if !strings.Contains(result, "Hello!") {
		t.Errorf("result should contain 'Hello!', got %q", result)
	}
	if !strings.Contains(result, "Goodbye!") {
		t.Errorf("result should contain 'Goodbye!', got %q", result)
	}
}

func TestCompilerEmpty(t *testing.T) {
	c := &compiler{}
	prog, errs := c.ParseSources(map[string]string{"test.vai": ""})
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if prog.Tasks() != 0 {
		t.Errorf("expected 0 tasks for empty source, got %d", prog.Tasks())
	}
}

func TestCompilerParseError(t *testing.T) {
	c := &compiler{}
	_, errs := c.ParseSources(map[string]string{"test.vai": `func {`})
	if len(errs) == 0 {
		t.Fatal("expected parse errors for invalid source")
	}
}

func TestCompilerTaskCount(t *testing.T) {
	source := `
	plan MyPlan {
		target "src/main.go"
		spec {
			Build a todo app
		}
		impl add {
			[target "src/main.go"]
			Add a new item
		}
	}
	`

	prog := compileSource(t, source)
	if prog.Tasks() < 1 {
		t.Errorf("expected at least 1 task, got %d", prog.Tasks())
	}
}

func TestCompilerMultiFile(t *testing.T) {
	sources := map[string]string{
		"/project/prompts.vai": `prompt base {
	You are helpful.
	}`,
		"/project/main.vai": ``,
	}

	c := &compiler{}
	prog, errs := c.ParseSources(sources)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	result, err := prog.Eval("inject base")
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if !strings.Contains(result, "You are helpful.") {
		t.Errorf("result should contain prompt text, got %q", result)
	}
}
