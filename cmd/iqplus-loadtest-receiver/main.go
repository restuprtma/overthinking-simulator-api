// Loadtest receiver — connects to a mock IQPlus server, exercises the
// SAME client.Client code path the production publisher uses (login,
// bufio reader, SO_RCVBUF, channel buffer), but DROPS every record into
// a counter instead of publishing to NATS.
//
// Purpose: isolate the TCP-kernel layer in load tests so we can measure
// pure receive-side loss without contaminating the live IDX_TICK stream
// with synthetic data.
//
// Compare:
//   mock-server "BURST DONE sent=N"  vs.  receiver "tcp_received=N"
// If sent > tcp_received → kernel-layer drops (TCP reassembly queue
// overflow, recv buffer too small, etc — surface via netstat -s -p tcp).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"tuai/internal/modules/stock/iqplus_publisher/client"
	"tuai/pkg/logger"
)

var (
	host           = flag.String("host", envOr("IQPLUS_HOST", "127.0.0.1"), "mock IQPlus server host")
	port           = flag.Int("port", envInt("IQPLUS_PORT", 18888), "mock IQPlus server port")
	user           = flag.String("user", envOr("IQPLUS_USER", "mock"), "login user")
	passMD5        = flag.String("pass", envOr("IQPLUS_PASS_MD5", "00000000000000000000000000000000"), "md5 password")
	bufferSize     = flag.Int("buffer", envInt("IQPLUS_BUFFER_SIZE", 2_000_000), "outbound channel buffer (records)")
	socketRecvBuf  = flag.Int("rcvbuf", envInt("IQPLUS_SOCKET_RECV_BUF", 16*1024*1024), "SO_RCVBUF bytes")
	bufioReadSize  = flag.Int("bufio", envInt("IQPLUS_BUFIO_READ_SIZE", 1024*1024), "bufio reader buffer bytes")
	statsInterval  = flag.Duration("stats", 5*time.Second, "stats log interval")
	runDuration    = flag.Duration("duration", 10*time.Minute, "max run duration; receiver exits after this regardless of mock state")
	idleExitAfter  = flag.Duration("idle-exit", 30*time.Second, "exit when no record received for this long after first record")
)

func main() {
	flag.Parse()

	if err := logger.Initialize(envOr("ENV", "production")); err != nil {
		log.Fatalf("logger init: %v", err)
	}

	cfg := client.DefaultConfig()
	cfg.Host = *host
	cfg.Port = *port
	cfg.User = *user
	cfg.PasswordMD5 = *passMD5
	cfg.BufferSize = *bufferSize
	cfg.SocketRecvBuffer = *socketRecvBuf
	cfg.BufioReaderSize = *bufioReadSize
	cfg.ReadTimeout = 60 * time.Second

	log.Printf("loadtest-receiver dialing %s:%d | buffer=%d rcvbuf=%d bufio=%d",
		cfg.Host, cfg.Port, cfg.BufferSize, cfg.SocketRecvBuffer, cfg.BufioReaderSize)

	c := client.New(cfg)

	rootCtx, cancel := context.WithTimeout(context.Background(), *runDuration)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Print("signal; exiting")
		cancel()
	}()

	out := c.Stream(rootCtx)

	var (
		recv         uint64
		byType       [256]uint64
		brokerOK     uint64
		brokerDash   uint64
		lastRecvUnix int64
		startTime    = time.Now()
	)

	statsTicker := time.NewTicker(*statsInterval)
	defer statsTicker.Stop()

	idleTicker := time.NewTicker(time.Second)
	defer idleTicker.Stop()

	logStats := func(final bool) {
		st := c.Snapshot()
		nrecv := atomic.LoadUint64(&recv)
		bok := atomic.LoadUint64(&brokerOK)
		bdash := atomic.LoadUint64(&brokerDash)
		elapsed := time.Since(startTime)
		rate := float64(nrecv) / elapsed.Seconds()
		tag := "STATS"
		if final {
			tag = "FINAL"
		}
		log.Printf("%s recv=%d rate=%.0f rec/s broker_ok=%d broker_dash=%d "+
			"client_received=%d client_dropped=%d client_parse_err=%d client_reconnects=%d "+
			"type27=%d type15=%d type57=%d type13=%d elapsed=%s",
			tag, nrecv, rate, bok, bdash,
			st.Received, st.Dropped, st.ParseErrors, st.Reconnects,
			byType[27], byType[15], byType[57], byType[13],
			elapsed.Round(time.Second))
	}

	for {
		select {
		case <-rootCtx.Done():
			logStats(true)
			return
		case <-statsTicker.C:
			logStats(false)
		case <-idleTicker.C:
			lru := atomic.LoadInt64(&lastRecvUnix)
			if lru > 0 && time.Since(time.Unix(lru, 0)) > *idleExitAfter {
				log.Printf("idle for %s; exiting", *idleExitAfter)
				logStats(true)
				return
			}
		case rec, ok := <-out:
			if !ok {
				logStats(true)
				return
			}
			atomic.AddUint64(&recv, 1)
			atomic.StoreInt64(&lastRecvUnix, time.Now().Unix())
			if rec.RecordType >= 0 && rec.RecordType < 256 {
				byType[rec.RecordType]++
			}
			// Cheap broker-vs-dash classifier on Type 27 / Type 15.
			// Field index 7 of RawData is the Buyer broker code in trade payloads
			// (per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.5). Mock generates
			// 8 chars before broker so we can do an inexpensive scan.
			if rec.RecordType == 27 || rec.RecordType == 15 {
				if hasDashBuyer(rec.RawData) {
					atomic.AddUint64(&brokerDash, 1)
				} else {
					atomic.AddUint64(&brokerOK, 1)
				}
			}
		}
	}
}

// hasDashBuyer returns true if the buyer field (8th pipe-segment, 0-indexed=7)
// is "--". Avoids strings.Split allocation in the hot path.
func hasDashBuyer(s string) bool {
	pipes := 0
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			if pipes == 7 {
				field := s[start:i]
				return field == "--"
			}
			pipes++
			start = i + 1
		}
	}
	return false
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Compile-time check that fmt is used (for future stat formatting).
var _ = fmt.Sprintf
