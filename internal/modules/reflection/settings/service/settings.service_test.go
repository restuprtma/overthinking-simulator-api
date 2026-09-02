package service

import (
	"context"
	"errors"
	"testing"
)

// fakeStore implements settingsStore backed by an in-memory map.
type fakeStore struct {
	data map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: make(map[string]string)}
}

func (f *fakeStore) Get(_ context.Context, key string) (string, error) {
	return f.data[key], nil
}

func (f *fakeStore) Set(_ context.Context, key, value string) error {
	f.data[key] = value
	return nil
}

func newTestService(store settingsStore, fallback []GroqCredential) *Service {
	return &Service{repo: store, fallbackCredentials: fallback}
}

func TestGetCredentialsFallsBackWhenEmpty(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store, []GroqCredential{{Key: "fb1", Model: "m1"}})

	got, err := svc.GetCredentials(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Key != "fb1" || got[0].Model != "m1" {
		t.Fatalf("expected fallback credentials, got %+v", got)
	}
}

func TestSetCredentialsPersists(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store, nil)

	err := svc.SetCredentials(context.Background(), []GroqCredential{
		{Key: "  key1  ", Model: ""},
		{Key: "", Model: "ignored"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.data[GroqCredentialsSetting] == "" {
		t.Fatal("expected credentials to be persisted")
	}

	got, err := svc.GetCredentials(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 credential after trimming/dropping empty, got %d", len(got))
	}
	if got[0].Key != "key1" {
		t.Fatalf("expected key trimmed to 'key1', got %q", got[0].Key)
	}
	if got[0].Model != defaultModel {
		t.Fatalf("expected empty model to default to %q, got %q", defaultModel, got[0].Model)
	}
}

func TestSetCredentialsEmptyReturnsError(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store, nil)

	err := svc.SetCredentials(context.Background(), []GroqCredential{{Key: "   "}})
	if !errors.Is(err, ErrEmptyCredentials) {
		t.Fatalf("expected ErrEmptyCredentials, got %v", err)
	}
}

func TestGetMaskedNeverReturnsFullKey(t *testing.T) {
	store := newFakeStore()
	store.data[GroqCredentialsSetting] = `[{"key":"AIzaSyFULLKEYHEREabcd","model":"openai/gpt-oss-120b"},{"key":"short","model":"x"}]`

	svc := newTestService(store, nil)
	got, err := svc.GetMaskedCredentials(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 masked credentials, got %d", len(got))
	}

	full := "AIzaSyFULLKEYHEREabcd"
	if got[0].Key == full {
		t.Fatal("masked key must never equal the full key")
	}
	if want := full[:4] + "****" + full[len(full)-4:]; got[0].Key != want {
		t.Fatalf("unexpected mask, got %q want %q", got[0].Key, want)
	}
	if got[1].Key != "****" {
		t.Fatalf("expected short key masked to '****', got %q", got[1].Key)
	}
}

func TestGetMaskedDefaultsEmptyModel(t *testing.T) {
	store := newFakeStore()
	store.data[GroqCredentialsSetting] = `[{"key":"AIzaSyFULLKEYHEREabcd","model":""}]`

	svc := newTestService(store, nil)
	got, err := svc.GetMaskedCredentials(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Model != defaultModel {
		t.Fatalf("expected empty model defaulted to %q, got %q", defaultModel, got[0].Model)
	}
}
