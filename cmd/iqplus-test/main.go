// Mini script to test IQPlus TCP feed connection.
// Spec: docs/iqplus/stock.md
//
// Usage:
//   go run ./cmd/iqplus-test
//   go run ./cmd/iqplus-test -duration=60s -types=15,27,57
//
// Env vars:
//   IQPLUS_HOST     (default: 103.114.143.237)
//   IQPLUS_PORT     (default: 8888)
//   IQPLUS_USER     (default: venturo)
//   IQPLUS_PASS_MD5 (default: ca0a9bbcfead18109abda813e3c03981)
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var debug bool

const (
	defaultHost     = "103.114.143.237"
	defaultPort     = 8888
	defaultUser     = "venturo"
	defaultPassMD5  = "ca0a9bbcfead18109abda813e3c03981"
	dialTimeout     = 10 * time.Second
	loginTimeout    = 10 * time.Second
	readTimeout     = 60 * time.Second
	reconnectDelay  = 5 * time.Second
)

type Record struct {
	Date       string
	Time       string
	Sequence   int64
	RecordType int
	RawData    string
	ReceivedAt time.Time
}

type typeFilter map[int]struct{}

func (f typeFilter) match(t int) bool {
	if len(f) == 0 {
		return true
	}
	_, ok := f[t]
	return ok
}

func main() {
	var (
		duration   = flag.Duration("duration", 0, "how long to stream (0 = until ctrl+c)")
		typesCSV   = flag.String("types", "", "comma-separated record types to print (empty = all)")
		maxRecords = flag.Int("max", 0, "stop after printing N records (0 = unlimited)")
		printRaw   = flag.Bool("raw", false, "print raw line instead of parsed fields")
		statsEvery = flag.Duration("stats", 5*time.Second, "stats log interval")
		debugFlag  = flag.Bool("debug", false, "print hex dump of login exchange")
	)
	flag.Parse()
	debug = *debugFlag

	host := envOr("IQPLUS_HOST", defaultHost)
	portStr := envOr("IQPLUS_PORT", strconv.Itoa(defaultPort))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("invalid IQPLUS_PORT: %v", err)
	}
	user := envOr("IQPLUS_USER", defaultUser)
	passMD5 := envOr("IQPLUS_PASS_MD5", defaultPassMD5)

	filter := make(typeFilter)
	for _, s := range strings.Split(*typesCSV, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		t, err := strconv.Atoi(s)
		if err != nil {
			log.Fatalf("invalid type %q: %v", s, err)
		}
		filter[t] = struct{}{}
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *duration > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, *duration)
		defer stop()
	}

	out := make(chan Record, 10_000)

	go func() {
		addr := fmt.Sprintf("%s:%d", host, port)
		for {
			if ctx.Err() != nil {
				close(out)
				return
			}
			if err := connectAndStream(ctx, addr, user, passMD5, out); err != nil {
				log.Printf("[iqplus] connection error: %v, reconnecting in %s", err, reconnectDelay)
			}
			select {
			case <-ctx.Done():
				close(out)
				return
			case <-time.After(reconnectDelay):
			}
		}
	}()

	counts := make(map[int]*int64)
	var total int64
	ticker := time.NewTicker(*statsEvery)
	defer ticker.Stop()

	printed := 0
	start := time.Now()

consume:
	for {
		select {
		case <-ctx.Done():
			break consume
		case <-ticker.C:
			printStats(counts, atomic.LoadInt64(&total), time.Since(start))
		case rec, ok := <-out:
			if !ok {
				break consume
			}
			atomic.AddInt64(&total, 1)
			c, exists := counts[rec.RecordType]
			if !exists {
				var n int64
				c = &n
				counts[rec.RecordType] = c
			}
			atomic.AddInt64(c, 1)

			if !filter.match(rec.RecordType) {
				continue
			}

			if *printRaw {
				fmt.Printf("[%s %s seq=%d] IQP|%d|%s\n",
					rec.Date, rec.Time, rec.Sequence, rec.RecordType, rec.RawData)
			} else {
				printParsed(rec)
			}
			printed++
			if *maxRecords > 0 && printed >= *maxRecords {
				cancel()
				break consume
			}
		}
	}

	printStats(counts, atomic.LoadInt64(&total), time.Since(start))
	log.Printf("[iqplus] done. total=%d printed=%d elapsed=%s",
		atomic.LoadInt64(&total), printed, time.Since(start).Round(time.Millisecond))
}

