package repository

import (
	"encoding/json"
	"testing"

	"venturo-skeleton-go/internal/modules/reflection/domain"
)

// These tests verify the JSON round-trip for the Distortion and DialogTurn
// slices so `go test ./...` stays green without a live database. The
// repository marshals these slices into JSONB columns and unmarshals them
// back; the same encoding/json path is exercised here.
func TestDistortionRoundTrip(t *testing.T) {
	original := []domain.Distortion{
		{ID: "catastrophizing", Intensity: 4},
		{ID: "mind-reading", Intensity: 3},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []domain.Distortion
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("item %d mismatch: got %+v, want %+v", i, decoded[i], original[i])
		}
	}
}

func TestDialogTurnRoundTrip(t *testing.T) {
	original := []domain.DialogTurn{
		{Speaker: "cemas", Text: "Ini pasti gagal."},
		{Speaker: "realistis", Text: "Belum tentu. Coba lihat faktanya."},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []domain.DialogTurn
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("item %d mismatch: got %+v, want %+v", i, decoded[i], original[i])
		}
	}
}

func TestEmptySlicesRoundTrip(t *testing.T) {
	distortions, err := json.Marshal([]domain.Distortion{})
	if err != nil {
		t.Fatalf("marshal distortions failed: %v", err)
	}
	var decodedDistortions []domain.Distortion
	if err := json.Unmarshal(distortions, &decodedDistortions); err != nil {
		t.Fatalf("unmarshal distortions failed: %v", err)
	}
	if decodedDistortions == nil || len(decodedDistortions) != 0 {
		t.Errorf("expected empty slice, got %v", decodedDistortions)
	}

	dialog, err := json.Marshal([]domain.DialogTurn{})
	if err != nil {
		t.Fatalf("marshal dialog failed: %v", err)
	}
	var decodedDialog []domain.DialogTurn
	if err := json.Unmarshal(dialog, &decodedDialog); err != nil {
		t.Fatalf("unmarshal dialog failed: %v", err)
	}
	if decodedDialog == nil || len(decodedDialog) != 0 {
		t.Errorf("expected empty slice, got %v", decodedDialog)
	}
}
