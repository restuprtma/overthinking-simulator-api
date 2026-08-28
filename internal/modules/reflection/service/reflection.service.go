package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"venturo-skeleton-go/internal/modules/reflection/domain"
	"venturo-skeleton-go/internal/modules/reflection/prompts"
	"venturo-skeleton-go/internal/modules/reflection/repository"
	"venturo-skeleton-go/internal/modules/reflection/settings/service"
	"venturo-skeleton-go/pkg/gemini"
	"venturo-skeleton-go/pkg/logger"
)

// Generator abstracts the Gemini client so tests can substitute a mock.
type Generator interface {
	GenerateJSON(ctx context.Context, systemPrompt, userContent string) (string, error)
}

// GeminiCredential aliases the settings credential type { Key, Model string }.
type GeminiCredential = service.GeminiCredential

// Service orchestrates the two-stage reflection flow with credential failover.
type Service struct {
	repo                *repository.ReflectionRepository
	timeout             time.Duration
	newGenerator        func(apiKey, model string) Generator
	credentialsProvider func(ctx context.Context) ([]GeminiCredential, error)
	distortionsJSON     []byte
	validIDs            map[string]bool
	persist             func(*domain.Reflection) error
}

// NewService parses distortionsJSON to build the valid distortion IDs and
// wires a default generator factory. Tests may override newGenerator and persist.
func NewService(repo *repository.ReflectionRepository, timeout time.Duration, distortionsJSON []byte, credentialsProvider func(ctx context.Context) ([]GeminiCredential, error)) *Service {
	s := &Service{
		repo:                repo,
		timeout:             timeout,
		newGenerator:        func(apiKey, model string) Generator { return gemini.NewClient(apiKey, model, timeout) },
		credentialsProvider: credentialsProvider,
		distortionsJSON:     distortionsJSON,
		validIDs:            buildValidIDs(distortionsJSON),
	}
	s.persist = func(r *domain.Reflection) error { return s.repo.Create(context.Background(), r) }
	return s
}

// buildValidIDs parses distortions.json and collects the "id" of each distortion.
func buildValidIDs(distortionsJSON []byte) map[string]bool {
	ids := map[string]bool{}
	var doc struct {
		Distortions []struct {
			ID string `json:"id"`
		} `json:"distortions"`
	}
	if err := json.Unmarshal(distortionsJSON, &doc); err != nil {
		logger.Error("Failed to parse distortions.json", logger.Err(err))
		return ids
	}
	for _, d := range doc.Distortions {
		if d.ID != "" {
			ids[d.ID] = true
		}
	}
	return ids
}

type classificationResult struct {
	DetectedDistortions []domain.Distortion `json:"detected_distortions"`
	CoreFear            string              `json:"core_fear"`
}

type dialogResult struct {
	Dialog               []domain.DialogTurn `json:"dialog"`
	ActionableSuggestion string              `json:"actionable_suggestion"`
	SafetyResponse       string              `json:"safety_response"`
}

// RunReflection fetches credentials, builds the failover order, and attempts
// each candidate until one succeeds or all are exhausted.
func (s *Service) RunReflection(ctx context.Context, userID, thought string) (*domain.Reflection, error) {
	creds, err := s.credentialsProvider(ctx)
	if err != nil || len(creds) == 0 {
		return nil, ErrMissingCredentials
	}

	candidates := buildFailoverOrder(creds)

	var lastErr error
	for _, cand := range candidates {
		// Attempt once; retry once more on validation failure.
		ref, err := s.runStages(ctx, cand, thought)
		if err == nil {
			ref.UserID = userID
			if perr := s.persist(ref); perr != nil {
				logger.Error("Failed to persist reflection", logger.Err(perr))
				return nil, perr
			}
			return ref, nil
		}

		lastErr = err

		if isRateLimited(err) {
			logger.Warn("reflection: rate limited, switching to next credential",
				logger.String("model", cand.Model))
			continue
		}

		if errors.Is(err, ErrClassificationFailed) || errors.Is(err, ErrDialogFailed) {
			logger.Warn("reflection: validation failed, retrying same credential",
				logger.String("model", cand.Model), logger.Err(err))
			ref, retryErr := s.runStages(ctx, cand, thought)
			if retryErr == nil {
				ref.UserID = userID
				if perr := s.persist(ref); perr != nil {
					logger.Error("Failed to persist reflection", logger.Err(perr))
					return nil, perr
				}
				return ref, nil
			}
			lastErr = retryErr
			if !isRateLimited(retryErr) {
				logger.Warn("reflection: validation failed again, switching to next credential",
					logger.String("model", cand.Model), logger.Err(retryErr))
			}
			continue
		}

		// Unknown error: treat as a failure for this credential and move on.
		logger.Warn("reflection: attempt failed, switching to next credential",
			logger.String("model", cand.Model), logger.Err(err))
	}

	if lastErr == nil {
		lastErr = ErrAllCredentialsFailed
	}
	return nil, errors.Join(ErrAllCredentialsFailed, lastErr)
}

// buildFailoverOrder produces the ordered list of (key, model) candidates.
// For each credential in list order, its own model (when non-empty) is tried
// first, then every other distinct model in the order they appear across the
// credential list.
func buildFailoverOrder(creds []GeminiCredential) []GeminiCredential {
	models := []string{}
	seen := map[string]bool{}
	for _, c := range creds {
		if c.Model == "" {
			continue
		}
		if !seen[c.Model] {
			seen[c.Model] = true
			models = append(models, c.Model)
		}
	}

	var order []GeminiCredential
	for _, c := range creds {
		own := c.Model
		if own != "" {
			order = append(order, GeminiCredential{Key: c.Key, Model: own})
		}
		for _, m := range models {
			if m == own {
				continue
			}
			order = append(order, GeminiCredential{Key: c.Key, Model: m})
		}
		if own == "" && len(models) == 0 {
			order = append(order, GeminiCredential{Key: c.Key, Model: ""})
		}
	}
	return order
}

