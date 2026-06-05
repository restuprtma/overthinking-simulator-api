package client

import "time"

// Record types per IQPlus Data Feed v4.0.0 spec.
const (
	TypeControl       = 13
	TypeQuote         = 14
	TypeTrade         = 15
	TypeOrder         = 16
	TypeTop20         = 17
	TypeBestQuote     = 18
	TypeResendOrder   = 26
	TypeResendTrade   = 27
	TypeNews          = 36
	TypeActivity      = 39
	TypeTradeDone     = 40
	TypeTradingStatus = 57
	TypeNBSStock      = 58
	TypeNBSBroker     = 59
	TypeTradingSummry = 130
	TypeAuth          = 149
)

// Record is one parsed IQPlus feed record.
//
// Wire format: IQP|Date|Time|Sequence#|RecordType|Data\r\n
//
// RawData is everything after RecordType (the Data segment), kept verbatim
// so downstream consumers can re-parse with their own schema.
type Record struct {
	Date       string    // YYYYMMDD
	Time       string    // HHMMSS (server-side)
	Sequence   int64     // unique per day, starts at 1
	RecordType int       // see Type* constants
	RawData    string    // pipe/semicolon separated payload
	ReceivedAt time.Time // local UTC timestamp when frame fully received
}
