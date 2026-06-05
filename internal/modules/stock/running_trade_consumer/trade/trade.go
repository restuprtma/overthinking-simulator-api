// Package trade parses the IQPlus Trade payload (record type 15) embedded
// in the publisher envelope's `data` field.
//
// Wire layout (per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.4):
//
//	Code | Date | Time | TradeNumber | TradeCommand | Price | Volume |
//	Buyer | BuyerType | Seller | SellerType | BuyerOrderNum | SellerOrderNum
//
// TradeCommand: 0 = matched, 1 = withdrawn.
// During market hours Buyer/Seller are masked as "--"; real broker codes
// only appear in resend records (type 27, see internal/.../trade_resend).
package trade

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Trade is the canonical parsed representation of a single matched trade.
//
// Market is NOT in the IQPlus Type 15 wire format — IDX board (RG/TN/NG)
// must be set externally by the consumer (typically from per-deployment
// config). Empty string means "not set"; downstream code should treat that
// as "fall back to default market".
type Trade struct {
	Code          string    // stock code, e.g. "BBCA"
	Timestamp     time.Time // trade timestamp in UTC (converted from WIB)
	TradeNumber   int64
	Command       int     // 0 = matched, 1 = withdrawn
	Price         float64 // last traded price
	Volume        int64   // last traded volume (in shares, not lots)
	Buyer         string  // broker code or "--" during live session
	BuyerType     string  // "F" = foreign, "D" = domestic
	Seller        string
	SellerType    string
	BuyerOrderNo  int64
	SellerOrderNo int64
	Market        string // "RG" / "TN" / "NG" — set by consumer, not parser
	RawDate       string // vendor wall-clock date "20260518" (fields[1] raw)
	RawTime       string // vendor wall-clock time "171517"   (fields[2] raw)
}

// IsMatched returns true for normal matched trades — withdrawn trades
// (Command=1) should not feed OHLCV.
func (t Trade) IsMatched() bool { return t.Command == 0 }

// DeriveMarket returns the IDX board (RG/TN/NG) from a stock code suffix.
//
// Convention: IDX appends "NG" / "TN" to the underlying ticker for those
// boards; RG is the default with no suffix. Examples:
//
//	BBCA      → RG (default)
//	BBCANG    → NG (Negosiasi)
//	BBCATN    → TN (Tunai)
//	BBCAZPCZ6A → RG (warrant/option, NOT a board suffix)
//
// The stock code is returned UNCHANGED — caller decides whether to strip
// the suffix in their own model. This keeps the IDX feed identifier
// faithful and preserves DEDUP key uniqueness.
func DeriveMarket(code string) string {
	if len(code) >= 5 {
		switch code[len(code)-2:] {
		case "NG":
			return "NG"
		case "TN":
			return "TN"
		}
	}
	return "RG"
}

// jakartaLoc is the IDX trading timezone. Trade timestamps from IQPlus are
// always WIB (UTC+7); we convert to UTC for consistent storage.
var jakartaLoc = time.FixedZone("WIB", 7*3600)

// Parse decodes a Trade record's `data` field (the segment after
// "IQP|date|time|seq|15|" in the original frame).
func Parse(data string) (Trade, error) {
	fields := strings.Split(data, "|")
	if len(fields) < 13 {
		return Trade{}, fmt.Errorf("trade: expected 13 fields, got %d", len(fields))
	}

	ts, err := parseTimestamp(fields[1], fields[2])
	if err != nil {
		return Trade{}, fmt.Errorf("trade: timestamp: %w", err)
	}

	tradeNo, _ := strconv.ParseInt(fields[3], 10, 64)
	cmd, _ := strconv.Atoi(fields[4])
	price, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return Trade{}, fmt.Errorf("trade: price %q: %w", fields[5], err)
	}
	volume, err := strconv.ParseInt(fields[6], 10, 64)
	if err != nil {
		return Trade{}, fmt.Errorf("trade: volume %q: %w", fields[6], err)
	}
	buyerOrder, _ := strconv.ParseInt(fields[11], 10, 64)
	sellerOrder, _ := strconv.ParseInt(fields[12], 10, 64)

	return Trade{
		Code:          strings.TrimSpace(fields[0]),
		Timestamp:     ts,
		RawDate:       strings.TrimSpace(fields[1]),
		RawTime:       strings.TrimSpace(fields[2]),
		TradeNumber:   tradeNo,
		Command:       cmd,
		Price:         price,
		Volume:        volume,
		Buyer:         strings.TrimSpace(fields[7]),
		BuyerType:     strings.TrimSpace(fields[8]),
		Seller:        strings.TrimSpace(fields[9]),
		SellerType:    strings.TrimSpace(fields[10]),
		BuyerOrderNo:  buyerOrder,
		SellerOrderNo: sellerOrder,
	}, nil
}

// parseTimestamp combines IQPlus Date (YYYYMMDD) + Time (HHMMSS) into a
// UTC time.Time. The source values are interpreted as Asia/Jakarta (WIB).
func parseTimestamp(date, ts string) (time.Time, error) {
	if len(date) != 8 || len(ts) != 6 {
		return time.Time{}, fmt.Errorf("invalid date/time format: %q %q", date, ts)
	}
	parsed, err := time.ParseInLocation("20060102150405", date+ts, jakartaLoc)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
