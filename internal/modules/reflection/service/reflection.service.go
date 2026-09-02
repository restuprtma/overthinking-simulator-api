package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"venturo-skeleton-go/internal/modules/reflection/domain"
	"venturo-skeleton-go/internal/modules/reflection/prompts"
	"venturo-skeleton-go/internal/modules/reflection/repository"
	settingsService "venturo-skeleton-go/internal/modules/reflection/settings/service"
	"venturo-skeleton-go/pkg/groq"
	"venturo-skeleton-go/pkg/logger"
)

// Generator abstracts the Groq client so tests can substitute a mock.
type Generator interface {
	GenerateJSON(ctx context.Context, systemPrompt, userContent string) (string, error)
}

// reflectionStore is the seam the service depends on. The real implementation
// is *repository.ReflectionRepository; tests substitute an in-memory fake.
type reflectionStore interface {
	Create(ctx context.Context, ref *domain.Reflection) error
	ListByUser(ctx context.Context, userID string, page, limit int) ([]domain.Reflection, int64, error)
	GetByIDAndUser(ctx context.Context, id, userID string) (*domain.Reflection, error)
	AppendDialogTurns(ctx context.Context, id, userID string, turns []domain.DialogTurn, maxTurns int) (*domain.DialogState, error)
}

// GroqCredential aliases the settings credential type { Key, Model string }.
type GroqCredential = settingsService.GroqCredential

// Service orchestrates the two-stage reflection flow with credential failover.
type Service struct {
	repo                reflectionStore
	timeout             time.Duration
	newGenerator        func(apiKey, model string) Generator
	credentialsProvider func(ctx context.Context) ([]GroqCredential, error)
	distortionsJSON     []byte
	validIDs            map[string]bool
	persist             func(*domain.Reflection) error
}

