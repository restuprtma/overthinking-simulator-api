// Package parser decodes the IQPlus payloads for the low-volume "market
// metadata" record types — Control (13), Trading Status (57), Activity
// (39), Trading Summary (130), and Top 20 (17).
//
// One parser per record type, each returning a strongly-typed event the
// service layer dispatches to the Redis sink.
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// ============================================================================
// Type 13 — Control
// ============================================================================

// Control state of feed connection to IQPlus central.
type Control struct {
	State string // "UP" or "DOWN"
}

// ParseControl decodes a Type 13 payload. The data is a single byte:
// "0" = UP, "1" = DOWN (per spec §5.1).
func ParseControl(data string) (Control, error) {
	switch strings.TrimSpace(data) {
	case "0":
		return Control{State: "UP"}, nil
	case "1":
		return Control{State: "DOWN"}, nil
	default:
		return Control{}, fmt.Errorf("control: unknown state %q", data)
	}
}

// ============================================================================
// Type 57 — Trading Status
// ============================================================================

// Status describes the current trading session phase.
type Status struct {
	Code        string // "1", "3", "a", "b", ... per spec §5.2
	Description string // human-readable, e.g. "Begin first session"
	Label       string // friendly slug, e.g. "begin_first_session" (or "" if unknown)
}

// statusLabel maps spec status codes to a slug for ergonomic Redis lookups.
var statusLabel = map[string]string{
	"1": "begin_sending",
	"3": "begin_first_session",
	"4": "end_first_session",
	"5": "begin_second_session",
	"6": "end_second_session",
	"7": "end_sending",
	"8": "begin_pre_opening",
	"9": "end_pre_opening",
	"a": "begin_pre_closing",
	"b": "end_pre_closing",
	"c": "begin_post_trading",
	"d": "end_post_trading",
	"e": "trading_suspension",
	"f": "trading_activation",
	"g": "board_suspension",
	"h": "board_activation",
	"i": "instrument_suspension",
	"j": "instrument_activation",
	"k": "market_suspension",
	"l": "market_activation",
}

// ParseStatus decodes a Type 57 payload. Layout: <code>|<description>.
func ParseStatus(data string) (Status, error) {
	parts := strings.SplitN(data, "|", 2)
	if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
		return Status{}, fmt.Errorf("status: empty payload")
	}
	code := strings.TrimSpace(parts[0])
	desc := ""
	if len(parts) == 2 {
		desc = strings.TrimSpace(parts[1])
	}
	return Status{
		Code:        code,
		Description: desc,
		Label:       statusLabel[code],
	}, nil
}

// ============================================================================
// Type 39 — Stock Activity (market-wide counters)
// ============================================================================

// Activity is the market-wide stock count snapshot.
type Activity struct {
	Inactive int64
	Active   int64
	Down     int64
	NoChange int64
	Up       int64
}

// ParseActivity decodes a Type 39 payload.
// Layout: inactive|active|down|nochg|up
func ParseActivity(data string) (Activity, error) {
	parts := strings.Split(data, "|")
	if len(parts) < 5 {
		return Activity{}, fmt.Errorf("activity: expected 5 fields, got %d", len(parts))
	}
	a := Activity{}
	a.Inactive, _ = strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	a.Active, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	a.Down, _ = strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	a.NoChange, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
	a.Up, _ = strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
	return a, nil
}

// ============================================================================
// Type 130 — Trading Summary (per stype × board)
// ============================================================================

// Summary aggregates per-board statistics for one share-type/board pair.
type Summary struct {
	Stype       string // raw: "0".."8"; alias via SummaryStypeAlias
	StypeLabel  string // "ordi", "preop", ... (or "" if unknown)
	Board       string // RG / TN / NG
	Frequency   int64
	Volume      int64
	Value       int64
	FBoughtFreq int64
	FBoughtVol  int64
	FBoughtVal  int64
	FSoldFreq   int64
	FSoldVol    int64
	FSoldVal    int64
}

// SummaryStypeAlias maps Trading Summary stype codes to slugs.
var SummaryStypeAlias = map[string]string{
	"0": "ordi",
	"1": "preop",
	"2": "right",
	"3": "waran",
	"4": "muti",
	"5": "accel",
	"6": "swaran",
	"7": "preferen",
	"8": "watchlist",
}

// ParseSummary decodes a Type 130 payload.
// Layout: Stype|Board|Frequency|Volume|Value|FBfreq|FBvol|FBval|FSfreq|FSvol|FSval
func ParseSummary(data string) (Summary, error) {
	parts := strings.Split(data, "|")
	if len(parts) < 11 {
		return Summary{}, fmt.Errorf("summary: expected 11 fields, got %d", len(parts))
	}
	s := Summary{
		Stype: strings.TrimSpace(parts[0]),
		Board: strings.TrimSpace(parts[1]),
	}
	s.StypeLabel = SummaryStypeAlias[s.Stype]
	s.Frequency, _ = strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	s.Volume, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
	s.Value, _ = strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
	s.FBoughtFreq, _ = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	s.FBoughtVol, _ = strconv.ParseInt(strings.TrimSpace(parts[6]), 10, 64)
	s.FBoughtVal, _ = strconv.ParseInt(strings.TrimSpace(parts[7]), 10, 64)
	s.FSoldFreq, _ = strconv.ParseInt(strings.TrimSpace(parts[8]), 10, 64)
	s.FSoldVol, _ = strconv.ParseInt(strings.TrimSpace(parts[9]), 10, 64)
	s.FSoldVal, _ = strconv.ParseInt(strings.TrimSpace(parts[10]), 10, 64)
	return s, nil
}

// ============================================================================
// Type 17 — Top 20 (snapshot list per category)
// ============================================================================

// Top20 is one full snapshot — replaces the previous list for the same
// category code.
type Top20 struct {
	TypeCode  string   // raw "0".."16"
	TypeLabel string   // friendly alias (e.g. "volume_rg") or "" if unknown
	Codes     []string // 20 stock or broker codes (may be <20 if vendor sends fewer)
}

// Top20Alias maps category code → slug. Kept in sync with spec §5.8.
var Top20Alias = map[string]string{
	"0":  "volume_rg",
	"1":  "value_rg",
	"2":  "freq_rg",
	"3":  "gainer_rg",
	"4":  "loser_rg",
	"5":  "pct_gainer_rg",
	"6":  "pct_loser_rg",
	"7":  "volume_nonrg",
	"8":  "value_nonrg",
	"9":  "freq_nonrg",
	"10": "gainer_nonrg",
	"11": "loser_nonrg",
	"12": "pct_gainer_nonrg",
	"13": "pct_loser_nonrg",
	"14": "volume_broker",
	"15": "value_broker",
	"16": "freq_broker",
}

// ParseTop20 decodes a Type 17 payload.
// Layout: <TopType>|Code1|Code2|...|Code20
func ParseTop20(data string) (Top20, error) {
	parts := strings.Split(data, "|")
	if len(parts) < 2 {
		return Top20{}, fmt.Errorf("top20: expected at least type+1 code, got %d", len(parts))
	}
	t := Top20{
		TypeCode: strings.TrimSpace(parts[0]),
		Codes:    make([]string, 0, len(parts)-1),
	}
	t.TypeLabel = Top20Alias[t.TypeCode]
	for _, c := range parts[1:] {
		c = strings.TrimSpace(c)
		if c != "" {
			t.Codes = append(t.Codes, c)
		}
	}
	if len(t.Codes) == 0 {
		return Top20{}, fmt.Errorf("top20: no codes for type %q", t.TypeCode)
	}
	return t, nil
}
