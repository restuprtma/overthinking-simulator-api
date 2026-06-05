// Package book is the in-memory order book state machine.
//
// Maintains:
//   - orderIdx: map[order_no] → orderEntry  (lookup for cancel/fill)
//   - books:    map[stock]   → {bid, ask}    (price-level aggregates per stock)
//   - dirty:    set of stocks that changed since last drain (both sides drained together)
//   - seq:      monotonic per-stock counter, incremented on every mutation
//
// Apply events:
//   - OnOrder: Type 16 frame (add/update bid/offer or cancel)
//   - OnTrade: Type 15 frame (reduce volume of buyer + seller orders;
//              if volume reaches 0, remove from book)
//   - Reset:   wipe all state (call at session begin / midnight)
//
// Concurrency: a single mutex guards all mutating + reading methods.
// The subscriber dispatches events sequentially in one goroutine, so
// contention is only between event handling and the snapshotter's
// DrainDirty call.
//
// Units: LevelStats.Lot stores shares (raw vendor volume). Conversion to
// IDX lot units (÷ LotSize) happens in the sink/serializer layer, not here,
// to keep state arithmetic exact.
package book

import (
	"sync"

	"tuai/internal/modules/stock/orderbook_consumer/parser"
)

// LotSize is the IDX standard lot size — 100 shares per lot for almost all
// equities on RG/NG/TN boards. Used by sink/snapshot serializers to convert
// internal share-denominated state into lot-denominated wire format.
const LotSize = 100

// Side enum.
type Side int

const (
	SideBid Side = iota
	SideAsk
)

func (s Side) String() string {
	if s == SideBid {
		return "bid"
	}
	return "ask"
}

// orderEntry tracks one open order — used to look up a cancel/fill target.
//
// Remaining is updated by add (re-emit from vendor with current Balance)
// and by fill (Type 15 match). It is the engine's source of truth for an
// order's contribution to its price level.
type orderEntry struct {
	Stock     string
	Price     float64
	Side      Side
	Remaining int64
	Foreign   bool   // F=foreign, D=domestic; false for "-"/unknown
	Broker    string // "--" during live RG
}

// LevelStats aggregate per price level. Field names intentionally short;
// public JSON serialization is done in the sink layer with field-renaming
// so this internal struct can keep readable names without bloating wire
// format. Lot/LotForeign stored in shares (raw vendor volume).
type LevelStats struct {
	Lot          int64 // total shares at this price (sum of orders' Remaining)
	OrderCount   int64 // count of distinct orders at this price
	LotForeign   int64 // subset of Lot from foreign orders
	OrderForeign int64 // subset of OrderCount from foreign orders
}

// Book holds price-level aggregates for one stock.
type Book struct {
	Bid map[float64]*LevelStats
	Ask map[float64]*LevelStats
}

func newBook() *Book {
	return &Book{
		Bid: make(map[float64]*LevelStats, 16),
		Ask: make(map[float64]*LevelStats, 16),
	}
}

// Engine is the order book state machine.
type Engine struct {
	mu     sync.Mutex
	orders map[int64]*orderEntry
	books  map[string]*Book
	// dirty: stock → true. Both sides are always drained together for a
	// dirty stock (cheaper bookkeeping; the snapshotter compares against
	// previous state to discover which side actually changed).
	dirty map[string]bool
	// seq: monotonic counter per stock. Incremented on every mutation that
	// affects the book (add/update/cancel/fill). Snapshot + delta carry
	// this value so the FE can detect gaps.
	seq   map[string]uint64
	stats Stats
}

// New constructs an empty engine.
func New() *Engine {
	return &Engine{
		orders: make(map[int64]*orderEntry, 100_000),
		books:  make(map[string]*Book, 1024),
		dirty:  make(map[string]bool, 1024),
		seq:    make(map[string]uint64, 1024),
	}
}

// OnOrder applies a Type 16 event to the book.
//
// For add (cmd 0/1): uses o.Balance as the order's remaining quantity (not
// o.Volume — Volume is the original placed quantity, Balance is the current
// remaining after any prior fills/cancels). Vendor may re-emit the same
// order_no with updated Balance; we treat that as an in-place update.
//
// For cancel (cmd 2/3): looks up the tracked order's recorded state and
// removes its contribution from the book.
func (e *Engine) OnOrder(o parser.Order) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch o.Command {
	case parser.CmdAddBid:
		e.add(o.OrderNumber, o.Stock, o.Price, SideBid, o.Balance, o.IsForeign(), o.Broker)
		e.stats.AddedBid++
	case parser.CmdAddOffer:
		e.add(o.OrderNumber, o.Stock, o.Price, SideAsk, o.Balance, o.IsForeign(), o.Broker)
		e.stats.AddedAsk++
	case parser.CmdCancelBid, parser.CmdCancelOffer:
		e.cancel(o.OrderNumber)
	}
}

