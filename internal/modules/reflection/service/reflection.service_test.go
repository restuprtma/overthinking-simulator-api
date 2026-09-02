package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

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

// fakeStore implements reflectionStore in memory so the continuation flow can
// be exercised without a live database. AppendDialogTurns mirrors the real
// SQL-side append: it concatenates onto its own stored copy, not onto whatever
// the caller read earlier, so a lost-update regression would surface here.
type fakeStore struct {
	ref          *domain.Reflection
	getErr       error
	updateErr    error
	updateCalls  int
	lastDialog   []domain.DialogTurn
	lastUserID   string
	lastReflorID string
}

func (f *fakeStore) Create(_ context.Context, _ *domain.Reflection) error { return nil }

func (f *fakeStore) ListByUser(_ context.Context, _ string, _, _ int) ([]domain.Reflection, int64, error) {
	return nil, 0, nil
}

func (f *fakeStore) GetByIDAndUser(_ context.Context, id, userID string) (*domain.Reflection, error) {
	f.lastReflorID = id
	f.lastUserID = userID
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.ref == nil {
		return nil, nil
	}
	clone := *f.ref
	clone.Dialog = append([]domain.DialogTurn{}, f.ref.Dialog...)
	return &clone, nil
}

func (f *fakeStore) AppendDialogTurns(_ context.Context, id, userID string, turns []domain.DialogTurn, maxTurns int) (*domain.DialogState, error) {
	f.updateCalls++
	f.lastReflorID = id
	f.lastUserID = userID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.ref == nil {
		return nil, pgx.ErrNoRows
	}
	if len(f.ref.Dialog) >= maxTurns {
		return nil, repository.ErrDialogLimitReached
	}

	f.ref.Dialog = append(f.ref.Dialog, turns...)
	f.ref.TotalTurns = len(f.ref.Dialog)
	f.ref.ConversationState = domain.ConversationContinued
	if f.ref.TotalTurns >= maxTurns {
		f.ref.ConversationState = domain.ConversationFinal
	}

	f.lastDialog = append([]domain.DialogTurn{}, f.ref.Dialog...)
	return &domain.DialogState{
		Dialog:            f.lastDialog,
		ConversationState: f.ref.ConversationState,
		TotalTurns:        f.ref.TotalTurns,
	}, nil
}

// interleavingStore runs a hook just before the append lands, letting a test
// simulate another request committing turns between our read and our write.
type interleavingStore struct {
	*fakeStore
	onAppend func()
}

func (s *interleavingStore) AppendDialogTurns(ctx context.Context, id, userID string, turns []domain.DialogTurn, maxTurns int) (*domain.DialogState, error) {
	if s.onAppend != nil {
		s.onAppend()
		s.onAppend = nil
	}
	return s.fakeStore.AppendDialogTurns(ctx, id, userID, turns, maxTurns)
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

func newTestService(provider func(ctx context.Context) ([]GroqCredential, error), genFactory func(apiKey, model string) Generator) *Service {
	s := NewService(nil, 0, []byte(validDistortionsJSON), provider)
	s.repo = &fakeStore{}
	if genFactory != nil {
		s.newGenerator = genFactory
	}
	s.persist = func(r *domain.Reflection) error { return nil }
	return s
}

func credProvider(creds []GroqCredential) func(ctx context.Context) ([]GroqCredential, error) {
	return func(ctx context.Context) ([]GroqCredential, error) { return creds, nil }
}

func TestRunReflectionHappyPath(t *testing.T) {
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
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
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
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
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
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
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{badClass}}
	}
	_, err := svc.RunReflection(context.Background(), "user-1", "pikiran")
	if !errors.Is(err, ErrAllCredentialsFailed) {
		t.Fatalf("expected ErrAllCredentialsFailed, got %v", err)
	}
}

