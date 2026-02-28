package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/vailang/vai/internal/config"
)

const geminiEndpointFmt = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

type geminiProvider struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	model      string
	maxTokens  int
	maxRetries int
	retryDelay time.Duration
}

func newGemini(cfg config.LLMConfig) *geminiProvider {
	envVar := cfg.EnvTokenVariableName
	if envVar == "" {
		envVar = "GEMINI_API_KEY"
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	return &geminiProvider{
		client:     newHTTPClient(),
		baseURL:    cfg.BaseURL,
		apiKey:     os.Getenv(envVar),
		model:      cfg.Model,
		maxTokens:  maxTokens,
		maxRetries: cfg.MaxRetries,
		retryDelay: time.Duration(cfg.DelayRetrySeconds) * time.Second,
	}
}

// Gemini request/response types.
type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	SystemInstruction *geminiContent `json:"systemInstruction,omitempty"`
	Tools            []geminiTool   `json:"tools,omitempty"`
	GenerationConfig *geminiGenCfg  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFuncCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse `json:"functionResponse,omitempty"`
}

type geminiFuncResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiFuncCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiGenCfg struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

func (p *geminiProvider) Call(ctx context.Context, req Request) (*Response, error) {
	var contents []geminiContent
	for _, m := range req.Messages {
		switch {
		case len(m.ToolCalls) > 0:
			// Assistant message with tool calls → model message with functionCall parts.
			var parts []geminiPart
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFuncCall{
						Name: tc.Name,
						Args: json.RawMessage(tc.Input),
					},
				})
			}
			contents = append(contents, geminiContent{Role: "model", Parts: parts})

		case m.Role == "tool":
			// Tool result → user message with functionResponse part.
			contents = append(contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFuncResponse{
						Name:     m.ToolCallID,
						Response: json.RawMessage(`{"result":` + strconv.Quote(m.Content) + `}`),
					},
				}},
			})

		default:
			role := m.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}

	var sysInstr *geminiContent
	if req.System != "" {
		sysInstr = &geminiContent{
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	var tools []geminiTool
	if len(req.Tools) > 0 {
		var decls []geminiFuncDecl
		for _, t := range req.Tools {
			decls = append(decls, geminiFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		tools = append(tools, geminiTool{FunctionDeclarations: decls})
	}

	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	body := geminiRequest{
		Contents:          contents,
		SystemInstruction: sysInstr,
		Tools:             tools,
		GenerationConfig:  &geminiGenCfg{MaxOutputTokens: maxTokens},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling gemini request: %w", err)
	}

	endpoint := fmt.Sprintf(geminiEndpointFmt, model)
	if p.baseURL != "" {
		endpoint = p.baseURL
	}

	httpResp, err := retryDo(ctx, p.maxRetries, p.retryDelay, func() (*http.Response, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"?key="+p.apiKey, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		return p.client.Do(httpReq)
	})
	if err != nil {
		return nil, fmt.Errorf("gemini API call: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading gemini response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error (HTTP %d): %s", httpResp.StatusCode, string(respBody))
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return nil, fmt.Errorf("parsing gemini response: %w", err)
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("gemini error: %s", gr.Error.Message)
	}

	resp := &Response{}
	if gr.UsageMetadata != nil {
		resp.TokensIn = gr.UsageMetadata.PromptTokenCount
		resp.TokensOut = gr.UsageMetadata.CandidatesTokenCount
	}

	if len(gr.Candidates) > 0 {
		for _, part := range gr.Candidates[0].Content.Parts {
			if part.Text != "" {
				resp.Content += part.Text
			}
			if part.FunctionCall != nil {
				resp.ToolCalls = append(resp.ToolCalls, ToolCall{
					Name:  part.FunctionCall.Name,
					Input: string(part.FunctionCall.Args),
				})
			}
		}
	}

	return resp, nil
}
