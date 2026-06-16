// Package ai talks to an OpenAI-compatible Chat Completions endpoint using only
// the standard library. All parameters come from the settings service.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"murmur/settings"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDef struct {
	Type     string          `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolExecutor runs a tool call and returns a textual result for the model.
type ToolExecutor func(name, argsJSON string) string

type Service struct {
	st     *settings.Service
	client *http.Client
}

func New(st *settings.Service) *Service {
	return &Service{
		st:     st,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (s *Service) Enabled() bool { return s.st.GetBool(settings.AIEnabled) }
func (s *Service) HasKey() bool  { return strings.TrimSpace(s.st.Get(settings.AIAPIKey)) != "" }
func (s *Service) AllowDM() bool { return s.st.GetBool(settings.AIAllowDM) }

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []ToolDef `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Service) endpoint() (url, key string, err error) {
	key = strings.TrimSpace(s.st.Get(settings.AIAPIKey))
	if key == "" {
		return "", "", errors.New("AI API key 未配置")
	}
	base := strings.TrimRight(strings.TrimSpace(s.st.Get(settings.AIBaseURL)), "/")
	if base == "" {
		return "", "", errors.New("AI base URL 未配置")
	}
	return base + "/chat/completions", key, nil
}

func (s *Service) withSystem(history []Message) []Message {
	messages := make([]Message, 0, len(history)+1)
	if sp := strings.TrimSpace(s.st.Get(settings.AISystemPrompt)); sp != "" {
		messages = append(messages, Message{Role: "system", Content: sp})
	}
	return append(messages, history...)
}

// post performs one chat-completions round-trip.
func (s *Service) post(ctx context.Context, messages []Message, tools []ToolDef) (*chatResponse, error) {
	endpoint, key, err := s.endpoint()
	if err != nil {
		return nil, err
	}
	reqBody := chatRequest{
		Model:       s.st.Get(settings.AIModel),
		Messages:    messages,
		Temperature: s.st.GetFloat(settings.AITemperature),
		MaxTokens:   s.st.GetInt(settings.AIMaxTokens),
		Tools:       tools,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var cr chatResponse
		if json.Unmarshal(body, &cr) == nil && cr.Error != nil && cr.Error.Message != "" {
			return nil, fmt.Errorf("LLM 接口错误 (%d): %s", resp.StatusCode, cr.Error.Message)
		}
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("LLM 接口返回 %d: %s", resp.StatusCode, snippet)
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return nil, errors.New(cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return nil, errors.New("LLM 未返回任何内容")
	}
	return &cr, nil
}

// Complete sends a full message list (system prompt is prepended automatically)
// and returns the assistant reply text.
func (s *Service) Complete(ctx context.Context, history []Message) (string, error) {
	cr, err := s.post(ctx, s.withSystem(history), nil)
	if err != nil {
		return "", err
	}
	reply := strings.TrimSpace(cr.Choices[0].Message.Content)
	if reply == "" {
		return "", errors.New("LLM 返回空内容")
	}
	return reply, nil
}

// CompleteWithTools runs a tool-calling loop: the model may call tools (executed
// via exec), whose results are fed back until it produces a final reply.
func (s *Service) CompleteWithTools(ctx context.Context, history []Message, tools []ToolDef, exec ToolExecutor) (string, error) {
	messages := s.withSystem(history)
	for i := 0; i < 5; i++ {
		cr, err := s.post(ctx, messages, tools)
		if err != nil {
			return "", err
		}
		msg := cr.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			reply := strings.TrimSpace(msg.Content)
			if reply == "" {
				reply = "好的。"
			}
			return reply, nil
		}
		messages = append(messages, msg)
		for _, tc := range msg.ToolCalls {
			result := exec(tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
	return "", errors.New("工具调用次数过多")
}

// Test performs a minimal round-trip to validate connectivity & credentials.
func (s *Service) Test(ctx context.Context) (string, error) {
	return s.Complete(ctx, []Message{{Role: "user", Content: "你好，请用一句话简单回复以确认连通。"}})
}