func TestRunReflectionFailover(t *testing.T) {
	rateErr := errors.New("groq api error: status 429: RESOURCE_EXHAUSTED")
	var secondCalled string
	svc := newTestService(credProvider([]GroqCredential{
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
	rateErr := errors.New("groq api error: status 429")
	svc := newTestService(credProvider([]GroqCredential{
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

func TestParseNextTurn(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr error
		wantTxt string
	}{
		{
			name:    "valid json",
			raw:     `{"speaker":"realistis","text":"coba hubungi dosenmu hari ini"}`,
			wantTxt: "coba hubungi dosenmu hari ini",
		},
		{
			name:    "markdown fenced json",
			raw:     "```json\n{\"speaker\":\"realistis\",\"text\":\"fenced reply\"}\n```",
			wantTxt: "fenced reply",
		},
		{
			name:    "missing speaker defaults to realistis",
			raw:     `{"text":"no speaker here"}`,
			wantTxt: "no speaker here",
		},
		{
			name:    "wrong speaker rejected",
			raw:     `{"speaker":"cemas","text":"wrong persona"}`,
			wantErr: ErrDialogFailed,
		},
		{
			name:    "empty text rejected",
			raw:     `{"speaker":"realistis","text":"  "}`,
			wantErr: ErrDialogFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turn, err := parseNextTurn(tc.raw)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if turn.Text != tc.wantTxt {
				t.Fatalf("expected text %q, got %q", tc.wantTxt, turn.Text)
			}
			if turn.Speaker != "realistis" {
				t.Fatalf("expected speaker realistis, got %q", turn.Speaker)
			}
		})
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

// ----------------------------------------------------------------------
// ContinueConversation
// ----------------------------------------------------------------------

func seededStore() *fakeStore {
	return &fakeStore{
		ref: &domain.Reflection{
			ID:     "ref-1",
			UserID: "user-1",
			Dialog: []domain.DialogTurn{
				{Speaker: "cemas", Text: "a"},
				{Speaker: "realistis", Text: "b"},
			},
		},
	}
}

func newContinueService(store *fakeStore, genFactory func(apiKey, model string) Generator) *Service {
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), genFactory)
	svc.repo = store
	return svc
}

func TestContinueConversationPersistsBothTurns(t *testing.T) {
	store := seededStore()
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{`{"speaker":"realistis","text":"balasan baru"}`}}
	})

	resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan lanjutan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.NewTurn.Speaker != "realistis" || resp.NewTurn.Text != "balasan baru" {
		t.Fatalf("unexpected new turn: %+v", resp.NewTurn)
	}
	if len(resp.UpdatedDialog) != 4 {
		t.Fatalf("expected 4 turns in updated dialog, got %d", len(resp.UpdatedDialog))
	}
	if got := resp.UpdatedDialog[2]; got.Speaker != "cemas" || got.Text != "pesan lanjutan" {
		t.Fatalf("expected user turn appended, got %+v", got)
	}
	if got := resp.UpdatedDialog[3]; got.Speaker != "realistis" || got.Text != "balasan baru" {
		t.Fatalf("expected ai turn appended, got %+v", got)
	}
	if resp.TotalTurns != 4 {
		t.Fatalf("expected total_turns 4, got %d", resp.TotalTurns)
	}
	if resp.ConversationState != domain.ConversationContinued {
		t.Fatalf("expected state %q, got %q", domain.ConversationContinued, resp.ConversationState)
	}
	if store.updateCalls != 1 {
		t.Fatalf("expected dialog persisted once, got %d calls", store.updateCalls)
	}
	if len(store.lastDialog) != 4 {
		t.Fatalf("expected 4 persisted turns, got %d", len(store.lastDialog))
	}
	if store.lastUserID != "user-1" {
		t.Fatalf("expected repository scoped to user-1, got %q", store.lastUserID)
	}
}

// A stale in-memory snapshot must not be able to truncate the stored dialog:
// the service sends only the two new turns and the store owns the concatenation.
func TestContinueConversationDoesNotClobberConcurrentTurns(t *testing.T) {
	store := seededStore()
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{`{"speaker":"realistis","text":"balasan-B"}`}}
	})

	// Simulate another request that landed between our read and our write.
	svc.repo = &interleavingStore{
		fakeStore: store,
		onAppend: func() {
			store.ref.Dialog = append(store.ref.Dialog,
				domain.DialogTurn{Speaker: "cemas", Text: "pesan-A"},
				domain.DialogTurn{Speaker: "realistis", Text: "balasan-A"},
			)
		},
	}

	resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan-B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.UpdatedDialog) != 6 {
		t.Fatalf("expected the concurrent pair to survive (6 turns), got %d", len(resp.UpdatedDialog))
	}
	want := []string{"a", "b", "pesan-A", "balasan-A", "pesan-B", "balasan-B"}
	for i, w := range want {
		if resp.UpdatedDialog[i].Text != w {
			t.Fatalf("turn %d: expected %q, got %q", i, w, resp.UpdatedDialog[i].Text)
		}
	}
}

