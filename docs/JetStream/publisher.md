# Catatan untuk Backend Service yang Mau Publish ke NATS JetStream

> Konteks: VM `tuai-jetstream` (10.10.8.2), stream `STOCKS`, subjects `stocks.>`.
> Audience: developer yang implement Go/Node/Python publisher service.

Bukan tutorial step-by-step — ini **hal-hal yang biasanya bikin masalah**
kalau tidak dipahami sejak awal.

---

## 1. Connection Lifecycle

### ✅ DO — buat 1 connection, share di seluruh aplikasi

```go
// init sekali di startup
nc, js, _ := natsx.Connect("idx-publisher")
defer nc.Drain()

// reuse `js` di semua handler / goroutine — TCP connection-nya thread-safe
```

### ❌ DON'T — bikin connection per request

Setiap `nats.Connect()` = TCP handshake + auth + client INFO exchange.
Kalau publisher nge-spawn koneksi per tick stock, server akan kewalahan dan
publisher Anda lambat 10–100×.

### Reconnect otomatis — selalu ON

```go
nats.MaxReconnects(-1),               // -1 = tanpa batas
nats.ReconnectWait(2*time.Second),
```

NATS server restart? Jaringan blip? Library handle automatic. Pesan yang sempat
buffered di client akan di-flush setelah reconnect (kecuali buffer overflow —
lihat poin 8).

---

## 2. Subject Design — Pikirkan Sebelum Mulai Publish

Subject design itu **kontrak antara publisher dan consumer**. Ganti subject =
breaking change.

### Pola yang baik

```
stocks.idx.<TICKER>            → stocks.idx.BBCA
stocks.idx.<TICKER>.<KIND>     → stocks.idx.BBCA.tick
                                stocks.idx.BBCA.orderbook
                                stocks.idx.BBCA.trade
```

### Kenapa hierarchy penting

Consumer bisa filter dengan wildcard:

| Filter consumer     | Match                                                    |
| ------------------- | -------------------------------------------------------- |
| `stocks.>`          | semua di bawah `stocks`                                  |
| `stocks.idx.*`      | hanya 1 segment, jadi tidak match `stocks.idx.BBCA.tick` |
| `stocks.idx.BBCA.>` | hanya BBCA, semua kind                                   |
| `stocks.idx.*.tick` | semua ticker, hanya tick                                 |

### Aturan main subject

- Pakai **lowercase**, segmen dipisah `.` — bukan `_` atau `-`.
- Jangan masukkan field yang sering berubah (timestamp, sequence) — itu di
  payload, bukan subject. Subject = identitas routing, bukan data.
- Jangan terlalu dalam (>5 level) — performa filter degrade.
- Hindari karakter spesial (`*`, `>`, spasi, `.` literal di nama segment).

---

## 3. Sync `Publish` vs `PublishAsync`

### Sync — default, simpel

```go
ack, err := js.Publish(ctx, subject, body)
// blocking sampai server konfirmasi pesan masuk stream
```

- **Throughput**: ~5K–20K msg/s tergantung RTT.
- **Pakai kalau**: < 1000 msg/s, atau Anda butuh ack per-pesan untuk decision.

### Async — pipeline ack

```go
ackF, err := js.PublishAsync(subject, body)
// langsung return, ack datang via channel
```

- **Throughput**: 100K+ msg/s.
- **Pakai kalau**: high-frequency tick (orderbook, trade), atau publisher
  punya batch/buffer di atasnya.

### ⚠️ Async punya quirk

1. **Max in-flight default = 4000**. Kalau publisher jalan lebih cepat dari
   server bisa ack, `PublishAsync` akan **block atau error**. Atur lewat:
   ```go
   js, _ := jetstream.New(nc, jetstream.WithPublishAsyncMaxPending(8000))
   ```
2. **Wajib tunggu `PublishAsyncComplete()`** sebelum process exit — kalau
   tidak, pesan di-buffer client tapi belum ke server akan hilang.
