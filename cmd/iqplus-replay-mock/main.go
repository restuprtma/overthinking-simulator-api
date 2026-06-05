// Replay-mock TCP server: emits IQPlus Type 27 wire frames built from
// real QuestDB trade rows (NDJSON / JSON-lines export). Used for load
// testing the publisher with realistic data shape and burst patterns.
//
// Input file format (1 line per trade — format produced by QuestDB
// `/exec?query=...&fmt=json` then jq'd to NDJSON, OR by:
//   /tmp/exec.sh "SELECT timestamp,stock,trade_no,buyer,buyer_type,
//     seller,seller_type,buyer_order_no,seller_order_no,price,volume
//     FROM trades WHERE ... LIMIT N" | jq -c '.dataset[]'):
//
//   ["2026-04-30T02:00:01.000000Z","IMPC",2573190,"CC","F","XL","D",
//    6604548,6560444,2220,600]
//
// Field order MUST match the SELECT — the mock indexes by position.
//
// Wire output (per record) — same as the real iqplus-mock-server emits:
//
//	IQP|<date>|<time>|<seq>|27|<stock>|<date>|<time>|<trade_no>|0|
//	    <price>|<volume>|<buyer>|<bt>|<seller>|<st>|<bo>|<so>\r\n
//
// Run:
//
//	./iqplus-replay-mock -listen :18888 -input /tmp/trades_today.ndjson \
//	    -rps 0 -loops 1 -seq-base 9000000000
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	listenAddr     = flag.String("listen", ":18888", "TCP listen address")
	inputFile      = flag.String("input", "", "NDJSON dataset to replay (one [tuple] per line)")
	loops          = flag.Int("loops", 1, "how many times to replay the dataset (>=1)")
	ratePerSec     = flag.Int("rps", 0, "throttle target records/sec; 0 = max speed (burst)")
	holdAfter      = flag.Duration("hold-after", 5*time.Minute, "after replay, hold connection alive this long with keepalives")
	statsTick      = flag.Duration("stats-tick", 5*time.Second, "stats log interval")
	heartbeatEvery = flag.Duration("heartbeat-every", 30*time.Second, "Type-13 keepalive while idle")
	tcpNoDelay     = flag.Bool("nodelay", true, "TCP_NODELAY on accepted socket")
	seqBase        = flag.Int64("seq-base", 9_000_000_000, "starting wire-Sequence (avoid colliding with live publisher MsgIDs)")
	logHandshake   = flag.Int("log-handshake", 5, "lines of handshake to log verbatim")
)

// row is a positional view of one NDJSON record. The columns are fixed by
// the SELECT we ask the user to run — see top-of-file comment.
type row struct {
	Timestamp     string  `json:"-"`
	Stock         string  `json:"-"`
	TradeNo       int64   `json:"-"`
	Buyer         string  `json:"-"`
	BuyerType     string  `json:"-"`
	Seller        string  `json:"-"`
	SellerType    string  `json:"-"`
	BuyerOrderNo  int64   `json:"-"`
	SellerOrderNo int64   `json:"-"`
	Price         float64 `json:"-"`
	Volume        int64   `json:"-"`
}

