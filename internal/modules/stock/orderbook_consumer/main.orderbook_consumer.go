// Package orderbook_consumer reconstructs the IDX order book in real time
// from per-event sources (Type 16 Order + Type 15 Trade), persists snapshots
// to Redis db 9, and publishes per-stock deltas to NATS for the ws-gateway
// to fan out to browser clients.
//
// Why event-sourced (not Type 18 snapshot):
//
//   IQPlus Type 18 (Best Quote) is vendor-aggregated periodic — lags actual
//   order activity by seconds and may not include the true best bid/ask
//   for less-active stocks. For sub-second depth display we reconstruct
//   from the event stream:
//
//     - Type 16: every order add (with current Balance) and cancel
//     - Type 15: every match — both sides decremented by trade volume
//     - Type 57: trading-session status — drives engine reset + phase tag
//
// State machine:    internal/.../book/engine.go
// Background flush: internal/.../snapshotter/snapshotter.go
// Wire protocol:    docs/orderbook/protocol.md
//
// Output:
//
//   Redis db 9:
//     HASH orderbook:<stock>:bid    field=<price> value=JSON {l,f,lf,ff}
//     HASH orderbook:<stock>:ask    field=<price> value=JSON {l,f,lf,ff}
//     HASH orderbook:<stock>:_meta  { seq, top_bid, top_ask, totals, phase }
//
//   NATS:
//     idx.orderbook.delta.<stock>   per snapshotter cycle when book changed
//     idx.orderbook.reset.all       on engine reset (session begin)
//
// Limitations (current):
//   - First few minutes after cold-start may show "unknown order" warnings
//     as Type 15 fills reference orders from before the consumer attached.
//     Self-corrects as new Type 16 events arrive.
//   - No Type 26 (Resend Order) handling — broker-code backfill not needed
//     for live depth display.
package orderbook_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/orderbook_consumer/book"
	"tuai/internal/modules/stock/orderbook_consumer/service"
	"tuai/internal/modules/stock/orderbook_consumer/sink"
	"tuai/internal/modules/stock/orderbook_consumer/snapshotter"
	"tuai/internal/modules/stock/orderbook_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber  *subscriber.Subscriber
	RedisSink   *sink.RedisSink
	NATSSink    *sink.NATSSink
	Engine      *book.Engine
	Snapshotter *snapshotter.Snapshotter
	Service     *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber  subscriber.Config
	Redis       sink.Config
	NATS        sink.NATSConfig
	Snapshotter snapshotter.Config
	Service     service.Config
}

// Initialize constructs all dependencies and pings Redis + NATS up-front.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	redisSink, err := sink.New(ctx, cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("init redis sink: %w", err)
	}

	natsSink, err := sink.NewNATS(cfg.NATS)
	if err != nil {
		_ = redisSink.Shutdown()
		return nil, fmt.Errorf("init nats sink: %w", err)
	}

	engine := book.New()
	snap := snapshotter.New(engine, redisSink, natsSink, cfg.Snapshotter)

	svc := service.New(engine, snap, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber:  sub,
		RedisSink:   redisSink,
		NATSSink:    natsSink,
		Engine:      engine,
		Snapshotter: snap,
		Service:     svc,
	}, nil
}

// Run blocks until ctx is cancelled. On shutdown, the snapshotter
// performs a final drain so any pending dirty state lands in Redis + NATS.
func (m *Module) Run(ctx context.Context) error {
	defer func() {
		if m.NATSSink != nil {
			m.NATSSink.Shutdown()
		}
		if m.RedisSink != nil {
			_ = m.RedisSink.Shutdown()
		}
	}()
	return m.Service.Run(ctx)
}