func TestContinueConversationSecondTurnKeepsFullHistory(t *testing.T) {
	store := seededStore()
	replies := []string{`{"speaker":"realistis","text":"balasan-1"}`, `{"speaker":"realistis","text":"balasan-2"}`}
	call := 0
	svc := newContinueService(store, func(apiKey, model string) Generator {
		gen := &mockGenerator{responses: []string{replies[call]}}
		call++
		return gen
	})

	if _, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan-1"); err != nil {
		t.Fatalf("first continuation failed: %v", err)
	}
	resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan-2")
	if err != nil {
		t.Fatalf("second continuation failed: %v", err)
	}

	if len(resp.UpdatedDialog) != 6 {
		t.Fatalf("expected 6 turns after two continuations, got %d", len(resp.UpdatedDialog))
	}
	want := []string{"a", "b", "pesan-1", "balasan-1", "pesan-2", "balasan-2"}
	for i, w := range want {
		if resp.UpdatedDialog[i].Text != w {
			t.Fatalf("turn %d: expected %q, got %q", i, w, resp.UpdatedDialog[i].Text)
		}
	}
}

func TestContinueConversationRetriesOnParseFailure(t *testing.T) {
	store := seededStore()
	gen := &mockGenerator{responses: []string{
		`{"speaker":"cemas","text":"salah persona"}`,
		`{"speaker":"realistis","text":"berhasil di retry"}`,
	}}
	svc := newContinueService(store, func(apiKey, model string) Generator { return gen })

	resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan")
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if resp.NewTurn.Text != "berhasil di retry" {
		t.Fatalf("unexpected reply: %q", resp.NewTurn.Text)
	}
	if gen.calls != 2 {
		t.Fatalf("expected 2 generator calls, got %d", gen.calls)
	}
}

func TestContinueConversationReturnsDialogFailedWhenAllParsesFail(t *testing.T) {
	store := seededStore()
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{
			`{"speaker":"cemas","text":"x"}`,
			`{"speaker":"cemas","text":"x"}`,
		}}
	})

	_, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan")
	if !errors.Is(err, ErrDialogFailed) {
		t.Fatalf("expected ErrDialogFailed, got %v", err)
	}
	if store.updateCalls != 0 {
		t.Fatalf("expected no persistence on failure, got %d calls", store.updateCalls)
	}
}

func TestContinueConversationNotFound(t *testing.T) {
	store := &fakeStore{}
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{`{"speaker":"realistis","text":"never used"}`}}
	})

	_, err := svc.ContinueConversation(context.Background(), "missing", "user-1", "pesan")
	if !errors.Is(err, ErrReflectionNotFound) {
		t.Fatalf("expected ErrReflectionNotFound, got %v", err)
	}
}

