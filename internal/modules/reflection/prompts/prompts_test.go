package prompts

import (
	"strings"
	"testing"
)

// TestContinuationPromptDeclaresScopeBoundary guards the prompt half of the
// off-topic fix: the continuation prompt must carry a scope block, must name the
// out_of_scope output field the service layer parses, and must keep the
// anxiety-as-context exception so "takut kodeku dinilai jelek" is still handled.
func TestContinuationPromptDeclaresScopeBoundary(t *testing.T) {
	for _, want := range []string{"ATURAN SCOPE", "out_of_scope", "PENGECUALIAN"} {
		if !strings.Contains(ContinuationSystemPrompt, want) {
			t.Errorf("expected ContinuationSystemPrompt to contain %q", want)
		}
	}
}

// TestContinuationPromptKeepsSafetyAboveScope asserts declared priority order,
// not just presence: safety must stay the first-listed block so a crisis message
// that also asks for a chore is answered as a crisis, not as a scope refusal.
func TestContinuationPromptKeepsSafetyAboveScope(t *testing.T) {
	safetyIdx := strings.Index(ContinuationSystemPrompt, "ATURAN SAFETY")
	scopeIdx := strings.Index(ContinuationSystemPrompt, "ATURAN SCOPE")

	if safetyIdx < 0 {
		t.Fatal("expected ContinuationSystemPrompt to contain \"ATURAN SAFETY\"")
	}
	if scopeIdx < 0 {
		t.Fatal("expected ContinuationSystemPrompt to contain \"ATURAN SCOPE\"")
	}
	if safetyIdx >= scopeIdx {
		t.Fatalf("expected ATURAN SAFETY (index %d) to precede ATURAN SCOPE (index %d)", safetyIdx, scopeIdx)
	}
}

// TestDialogPromptDeclaresScopeBoundary covers the initial-submission prompt,
// which must also refuse task requests while keeping safety on top.
func TestDialogPromptDeclaresScopeBoundary(t *testing.T) {
	safetyIdx := strings.Index(DialogSystemPrompt, "ATURAN SAFETY")
	scopeIdx := strings.Index(DialogSystemPrompt, "ATURAN SCOPE")

	if safetyIdx < 0 {
		t.Fatal("expected DialogSystemPrompt to contain \"ATURAN SAFETY\"")
	}
	if scopeIdx < 0 {
		t.Fatal("expected DialogSystemPrompt to contain \"ATURAN SCOPE\"")
	}
	if safetyIdx >= scopeIdx {
		t.Fatalf("expected ATURAN SAFETY (index %d) to precede ATURAN SCOPE (index %d)", safetyIdx, scopeIdx)
	}
}

// TestDialogPromptOutputContractUnchanged is a regression guard: the dialog
// prompt's JSON shape is stored in a DB column and consumed by the frontend and
// validateDialog. The out_of_scope field belongs to the continuation prompt only
// and must never leak into this contract.
func TestDialogPromptOutputContractUnchanged(t *testing.T) {
	if strings.Contains(DialogSystemPrompt, "out_of_scope") {
		t.Error("DialogSystemPrompt must not declare an out_of_scope field: it would change the stored dialog JSON shape")
	}
}

// TestClassificationPromptDeclaresSafety guards that ClassificationSystemPrompt
// includes explicit safety rules for crisis and self-harm.
func TestClassificationPromptDeclaresSafety(t *testing.T) {
	if !strings.Contains(ClassificationSystemPrompt, "ATURAN SAFETY") {
		t.Fatal("expected ClassificationSystemPrompt to contain \"ATURAN SAFETY\"")
	}
	if !strings.Contains(ClassificationSystemPrompt, "safety_triggered") {
		t.Fatal("expected ClassificationSystemPrompt to mention \"safety_triggered\"")
	}
}
