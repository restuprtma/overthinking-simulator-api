// Trading-status parser — Type 57. Payload after the envelope prefix is
// just `<status_code>|<description>` (description optional). See
// docs/iqplus/iqplus-data-feed-v4.0.0.md §5.2 for the full status enum.
//
// We surface three pieces of information:
//
//   - Code:    the single-character status code (vendor literal)
//   - Phase:   coarse-grained string used in the orderbook meta + wire
//              protocol — one of preopen / open / midday / preclose / post /
//              closed / init / unknown
//   - IsReset: whether this status should trigger engine.Reset() (session
//              boundary that invalidates accumulated order state)

package parser

import (
	"fmt"
	"strings"
)

// Status is one parsed Type 57 frame.
type Status struct {
	Code        string // vendor literal, e.g. "1", "8", "a"
	Description string
	Phase       string
	IsReset     bool
}

// ParseStatus decodes the data segment after `IQP|date|time|seq|57|`.
//
// Example input: "1|Begin sending records"
func ParseStatus(data string) (Status, error) {
	parts := strings.Split(data, "|")
	if len(parts) == 0 || parts[0] == "" {
		return Status{}, fmt.Errorf("status: empty payload")
	}
	code := strings.TrimSpace(parts[0])
	desc := ""
	if len(parts) > 1 {
		desc = strings.TrimSpace(parts[1])
	}
	phase, reset := phaseFor(code)
	return Status{
		Code:        code,
		Description: desc,
		Phase:       phase,
		IsReset:     reset,
	}, nil
}

// phaseFor maps a Type 57 status code to (coarse phase, is-reset).
// Reset only on the earliest "Begin sending records" — Pre-opening etc.
// happen daily but engine state at that point is already clean (or being
// rebuilt). If consumer cold-starts mid-day, the next day's "1" naturally
// resets.
func phaseFor(code string) (phase string, reset bool) {
	switch code {
	case "1":
		return "init", true
	case "3":
		return "open", false // begin first session
	case "4":
		return "midday", false
	case "5":
		return "open", false // begin second session
	case "6":
		return "closed", false
	case "7":
		return "closed", false
	case "8":
		return "preopen", false
	case "9":
		return "preopen", false
	case "a":
		return "preclose", false
	case "b":
		return "preclose", false
	case "c":
		return "post", false
	case "d":
		return "closed", false
	default:
		return "unknown", false
	}
}
