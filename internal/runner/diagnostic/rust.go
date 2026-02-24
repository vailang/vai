package diagnostic

import (
	"bufio"
	"bytes"
	"encoding/json"
)

// RustParser parses `cargo build --message-format=json` NDJSON output.
type RustParser struct{}

type cargoMessage struct {
	Reason  string      `json:"reason"`
	Message rustMessage `json:"message"`
}

type rustMessage struct {
	Level   string     `json:"level"`
	Message string     `json:"message"`
	Spans   []rustSpan `json:"spans"`
}

type rustSpan struct {
	FileName  string `json:"file_name"`
	LineStart int    `json:"line_start"`
	ColStart  int    `json:"column_start"`
	IsPrimary bool   `json:"is_primary"`
}

func (p *RustParser) Parse(output []byte) ([]Diagnostic, error) {
	var diags []Diagnostic
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var msg cargoMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if msg.Reason != "compiler-message" {
			continue
		}
		if msg.Message.Level != "error" {
			continue
		}
		for _, span := range msg.Message.Spans {
			if !span.IsPrimary {
				continue
			}
			diags = append(diags, Diagnostic{
				File:    span.FileName,
				Line:    span.LineStart,
				Column:  span.ColStart,
				Message: msg.Message.Message,
				Level:   "error",
			})
		}
	}
	return diags, nil
}
