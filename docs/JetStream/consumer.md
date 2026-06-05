# Catatan untuk Backend Worker / Consumer NATS JetStream

> Konteks: VM `tuai-jetstream` (10.10.8.2), stream `STOCKS`, subjects `stocks.>`.
> Audience: developer yang implement worker QuestDB / Postgres / Redis / dll
> yang baca dari JetStream.

Bukan tutorial — ini **hal-hal yang biasa bikin masalah** kalau tidak
dipahami sejak awal.

> 📎 Untuk publisher, lihat `nats-publisher-notes.md`.
> Connection lifecycle, reconnect, dan token handling sama → tidak diulang di sini.

---

## 1. Mental Model Pertama: At-Least-Once

JetStream **bukan exactly-once delivery secara default**.
Yang exactly-once cuma **publish-side** (via `Msg-Id` dedup).
Untuk consumer, **delivery selalu at-least-once** — pesan yang sama bisa
sampai ke worker > 1× karena:

- Ack hilang di network → server kira gagal → redeliver.
- Worker crash setelah proses tapi sebelum Ack → redeliver.
- Worker lambat melewati `ack_wait` → server timeout → redeliver.

### Konsekuensi: **idempotency wajib di sisi worker**

Kalau worker insert tick ke QuestDB tanpa idempotency, tick yang sama bisa
masuk 2× dan harga rata-rata Anda jadi salah.

**Pola idempotency yang umum:**

| Pola                                                         | Cocok untuk                  |
| ------------------------------------------------------------ | ---------------------------- |
| `INSERT ON CONFLICT DO NOTHING` (PK = ticker+timestamp_ns)   | Postgres                     |
| `INSERT INTO ... DEDUP UPSERT`                               | QuestDB (ada native DEDUP)   |
| `SET key value EX ttl NX`                                    | Redis                        |
| Redis `SET nats:processed:<msgID> 1 EX 600 NX` sebelum write | Generic, bukti pernah-proses |
| Pakai `Nats-Msg-Id` dari header sebagai natural dedup key    | Universal                    |

```go
// Akses Msg-Id yang dikirim publisher
msgID := msg.Headers().Get("Nats-Msg-Id")
// pakai ini sebagai dedup key di DB
```

---

## 2. Pull vs Push Consumer

|                  | Pull (yang kita pakai)                                                  | Push                              |
| ---------------- | ----------------------------------------------------------------------- | --------------------------------- |
| Inisiatif fetch  | Worker                                                                  | Server                            |
| Flow control     | Otomatis (worker tarik sesuai kapasitas)                                | Manual (`max_msgs_per_sec`, dll)  |
| Horizontal scale | ✅ Trivial — banyak worker pakai durable name sama, server load-balance | ⚠️ Lebih ribet                    |
| Ack              | Wajib eksplisit                                                         | Bisa ack/no-ack                   |
| Recommended      | ✅ Hampir selalu                                                        | Untuk subscriber legacy / browser |

**Aturan praktis: pakai pull.** Push hanya kalau Anda punya constraint khusus.

---

## 3. Durable vs Ephemeral

```bash
# Durable (yang kita pakai)
nats consumer add STOCKS questdb-writer --pull ...
# State (ack_floor, redelivery count) tersimpan permanen.
# Nama tetap. Worker boleh restart, resume dari posisi terakhir.

# Ephemeral
# State hilang saat consumer disconnect. Auto-deleted.
# Cocok untuk: debugging, ad-hoc query, browser dashboard.
```

**Untuk worker yang tulis ke DB: WAJIB durable.** Tanpa itu, worker restart =
mulai dari awal stream lagi (kalau `DeliverAll`) atau lewatkan pesan yang
sempat masuk saat down (kalau `DeliverNew`).

---

## 4. API: `Consume()` vs `Fetch()` vs `Messages()`

Library `nats.go/jetstream` punya 3 cara baca pesan dari pull consumer.

### `Consume()` — push-style callback (paling umum)

```go
cc, err := cons.Consume(func(msg jetstream.Msg) {
    // handler dipanggil per pesan, library handle pull batching internal
    process(msg)
})
defer cc.Stop()
```