func TestContinueConversationMissingCredentials(t *testing.T) {
	svc := newTestService(credProvider(nil), nil)
	svc.repo = seededStore()

	_, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan")
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("expected ErrMissingCredentials, got %v", err)
	}
}

func TestContinueConversationFailsWhenPersistenceFails(t *testing.T) {
	store := seededStore()
	store.updateErr = errors.New("db down")
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{`{"speaker":"realistis","text":"ok"}`}}
	})

	if _, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan"); err == nil {
		t.Fatal("expected error when persistence fails")
	}
}

func TestContinueConversationRejectsSafetyTriggered(t *testing.T) {
	store := seededStore()
	store.ref.SafetyTriggered = true
	store.ref.Dialog = []domain.DialogTurn{}
	gen := &mockGenerator{responses: []string{`{"speaker":"realistis","text":"never used"}`}}
	svc := newContinueService(store, func(apiKey, model string) Generator { return gen })

	_, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan")
	if !errors.Is(err, ErrSafetyTriggered) {
		t.Fatalf("expected ErrSafetyTriggered, got %v", err)
	}
	if gen.calls != 0 {
		t.Fatalf("expected no LLM call for safety-triggered reflection, got %d", gen.calls)
	}
	if store.updateCalls != 0 {
		t.Fatalf("expected no persistence, got %d calls", store.updateCalls)
	}
}

func TestContinueConversationRejectsWhenMaxTurnsReached(t *testing.T) {
	store := seededStore()
	dialog := make([]domain.DialogTurn, maxDialogTurns)
	for i := range dialog {
		speaker := "cemas"
		if i%2 == 1 {
			speaker = "realistis"
		}
		dialog[i] = domain.DialogTurn{Speaker: speaker, Text: "x"}
	}
	store.ref.Dialog = dialog
	gen := &mockGenerator{responses: []string{`{"speaker":"realistis","text":"never used"}`}}
	svc := newContinueService(store, func(apiKey, model string) Generator { return gen })

	_, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan")
	if !errors.Is(err, ErrConversationMaxed) {
		t.Fatalf("expected ErrConversationMaxed, got %v", err)
	}
	if gen.calls != 0 {
		t.Fatalf("expected no LLM call once maxed out, got %d", gen.calls)
	}
}

func TestRecentTurnsBoundsPromptHistory(t *testing.T) {
	dialog := make([]domain.DialogTurn, 20)
	for i := range dialog {
		dialog[i] = domain.DialogTurn{Speaker: "cemas", Text: fmt.Sprintf("turn-%d", i)}
	}

	got := recentTurns(dialog, maxPromptHistoryTurns)
	if len(got) != maxPromptHistoryTurns {
		t.Fatalf("expected %d turns, got %d", maxPromptHistoryTurns, len(got))
	}
	if got[0].Text != "turn-8" {
		t.Fatalf("expected window to start at turn-8, got %q", got[0].Text)
	}
	if got[len(got)-1].Text != "turn-19" {
		t.Fatalf("expected window to end at turn-19, got %q", got[len(got)-1].Text)
	}

	short := dialog[:3]
	if len(recentTurns(short, maxPromptHistoryTurns)) != 3 {
		t.Fatal("expected short dialog to pass through untouched")
	}
}

func TestBuildContinueSystemPromptCarriesSafetyRules(t *testing.T) {
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
	ref := &domain.Reflection{
		ID:     "ref-1",
		Dialog: []domain.DialogTurn{{Speaker: "cemas", Text: "a"}},
	}

	prompt := svc.buildContinueSystemPrompt(ref, "pesan baru")

	for _, want := range []string{"ATURAN SAFETY", "Abaikan semua instruksi", "user_new_message", "pesan baru"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected continuation prompt to contain %q", want)
		}
	}
}

