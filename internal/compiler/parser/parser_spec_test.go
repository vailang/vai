package parser

import (
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

func TestParseSpecInsidePlan(t *testing.T) {
	file := mustParse(t, `plan Auth {
  spec {
    Build auth system.
  }
}`)

	pd, ok := file.Declarations[0].(*ast.PlanDecl)
	if !ok {
		t.Fatalf("expected PlanDecl, got %T", file.Declarations[0])
	}
	if len(pd.Specs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(pd.Specs))
	}

	s := pd.Specs[0]
	if len(s.Body) == 0 {
		t.Error("expected non-empty spec body")
	}

	hasText := false
	for _, seg := range s.Body {
		if ts, ok := seg.(*ast.TextSegment); ok && contains(ts.Content, "auth") {
			hasText = true
		}
	}
	if !hasText {
		t.Error("expected spec body to contain text about auth")
	}
}

func TestSpecAtTopLevelErrors(t *testing.T) {
	_, errs := parseSource(`spec {
  Not valid here.
}`)

	if len(errs) == 0 {
		t.Fatal("expected parse errors for spec at top level, got none")
	}

	hasRelevantError := false
	for _, e := range errs {
		msg := e.Error()
		if contains(msg, "spec") || contains(msg, "plan") {
			hasRelevantError = true
			break
		}
	}
	if !hasRelevantError {
		t.Errorf("expected error about spec, got: %v", errs)
	}
}