// runStages executes the two-stage flow against a single credential.
func (s *Service) runStages(ctx context.Context, cred GeminiCredential, thought string) (*domain.Reflection, error) {
	gen := s.newGenerator(cred.Key, cred.Model)

	classificationJSON, err := gen.GenerateJSON(ctx, prompts.ClassificationSystemPrompt, s.classificationUserContent(thought))
	if err != nil {
		return nil, err
	}

	classification, err := parseClassification(classificationJSON)
	if err != nil {
		return nil, err
	}
	if err := validateClassification(classification, s.validIDs); err != nil {
		return nil, err
	}

	dialogJSON, err := gen.GenerateJSON(ctx, prompts.DialogSystemPrompt, s.dialogUserContent(thought, classificationJSON))
	if err != nil {
		return nil, err
	}

	dialog, err := parseDialog(dialogJSON)
	if err != nil {
		return nil, err
	}

	ref := &domain.Reflection{
		ID:                  uuid.NewString(),
		Thought:             thought,
		DetectedDistortions: classification.DetectedDistortions,
		CoreFear:            classification.CoreFear,
		CreatedAt:           time.Now(),
	}

	if dialog.SafetyResponse != "" {
		ref.SafetyTriggered = true
		sr := dialog.SafetyResponse
		ref.SafetyResponse = &sr
		ref.Dialog = []domain.DialogTurn{}
		ref.ActionableSuggestion = dialog.ActionableSuggestion
		if strings.TrimSpace(ref.ActionableSuggestion) == "" {
			ref.ActionableSuggestion = defaultSafetySuggestion
		}
		return ref, nil
	}

	if err := validateDialog(dialog.Dialog, dialog.ActionableSuggestion); err != nil {
		return nil, err
	}

	ref.Dialog = dialog.Dialog
	ref.ActionableSuggestion = dialog.ActionableSuggestion
	return ref, nil
}

func (s *Service) classificationUserContent(thought string) string {
	payload := map[string]json.RawMessage{
		"distortions": s.distortionsJSON,
		"thought":     mustJSONString(thought),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *Service) dialogUserContent(thought, classificationJSON string) string {
	type dialogInput struct {
		TeksAsli          string          `json:"teks_asli"`
		HasilKlasifikasi json.RawMessage `json:"hasil_klasifikasi"`
	}
	input := dialogInput{
		TeksAsli:          thought,
		HasilKlasifikasi: json.RawMessage(classificationJSON),
	}
	b, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return string(b)
}

func mustJSONString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return json.RawMessage(b)
}

func parseClassification(raw string) (classificationResult, error) {
	var res classificationResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return res, err
	}
	return res, nil
}

func validateClassification(res classificationResult, validIDs map[string]bool) error {
	if len(res.DetectedDistortions) > 2 {
		return ErrClassificationFailed
	}
	if strings.TrimSpace(res.CoreFear) == "" {
		return ErrClassificationFailed
	}
	for _, d := range res.DetectedDistortions {
		if !validIDs[d.ID] {
			return ErrClassificationFailed
		}
		if d.Intensity < 1 || d.Intensity > 5 {
			return ErrClassificationFailed
		}
	}
	return nil
}

func parseDialog(raw string) (dialogResult, error) {
	var res dialogResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return res, err
	}
	return res, nil
}

func validateDialog(dialog []domain.DialogTurn, suggestion string) error {
	if len(dialog) != 4 && len(dialog) != 6 {
		return ErrDialogFailed
	}
	for i, turn := range dialog {
		want := "cemas"
		if i%2 == 1 {
			want = "realistis"
		}
		if turn.Speaker != want {
			return ErrDialogFailed
		}
	}
	if strings.TrimSpace(suggestion) == "" {
		return ErrDialogFailed
	}
	return nil
}

// List delegates to the repository.
func (s *Service) List(ctx context.Context, userID string, page, limit int) ([]domain.Reflection, int64, error) {
	return s.repo.ListByUser(ctx, userID, page, limit)
}

// Get delegates to the repository and returns ErrReflectionNotFound when absent.
func (s *Service) Get(ctx context.Context, id, userID string) (*domain.Reflection, error) {
	ref, err := s.repo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, ErrReflectionNotFound
	}
	return ref, nil
}

// isRateLimited reports whether err indicates an HTTP 429 or RESOURCE_EXHAUSTED.
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") || strings.Contains(msg, "RESOURCE_EXHAUSTED")
}

var (
	ErrClassificationFailed = errors.New("gagal mengklasifikasi pikiran")
	ErrDialogFailed         = errors.New("gagal membuat dialog")
	ErrReflectionNotFound   = errors.New("reflection not found")
	ErrMissingCredentials   = errors.New("kredensial Gemini belum diatur")
	ErrAllCredentialsFailed = errors.New("semua kredensial Gemini gagal (limit/kadaluarsa)")
	ErrRateLimited          = errors.New("rate limited")
)

const defaultSafetySuggestion = "Hubungi orang tepercaya atau layanan bantuan profesional/darurat setempat sekarang."