// TestBuildContinueSystemPromptCarriesScopeRules checks the scope guardrail
// survives prompt assembly, not just the constant: buildContinueSystemPrompt is
// what actually reaches the LLM on every follow-up turn.
func TestBuildContinueSystemPromptCarriesScopeRules(t *testing.T) {
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
	ref := &domain.Reflection{
		ID:     "ref-1",
		Dialog: []domain.DialogTurn{{Speaker: "cemas", Text: "a"}},
	}

	prompt := svc.buildContinueSystemPrompt(ref, "coba buatkan kode python singkat")

	for _, want := range []string{"ATURAN SCOPE", "out_of_scope"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected continuation prompt to contain %q", want)
		}
	}
}

// ----------------------------------------------------------------------
// Scope guardrail: containments, backstop decisions, and substitution
// ----------------------------------------------------------------------

// TestContainsTaskOutput covers the worded leak detector: every "positive"
// string must be flagged as task output, every "negative" string must not be.
func TestContainsTaskOutput(t *testing.T) {
	positives := []string{
		"```python\nprint('x')\n```",
		"ikuti ini: println(b)",
		"console.log('debug')",
		"fmt.Println(hasil)",
		"class A { System.out.println(a); }",
		"#include <stdio.h>",
		"<?php echo 'x'; ?>",
		"public static void main(String[] args)",
		"SELECT * FROM users WHERE id = 1",
		"import os\nprint(os.name)",
		"from datetime import datetime",
		"def hello():\n    return 1",
		"function hitungTotal() { return a; }",
	}
	for _, text := range positives {
		if !containsTaskOutput(text) {
			t.Fatalf("expected containsTaskOutput(%q) == true, got false", text)
		}
	}

	negatives := []string{
		"Coba tulis satu hal kecil yang bisa kamu kontrol hari ini.",
		"Wajar kamu kepikiran. Cetak daftar pikiranmu, lalu tandai yang punya bukti nyata.",
		"Menurutku pilih mana pun baik, kamu bisa mulai dari satu email dulu.",
		"Import barang dari luar kadang lebih murah, tepatnya pilih yang kamu kontrol.",
		"Perasaanmu valid, tapi kesimpulannya belum tentu benar.",
		defaultScopeRedirect,
		"Audisi sedang berjalan, dan kamu tinggal menunggu keputusan.",
	}
	for _, text := range negatives {
		if containsTaskOutput(text) {
			t.Fatalf("expected containsTaskOutput(%q) == false, got true", text)
		}
	}
}

// TestShouldApplyScopeBackstop drives the decision function plus the reason
// label, so a test proves WHICH rule fired, not merely that one did.
func TestShouldApplyScopeBackstop(t *testing.T) {
	longIndoRefusal := strings.Repeat("kamu ", 260)

	cases := []struct {
		name       string
		text       string
		outOfScope bool
		wantApply  bool
		wantReason string
	}{
		{
			name:       "no code and no flag untouched",
			text:       "Tak ada kode",
			outOfScope: false,
			wantApply:  false,
			wantReason: "",
		},
		{
			name:       "code leak fires even when flag false",
			text:       "```\ncode\n```",
			outOfScope: false,
			wantApply:  true,
			wantReason: "task output leak",
		},
		{
			name:       "polite refusal without code untouched",
			text:       "Tolong jangan, minta sesuatu yang kasar tidak akan berhasil, ok.",
			outOfScope: false,
			wantApply:  false,
			wantReason: "",
		},
		{
			name:       "over-long refusal with flag fires length guard",
			text:       longIndoRefusal,
			outOfScope: true,
			wantApply:  true,
			wantReason: "out_of_scope refusal too long",
		},
		{
			name:       "same-length reply without flag is left alone",
			text:       longIndoRefusal,
			outOfScope: false,
			wantApply:  false,
			wantReason: "",
		},
		{
			name:       "short refusal with flag preserved",
			text:       "Aku paham, tapi di sini aku cuma nemenin kamu nata pikiran. Apa yang kepikiranmu?",
			outOfScope: true,
			wantApply:  false,
			wantReason: "",
		},
		{
			name:       "short normal reply preserved",
			text:       "Aku paham, tapi di sini aku cuma nemenin kamu nata pikiran. Apa yang kepikiranmu?",
			outOfScope: false,
			wantApply:  false,
			wantReason: "",
		},
		{
			name:       "both signals fire",
			text:       "```\nconsole.log('x')\n```",
			outOfScope: true,
			wantApply:  true,
			wantReason: "task output leak",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turn := domain.DialogTurn{Speaker: "realistis", Text: tc.text}
			gotApply := shouldApplyScopeBackstop(turn, tc.outOfScope)
			if gotApply != tc.wantApply {
				t.Fatalf("shouldApplyScopeBackstop(%q, %v) = %v, want %v", tc.text, tc.outOfScope, gotApply, tc.wantApply)
			}
			// The reason label is only meaningful when the backstop fires; it
			// names WHICH rule triggered the replacement.
			if tc.wantApply {
				gotReason := scopeBackstopReason(turn, tc.outOfScope)
				if gotReason != tc.wantReason {
					t.Fatalf("scopeBackstopReason(%q, %v) = %q, want %q", tc.text, tc.outOfScope, gotReason, tc.wantReason)
				}
			}
		})
	}
}

