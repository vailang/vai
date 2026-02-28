package prompter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vailang/vai/internal/compiler"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/runner/provider"
)

const maxTurns = 25

// Prompter orchestrates the LLM-driven plan creation workflow.
type Prompter struct {
	cfg      *config.Config
	baseDir  string
	provider provider.Provider
	server   *ToolServer
	tools    []provider.ToolDefinition
	events   chan Event
}

// SetEvents sets the event channel for progress reporting.
// Must be called before Run. The caller is responsible for consuming the channel.
func (p *Prompter) SetEvents(ch chan Event) {
	p.events = ch
}

// emit sends an event if a channel is set.
func (p *Prompter) emit(e Event) {
	if p.events != nil {
		p.events <- e
	}
}

// toolCallSummary returns a short human-readable description of a tool call.
func toolCallSummary(tc provider.ToolCall) string {
	var input map[string]any
	if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
		return tc.Name
	}
	if n, ok := input["name"].(string); ok {
		return fmt.Sprintf("%s %q", tc.Name, n)
	}
	return tc.Name
}

// New creates a Prompter for the given project.
func New(cfg *config.Config, baseDir string) (*Prompter, error) {
	prov, err := provider.New(cfg.Planner)
	if err != nil {
		return nil, fmt.Errorf("creating planner provider: %w", err)
	}
	return &Prompter{
		cfg:      cfg,
		baseDir:  baseDir,
		provider: prov,
		server:   NewToolServer(baseDir, cfg),
		tools:    ToolDefinitions(),
	}, nil
}

// Run executes the prompter loop: sends the user's prompt to the LLM,
// processes tool calls, and returns the result with all file changes.
func (p *Prompter) Run(ctx context.Context, userPrompt string) (*Result, error) {
	system, err := p.buildSystemPrompt()
	if err != nil {
		return nil, err
	}

	messages := []provider.Message{
		{Role: "user", Content: userPrompt},
	}

	result := &Result{}

	for turn := 0; turn < maxTurns; turn++ {
		p.emit(Event{Kind: EventTurnStart, Turn: turn + 1})

		resp, err := p.provider.Call(ctx, provider.Request{
			System:    system,
			Messages:  messages,
			Tools:     p.tools,
			MaxTokens: p.cfg.Planner.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("LLM call failed (turn %d): %w", turn+1, err)
		}

		result.TokensIn += resp.TokensIn
		result.TokensOut += resp.TokensOut

		p.emit(Event{
			Kind:           EventTurnComplete,
			Turn:           turn + 1,
			TokensIn:       resp.TokensIn,
			TokensOut:      resp.TokensOut,
			TotalTokensIn:  result.TokensIn,
			TotalTokensOut: result.TokensOut,
		})

		if resp.Content != "" {
			p.emit(Event{
				Kind:    EventLLMText,
				Turn:    turn + 1,
				Content: resp.Content,
			})
		}

		if len(resp.ToolCalls) == 0 {
			// LLM is done.
			result.Summary = resp.Content
			result.Changes = p.server.Changes()
			p.emit(Event{
				Kind:           EventDone,
				Turn:           turn + 1,
				TotalTokensIn:  result.TokensIn,
				TotalTokensOut: result.TokensOut,
			})
			return result, nil
		}

		// Append assistant message with tool calls.
		messages = append(messages, provider.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results.
		for _, tc := range resp.ToolCalls {
			p.emit(Event{
				Kind:     EventToolCall,
				Turn:     turn + 1,
				ToolName: tc.Name,
				Message:  toolCallSummary(tc),
				Content:  tc.Input,
			})
			toolResult := p.server.Execute(tc.Name, tc.Input)
			p.emit(Event{
				Kind:     EventToolResult,
				Turn:     turn + 1,
				ToolName: tc.Name,
				Content:  toolResult,
			})
			messages = append(messages, provider.Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
			})
		}
	}

	// Exceeded max turns — return what we have.
	result.Changes = p.server.Changes()
	p.emit(Event{
		Kind:           EventDone,
		Turn:           maxTurns,
		TotalTokensIn:  result.TokensIn,
		TotalTokensOut: result.TokensOut,
	})
	return result, fmt.Errorf("prompter exceeded maximum turns (%d)", maxTurns)
}

// buildSystemPrompt creates the system prompt for the prompter LLM.
// It tries to load from the standard library first, falls back to a built-in.
func (p *Prompter) buildSystemPrompt() (string, error) {
	// Try to load via compiler's standard library.
	comp := compiler.New()
	comp.SetBaseDir(p.baseDir)

	// Parse with empty sources to get access to std library.
	prog, errs := comp.ParseSources(map[string]string{})
	if len(errs) == 0 && prog.HasPrompt("std.vai_prompter") {
		result, err := prog.Eval("inject std.vai_prompter")
		if err == nil && result != "" {
			return result, nil
		}
	}

	// Fall back to built-in system prompt.
	return builtinSystemPrompt, nil
}

const builtinSystemPrompt = `You are a project planner. You receive a user request and adapt plan specifications to match it.

WORKFLOW:
1. Call list_plans to see available plans
2. If existing plans cover the request:
   - Call read_spec to read their current specifications
   - Call update_spec with the modified specification
3. If new plans are needed:
   - Call create_plans to create new plans with specs
   - Use short, descriptive names without spaces (e.g. auth_handler, data_pipeline)
   - Names should reflect the role and domain of the plan
   - You can create multiple plans at once for a multi-plan architecture

RULES:
- Do NOT invent requirements beyond what the user asked.
- Keep specs concise: describe what, not how.
- Preserve existing spec content that is still relevant.
- When done, stop calling tools and provide a brief summary.
`
