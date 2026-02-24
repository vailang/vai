package diagnostic

import "encoding/json"

// ClangParser parses `gcc -fdiagnostics-format=json` output.
type ClangParser struct{}

type gccDiag struct {
	Kind      string        `json:"kind"`
	Locations []gccLocation `json:"locations"`
	Message   string        `json:"message"`
}

type gccLocation struct {
	Caret gccPos `json:"caret"`
}

type gccPos struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func (p *ClangParser) Parse(output []byte) ([]Diagnostic, error) {
	var raw []gccDiag
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, nil
	}
	var diags []Diagnostic
	for _, d := range raw {
		if d.Kind != "error" {
			continue
		}
		for _, loc := range d.Locations {
			diags = append(diags, Diagnostic{
				File:    loc.Caret.File,
				Line:    loc.Caret.Line,
				Column:  loc.Caret.Column,
				Message: d.Message,
				Level:   "error",
			})
		}
	}
	return diags, nil
}