// NewService parses distortionsJSON to build the valid distortion IDs and
// wires a default generator factory. Tests may override newGenerator and persist.
func NewService(repo *repository.ReflectionRepository, timeout time.Duration, distortionsJSON []byte, credentialsProvider func(ctx context.Context) ([]GroqCredential, error)) *Service {
	s := &Service{
		repo:    repo,
		timeout: timeout,
		newGenerator: func(apiKey, model string) Generator {
			return groq.NewClient(apiKey, "https://api.groq.com/openai/v1", model, timeout)
		},
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
	SafetyTriggered     bool                `json:"safety_triggered,omitempty"`
}

type dialogResult struct {
	Dialog               []domain.DialogTurn `json:"dialog"`
	ActionableSuggestion string              `json:"actionable_suggestion"`
	SafetyResponse       string              `json:"safety_response"`
}

// RunReflection fetches credentials, builds the failover order, and attempts
// each candidate until one succeeds or all are exhausted.
func (s *Service) RunReflection(ctx context.Context, userID, thought string) (*domain.Reflection, error) {
	if isCrisisSafetyInput(thought) {
		logger.Warn("reflection: upfront safety triggered for crisis input")
		ref := newSafetyReflection(thought)
		ref.UserID = userID
		if perr := s.persist(ref); perr != nil {
			logger.Error("Failed to persist safety reflection", logger.Err(perr))
			return nil, perr
		}
		return ref, nil
	}

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
func buildFailoverOrder(creds []GroqCredential) []GroqCredential {
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

	var order []GroqCredential
	for _, c := range creds {
		own := c.Model
		if own != "" {
			order = append(order, GroqCredential{Key: c.Key, Model: own})
		}
		for _, m := range models {
			if m == own {
				continue
			}
			order = append(order, GroqCredential{Key: c.Key, Model: m})
		}
		if own == "" && len(models) == 0 {
			order = append(order, GroqCredential{Key: c.Key, Model: ""})
		}
	}
	return order
}

// runStages executes the two-stage flow against a single credential.
func (s *Service) runStages(ctx context.Context, cred GroqCredential, thought string) (*domain.Reflection, error) {
	gen := s.newGenerator(cred.Key, cred.Model)

	classificationJSON, err := gen.GenerateJSON(ctx, prompts.ClassificationSystemPrompt, s.classificationUserContent(thought))
	if err != nil {
		if isSafetyRefusal(err.Error()) {
			return newSafetyReflection(thought), nil
		}
		return nil, err
	}

	classification, err := parseClassification(classificationJSON)
	if err != nil {
		return nil, err
	}
	if classification.SafetyTriggered {
		return newSafetyReflection(thought), nil
	}
	if err := validateClassification(classification, s.validIDs); err != nil {
		return nil, err
	}

	dialogJSON, err := gen.GenerateJSON(ctx, prompts.DialogSystemPrompt, s.dialogUserContent(thought, classificationJSON))
	if err != nil {
		if isSafetyRefusal(err.Error()) {
			return newSafetyReflection(thought), nil
		}
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
		ConversationState:   domain.ConversationInitial,
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
	ref.TotalTurns = len(dialog.Dialog)
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
		TeksAsli         string          `json:"teks_asli"`
		HasilKlasifikasi json.RawMessage `json:"hasil_klasifikasi"`
	}
	input := dialogInput{
		TeksAsli:         thought,
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

func normalizeDistortionID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	switch id {
	case "all_or_nothing", "all_or_nothing_thinking", "black_and_white", "black_white":
		return "black_and_white_thinking"
	case "mental_filter", "negative_filtering", "selective_abstraction":
		return "filtering"
	case "should_statement", "should", "must_statement":
		return "should_statements"
	case "jumping_to_conclusions", "fortunetelling":
		return "fortune_telling"
	case "mindreading":
		return "mind_reading"
	case "overgeneralizing":
		return "overgeneralization"
	case "personalizing":
		return "personalization"
	case "emotionalreasoning":
		return "emotional_reasoning"
	case "catastrophize", "magnification":
		return "catastrophizing"
	case "labelling":
		return "labeling"
	default:
		return id
	}
}

func normalizeSpeaker(speaker string) string {
	s := strings.ToLower(strings.TrimSpace(speaker))
	s = strings.TrimPrefix(s, "si ")
	s = strings.TrimPrefix(s, "si_")
	s = strings.TrimPrefix(s, "si-")
	if strings.Contains(s, "realis") {
		return "realistis"
	}
	if strings.Contains(s, "cemas") || strings.Contains(s, "anxious") {
		return "cemas"
	}
	return s
}

func parseClassification(raw string) (classificationResult, error) {
	cleaned := stripMarkdownFences(raw)
	var res classificationResult
	if err := json.Unmarshal([]byte(cleaned), &res); err != nil {
		if isSafetyRefusal(raw) {
			return classificationResult{
				DetectedDistortions: []domain.Distortion{},
				CoreFear:            "Krisis emosional berat atau pikiran menyakiti diri sendiri.",
				SafetyTriggered:     true,
			}, nil
		}
		return res, fmt.Errorf("%w: %v", ErrClassificationFailed, err)
	}

	validDistortions := make([]domain.Distortion, 0, len(res.DetectedDistortions))
	for _, d := range res.DetectedDistortions {
		d.ID = normalizeDistortionID(d.ID)
		if d.Intensity < 1 {
			d.Intensity = 3
		} else if d.Intensity > 5 {
			d.Intensity = 5
		}
		validDistortions = append(validDistortions, d)
	}
	if len(validDistortions) > 2 {
		validDistortions = validDistortions[:2]
	}
	res.DetectedDistortions = validDistortions

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
	cleaned := stripMarkdownFences(raw)
	var res dialogResult
	if err := json.Unmarshal([]byte(cleaned), &res); err != nil {
		if isSafetyRefusal(raw) {
			return dialogResult{
				Dialog:               []domain.DialogTurn{},
				ActionableSuggestion: defaultSafetySuggestion,
				SafetyResponse:       defaultSafetyResponse,
			}, nil
		}
		return res, fmt.Errorf("%w: %v", ErrDialogFailed, err)
	}
	for i := range res.Dialog {
		res.Dialog[i].Speaker = normalizeSpeaker(res.Dialog[i].Speaker)
	}
	if strings.TrimSpace(res.ActionableSuggestion) == "" && len(res.Dialog) > 0 {
		res.ActionableSuggestion = "Cobalah untuk fokus pada satu hal konkret yang bisa kamu kendalikan saat ini."
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
		if containsTaskOutput(turn.Text) {
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

// ContinueConversation generates the next dialog turn based on user's input.
// Both the user's message and the generated reply are persisted atomically so
// the stored dialog never drifts from what the client displays.
func (s *Service) ContinueConversation(ctx context.Context, reflectionID, userID, newUserMessage string) (*domain.ContinueResponse, error) {
	if isCrisisSafetyInput(newUserMessage) {
		logger.Warn("continue-conversation: upfront safety triggered for crisis input",
			logger.String("reflection_id", reflectionID))
		aiTurn := domain.DialogTurn{
			Speaker: "realistis",
			Text:    defaultSafetyResponse,
		}
		newTurns := []domain.DialogTurn{
			{Speaker: "cemas", Text: newUserMessage},
			aiTurn,
		}
		state, perr := s.repo.AppendDialogTurns(ctx, reflectionID, userID, newTurns, maxDialogTurns)
		if perr != nil {
			switch {
			case errors.Is(perr, repository.ErrDialogLimitReached):
				return nil, ErrConversationMaxed
			case errors.Is(perr, pgx.ErrNoRows):
				return nil, ErrReflectionNotFound
			}
			logger.Error("Failed to persist continued dialog", logger.Err(perr))
			return nil, perr
		}
		return &domain.ContinueResponse{
			NewTurn:           aiTurn,
			UpdatedDialog:     state.Dialog,
			ConversationState: state.ConversationState,
			TotalTurns:        state.TotalTurns,
		}, nil
	}

	creds, err := s.credentialsProvider(ctx)
	if err != nil || len(creds) == 0 {
		return nil, ErrMissingCredentials
	}

	ref, err := s.getReflectionForUser(ctx, reflectionID, userID)
	if err != nil {
		return nil, err
	}

	// A crisis-flagged reflection has no debate dialog by design; continuing it
	// would send the user's message to the model unscreened by the safety stage.
	if ref.SafetyTriggered {
		return nil, ErrSafetyTriggered
	}

	if len(ref.Dialog) >= maxDialogTurns {
		return nil, ErrConversationMaxed
	}

	candidates := buildFailoverOrder(creds)
	var lastErr error

	for _, cand := range candidates {
		aiTurn, err := s.generateContinueTurnOnce(ctx, cand, ref, newUserMessage)
		if err != nil && errors.Is(err, ErrDialogFailed) {
			// Retry once on the same credential: LLM output parsing is flaky.
			logger.Warn("continue-conversation: parse failed, retrying same credential",
				logger.String("model", cand.Model), logger.Err(err))
			aiTurn, err = s.generateContinueTurnOnce(ctx, cand, ref, newUserMessage)
		}
		if err == nil {
			// Append inside the UPDATE so concurrent continuations cannot clobber
			// each other; the DB returns the authoritative post-append dialog.
			newTurns := []domain.DialogTurn{
				{Speaker: "cemas", Text: newUserMessage},
				aiTurn,
			}
			state, perr := s.repo.AppendDialogTurns(ctx, reflectionID, userID, newTurns, maxDialogTurns)
			if perr != nil {
				switch {
				case errors.Is(perr, repository.ErrDialogLimitReached):
					return nil, ErrConversationMaxed
				case errors.Is(perr, pgx.ErrNoRows):
					return nil, ErrReflectionNotFound
				}
				logger.Error("Failed to persist continued dialog", logger.Err(perr))
				return nil, perr
			}
			return &domain.ContinueResponse{
				NewTurn:           aiTurn,
				UpdatedDialog:     state.Dialog,
				ConversationState: state.ConversationState,
				TotalTurns:        state.TotalTurns,
			}, nil
		}
		lastErr = err
		if isRateLimited(err) {
			logger.Warn("continue-conversation: rate limited, switching to next credential", logger.String("model", cand.Model))
			continue
		}
		if errors.Is(err, ErrDialogFailed) {
			logger.Warn("continue-conversation: dialog generation failed twice, switching to next credential",
				logger.String("model", cand.Model), logger.Err(err))
			continue
		}
		logger.Warn("continue-conversation: attempt failed, switching to next credential", logger.String("model", cand.Model), logger.Err(err))
	}

	if lastErr == nil {
		lastErr = ErrAllCredentialsFailed
	}
	// Preserve ErrDialogFailed for the handler when parsing failed everywhere.
	if errors.Is(lastErr, ErrDialogFailed) {
		return nil, errors.Join(ErrDialogFailed, lastErr)
	}
	return nil, errors.Join(ErrAllCredentialsFailed, lastErr)
}

// getReflectionForUser fetches the reflection scoped to the owning user.
func (s *Service) getReflectionForUser(ctx context.Context, reflectionID, userID string) (*domain.Reflection, error) {
	ref, err := s.repo.GetByIDAndUser(ctx, reflectionID, userID)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, ErrReflectionNotFound
	}
	return ref, nil
}

func (s *Service) generateContinueTurnOnce(ctx context.Context, cred GroqCredential, ref *domain.Reflection, newUserMessage string) (domain.DialogTurn, error) {
	gen := s.newGenerator(cred.Key, cred.Model)

	logger.Info("Generating continue turn",
		logger.String("reflection_id", ref.ID),
		logger.String("user_message_length", fmt.Sprintf("%d chars", len(newUserMessage))))

	dialogSystemPromptWithHistory := s.buildContinueSystemPrompt(ref, newUserMessage)

	nextTurnJSON, err := gen.GenerateJSON(ctx, dialogSystemPromptWithHistory, newUserMessage)
	if err != nil {
		logger.Error("GenerateJSON failed", logger.Err(err))
		return domain.DialogTurn{}, err
	}

	logger.Info("Generated JSON response", logger.String("raw_output", nextTurnJSON[:min(200, len(nextTurnJSON))]))

	nextTurn, err := parseNextTurn(nextTurnJSON)
	if err != nil {
		logger.Error("parseNextTurn failed", logger.Err(err))
		return domain.DialogTurn{}, err
	}

	if oos := scopeFlag(nextTurnJSON); shouldApplyScopeBackstop(nextTurn, oos) {
		logger.Warn("scope backstop replaced leaked reply",
			logger.String("reflection_id", ref.ID),
			logger.String("reason", scopeBackstopReason(nextTurn, oos)))
		nextTurn.Text = defaultScopeRedirect
	}

	logger.Info("Continue turn generated successfully",
		logger.String("response_text_preview", nextTurn.Text[:min(100, len(nextTurn.Text))]))

	return nextTurn, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildContinueSystemPrompt appends the conversation state to the shared
// continuation prompt. Only the most recent turns are sent so the prompt cannot
// grow without bound as the conversation gets longer.
func (s *Service) buildContinueSystemPrompt(ref *domain.Reflection, newUserMessage string) string {
	type InputData struct {
		ExistingDialog []domain.DialogTurn `json:"existing_dialog"`
		UserNewMessage string              `json:"user_new_message"`
	}

	input := InputData{
		ExistingDialog: recentTurns(ref.Dialog, maxPromptHistoryTurns),
		UserNewMessage: newUserMessage,
	}

	jsonInput, _ := json.Marshal(input)

	return prompts.ContinuationSystemPrompt + "\n\nDATA:\n" + string(jsonInput)
}

// recentTurns returns the trailing window of at most limit turns.
func recentTurns(dialog []domain.DialogTurn, limit int) []domain.DialogTurn {
	if limit <= 0 || len(dialog) <= limit {
		return dialog
	}
	return dialog[len(dialog)-limit:]
}

// parseNextTurn normalizes and validates the LLM's single-turn reply. It
// tolerates markdown fences and a missing/wrong speaker field as long as the
// reply text itself is usable.
func parseNextTurn(raw string) (domain.DialogTurn, error) {
	cleaned := stripMarkdownFences(raw)

	var turn domain.DialogTurn
	if err := json.Unmarshal([]byte(cleaned), &turn); err != nil {
		return turn, err
	}
	if strings.TrimSpace(turn.Text) == "" {
		return turn, ErrDialogFailed
	}
	if turn.Speaker == "" {
		turn.Speaker = "realistis"
	}
	if turn.Speaker != "realistis" {
		return turn, ErrDialogFailed
	}
	return turn, nil
}

// stripMarkdownFences removes leading ``` / ```json fences and trailing ```
// that LLMs sometimes wrap around JSON despite instructions.
func stripMarkdownFences(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:]
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "```" {
		lines = lines[:n-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// taskOutputImportLine matches a statement line that starts an import.
var taskOutputImportLine = regexp.MustCompile(`(?m)^\s*import\s+\S+`)

// taskOutputFromImportLine matches a statement line of the form "from x import y".
var taskOutputFromImportLine = regexp.MustCompile(`(?m)^\s*from\s+\S+\s+import\s+`)

// taskOutputDef matches a Python function definition line.
var taskOutputDef = regexp.MustCompile(`\bdef\s+[a-zA-Z_]\w*\s*\(`)

// taskOutputFunction matches a JavaScript function declaration.
var taskOutputFunction = regexp.MustCompile(`function\s+[a-zA-Z_$]\w*\s*\(`)

// containsTaskOutput reports whether text looks like it contains delivered
// code or terminal output. It uses only strong signals: literal code markers,
// case-sensitive SQL keywords, or statement-line imports/definitions that a
// natural-language refusal would never produce.
func containsTaskOutput(text string) bool {
	if strings.Contains(text, "```") ||
		strings.Contains(text, "print(") ||
		strings.Contains(text, "println(") ||
		strings.Contains(text, "console.log(") ||
		strings.Contains(text, "fmt.Println") ||
		strings.Contains(text, "System.out.print") ||
		strings.Contains(text, "#include") ||
		strings.Contains(text, "<?php") ||
		strings.Contains(text, "public static") ||
		(strings.Contains(text, "SELECT ") && strings.Contains(text, " FROM ")) {
		return true
	}
	if taskOutputImportLine.MatchString(text) ||
		taskOutputFromImportLine.MatchString(text) ||
		taskOutputDef.MatchString(text) ||
		taskOutputFunction.MatchString(text) {
		return true
	}
	return false
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
	ErrMissingCredentials   = errors.New("kredensial Groq belum diatur")
	ErrAllCredentialsFailed = errors.New("semua kredensial Groq gagal (limit/kadaluarsa)")
	ErrRateLimited          = errors.New("rate limited")
	ErrSafetyTriggered      = errors.New("refleksi ini ditandai safety, tidak bisa dilanjutkan")
	ErrConversationMaxed    = errors.New("percakapan sudah mencapai batas maksimal")
)

const (
	// maxDialogTurns caps how long one reflection's conversation can grow. Each
	// continuation appends two turns (user + reply).
	maxDialogTurns = 30
	// maxPromptHistoryTurns bounds how much history is sent to the LLM so token
	// usage stays flat as the stored dialog grows.
	maxPromptHistoryTurns = 12
	// maxScopeRefusalRunes caps how long a flagged refusal may be before it is
	// presumed to have leaked the task and replaced with the canonical redirect.
	maxScopeRefusalRunes = 400
)

const defaultSafetySuggestion = "Hubungi orang tepercaya atau layanan bantuan profesional/darurat setempat sekarang."

// defaultScopeRedirect is the deterministic reply substituted whenever the LLM
// leaks an out-of-scope task answer through. It mirrors the refusal example in
// ContinuationSystemPrompt so the voice stays consistent.
const defaultScopeRedirect = "Aku paham kamu pengin itu selesai, tapi di sini aku cuma nemenin kamu nata pikiran, bukan ngerjain tugasnya. Sekarang yang bikin kamu kepikiran soal itu apa?"

// scopeFlag reads the LLM's optional out_of_scope boolean from the raw JSON
// reply. The continuation reply now looks like
// {"speaker":"realistis","text":"...","out_of_scope":false}. A missing key
// (models that ignored the new field) or a malformed reply decodes to false.
func scopeFlag(raw string) bool {
	var rawField struct {
		OutOfScope json.RawMessage `json:"out_of_scope"`
	}
	if err := json.Unmarshal([]byte(raw), &rawField); err != nil {
		return false
	}
	trimmed := bytes.TrimSpace(rawField.OutOfScope)
	if len(trimmed) == 0 {
		return false
	}
	var b bool
	if err := json.Unmarshal(trimmed, &b); err == nil {
		return b
	}
	return bytes.Equal(trimmed, []byte("true"))
}

// shouldApplyScopeBackstop decides whether a continuant reply must be replaced by
// the canonical redirect instead of being shown verbatim.
func shouldApplyScopeBackstop(turn domain.DialogTurn, outOfScope bool) bool {
	if containsTaskOutput(turn.Text) {
		return true
	}
	if outOfScope && utf8.RuneCountInString(turn.Text) > maxScopeRefusalRunes {
		return true
	}
	return false
}

// scopeBackstopReason is a short human-readable label for logging.
func scopeBackstopReason(turn domain.DialogTurn, outOfScope bool) string {
	if containsTaskOutput(turn.Text) {
		return "task output leak"
	}
	return "out_of_scope refusal too long"
}

const defaultSafetyResponse = "Aku turut prihatin kamu sedang mengalami ini. Fitur refleksi ini bukan untuk kondisi darurat - tolong cari bantuan profesional atau kontak darurat setempat sekarang. Stay close dengan orang yang kamu percaya ya."

func newSafetyReflection(thought string) *domain.Reflection {
	sr := defaultSafetyResponse
	return &domain.Reflection{
		ID:                   uuid.NewString(),
		Thought:              thought,
		DetectedDistortions:  []domain.Distortion{},
		CoreFear:             "Krisis emosional atau kelelahan mental berat.",
		Dialog:               []domain.DialogTurn{},
		ActionableSuggestion: defaultSafetySuggestion,
		SafetyTriggered:      true,
		SafetyResponse:       &sr,
		ConversationState:    domain.ConversationInitial,
		TotalTurns:           0,
		CreatedAt:            time.Now(),
	}
}

var crisisPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(mau|ingin|pengen|rasanya|pengin|hendak)\s+(mati|meninggal|wafat|mengakhiri\s+hidup|bundir|bunuh\s+diri)\b`),
	regexp.MustCompile(`(?i)\b(bunuh\s*diri|bundir|akhiri\s*hidup|gantung\s*diri|potong\s*urat\s*nadi|sayat\s*tangan|minum\s*racun)\b`),
	regexp.MustCompile(`(?i)\b(gak|ga|gamau|nggak|tidak|tak)\s+(mau|ingin|pengen|bisa)\s+hidup(\s+lagi)?\b`),
	regexp.MustCompile(`(?i)\b(capek|lelah|nyerah)\s+(sama\s+hidup|hidup\s+ini|menjalani\s+hidup)\b`),
	regexp.MustCompile(`(?i)\b(kill\s+my\s*self|end\s+my\s+life|want\s+to\s+die|wanna\s+die|suicid(e|al)|better\s+off\s+dead|self\s*harm|hurt\s+my\s*self|cutting\s+my\s*self|end\s+it\s+all)\b`),
}

var directCrisisPhrases = []string{
	"mau mati", "ingin mati", "pengen mati", "pengin mati",
	"bunuh diri", "bundir", "akhiri hidup", "mengakhiri hidup",
	"gak mau hidup", "ga mau hidup", "gamau hidup", "tidak mau hidup",
	"capek hidup", "lelah hidup", "nyerah hidup",
	"kill myself", "end my life", "want to die", "wanna die", "suicide", "self harm",
}

func isCrisisSafetyInput(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range directCrisisPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	for _, re := range crisisPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

func isSafetyRefusal(text string) bool {
	lower := strings.ToLower(text)
	indicators := []string{
		"suicide", "self-harm", "crisis lifeline", "988", "layanan darurat",
		"bantuan profesional", "harm yourself", "kill yourself",
		"thoughts of suicide", "cannot assist with self-harm",
	}
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}
