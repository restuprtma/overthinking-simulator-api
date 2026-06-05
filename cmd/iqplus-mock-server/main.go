// Mock IQPlus TCP server for load-testing the publisher.
//
// Speaks the same wire protocol as the real IQPlus feed (per
// docs/iqplus/iqplus-data-feed-v4.0.0.md and parser.go) so an unmodified
// publisher binary can connect, log in, and consume records.
//
// Behavior:
//   1. Listens on -listen (default :18888).
//   2. Accepts the publisher connection.
//   3. Reads the IQP|149|0|1|<user>|<md5>\r\n login frame and replies
//      IQP|149|0|0|OK\r\n (no password check).
//   4. Sends Type 57 status "begin sending" then bursts N records of
//      configurable type at the requested rate (-rps 0 = max speed).
//   5. After the burst, sends a Type 13 control "UP" every 30s as
//      keepalive so the publisher's 60s ReadTimeout doesn't fire.
//   6. Holds connection until publisher disconnects or -hold-after expires.
//
// Each record is a Type 27 (Resend Trade) by default with realistic
// pipe-delim payload:
//
//	IQP|<date>|<time>|<seq>|27|<stock>|<date>|<time>|<trade_no>|0|<price>|<volume>|<buyer>|<bt>|<seller>|<st>|<bo>|<so>\r\n
//
// Half the records (configurable via -broker-pct) carry a real broker
// code pair (CC/XL) — mimicking EOD resend. The other half use "--" —
// mimicking mid-day resend that has no broker yet.
//
// IMPORTANT: for true kernel-TCP-buffer stress test, do NOT run on the
// SAME host as the publisher (loopback has near-zero reordering). Run
// the mock on the main VM (10.10.8.2) so the publisher (edge VM) reads
// across the LAN where reordering / drops can actually trigger.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	listenAddr     = flag.String("listen", ":18888", "listen address")
	totalRecords   = flag.Int64("count", 5_000_000, "total records to send before going idle")
	ratePerSec     = flag.Int("rps", 0, "throttle to N records/sec; 0 = max speed (burst)")
	brokerPct      = flag.Int("broker-pct", 100, "percentage of records that carry a real broker code (rest are '--')")
	stockCount     = flag.Int("stocks", 1000, "number of distinct synthetic stock codes to round-robin across")
	stockPrefix    = flag.String("stock-prefix", "LDT", "stock code prefix (3-char keeps subjects short)")
	recordType     = flag.Int("type", 27, "record type to emit (27=Resend Trade, 15=Live Trade)")
	holdAfter      = flag.Duration("hold-after", 5*time.Minute, "after burst, hold connection alive this long with keepalives, then close")
	statsTick      = flag.Duration("stats-tick", 5*time.Second, "stats log interval")
	heartbeatEvery = flag.Duration("heartbeat-every", 30*time.Second, "send Type 13 control while idle to keep publisher's 60s ReadTimeout alive")
	tcpNoDelay     = flag.Bool("nodelay", true, "TCP_NODELAY on accepted socket (off=may aggregate via Nagle)")
	logLines       = flag.Int("log-handshake", 5, "number of handshake lines to log verbatim")
	seqBase        = flag.Int64("seq-base", 9_000_000_000, "starting wire-Sequence so NATS dedup (Nats-Msg-Id=iqplus-<date>-<seq>) doesn't collide with the live publisher")
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", *listenAddr, err)
	}
	log.Printf("mock-iqplus listening on %s | count=%d rps=%d broker_pct=%d stocks=%d type=%d",
		*listenAddr, *totalRecords, *ratePerSec, *brokerPct, *stockCount, *recordType)

	ctx, cancel := signalCtx()
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("accept error: %v", err)
			continue
		}
		go handleClient(ctx, conn)
	}
}

