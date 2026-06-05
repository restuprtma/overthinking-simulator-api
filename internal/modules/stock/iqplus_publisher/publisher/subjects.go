package publisher

import (
	"fmt"
	"strings"

	"tuai/internal/modules/stock/iqplus_publisher/client"
)

// Subject naming follows docs/infra/topology.md §4.2.
//
// Format: idx.<kategori>.<identifier>
//
//	13  Control          → idx.status.feed
//	14  Quote (saham)    → idx.quote.<stockcode>
//	14  Quote (regional) → idx.quote.regional.<symbol>
//	15  Trade            → idx.trade.<stockcode>
//	16  Order            → idx.order.<stockcode>
//	17  Top 20           → idx.top20.<category_code>
//	18  Best Quote       → idx.bestquote.<stockcode>
//	26  Resend Order     → idx.resend.order.<stockcode>
//	27  Resend Trade     → idx.resend.trade.<stockcode>
//	36  News             → idx.news.<category>
//	39  Activity         → idx.activity.market
//	40  Trade Done       → idx.tradedone.<stockcode>
//	57  Trading Status   → idx.status.session
//	58  NBS Stock        → idx.nbs.stock.<stockcode>
//	59  NBS Broker       → idx.nbs.broker.<brokercode>
//	130 Trading Summary  → idx.summary.<stype>.<board>
//
// Identifier segments preserve UPPERCASE (matches topology.md examples and
// IDX ticker convention) but strip leading "-" markers IQPlus uses for
// non-stock instruments (-FTSE, -LGD, etc).

// Stream names per topology.md §4.3.
const (
	StreamTick  = "IDX_TICK"  // trade, order, tradedone, resend
	StreamQuote = "IDX_QUOTE" // quote, bestquote
	StreamMeta  = "IDX_META"  // status, activity, summary, top20, nbs
	StreamNews  = "IDX_NEWS"  // news
)

// SubjectFor derives the JetStream subject for a parsed IQPlus record.
//
// Returns "" if the record has no defined routing (caller should drop).
func SubjectFor(r client.Record) string {
	switch r.RecordType {
	case client.TypeControl:
		return "idx.status.feed"
	case client.TypeTradingStatus:
		return "idx.status.session"
	case client.TypeActivity:
		return "idx.activity.market"

	case client.TypeQuote:
		// FID 0 = code. Regional/commodity/currency codes contain "-".
		code := quoteCode(r.RawData)
		if code == "" {
			return ""
		}
		if isNonStockCode(code) {
			return "idx.quote.regional." + normalizeIdent(code)
		}
		return "idx.quote." + normalizeIdent(code)

	case client.TypeTrade:
		return tickerSubject(r.RawData, "idx.trade.")
	case client.TypeOrder:
		return tickerSubject(r.RawData, "idx.order.")
	case client.TypeBestQuote:
		return tickerSubject(r.RawData, "idx.bestquote.")
	case client.TypeTradeDone:
		return tickerSubject(r.RawData, "idx.tradedone.")
	case client.TypeResendTrade:
		return tickerSubject(r.RawData, "idx.resend.trade.")
	case client.TypeResendOrder:
		return tickerSubject(r.RawData, "idx.resend.order.")
	case client.TypeNBSStock:
		return tickerSubject(r.RawData, "idx.nbs.stock.")

	case client.TypeNBSBroker:
		// Layout: Broker_code|Stock_code|...
		broker := nthField(r.RawData, 0)
		if broker == "" {
			return ""
		}
		return "idx.nbs.broker." + normalizeIdent(broker)

	case client.TypeTop20:
		// Layout: TopType|Code1|...
		topType := nthField(r.RawData, 0)
		if topType == "" {
			return "idx.top20.unknown"
		}
		return "idx.top20." + normalizeIdent(topType)

	case client.TypeTradingSummry:
		// Layout: Stype|Board|Frequency|...
		stype := nthField(r.RawData, 0)
		board := nthField(r.RawData, 1)
		if stype == "" || board == "" {
			return ""
		}
		return "idx.summary." + normalizeIdent(stype) + "." + normalizeIdent(board)

	case client.TypeNews:
		// Layout: News_type|Num_packet|Current_packet|News_id|Date|Time|Category|...
		category := nthField(r.RawData, 6)
		if category == "" {
			return "idx.news.unknown"
		}
		return "idx.news." + normalizeIdent(category)

	case client.TypeAuth:
		// Login response — never republish.
		return ""

	default:
		return ""
	}
}