3. **Error per-pesan tidak ketauan langsung** — harus baca `ackF.Err()` channel
   atau set timeout.

### Aturan praktis

| Volume         | Pakai                                     |
| -------------- | ----------------------------------------- |
| < 100 msg/s    | Sync, pasti bener                         |
| 100–1000 msg/s | Sync + goroutine pool kalau perlu paralel |
| > 1000 msg/s   | Async + monitor pending                   |

---

## 4. Message ID — Kunci Exactly-Once

### Masalah tanpa Msg-Id

Network blip → client sudah kirim, ack hilang → library retry → server terima
2× → **stream punya 2 entry duplikat**.

Untuk data stock yang nanti agregat ke OHLC, duplikat = harga rata-rata salah,
volume double-count. **Bug yang sulit di-trace.**

### Solusi

```go
ack, _ := js.Publish(ctx, subject, body,
    jetstream.WithMsgID("BBCA-1714123456789"),
)
```

Stream punya **dedup window** (di setup kita: 2 menit). Pesan dengan Msg-Id
yang sama dalam window akan di-tolak server, ack-nya `Duplicate=true`.

### Cara bikin Msg-Id yang baik

| Strategi                     | Cocok untuk                                            |
| ---------------------------- | ------------------------------------------------------ |
| `<ticker>-<exchange-seq-id>` | Kalau exchange feed kasih sequence number              |
| `<ticker>-<timestamp-ns>`    | Kalau timestamp dari exchange resolusi tinggi (ns)     |
| `<source>-<uuid-v4>`         | Kalau publisher generate event sendiri (bukan forward) |
| `sha256(payload)[:16]`       | Kalau payload identik = pesan sama                     |

### ❌ Jangan

- Jangan pakai timestamp `time.Now()` di Go process — kalau Anda punya 2
  publisher pod, mereka generate timestamp beda untuk pesan yang sama dari upstream.
- Jangan pakai counter monotonic per-process — restart pod = counter reset.

---

## 5. Proteksi: `WithExpectStream`

```go
js.Publish(ctx, subject, body,
    jetstream.WithMsgID(id),
    jetstream.WithExpectStream("STOCKS"),
)
```

**Skenario yang dicegah:** seseorang accidentally hapus stream `STOCKS`.
Tanpa `ExpectStream`, publish ke `stocks.idx.BBCA` akan **diam-diam masuk
ke stream lain** kalau ada yang capture subject itu (atau dianggap NATS Core
fire-and-forget message yang langsung hilang).

Dengan `ExpectStream`, publish gagal eksplisit dengan error → publisher tahu.

Opsi lain yang sejenis:

- `WithExpectLastSequence(n)` — atomic compare-and-swap, advanced.
- `WithExpectLastMsgID(id)` — ordering check.

---

## 6. Context & Timeout

### Setiap publish wajib punya timeout

```go
pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
ack, err := js.Publish(pubCtx, subject, body)
cancel()
```

### Kenapa

- Kalau server lemot/freeze, **publisher akan stuck infinite**.
- Stuck = stock feed dari upstream menumpuk di memory publisher → OOM.
- Lebih baik timeout cepat → masuk error path → metrics naik → alert nyala.

### Timeout berapa?

| Pesan                            | Timeout    |
| -------------------------------- | ---------- |
| Tick / orderbook (loss-tolerant) | 1–2 detik  |
| Order placement (critical)       | 5–10 detik |
| Bulk import (batch)              | 30 detik+  |

Jangan pakai `context.Background()` untuk publish.

---

## 7. Payload — Format & Size

### Format: pilih satu, konsisten

| Format          | Pro                         | Contra                                              |
| --------------- | --------------------------- | --------------------------------------------------- |
| **JSON**        | Human-readable, mudah debug | Verbose (~3× lebih besar dari binary), parse lambat |
| **Protobuf**    | Compact, fast, schema       | Harus generate code, debug perlu tool               |
| **MessagePack** | Compact, dynamic            | Less ecosystem                                      |

