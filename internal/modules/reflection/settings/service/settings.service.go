package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"venturo-skeleton-go/internal/modules/reflection/settings/repository"
	"venturo-skeleton-go/pkg/logger"
)

const GeminiCredentialsSetting = "gemini_credentials"

const defaultModel = "gemini-1.5-flash"

var ErrEmptyCredentials = errors.New("kredensial tidak boleh kosong")

// GeminiCredential represents a single API key + model pair.
type GeminiCredential struct {
	Key   string `json:"key"`
	Model string `json:"model"`
}

// settingsStore is the seam the service depends on. The real implementation
// is *repository.SettingsRepository; tests substitute an in-memory fake.
type settingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type Service struct {
	repo                settingsStore
	fallbackCredentials []GeminiCredential
}

// NewService builds a Service from a settings repository and env fallback
// values. fallbackAPIKeys[i] is paired with fallbackModels[i]; when
// fallbackModels is shorter, the missing model defaults to the default model.
func NewService(repo *repository.SettingsRepository, fallbackAPIKeys, fallbackModels []string) *Service {
	fallbackCredentials := make([]GeminiCredential, 0, len(fallbackAPIKeys))
	for i, key := range fallbackAPIKeys {
		model := defaultModel
		if i < len(fallbackModels) && strings.TrimSpace(fallbackModels[i]) != "" {
			model = strings.TrimSpace(fallbackModels[i])
		}
		fallbackCredentials = append(fallbackCredentials, GeminiCredential{
			Key:   key,
			Model: model,
		})
	}

	return &Service{
		repo:                repo,
		fallbackCredentials: fallbackCredentials,
	}
}

// GetCredentials returns the active credential list. DB value (if present)
// takes precedence; otherwise it falls back to the env-derived list.
func (s *Service) GetCredentials(ctx context.Context) ([]GeminiCredential, error) {
	raw, err := s.repo.Get(ctx, GeminiCredentialsSetting)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(raw) == "" {
		return s.fallbackCredentials, nil
	}

	var creds []GeminiCredential
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		logger.Error("Failed to unmarshal gemini credentials", logger.Err(err))
		return nil, err
	}

	for i := range creds {
		if strings.TrimSpace(creds[i].Model) == "" {
			creds[i].Model = defaultModel
		}
	}

	return creds, nil
}

// SetCredentials trims keys, drops empty-key entries, and persists the list.
func (s *Service) SetCredentials(ctx context.Context, creds []GeminiCredential) error {
	cleaned := make([]GeminiCredential, 0, len(creds))
	for _, c := range creds {
		key := strings.TrimSpace(c.Key)
		if key == "" {
			continue
		}
		model := strings.TrimSpace(c.Model)
		if model == "" {
			model = defaultModel
		}
		cleaned = append(cleaned, GeminiCredential{Key: key, Model: model})
	}

	if len(cleaned) == 0 {
		return ErrEmptyCredentials
	}

	jsonBytes, err := json.Marshal(cleaned)
	if err != nil {
		logger.Error("Failed to marshal gemini credentials", logger.Err(err))
		return err
	}

	return s.repo.Set(ctx, GeminiCredentialsSetting, string(jsonBytes))
}

// GetMaskedCredentials returns the active credentials with keys masked so the
// full API key never leaves the server.
func (s *Service) GetMaskedCredentials(ctx context.Context) ([]GeminiCredential, error) {
	creds, err := s.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}

	masked := make([]GeminiCredential, len(creds))
	for i, c := range creds {
		masked[i] = GeminiCredential{
			Key:   maskKey(c.Key),
			Model: c.Model,
		}
	}

	return masked, nil
}

// maskKey shows the first 4 and last 4 characters with "..." in between
// (e.g. "AIza****abcd"). If the key is 8 chars or fewer it returns "****".
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
