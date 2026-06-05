package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_publisher/client"
	"tuai/pkg/logger"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Config holds NATS / JetStream connection parameters.
//
// Stream selection is per-subject via subjects.StreamFor() — there is no
// single "the stream" because topology.md splits IDX data across four
// streams (IDX_TICK, IDX_QUOTE, IDX_META, IDX_NEWS).
type Config struct {
	URL             string
	Token           string
	ClientName      string        // identifies this publisher in nats monitoring
	PublishTimeout  time.Duration // ack-wait timeout per message
	AsyncMaxPending int           // jetstream async in-flight cap PER STREAM
	ConnectTimeout  time.Duration
	ReconnectWait   time.Duration
	MaxReconnects   int // -1 = forever
	SchemaVersion   string
	DrainOnShutdown time.Duration
	EnforceStream   bool // if true: WithExpectStream per derived stream

	// LOAD-TEST ISOLATION (default empty = production behavior)
	//
	// SubjectPrefix is prepended to every derived subject. e.g. with
	// "test." the live subject "idx.resend.trade.IMPC" becomes
	// "test.idx.resend.trade.IMPC". Combine with RouteToStream so the
	// prefixed traffic lands in a dedicated test stream that is filtered
	// out of downstream consumers.
	SubjectPrefix string

	// RouteToStream forces every record to a single named stream regardless
	// of the subject-prefix mapping in subjects.go. When set, AllStreams()
	// is bypassed and only this one stream's JetStream context is opened.
	// Pair with SubjectPrefix + a pre-created test stream (e.g.
	// IDX_TICK_TEST with subject filter "test.idx.>") for isolated load tests.
	RouteToStream string
}

// DefaultConfig returns a Config with publisher.md-aligned defaults.
func DefaultConfig() Config {
	return Config{
		ClientName:      "iqplus-publisher",
		PublishTimeout:  10 * time.Second,
		AsyncMaxPending: 32_000,
		ConnectTimeout:  5 * time.Second,
		ReconnectWait:   2 * time.Second,
		MaxReconnects:   -1,
		SchemaVersion:   "v1",
		DrainOnShutdown: 30 * time.Second,
		EnforceStream:   true,
	}
}

// ErrPublishBackpressure is returned by Publish when the JetStream async
// queue is saturated or the NATS connection is mid-reconnect. The caller
// MUST wait and retry — silent drop is loss. errors.Is(err, ErrPublishBackpressure)
// is the correct way to test for it.
var ErrPublishBackpressure = errors.New("jetstream publish backpressure")

// ErrPublishPermanent is returned for non-retriable failures (oversize
// payload, marshal error, unknown stream). The caller should drop the record
// and log — retrying will not help.
var ErrPublishPermanent = errors.New("jetstream publish permanent failure")

// Stats are best-effort publish counters surfaced for ops/logging.
//
// Error counters are split so we can tell apart soft failures (ack didn't
// arrive in time but JetStream may still have persisted the message — dedup
// via Nats-Msg-Id protects retries) from hard ones (message never queued).
type Stats struct {
	PublishedOK      uint64
	Duplicates       uint64
	MarshalErrors    uint64 // json.Marshal failed; record never sent
	AsyncQueueErrors uint64 // PublishMsgAsync immediate failure — caller is expected to retry on ErrPublishBackpressure
	AckErrors        uint64 // ack future returned error (stream mismatch, dedup conflict, async err handler)
	AckTimeouts      uint64 // ack future didn't fire within PublishTimeout — usually still persisted
	AckUntracked     uint64 // ack-tracker semaphore saturated; ack outcome only visible via async err handler
	Dropped          uint64 // subject derivation returned ""
	Unrouted         uint64 // no stream configured for the subject
}

// TotalErrors returns the sum of all error categories.
func (s Stats) TotalErrors() uint64 {
	return s.MarshalErrors + s.AsyncQueueErrors + s.AckErrors + s.AckTimeouts
}

