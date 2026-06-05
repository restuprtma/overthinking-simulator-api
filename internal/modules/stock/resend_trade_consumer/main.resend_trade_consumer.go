// Package resend_trade_consumer wires the JetStream subscriber for
// idx.resend.trade.> to a single QuestDB table (`trades`).
//
// Per docs/infra/topology.md §5.3 + §6.1 (Type 27 — Resend Trade):
//
//   During market hours, IDX masks broker codes in Type 15 (Trade) as "--".
//   The vendor re-emits each matched trade as Type 27 (Resend Trade) with
//   the real broker codes (e.g. "PD", "YP") in TWO batches per session:
//     - mid-day, ~12:00 WIB (lunch break)
//     - post-close, ~17:00 WIB (occasionally delivered late, past midnight)
//
//   Both batches are written to the same `trades` table; DEDUP UPSERT
//   KEYS(timestamp, stock, trade_no) on the table prevents duplicates if
//   a row is re-processed.
//
// History: an earlier version routed rows into `mid_trades` vs `trades`
// based on the consumer&apos;s wall-clock receipt hour in WIB. That proxy broke
// whenever the vendor delivered late (e.g. ~00:02 WIB) or JetStream
// replayed messages — post-close rows ended up in `mid_trades`.
//
// The live `running_trades` table written by running-trade-consumer is left alone
// (broker codes there stay masked as "--"); `trades` is the broker-real
// archive queried separately. A previous design tried to overwrite the
// live row via DEDUP UPSERT cross-table, but that path was too slow at
// IDX-scale.
//
// Deps shared with running-trade-consumer:
//   - parser: internal/modules/stock/running_trade_consumer/trade
//   - sink:   internal/modules/stock/running_trade_consumer/sink (QuestDBSink)
//
// Type 26 (Resend Order) is NOT handled here.
package resend_trade_consumer

import (
	"context"
	"fmt"

	tradesink "tuai/internal/modules/stock/running_trade_consumer/sink"
	"tuai/internal/modules/stock/resend_trade_consumer/service"
	"tuai/internal/modules/stock/resend_trade_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber *subscriber.Subscriber
	TickWriter tradesink.TickWriter
	Service    *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber subscriber.Config
	QuestDB    tradesink.QuestDBConfig
	Service    service.Config
}

// Initialize constructs the QuestDB sink and wires the service.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	sink, err := tradesink.NewQuestDB(ctx, cfg.QuestDB)
	if err != nil {
		return nil, fmt.Errorf("init questdb sink: %w", err)
	}

	svc := service.New(sink, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber: sub,
		TickWriter: sink,
		Service:    svc,
	}, nil
}

// Run blocks until ctx is cancelled.
func (m *Module) Run(ctx context.Context) error {
	return m.Service.Run(ctx)
}
