package book

import (
	"testing"

	"tuai/internal/modules/stock/orderbook_consumer/parser"
)

// makeAdd builds an Order with cmd=AddBid (or AddOffer) for terser tests.
func makeAdd(orderNo int64, stock string, price float64, bal int64, foreign bool, side Side) parser.Order {
	cmd := parser.CmdAddBid
	if side == SideAsk {
		cmd = parser.CmdAddOffer
	}
	inv := "D"
	if foreign {
		inv = "F"
	}
	return parser.Order{
		Stock:       stock,
		Command:     cmd,
		OrderNumber: orderNo,
		Price:       price,
		Volume:      bal, // original — fine for fresh-add tests
		Balance:     bal,
		Investor:    inv,
		Broker:      "--",
	}
}

func makeCancel(orderNo int64, side Side) parser.Order {
	cmd := parser.CmdCancelBid
	if side == SideAsk {
		cmd = parser.CmdCancelOffer
	}
	return parser.Order{
		Command:     cmd,
		OrderNumber: orderNo,
	}
}

// drainOne expects exactly one dirty stock; returns its snapshot.
func drainOne(t *testing.T, e *Engine) DirtySnapshot {
	t.Helper()
	got := e.DrainDirty()
	if len(got) != 1 {
		t.Fatalf("expected 1 dirty stock, got %d", len(got))
	}
	return got[0]
}

// TestFreshAddCreatesLevel verifies a single new order populates Lot, OrderCount, and seq.
func TestFreshAddCreatesLevel(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 10000, false, SideBid))

	snap := drainOne(t, e)
	if snap.Stock != "BBCA" {
		t.Fatalf("stock: got %q want BBCA", snap.Stock)
	}
	if snap.Seq != 1 {
		t.Fatalf("seq: got %d want 1", snap.Seq)
	}
	lvl, ok := snap.Bid[5975]
	if !ok {
		t.Fatalf("bid level 5975 missing")
	}
	if lvl.Lot != 10000 || lvl.OrderCount != 1 {
		t.Fatalf("bid 5975: got lot=%d freq=%d, want 10000/1", lvl.Lot, lvl.OrderCount)
	}
	if len(snap.Ask) != 0 {
		t.Fatalf("ask should be empty, got %v", snap.Ask)
	}
}

// TestDuplicateOrderNoDoesNotInflateFreq is the regression test for B2 —
// the duplicate-order_no path used to inflate OrderCount because it never
// called decOrderCount before re-adding.
func TestDuplicateOrderNoDoesNotInflateFreq(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 10000, false, SideBid))
	// Same order_no re-emitted by vendor with updated Balance (same price).
	e.OnOrder(makeAdd(1, "BBCA", 5975, 6500, false, SideBid))

	snap := drainOne(t, e)
	lvl := snap.Bid[5975]
	if lvl.OrderCount != 1 {
		t.Fatalf("OrderCount: got %d want 1 (B2 regression)", lvl.OrderCount)
	}
	if lvl.Lot != 6500 {
		t.Fatalf("Lot: got %d want 6500 (should reflect latest Balance)", lvl.Lot)
	}
	if snap.Seq != 2 {
		t.Fatalf("seq: got %d want 2 (two mutations)", snap.Seq)
	}
}

// TestAddUsesBalanceNotVolume verifies the engine takes o.Balance for the
// level lot, not o.Volume (B1). Vendor may send Type 16 with Balance < Volume
// when the order is already partly filled.
func TestAddUsesBalanceNotVolume(t *testing.T) {
	e := New()
	o := makeAdd(1, "BBCA", 5975, 10000, false, SideBid)
	o.Volume = 10000  // original size
	o.Balance = 6500  // current remaining (vendor net-out)
	e.OnOrder(o)

	snap := drainOne(t, e)
	if snap.Bid[5975].Lot != 6500 {
		t.Fatalf("Lot: got %d want 6500 (must use Balance not Volume)", snap.Bid[5975].Lot)
	}
}

// TestCancelRemovesContribution covers cancel correctness: both lot and
// OrderCount must drop, and if it was the last order at the level, the
// level itself must disappear.
func TestCancelRemovesContribution(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 3000, false, SideBid))
	e.OnOrder(makeAdd(2, "BBCA", 5975, 7000, true, SideBid))

	// State: lot=10000, freq=2, lotF=7000, freqF=1
	snap := drainOne(t, e)
	lvl := snap.Bid[5975]
	if lvl.Lot != 10000 || lvl.OrderCount != 2 || lvl.LotForeign != 7000 || lvl.OrderForeign != 1 {
		t.Fatalf("pre-cancel: %+v", lvl)
	}

	e.OnOrder(makeCancel(2, SideBid))
	snap = drainOne(t, e)
	lvl = snap.Bid[5975]
	if lvl.Lot != 3000 || lvl.OrderCount != 1 {
		t.Fatalf("post-cancel lot/freq: %+v", lvl)
	}
	if lvl.LotForeign != 0 || lvl.OrderForeign != 0 {
		t.Fatalf("post-cancel foreign should be 0: %+v", lvl)
	}

	// Cancel the last order → level should vanish entirely.
	e.OnOrder(makeCancel(1, SideBid))
	snap = drainOne(t, e)
	if _, ok := snap.Bid[5975]; ok {
		t.Fatalf("level should have been removed after final cancel")
	}
}

