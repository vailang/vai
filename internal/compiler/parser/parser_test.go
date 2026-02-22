package parser

import (
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/lexer"
	"github.com/vailang/vai/internal/compiler/reader"
)

// helper to parse source and return the AST + errors
func parseSource(src string) (*ast.File, []Error) {
	scanner := lexer.NewScanner(reader.NewVaiSource(src))
	p := New(scanner)
	return p.ParseFile()
}

// mustParse parses source and fails if there are errors.
func mustParse(t *testing.T, src string) *ast.File {
	t.Helper()
	file, errs := parseSource(src)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Logf("parse error: %s", e)
		}
		t.Fatalf("expected no parse errors, got %d", len(errs))
	}
	return file
}

// helpers
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestFullExample tests parsing the complete example from the lexer test.
func TestFullExample(t *testing.T) {
	file := mustParse(t, `
		constraint life_handler {
			work hard, stay with family
		}

		prompt greet {
			Hello, World!

			[match user.language]{
				[case "en"] {
					Hello, World!
				}
				[case "es"] {
					¡Hola, Mundo!
				}
				[case _] {
					Hello, World!
				}
			}
		}

		inject greet

		plan my_hero_plan {
			target "src/hero.c"
			target "src/life.c"

			spec {
				[inject greet]
				Answer the question about the meaning of life in a concise way.
			}

			prompt handler {
				see good film!
			}

			impl main {
				[target "life.c"]
				[inject handler]
				[use life]
			}
		}

		inject my_hero_plan
	`)

	// Should have: constraint, prompt, inject, plan, inject = 5 declarations
	if len(file.Declarations) != 5 {
		t.Fatalf("expected 5 declarations, got %d", len(file.Declarations))
	}

	// 1. constraint life_handler
	c, ok := file.Declarations[0].(*ast.ConstraintDecl)
	if !ok {
		t.Fatalf("expected ConstraintDecl, got %T", file.Declarations[0])
	}
	if c.Name != "life_handler" {
		t.Errorf("constraint name = %q, want 'life_handler'", c.Name)
	}
	if len(c.Body) == 0 {
		t.Error("expected non-empty constraint body")
	}

	// 2. prompt greet with match/case
	pr, ok := file.Declarations[1].(*ast.PromptDecl)
	if !ok {
		t.Fatalf("expected PromptDecl, got %T", file.Declarations[1])
	}
	if pr.Name != "greet" {
		t.Errorf("prompt name = %q, want 'greet'", pr.Name)
	}
	// Should have text + match
	var matchSeg *ast.MatchSegment
	for _, seg := range pr.Body {
		if m, ok := seg.(*ast.MatchSegment); ok {
			matchSeg = m
		}
	}
	if matchSeg == nil {
		t.Fatal("expected MatchSegment in prompt body")
	}
	if matchSeg.Field != "user.language" {
		t.Errorf("match field = %q, want 'user.language'", matchSeg.Field)
	}
	if len(matchSeg.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(matchSeg.Cases))
	}
	if matchSeg.Cases[0].Value != "en" {
		t.Errorf("case[0] = %q, want 'en'", matchSeg.Cases[0].Value)
	}
	if matchSeg.Cases[1].Value != "es" {
		t.Errorf("case[1] = %q, want 'es'", matchSeg.Cases[1].Value)
	}
	if matchSeg.Cases[2].Value != "_" {
		t.Errorf("case[2] = %q, want '_' (default)", matchSeg.Cases[2].Value)
	}

	// 3. inject greet
	inj, ok := file.Declarations[2].(*ast.InjectDecl)
	if !ok {
		t.Fatalf("expected InjectDecl, got %T", file.Declarations[2])
	}
	if inj.Name != "greet" {
		t.Errorf("inject name = %q, want 'greet'", inj.Name)
	}

	// 4. plan my_hero_plan
	pd, ok := file.Declarations[3].(*ast.PlanDecl)
	if !ok {
		t.Fatalf("expected PlanDecl, got %T", file.Declarations[3])
	}
	if pd.Name != "my_hero_plan" {
		t.Errorf("plan name = %q, want 'my_hero_plan'", pd.Name)
	}
	if len(pd.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(pd.Targets))
	}
	if pd.Targets[0] != "src/hero.c" {
		t.Errorf("target[0] = %q, want 'src/hero.c'", pd.Targets[0])
	}
	if pd.Targets[1] != "src/life.c" {
		t.Errorf("target[1] = %q, want 'src/life.c'", pd.Targets[1])
	}
	if len(pd.Specs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(pd.Specs))
	}
	if len(pd.Declarations) != 1 {
		t.Errorf("expected 1 declaration (prompt), got %d", len(pd.Declarations))
	}
	if len(pd.Impls) != 1 {
		t.Fatalf("expected 1 impl, got %d", len(pd.Impls))
	}

	// Check impl
	impl := pd.Impls[0]
	if impl.Name != "main" {
		t.Errorf("impl name = %q, want 'main'", impl.Name)
	}
	// impl body: [target "life.c"], [inject handler], [use life]
	if len(impl.Body) != 3 {
		t.Fatalf("expected 3 impl body segments, got %d", len(impl.Body))
	}
	targetRef, ok := impl.Body[0].(*ast.TargetRefSegment)
	if !ok {
		t.Fatalf("expected TargetRefSegment, got %T", impl.Body[0])
	}
	if targetRef.Name != "life.c" {
		t.Errorf("target ref = %q, want 'life.c'", targetRef.Name)
	}

	// 5. inject my_hero_plan
	inj2, ok := file.Declarations[4].(*ast.InjectDecl)
	if !ok {
		t.Fatalf("expected InjectDecl, got %T", file.Declarations[4])
	}
	if inj2.Name != "my_hero_plan" {
		t.Errorf("inject name = %q, want 'my_hero_plan'", inj2.Name)
	}
}