// parseTuple parses a JSON array tuple into a row by positional index.
// Position must match the canonical SELECT:
//
//	[0] timestamp string
//	[1] stock string
//	[2] trade_no int
//	[3] buyer string
//	[4] buyer_type string
//	[5] seller string
//	[6] seller_type string
//	[7] buyer_order_no int
//	[8] seller_order_no int
//	[9] price number
//	[10] volume int
func parseTuple(line []byte) (row, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return row{}, err
	}
	if len(raw) < 11 {
		return row{}, fmt.Errorf("expected 11 columns, got %d", len(raw))
	}
	var r row
	if err := json.Unmarshal(raw[0], &r.Timestamp); err != nil {
		return row{}, fmt.Errorf("col0 timestamp: %w", err)
	}
	if err := json.Unmarshal(raw[1], &r.Stock); err != nil {
		return row{}, fmt.Errorf("col1 stock: %w", err)
	}
	if err := json.Unmarshal(raw[2], &r.TradeNo); err != nil {
		return row{}, fmt.Errorf("col2 trade_no: %w", err)
	}
	if err := json.Unmarshal(raw[3], &r.Buyer); err != nil {
		return row{}, fmt.Errorf("col3 buyer: %w", err)
	}
	if err := json.Unmarshal(raw[4], &r.BuyerType); err != nil {
		return row{}, fmt.Errorf("col4 buyer_type: %w", err)
	}
	if err := json.Unmarshal(raw[5], &r.Seller); err != nil {
		return row{}, fmt.Errorf("col5 seller: %w", err)
	}
	if err := json.Unmarshal(raw[6], &r.SellerType); err != nil {
		return row{}, fmt.Errorf("col6 seller_type: %w", err)
	}
	if err := json.Unmarshal(raw[7], &r.BuyerOrderNo); err != nil {
		return row{}, fmt.Errorf("col7 buyer_order_no: %w", err)
	}
	if err := json.Unmarshal(raw[8], &r.SellerOrderNo); err != nil {
		return row{}, fmt.Errorf("col8 seller_order_no: %w", err)
	}
	if err := json.Unmarshal(raw[9], &r.Price); err != nil {
		return row{}, fmt.Errorf("col9 price: %w", err)
	}
	if err := json.Unmarshal(raw[10], &r.Volume); err != nil {
		return row{}, fmt.Errorf("col10 volume: %w", err)
	}
	return r, nil
}

// loadDataset slurps the NDJSON file fully into memory. Each line is a
// trade tuple. Empty lines and lines starting with `[#` are skipped.
func loadDataset(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows := make([]row, 0, 1<<20) // pre-size to ~1M rows
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineno := 0
	for sc.Scan() {
		lineno++
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		// Skip first-line jq metadata or comments.
		if b[0] != '[' {
			continue
		}
		r, err := parseTuple(b)
		if err != nil {
			log.Printf("dataset line %d: %v (skipping)", lineno, err)
			continue
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows parsed from %s", path)
	}
	log.Printf("dataset loaded: %d rows from %s", len(rows), path)
	return rows, nil
}

// timestampToWire converts an ISO-8601 UTC timestamp string from QuestDB
// into IQPlus wire date+time fields (WIB). Returns (yyyymmdd, hhmmss).
//
// QuestDB stores in UTC; IQPlus wire field uses Asia/Jakarta (WIB, UTC+7).
func timestampToWire(iso string) (string, string) {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		// Fallback: try without nanos.
		t, err = time.Parse("2006-01-02T15:04:05Z", iso)
		if err != nil {
			// Last resort: today, midnight.
			t = time.Now().UTC()
		}
	}
	wib := t.In(time.FixedZone("WIB", 7*3600))
	return wib.Format("20060102"), wib.Format("150405")
}

func buildFrame(seq int64, r row) string {
	dateStr, timeStr := timestampToWire(r.Timestamp)
	buyer := r.Buyer
	if buyer == "" {
		buyer = "--"
	}
	seller := r.Seller
	if seller == "" {
		seller = "--"
	}
	bt := r.BuyerType
	if bt == "" {
		bt = "-"
	}
	st := r.SellerType
	if st == "" {
		st = "-"
	}
	return fmt.Sprintf(
		"IQP|%s|%s|%d|27|%s|%s|%s|%d|0|%d|%d|%s|%s|%s|%s|%d|%d\r\n",
		dateStr, timeStr, seq,
		r.Stock, dateStr, timeStr, r.TradeNo,
		int64(r.Price), r.Volume, buyer, bt, seller, st,
		r.BuyerOrderNo, r.SellerOrderNo,
	)
}

