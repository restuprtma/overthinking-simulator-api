package domain

import "time"

// ConversationState tracks how far a reflection's dialog has progressed.
const (
	ConversationInitial   = "initial"
	ConversationContinued = "continued"
	ConversationFinal     = "final"
)

// Distortion is a single detected cognitive distortion with its intensity.
type Distortion struct {
	ID        string `json:"id"`
	Intensity int    `json:"intensity"`
}

// DialogTurn is one exchange in the "Si Cemas vs Si Realistis" debate.
type DialogTurn struct {
	Speaker string `json:"speaker"` // "cemas" or "realistis"
	Text    string `json:"text"`
}

// Reflection is the persisted result of a reflection session for a user.
type Reflection struct {
	ID                   string       `json:"id"`
	UserID               string       `json:"user_id"`
	Thought              string       `json:"thought"`
	DetectedDistortions  []Distortion `json:"detected_distortions"`
	CoreFear             string       `json:"core_fear"`
	Dialog               []DialogTurn `json:"dialog"`
	ActionableSuggestion string       `json:"actionable_suggestion"`
	SafetyTriggered      bool         `json:"safety_triggered"`
	SafetyResponse       *string      `json:"safety_response,omitempty"`
	ConversationState    string       `json:"conversation_state"`
	TotalTurns           int          `json:"total_turns"`
	CreatedAt            time.Time    `json:"created_at"`
}

// ContinueResponse is the response for continuing an interactive conversation.
type ContinueResponse struct {
	NewTurn           DialogTurn   `json:"new_turn"`
	UpdatedDialog     []DialogTurn `json:"dialog_updated"`
	ConversationState string       `json:"conversation_state"`
	TotalTurns        int          `json:"total_turns"`
}

// DialogState is the post-append snapshot returned by the repository after
// atomically extending a reflection's dialog.
type DialogState struct {
	Dialog            []DialogTurn
	ConversationState string
	TotalTurns        int
}
