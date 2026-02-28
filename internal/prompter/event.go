package prompter

// EventKind classifies the type of a prompter event.
type EventKind string

const (
	EventTurnStart    EventKind = "turn_start"
	EventTurnComplete EventKind = "turn_complete"
	EventToolCall     EventKind = "tool_call"
	EventToolResult   EventKind = "tool_result"
	EventLLMText      EventKind = "llm_text"
	EventDone         EventKind = "done"
)

// Event represents a single prompter lifecycle event.
type Event struct {
	Kind           EventKind `json:"kind"`
	Turn           int       `json:"turn"`
	Message        string    `json:"message,omitempty"`
	TokensIn       int       `json:"tokens_in,omitempty"`
	TokensOut      int       `json:"tokens_out,omitempty"`
	TotalTokensIn  int       `json:"total_tokens_in,omitempty"`
	TotalTokensOut int       `json:"total_tokens_out,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	Content        string    `json:"content,omitempty"`
}
