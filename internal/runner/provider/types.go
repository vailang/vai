package provider

import (
	"context"
	"encoding/json"
)

// Provider is the interface for LLM API calls.
type Provider interface {
	Call(ctx context.Context, req Request) (*Response, error)
}

// Request is sent to the provider.
type Request struct {
	System    string
	Messages  []Message
	Tools     []ToolDefinition
	Model     string
	MaxTokens int
}

// Message represents a single message in the conversation.
type Message struct {
	Role       string     // "user", "assistant", or "tool"
	Content    string
	ToolCalls  []ToolCall // set when Role="assistant" and LLM made tool calls
	ToolCallID string     // set when Role="tool" (result of a tool call)
}

// ToolDefinition describes a tool the LLM can call.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	ID    string
	Name  string
	Input string // raw JSON string
}

// Response is returned from the provider.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	TokensIn  int
	TokensOut int
}
