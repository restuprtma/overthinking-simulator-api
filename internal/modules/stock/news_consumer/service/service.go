package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/news_consumer/assembler"
	"tuai/internal/modules/stock/news_consumer/parser"
	"tuai/internal/modules/stock/news_consumer/sink"
	"tuai/internal/modules/stock/news_consumer/subscriber"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick time.Duration
}

// Service glues subscriber → parser → assembler → MongoDB sink.
//
// Per-message flow:
//
//	envelope → parser.Parse → assembler.Add
//	    └─ if news complete → sink.Insert → ack
//	    └─ if more packets pending → ack (we keep the buffer in memory)
type Service struct {
	sink       *sink.MongoSink
	asm        *assembler.Assembler
	cfg        Config
	subscriber *subscriber.Subscriber

	parseErrors uint64
	dropped     uint64 // non-news record types received unexpectedly
	inserts     uint64
	insertErr   uint64
}

func New(s *sink.MongoSink, asm *assembler.Assembler, cfg Config) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	return &Service{sink: s, asm: asm, cfg: cfg}
}

// Handler returns the subscriber callback.
func (s *Service) Handler() subscriber.Handler {
	return s.onEnvelope
}

// AttachSubscriber wires the subscriber back-reference.
func (s *Service) AttachSubscriber(sub *subscriber.Subscriber) {
	s.subscriber = sub
}

// Run starts the subscriber and blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	if s.subscriber == nil {
		panic("service.Run: subscriber not attached")
	}
	if err := s.subscriber.Start(ctx); err != nil {
		return err
	}
	s.asm.StartSweeper()
	defer s.shutdown(ctx)

	ticker := time.NewTicker(s.cfg.StatsTick)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			s.logStats(time.Since(start))
			return ctx.Err()
		case <-ticker.C:
			s.logStats(time.Since(start))
		}
	}
}

func (s *Service) onEnvelope(ctx context.Context, env iqplus_envelope.Envelope) error {
	if env.RecordType != 36 {
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}

	pkt, err := parser.Parse(env.Data)
	if err != nil {
		atomic.AddUint64(&s.parseErrors, 1)
		logger.Log.Warn("news parse error",
			zap.Int64("seq", env.Sequence),
			zap.String("data", truncate(env.Data, 200)),
			zap.Error(err))
		// Bad payload — ack to avoid infinite retry of garbage.
		return nil
	}

	news := s.asm.Add(pkt)
	if news == nil {
		// Still waiting for more packets — ack the current frame.
		return nil
	}

	if err := s.sink.Insert(ctx, *news); err != nil {
		atomic.AddUint64(&s.insertErr, 1)
		logger.Log.Warn("mongo insert error",
			zap.String("news_id", news.NewsID), zap.Error(err))
		// NAK so JetStream redelivers — but the assembler already emitted
		// this News and dropped its buffer. On redelivery, the same packet
		// arrives again, the assembler treats it as a fresh single-packet
		// (or partial) — risk of incomplete reinsert. For MVP, accept this
		// edge case; alternative is to keep buffers around longer.
		return err
	}
	atomic.AddUint64(&s.inserts, 1)
	logger.Log.Info("news inserted",
		zap.String("news_id", news.NewsID),
		zap.String("ticker", news.CompanyID),
		zap.String("headline", truncate(news.Headline, 80)))
	return nil
}

func (s *Service) shutdown(ctx context.Context) {
	logger.Log.Info("news consumer shutting down")
	s.subscriber.Close()
	s.asm.Stop()
	if err := s.sink.Shutdown(ctx); err != nil {
		logger.Log.Warn("mongo shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	asmStats := s.asm.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("news consumer stats",
		zap.Uint64("packets_received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.parseErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Uint64("news_inserted", atomic.LoadUint64(&s.inserts)),
		zap.Uint64("insert_err", atomic.LoadUint64(&s.insertErr)),
		zap.Int("buffers_open", asmStats.BuffersOpen),
		zap.Uint64("buffers_evicted", asmStats.BuffersEvicted),
		zap.Uint64("dup_packets", asmStats.DuplicatePackets),
		zap.Float64("packets_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