// TestFillReducesByVolume covers Type 15 trade application.
func TestFillReducesByVolume(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 10000, false, SideBid))
	e.OnOrder(makeAdd(2, "BBCA", 5975, 10000, false, SideAsk))
	_ = e.DrainDirty()

	// Partial fill 3000 shares between buyer #1 and seller #2.
	e.OnTrade("BBCA", 1, 2, 3000)
	snap := drainOne(t, e)
	if snap.Bid[5975].Lot != 7000 || snap.Bid[5975].OrderCount != 1 {
		t.Fatalf("bid post-partial: %+v", snap.Bid[5975])
	}
	if snap.Ask[5975].Lot != 7000 || snap.Ask[5975].OrderCount != 1 {
		t.Fatalf("ask post-partial: %+v", snap.Ask[5975])
	}

	// Full remaining fill: 7000 — both orders fully consumed → levels gone.
	e.OnTrade("BBCA", 1, 2, 7000)
	snap = drainOne(t, e)
	if _, ok := snap.Bid[5975]; ok {
		t.Fatalf("bid level should vanish after full fill")
	}
	if _, ok := snap.Ask[5975]; ok {
		t.Fatalf("ask level should vanish after full fill")
	}
}

// TestSeqMonotonicPerStock verifies seq advances only when that stock's
// state changes.
func TestSeqMonotonicPerStock(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 1000, false, SideBid))
	e.OnOrder(makeAdd(2, "BMRI", 5000, 1000, false, SideBid))
	e.OnOrder(makeAdd(3, "BBCA", 5980, 2000, false, SideBid))

	got := e.DrainDirty()
	seqByStock := map[string]uint64{}
	for _, s := range got {
		seqByStock[s.Stock] = s.Seq
	}
	if seqByStock["BBCA"] != 2 {
		t.Fatalf("BBCA seq: got %d want 2", seqByStock["BBCA"])
	}
	if seqByStock["BMRI"] != 1 {
		t.Fatalf("BMRI seq: got %d want 1", seqByStock["BMRI"])
	}
}

// TestResetClearsAll exercises Reset() including the per-stock seq counter.
func TestResetClearsAll(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 1000, false, SideBid))
	_ = e.DrainDirty()

	e.Reset()
	if got := e.DrainDirty(); len(got) != 0 {
		t.Fatalf("after reset, dirty should be empty, got %d", len(got))
	}

	// Same order_no after reset must be treated as fresh (seq starts back at 1).
	e.OnOrder(makeAdd(1, "BBCA", 5975, 2000, false, SideBid))
	snap := drainOne(t, e)
	if snap.Seq != 1 {
		t.Fatalf("post-reset seq: got %d want 1", snap.Seq)
	}
	if snap.Bid[5975].Lot != 2000 {
		t.Fatalf("post-reset state stale: %+v", snap.Bid[5975])
	}
}

// TestForeignTrackingFollowsInvestor ensures lf/ff get filled only when
// the order's investor is F (and reset to 0 when foreign orders leave).
func TestForeignTrackingFollowsInvestor(t *testing.T) {
	e := New()
	e.OnOrder(makeAdd(1, "BBCA", 5975, 3000, false, SideBid)) // domestic
	e.OnOrder(makeAdd(2, "BBCA", 5975, 5000, true, SideBid))  // foreign

	snap := drainOne(t, e)
	lvl := snap.Bid[5975]
	if lvl.Lot != 8000 || lvl.LotForeign != 5000 {
		t.Fatalf("foreign tracking: %+v", lvl)
	}
	if lvl.OrderCount != 2 || lvl.OrderForeign != 1 {
		t.Fatalf("foreign freq: %+v", lvl)
	}

	// Cancel the foreign order — foreign counters back to 0.
	e.OnOrder(makeCancel(2, SideBid))
	snap = drainOne(t, e)
	lvl = snap.Bid[5975]
	if lvl.LotForeign != 0 || lvl.OrderForeign != 0 {
		t.Fatalf("foreign should clear: %+v", lvl)
	}
}
