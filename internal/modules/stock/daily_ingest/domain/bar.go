// Package domain holds the canonical struct for one daily OHLCV bar.
package domain

import "time"

// DailyBar is one row of stock.prices_daily — output of QuestDB SAMPLE BY 1d
// over the trades table, ready to UPSERT into Postgres.
//
// Date is the trading day in WIB calendar (Asia/Jakarta), not UTC. The
// QuestDB query aligns its 1d bucket to WIB; the consumer treats Date as
// a wall-clock date.
type DailyBar struct {
	StockCode string
	Date      time.Time
	Market    string
	Open      int64
	High      int64
	Low       int64
	Close     int64
	Volume    int64
	Value     int64
	Freq      int64
}