// Publisher owns one NATS connection and one JetStream context per logical
// stream (IDX_TICK / IDX_QUOTE / IDX_META / IDX_NEWS). Each stream gets its
// own AsyncMaxPending budget so a stalled-disk on IDX_QUOTE cannot block
// IDX_TICK trades — that decoupling was the root cause of the morning-burst
// loss observed on 2026-04-27.
type Publisher struct {
	cfg     Config
	nc      *nats.Conn
	streams map[string]jetstream.JetStream
	stats   Stats

	// ackSem bounds the number of concurrent ack-tracking goroutines.
	// Without this, a resend burst spawns one goroutine per record (millions),
	// driving the scheduler into starvation and bloating RSS by GBs.
	// Capacity = 4× per-stream AsyncMaxPending: covers all in-flight publishes
	// across the four streams with headroom. When saturated, Publish proceeds
	// without launching a tracker — the message is still in flight; ack errors
	// will land in the WithPublishAsyncErrHandler set in Connect.
	ackSem chan struct{}
}

func New(cfg Config) *Publisher {
	if cfg.PublishTimeout == 0 {
		cfg.PublishTimeout = 10 * time.Second
	}
	if cfg.AsyncMaxPending <= 0 {
		cfg.AsyncMaxPending = 32_000
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 2 * time.Second
	}
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = -1
	}
	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = "v1"
	}
	if cfg.DrainOnShutdown == 0 {
		cfg.DrainOnShutdown = 30 * time.Second
	}
	if cfg.ClientName == "" {
		cfg.ClientName = "iqplus-publisher"
	}
	// 4 streams × AsyncMaxPending in-flight max. Doubled for headroom.
	semCap := 8 * cfg.AsyncMaxPending
	return &Publisher{
		cfg:    cfg,
		ackSem: make(chan struct{}, semCap),
	}
}

// Connect dials NATS and constructs one JetStream context per stream so
// each stream owns an independent async-pending pool.
func (p *Publisher) Connect() error {
	opts := []nats.Option{
		nats.Name(p.cfg.ClientName),
		nats.MaxReconnects(p.cfg.MaxReconnects),
		nats.ReconnectWait(p.cfg.ReconnectWait),
		nats.ReconnectJitter(500*time.Millisecond, 2*time.Second),
		nats.Timeout(p.cfg.ConnectTimeout),
		nats.PingInterval(20 * time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Log.Warn("nats disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Log.Info("nats reconnected", zap.String("url", c.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Log.Info("nats connection closed")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			logger.Log.Warn("nats async error", zap.Error(err))
		}),
	}
	if p.cfg.Token != "" {
		opts = append(opts, nats.Token(p.cfg.Token))
	}

	nc, err := nats.Connect(p.cfg.URL, opts...)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}

	streams := AllStreams()
	if p.cfg.RouteToStream != "" {
		streams = []string{p.cfg.RouteToStream}
		logger.Log.Info("publisher running in single-stream test mode",
			zap.String("route_to_stream", p.cfg.RouteToStream),
			zap.String("subject_prefix", p.cfg.SubjectPrefix))
	}
	contexts := make(map[string]jetstream.JetStream, len(streams))
	for _, name := range streams {
		js, jerr := jetstream.New(nc,
			jetstream.WithPublishAsyncMaxPending(p.cfg.AsyncMaxPending),
			jetstream.WithPublishAsyncErrHandler(func(_ jetstream.JetStream, msg *nats.Msg, jsErr error) {
				atomic.AddUint64(&p.stats.AckErrors, 1)
				logger.Log.Warn("jetstream async ack error",
					zap.String("subject", msg.Subject),
					zap.Error(jsErr))
			}),
		)
		if jerr != nil {
			nc.Close()
			return fmt.Errorf("jetstream new for %s: %w", name, jerr)
		}
		contexts[name] = js
	}

	p.nc = nc
	p.streams = contexts
	logger.Log.Info("nats jetstream ready",
		zap.String("url", nc.ConnectedUrl()),
		zap.String("client", p.cfg.ClientName),
		zap.Strings("streams", streams),
		zap.Int("async_max_pending_per_stream", p.cfg.AsyncMaxPending),
		zap.Bool("enforce_stream", p.cfg.EnforceStream))
	return nil
}

