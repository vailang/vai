package diagnostic

import "testing"

func TestGoParserBasic(t *testing.T) {
	input := `{"Action":"build-output","Output":"main.go:15:2: undefined: addTodo\n"}
{"Action":"build-output","Output":"main.go:20:5: cannot use x (variable of type string) as int\n"}
{"Action":"build-fail","Output":""}
`
	p := &GoParser{}
	diags, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}
	if diags[0].File != "main.go" || diags[0].Line != 15 || diags[0].Column != 2 {
		t.Errorf("diag[0] = %+v", diags[0])
	}
	if diags[1].Line != 20 || diags[1].Message != "cannot use x (variable of type string) as int" {
		t.Errorf("diag[1] = %+v", diags[1])
	}
}

func TestGoParserEmpty(t *testing.T) {
	p := &GoParser{}
	diags, err := p.Parse([]byte(`{"Action":"build-start"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestGoParserInvalidJSON(t *testing.T) {
	p := &GoParser{}
	diags, err := p.Parse([]byte("not json at all\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}
