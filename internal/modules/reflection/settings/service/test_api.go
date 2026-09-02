package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"venturo-skeleton-go/pkg/groq"
)

var ErrTestConnectionFailed = errors.New("failed to connect to Groq API")

// TestGroqConnection tests if the Groq API is accessible with the provided credentials
func TestGroqConnection(apiKey, baseURL, model string, timeout time.Duration) error {
	if apiKey == "" {
		return errors.New("API key cannot be empty")
	}
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	if model == "" {
		model = "openai/gpt-oss-120b"
	}

	client := groq.NewClient(apiKey, baseURL, model, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	simplePrompt := `You are a health checker. Reply with exactly 'OK' if you can respond.`
	userContent := "What is your status?"
	response, err := client.GenerateJSON(ctx, simplePrompt, userContent)
	
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTestConnectionFailed, err)
	}

	// Check if response contains "OK" or is valid JSON
	response = strings.TrimSpace(response)
	if len(response) >= 2 && strings.ToLower(response[:2]) == "ok" || (strings.HasPrefix(response, "{") && strings.HasSuffix(response, "}")) {
		return nil
	}

	return fmt.Errorf("unexpected response format: %s", response)
}
