// Package parser decodes the IQPlus Order payload (record type 16).
//
// Wire layout (per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.6):
//
//	Code | Time | OrderCommand | OrderNumber | Price | Volume |
//	Broker | Balance | Investor | NoReference
//
// OrderCommand:
//   0 = Bid (add to bid book)
//   1 = Offer (add to ask book)
//   2 = Cancel Bid (remove order from bid book)
//   3 = Cancel Offer (remove order from ask book)
//
// Investor: "F" foreign, "D" domestic.
// Broker: "--" during live RG board (regulation), real code on resend.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// jakartaLoc is the IDX trading timezone. Order Time fields are always
// HHMMSS in WIB; we combine with envelope.Date and convert to UTC.
var jakartaLoc = time.FixedZone("WIB", 7*3600)

// ResolveTimestamp combines the publisher envelope's frame Date
// (YYYYMMDD, vendor stamp at frame emit) with the order's HHMMSS time.
// If the naive combine puts the order *after* the envelope's own emit
// time, the vendor sent the frame past midnight WIB — we subtract one
// day so the order falls on the actual trading day.
//
// Example (late post-midnight resend):
//
//	envelope: 20260507 / 000200 (vendor emitted at 00:02 WIB Day+1)
//	order:    165900            (matched at 16:59 WIB the previous day)
//	naive:    2026-05-07 16:59 WIB → in the future → roll back 1 day
//	result:   2026-05-06 16:59 WIB
//
// All inputs interpreted as Asia/Jakarta; output is UTC.
func ResolveTimestamp(envDate, envTime, orderTime string) (time.Time, error) {
	if len(envDate) != 8 {
		return time.Time{}, fmt.Errorf("envelope date %q invalid", envDate)
	}
	if len(envTime) != 6 {
		return time.Time{}, fmt.Errorf("envelope time %q invalid", envTime)
	}
	if len(orderTime) != 6 {
		return time.Time{}, fmt.Errorf("order time %q invalid", orderTime)
	}
	envTs, err := time.ParseInLocation("20060102150405", envDate+envTime, jakartaLoc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse envelope ts: %w", err)
	}
	candidate, err := time.ParseInLocation("20060102150405", envDate+orderTime, jakartaLoc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse order ts: %w", err)
	}
	if candidate.After(envTs) {
		candidate = candidate.AddDate(0, 0, -1)
	}
	return candidate.UTC(), nil
}

// OrderCommand enum.
type OrderCommand int

const (
	CmdAddBid      OrderCommand = 0
	CmdAddOffer    OrderCommand = 1
	CmdCancelBid   OrderCommand = 2
	CmdCancelOffer OrderCommand = 3
)

// IsAdd reports whether this command adds an order to the book.
func (c OrderCommand) IsAdd() bool { return c == CmdAddBid || c == CmdAddOffer }

// IsCancel reports whether this command cancels an existing order.
func (c OrderCommand) IsCancel() bool { return c == CmdCancelBid || c == CmdCancelOffer }

// IsBidSide reports whether this command operates on the bid book.
func (c OrderCommand) IsBidSide() bool { return c == CmdAddBid || c == CmdCancelBid }

// Order is one parsed Type 16 frame.
type Order struct {
	Stock       string
	Time        string // HHMMSS (server clock, WIB) — vendor raw, from data parts[1]
	Command     OrderCommand
	OrderNumber int64   // unique per-day across all stocks
	Price       float64 // price tick
	Volume      int64   // initial order quantity
	Broker      string  // "--" during live RG, real code on resend
	Balance     int64   // remaining quantity (vendor field; not always reliable)
	Investor    string  // "F" or "D"
	NoReference int64
	RawDate     string // vendor wall-clock date "YYYYMMDD" — set by caller from envelope.Date (Order payload has no date field of its own)
}

// IsForeign reports whether this is a foreign-investor order.
func (o Order) IsForeign() bool { return o.Investor == "F" }

// Parse decodes a Type 16 payload (segment after "IQP|date|time|seq|16|").
func Parse(data string) (Order, error) {
	parts := strings.Split(data, "|")
	if len(parts) < 10 {
		return Order{}, fmt.Errorf("order: expected 10 fields, got %d", len(parts))
	}

	o := Order{
		Stock: strings.TrimSpace(parts[0]),
		Time:  strings.TrimSpace(parts[1]),
	}
	if o.Stock == "" {
		return Order{}, fmt.Errorf("order: missing stock code")
	}

	cmdInt, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return Order{}, fmt.Errorf("order: invalid command %q: %w", parts[2], err)
	}
	if cmdInt < 0 || cmdInt > 3 {
		return Order{}, fmt.Errorf("order: unknown command %d", cmdInt)
	}
	o.Command = OrderCommand(cmdInt)

	o.OrderNumber, err = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
	if err != nil {
		return Order{}, fmt.Errorf("order: invalid order_number %q: %w", parts[3], err)
	}

	price, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	if err != nil {
		return Order{}, fmt.Errorf("order: invalid price %q: %w", parts[4], err)
	}
	o.Price = price

	o.Volume, _ = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	o.Broker = strings.TrimSpace(parts[6])
	o.Balance, _ = strconv.ParseInt(strings.TrimSpace(parts[7]), 10, 64)
	o.Investor = strings.TrimSpace(parts[8])
	o.NoReference, _ = strconv.ParseInt(strings.TrimSpace(parts[9]), 10, 64)

	return o, nil
}
