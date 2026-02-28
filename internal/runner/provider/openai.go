package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vailang/vai/internal/config"
)

const openaiDefaultEndpoint = "https://api.openai.com/v1/chat/completions"

type openaiProvider struct {
	client     *http.Client
	apiKey     string
	baseURL    string
	model      string
	maxTokens  int
	maxRetries int
	retryDelay time.Duration
}

func newOpenAI(cfg config.LLMConfig) *openaiProvider {
	envVar := cfg.EnvTokenVariableName
	if envVar == "" {
		envVar = "OPENAI_API_KEY"
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = openaiDefaultEndpoint
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	return &openaiProvider{
		client:     newHTTPClient(),
		apiKey:     os.Getenv(envVar),
		baseURL:    baseURL,
		model:      cfg.Model,
		maxTokens:  maxTokens,
		maxRetries: cfg.MaxRetries,
		retryDelay: time.Duration(cfg.DelayRetrySeconds) * time.Second,
	}
}

// OpenAI request/response types.
type openaiRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Messages  []openaiMsg   `json:"messages"`
	Tools     []openaiTool  `json:"tools,omitempty"`
}

type openaiMsg struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type openaiChoice struct {
	Message openaiChoiceMsg `json:"message"`
}

type openaiChoiceMsg struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (p *openaiProvider) Call(ctx context.Context, req Request) (*Response, error) {
	// Build messages: system message first, then user messages.
	var msgs []openaiMsg
	if req.System != "" {
		msgs = append(msgs, openaiMsg{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		msg := openaiMsg{Role: m.Role, Content: m.Content}
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tc.Name, Arguments: tc.Input},
				})
			}
		}
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}
		msgs = append(msgs, msg)
	}

	var tools []openaiTool
	for _, t := range req.Tools {
		tools = append(tools, openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	body := openaiRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  msgs,
		Tools:     tools,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling openai request: %w", err)
	}

	endpoint := p.baseURL
	if !strings.Contains(endpoint, "/chat/completions") && !strings.HasSuffix(endpoint, "/") {
		endpoint = strings.TrimRight(endpoint, "/") + "/v1/chat/completions"
	}

	httpResp, err := retryDo(ctx, p.maxRetries, p.retryDelay, func() (*http.Response, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
		return p.client.Do(httpReq)
	})
	if err != nil {
		return nil, fmt.Errorf("openai API call: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading openai response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error (HTTP %d): %s", httpResp.StatusCode, string(respBody))
	}

	var or openaiResponse
	if err := json.Unmarshal(respBody, &or); err != nil {
		return nil, fmt.Errorf("parsing openai response: %w", err)
	}
	if or.Error != nil {
		return nil, fmt.Errorf("openai error: %s", or.Error.Message)
	}

	resp := &Response{
		TokensIn:  or.Usage.PromptTokens,
		TokensOut: or.Usage.CompletionTokens,
	}

	if len(or.Choices) > 0 {
		msg := or.Choices[0].Message
		resp.Content = msg.Content
		for _, tc := range msg.ToolCalls {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}
	}

	return resp, nil
}
