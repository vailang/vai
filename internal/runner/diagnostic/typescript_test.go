package diagnostic

import "testing"

func TestTypeScriptParserBasic(t *testing.T) {
	input := `src/index.ts(10,5): error TS2304: Cannot find name 'foo'.
src/index.ts(20,1): error TS2322: Type 'string' is not assignable to type 'number'.
`
	p := &TypeScriptParser{}
	diags, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}
	if diags[0].File != "src/index.ts" || diags[0].Line != 10 || diags[0].Column != 5 {
		t.Errorf("diag[0] = %+v", diags[0])
	}
	if diags[1].Message != "Type 'string' is not assignable to type 'number'." {
		t.Errorf("diag[1].Message = %q", diags[1].Message)
	}
}

func TestTypeScriptParserEmpty(t *testing.T) {
	p := &TypeScriptParser{}
	diags, err := p.Parse([]byte("some random output\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}
