package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/vailang/vai/internal/config"
)

const anthropicEndpoint = "https://api.anthropic.com/v1/messages"

type anthropicProvider struct {
	client     *http.Client
	endpoint   string
	apiKey     string
	model      string
	maxTokens  int
	maxRetries int
	retryDelay time.Duration
}

func newAnthropic(cfg config.LLMConfig) *anthropicProvider {
	envVar := cfg.EnvTokenVariableName
	if envVar == "" {
		envVar = "ANTHROPIC_API_KEY"
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	endpoint := anthropicEndpoint
	if cfg.BaseURL != "" {
		endpoint = cfg.BaseURL
	}
	return &anthropicProvider{
		client:     newHTTPClient(),
		endpoint:   endpoint,
		apiKey:     os.Getenv(envVar),
		model:      cfg.Model,
		maxTokens:  maxTokens,
		maxRetries: cfg.MaxRetries,
		retryDelay: time.Duration(cfg.DelayRetrySeconds) * time.Second,
	}
}

// Anthropic request/response types.
type anthropicRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system,omitempty"`
	Messages  []anthropicMsg    `json:"messages"`
	Tools     []anthropicTool   `json:"tools,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

func (p *anthropicProvider) Call(ctx context.Context, req Request) (*Response, error) {
	msgs := make([]anthropicMsg, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = anthropicMsg(m)
	}

	var tools []anthropicTool
	for _, t := range req.Tools {
		tools = append(tools, anthropicTool(t))
	}

	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	body := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.System,
		Messages:  msgs,
		Tools:     tools,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling anthropic request: %w", err)
	}

	httpResp, err := retryDo(ctx, p.maxRetries, p.retryDelay, func() (*http.Response, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", p.apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		return p.client.Do(httpReq)
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading anthropic response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error (HTTP %d): %s", httpResp.StatusCode, string(respBody))
	}

	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return nil, fmt.Errorf("parsing anthropic response: %w", err)
	}
	if ar.Error != nil {
		return nil, fmt.Errorf("anthropic error: %s", ar.Error.Message)
	}

	resp := &Response{
		TokensIn:  ar.Usage.InputTokens,
		TokensOut: ar.Usage.OutputTokens,
	}

	for _, c := range ar.Content {
		switch c.Type {
		case "text":
			resp.Content += c.Text
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    c.ID,
				Name:  c.Name,
				Input: string(c.Input),
			})
		}
	}

	return resp, nil
}
