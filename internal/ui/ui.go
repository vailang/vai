package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vailang/vai/internal/runner"
)

// OutputMode selects the output format.
type OutputMode int

const (
	ModeTerminal OutputMode = iota // formatted terminal output
	ModeJSON                       // single JSON object at end
)

// UI consumes runner events and produces formatted output.
type UI struct {
	mode      OutputMode
	events    []runner.Event // collected for JSON mode
	hasStatus bool          // true if a transient status line is currently displayed
}

// New creates a UI with the specified output mode.
func New(mode OutputMode) *UI {
	return &UI{mode: mode}
}

// Consume reads all events from the channel and produces output.
func (u *UI) Consume(events <-chan runner.Event) {
	for ev := range events {
		u.events = append(u.events, ev)
		if u.mode == ModeTerminal {
			u.printTerminal(ev)
		}
	}
	if u.mode == ModeJSON {
		u.outputJSON()
	}
}

// printStatus writes a transient status line that will be overwritten by the next event.
func (u *UI) printStatus(msg string) {
	u.clearStatus()
	fmt.Printf("\r    %s", msg)
	u.hasStatus = true
}

// clearStatus erases the current transient status line.
func (u *UI) clearStatus() {
	if u.hasStatus {
		fmt.Printf("\r%s\r", strings.Repeat(" ", 72))
		u.hasStatus = false
	}
}

// printTerminal formats and prints a single event to the terminal.
func (u *UI) printTerminal(ev runner.Event) {
	switch ev.Kind {
	case runner.EventStepStart:
		u.clearStatus()
		fmt.Printf("==> %s: %s\n", ev.Step, ev.Message)
	case runner.EventStepComplete:
		u.clearStatus()
	case runner.EventStepFailed:
		u.clearStatus()
		fmt.Fprintf(os.Stderr, "==> %s: FAILED: %s\n", ev.Step, ev.Message)
	case runner.EventInfo:
		u.printStatus(ev.Message)
	case runner.EventWarning:
		u.clearStatus()
		fmt.Fprintf(os.Stderr, "    warning: %s\n", ev.Message)
	case runner.EventImplStart:
		u.printStatus("executor: " + ev.Name)
	case runner.EventSkeleton:
		u.clearStatus()
		if skel, ok := ev.Data.(runner.SkeletonData); ok {
			fmt.Printf("    skeleton for %q: %d imports, %d declarations, %d impls\n",
				ev.Name, skel.ImportCount, skel.DeclCount, skel.ImplCount)
		}
	case runner.EventSummary:
		u.clearStatus()
		if stats, ok := ev.Data.(runner.RunStats); ok {
			u.printSummaryTable(stats)
		}
	case runner.EventDone:
		u.clearStatus()
		fmt.Println("==> Done.")
	}
}

// printSummaryTable prints the token usage summary table.
func (u *UI) printSummaryTable(s runner.RunStats) {
	fmt.Println("\n==> Summary")
	fmt.Println("    Step                 Status      Cycles   Tokens In   Tokens Out")
	fmt.Println("    -------------------  ----------  ------   ---------   ----------")

	archStatus := s.ArchitectStatus
	if archStatus == "" {
		archStatus = runner.StatusSkipped
	}
	fmt.Printf("    %-20s %-10s  %6d   %9d   %10d\n",
		"Architect", archStatus, s.ArchitectCycles,
		s.ArchitectTokensIn, s.ArchitectTokensOut)

	execIn, execOut := 0, 0
	execFailed := 0
	for _, is := range s.ImplStats {
		execIn += is.TokensIn
		execOut += is.TokensOut
		if is.Status == runner.StatusFailed {
			execFailed++
		}
	}
	execStatus := runner.StepStatus(runner.StatusComplete)
	if execFailed > 0 {
		execStatus = runner.StatusFailed
	}
	if len(s.ImplStats) == 0 {
		execStatus = runner.StatusSkipped
	}
	fmt.Printf("    %-20s %-10s  %6d   %9d   %10d\n",
		"Executor", execStatus, len(s.ImplStats), execIn, execOut)
	for _, is := range s.ImplStats {
		fmt.Printf("      - %-18s %-10s           %9d   %10d\n",
			is.Name, is.Status, is.TokensIn, is.TokensOut)
	}

	if s.DebugCalls > 0 || s.DebugStatus != "" {
		debugStatus := s.DebugStatus
		if debugStatus == "" {
			debugStatus = runner.StatusSkipped
		}
		fmt.Printf("    %-20s %-10s  %6d   %9d   %10d\n",
			"Debug", debugStatus, s.DebugCalls,
			s.DebugTokensIn, s.DebugTokensOut)
	}

	// Cache stats.
	cached := s.CachedPlans + s.CachedImpls
	if cached > 0 {
		cachedLabel := fmt.Sprintf("%d cached", cached)
		savedLabel := ""
		if s.SavedTokensEstimate != "" {
			savedLabel = s.SavedTokensEstimate + " saved"
		}
		fmt.Printf("    %-20s             %s              %s\n",
			"Cache", cachedLabel, savedLabel)
	}

	fmt.Println("    -------------------  ----------  ------   ---------   ----------")
	totalCycles := s.ArchitectCycles + len(s.ImplStats) + s.DebugCalls
	totalIn := s.ArchitectTokensIn + execIn + s.DebugTokensIn
	totalOut := s.ArchitectTokensOut + execOut + s.DebugTokensOut
	fmt.Printf("    %-20s             %6d   %9d   %10d\n",
		"Total", totalCycles, totalIn, totalOut)
}

// jsonOutput is the JSON output structure.
type jsonOutput struct {
	Events []jsonEvent      `json:"events"`
	Stats  *runner.RunStats `json:"stats,omitempty"`
}

// jsonEvent is a simplified event for JSON output.
type jsonEvent struct {
	Kind    runner.EventKind `json:"kind"`
	Step    string           `json:"step,omitempty"`
	Name    string           `json:"name,omitempty"`
	Message string           `json:"message,omitempty"`
}

// outputJSON writes all collected events as a single JSON object.
func (u *UI) outputJSON() {
	out := jsonOutput{}

	for _, ev := range u.events {
		out.Events = append(out.Events, jsonEvent{
			Kind:    ev.Kind,
			Step:    ev.Step,
			Name:    ev.Name,
			Message: ev.Message,
		})
		// Extract stats from summary event.
		if ev.Kind == runner.EventSummary {
			if stats, ok := ev.Data.(runner.RunStats); ok {
				out.Stats = &stats
			}
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