// recordEnvelope is the JSON payload published per IQPlus record.
//
// Keep field order/casing stable — consumers depend on this contract.
type recordEnvelope struct {
	Date          string    `json:"date"`        // YYYYMMDD (server)
	Time          string    `json:"time"`        // HHMMSS (server, WIB)
	Sequence      int64     `json:"sequence"`    // unique per day
	RecordType    int       `json:"record_type"` // 13/14/15/...
	Data          string    `json:"data"`        // raw IQPlus payload
	ReceivedAt    time.Time `json:"received_at"` // publisher receive time (UTC)
	SchemaVersion string    `json:"schema_version"`
	PublisherName string    `json:"publisher"`
}

// Publish marshals one record and asynchronously sends it to JetStream.
//
// Returns nil for queued records (ack outcomes are observed via the
// async err handler set in Connect, plus best-effort trackAck if the
// semaphore has room).
//
// Returns wrapped ErrPublishBackpressure when the caller should retry the
// SAME record after a short wait (queue full / conn mid-reconnect). Returns
// wrapped ErrPublishPermanent for failures that retrying cannot fix
// (oversize, marshal error, unknown stream).
func (p *Publisher) Publish(_ context.Context, r client.Record) error {
	subject := SubjectFor(r)
	if subject == "" {
		atomic.AddUint64(&p.stats.Dropped, 1)
		return nil
	}

	if p.cfg.SubjectPrefix != "" {
		subject = p.cfg.SubjectPrefix + subject
	}

	var stream string
	if p.cfg.RouteToStream != "" {
		stream = p.cfg.RouteToStream
	} else {
		stream = StreamFor(subject)
	}
	if stream == "" {
		atomic.AddUint64(&p.stats.Unrouted, 1)
		logger.Log.Warn("no stream configured for subject",
			zap.String("subject", subject),
			zap.Int("record_type", r.RecordType))
		return fmt.Errorf("%w: %v", ErrPublishPermanent, errUnknownStream)
	}

	js, ok := p.streams[stream]
	if !ok {
		atomic.AddUint64(&p.stats.Unrouted, 1)
		logger.Log.Warn("no jetstream context for stream",
			zap.String("stream", stream), zap.String("subject", subject))
		return fmt.Errorf("%w: %v", ErrPublishPermanent, errUnknownStream)
	}

	body, err := json.Marshal(recordEnvelope{
		Date:          r.Date,
		Time:          r.Time,
		Sequence:      r.Sequence,
		RecordType:    r.RecordType,
		Data:          r.RawData,
		ReceivedAt:    r.ReceivedAt.UTC(),
		SchemaVersion: p.cfg.SchemaVersion,
		PublisherName: p.cfg.ClientName,
	})
	if err != nil {
		atomic.AddUint64(&p.stats.MarshalErrors, 1)
		return fmt.Errorf("%w: marshal envelope: %v", ErrPublishPermanent, err)
	}

	msgID := fmt.Sprintf("iqplus-%s-%d", r.Date, r.Sequence)

	msg := &nats.Msg{
		Subject: subject,
		Data:    body,
		Header:  nats.Header{},
	}
	msg.Header.Set("Nats-Msg-Id", msgID)
	msg.Header.Set("X-Schema-Version", p.cfg.SchemaVersion)
	msg.Header.Set("X-Source", p.cfg.ClientName)
	msg.Header.Set("X-Record-Type", fmt.Sprintf("%d", r.RecordType))
	msg.Header.Set("X-Stream", stream)

	opts := []jetstream.PublishOpt{}
	if p.cfg.EnforceStream {
		opts = append(opts, jetstream.WithExpectStream(stream))
	}

	ackF, err := js.PublishMsgAsync(msg, opts...)
	if err != nil {
		atomic.AddUint64(&p.stats.AsyncQueueErrors, 1)
		switch {
		case errors.Is(err, nats.ErrMaxPayload):
			// Permanent: payload structurally too large.
			logger.Log.Warn("jetstream payload too large; record dropped",
				zap.String("subject", subject), zap.Int("size", len(body)))
			return fmt.Errorf("%w: payload too large: %v", ErrPublishPermanent, err)
		case errors.Is(err, nats.ErrConnectionClosed):
			// Transient: NATS is reconnecting. Caller should retry.
			logger.Log.Debug("jetstream connection closed; will retry",
				zap.String("subject", subject), zap.String("msg_id", msgID))
			return fmt.Errorf("%w: connection closed: %v", ErrPublishBackpressure, err)
		default:
			// Most likely "max async pending" — the queue is full. Caller
			// retries; that creates real backpressure to the TCP reader.
			logger.Log.Debug("jetstream async queue full; will retry",
				zap.String("stream", stream),
				zap.String("subject", subject),
				zap.Int("async_max_pending", p.cfg.AsyncMaxPending),
				zap.Error(err))
			return fmt.Errorf("%w: %v", ErrPublishBackpressure, err)
		}
	}

	// Track ack outcome — but only if we have semaphore room. When saturated
	// (e.g., during a 5M-record resend burst), skip tracking; the JetStream
	// async err handler in Connect still surfaces failures into AckErrors.
	select {
	case p.ackSem <- struct{}{}:
		go func() {
			defer func() { <-p.ackSem }()
			p.trackAck(ackF, subject)
		}()
	default:
		atomic.AddUint64(&p.stats.AckUntracked, 1)
	}

	return nil
}

