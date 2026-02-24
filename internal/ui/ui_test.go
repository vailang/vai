package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/vailang/vai/internal/runner"
)

func TestTerminalModeStepStart(t *testing.T) {
	out := captureStdout(func() {
		u := New(ModeTerminal)
		events := make(chan runner.Event, 2)
		events <- runner.Event{Kind: runner.EventStepStart, Step: "architect", Message: "planning skeleton..."}
		events <- runner.Event{Kind: runner.EventDone}
		close(events)
		u.Consume(events)
	})

	if !strings.Contains(out, "==> architect: planning skeleton...") {
		t.Errorf("expected step start output, got: %q", out)
	}
	if !strings.Contains(out, "==> Done.") {
		t.Errorf("expected done output, got: %q", out)
	}
}

func TestTerminalModeImplStart(t *testing.T) {
	out := captureStdout(func() {
		u := New(ModeTerminal)
		events := make(chan runner.Event, 1)
		events <- runner.Event{Kind: runner.EventImplStart, Step: "executor", Name: "plan.add"}
		close(events)
		u.Consume(events)
	})

	if !strings.Contains(out, "executor: plan.add") {
		t.Errorf("expected impl start output, got: %q", out)
	}
}

func TestTerminalModeSummary(t *testing.T) {
	out := captureStdout(func() {
		u := New(ModeTerminal)
		events := make(chan runner.Event, 1)
		events <- runner.Event{
			Kind: runner.EventSummary,
			Data: runner.RunStats{
				ArchitectStatus:    runner.StatusComplete,
				ArchitectTokensIn:  1000,
				ArchitectTokensOut: 500,
				ArchitectCycles:    1,
			},
		}
		close(events)
		u.Consume(events)
	})

	if !strings.Contains(out, "Summary") {
		t.Errorf("expected summary header, got: %q", out)
	}
	if !strings.Contains(out, "Architect") {
		t.Errorf("expected Architect row, got: %q", out)
	}
}

func TestJSONModeOutput(t *testing.T) {
	out := captureStdout(func() {
		u := New(ModeJSON)
		events := make(chan runner.Event, 3)
		events <- runner.Event{Kind: runner.EventStepStart, Step: "architect", Message: "planning..."}
		events <- runner.Event{
			Kind: runner.EventSummary,
			Data: runner.RunStats{
				ArchitectStatus:    runner.StatusComplete,
				ArchitectTokensIn:  1000,
				ArchitectTokensOut: 500,
				ArchitectCycles:    1,
			},
		}
		events <- runner.Event{Kind: runner.EventDone}
		close(events)
		u.Consume(events)
	})

	var result jsonOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, out)
	}
	if len(result.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(result.Events))
	}
	if result.Stats == nil {
		t.Fatal("expected stats in JSON output")
	}
	if result.Stats.ArchitectTokensIn != 1000 {
		t.Errorf("expected ArchitectTokensIn=1000, got %d", result.Stats.ArchitectTokensIn)
	}
}

func TestJSONModeNoTerminalOutput(t *testing.T) {
	out := captureStdout(func() {
		u := New(ModeJSON)
		events := make(chan runner.Event, 1)
		events <- runner.Event{Kind: runner.EventStepStart, Step: "architect", Message: "planning..."}
		close(events)
		u.Consume(events)
	})

	// Should be valid JSON, not terminal text
	if strings.Contains(out, "==>") {
		t.Errorf("JSON mode should not produce terminal formatting, got: %q", out)
	}
	var result jsonOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON mode output is not valid JSON: %v", err)
	}
}

// captureStdout captures stdout during the execution of fn.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