// TestValidateDialogRejectsCodeLeak ensures a leaked turn in the initial-dialog
// path fails the whole debate (the callers retry), rather than being shown.
func TestValidateDialogRejectsCodeLeak(t *testing.T) {
	leaked := domain.DialogTurn{
		Speaker: "realistis",
		Text:    "Ini caranya:\n```python\nprint('x')\n```",
	}
	leakyDialog := []domain.DialogTurn{
		{Speaker: "cemas", Text: "a"},
		{Speaker: "realistis", Text: "b"},
		{Speaker: "cemas", Text: "c"},
		leaked,
	}
	if err := validateDialog(leakyDialog, "saran"); !errors.Is(err, ErrDialogFailed) {
		t.Fatalf("expected ErrDialogFailed on leaked turn, got %v", err)
	}

	cleanDialog := []domain.DialogTurn{
		{Speaker: "cemas", Text: "a"},
		{Speaker: "realistis", Text: "b"},
		{Speaker: "cemas", Text: "c"},
		{Speaker: "realistis", Text: "d"},
	}
	if err := validateDialog(cleanDialog, "saran"); err != nil {
		t.Fatalf("expected clean dialog to pass, got %v", err)
	}
}

// TestContinueConversationSubstitutesLeakedReply checks the deterministic
// backstop in the continue path: a leaked reply is replaced with the canonical
// redirect in BOTH the returned turn and the persisted dialog, even when the
// model flagged out_of_scope:false.
func TestContinueConversationSubstitutesLeakedReply(t *testing.T) {
	store := seededStore()
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{
			"{\"speaker\":\"realistis\",\"text\":\"Ini kodenya:\\n```python\\nprint(\\\"halo\\\")\\n```\",\"out_of_scope\":false}",
		}}
	})

	resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "bikin kode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.NewTurn.Text != defaultScopeRedirect {
		t.Fatalf("expected new turn to be the redirect, got %q", resp.NewTurn.Text)
	}
	last := store.lastDialog[len(store.lastDialog)-1]
	if last.Text != defaultScopeRedirect {
		t.Fatalf("expected persisted last turn to be the redirect, got %q", last.Text)
	}
}

