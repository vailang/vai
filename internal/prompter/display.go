package prompter

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Display consumes prompter events and prints terminal progress.
type Display struct {
	verbose   bool
	hasStatus bool
}

// NewDisplay creates a Display. When verbose is true, all events are printed
// permanently with full detail instead of transient status lines.
func NewDisplay(verbose bool) *Display {
	return &Display{verbose: verbose}
}

// Consume reads all events from the channel and prints progress.
// Blocks until the channel is closed.
func (d *Display) Consume(events <-chan Event) {
	fmt.Fprintln(os.Stderr, "==> Prompter: analyzing project...")
	for ev := range events {
		if d.verbose {
			d.printVerbose(ev)
		} else {
			d.printEvent(ev)
		}
	}
}

func (d *Display) printEvent(ev Event) {
	switch ev.Kind {
	case EventTurnStart:
		d.printStatus(fmt.Sprintf("turn %d: calling LLM...", ev.Turn))
	case EventTurnComplete:
		d.printStatus(fmt.Sprintf("turn %d: %d in / %d out (total: %d / %d)",
			ev.Turn, ev.TokensIn, ev.TokensOut, ev.TotalTokensIn, ev.TotalTokensOut))
	case EventToolCall:
		d.printStatus(fmt.Sprintf("turn %d: %s", ev.Turn, ev.Message))
	case EventDone:
		d.clearStatus()
		fmt.Fprintf(os.Stderr, "==> Prompter: done (%d turns, %d in / %d out)\n",
			ev.Turn, ev.TotalTokensIn, ev.TotalTokensOut)
	}
}

func (d *Display) printVerbose(ev Event) {
	switch ev.Kind {
	case EventTurnStart:
		fmt.Fprintf(os.Stderr, "--- turn %d: calling LLM...\n", ev.Turn)
	case EventTurnComplete:
		fmt.Fprintf(os.Stderr, "--- turn %d: %d in / %d out (total: %d / %d)\n",
			ev.Turn, ev.TokensIn, ev.TokensOut, ev.TotalTokensIn, ev.TotalTokensOut)
	case EventLLMText:
		text := ev.Content
		if len(text) > 120 {
			text = text[:120] + "..."
		}
		text = strings.ReplaceAll(text, "\n", " ")
		fmt.Fprintf(os.Stderr, "--- turn %d: LLM: %s\n", ev.Turn, text)
	case EventToolCall:
		fmt.Fprintf(os.Stderr, "--- turn %d: tool call: %s\n", ev.Turn, ev.Message)
		d.printToolInput(ev)
	case EventToolResult:
		d.printToolResultVerbose(ev)
	case EventDone:
		fmt.Fprintf(os.Stderr, "==> Prompter: done (%d turns, %d in / %d out)\n",
			ev.Turn, ev.TotalTokensIn, ev.TotalTokensOut)
	}
}

// printToolInput shows relevant content from the tool call input.
func (d *Display) printToolInput(ev Event) {
	if ev.Content == "" {
		return
	}
	switch ev.ToolName {
	case "update_spec":
		// Show line count of the content being written.
		var input struct {
			Content string `json:"content"`
		}
		if err := parseJSON(ev.Content, &input); err == nil && input.Content != "" {
			lines := strings.Count(input.Content, "\n") + 1
			fmt.Fprintf(os.Stderr, "    content (%d lines)\n", lines)
		}
	}
}

// printToolResultVerbose shows tool result info.
func (d *Display) printToolResultVerbose(ev Event) {
	result := ev.Content
	if result == "ok" {
		fmt.Fprintf(os.Stderr, "    result: ok\n")
		return
	}
	if strings.HasPrefix(result, "error:") {
		fmt.Fprintf(os.Stderr, "    result: %s\n", result)
		return
	}
	fmt.Fprintf(os.Stderr, "    result (%d chars)\n", len(result))
}

func (d *Display) printStatus(msg string) {
	d.clearStatus()
	fmt.Fprintf(os.Stderr, "\r    %s", msg)
	d.hasStatus = true
}

func (d *Display) clearStatus() {
	if d.hasStatus {
		fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 72))
		d.hasStatus = false
	}
}

// parseJSON is a helper to unmarshal JSON into a struct.
func parseJSON(raw string, v any) error {
	return json.Unmarshal([]byte(raw), v)
}
