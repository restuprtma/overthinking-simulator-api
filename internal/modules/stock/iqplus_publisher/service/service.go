package service

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_publisher/client"
	"tuai/internal/modules/stock/iqplus_publisher/publisher"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// backpressureRetryDelay is how long the service waits between retries when
// the publisher signals ErrPublishBackpressure. Short enough to recover
// quickly when async slots free up; long enough to avoid a tight CPU spin.
const backpressureRetryDelay = 50 * time.Millisecond

// Config holds runtime knobs for the pipeline.
type Config struct {
	StatsTick    time.Duration
	ExcludeTypes map[int]struct{} // record types to drop before publish
}

// Service is the long-lived pipeline that pulls records from the IQPlus
// client and forwards each one to JetStream.
type Service struct {
	client    *client.Client
	publisher *publisher.Publisher
	cfg       Config

	filtered      uint64
	backpressured uint64 // total backpressure retries observed
	dropped       uint64 // records dropped due to permanent publish errors
}

func New(c *client.Client, p *publisher.Publisher, cfg Config) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	return &Service{
		client:    c,
		publisher: p,
		cfg:       cfg,
	}
}

// Run blocks until ctx is cancelled or the upstream channel closes.
func (s *Service) Run(ctx context.Context) error {
	if err := s.publisher.Connect(); err != nil {
		return err
	}
	defer s.publisher.Close()

	if len(s.cfg.ExcludeTypes) > 0 {
		excluded := make([]int, 0, len(s.cfg.ExcludeTypes))
		for t := range s.cfg.ExcludeTypes {
			excluded = append(excluded, t)
		}
		logger.Log.Info("iqplus type filter active",
			zap.Ints("excluded", excluded))
	}

	records := s.client.Stream(ctx)
	ticker := time.NewTicker(s.cfg.StatsTick)
	defer ticker.Stop()

	var consumed uint64
	start := time.Now()

	for {
		select {
		case <-ctx.Done():
			s.logStats(consumed, time.Since(start))
			return ctx.Err()

		case <-ticker.C:
			s.logStats(consumed, time.Since(start))

		case rec, ok := <-records:
			if !ok {
				s.logStats(consumed, time.Since(start))
				logger.Log.Info("iqplus stream closed, exiting service")
				return nil
			}
			atomic.AddUint64(&consumed, 1)

			if _, skip := s.cfg.ExcludeTypes[rec.RecordType]; skip {
				atomic.AddUint64(&s.filtered, 1)
				continue
			}

			if err := s.publishWithRetry(ctx, rec); err != nil {
				// Only ctx.Err() bubbles up — service-level shutdown.
				s.logStats(consumed, time.Since(start))
				return err
			}
		}
	}
}

// publishWithRetry calls Publish and loops on ErrPublishBackpressure until
// the record is queued or ctx is cancelled. Permanent errors are logged and
// the record is dropped (counted in s.dropped) — retrying cannot fix them.
//
// Retrying on backpressure is what creates the end-to-end backpressure path:
// blocking here means the service goroutine doesn't drain the client.Stream()
// channel, which fills, which blocks the TCP read loop, which finally pushes
// back on IQPlus. Without this loop the publisher silently drops records on
// queue-full — that was the morning-burst loss observed before this fix.
func (s *Service) publishWithRetry(ctx context.Context, rec client.Record) error {
	for {
		err := s.publisher.Publish(ctx, rec)
		if err == nil {
			return nil
		}
		if errors.Is(err, publisher.ErrPublishPermanent) {
			atomic.AddUint64(&s.dropped, 1)
			logger.Log.Warn("permanent publish error; dropping record",
				zap.Int("type", rec.RecordType),
				zap.Int64("seq", rec.Sequence),
				zap.Error(err))
			return nil
		}
		if !errors.Is(err, publisher.ErrPublishBackpressure) {
			// Unexpected error class — log loudly and drop to avoid an
			// infinite tight loop on something we don't understand.
			atomic.AddUint64(&s.dropped, 1)
			logger.Log.Error("unknown publish error; dropping record",
				zap.Int("type", rec.RecordType),
				zap.Int64("seq", rec.Sequence),
				zap.Error(err))
			return nil
		}
		atomic.AddUint64(&s.backpressured, 1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backpressureRetryDelay):
		}
	}
}

func (s *Service) logStats(consumed uint64, elapsed time.Duration) {
	st := s.publisher.Snapshot()
	cs := s.client.Snapshot()
	rate := float64(consumed) / elapsed.Seconds()
	logger.Log.Info("iqplus publisher stats",
		zap.Uint64("consumed", consumed),
		zap.Float64("rate_per_sec", rate),
		zap.Uint64("filtered", atomic.LoadUint64(&s.filtered)),
		zap.Uint64("ok", st.PublishedOK),
		zap.Uint64("dup", st.Duplicates),
		zap.Uint64("err", st.TotalErrors()),
		zap.Uint64("err_marshal", st.MarshalErrors),
		zap.Uint64("err_queue", st.AsyncQueueErrors),
		zap.Uint64("err_ack", st.AckErrors),
		zap.Uint64("err_ack_timeout", st.AckTimeouts),
		zap.Uint64("ack_untracked", st.AckUntracked),
		zap.Uint64("backpressured", atomic.LoadUint64(&s.backpressured)),
		zap.Uint64("svc_dropped", atomic.LoadUint64(&s.dropped)),
		zap.Uint64("dropped", st.Dropped),
		zap.Uint64("unrouted", st.Unrouted),
		zap.Uint64("tcp_received", cs.Received),
		zap.Uint64("tcp_dropped", cs.Dropped),
		zap.Uint64("tcp_parse_err", cs.ParseErrors),
		zap.Uint64("tcp_reconnects", cs.Reconnects),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