func main() {
	flag.Parse()
	if *inputFile == "" {
		log.Fatal("--input is required (NDJSON file)")
	}
	if *loops < 1 {
		*loops = 1
	}

	dataset, err := loadDataset(*inputFile)
	if err != nil {
		log.Fatalf("load dataset: %v", err)
	}

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", *listenAddr, err)
	}
	log.Printf("replay-mock listening on %s | rows=%d loops=%d rps=%d total=%d",
		*listenAddr, len(dataset), *loops, *ratePerSec, len(dataset)*(*loops))

	ctx, cancel := signalCtx()
	defer cancel()
	go func() { <-ctx.Done(); _ = ln.Close() }()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("accept: %v", err)
			continue
		}
		go handleClient(ctx, conn, dataset)
	}
}

func handleClient(ctx context.Context, conn net.Conn, dataset []row) {
	remote := conn.RemoteAddr().String()
	log.Printf("client connected: %s", remote)
	defer func() {
		_ = conn.Close()
		log.Printf("client disconnected: %s", remote)
	}()

	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(*tcpNoDelay)
		_ = tc.SetWriteBuffer(8 * 1024 * 1024)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)
	writer := bufio.NewWriterSize(conn, 1024*1024)

	// Login handshake.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	loginLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("read login: %v", err)
		return
	}
	if *logHandshake > 0 {
		log.Printf("login from %s: %q", remote, strings.TrimRight(loginLine, "\r\n"))
	}
	if !strings.HasPrefix(strings.TrimLeft(loginLine, "\x00"), "IQP|149|") {
		log.Printf("unexpected login frame: %q", loginLine)
		return
	}
	_, _ = writer.WriteString("IQP|149|0|0|OK\r\n")
	_ = writer.Flush()

	// Burst.
	_ = conn.SetWriteDeadline(time.Time{})
	burstCtx, burstCancel := context.WithCancel(ctx)
	defer burstCancel()

	var sent int64
	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		t := time.NewTicker(*statsTick)
		defer t.Stop()
		var last int64
		var lastTs = time.Now()
		for {
			select {
			case <-burstCtx.Done():
				return
			case tNow := <-t.C:
				cur := atomic.LoadInt64(&sent)
				delta := cur - last
				secs := tNow.Sub(lastTs).Seconds()
				rate := float64(delta) / secs
				log.Printf("STATS sent=%d delta=%d rate=%.0f rec/s remote=%s",
					cur, delta, rate, remote)
				last = cur
				lastTs = tNow
			}
		}
	}()

	burstStart := time.Now()
	throttle := newThrottle(*ratePerSec)
	seq := *seqBase
	rowsTotal := int64(len(dataset)) * int64(*loops)

	for loop := 0; loop < *loops; loop++ {
		for i := range dataset {
			if burstCtx.Err() != nil {
				goto burstDone
			}
			throttle.wait()
			line := buildFrame(seq, dataset[i])
			seq++
			if _, err := writer.WriteString(line); err != nil {
				log.Printf("write record: %v", err)
				goto burstDone
			}
			s := atomic.AddInt64(&sent, 1)
			if s%4096 == 0 {
				if err := writer.Flush(); err != nil {
					log.Printf("flush mid-burst: %v", err)
					goto burstDone
				}
			}
		}
	}
burstDone:
	if err := writer.Flush(); err != nil {
		log.Printf("flush final: %v", err)
	}
	burstElapsed := time.Since(burstStart)
	burstCancel()
	<-statsDone
	finalSent := atomic.LoadInt64(&sent)
	log.Printf("BURST DONE sent=%d/%d elapsed=%s rate=%.0f rec/s",
		finalSent, rowsTotal, burstElapsed, float64(finalSent)/burstElapsed.Seconds())

	// Hold + keepalive (Type-13 control).
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

type throttle struct{ t *time.Ticker }

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
	go func() { <-ch; log.Print("signal received; shutting down"); cancel() }()
	return ctx, cancel
}

var _ = io.EOF
