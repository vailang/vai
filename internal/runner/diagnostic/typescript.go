package diagnostic

import (
	"bufio"
	"bytes"
	"regexp"
	"strconv"
)

// TypeScript has no JSON output mode. Parse `tsc --pretty false` text output.
// Format: file(line,col): error TSxxxx: message
var tsErrorRe = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): error TS\d+: (.+)$`)

// TypeScriptParser parses `tsc --pretty false` text output via regex.
type TypeScriptParser struct{}

func (p *TypeScriptParser) Parse(output []byte) ([]Diagnostic, error) {
	var diags []Diagnostic
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		m := tsErrorRe.FindStringSubmatch(scanner.Text())
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