// StreamFor maps a derived subject to its expected JetStream stream name
// per topology.md §4.3. Returns "" when the subject does not belong to any
// configured stream (publisher should treat as an error).
func StreamFor(subject string) string {
	switch {
	case strings.HasPrefix(subject, "idx.trade."),
		strings.HasPrefix(subject, "idx.order."),
		strings.HasPrefix(subject, "idx.tradedone."),
		strings.HasPrefix(subject, "idx.resend."):
		return StreamTick

	case strings.HasPrefix(subject, "idx.quote."),
		strings.HasPrefix(subject, "idx.bestquote."):
		return StreamQuote

	case strings.HasPrefix(subject, "idx.status."),
		strings.HasPrefix(subject, "idx.activity."),
		strings.HasPrefix(subject, "idx.summary."),
		strings.HasPrefix(subject, "idx.top20."),
		strings.HasPrefix(subject, "idx.nbs."):
		return StreamMeta

	case strings.HasPrefix(subject, "idx.news."):
		return StreamNews
	}
	return ""
}

// AllStreams lists the streams this publisher will route to. Surfaced for
// startup logging and for the streams.md setup script.
func AllStreams() []string {
	return []string{StreamTick, StreamQuote, StreamMeta, StreamNews}
}

// tickerSubject builds <prefix><ticker> for records whose RawData starts
// with the stock code as the first pipe-separated field.
func tickerSubject(raw, prefix string) string {
	code := nthField(raw, 0)
	if code == "" {
		return ""
	}
	return prefix + normalizeIdent(code)
}

// nthField returns the n-th pipe-separated field (0-indexed) trimmed of
// whitespace. Empty string if the field is missing.
func nthField(raw string, n int) string {
	idx := 0
	start := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == '|' {
			if idx == n {
				return strings.TrimSpace(raw[start:i])
			}
			idx++
			start = i + 1
		}
	}
	if idx == n {
		return strings.TrimSpace(raw[start:])
	}
	return ""
}

// quoteCode extracts FID 0 (code) from a Quote payload of the form
// "0;BBCA|1;Bank Central Asia|...".
func quoteCode(raw string) string {
	for _, group := range strings.Split(raw, "|") {
		semi := strings.IndexByte(group, ';')
		if semi < 0 {
			continue
		}
		if group[:semi] == "0" {
			return strings.TrimSpace(group[semi+1:])
		}
	}
	return ""
}

// isNonStockCode reports whether a Quote code refers to a regional index,
// commodity, or currency rather than an IDX stock.
//
// IQPlus convention (spec §5.3.2):
//   - Regional/commodity codes start with "-" (e.g. "-FTSE", "-LGD")
//   - Currency codes contain a "-" between symbols (e.g. "AUD-USD")
//   - IDX stock codes are pure alphanumeric (e.g. "BBCA", "WIKA", "KBAG-W")
//
// Note: warrants like "KBAG-W" also contain "-". They are still stocks, so
// the heuristic is: leading "-" OR contains "-USD"/"-IDR"/etc currency
// suffix. To stay conservative we treat any code containing "-" as
// non-stock UNLESS it ends with "-W" (warrant) or "-R" (right). Adjust as
// vendor patterns become clearer.
func isNonStockCode(code string) bool {
	if strings.HasPrefix(code, "-") {
		return true
	}
	if !strings.Contains(code, "-") {
		return false
	}
	// Common warrant/right suffixes — keep these on the stock side.
	if strings.HasSuffix(code, "-W") || strings.HasSuffix(code, "-R") {
		return false
	}
	return true
}

// normalizeIdent prepares an identifier segment for use inside a NATS
// subject token. NATS forbids '.', '*', '>', and whitespace inside tokens;
// case is preserved (subjects are case-sensitive and topology.md uses
// uppercase tickers).
func normalizeIdent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "-") // strip IQPlus regional marker
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// errUnknownStream is returned by the publisher when a derived subject has
// no configured destination stream — almost always a bug in SubjectFor /
// StreamFor mappings.
var errUnknownStream = fmt.Errorf("no stream configured for subject")