- Library otomatis batch fetch di belakang layar.
- Concurrent processing? Pakai goroutine pool atau parameter `PullMaxMessages`.
- **Pakai untuk**: streaming continuous, latency-sensitive (per-message processing).

### `Fetch(n)` — manual batch

```go
batch, err := cons.Fetch(500, jetstream.FetchMaxWait(200*time.Millisecond))
for msg := range batch.Messages() {
    process(msg)
}
```

- Anda kontrol berapa pesan yang ditarik tiap batch.
- **Pakai untuk**: bulk insert ke DB (QuestDB ILP, Postgres COPY) — 10–50× lebih cepat dari per-row.

### `Messages()` — iterator manual

```go
iter, err := cons.Messages()
defer iter.Stop()
for {
    msg, err := iter.Next()
    if err != nil { break }
    process(msg)
}
```

- Mirip `Consume` tapi caller yang loop. Kontrol penuh.
- **Pakai untuk**: state-machine kompleks, atau integrasi dengan event loop existing.

### Aturan praktis untuk use case stock

| Worker                                         | API          | Alasan                                 |
| ---------------------------------------------- | ------------ | -------------------------------------- |
| QuestDB writer (high-throughput, batch insert) | `Fetch(500)` | ILP optimal di batch                   |
| Postgres writer (per-row OK)                   | `Consume()`  | Simpler, low-volume table biasanya     |
| Redis writer (cache last price per ticker)     | `Consume()`  | Set per-key, tidak ada gain dari batch |
| Alert / notifikasi (low-volume)                | `Consume()`  | Latency penting                        |

---

## 5. Ack Methods — Mana yang Dipakai Kapan

| Method                | Efek di server                                  | Pakai saat                                                    |
| --------------------- | ----------------------------------------------- | ------------------------------------------------------------- |
| `msg.Ack()`           | Pesan sukses, advance ack_floor                 | Berhasil tulis ke DB                                          |
| `msg.Nak()`           | Negative ack, **langsung** redeliver            | Error transient (DB sibuk sebentar) — jarang dipakai langsung |
| `msg.NakWithDelay(d)` | Negative ack, redeliver setelah `d`             | Error transient (kasih nafas DB) — **default error path**     |
| `msg.Term()`          | Pesan **rusak permanen**, jangan redeliver lagi | Decode error, schema invalid, business rule fail              |
| `msg.InProgress()`    | "Masih kerja, jangan timeout" — extend ack-wait | Pekerjaan butuh > ack-wait (default 30s)                      |

### Decision tree

```
Pesan masuk handler
├─ Decode JSON gagal?           → Term()      (jangan retry barang rusak)
├─ Schema/validation fail?      → Term()      (atau publish ke DLQ subject dulu, lalu Term)
├─ DB error transient (timeout, sibuk)?  → NakWithDelay(2s-30s)
├─ Worker shutting down (ctx canceled)?  → Nak()  (biar worker lain handle)
├─ Process > 25s?               → InProgress() periodic, lalu Ack/Nak di akhir
└─ Sukses                        → Ack()
```

### ⚠️ Jangan pernah skip Ack tanpa keputusan

Pesan yang tidak di-ack/nak/term akan timeout di `ack_wait` (default 30s) dan
diredeliver. Tiga kali kena (default `max_deliver`) → masuk DLQ atau drop.
**Diam-diam tidak meng-ack adalah bug yang paling sulit dideteksi**, karena
worker kelihatan "berhasil" tapi server keep redelivering.

---

## 6. Tuning `ack_wait` dan `max_deliver`

Setup kita saat ini:

```
--ack-wait=30s      # default jetstream
--max-deliver=3     # max 3× retry sebelum di-skip
```

### Kapan naikkan `ack_wait`?

Kalau worker proses > 30s per pesan (mis. ML inference, big query),
naikkan ke 5–10 menit:

```bash
nats consumer edit STOCKS questdb-writer --ack-wait=5m
```

**Atau lebih baik**: panggil `msg.InProgress()` periodic (mis. tiap 10s) supaya
ack-wait di-reset terus selama worker masih kerja. Lebih responsive saat
worker beneran crash.

### Kapan naikkan `max_deliver`?

Jarang — 3× sudah cukup untuk transient error. Kalau pesan gagal 3× berturut,
biasanya **bug, bukan masalah transient**. Lebih baik kirim ke DLQ daripada
retry sampai forever.

