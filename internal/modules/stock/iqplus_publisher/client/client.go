package client

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds IQPlus TCP feed connection parameters.
type Config struct {
	Host             string
	Port             int
	User             string
	PasswordMD5      string
	DialTimeout      time.Duration
	LoginTimeout     time.Duration
	ReadTimeout      time.Duration
	ReconnectDelay   time.Duration
	BufferSize       int           // outbound channel buffer (records)
	SendTimeout      time.Duration // max time to wait pushing onto a full buffer before counting a drop (0 = block forever, only ctx escapes)
	SocketRecvBuffer int           // SO_RCVBUF in bytes (0 = leave OS default + auto-tune); capped by kern.ipc.maxsockbuf
	BufioReaderSize  int           // bufio reader internal buffer (bytes); 0 = use default 1 MiB
}

// DefaultConfig returns a Config with sane production defaults.
//
// BufferSize is sized for the IDX EOD resend burst — IQPlus can flush 5–10
// million records in a few minutes during the 16:30–18:00 WIB window. The
// earlier 200_000 default left only ~3 seconds of in-memory slack at peak
// rate, so the read loop blocked on channel-send while the kernel TCP
// reassembly queue overflowed (visible as `discarded due to full reassembly
// queue` in `netstat -s -p tcp`) and bytes were lost upstream of the
// publisher's `tcp_received` counter.
//
// SocketRecvBuffer requests SO_RCVBUF on the TCP socket. Default 16 MiB
// aligns with `net.inet.tcp.recvbuf_max=16777216` we set in /etc/sysctl.conf
// on edge VM. The kernel silently caps to `kern.ipc.maxsockbuf`; setsockopt
// will not error, just clamp.
func DefaultConfig() Config {
	return Config{
		DialTimeout:      10 * time.Second,
		LoginTimeout:     10 * time.Second,
		ReadTimeout:      60 * time.Second,
		ReconnectDelay:   5 * time.Second,
		BufferSize:       2_000_000,
		SendTimeout:      0,
		SocketRecvBuffer: 16 * 1024 * 1024,
		BufioReaderSize:  1024 * 1024,
	}
}

// Stats are best-effort TCP-side counters. Surfaced via Snapshot() so the
// service-level stats log can show whether the loss is happening BEFORE
// records reach the publisher.
type Stats struct {
	Received    uint64 // records successfully parsed from the socket
	Dropped     uint64 // outbound channel was full past SendTimeout (real loss before publisher)
	ParseErrors uint64 // ParseRecord rejected a line
	Reconnects  uint64 // TCP reconnect attempts after the first connect
}

// Client is a long-lived IQPlus TCP feed reader. One Client = one logical
// session that auto-reconnects until the supplied context is cancelled.
type Client struct {
	cfg   Config
	stats Stats
}

func New(cfg Config) *Client {
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.LoginTimeout == 0 {
		cfg.LoginTimeout = 10 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 60 * time.Second
	}
	if cfg.ReconnectDelay == 0 {
		cfg.ReconnectDelay = 5 * time.Second
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 2_000_000
	}
	if cfg.BufioReaderSize <= 0 {
		cfg.BufioReaderSize = 1024 * 1024
	}
	return &Client{cfg: cfg}
}

// Snapshot returns a copy of the current TCP-side counters.
func (c *Client) Snapshot() Stats {
	return Stats{
		Received:    atomic.LoadUint64(&c.stats.Received),
		Dropped:     atomic.LoadUint64(&c.stats.Dropped),
		ParseErrors: atomic.LoadUint64(&c.stats.ParseErrors),
		Reconnects:  atomic.LoadUint64(&c.stats.Reconnects),
	}
}

// Stream connects to IQPlus and pushes parsed records into the returned
// channel until ctx is cancelled. The channel is closed when the goroutine
// exits. On any TCP/login error the loop reconnects after ReconnectDelay.
//
// Backpressure model: when the outbound channel fills, the read loop BLOCKS
// the bufio reader (and ultimately the TCP socket) instead of dropping
// records. If SendTimeout > 0, a record is dropped after that timeout and
// stats.Dropped is incremented. Default (0) means block forever / only ctx
// can escape — preferring latency over data loss, which is what we want for
// market data that is otherwise reconstructable only via the EOD resend.
func (c *Client) Stream(ctx context.Context) <-chan Record {
	out := make(chan Record, c.cfg.BufferSize)
	go func() {
		defer close(out)
		addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
		first := true
		for {
			if ctx.Err() != nil {
				return
			}
			if !first {
				atomic.AddUint64(&c.stats.Reconnects, 1)
			}
			first = false
			if err := c.connectAndStream(ctx, addr, out); err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Log.Warn("iqplus connection error, reconnecting",
					zap.String("addr", addr),
					zap.Duration("delay", c.cfg.ReconnectDelay),
					zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.cfg.ReconnectDelay):
			}
		}
	}()
	return out
}

