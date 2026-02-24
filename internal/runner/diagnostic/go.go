package diagnostic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var goErrorRe = regexp.MustCompile(`^(.+?):(\d+):(\d+): (.+)$`)

// GoParser parses `go build -json` NDJSON output.
// Each line is a JSON object with Action and Output fields.
// Lines with Action=="build-output" contain the raw error text in Output.
type GoParser struct{}

type goBuildEvent struct {
	Action string `json:"Action"`
	Output string `json:"Output"`
}

func (p *GoParser) Parse(output []byte) ([]Diagnostic, error) {
	var diags []Diagnostic
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var evt goBuildEvent
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}
		if evt.Action != "build-output" {
			continue
		}
		m := goErrorRe.FindStringSubmatch(strings.TrimRight(evt.Output, "\n"))
		if m == nil {
			continue
		}
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		diags = append(diags, Diagnostic{
			File:    m[1],
			Line:    line,
			Column:  col,
			Message: m[4],
			Level:   "error",
		})
	}
	return diags, nil
}