### Trade-off

| Setting                                 | Efek                                                    |
| --------------------------------------- | ------------------------------------------------------- |
| `ack_wait` rendah, `max_deliver` tinggi | Worker lambat → banyak redelivery (dupliksi processing) |
| `ack_wait` tinggi, `max_deliver` rendah | Worker crash → pesan stuck lama sebelum di-redeliver    |

Sweet spot: `ack_wait = 2× p99 processing time`, `max_deliver = 3-5`.

---

## 7. Max Ack Pending — Backpressure Worker

```bash
--max-pending=1000   # max 1000 pesan in-flight tanpa ack
```

Server akan **stop kirim** pesan ke worker kalau in-flight (delivered but
not yet ack'd) sudah mencapai `max-pending`. Worker terlalu lambat?
Server akan tunggu, bukan banjiri RAM worker.

### Tuning

| Skenario                                                | `max_pending`                 |
| ------------------------------------------------------- | ----------------------------- |
| Worker single-threaded, per-msg processing cepat (<5ms) | 100–500                       |
| Worker batch, fetch 500 per call                        | 1000–2000 (= 2-4× batch size) |
| Worker dengan goroutine pool (mis. 50 concurrent)       | 500–1000                      |
| Long-running per-msg (>1s)                              | 50–200                        |

**Kalau worker OOM**: turunkan `max_pending`.
**Kalau worker idle padahal ada lag**: naikkan.

---

## 8. Filter Subject — Server-Side Filter

Daripada subscribe ke `stocks.>` lalu filter di Go, lebih efisien filter
server-side:

```bash
# Worker hanya butuh saham LQ45
nats consumer add STOCKS redis-lq45 \
  --pull --filter "stocks.idx.BBCA,stocks.idx.TLKM,stocks.idx.BBRI,..." \
  --ack explicit ...
```

- Network jadi lebih hemat (server tidak kirim pesan yang tidak match).
- Library Go hanya proses pesan relevan.
- Wildcard didukung: `stocks.idx.*.tick` (semua ticker, hanya tick).

### Multi-filter (NATS 2.10+)

```bash
--filter "stocks.idx.BBCA,stocks.idx.TLKM"   # explicit list
--filter "stocks.idx.>"                       # subtree
--filter "stocks.*.BBCA.>"                    # all market for BBCA
```

---

## 9. Replay Policy — Dari Mana Mulai Membaca?

Saat consumer **pertama kali** dibuat:

| Policy                   | Mulai dari                                                       |
| ------------------------ | ---------------------------------------------------------------- |
| `DeliverAll` (default)   | Pesan paling awal di stream — **replay dari awal**               |
| `DeliverNew`             | Pesan baru sejak consumer dibuat (lewatkan history)              |
| `DeliverLast`            | Hanya pesan terakhir di stream                                   |
| `DeliverLastPerSubject`  | Pesan terakhir per unique subject (snapshot terkini per ticker!) |
| `DeliverByStartSequence` | Mulai dari stream sequence N                                     |
| `DeliverByStartTime`     | Mulai dari timestamp tertentu                                    |

Setelah pertama kali, posisi ditentukan oleh `ack_floor` consumer — replay
policy tidak berpengaruh lagi.

### Use case typical

| Worker                                   | Policy                  | Alasan                             |
| ---------------------------------------- | ----------------------- | ---------------------------------- |
| Initial QuestDB load (recompute history) | `DeliverAll`            | Backfill semua tick                |
| Production QuestDB writer (sudah live)   | `DeliverNew`            | Mulai dari sekarang, tidak overlap |
| Redis cache "harga terkini per ticker"   | `DeliverLastPerSubject` | Warm-up cache cepat                |
| Recompute setelah bug fix                | `DeliverByStartTime`    | Replay sejak X                     |

---

## 10. Horizontal Scaling — Banyak Worker Pakai Consumer Sama

Ini salah satu **fitur terbesar** JetStream pull consumer: beberapa worker
bisa subscribe ke consumer durable yang sama, server akan **load-balance**.

```bash
# Bukan: bikin 3 consumer berbeda
# Tapi: jalankan 3 instance worker yang semuanya bind ke `questdb-writer`

# Worker 1, 2, 3 (di pod/VM berbeda):
go run ./cmd/worker-questdb   # masing-masing bind ke STOCKS/questdb-writer
```

Server akan kirim pesan ke worker mana saja yang punya kapasitas (in-flight <
`max-pending`). Tiap pesan hanya di-deliver ke **satu** worker (bukan broadcast).

### Kalau salah satu worker mati

- In-flight pesan di worker mati → ack timeout setelah `ack_wait` → server
  redeliver ke worker yang masih hidup.
- Auto-failover, no config needed.

### Kalau perlu broadcast (semua worker dapat semua pesan)

Bukan dengan satu consumer. **Bikin consumer berbeda untuk tiap worker:**

```bash
nats consumer add STOCKS analytics-1 --pull --filter "stocks.>" ...
nats consumer add STOCKS analytics-2 --pull --filter "stocks.>" ...
```

Tiap consumer = stream offset independen. Sama persis pola fan-out
QuestDB / Postgres / Redis writer di setup kita.

---

## 11. Dead Letter Queue (DLQ) Pattern

Pesan yang gagal `max_deliver` kali → server hentikan delivery. Default-nya
**hilang dari perspektif worker** (masih ada di stream, tapi consumer tidak
akan dapat lagi).

### Pattern: kirim ke DLQ subject

```go
// Cek redelivery count
meta, _ := msg.Metadata()
if meta.NumDelivered >= 3 {
    // Sebelum Term, publish ke DLQ untuk diinvestigasi
    dlqBody := struct {
        OriginalSubject string          `json:"original_subject"`
        Error           string          `json:"error"`
        Payload         json.RawMessage `json:"payload"`
        Headers         nats.Header     `json:"headers"`
    }{
        OriginalSubject: msg.Subject(),
        Error:           lastErr.Error(),
        Payload:         msg.Data(),
        Headers:         msg.Headers(),
    }
    body, _ := json.Marshal(dlqBody)
    js.Publish(ctx, "dlq.stocks", body)
    msg.Term()
    return
}
```

Lalu bikin stream `DLQ` yang capture `dlq.>` untuk review manual.

### Atau pakai advisory subjects (built-in)

JetStream auto-publish event ke `$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.>`
saat pesan kena `max_deliver`. Subscribe ke situ untuk alerting.

---

## 12. Graceful Shutdown — Wajib Tahu

### Skenario buruk tanpa graceful shutdown

1. Worker ambil 100 pesan in-flight (ack_pending=100).
2. Pod kena SIGTERM (Kubernetes rolling deploy).
3. Worker langsung exit.
4. 100 pesan timeout di `ack_wait` (30s) → server redeliver ke worker lain.
5. **Latency spike 30s** untuk 100 pesan tersebut.

### Skenario benar

```go
ctx, stop := signal.NotifyContext(context.Background(),
    syscall.SIGINT, syscall.SIGTERM)
defer stop()

cc, _ := cons.Consume(handler)
<-ctx.Done()          // signal masuk
cc.Stop()             // STOP terima pesan baru, tunggu in-flight selesai
nc.Drain()            // close koneksi setelah semua flushed
```

### Kalau pakai `Fetch()`

```go
for {
    select {
    case <-ctx.Done():
        return
    default:
    }
    batch, _ := cons.Fetch(500, jetstream.FetchMaxWait(200*time.Millisecond))
    // … process batch …
    // pesan in-flight di batch ini akan selesai di-process sebelum loop next
}
```

### Penting di handler

```go
func handler(msg jetstream.Msg) {
    if ctx.Err() != nil {
        // shutting down — Nak supaya pesan ke worker lain, jangan stuck
        msg.Nak()
        return
    }
    // … normal process …
}
```

### Timeout shutdown

Kalau ada pesan yang processing-nya lama, bikin grace period:

```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
done := make(chan struct{})
go func() { cc.Stop(); close(done) }()
select {
case <-done:
case <-shutdownCtx.Done():
    log.Println("shutdown timeout — force exit")
}
```

---

## 13. Error Handling per Tipe

```go
err := writeToDB(ctx, t)
switch {
case err == nil:
    msg.Ack()

case errors.Is(err, context.Canceled):
    // worker shutting down
    msg.Nak()

case errors.Is(err, context.DeadlineExceeded):
    // DB lambat — kasih jeda lebih lama
    msg.NakWithDelay(10 * time.Second)

case isUniqueViolation(err):
    // PK conflict di Postgres — duplikat sudah ada, anggap sukses (idempotent)
    msg.Ack()

case isConnectionError(err):
    // DB down — backoff besar, biarkan worker lain coba
    msg.NakWithDelay(30 * time.Second)

case isSchemaError(err):
    // Data tidak bisa diproses ever — DLQ + Term
    publishToDLQ(msg, err)
    msg.Term()

default:
    // Unknown — log loud, retry dengan backoff sedang
    log.Printf("UNHANDLED err: %v", err)
    msg.NakWithDelay(5 * time.Second)
}
```

### Aturan emas

- **Permanent error** (decode, schema, business rule): `Term()`. Jangan habiskan retry.
- **Transient error** (network, DB busy): `NakWithDelay()` dengan delay yang masuk akal.
- **Idempotent conflict** (PK duplicate): `Ack()` — itu artinya pesan sudah pernah diproses, sukses.

---

## 14. Observability — Apa yang Diukur

### Metrics minimum dari worker

```
# Counter
nats_consume_total{consumer, status="ack|nak|term"}
nats_consume_processing_errors_total{consumer, error_type}

# Histogram
nats_consume_processing_duration_seconds{consumer}
nats_consume_message_age_seconds{consumer}      # = now - msg.Metadata().Timestamp

# Gauge
nats_consume_inflight{consumer}
nats_consume_redelivery_ratio{consumer}
```

### Metrics dari server (scrape dari `:8222/jsz?consumers=true`)

```
jetstream_consumer_num_pending{stream, consumer}    # ← KEY metric
jetstream_consumer_num_ack_pending{stream, consumer}
jetstream_consumer_num_redelivered{stream, consumer}
jetstream_consumer_delivered_consumer_seq{stream, consumer}
jetstream_consumer_ack_floor_consumer_seq{stream, consumer}
```

`num_pending` = jumlah pesan di stream yang **belum dikirim** ke consumer ini.
Kalau angka ini terus naik = worker tidak bisa keep up.

### Alert minimum

| Alert                   | Kondisi                                           | Severity                      |
| ----------------------- | ------------------------------------------------- | ----------------------------- |
| Consumer lag tinggi     | `num_pending > 10000 for 5m`                      | Warning                       |
| Consumer lag terus naik | rate(`num_pending`) > 0 for 10m                   | Critical                      |
| Redelivery tinggi       | rate(`num_redelivered`) > 1% of rate(`delivered`) | Warning                       |
| Worker error rate       | rate(`status="nak"`) > 5%                         | Warning                       |
| Term spike              | rate(`status="term"`) > 0.1/s                     | Critical (data quality issue) |
| Processing latency      | p99 `processing_duration_seconds` > ack_wait/2    | Warning                       |

### Akses metadata pesan

```go
meta, err := msg.Metadata()
// meta.Sequence.Stream, meta.Sequence.Consumer
// meta.NumDelivered, meta.NumPending
// meta.Timestamp (waktu publish)
// meta.Stream, meta.Consumer
```

`meta.NumDelivered > 1` = ini redelivery, log untuk investigasi.

---

## 15. Testing — Skenario Wajib

### Unit test

Pakai mock `jetstream.Msg`:

```go
type fakeMsg struct {
    data    []byte
    headers nats.Header
    acked   bool
    nakd    bool
    termd   bool
}
func (f *fakeMsg) Data() []byte { return f.data }
func (f *fakeMsg) Ack() error   { f.acked = true; return nil }
// ... dst
```

### Integration test

```go
// Pakai testcontainers
container, _ := nats.Run(ctx, "nats:2.12-alpine",
    nats.WithArgument("jetstream", ""),
)
```

### Skenario yang harus di-test

1. **Happy path** — pesan masuk, ack, DB ada datanya.
2. **Decode error** — payload jelek → harus Term, bukan loop.
3. **DB transient error** — mock DB error sekali, pastikan retry sukses.
4. **DB persistent error** — mock error 3× → harus Term + DLQ.
5. **Duplicate** — kirim 2× pesan sama → DB hanya 1 row (idempotency).
6. **Concurrent worker** — 3 instance worker sama, tidak ada race / double-process.
7. **Graceful shutdown** — kirim SIGTERM saat 50 pesan in-flight, pastikan
   semua ke-ack atau ke-nak (tidak ada yang di-orphan).
8. **Replay** — worker baru dengan `DeliverAll`, dapat semua history dengan urutan benar.
9. **Long processing** — handler sleep > ack_wait, pastikan `InProgress()`
   keep alive atau redelivery handle dengan benar (idempotency!).

---

## 16. Production Checklist

- [ ] Consumer **durable** (bukan ephemeral).
- [ ] `--ack=explicit`, jangan `--ack=none`/`--ack=all`.
- [ ] `--filter` di server-side, bukan filter di Go.
- [ ] `--max-pending` di-set sesuai kapasitas worker (bukan default unlimited).
- [ ] `--ack-wait` >= 2× p99 processing time, ATAU `InProgress()` periodic.
- [ ] `--max-deliver=3-5`, bukan unlimited.
- [ ] Idempotency di DB write (`ON CONFLICT`, `SET NX`, dll).
- [ ] Error handling switch by error type — Term untuk permanent, Nak untuk transient.
- [ ] DLQ pattern: cek `meta.NumDelivered`, publish ke `dlq.>` sebelum Term.
- [ ] Graceful shutdown: `cc.Stop()` lalu `nc.Drain()`.
- [ ] `defer cancel()` di context per-message (bukan reuse).
- [ ] Worker akses `Nats-Msg-Id` dari header sebagai dedup key.
- [ ] Prometheus metrics: ack/nak/term rate, processing latency, message age, lag.
- [ ] Alert: lag, redelivery rate, term rate.
- [ ] Worker bisa di-scale horizontal (banyak instance, durable name sama).
- [ ] Test coverage: happy path, decode error, transient retry, duplicate, shutdown.
- [ ] Token via env / secret manager, **bukan** hardcoded.

---

## 17. Anti-Pattern yang Sering Dilakukan

| ❌ Anti-pattern                                              | Akibat                                                                            |
| ------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| Skip ack saat error (return tanpa Ack/Nak/Term)              | Pesan stuck sampai ack_wait → redelivery → loop sampai max_deliver, latency spike |
| `Nak()` polos (tanpa delay) untuk semua error                | DB hammered, error escalate, worker thrashing                                     |
| `Ack` sebelum DB commit selesai                              | Data hilang kalau DB gagal commit setelahnya                                      |
| Ack berdasarkan in-memory state, bukan persisted             | Worker crash → data di memory hilang, tapi pesan udah di-ack                      |
| Tidak handle duplicate (asumsi exactly-once)                 | Double insert, harga rata-rata salah, volume double                               |
| Filter pakai `if` di Go, bukan `--filter` consumer           | Network waste, parse waste                                                        |
| `ack_wait` default 30s untuk processing > 30s                | Pesan ke-redeliver saat masih diproses → duplicate work                           |
| Banyak consumer berbeda untuk pesan yang sama (load balance) | **Bukan** load balance, itu **fan-out**. Pesan sama diproses 2×                   |
| Ephemeral consumer untuk worker production                   | State hilang → re-process semua / lewatkan pesan saat down                        |
| `cc.Stop()` tanpa wait → langsung `nc.Close()`               | In-flight pesan ter-orphan → redelivery 30s lag                                   |
| Unlimited `max-pending`                                      | Worker OOM saat burst                                                             |
| Term tanpa DLQ                                               | Data jelek hilang tanpa jejak, susah debug                                        |
| Tidak monitor `num_pending`                                  | Lag membesar diam-diam, baru ketauan saat bos komplain                            |

---

## 18. Quick Reference — Worker Snippet Final

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "log"
    "os/signal"
    "syscall"
    "time"

    "github.com/nats-io/nats.go/jetstream"
    "idx-stream/pkg/natsx"
)