// OnTrade applies a Type 15 event — reduces remaining of buyer + seller
// orders. Either order_no may be 0 (no associated tracked order); skip
// those. Volume is in shares (raw vendor value).
func (e *Engine) OnTrade(stock string, buyerOrderNo, sellerOrderNo, volume int64) {
	if volume <= 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if buyerOrderNo > 0 {
		e.fill(buyerOrderNo, volume)
	}
	if sellerOrderNo > 0 {
		e.fill(sellerOrderNo, volume)
	}
	e.stats.Matched++
}

// Reset wipes all state. Call at session begin (Type 57 status "1" or "3"),
// midnight cron, or consumer cold start.
//
// After reset, every stock's seq counter restarts at 0 — the next mutation
// will publish seq=1. Downstream FE must treat reset as a hard barrier
// (clear local state, await fresh snapshot).
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.orders = make(map[int64]*orderEntry, 100_000)
	e.books = make(map[string]*Book, 1024)
	e.dirty = make(map[string]bool, 1024)
	e.seq = make(map[string]uint64, 1024)
	e.stats.Resets++
}

// DirtySnapshot is one stock's drained state — both sides together, with
// the snapshot's seq number stamped from the engine's per-stock counter.
//
// Bid and Ask are deep copies; safe to use outside the engine lock.
type DirtySnapshot struct {
	Stock string
	Seq   uint64
	Bid   map[float64]LevelStats
	Ask   map[float64]LevelStats
}

// DrainDirty returns and clears the current dirty set, with a deep-copy
// snapshot of each dirty stock's full book (both sides) + the stock's
// current seq. The snapshotter uses this to compute deltas vs. its own
// previous snapshot and flush to Redis + NATS without holding the engine
// lock during I/O.
func (e *Engine) DrainDirty() []DirtySnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]DirtySnapshot, 0, len(e.dirty))
	for stock := range e.dirty {
		bk := e.books[stock]
		if bk == nil {
			continue
		}
		out = append(out, DirtySnapshot{
			Stock: stock,
			Seq:   e.seq[stock],
			Bid:   copyLevels(bk.Bid),
			Ask:   copyLevels(bk.Ask),
		})
	}
	e.dirty = make(map[string]bool, len(e.dirty))
	return out
}

func copyLevels(src map[float64]*LevelStats) map[float64]LevelStats {
	out := make(map[float64]LevelStats, len(src))
	for price, lvl := range src {
		if lvl.Lot <= 0 {
			continue
		}
		out[price] = *lvl
	}
	return out
}

// Stats are operational counters surfaced for logging.
type Stats struct {
	AddedBid     uint64
	AddedAsk     uint64
	CancelledBid uint64
	CancelledAsk uint64
	Matched      uint64
	UnknownOrder uint64 // cancel/fill referenced an unknown order_no
	Resets       uint64
	OpenOrders   int
	ActiveStocks int
}

// Snapshot returns a copy of current counters + bounded gauge values.
func (e *Engine) Snapshot() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.stats
	s.OpenOrders = len(e.orders)
	s.ActiveStocks = len(e.books)
	return s
}

// ------------------------------------------------------------------ helpers

// add inserts or updates an order on the book. If order_no already exists
// (vendor re-emit with new Balance, or rare modify-price scenario), the
// previous contribution is fully removed (including OrderCount) before the
// new contribution is added — net effect is a delta on the price level.
//
// The whole add/update is ONE logical event: only the final markChanged
// bumps seq, even if the duplicate-cleanup path mutates a different
// (stock,side). The first stock is still marked dirty so its drained
// state reflects the removal.
func (e *Engine) add(orderNo int64, stock string, price float64, side Side, lot int64, foreign bool, broker string) {
	if lot <= 0 || stock == "" || orderNo <= 0 {
		return
	}
	// Update path: existing order_no → fully remove old contribution first.
	// decOrderCount keeps freq correct when same-price update; markDirty
	// (no seq bump) flags the stock if it differs from the new one.
	if existing, ok := e.orders[orderNo]; ok {
		e.removeLevel(existing.Stock, existing.Side, existing.Price, existing.Remaining, existing.Foreign)
		e.decOrderCount(existing.Stock, existing.Side, existing.Price, existing.Foreign)
		e.markDirty(existing.Stock)
		delete(e.orders, orderNo)
	}

	e.orders[orderNo] = &orderEntry{
		Stock: stock, Price: price, Side: side,
		Remaining: lot, Foreign: foreign, Broker: broker,
	}
	e.addLevel(stock, side, price, lot, foreign)
	e.markChanged(stock)
}