func (p *Publisher) trackAck(ackF jetstream.PubAckFuture, subject string) {
	timer := time.NewTimer(p.cfg.PublishTimeout)
	defer timer.Stop()
	select {
	case ack := <-ackF.Ok():
		if ack.Duplicate {
			atomic.AddUint64(&p.stats.Duplicates, 1)
		} else {
			atomic.AddUint64(&p.stats.PublishedOK, 1)
		}
	case err := <-ackF.Err():
		atomic.AddUint64(&p.stats.AckErrors, 1)
		// stream-mismatch / dedup / etc — info enough at debug.
		if !strings.Contains(err.Error(), "context") {
			logger.Log.Debug("jetstream ack error",
				zap.String("subject", subject), zap.Error(err))
		}
	case <-timer.C:
		atomic.AddUint64(&p.stats.AckTimeouts, 1)
		logger.Log.Debug("jetstream ack timeout",
			zap.String("subject", subject),
			zap.Duration("after", p.cfg.PublishTimeout))
	}
}

// Snapshot returns a copy of the current publish counters.
func (p *Publisher) Snapshot() Stats {
	return Stats{
		PublishedOK:      atomic.LoadUint64(&p.stats.PublishedOK),
		Duplicates:       atomic.LoadUint64(&p.stats.Duplicates),
		MarshalErrors:    atomic.LoadUint64(&p.stats.MarshalErrors),
		AsyncQueueErrors: atomic.LoadUint64(&p.stats.AsyncQueueErrors),
		AckErrors:        atomic.LoadUint64(&p.stats.AckErrors),
		AckTimeouts:      atomic.LoadUint64(&p.stats.AckTimeouts),
		AckUntracked:     atomic.LoadUint64(&p.stats.AckUntracked),
		Dropped:          atomic.LoadUint64(&p.stats.Dropped),
		Unrouted:         atomic.LoadUint64(&p.stats.Unrouted),
	}
}

// Close waits for pending async acks across all streams (bounded by
// DrainOnShutdown) and then drains the NATS connection. Safe to call multiple
// times.
func (p *Publisher) Close() {
	if len(p.streams) > 0 {
		deadline := time.After(p.cfg.DrainOnShutdown)
		for name, js := range p.streams {
			select {
			case <-js.PublishAsyncComplete():
			case <-deadline:
				logger.Log.Warn("jetstream async drain timed out",
					zap.String("stream", name),
					zap.Duration("after", p.cfg.DrainOnShutdown))
			}
		}
	}
	if p.nc != nil {
		if err := p.nc.Drain(); err != nil {
			logger.Log.Warn("nats drain error", zap.Error(err))
		}
	}
}