func handleClient(ctx context.Context, conn net.Conn) {
	remote := conn.RemoteAddr().String()
	log.Printf("client connected: %s", remote)
	defer func() {
		_ = conn.Close()
		log.Printf("client disconnected: %s", remote)
	}()

	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(*tcpNoDelay)
		// Generous send buffer so the kernel doesn't stall us mid-burst.
		_ = tc.SetWriteBuffer(8 * 1024 * 1024)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)
	writer := bufio.NewWriterSize(conn, 1024*1024)

	// 1. Consume login line (IQP|149|0|1|user|md5\r\n).
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	loginLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("read login: %v", err)
		return
	}
	if *logLines > 0 {
		log.Printf("login from %s: %q", remote, strings.TrimRight(loginLine, "\r\n"))
	}
	if !strings.HasPrefix(strings.TrimLeft(loginLine, "\x00"), "IQP|149|") {
		log.Printf("unexpected login frame: %q", loginLine)
		return
	}

	// 2. Send login OK.
	if _, err := writer.WriteString("IQP|149|0|0|OK\r\n"); err != nil {
		log.Printf("write login resp: %v", err)
		return
	}
	if err := writer.Flush(); err != nil {
		log.Printf("flush login resp: %v", err)
		return
	}

	// 3. Send a Type 57 "Trading Status: begin sending" so the publisher
	//    knows it's live. (Optional; publisher does not gate on this.)
	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	timeStr := now.Format("150405")
	seq := *seqBase
	statusLine := fmt.Sprintf("IQP|%s|%s|%d|57|1\r\n", dateStr, timeStr, seq)
	seq++
	_, _ = writer.WriteString(statusLine)
	_ = writer.Flush()

	// 4. Burst N records.
	_ = conn.SetWriteDeadline(time.Time{}) // no per-write deadline; burst freely
	burstCtx, burstCancel := context.WithCancel(ctx)
	defer burstCancel()

	var sent int64
	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		t := time.NewTicker(*statsTick)
		defer t.Stop()
		var lastSent int64
		var lastTs = time.Now()
		for {
			select {
			case <-burstCtx.Done():
				return
			case tNow := <-t.C:
				cur := atomic.LoadInt64(&sent)
				delta := cur - lastSent
				secs := tNow.Sub(lastTs).Seconds()
				rate := float64(delta) / secs
				log.Printf("STATS sent=%d delta=%d rate=%.0f rec/s remote=%s",
					cur, delta, rate, remote)
				lastSent = cur
				lastTs = tNow
			}
		}
	}()

	burstStart := time.Now()
	throttle := newThrottle(*ratePerSec)

	for i := int64(0); i < *totalRecords; i++ {
		if burstCtx.Err() != nil {
			break
		}
		throttle.wait()
		line := buildRecord(seq, i)
		seq++
		if _, err := writer.WriteString(line); err != nil {
			log.Printf("write record (seq=%d): %v", seq-1, err)
			burstCancel()
			return
		}
		atomic.AddInt64(&sent, 1)
		// Periodically flush so records stream out instead of accumulating
		// in the bufio writer's 1 MiB buffer.
		if i%4096 == 0 {
			if err := writer.Flush(); err != nil {
				log.Printf("flush mid-burst: %v", err)
				burstCancel()
				return
			}
		}
	}
	if err := writer.Flush(); err != nil {
		log.Printf("flush final: %v", err)
	}
	burstElapsed := time.Since(burstStart)
	burstCancel()
	<-statsDone

	finalSent := atomic.LoadInt64(&sent)
	log.Printf("BURST DONE sent=%d elapsed=%s rate=%.0f rec/s",
		finalSent, burstElapsed, float64(finalSent)/burstElapsed.Seconds())

	// 5. Hold connection alive with periodic Type 13 control records so the
	//    publisher can drain its in-memory buffer + the data is fully consumed
	//    before we close. Without this, closing immediately after the last
	//    write may cause the publisher to see EOF before reading remaining
	//    bytes (less likely with TCP, but cheap insurance).
	holdCtx, holdCancel := context.WithTimeout(ctx, *holdAfter)
	defer holdCancel()
	hb := time.NewTicker(*heartbeatEvery)
	defer hb.Stop()
	for {
		select {
		case <-holdCtx.Done():
			log.Printf("hold-after expired, closing %s", remote)
			return
		case t := <-hb.C:
			ds := t.UTC().Format("20060102")
			ts := t.UTC().Format("150405")
			line := fmt.Sprintf("IQP|%s|%s|%d|13|0\r\n", ds, ts, seq)
			seq++
			if _, err := writer.WriteString(line); err != nil {
				log.Printf("write keepalive: %v", err)
				return
			}
			if err := writer.Flush(); err != nil {
				log.Printf("flush keepalive: %v", err)
				return
			}
		}
	}
}

// buildRecord composes a single wire frame.
//
//	idx  = monotonic counter (0..totalRecords-1)
//	seq  = wire-level Sequence integer (also monotonic)
func buildRecord(seq, idx int64) string {
	stockN := idx % int64(*stockCount)
	stock := fmt.Sprintf("%s%03d", *stockPrefix, stockN)

	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	// Spread time across the trading session so QuestDB designated timestamps
	// look realistic. Map idx to 09:00:00..15:59:59 WIB → 02:00:00..08:59:59 UTC.
	secOfDay := int64(2*3600) + (idx % (7 * 3600))
	hh := (secOfDay / 3600) % 24
	mm := (secOfDay / 60) % 60
	ss := secOfDay % 60
	timeStr := fmt.Sprintf("%02d%02d%02d", hh, mm, ss)

	tradeNo := 1_000_000 + idx
	price := 1000 + (idx % 5000) // realistic IDX price range
	volume := 100 + (idx*7)%10000

	withBroker := int(idx%100) < *brokerPct
	var buyer, bt, seller, st string
	var bOrd, sOrd int64
	if withBroker {
		buyer, bt, seller, st = "CC", "F", "XL", "D"
		bOrd = 4_000_000 + idx*3
		sOrd = 4_000_001 + idx*3
	} else {
		buyer, bt, seller, st = "--", "-", "--", "-"
		bOrd, sOrd = 0, 0
	}

	// Wire format:
	// IQP|<date>|<time>|<seq>|<type>|<stock>|<date>|<time>|<trade_no>|0|<price>|<volume>|<buyer>|<bt>|<seller>|<st>|<bo>|<so>\r\n
	return fmt.Sprintf(
		"IQP|%s|%s|%d|%d|%s|%s|%s|%d|0|%d|%d|%s|%s|%s|%s|%d|%d\r\n",
		dateStr, timeStr, seq, *recordType,
		stock, dateStr, timeStr, tradeNo,
		price, volume, buyer, bt, seller, st, bOrd, sOrd,
	)
}

// throttle is a no-op when rps==0 (max-speed burst). Otherwise it spreads
// requests at ~rps/sec using a ticker. We use a ticker per goroutine to
// avoid a global lock on the hot path.
type throttle struct {
	t *time.Ticker
}

func newThrottle(rps int) *throttle {
	if rps <= 0 {
		return &throttle{}
	}
	return &throttle{t: time.NewTicker(time.Second / time.Duration(rps))}
}

func (t *throttle) wait() {
	if t.t != nil {
		<-t.t.C
	}
}

func signalCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		log.Print("signal received; shutting down")
		cancel()
	}()
	return ctx, cancel
}

// Compile-time check that we use io somewhere (silences unused-import on
// some Go versions if a refactor removes the only ref).
var _ = io.EOF
