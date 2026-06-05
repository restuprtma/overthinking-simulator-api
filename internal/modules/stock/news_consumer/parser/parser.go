// Package parser decodes the IQPlus News payload (record type 36).
//
// Wire layout (per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.14):
//
//	News_type | Num_packet | Current_packet | News_id |
//	Date | Time | Category | Company_id | Headline | <Story chunk>
//
// Story is the LAST field — it MAY contain pipe characters (no escaping
// in the spec), so we use SplitN with limit=10 and treat everything past
// the 9th delimiter as Story.
//
// Multi-packet handling: a single news item is split into Num_packet
// frames (max 1024 char per frame). Headline + metadata fields repeat
// in every frame; only Story differs. Caller must reassemble — see
// internal/.../assembler.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// jakartaLoc is the IDX timezone — vendor sends Date/Time in WIB.
var jakartaLoc = time.FixedZone("WIB", 7*3600)

// Packet is one IQPlus news frame. A complete news item is N packets.
type Packet struct {
	NewsType      int       // 1 = OK
	NumPackets    int       // total packets for this News_id
	CurrentPacket int       // 1-indexed packet number
	NewsID        string    // unique vendor-assigned id
	Timestamp     time.Time // Date+Time → UTC
	Date          string    // raw YYYYMMDD (kept for storage)
	Time          string    // raw HHMMSS (kept for storage)
	Category      string    // e.g. "BIS"
	CompanyID     string    // ticker / subject of news
	Headline      string    // identical across all packets of same News_id
	StoryChunk    string    // this packet's slice of the full story
}

// Parse decodes a Type 36 payload (the segment after
// "IQP|date|time|seq|36|" in the original IQPlus frame).
func Parse(data string) (Packet, error) {
	parts := strings.SplitN(data, "|", 10)
	if len(parts) < 10 {
		return Packet{}, fmt.Errorf("news: expected 10 fields, got %d", len(parts))
	}

	newsType, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	numPackets, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || numPackets <= 0 {
		return Packet{}, fmt.Errorf("news: invalid num_packets %q", parts[1])
	}
	currentPacket, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil || currentPacket <= 0 || currentPacket > numPackets {
		return Packet{}, fmt.Errorf("news: invalid current_packet %q (max %d)",
			parts[2], numPackets)
	}

	newsID := strings.TrimSpace(parts[3])
	if newsID == "" {
		return Packet{}, fmt.Errorf("news: missing news_id")
	}

	date := strings.TrimSpace(parts[4])
	tm := strings.TrimSpace(parts[5])
	ts, err := parseTimestamp(date, tm)
	if err != nil {
		return Packet{}, fmt.Errorf("news: timestamp: %w", err)
	}

	return Packet{
		NewsType:      newsType,
		NumPackets:    numPackets,
		CurrentPacket: currentPacket,
		NewsID:        newsID,
		Timestamp:     ts,
		Date:          date,
		Time:          tm,
		Category:      strings.TrimSpace(parts[6]),
		CompanyID:     strings.TrimSpace(parts[7]),
		Headline:      strings.TrimSpace(parts[8]),
		StoryChunk:    parts[9], // intentionally NOT trimmed — preserve content
	}, nil
}

func parseTimestamp(date, tm string) (time.Time, error) {
	if len(date) != 8 || len(tm) != 6 {
		return time.Time{}, fmt.Errorf("invalid date/time format: %q %q", date, tm)
	}
	parsed, err := time.ParseInLocation("20060102150405", date+tm, jakartaLoc)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
