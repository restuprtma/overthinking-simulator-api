package gemini

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
	apiKey  string
	model   string
	timeout time.Duration
	http    *http.Client
}

func NewClient(apiKey, model string, timeout time.Duration) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
	}
}

type generateContentRequest struct {
	SystemInstruction *systemInstruction `json:"systemInstruction"`
	Contents          []content          `json:"contents"`
	GenerationConfig  *generationConfig  `json:"generationConfig"`
}

type systemInstruction struct {
	Parts []part `json:"parts"`
}

type content struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	ResponseMimeType string `json:"responseMimeType"`
}

type generateContentResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content candidateContent `json:"content"`
}

type candidateContent struct {
	Parts []part `json:"parts"`
}

func (c *Client) GenerateJSON(ctx context.Context, systemPrompt, userContent string) (string, error) {
	reqBody := generateContentRequest{
		SystemInstruction: &systemInstruction{
			Parts: []part{{Text: systemPrompt}},
		},
		Contents: []content{
			{Role: "user", Parts: []part{{Text: userContent}}},
		},
		GenerationConfig: &generationConfig{
			ResponseMimeType: "application/json",
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model, c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		return "", fmt.Errorf("gemini api error: status %d: %s", resp.StatusCode, string(body))
	}

	var parsed generateContentResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	var text string
	for _, cand := range parsed.Candidates {
		for _, p := range cand.Content.Parts {
			text += p.Text
		}
	}

	if text == "" {
		return "", fmt.Errorf("gemini returned no candidate text")
	}

	return text, nil
}
