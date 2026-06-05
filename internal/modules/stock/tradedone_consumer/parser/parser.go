// Package parser decodes the IQPlus Trade Done payload (record type 40).
//
// Wire layout (per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.10):
//
//	Code | Price | Bvol | Svol | bfreq | Sfreq | bfreqF | BvolF | SfreqF | SvolF
//
// Each frame represents the **cumulative** buy/sell volume + frequency
// for one (stock, price) pair throughout the trading session. Vendor
// resends with updated totals — sink should overwrite, not accumulate.
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// TradeDone is one volume-at-price snapshot.
type TradeDone struct {
	Code   string  // stock code
	Price  float64 // price level
	BVol   int64   // cumulative buy volume at this price
	SVol   int64   // cumulative sell volume at this price
	BFreq  int64   // buy trade count
	SFreq  int64   // sell trade count
	BFreqF int64   // foreign buy frequency
	BVolF  int64   // foreign buy volume
	SFreqF int64   // foreign sell frequency
	SVolF  int64   // foreign sell volume
}

// Parse decodes a Type 40 payload (segment after "IQP|date|time|seq|40|").
func Parse(data string) (TradeDone, error) {
	parts := strings.Split(data, "|")
	if len(parts) < 10 {
		return TradeDone{}, fmt.Errorf("tradedone: expected 10 fields, got %d", len(parts))
	}

	td := TradeDone{
		Code: strings.TrimSpace(parts[0]),
	}
	if td.Code == "" {
		return TradeDone{}, fmt.Errorf("tradedone: missing stock code")
	}

	price, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return TradeDone{}, fmt.Errorf("tradedone: invalid price %q: %w", parts[1], err)
	}
	td.Price = price

	td.BVol, _ = strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	td.SVol, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
	td.BFreq, _ = strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
	td.SFreq, _ = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	td.BFreqF, _ = strconv.ParseInt(strings.TrimSpace(parts[6]), 10, 64)
	td.BVolF, _ = strconv.ParseInt(strings.TrimSpace(parts[7]), 10, 64)
	td.SFreqF, _ = strconv.ParseInt(strings.TrimSpace(parts[8]), 10, 64)
	td.SVolF, _ = strconv.ParseInt(strings.TrimSpace(parts[9]), 10, 64)

	return td, nil
}
