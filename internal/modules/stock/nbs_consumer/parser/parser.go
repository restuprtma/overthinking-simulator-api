// Package parser decodes the IQPlus Net Buy/Sell payloads — Type 58
// (NBS Stock, stock-centric) and Type 59 (NBS Broker, broker-centric).
//
// Both record types share the same 12 numeric fields; only the order of
// the first two identifier fields differs:
//
//	Type 58: Stock | Broker | Bfreq | Bvol | Blot | Bval | Bpct |
//	                                Sfreq | Svol | Slot | Sval | Spct
//
//	Type 59: Broker | Stock | <same 10 numeric fields as above>
//
// Vendor sends snapshots — sink overwrites previous value.
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// NBS is one Net Buy/Sell snapshot for a (stock, broker) pair. Source
// records the originating record type (58 or 59) for observability;
// downstream consumers don't need to differentiate.
type NBS struct {
	Stock  string  // always populated
	Broker string  // always populated
	BFreq  int64   // buy frequency
	BVol   int64   // buy volume (shares)
	BLot   int64   // buy lot
	BVal   int64   // buy value (rupiah)
	BPct   float64 // buy value percentage
	SFreq  int64
	SVol   int64
	SLot   int64
	SVal   int64
	SPct   float64
	Source int // 58 or 59
}

// ParseStock decodes a Type 58 payload (Stock|Broker|...).
func ParseStock(data string) (NBS, error) {
	return parse(data, 58, true)
}

// ParseBroker decodes a Type 59 payload (Broker|Stock|...).
func ParseBroker(data string) (NBS, error) {
	return parse(data, 59, false)
}

// parse handles both Type 58 and Type 59. stockFirst flips the first two
// field meanings.
func parse(data string, source int, stockFirst bool) (NBS, error) {
	parts := strings.Split(data, "|")
	if len(parts) < 12 {
		return NBS{}, fmt.Errorf("nbs: expected 12 fields, got %d", len(parts))
	}

	first := strings.TrimSpace(parts[0])
	second := strings.TrimSpace(parts[1])
	if first == "" || second == "" {
		return NBS{}, fmt.Errorf("nbs: missing identifier fields")
	}

	n := NBS{Source: source}
	if stockFirst {
		n.Stock = first
		n.Broker = second
	} else {
		n.Broker = first
		n.Stock = second
	}

	n.BFreq, _ = strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	n.BVol, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
	n.BLot, _ = strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
	n.BVal, _ = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	n.BPct, _ = strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
	n.SFreq, _ = strconv.ParseInt(strings.TrimSpace(parts[7]), 10, 64)
	n.SVol, _ = strconv.ParseInt(strings.TrimSpace(parts[8]), 10, 64)
	n.SLot, _ = strconv.ParseInt(strings.TrimSpace(parts[9]), 10, 64)
	n.SVal, _ = strconv.ParseInt(strings.TrimSpace(parts[10]), 10, 64)
	n.SPct, _ = strconv.ParseFloat(strings.TrimSpace(parts[11]), 64)

	return n, nil
}