Untuk stock realtime: **kalau >5K msg/s, switch ke protobuf**. JSON cukup
untuk awal.

### Size limit di setup kita

| Limit                  | Nilai        |
| ---------------------- | ------------ |
| Server `max_payload`   | 8 MB         |
| Stream `max_msg_size`  | 1 MB         |
| Realistic tick payload | 100–500 byte |

Kalau pesan > 1 MB, masuk error path. Pertimbangkan:

- Compress (gzip / zstd) — turunkan 5–10× untuk JSON.
- Split ke beberapa pesan dengan correlation ID.
- Object Store (untuk file besar / snapshot).

### Headers

Pakai header NATS untuk metadata routing/processing tanpa perlu parse payload:

```go
import "github.com/nats-io/nats.go"

msg := &nats.Msg{
    Subject: "stocks.idx.BBCA",
    Header:  nats.Header{},
    Data:    body,
}
msg.Header.Set("X-Source", "idx-feed-primary")
msg.Header.Set("X-Schema-Version", "v2")
msg.Header.Set("Nats-Msg-Id", msgID) // setara WithMsgID

js.PublishMsg(ctx, msg)
```

Berguna untuk:

- Versioning schema (consumer cek `X-Schema-Version`).
- Tracing (`X-Trace-Id` dari OpenTelemetry).
- Source attribution (multi-publisher).

---

## 8. Backpressure — Apa yang Terjadi Saat Server Lebih Lambat

Skenario: publisher generate 10K tick/detik, JetStream disk write hanya
muat 8K/detik. Sisanya?

### NATS Core (non-JetStream)

Pesan **dibuang**. Tanpa peringatan. Subscription Anda gak akan dapat semua.

### JetStream Sync `Publish`

Caller ter-block → backpressure naik sampai ke source. Ini **yang Anda mau**
biasanya — lebih baik publisher tertahan daripada data hilang.

### JetStream `PublishAsync`

In-flight queue penuh → `PublishAsync()` return error `ErrTooManyStalledMsgs`
atau block (tergantung config). Handle error ini eksplisit.

### Strategi handling

```go
ackF, err := js.PublishAsync(subject, body)
if err != nil {
    // Buffer penuh — pilih satu:
    // 1. Drop message + log + metric (loss-tolerant data)
    metrics.IncrCounter("publish.dropped", 1)
    return
    // 2. Block sampai ada slot
    // <-js.PublishAsyncComplete()
    // 3. Spill ke local disk queue, retry nanti
}
```

### Monitor backpressure

```bash
# Cek pending publish di server side
curl -s http://10.10.8.2:8222/jsz | jq '.streams[] | {name, num_subjects, messages, bytes}'

# Cek lag client connection
curl -s http://10.10.8.2:8222/connz | jq '.connections[] | {name, pending_bytes, in_msgs, out_msgs, slow_consumer}'
```

`slow_consumer: true` = client ketinggalan, server akan disconnect kalau parah.

---

## 9. Graceful Shutdown — Jangan Lupa `Drain()`

```go
func main() {
    nc, js, _ := natsx.Connect("idx-publisher")
    defer nc.Drain()  // ← INI WAJIB

    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // … publish loop …
    <-ctx.Done()
    // saat sampai sini, defer akan jalankan Drain() → flush buffer → close
}
```

### `Drain()` vs `Close()`

| Method       | Behavior                                                               |
| ------------ | ---------------------------------------------------------------------- |
| `nc.Close()` | Tutup TCP **paksa**. Pesan di buffer hilang.                           |
| `nc.Drain()` | Stop subscriptions baru, flush pending publish, **lalu** close. Async. |

Untuk publisher yang masih ada pesan in-flight saat container di-stop
(Kubernetes SIGTERM, dll.), `Drain()` mencegah data loss.

### Async + Drain

Kalau pakai `PublishAsync`, **wajib** tunggu `PublishAsyncComplete()` SEBELUM
`Drain()`:

