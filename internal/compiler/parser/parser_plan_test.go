package parser

import (
	"testing"

	"github.com/vailang/vai/internal/compiler/ast"
)

func TestParsePlanWithPromptAndSpec(t *testing.T) {
	file := mustParse(t, `plan BuildAPI {
  spec {
    Build the API endpoints.
  }
  prompt handler {
    Handle requests.
  }
}`)

	if len(file.Declarations) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Declarations))
	}
	pd, ok := file.Declarations[0].(*ast.PlanDecl)
	if !ok {
		t.Fatalf("expected PlanDecl, got %T", file.Declarations[0])
	}
	if pd.Name != "BuildAPI" {
		t.Errorf("expected name 'BuildAPI', got %q", pd.Name)
	}
	if len(pd.Specs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(pd.Specs))
	}
	if len(pd.Declarations) != 1 {
		t.Errorf("expected 1 declaration (prompt), got %d", len(pd.Declarations))
	}
}

func TestParsePlanWithImpl(t *testing.T) {
	file := mustParse(t, `plan Auth {
  constraint jwt_rules {
    Use JWT tokens.
  }
  spec {
    Build an authentication system.
  }
  impl "int main()" {
    [target "src/auth.c"]
    [inject login]
  }
}`)
	pd := file.Declarations[0].(*ast.PlanDecl)
	if len(pd.Constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(pd.Constraints))
	}
	if len(pd.Specs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(pd.Specs))
	}
	if len(pd.Impls) != 1 {
		t.Fatalf("expected 1 impl, got %d", len(pd.Impls))
	}
	impl := pd.Impls[0]
	if impl.Signature != "int main()" {
		t.Errorf("impl signature = %q, want 'int main()'", impl.Signature)
	}
	if len(impl.Body) != 2 {
		t.Fatalf("expected 2 impl body segments, got %d", len(impl.Body))
	}
	// Check target ref
	tr, ok := impl.Body[0].(*ast.TargetRefSegment)
	if !ok {
		t.Fatalf("expected TargetRefSegment, got %T", impl.Body[0])
	}
	if tr.Name != "src/auth.c" {
		t.Errorf("target = %q, want 'src/auth.c'", tr.Name)
	}
}

func TestParsePlanWithInject(t *testing.T) {
	file := mustParse(t, `plan runner {
  inject setup
  prompt main {
    Run the thing.
  }
}`)
	pd := file.Declarations[0].(*ast.PlanDecl)
	if len(pd.Declarations) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(pd.Declarations))
	}
	inj, ok := pd.Declarations[0].(*ast.InjectDecl)
	if !ok {
		t.Fatalf("expected InjectDecl, got %T", pd.Declarations[0])
	}
	if inj.Name != "setup" {
		t.Errorf("inject name = %q, want 'setup'", inj.Name)
	}
}