func main() {
    nc, js, err := natsx.Connect("questdb-writer")
    if err != nil { log.Fatal(err) }
    defer nc.Drain()

    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    cons, err := js.Consumer(ctx, "STOCKS", "questdb-writer")
    if err != nil { log.Fatal(err) }

    cc, err := cons.Consume(func(msg jetstream.Msg) {
        // 1. Cek shutdown signal
        if ctx.Err() != nil {
            _ = msg.Nak()
            return
        }

        // 2. Cek redelivery count untuk DLQ
        meta, _ := msg.Metadata()
        if meta != nil && meta.NumDelivered >= 3 {
            _ = publishToDLQ(ctx, js, msg, "max delivery exceeded")
            _ = msg.Term()
            return
        }

        // 3. Decode dengan idempotency key
        msgID := msg.Headers().Get("Nats-Msg-Id")
        var t Tick
        if err := json.Unmarshal(msg.Data(), &t); err != nil {
            log.Printf("decode err: %v — term", err)
            _ = msg.Term()
            return
        }

        // 4. Process dengan timeout
        wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
        err := writeQuestDB(wctx, msgID, t)
        cancel()

        // 5. Ack/Nak/Term per error type
        switch {
        case err == nil:
            _ = msg.Ack()
        case errors.Is(err, context.Canceled):
            _ = msg.Nak()
        case errors.Is(err, context.DeadlineExceeded):
            _ = msg.NakWithDelay(10 * time.Second)
        case isUniqueViolation(err):
            _ = msg.Ack() // idempotent: sudah pernah diproses
        case isPermanentError(err):
            _ = publishToDLQ(ctx, js, msg, err.Error())
            _ = msg.Term()
        default:
            log.Printf("transient err: %v — nak with delay", err)
            _ = msg.NakWithDelay(5 * time.Second)
        }
    },
        jetstream.PullMaxMessages(500),
        jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
            log.Printf("[consume] async err: %v", err)
        }),
    )
    if err != nil { log.Fatal(err) }

    log.Println("worker running…")
    <-ctx.Done()

    // Graceful shutdown
    log.Println("shutting down…")
    shutdownCtx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer scancel()
    done := make(chan struct{})
    go func() { cc.Stop(); close(done) }()
    select {
    case <-done:
        log.Println("clean shutdown")
    case <-shutdownCtx.Done():
        log.Println("shutdown timeout — force exit")
    }
}
```

---

## 19. Cara Cek Health Consumer dari Server

```bash
# Lag (paling penting)
nats consumer info STOCKS questdb-writer --json | jq '{
    pending: .num_pending,
    inflight: .num_ack_pending,
    redelivered: .num_redelivered,
    delivered_seq: .delivered.consumer_seq,
    ack_floor_seq: .ack_floor.consumer_seq
}'

