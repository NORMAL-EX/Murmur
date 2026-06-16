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
	Role    string `json:"role"`
	Content string `json:"content"`
}

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
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends a full message list (system prompt is prepended automatically)
// and returns the assistant reply text.
func (s *Service) Complete(ctx context.Context, history []Message) (string, error) {
	key := strings.TrimSpace(s.st.Get(settings.AIAPIKey))
	if key == "" {
		return "", errors.New("AI API key 未配置")
	}
	base := strings.TrimRight(strings.TrimSpace(s.st.Get(settings.AIBaseURL)), "/")
	if base == "" {
		return "", errors.New("AI base URL 未配置")
	}
	endpoint := base + "/chat/completions"

	messages := make([]Message, 0, len(history)+1)
	if sp := strings.TrimSpace(s.st.Get(settings.AISystemPrompt)); sp != "" {
		messages = append(messages, Message{Role: "system", Content: sp})
	}
	messages = append(messages, history...)

	reqBody := chatRequest{
		Model:       s.st.Get(settings.AIModel),
		Messages:    messages,
		Temperature: s.st.GetFloat(settings.AITemperature),
		MaxTokens:   s.st.GetInt(settings.AIMaxTokens),
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var cr chatResponse
		if json.Unmarshal(body, &cr) == nil && cr.Error != nil && cr.Error.Message != "" {
			return "", fmt.Errorf("LLM 接口错误 (%d): %s", resp.StatusCode, cr.Error.Message)
		}
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return "", fmt.Errorf("LLM 接口返回 %d: %s", resp.StatusCode, snippet)
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return "", errors.New(cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", errors.New("LLM 未返回任何内容")
	}
	reply := strings.TrimSpace(cr.Choices[0].Message.Content)
	if reply == "" {
		return "", errors.New("LLM 返回空内容")
	}
	return reply, nil
}

// Test performs a minimal round-trip to validate connectivity & credentials.
func (s *Service) Test(ctx context.Context) (string, error) {
	return s.Complete(ctx, []Message{{Role: "user", Content: "你好,请用一句话简单回复以确认连通。"}})
}