func connectAndStream(ctx context.Context, addr, user, md5Pass string, out chan<- Record) error {
	log.Printf("[iqplus] dialing %s ...", addr)
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	payload := fmt.Sprintf("IQP|149|0|1|%s|%s\r\n", user, md5Pass)
	if debug {
		log.Printf("[iqplus] sending login payload (%d bytes):\n%s", len(payload), hex.Dump([]byte(payload)))
	}
	_ = conn.SetWriteDeadline(time.Now().Add(loginTimeout))
	if _, err := conn.Write([]byte(payload)); err != nil {
		return fmt.Errorf("write login: %w", err)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)

	// Server kirim binary banner sebelum login response — skip sampai ketemu IQP|149|0|
	const maxBannerLines = 10
	var loginLine string
	for i := 0; i < maxBannerLines; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(loginTimeout))
		raw, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read login response: %w (partial=%q)", err, raw)
		}
		if debug {
			log.Printf("[iqplus] raw line %d (%d bytes):\n%s", i, len(raw), hex.Dump([]byte(raw)))
		}
		// Strip binary prefix — cari "IQP|" di dalam line
		idx := strings.Index(raw, "IQP|")
		if idx < 0 {
			if debug {
				log.Printf("[iqplus] skipping non-IQP banner line %d", i)
			}
			continue
		}
		candidate := strings.TrimRight(raw[idx:], "\r\n")
		if !strings.HasPrefix(candidate, "IQP|149|0|") {
			if debug {
				log.Printf("[iqplus] skipping non-login IQP line: %q", candidate)
			}
			continue
		}
		loginLine = candidate
		break
	}
	if loginLine == "" {
		return fmt.Errorf("no login response after %d lines", maxBannerLines)
	}
	log.Printf("[iqplus] login response: %s", loginLine)
	if !strings.HasPrefix(loginLine, "IQP|149|0|0") {
		return fmt.Errorf("login failed: %s", loginLine)
	}
	log.Printf("[iqplus] login success as %s, streaming...", user)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read stream: %w", err)
		}
		if idx := strings.Index(line, "IQP|"); idx > 0 {
			line = line[idx:]
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		rec, err := parseRecord(line)
		if err != nil {
			log.Printf("[iqplus] parse error: %v, line: %q", err, line)
			continue
		}
		select {
		case out <- rec:
		case <-ctx.Done():
			return ctx.Err()
		default:
			log.Printf("[iqplus] WARN: channel full, dropping seq=%d type=%d", rec.Sequence, rec.RecordType)
		}
	}
}

func parseRecord(line string) (Record, error) {
	if !strings.HasPrefix(line, "IQP|") {
		return Record{}, fmt.Errorf("missing IQP header")
	}
	parts := strings.SplitN(line, "|", 6)
	if len(parts) < 6 {
		return Record{}, fmt.Errorf("insufficient fields: got %d", len(parts))
	}
	seq, _ := strconv.ParseInt(parts[3], 10, 64)
	rt, _ := strconv.Atoi(parts[4])
	return Record{
		Date:       parts[1],
		Time:       parts[2],
		Sequence:   seq,
		RecordType: rt,
		RawData:    parts[5],
		ReceivedAt: time.Now(),
	}, nil
}

func printParsed(rec Record) {
	fields := strings.Split(rec.RawData, "|")
	label := recordTypeName(rec.RecordType)
	fmt.Printf("[%s %s seq=%-8d type=%3d %-14s] %s\n",
		rec.Date, rec.Time, rec.Sequence, rec.RecordType, label,
		strings.Join(fields, " | "))
}

func recordTypeName(t int) string {
	switch t {
	case 13:
		return "control"
	case 14:
		return "quote"
	case 15:
		return "trade"
	case 16:
		return "order"
	case 17:
		return "top20"
	case 18:
		return "best-quote"
	case 26:
		return "resend-order"
	case 27:
		return "resend-trade"
	case 36:
		return "news"
	case 39:
		return "activity"
	case 40:
		return "trade-done"
	case 57:
		return "trading-stat"
	case 58:
		return "nbs-stock"
	case 59:
		return "nbs-broker"
	case 130:
		return "trading-sum"
	case 149:
		return "login-resp"
	default:
		return "unknown"
	}
}

func printStats(counts map[int]*int64, total int64, elapsed time.Duration) {
	if total == 0 {
		log.Printf("[stats] no records yet (elapsed=%s)", elapsed.Round(time.Second))
		return
	}
	rate := float64(total) / elapsed.Seconds()
	var parts []string
	for t, c := range counts {
		parts = append(parts, fmt.Sprintf("%d=%d", t, atomic.LoadInt64(c)))
	}
	log.Printf("[stats] total=%d (%.1f rec/s) by_type=[%s]",
		total, rate, strings.Join(parts, " "))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func isPrintableASCII(s string) bool {
	for _, b := range []byte(s) {
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}