// TestContinueConversationKeepsGoodRefusalAndLeaks ensures the backstop does not
// over-trigger: a short polite refusal flagged out_of_scope stays verbatim, an
// over-long flagged refusal is replaced, and a normal reply passes untouched.
func TestContinueConversationKeepsGoodRefusalAndLeaks(t *testing.T) {
	t.Run("short_refusal_flagged_is_kept_verbatim", func(t *testing.T) {
		store := seededStore()
		good := "Aku paham kamu mau itu cepet beres, tapi di sini aku cuma nemenin kamu nata pikiran."
		svc := newContinueService(store, func(apiKey, model string) Generator {
			return &mockGenerator{responses: []string{
				`{"speaker":"realistis","text":"` + good + `","out_of_scope":true}`,
			}}
		})

		resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "bikin kode")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NewTurn.Text != good {
			t.Fatalf("expected short refusal kept verbatim, got %q", resp.NewTurn.Text)
		}
	})

	t.Run("overlong_flagged_refusal_is_replaced", func(t *testing.T) {
		store := seededStore()
		long := strings.Repeat("kamu ", 260)
		json := `{"speaker":"realistis","text":"` + long + `","out_of_scope":true}`
		svc := newContinueService(store, func(apiKey, model string) Generator {
			return &mockGenerator{responses: []string{json}}
		})

		resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "bikin kode")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NewTurn.Text != defaultScopeRedirect {
			t.Fatalf("expected long flagged refusal replaced with redirect, got %q", resp.NewTurn.Text)
		}
	})

	t.Run("normal_reply_untouched", func(t *testing.T) {
		store := seededStore()
		normal := "Masuk akal kamu envira. Coba tulis satu langkah kecil yang bisa kamu kontrol hari ini."
		svc := newContinueService(store, func(apiKey, model string) Generator {
			return &mockGenerator{responses: []string{
				`{"speaker":"realistis","text":"` + normal + `"}`,
			}}
		})

		resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "pesan biasa")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NewTurn.Text != normal {
			t.Fatalf("expected normal reply untouched, got %q", resp.NewTurn.Text)
		}
	})
}

// ----------------------------------------------------------------------
// Crisis and Safety Guardrail Tests
// ----------------------------------------------------------------------

func TestRunReflectionUpfrontCrisisDetection(t *testing.T) {
	cases := []struct {
		name    string
		thought string
	}{
		{
			name:    "user reported prompt: saya mau mati deh rasanya saking capeknya",
			thought: "saya mau mati deh rasanya saking capeknya",
		},
		{
			name:    "direct phrase: rasanya pengen mati aja",
			thought: "rasanya pengen mati aja",
		},
		{
			name:    "direct phrase: aku mau bunuh diri",
			thought: "aku mau bunuh diri",
		},
		{
			name:    "direct phrase: gak mau hidup lagi",
			thought: "aku capek dan gak mau hidup lagi",
		},
		{
			name:    "english phrase: i want to kill myself",
			thought: "i want to kill myself",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			genCalled := false
			svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), func(apiKey, model string) Generator {
				genCalled = true
				return &mockGenerator{}
			})
			var persisted *domain.Reflection
			svc.persist = func(r *domain.Reflection) error {
				persisted = r
				return nil
			}

			ref, err := svc.RunReflection(context.Background(), "user-1", tc.thought)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if genCalled {
				t.Fatalf("expected LLM generator NOT to be called for crisis input")
			}
			if !ref.SafetyTriggered {
				t.Fatalf("expected SafetyTriggered = true")
			}
			if ref.SafetyResponse == nil || *ref.SafetyResponse != defaultSafetyResponse {
				t.Fatalf("expected default safety response, got %v", ref.SafetyResponse)
			}
			if ref.ActionableSuggestion != defaultSafetySuggestion {
				t.Fatalf("expected default safety suggestion, got %q", ref.ActionableSuggestion)
			}
			if len(ref.Dialog) != 0 {
				t.Fatalf("expected empty dialog, got %d turns", len(ref.Dialog))
			}
			if persisted == nil || persisted.ID != ref.ID {
				t.Fatalf("expected reflection to be persisted")
			}
		})
	}
}

