// Package assembler buffers IQPlus News packets in memory and emits a
// complete News once every packet for a News_id has arrived.
//
// Trade-offs (why in-memory and not Redis):
//   - News rate is low (~5 msg/s peak) and packets per news are small (1-4)
//   - Single consumer instance — no need for cross-process buffer
//   - Restart loses partial news → vendor occasionally resends, otherwise
//     1-2 truncated articles is acceptable for MVP
//
// Stale buffers (incomplete after StaleAfter) are evicted by a periodic
// sweeper goroutine to bound memory.
package assembler

import (
	"sort"
	"strings"
	"sync"
	"time"

	"tuai/internal/modules/stock/news_consumer/parser"
)

// News is a fully-assembled news article ready to insert into MongoDB.
type News struct {
	NewsID        string
	Timestamp     time.Time
	Date          string
	Time          string
	Category      string
	CompanyID     string
	Headline      string
	Story         string
	PacketsRecv   int
	NumPackets    int
}

// Config controls assembler behavior.
type Config struct {
	// StaleAfter — incomplete buffers older than this are dropped to
	// prevent memory leaks from never-completing news. Default 10 min.
	StaleAfter time.Duration
	// SweepInterval — how often to scan for stale buffers. Default 1 min.
	SweepInterval time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		StaleAfter:    10 * time.Minute,
		SweepInterval: 1 * time.Minute,
	}
}

// Stats are best-effort counters surfaced for ops/logging.
type Stats struct {
	BuffersOpen      int    // current incomplete buffers
	NewsAssembled    uint64 // completed and emitted
	BuffersEvicted   uint64 // dropped stale (incomplete past StaleAfter)
	DuplicatePackets uint64 // packet arrived for slot already filled
}

// buffer holds the packets received so far for one News_id.
type buffer struct {
	headline    string
	category    string
	companyID   string
	newsID      string
	timestamp   time.Time
	date        string
	tm          string
	numPackets  int
	chunks      map[int]string
	createdAt   time.Time
	lastUpdate  time.Time
}

// Assembler accumulates packets and emits assembled News.
type Assembler struct {
	cfg     Config
	mu      sync.Mutex
	bufs    map[string]*buffer
	stats   struct {
		assembled    uint64
		evicted      uint64
		duplicate    uint64
	}
	stopCh chan struct{}
}

func New(cfg Config) *Assembler {
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 10 * time.Minute
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = 1 * time.Minute
	}
	return &Assembler{
		cfg:    cfg,
		bufs:   make(map[string]*buffer, 64),
		stopCh: make(chan struct{}),
	}
}

// StartSweeper launches the background eviction loop. Call Stop() to halt.
func (a *Assembler) StartSweeper() {
	go a.sweepLoop()
}

func (a *Assembler) sweepLoop() {
	t := time.NewTicker(a.cfg.SweepInterval)
	defer t.Stop()
	for {
		select {
		case <-a.stopCh:
			return
		case now := <-t.C:
			a.evictStale(now)
		}
	}
}

// Stop halts the sweeper.
func (a *Assembler) Stop() {
	select {
	case <-a.stopCh:
		// already stopped
	default:
		close(a.stopCh)
	}
}

// Add ingests one packet. Returns a non-nil *News when the packet was
// the final one needed (caller should insert it). Otherwise nil.
func (a *Assembler) Add(p parser.Packet) *News {
	a.mu.Lock()
	defer a.mu.Unlock()

	buf, ok := a.bufs[p.NewsID]
	if !ok {
		buf = &buffer{
			headline:   p.Headline,
			category:   p.Category,
			companyID:  p.CompanyID,
			newsID:     p.NewsID,
			timestamp:  p.Timestamp,
			date:       p.Date,
			tm:         p.Time,
			numPackets: p.NumPackets,
			chunks:     make(map[int]string, p.NumPackets),
			createdAt:  time.Now(),
		}
		a.bufs[p.NewsID] = buf
	}

	if _, dup := buf.chunks[p.CurrentPacket]; dup {
		a.stats.duplicate++
		// Already had this slot — overwrite (vendor may resend with same content).
	}
	buf.chunks[p.CurrentPacket] = p.StoryChunk
	buf.lastUpdate = time.Now()

	// Headline / metadata may differ slightly between packets; keep the
	// non-empty value if we got a blank earlier.
	if buf.headline == "" {
		buf.headline = p.Headline
	}
	if buf.category == "" {
		buf.category = p.Category
	}
	if buf.companyID == "" {
		buf.companyID = p.CompanyID
	}

	if len(buf.chunks) < buf.numPackets {
		return nil
	}

	news := assemble(buf)
	delete(a.bufs, p.NewsID)
	a.stats.assembled++
	return news
}

func assemble(b *buffer) *News {
	keys := make([]int, 0, len(b.chunks))
	for k := range b.chunks {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(b.chunks[k])
	}

	return &News{
		NewsID:      b.newsID,
		Timestamp:   b.timestamp,
		Date:        b.date,
		Time:        b.tm,
		Category:    b.category,
		CompanyID:   b.companyID,
		Headline:    b.headline,
		Story:       sb.String(),
		PacketsRecv: len(b.chunks),
		NumPackets:  b.numPackets,
	}
}

func (a *Assembler) evictStale(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cutoff := now.Add(-a.cfg.StaleAfter)
	for id, buf := range a.bufs {
		if buf.lastUpdate.Before(cutoff) {
			delete(a.bufs, id)
			a.stats.evicted++
		}
	}
}

// Snapshot returns a copy of current counters.
func (a *Assembler) Snapshot() Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Stats{
		BuffersOpen:      len(a.bufs),
		NewsAssembled:    a.stats.assembled,
		BuffersEvicted:   a.stats.evicted,
		DuplicatePackets: a.stats.duplicate,
	}
}
