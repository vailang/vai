package parser

import (
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

func TestParsePrompt(t *testing.T) {
	file := mustParse(t, `prompt GolangDev {
  You are an expert Go developer.
  Write idiomatic code.
}`)
	pr := file.Declarations[0].(*ast.PromptDecl)
	if pr.Name != "GolangDev" {
		t.Errorf("expected 'GolangDev', got %q", pr.Name)
	}
	if len(pr.Body) == 0 {
		t.Fatal("expected body segments")
	}
}

func TestParsePromptWithInject(t *testing.T) {
	file := mustParse(t, `prompt GoDev {
  Base instructions.
  [inject base.reference_follow]
}`)
	pr := file.Declarations[0].(*ast.PromptDecl)
	if len(pr.Body) < 2 {
		t.Fatalf("expected at least 2 body segments, got %d", len(pr.Body))
	}
	found := false
	for _, seg := range pr.Body {
		if inj, ok := seg.(*ast.InjectRefSegment); ok {
			found = true
			if inj.Path != "base.reference_follow" {
				t.Errorf("expected path 'base.reference_follow', got %q", inj.Path)
			}
		}
	}
	if !found {
		t.Error("expected InjectRefSegment in prompt body")
	}
}

func TestParseMatchCaseInPrompt(t *testing.T) {
	file := mustParse(t, `prompt greet {
	Hello.

	[match config.target] {
		[case "rust"] {
			I generate Rust code.
		}
		[case "go"] {
			I generate Go code.
		}
	}
}`)
	pr := file.Declarations[0].(*ast.PromptDecl)
	if pr.Name != "greet" {
		t.Errorf("expected 'greet', got %q", pr.Name)
	}

	var matchSeg *ast.MatchSegment
	for _, seg := range pr.Body {
		if m, ok := seg.(*ast.MatchSegment); ok {
			matchSeg = m
			break
		}
	}
	if matchSeg == nil {
		t.Fatal("expected MatchSegment in prompt body")
	}
	if matchSeg.Field != "config.target" {
		t.Errorf("match field = %q, want 'config.target'", matchSeg.Field)
	}
	if len(matchSeg.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(matchSeg.Cases))
	}
	if matchSeg.Cases[0].Value != "rust" {
		t.Errorf("case[0] value = %q, want 'rust'", matchSeg.Cases[0].Value)
	}
	if matchSeg.Cases[1].Value != "go" {
		t.Errorf("case[1] value = %q, want 'go'", matchSeg.Cases[1].Value)
	}
}