```go
select {
case <-js.PublishAsyncComplete():
case <-time.After(10 * time.Second):
    log.Println("warning: ack pending masih ada saat shutdown")
}
nc.Drain()
```

---

## 10. Error Handling — Yang Perlu Anda Tangani

### Error path yang umum

```go
ack, err := js.Publish(ctx, subject, body, jetstream.WithMsgID(id))
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        // server lambat / network — retry dengan backoff
    case errors.Is(err, jetstream.ErrNoStreamResponse):
        // tidak ada stream yang capture subject ini
    case errors.Is(err, nats.ErrMaxPayload):
        // pesan terlalu besar — split / compress
    case errors.Is(err, nats.ErrConnectionClosed):
        // koneksi tertutup — biasanya sedang reconnecting
    default:
        log.Printf("publish err: %v", err)
    }
    return
}

if ack.Duplicate {
    // bukan error, tapi info: pesan dengan Msg-Id ini sudah ada.
    // Biasanya bisa di-ignore.
}
```

### Retry strategy

Jangan retry blind dengan loop ketat. Pakai **exponential backoff + jitter**:

```go
delays := []time.Duration{100*time.Millisecond, 500*time.Millisecond, 2*time.Second}
for i, d := range delays {
    err := publishOnce()
    if err == nil { return nil }
    if i == len(delays)-1 { return err }
    time.Sleep(d + time.Duration(rand.Intn(100))*time.Millisecond)
}
```

Pasti pakai `WithMsgID` kalau retry — supaya kalau ternyata pesan sebelumnya
sudah masuk, dedup yang handle.

---

## 11. Observability — Yang Wajib Di-instrument

Minimal metrics yang publisher harus expose (Prometheus):

```
# Counter
nats_publish_total{subject, status="ok|duplicate|error"}
nats_publish_bytes_total{subject}

# Histogram
nats_publish_duration_seconds{subject}

# Gauge
nats_publish_pending{}                # untuk PublishAsync
nats_connection_status{} 0|1
nats_connection_reconnects_total{}
```

Tambah callback connection event:

```go
nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
    metrics.SetGauge("nats.connection.status", 0)
})
nats.ReconnectHandler(func(_ *nats.Conn) {
    metrics.SetGauge("nats.connection.status", 1)
    metrics.IncrCounter("nats.reconnects", 1)
})
```

### Alert minimum

| Alert              | Kondisi                                   |
| ------------------ | ----------------------------------------- |
| Publisher down     | `nats_connection_status == 0` for 1m      |
| Publish error rate | rate(`status="error"`) > 1% for 5m        |
| Slow publish       | p99 `nats_publish_duration_seconds` > 1s  |
| Stream lag         | `jsz` `messages` naik tapi consumer tidak |

---

## 12. Testing Locally

Jangan test pakai prod stream. Bikin stream ephemeral untuk test:

```bash
# di lokal — jalankan nats-server -js untuk test cepat
nats-server -js -sd /tmp/jetstream &

# di test code, connect ke localhost:4222 tanpa auth
```

Atau pakai `testcontainers-go` untuk integration test:

```go
import "github.com/testcontainers/testcontainers-go/modules/nats"

container, _ := nats.Run(ctx, "nats:2.12-alpine",
    nats.WithArgument("jetstream", ""),
)
```

### Yang perlu di-test sebelum deploy

1. **Reconnect** — kill server di tengah publish, pastikan publisher resume.
2. **Dedup** — publish 2× dengan Msg-Id sama, pastikan stream count tetap 1.
3. **Backpressure** — generate 10× rate normal, pastikan tidak OOM.
4. **Shutdown** — kirim SIGTERM saat 1000 pesan in-flight, pastikan semua ke-drain.

---

## 13. Checklist Sebelum Production

