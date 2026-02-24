package diagnostic

import "testing"

func TestRustParserBasic(t *testing.T) {
	input := `{"reason":"compiler-message","message":{"level":"error","message":"cannot find value ` + "`x`" + `","spans":[{"file_name":"src/main.rs","line_start":10,"column_start":5,"is_primary":true}],"children":[]}}
{"reason":"build-finished","success":false}
`
	p := &RustParser{}
	diags, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].File != "src/main.rs" || diags[0].Line != 10 || diags[0].Column != 5 {
		t.Errorf("diag = %+v", diags[0])
	}
}

func TestRustParserSkipsWarnings(t *testing.T) {
	input := `{"reason":"compiler-message","message":{"level":"warning","message":"unused variable","spans":[{"file_name":"src/main.rs","line_start":5,"column_start":1,"is_primary":true}]}}
`
	p := &RustParser{}
	diags, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestRustParserSkipsNonPrimarySpans(t *testing.T) {
	input := `{"reason":"compiler-message","message":{"level":"error","message":"type mismatch","spans":[{"file_name":"src/main.rs","line_start":10,"column_start":5,"is_primary":false},{"file_name":"src/main.rs","line_start":15,"column_start":1,"is_primary":true}]}}
`
	p := &RustParser{}
	diags, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Line != 15 {
		t.Errorf("expected primary span line 15, got %d", diags[0].Line)
	}
}
