package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey   string
	baseURL  string
	model    string
	timeout  time.Duration
	http     *http.Client
}

func NewClient(apiKey, baseURL, model string, timeout time.Duration) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
	}
}

type ChatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []Message      `json:"messages"`
	ResponseFormat   ResponseFormat `json:"response_format,omitempty"`
	Temperature      float64        `json:"temperature,omitempty"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	TopP             float64        `json:"top_p,omitempty"`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64        `json:"presence_penalty,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Error   *Error   `json:"error,omitempty"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
	FinishReason string      `json:"finish_reason"`
	LogProbs     interface{} `json:"logprobs,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (c *Client) GenerateJSON(ctx context.Context, systemPrompt, userContent string) (string, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userContent},
	}

	reqBody := ChatCompletionRequest{
		Model: c.model,
		Messages: messages,
		ResponseFormat: ResponseFormat{
			Type: "json_object",
		},
		Temperature: 0.7,
		TopP:        1,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp Error
		if json.Unmarshal(body, &errResp) == nil {
			return "", fmt.Errorf("groq api error [%s]: %s - Code: %s", errResp.Type, errResp.Message, errResp.Code)
		}
		return "", fmt.Errorf("groq api error: status %d: %s", resp.StatusCode, string(body))
	}

	var parsed ChatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("groq error: %s (type: %s, code: %s)", parsed.Error.Message, parsed.Error.Type, parsed.Error.Code)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}