- [ ] 1 koneksi NATS, share di seluruh app — bukan per-request.
- [ ] `MaxReconnects(-1)` — auto-reconnect tanpa batas.
- [ ] Subject design jelas, dokumentasikan di shared schema repo.
- [ ] `WithMsgID` di setiap publish — exactly-once.
- [ ] `WithExpectStream("STOCKS")` — proteksi stream salah.
- [ ] `context.WithTimeout` di setiap publish — bukan `Background()`.
- [ ] Token di env / secret manager — bukan hardcoded di repo.
- [ ] Async publish (kalau >1K msg/s) dengan `PublishAsyncComplete()` di shutdown.
- [ ] `defer nc.Drain()` di main.
- [ ] Handle `ctx.DeadlineExceeded`, `ErrMaxPayload`, `ErrConnectionClosed` eksplisit.
- [ ] Exponential backoff retry untuk error transient.
- [ ] Prometheus metrics: publish rate, error rate, latency, connection status.
- [ ] Alert: connection down, error rate, p99 latency, stream lag.
- [ ] Integration test cover: reconnect, dedup, backpressure, shutdown.
- [ ] Payload format terdokumentasi (JSON schema atau protobuf .proto).
- [ ] Schema versioning via header `X-Schema-Version`.

---

## 14. Anti-Pattern yang Sering Dilakukan

| ❌ Anti-pattern                    | Akibat                                                  |
| ---------------------------------- | ------------------------------------------------------- |
| Connect per-request                | TCP overhead, server overload                           |
| Tanpa Msg-Id, retry on error       | Duplikat di stream, agregat salah                       |
| `context.Background()` di Publish  | Stuck infinite saat server freeze                       |
| Publish dari `init()`              | Tidak ada error handling, app crash diam                |
| Ignore `ack.Duplicate` di retry    | Log spam, panic di parser                               |
| Subject pakai user-input           | Subject injection (subscriber lain dapat data sensitif) |
| `Close()` tanpa `Drain()`          | Data loss saat shutdown                                 |
| Tidak monitor `slow_consumer`      | Server force-disconnect tanpa warning                   |
| Pakai NATS Core untuk data penting | Pesan hilang saat tidak ada subscriber                  |
| Hardcode token di repo             | Bocor → akses penuh untuk siapapun yang clone           |

---

## 15. Quick Reference — Connection Snippet Final

```go
// Versi production-ready, copy dari sini
nc, err := nats.Connect(
    os.Getenv("NATS_URL"),
    nats.Token(os.Getenv("NATS_TOKEN")),
    nats.Name("idx-publisher-"+hostname),
    nats.MaxReconnects(-1),
    nats.ReconnectWait(2*time.Second),
    nats.ReconnectJitter(500*time.Millisecond, 2*time.Second),
    nats.Timeout(5*time.Second),
    nats.PingInterval(20*time.Second),
    nats.MaxPingsOutstanding(3),
    nats.DisconnectErrHandler(onDisconnect),
    nats.ReconnectHandler(onReconnect),
    nats.ClosedHandler(onClosed),
    nats.ErrorHandler(onAsyncErr),
)
if err != nil { return err }

js, err := jetstream.New(nc,
    jetstream.WithPublishAsyncMaxPending(4000),
    jetstream.WithPublishAsyncErrHandler(onPublishAsyncErr),
)
if err != nil { nc.Close(); return err }

// publish:
ackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
ack, err := js.Publish(ackCtx, "stocks.idx."+ticker, body,
    jetstream.WithMsgID(msgID),
    jetstream.WithExpectStream("STOCKS"),
    jetstream.WithRetryAttempts(3),       // retry built-in untuk transient
    jetstream.WithRetryWait(200*time.Millisecond),
)
cancel()
```

---

## 16. Referensi Pelengkap

- `nats-jetstream-setup.md` — info VM, credential, stream config aktif
- `nats-tutorial-go.md` — tutorial step-by-step
- nats.go docs: https://pkg.go.dev/github.com/nats-io/nats.go
- Subject best practices: https://docs.nats.io/nats-concepts/subjects
- Publishing concepts: https://docs.nats.io/using-nats/developer/publishing
