// Package quote parses the IQPlus Quote payload (record type 14) embedded
// in the publisher envelope's `data` field.
//
// Wire layout (per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.3):
//
//	0;<code>|1;<name>|2;<xgroup>|...|79;<pct_change>
//
// Each "FID;value" group is one Field Identification Number with its
// current value. Groups are pipe-separated. Quote messages are
// **incremental** — vendor sends only the FIDs that changed.
//
// FID 0 (Code) MUST be present in every Quote message; the consumer
// uses it as the Redis hash key.
package quote

import (
	"fmt"
	"strings"
)

// Quote is the parsed representation of one Quote frame.
type Quote struct {
	Code   string            // FID 0 — required
	Fields map[string]string // raw FID number → value, e.g. {"60": "5400"}
}

// Parse decodes a Quote record's `data` field (the segment after
// "IQP|date|time|seq|14|" in the original frame).
func Parse(data string) (Quote, error) {
	if data == "" {
		return Quote{}, fmt.Errorf("quote: empty payload")
	}

	q := Quote{Fields: make(map[string]string, 16)}

	for _, group := range strings.Split(data, "|") {
		semi := strings.IndexByte(group, ';')
		if semi <= 0 {
			// Skip malformed groups (no FID or no separator).
			continue
		}
		fid := strings.TrimSpace(group[:semi])
		val := strings.TrimSpace(group[semi+1:])
		if fid == "" {
			continue
		}
		q.Fields[fid] = val
		if fid == "0" {
			q.Code = val
		}
	}

	if q.Code == "" {
		return Quote{}, fmt.Errorf("quote: missing FID 0 (code)")
	}
	return q, nil
}

// FidAlias maps known FID numbers to friendly field names per spec §5.3.
//
// Consumers can access either form in Redis:
//
//	HGET quote:BBCA prev_close   → "5400"
//	HGET quote:BBCA 60           → also "5400"  (raw FID also stored)
//
// FIDs not listed here are stored only by their raw number.
var FidAlias = map[string]string{
	"0":  "code",
	"1":  "name",
	"2":  "xgroup",
	"3":  "status",
	"4":  "listing_date",
	"5":  "group",
	"6":  "issuer_id",
	"7":  "listed_shares",
	"8":  "tradable_shares",
	"9":  "ipo",
	"10": "order_book_id",
	"11": "base_price",
	"12": "currency",
	"13": "remark",
	"14": "eps",
	"15": "isin",
	"16": "foreign_limit",
	"17": "sector_name",
	"18": "industry_name",
	"19": "expiry_date",
	"20": "underlying",
	"21": "nta",
	"22": "contract_size",
	"23": "underwriter",
	"24": "bid_price",
	"25": "verb",
	"26": "strike_price",
	"27": "high_bid_price",
	"28": "weight",
	"29": "low_bid_price",
	"30": "maturity_date",
	"31": "bid_volume",
	"32": "xgroup1",
	"33": "security_type",
	"34": "flag",
	"35": "indicator",
	"36": "rsi",
	"37": "high5",
	"38": "offer_orders",
	"39": "offer_price",
	"40": "low5",
	"41": "index",
	"42": "high_offer_price",
	"43": "frg_bought_freq",
	"44": "low_offer_price",
	"45": "dom_bought_freq",
	"46": "offer_volume",
	"47": "frg_sold_freq",
	"48": "dom_sold_freq",
	"49": "frg_bought_vol",
	"50": "dom_bought_vol",
	"51": "frg_sold_vol",
	"52": "dom_sold_vol",
	"53": "bid_orders",
	"54": "open_price",
	"55": "theoretical_price",
	"56": "last_price",
	"57": "high_price",
	"58": "theoretical_vol",
	"59": "low_price",
	"60": "prev_close", // ← prev day close
	"61": "close_date",
	"65": "x_base_val",
	"66": "x_market_val",
	"67": "change",
	"68": "ratio",
	"69": "rec_date",
	"70": "board",
	"71": "source",
	"72": "volume",
	"73": "share_lot",
	"74": "frg_bought_val",
	"75": "frg_sold_val",
	"76": "dom_bought_val",
	"77": "dom_sold_val",
	"78": "avg",
	"79": "pct_change",
}