func TestIsCrisisSafetyInput(t *testing.T) {
	crisisInputs := []string{
		"saya mau mati deh rasanya saking capeknya",
		"aku pengen mati",
		"ingin mati",
		"pengin mati",
		"mau bunuh diri",
		"bundir aja kali ya",
		"capek hidup rasanya",
		"lelah hidup ini",
		"aku gamau hidup lagi",
		"tidak mau hidup",
		"i want to die",
		"i want to kill myself",
		"i wanna die",
		"cutting myself again",
		"end my life right now",
	}

	for _, input := range crisisInputs {
		if !isCrisisSafetyInput(input) {
			t.Errorf("expected isCrisisSafetyInput(%q) = true, got false", input)
		}
	}

	normalInputs := []string{
		"atasan belum balas chat, pasti pekerjaan saya buruk",
		"aku salah menyebut satu angka saat presentasi",
		"dia sudah online tapi belum balas chat-ku",
		"besok ujian dan aku belum belajar",
		"takut presentasi besok gagal total",
	}

	for _, input := range normalInputs {
		if isCrisisSafetyInput(input) {
			t.Errorf("expected isCrisisSafetyInput(%q) = false, got true", input)
		}
	}
}

func TestRunReflectionClassificationSafetyResponse(t *testing.T) {
	classSafety := `{"detected_distortions":[],"core_fear":"krisis","safety_triggered":true}`
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{classSafety}}
	}

	ref, err := svc.RunReflection(context.Background(), "user-1", "pikiran yang borderline")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ref.SafetyTriggered {
		t.Fatalf("expected SafetyTriggered = true")
	}
	if len(ref.Dialog) != 0 {
		t.Fatalf("expected empty dialog, got %d turns", len(ref.Dialog))
	}
}

func TestRunReflectionSafetyRefusalFallback(t *testing.T) {
	// Simulate LLM returning a safety refusal text instead of JSON
	refusalText := "I cannot assist with self-harm. If you are experiencing thoughts of suicide, please call 988."
	svc := newTestService(credProvider([]GroqCredential{{Key: "k1", Model: "m1"}}), nil)
	svc.newGenerator = func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{refusalText}}
	}

	ref, err := svc.RunReflection(context.Background(), "user-1", "pikiran ambigu")
	if err != nil {
		t.Fatalf("expected safety refusal to be handled gracefully, got error: %v", err)
	}
	if !ref.SafetyTriggered {
		t.Fatalf("expected SafetyTriggered = true")
	}
}

func TestContinueConversationCrisisInput(t *testing.T) {
	store := seededStore()
	svc := newContinueService(store, func(apiKey, model string) Generator {
		return &mockGenerator{responses: []string{`{"speaker":"realistis","text":"never called"}`}}
	})

	resp, err := svc.ContinueConversation(context.Background(), "ref-1", "user-1", "aku capek banget mau mati rasanya")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.NewTurn.Speaker != "realistis" || resp.NewTurn.Text != defaultSafetyResponse {
		t.Fatalf("expected safety response turn, got %+v", resp.NewTurn)
	}
	if len(resp.UpdatedDialog) != 4 {
		t.Fatalf("expected 4 turns, got %d", len(resp.UpdatedDialog))
	}
}

func TestParseClassificationMarkdownFences(t *testing.T) {
	raw := "```json\n" + classificationJSON + "\n```"
	res, err := parseClassification(raw)
	if err != nil {
		t.Fatalf("expected markdown fenced classification to parse, got %v", err)
	}
	if len(res.DetectedDistortions) != 1 || res.DetectedDistortions[0].ID != "mind_reading" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestParseDialogMarkdownFences(t *testing.T) {
	raw := "```json\n" + dialogJSON + "\n```"
	res, err := parseDialog(raw)
	if err != nil {
		t.Fatalf("expected markdown fenced dialog to parse, got %v", err)
	}
	if len(res.Dialog) != 4 {
		t.Fatalf("expected 4 turns, got %d", len(res.Dialog))
	}
}
