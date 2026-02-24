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

// helper: compile sources with a specific baseDir (simulates package mode).
func compileSourceWithBaseDir(t *testing.T, sources map[string]string, baseDir string) Program {
	t.Helper()
	c := &compiler{baseDir: baseDir}
	prog, errs := c.ParseSources(sources)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	return prog
}

func TestEvalInjectPlan(t *testing.T) {
	source := `
	plan todo {
		target "src/todo.c"
		spec {
			Build a todo app with add and list functions.
		}
		impl add_todo {
			[target "src/todo.c"]
			implement the add function
		}
		impl list_todos {
			[target "src/todo.c"]
			implement the list function
		}
	}
	`

	prog := compileSource(t, source)
	result, err := prog.Eval("inject todo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("Eval('inject todo') returned empty result")
	}
	for _, want := range []string{"# todo", "Specification", "add_todo", "list_todos"} {
		if !strings.Contains(result, want) {
			t.Errorf("result should contain %q, got:\n%s", want, result)
		}
	}
}

func TestEvalInjectPlanImpl(t *testing.T) {
	source := `
	plan todo {
		target "src/todo.c"
		spec {
			Build a todo app.
		}
		impl add_todo {
			[target "src/todo.c"]
			implement the add function
		}
	}
	`

	prog := compileSource(t, source)
	result, err := prog.Eval("inject todo.add_todo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("Eval('inject todo.add_todo') returned empty result")
	}
	if !strings.Contains(result, "# add_todo") {
		t.Errorf("result should contain impl heading, got:\n%s", result)
	}
	if !strings.Contains(result, "implement the add function") {
		t.Errorf("result should contain impl body, got:\n%s", result)
	}
}

func TestEvalInjectNonexistent(t *testing.T) {
	prog := compileSource(t, `prompt greet { hi }`)
	result, err := prog.Eval("inject nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for nonexistent inject, got %q", result)
	}
}

func TestEvalInjectPlanWithBaseDir(t *testing.T) {
	source := `
	plan todo {
		target "src/todo.c"
		spec {
			Build a todo app with add and list.
		}
		impl add_todo {
			[target "src/todo.c"]
			implement add
		}
	}
	`

	// Simulate package mode: source lives in a subdirectory of baseDir.
	prog := compileSourceWithBaseDir(t, map[string]string{
		"/project/plans/todo.plan": source,
	}, "/project")

	result, err := prog.Eval("inject todo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("Eval with baseDir returned empty result")
	}
	for _, want := range []string{"# todo", "Specification", "add_todo"} {
		if !strings.Contains(result, want) {
			t.Errorf("result should contain %q, got:\n%s", want, result)
		}
	}
}

func TestEvalParseErrorTypo(t *testing.T) {
	prog := compileSource(t, `prompt greet { hi }`)
	_, err := prog.Eval("inejct todo")
	if err == nil {
		t.Fatal("expected parse error for typo 'inejct'")
	}
}

func TestCompilerUnresolvedUseIsWarningNotError(t *testing.T) {
	source := `
	plan MyPlan {
		target "src/main.go"
		spec {
			Build something
		}
		impl add {
			[target "src/main.go"]
			[use NonExistent]
			Add a new item
		}
	}
	`

	c := &compiler{}
	prog, errs := c.ParseSources(map[string]string{"test.vai": source})
	if len(errs) > 0 {
		t.Fatalf("compilation should succeed with unresolved [use], got errors: %v", errs)
	}
	warnings := prog.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning for unresolved [use NonExistent]")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Error(), "NonExistent") && strings.Contains(w.Error(), "not declared") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about 'NonExistent' not declared, got: %v", warnings)
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