func (c *Client) connectAndStream(ctx context.Context, addr string, out chan<- Record) error {
	logger.Log.Info("iqplus dialing", zap.String("addr", addr))
	conn, err := net.DialTimeout("tcp", addr, c.cfg.DialTimeout)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if c.cfg.SocketRecvBuffer > 0 {
		if tc, ok := conn.(*net.TCPConn); ok {
			if err := tc.SetReadBuffer(c.cfg.SocketRecvBuffer); err != nil {
				logger.Log.Warn("set SO_RCVBUF failed; relying on OS default + auto-tune",
					zap.Int("requested_bytes", c.cfg.SocketRecvBuffer),
					zap.Error(err))
			} else {
				logger.Log.Info("SO_RCVBUF set",
					zap.Int("requested_bytes", c.cfg.SocketRecvBuffer))
			}
		}
	}

	// Cancel blocking reads if ctx is cancelled while we're stuck in I/O.
	stopCloser := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopCloser:
		}
	}()
	defer close(stopCloser)

	loginPayload := fmt.Sprintf("IQP|149|0|1|%s|%s\r\n", c.cfg.User, c.cfg.PasswordMD5)
	_ = conn.SetWriteDeadline(time.Now().Add(c.cfg.LoginTimeout))
	if _, err := conn.Write([]byte(loginPayload)); err != nil {
		return fmt.Errorf("write login: %w", err)
	}

	reader := bufio.NewReaderSize(conn, c.cfg.BufioReaderSize)

	if err := c.consumeLoginResponse(conn, reader); err != nil {
		return err
	}
	logger.Log.Info("iqplus login success, streaming",
		zap.String("user", c.cfg.User))

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read stream: %w", err)
		}
		// Strip stray binary prefix the server sometimes injects.
		if idx := strings.Index(line, "IQP|"); idx > 0 {
			line = line[idx:]
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		rec, err := ParseRecord(line)
		if err != nil {
			atomic.AddUint64(&c.stats.ParseErrors, 1)
			logger.Log.Warn("iqplus parse error",
				zap.Error(err), zap.String("line", line))
			continue
		}
		atomic.AddUint64(&c.stats.Received, 1)
		if err := c.send(ctx, out, rec); err != nil {
			return err
		}
	}
}

// send pushes a record onto out, blocking if the buffer is full so the TCP
// socket gets backpressured instead of silently dropping data.
//
// If SendTimeout > 0, after that timeout we increment stats.Dropped and
// continue (preferring liveness over latency). Default SendTimeout=0 means
// block forever; only ctx cancel can escape.
func (c *Client) send(ctx context.Context, out chan<- Record, rec Record) error {
	if c.cfg.SendTimeout <= 0 {
		select {
		case out <- rec:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	timer := time.NewTimer(c.cfg.SendTimeout)
	defer timer.Stop()
	select {
	case out <- rec:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		atomic.AddUint64(&c.stats.Dropped, 1)
		logger.Log.Warn("iqplus channel full beyond send_timeout, dropping record",
			zap.Int64("seq", rec.Sequence),
			zap.Int("type", rec.RecordType),
			zap.Duration("waited", c.cfg.SendTimeout))
		return nil
	}
}

// consumeLoginResponse skips the binary banner the server emits before the
// real login reply, then validates the IQP|149|0|0 success line.
func (c *Client) consumeLoginResponse(conn net.Conn, reader *bufio.Reader) error {
	const maxBannerLines = 10
	for i := 0; i < maxBannerLines; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(c.cfg.LoginTimeout))
		raw, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read login response: %w (partial=%q)", err, raw)
		}
		idx := strings.Index(raw, "IQP|")
		if idx < 0 {
			continue
		}
		candidate := strings.TrimRight(raw[idx:], "\r\n")
		if !strings.HasPrefix(candidate, "IQP|149|0|") {
			continue
		}
		if !strings.HasPrefix(candidate, "IQP|149|0|0") {
			return fmt.Errorf("login failed: %s", candidate)
		}
		logger.Log.Info("iqplus login response", zap.String("line", candidate))
		return nil
	}
	return fmt.Errorf("no login response within %d lines", maxBannerLines)
}
