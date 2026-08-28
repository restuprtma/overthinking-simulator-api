package service

import (
	"context"
	"errors"
	"testing"

	"venturo-skeleton-go/internal/modules/reflection/domain"
	"venturo-skeleton-go/internal/modules/reflection/repository"
)

type mockGenerator struct {
	responses []string
	errs      []error
	calls     int
	key       string
	model     string
}

func (m *mockGenerator) GenerateJSON(ctx context.Context, systemPrompt, userContent string) (string, error) {
	idx := m.calls
	m.calls++
	if idx < len(m.errs) && m.errs[idx] != nil {
		return "", m.errs[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return "", errors.New("mock: no scripted response")
}

const validDistortionsJSON = `{
  "disclaimer_note": "x",
  "distortions": [
    {"id":"catastrophizing"},{"id":"mind_reading"},{"id":"fortune_telling"},
    {"id":"overgeneralization"},{"id":"personalization"},{"id":"black_and_white_thinking"},
    {"id":"emotional_reasoning"},{"id":"should_statements"},{"id":"filtering"},{"id":"labeling"}
  ]
}`

const classificationJSON = `{"detected_distortions":[{"id":"mind_reading","intensity":4}],"core_fear":"takut ditolak"}`

const dialogJSON = `{"dialog":[{"speaker":"cemas","text":"a"},{"speaker":"realistis","text":"b"},{"speaker":"cemas","text":"c"},{"speaker":"realistis","text":"d"}],"actionable_suggestion":"lakukan ini"}`

func newTestService(provider func(ctx context.Context) ([]GeminiCredential, error), genFactory func(apiKey, model string) Generator) *Service {
	repo := repository.NewReflectionRepository(nil)
	s := NewService(repo, 0, []byte(validDistortionsJSON), provider)
	if genFactory != nil {
		s.newGenerator = genFactory
	}
	s.persist = func(r *domain.Reflection) error { return nil }
	return s
}

func credProvider(creds []GeminiCredential) func(ctx context.Context) ([]GeminiCredential, error) {
	return func(ctx context.Context) ([]GeminiCredential, error) { return creds, nil }
}

func TestRunReflectionHappyPath(t *testing.T) {
	svc := newTestService(credProvider([]GeminiCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{classificationJSON, dialogJSON}, key: apiKey, model: model}
	}

	ref, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ref.UserID != "user-1" {
		t.Fatalf("expected user_id user-1, got %q", ref.UserID)
	}
	if len(ref.Dialog) != 4 {
		t.Fatalf("expected 4 dialog turns, got %d", len(ref.Dialog))
	}
	if ref.SafetyTriggered {
		t.Fatalf("expected safety not triggered")
	}
}

func TestRunReflectionZeroDistortion(t *testing.T) {
	class := `{"detected_distortions":[],"core_fear":"sedikit kecewa"}`
	svc := newTestService(credProvider([]GeminiCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{class, dialogJSON}}
	}
	ref, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ref.DetectedDistortions) != 0 {
		t.Fatalf("expected zero distortions, got %d", len(ref.DetectedDistortions))
	}
}

func TestRunReflectionSafetyPath(t *testing.T) {
	safety := `{"dialog":[],"actionable_suggestion":"","safety_response":"cari bantuan profesional"}`
	svc := newTestService(credProvider([]GeminiCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{classificationJSON, safety}}
	}
	ref, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ref.SafetyTriggered {
		t.Fatalf("expected safety triggered")
	}
	if ref.SafetyResponse == nil || *ref.SafetyResponse == "" {
		t.Fatalf("expected safety response set")
	}
	if len(ref.Dialog) != 0 {
		t.Fatalf("expected empty dialog")
	}
}

func TestRunReflectionInvalidID(t *testing.T) {
	badClass := `{"detected_distortions":[{"id":"not_a_real_id","intensity":4}],"core_fear":"takut"}`
	svc := newTestService(credProvider([]GeminiCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{badClass}}
	}
	_, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if !errors.Is(err, ErrAllCredentialsFailed) {
		t.Fatalf("expected ErrAllCredentialsFailed, got %v", err)
	}
}

func TestRunReflectionFailover(t *testing.T) {
	rateErr := errors.New("gemini api error: status 429: RESOURCE_EXHAUSTED")
	var secondCalled string
	svc := newTestService(credProvider([]GeminiCredential{
		{Key: "k1", Model: "m1"},
		{Key: "k2", Model: "m2"},
	}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		if apiKey == "k1" {
			return &mockGenerator{errs: []error{rateErr}}
		}
		secondCalled = apiKey + "/" + model
		return &mockGenerator{responses: []string{classificationJSON, dialogJSON}}
	}
	ref, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if err != nil {
		t.Fatalf("expected success via failover, got %v", err)
	}
	if secondCalled != "k2/m2" {
		t.Fatalf("expected second credential k2/m2, got %q", secondCalled)
	}
	if ref == nil {
		t.Fatalf("expected reflection")
	}
}

func TestRunReflectionAllFail(t *testing.T) {
	rateErr := errors.New("gemini api error: status 429")
	svc := newTestService(credProvider([]GeminiCredential{
		{Key: "k1", Model: "m1"},
		{Key: "k2", Model: "m2"},
	}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{errs: []error{rateErr}}
	}
	_, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if !errors.Is(err, ErrAllCredentialsFailed) {
		t.Fatalf("expected ErrAllCredentialsFailed, got %v", err)
	}
}

func TestRunReflectionMissingCredentials(t *testing.T) {
	svc := newTestService(credProvider(nil), nil)
	_, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("expected ErrMissingCredentials, got %v", err)
	}
}

func TestIsRateLimited(t *testing.T) {
	if !isRateLimited(errors.New("status 429")) {
		t.Fatalf("expected 429 to be rate limited")
	}
	if !isRateLimited(errors.New("RESOURCE_EXHAUSTED")) {
		t.Fatalf("expected RESOURCE_EXHAUSTED to be rate limited")
	}
	if isRateLimited(errors.New("other")) {
		t.Fatalf("expected other not to be rate limited")
	}
}
