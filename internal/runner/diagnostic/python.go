package diagnostic

import "encoding/json"

// PythonParser parses `pyright --outputjson` output.
type PythonParser struct{}

type pyrightOutput struct {
	GeneralDiagnostics []pyrightDiag `json:"generalDiagnostics"`
}

type pyrightDiag struct {
	File     string       `json:"file"`
	Severity string       `json:"severity"`
	Message  string       `json:"message"`
	Range    pyrightRange `json:"range"`
}

type pyrightRange struct {
	Start pyrightPos `json:"start"`
}

type pyrightPos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func (p *PythonParser) Parse(output []byte) ([]Diagnostic, error) {
	var pr pyrightOutput
	if err := json.Unmarshal(output, &pr); err != nil {
		return nil, nil
	}
	var diags []Diagnostic
	for _, d := range pr.GeneralDiagnostics {
		if d.Severity != "error" {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    d.File,
			Line:    d.Range.Start.Line + 1, // pyright lines are 0-indexed
			Column:  d.Range.Start.Character + 1,
			Message: d.Message,
			Level:   "error",
		})
	}
	return diags, nil
}