# Live monitor
watch -n 1 'nats consumer info STOCKS questdb-writer --json | jq "{pending:.num_pending,inflight:.num_ack_pending,redelivered:.num_redelivered}"'

# Subscribe ke advisory event saat max_deliver tercapai
nats sub '$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.>'

# Subscribe ke advisory event saat ack
nats sub '$JS.EVENT.ADVISORY.CONSUMER.MSG_TERMINATED.>'
```

### Indikator masalah

| Gejala                                       | Diagnosa                                                              |
| -------------------------------------------- | --------------------------------------------------------------------- |
| `num_pending` naik terus                     | Worker terlalu lambat — scale out atau optimasi                       |
| `num_redelivered` tinggi                     | Worker sering error / crash — cek log, naikkan ack_wait, atau fix bug |
| `num_ack_pending` mentok di max              | Worker stuck — cek goroutine leak, DB connection exhaustion           |
| Term events spike di advisory                | Banyak data jelek — investigasi schema mismatch                       |
| `ack_floor` tidak maju tapi `delivered` maju | Worker terima pesan tapi tidak ack — bug di handler                   |

---

## 20. Referensi Pelengkap

- `nats-jetstream-setup.md` — info VM, credential, stream config aktif
- `nats-tutorial-go.md` — tutorial publish & consume step-by-step
- `nats-publisher-notes.md` — best practices publisher
- nats.go JetStream: https://pkg.go.dev/github.com/nats-io/nats.go/jetstream
- Consumer concepts: https://docs.nats.io/nats-concepts/jetstream/consumers
- Advisory events: https://docs.nats.io/running-a-nats-service/nats_admin/jetstream_admin/advisories
