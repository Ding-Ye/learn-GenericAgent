package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicProvider talks to https://api.anthropic.com/v1/messages over raw
// HTTP — same approach as upstream's NativeClaudeSession. We use Server-Sent
// Events (SSE) streaming so chunks flow as the model generates them.
//
// Upstream parallel:
//   llmcore.py:NativeClaudeSession.raw_ask uses requests.post(url, stream=True)
//   and iter_lines() to walk SSE. Our Go version uses bufio.Scanner.
type AnthropicProvider struct {
	APIKey     string
	Model      string // e.g. "claude-haiku-4-5-20251001"
	MaxTokens  int    // e.g. 4096
	HTTPClient *http.Client
	Endpoint   string // override for testing; default https://api.anthropic.com/v1/messages
}

// NewAnthropicProvider returns a provider with sensible defaults.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		APIKey:     apiKey,
		Model:      model,
		MaxTokens:  4096,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		Endpoint:   "https://api.anthropic.com/v1/messages",
	}
}

type anthropicReqMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicReqTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicReq struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicReqMsg  `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Tools     []anthropicReqTool `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
}

// toAnthropic converts our flat Messages into Anthropic's content-block shape.
//
//   role: "user" | "assistant"
//   content: [{type:"text", text:"..."} | {type:"tool_use", id, name, input}
//             | {type:"tool_result", tool_use_id, content}]
//
// We extract role="system" out as the top-level `system` field.
func toAnthropic(msgs []Message) (system string, out []anthropicReqMsg) {
	for _, m := range msgs {
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n"
			}
			system += m.Content

		case "user":
			out = append(out, anthropicReqMsg{
				Role:    "user",
				Content: []map[string]any{{"type": "text", "text": m.Content}},
			})

		case "assistant":
			blocks := []map[string]any{}
			if m.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": tc.Args,
				})
			}
			out = append(out, anthropicReqMsg{Role: "assistant", Content: blocks})

		case "tool":
			// tool_result blocks must live inside a "user" message in the
			// Anthropic schema.
			out = append(out, anthropicReqMsg{
				Role: "user",
				Content: []map[string]any{{
					"type":         "tool_result",
					"tool_use_id":  m.ToolUseID,
					"content":      m.Content,
				}},
			})
		}
	}
	return
}

// Chat sends one request and streams text chunks via `chunks`. Returns the
// full assembled Response when SSE terminates.
//
// Upstream parallel: llmcore.py:_parse_claude_sse — same SSE event names.
func (a *AnthropicProvider) Chat(ctx context.Context, msgs []Message, tools []ToolSpec, chunks chan<- string) (Response, error) {
	system, reqMsgs := toAnthropic(msgs)
	reqTools := make([]anthropicReqTool, 0, len(tools))
	for _, t := range tools {
		reqTools = append(reqTools, anthropicReqTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	body, err := json.Marshal(anthropicReq{
		Model:     a.Model,
		System:    system,
		Messages:  reqMsgs,
		MaxTokens: a.MaxTokens,
		Tools:     reqTools,
		Stream:    true,
	})
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return Response{}, fmt.Errorf("api status %d: %s", resp.StatusCode, string(raw))
	}

	return parseSSE(resp.Body, chunks)
}

// parseSSE walks the Anthropic streaming protocol and reassembles a Response.
// The protocol shape (see https://docs.anthropic.com/en/api/messages-streaming):
//
//   event: message_start          → preamble
//   event: content_block_start    → starts a text or tool_use block
//   event: content_block_delta    → text delta or input_json_delta
//   event: content_block_stop     → ends current block
//   event: message_delta          → stop_reason + usage
//   event: message_stop           → SSE done
//   event: ping                   → ignore
//   event: error                  → server-side error
func parseSSE(r io.Reader, chunks chan<- string) (Response, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var (
		out          Response
		blockType    string // "text" | "tool_use"
		blockText    strings.Builder
		blockJSON    strings.Builder
		blockToolID  string
		blockToolNm  string
	)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue // skip malformed lines, matches upstream tolerance
		}
		switch ev["type"] {
		case "content_block_start":
			cb, _ := ev["content_block"].(map[string]any)
			blockType, _ = cb["type"].(string)
			blockText.Reset()
			blockJSON.Reset()
			if blockType == "tool_use" {
				blockToolID, _ = cb["id"].(string)
				blockToolNm, _ = cb["name"].(string)
			}

		case "content_block_delta":
			d, _ := ev["delta"].(map[string]any)
			switch d["type"] {
			case "text_delta":
				if t, ok := d["text"].(string); ok {
					blockText.WriteString(t)
					select {
					case chunks <- t:
					default:
					}
				}
			case "input_json_delta":
				if pj, ok := d["partial_json"].(string); ok {
					blockJSON.WriteString(pj)
				}
			}

		case "content_block_stop":
			switch blockType {
			case "text":
				out.Content += blockText.String()
			case "tool_use":
				args := map[string]any{}
				_ = json.Unmarshal([]byte(blockJSON.String()), &args)
				out.ToolCalls = append(out.ToolCalls, ToolCall{
					ID: blockToolID, Name: blockToolNm, Args: args,
				})
			}

		case "message_stop":
			return out, nil

		case "error":
			eb, _ := json.Marshal(ev["error"])
			return out, fmt.Errorf("anthropic api error: %s", string(eb))
		}
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	return out, nil
}
