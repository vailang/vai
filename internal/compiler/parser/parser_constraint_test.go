package parser

import (
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

func TestParseConstraintTopLevel(t *testing.T) {
	file := mustParse(t, `constraint rules {
  Use tokio.
}`)

	if len(file.Declarations) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Declarations))
	}

	c, ok := file.Declarations[0].(*ast.ConstraintDecl)
	if !ok {
		t.Fatalf("expected ConstraintDecl, got %T", file.Declarations[0])
	}
	if c.Name != "rules" {
		t.Errorf("expected name 'rules', got %q", c.Name)
	}
	if len(c.Body) == 0 {
		t.Error("expected non-empty body")
	}

	hasText := false
	for _, seg := range c.Body {
		if ts, ok := seg.(*ast.TextSegment); ok && contains(ts.Content, "tokio") {
			hasText = true
		}
	}
	if !hasText {
		t.Error("expected body to contain text about tokio")
	}
}

func TestParseConstraintInsidePlan(t *testing.T) {
	file := mustParse(t, `plan Auth {
  constraint jwt_rules {
    Use JWT.
  }
  prompt login {
    Login.
  }
}`)

	pd, ok := file.Declarations[0].(*ast.PlanDecl)
	if !ok {
		t.Fatalf("expected PlanDecl, got %T", file.Declarations[0])
	}
	if len(pd.Constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(pd.Constraints))
	}
	if len(pd.Declarations) != 1 {
		t.Errorf("expected 1 declaration (prompt), got %d", len(pd.Declarations))
	}
	if pd.Constraints[0].Name != "jwt_rules" {
		t.Errorf("constraint name = %q, want 'jwt_rules'", pd.Constraints[0].Name)
	}
}

func TestParseMatchCaseInConstraint(t *testing.T) {
	file := mustParse(t, `constraint best_practices {
	Always follow best practices.
	[match config.target] {
		[case "go"] {
			Use gofmt.
		}
	}
}`)
	cd := file.Declarations[0].(*ast.ConstraintDecl)
	if len(cd.Body) < 2 {
		t.Fatalf("expected at least 2 body segments, got %d", len(cd.Body))
	}
	found := false
	for _, seg := range cd.Body {
		if _, ok := seg.(*ast.MatchSegment); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("expected MatchSegment in constraint body")
	}
}