// cancel removes an order entirely. Uses the engine's recorded side/price/
// remaining — the inbound cmd only signals "cancel"; we never trust the
// vendor's incoming Balance for cancel events (some vendors send 0).
func (e *Engine) cancel(orderNo int64) {
	o, ok := e.orders[orderNo]
	if !ok {
		e.stats.UnknownOrder++
		return
	}
	e.removeLevel(o.Stock, o.Side, o.Price, o.Remaining, o.Foreign)
	e.decOrderCount(o.Stock, o.Side, o.Price, o.Foreign)
	e.markChanged(o.Stock)
	delete(e.orders, orderNo)

	if o.Side == SideBid {
		e.stats.CancelledBid++
	} else {
		e.stats.CancelledAsk++
	}
}

// fill reduces an order's remaining by `volume` (in shares). Bounded by
// the order's current Remaining so partial-fill on a partly-resolved order
// can't go negative. Order is dropped when fully consumed.
func (e *Engine) fill(orderNo int64, volume int64) {
	o, ok := e.orders[orderNo]
	if !ok {
		e.stats.UnknownOrder++
		return
	}
	actual := volume
	if actual > o.Remaining {
		actual = o.Remaining
	}
	e.removeLevel(o.Stock, o.Side, o.Price, actual, o.Foreign)

	o.Remaining -= actual
	if o.Remaining <= 0 {
		delete(e.orders, orderNo)
		e.decOrderCount(o.Stock, o.Side, o.Price, o.Foreign)
	}
	e.markChanged(o.Stock)
}

func (e *Engine) bookFor(stock string) *Book {
	bk, ok := e.books[stock]
	if !ok {
		bk = newBook()
		e.books[stock] = bk
	}
	return bk
}

func (e *Engine) levelMap(stock string, side Side) map[float64]*LevelStats {
	bk := e.bookFor(stock)
	if side == SideBid {
		return bk.Bid
	}
	return bk.Ask
}

// addLevel creates/updates the price level when an order joins the book.
func (e *Engine) addLevel(stock string, side Side, price float64, lot int64, foreign bool) {
	m := e.levelMap(stock, side)
	lvl, ok := m[price]
	if !ok {
		lvl = &LevelStats{}
		m[price] = lvl
	}
	lvl.Lot += lot
	lvl.OrderCount++
	if foreign {
		lvl.LotForeign += lot
		lvl.OrderForeign++
	}
}

// removeLevel reduces the price level by `lot` (called on partial fill /
// cancel / full fill). Does NOT decrement OrderCount — call decOrderCount
// separately when the underlying order is fully removed.
func (e *Engine) removeLevel(stock string, side Side, price float64, lot int64, foreign bool) {
	if lot <= 0 {
		return
	}
	m := e.levelMap(stock, side)
	lvl, ok := m[price]
	if !ok {
		return
	}
	lvl.Lot -= lot
	if foreign {
		lvl.LotForeign -= lot
		if lvl.LotForeign < 0 {
			lvl.LotForeign = 0
		}
	}
	if lvl.Lot <= 0 {
		// All lot at this price gone — drop the level entirely.
		delete(m, price)
	}
}

// decOrderCount decrements the OrderCount/OrderForeign at a price level
// when an order is fully consumed (cancel or full fill).
func (e *Engine) decOrderCount(stock string, side Side, price float64, foreign bool) {
	m := e.levelMap(stock, side)
	lvl, ok := m[price]
	if !ok {
		return
	}
	if lvl.OrderCount > 0 {
		lvl.OrderCount--
	}
	if foreign && lvl.OrderForeign > 0 {
		lvl.OrderForeign--
	}
}

// markChanged flags a stock as dirty AND bumps its monotonic seq. Public
// methods (OnOrder, OnTrade fill path) call this exactly once per logical
// event so seq increments are predictable from the FE's perspective.
//
// Called under e.mu — no further locking required.
func (e *Engine) markChanged(stock string) {
	e.dirty[stock] = true
	e.seq[stock]++
}

// markDirty flags a stock as dirty without bumping seq — used inside
// multi-step operations (e.g. the duplicate-order cleanup in add()) where
// the seq bump must wait for the closing markChanged call so one logical
// event = one seq increment.
func (e *Engine) markDirty(stock string) {
	e.dirty[stock] = true
}
