# IDX Data Feed — Flow Lengkap dari IQPlus ke Storage

Dokumen ini menjelaskan alur data realtime dari **IQPlus Data Feed Service v4.0.0** sampai ke layer penyimpanan akhir, tier per tier. Cocok dipakai sebagai referensi saat development dan untuk onboarding tim.

---

## Daftar Isi

1. [Overview Pipeline](#1-overview-pipeline)
2. [Tier 0 — IQPlus ke FreeBSD VM](#2-tier-0--iqplus-ke-freebsd-vm)
3. [Tier 1 — Go Publisher di FreeBSD](#3-tier-1--go-publisher-di-freebsd)
4. [Tier 2 — NATS JetStream Subjects](#4-tier-2--nats-jetstream-subjects)
5. [Tier 3 — Consumer per Record Type](#5-tier-3--consumer-per-record-type)
6. [Decision Matrix — Apa Disimpan Di Mana](#6-decision-matrix--apa-disimpan-di-mana)
7. [Catatan Praktis](#7-catatan-praktis)

---

## 1. Overview Pipeline

```
┌────────────────────────────────────────────────────────────────┐
│ TIER 0  Vendor                                                  │
│   IQPlus pusat (server vendor, di luar infrastruktur kita)      │
└────────────────────────────────────────────────────────────────┘
                          │
                          │ TCP push, ASCII pipe-delimited, CRLF
                          ▼
┌────────────────────────────────────────────────────────────────┐
│ TIER 1  FreeBSD VM (1 host, 2 proses)                           │
│   ┌──────────────────────┐    ┌─────────────────────────────┐   │
│   │ Daemon vendor IQPlus │ →  │ Go ingestion publisher      │   │
│   │ (di-setup vendor)    │    │ (kita yg develop)           │   │
│   └──────────────────────┘    └─────────────────────────────┘   │
│            ▲ outbound TCP              │                        │
│            │ ke server IQPlus          │ publish per record     │
└────────────┼──────────────────────────┼────────────────────────┘
             │                          │
             │ (TBD ke vendor)          ▼
                                ┌──────────────────────────┐
                                │ NATS JetStream           │
                                │ idx.{type}.{stockcode}   │
                                └──────────────────────────┘
                                          │
            ┌─────────────────┬───────────┼───────────────┬──────────────┐
            ▼                 ▼           ▼               ▼              ▼
       ┌─────────┐      ┌──────────┐ ┌──────────┐  ┌────────────┐  ┌──────────┐
       │ Redis   │      │ QuestDB  │ │ MongoDB  │  │ PostgreSQL │  │ Temporal │
       │ hot     │      │ tick &   │ │ news,    │  │ user, audit│  │ alert    │
       │ state   │      │ OHLCV    │ │ metadata │  │            │  │ workflow │
       └─────────┘      └──────────┘ └──────────┘  └────────────┘  └──────────┘
```

Prinsip umum: **satu byte mentah dari IQPlus akan berakhir di lebih dari satu storage**, sesuai kebutuhan akses (live, historical, transaksional, atau alerting).

---

## 2. Tier 0 — IQPlus ke FreeBSD VM

### 2.1 Apa yang terjadi

Di FreeBSD VM yang Anda siapkan, vendor IQPlus akan setup **daemon proprietary** mereka. Daemon ini bertanggung jawab:

- Connect (atau diconnect, tergantung skenario yang dikonfirmasi vendor) ke server pusat IQPlus
- Login dengan credential akun Anda
- Maintain stateful TCP connection sepanjang jam trading
- Handle reconnect, sequence tracking, dan optional resend
- Expose hasil stream ke aplikasi lokal di server Anda

### 2.2 Tiga skenario arah koneksi (perlu dikonfirmasi ke vendor)

**Skenario A — Daemon outbound ke IQPlus pusat (paling umum di industri)**

```
[IQPlus pusat] <—outbound TCP— [daemon di FreeBSD VM]
```

Firewall: hanya butuh outbound rule ke IP/port IQPlus.

**Skenario B — IQPlus pusat inbound ke FreeBSD VM**

```
[IQPlus pusat] —inbound TCP—> [daemon listen di FreeBSD VM]
```

Firewall: butuh public IP dan inbound rule allow IP IQPlus saja.

**Skenario C — Protokol proprietary (multicast UDP, TIBCO, dll)**

Tidak relevan dari sisi aplikasi kita. Yang penting daemon expose hasilnya ke localhost.

### 2.3 Format byte di kabel

Format ini **sama** baik di mode demo public IP maupun di mode produksi via daemon (perlu dikonfirmasi vendor, tapi sangat probable):

```
IQP|YYYYMMDD|HHMMSS|<sequence>|<record_type>|<data>\r\n
```

- Setiap record diakhiri **CRLF** (byte `\r\n`, bukan cuma `\n`)
- Field dipisah pipe `|`
- Sub-field di dalam `data` dipisah semicolon `;` (untuk record dengan FID)
- Sequence number reset ke `1` setiap awal hari trading

Contoh nyata:

```
IQP|20211223|085900|69397|15|WIKA|20211208|085900|1|0|1225|200|--|D|--|D|48941|34504\r\n
```

Artinya: trade WIKA jam 08:59:00, sequence #69397, harga 1225, volume 200 lot, buyer order 48941, seller order 34504.

### 2.4 Login (kalau aplikasi Go connect langsung ke server IQPlus, bukan daemon lokal)

```
Request:  IQP|149|0|1|<username>|<MD5(password)>\r\n
Response: IQP|149|0|<status_code>|<message>\r\n
```

Status code `0` = OK. Selain itu, koneksi ditutup. Lihat spec halaman 10 untuk daftar lengkap error code.

> Catatan penting: kalau daemon vendor jalan di FreeBSD VM Anda, **kemungkinan besar aplikasi Go Anda tidak perlu login lagi** — cukup connect ke localhost daemon. Tapi ini **wajib dikonfirmasi** ke vendor.

---

## 3. Tier 1 — Go Publisher di FreeBSD

### 3.1 Tanggung jawab

Aplikasi Go di FreeBSD VM hanya punya **satu pekerjaan**: ambil byte dari daemon lokal, parse jadi struct, publish ke NATS JetStream. Tidak ada business logic, tidak ada state, tidak ada query.

Filosofi ini penting: kalau ada bug di logic OHLCV atau alert, recovery cuma butuh restart consumer downstream. Tapi kalau bug di publisher, Anda bisa kehilangan data **permanently** karena IQPlus tidak buffer untuk klien yang offline.

### 3.2 Connection ke daemon

```go
// Pseudocode
conn, err := net.Dial("tcp", "127.0.0.1:5555")  // port daemon, dikonfirmasi vendor
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

Aplikasi Go adalah **klien** (dial), bukan server (listen). Daemon yang listen di localhost.

### 3.3 Loop baca + parse + publish

```go
scanner := bufio.NewScanner(conn)
scanner.Buffer(make([]byte, 64*1024), 1024*1024)  // record bisa besar (news Type 36)

for scanner.Scan() {
    line := scanner.Bytes()

    record, err := parseRecord(line)
    if err != nil {
        metrics.ParseErrors.Inc()
        continue
    }

    subject := buildSubject(record)  // mis. "idx.trade.BBRI"
    payload := serialize(record)     // MessagePack atau JSON

    js.Publish(subject, payload)     // ke NATS JetStream
    metrics.MessagesPublished.WithLabelValues(record.Type).Inc()
}
```

### 3.4 Batching untuk throughput

Sustained throughput target 20-40rb msg/sec, burst 120rb msg/sec. Publish per-message ke NATS akan jadi bottleneck karena network roundtrip per call.

**Solusi:** batch 100-500 msg sebelum publish, atau timeout 50ms (mana yang duluan tercapai).

```go
// Sederhana: pakai NATS async publisher
js.PublishAsync(subject, payload)
// ... terus publish ...
<-js.PublishAsyncComplete()  // wait sampai semua acked
```

### 3.5 Yang HARUS ada di publisher

- **Reconnect logic** ke daemon dengan exponential backoff (1s → 2s → 4s → 8s, capped di 30s)
- **Metrics Prometheus** di port localhost: `iqplus_messages_received_total{type=...}`, `iqplus_last_message_timestamp`, `iqplus_publish_errors_total`, `iqplus_connection_state`
- **Health endpoint** `/health` untuk liveness check
- **Process supervisor** (daemontools/runit) untuk auto-restart kalau crash
- **Graceful shutdown** saat SIGTERM — flush batch terakhir sebelum exit

### 3.6 Yang TIDAK boleh ada di publisher

- Logic OHLCV aggregation
- Direct write ke Redis/Postgres/MongoDB
- HTTP API endpoint untuk client
- Alert evaluation
- Caching atau in-memory state berlapis

Semua itu adalah tanggung jawab consumer di Tier 3.

---

## 4. Tier 2 — NATS JetStream Subjects

### 4.1 Subject naming convention

Format: `idx.<kategori>.<identifier>`

Pemilihan kategori dan identifier mempengaruhi efisiensi subscriber filter — design ini harus stable dari awal karena ganti subject di production berarti consumer harus re-subscribe semua.

### 4.2 Mapping record type ke subject

| Type | Nama                               | Subject Pattern                | Contoh                           |
| ---- | ---------------------------------- | ------------------------------ | -------------------------------- |
| 13   | Control Messages                   | `idx.status.feed`              | `idx.status.feed`                |
| 14   | Quote (saham)                      | `idx.quote.<stockcode>`        | `idx.quote.BBRI`                 |
| 14   | Quote (regional/komoditi/currency) | `idx.quote.regional.<symbol>`  | `idx.quote.regional.FTSE`        |
| 15   | Trade                              | `idx.trade.<stockcode>`        | `idx.trade.BBRI`                 |
| 16   | Order                              | `idx.order.<stockcode>`        | `idx.order.BBRI`                 |
| 17   | Top 20                             | `idx.top20.<category_code>`    | `idx.top20.0` (top 20 volume RG) |
| 18   | Best Quote                         | `idx.bestquote.<stockcode>`    | `idx.bestquote.BBRI`             |
| 26   | Resend Order                       | `idx.resend.order.<stockcode>` | `idx.resend.order.BBRI`          |
| 27   | Resend Trade                       | `idx.resend.trade.<stockcode>` | `idx.resend.trade.BBRI`          |
| 36   | News                               | `idx.news.<category>`          | `idx.news.BIS`                   |
| 39   | Activity                           | `idx.activity.market`          | `idx.activity.market`            |
| 40   | Trade Done                         | `idx.tradedone.<stockcode>`    | `idx.tradedone.BBRI`             |
| 57   | Trading Status                     | `idx.status.session`           | `idx.status.session`             |
| 58   | NBS Stock                          | `idx.nbs.stock.<stockcode>`    | `idx.nbs.stock.BBYB`             |
| 59   | NBS Broker                         | `idx.nbs.broker.<brokercode>`  | `idx.nbs.broker.PD`              |
| 130  | Trading Summary                    | `idx.summary.<stype>.<board>`  | `idx.summary.0.RG`               |

### 4.3 Stream grouping

Subject di-group ke stream berdasarkan retention policy dan SLA-nya. Beda kelas data, beda stream — supaya retention news 7 hari tidak campur dengan trade tick yang cuma butuh 24 jam.

```yaml
stream: IDX_TICK
  subjects: ["idx.trade.>", "idx.order.>", "idx.tradedone.>", "idx.resend.>"]
  retention: 24h
  storage: file
  replicas: 3

stream: IDX_QUOTE
  subjects: ["idx.quote.>", "idx.bestquote.>"]
  retention: 12h
  storage: file
  replicas: 3

stream: IDX_META
  subjects: ["idx.status.>", "idx.activity.>", "idx.summary.>",
             "idx.top20.>", "idx.nbs.>"]
  retention: 24h
  storage: file
  replicas: 3

stream: IDX_NEWS
  subjects: ["idx.news.>"]
  retention: 7d
  storage: file
  replicas: 3
```

### 4.4 Consumer model

Setiap consumer subscribe ke pola yang dibutuhkan. NATS JetStream akan deliver subset record yang match — tidak perlu subscribe semua lalu filter di aplikasi.

```go
// OHLCV consumer butuh trade saja
sub, _ := js.Subscribe("idx.trade.>", handler, nats.Durable("ohlcv-worker"))

// Live state updater butuh quote + bestquote
sub, _ := js.Subscribe("idx.quote.>", handler1, nats.Durable("state-worker"))
sub, _ := js.Subscribe("idx.bestquote.>", handler2, nats.Durable("state-worker"))
```

`Durable` name penting — kalau consumer crash, JetStream akan resume dari posisi terakhir saat consumer reconnect (bukan dari awal stream).

---

## 5. Tier 3 — Consumer per Record Type

Bagian ini detail per consumer: apa yang dia subscribe, apa yang dia hasilkan, dan kemana hasilnya disimpan.

### 5.1 Consumer: OHLCV Aggregator

**Tujuan:** generate candle multi-timeframe untuk chart trader.

| Aspek                  | Detail                                                          |
| ---------------------- | --------------------------------------------------------------- |
| Subscribe              | `idx.trade.>` (JetStream)                                       |
| Source data            | Type 15 (Trade)                                                 |
| Logic                  | Per tick → update bucket bar saat ini, satu per timeframe paralel (config: `OHLCV_TIMEFRAMES`, default `1m,5m,15m,1h,4h`) |
| Output (live state)    | Redis hash `candle:<stock>:<timeframe>` (HSET on every Update)  |
| Output (broadcast)     | NATS core subject `idx.candle.<stock>.<timeframe>` JSON payload (when `ENABLE_CANDLE_PUBLISHER=true`) — consumed by `cmd/ws-gateway` |
| Output (raw tick)      | QuestDB `trades` (one row per tick — basis untuk MAT VIEW historical) |
| Output (historical)    | QuestDB MAT VIEWs: `candles_1m`, `candles_5m`, `candles_15m`, `candles_1h`, `candles_4h` (lihat `docs/infra/questdb-mat-views.sql`) |
| Latency target         | <50ms tick masuk → Redis ter-update + NATS publish              |

**Tiga jalur output dari aggregator:**

1. **Redis** = "current bar yang masih jalan" — source of truth untuk WS snapshot on subscribe.
2. **NATS core** = realtime broadcast — `cmd/ws-gateway` subscribe wildcard `idx.candle.>` lalu fan-out ke browser. Best-effort, no JetStream, no durability — kalau publish gagal, log warn dan continue.
3. **QuestDB `trades`** = durable raw history. Historical chart query dilayani lewat MAT VIEW `candles_<tf>` (sub-100ms latency vs. recompute SAMPLE BY).

Daily OHLCV ke Postgres (`stock.prices_daily`) bukan dari aggregator langsung — di-batch oleh `cmd/daily-ingest` post-close (default 17:00 WIB) yang baca `trades` dengan SAMPLE BY 1d ALIGN TO Asia/Jakarta dan UPSERT ke Postgres. Detail di [§5.6 Daily Ingest](#56-daily-ingest-questdb--postgres).

### 5.2 Consumer: Quote State Updater

**Tujuan:** maintain "last known state" tiap saham untuk quick lookup tanpa scan tick history.

| Aspek               | Detail                                                                   |
| ------------------- | ------------------------------------------------------------------------ |
| Subscribe           | `idx.quote.>`, `idx.bestquote.>`                                         |
| Source data         | Type 14 (Quote, ~80 FID), Type 18 (Best Quote)                           |
| Logic               | Incremental merge FID baru ke state lama (Quote bersifat partial update) |
| Output              | Redis hash `quote:<stock>` dengan semua 80+ FID                          |
| Output (best quote) | Redis hash `bestquote:<stock>:bid` dan `bestquote:<stock>:ask`           |
| Reset               | Tiap pre-open (~08:45) — flush semua key, mulai ulang                    |

**Why Redis bukan QuestDB:**

Quote update bersifat **incremental** — vendor kirim FID 56=last_price, lalu kemudian FID 24=bid_price, lalu FID 39=offer_price, terpisah-pisah. Untuk merge incremental ini, pattern yang efisien adalah hash atomic update di Redis pakai Lua script:

```lua
-- KEYS[1] = "quote:BBRI"
-- ARGV = key-value pairs: "56" "5400" "24" "5395" ...
for i=1, #ARGV, 2 do
  redis.call('HSET', KEYS[1], ARGV[i], ARGV[i+1])
end
```

QuestDB tidak ideal untuk pattern ini karena dia time-series (immutable insert), bukan key-value mutable.

### 5.3 Consumer: Trade Persistence (untuk historical & analytics)

**Tujuan:** simpan setiap trade tick untuk query historical, OHLCV reconstruction, dan analytics post-trading.

| Aspek       | Detail                                                            |
| ----------- | ----------------------------------------------------------------- |
| Subscribe   | `idx.trade.>`, `idx.resend.trade.>`                               |
| Source data | Type 15 (Trade), Type 27 (Resend Trade)                           |
| Logic       | Batch 5000 row atau 1 detik → insert via ILP                      |
| Output      | QuestDB table `trades`                                            |
| Retention   | 90 hari hot (NVMe), >90 hari archive ke Parquet di object storage |

**Schema QuestDB:**

```sql
CREATE TABLE trades (
    timestamp        TIMESTAMP,                    -- designated; UTC (WIB→UTC at consumer)
    stock            SYMBOL CAPACITY 2048 INDEX,
    market           SYMBOL CAPACITY 8,            -- RG/TN/NG; stamped by consumer config (Type 15 doesn't carry it)
    command          LONG,                         -- 0=matched, 1=withdrawn (from Type 15 TradeCommand)
    price            DOUBLE,
    volume           LONG,
    buyer            SYMBOL CAPACITY 256,          -- "--" saat live, real broker code saat resend
    seller           SYMBOL CAPACITY 256,
    buyer_type       SYMBOL CAPACITY 8,            -- F=foreign, D=domestic
    seller_type      SYMBOL CAPACITY 8,
    buyer_order_no   LONG,
    seller_order_no  LONG,
    trade_no         LONG
) timestamp(timestamp)
PARTITION BY HOUR
WAL
DEDUP UPSERT KEYS(timestamp, stock, trade_no);
```

**Catatan dedup key**: `(timestamp, stock, trade_no)` — bukan order_no. `trade_no` adalah identifier unik per matched trade dari IDX yang konsisten antara Type 15 (live) dan Type 27 (resend). Pakai `order_no` justru bermasalah untuk auction trades / single-sided trades di mana salah satu order_no = 0 (mengakibatkan resend gagal overwrite live row).

**Field-level mapping vs IQPlus Type 15** (10 wire fields → 12 columns + designated ts):

| Type 15 wire field    | QuestDB column          |
| --------------------- | ----------------------- |
| Code                  | `stock`                 |
| Date + Time           | `timestamp` (designated; UTC) |
| Trade number          | `trade_no`              |
| Trade command (0/1)   | `command`               |
| Price                 | `price`                 |
| Volume                | `volume`                |
| Buyer / Buyer type    | `buyer` / `buyer_type`  |
| Seller / Seller type  | `seller` / `seller_type`|
| Buyer/Seller order num| `buyer_order_no` / `seller_order_no` |
| _(not in wire format)_| `market` — set externally by consumer config (`IDX_MARKET` env) |

**`DEDUP UPSERT KEYS`** ini penting — saat post-close (15:00-16:15) Type 27 datang dengan broker code real, row yang sama akan ter-overwrite. Ini menggantikan pattern "ReplacingMergeTree" yang ada di dokumen topologi (yang sebenarnya istilah ClickHouse, bukan QuestDB).

**Migration dari schema lama:**

Karena dedup key berubah (dari `order_no` ke `trade_no`) dan ada kolom baru (`market`, `command`), preferred migration: **drop & recreate**. Kalau retain old data:

```sql
-- (1) tambah kolom; (2) tidak bisa change dedup key di QuestDB tanpa rebuild —
-- harus drop+recreate. Untuk lab/dev:
DROP TABLE trades;
-- ...lalu run CREATE TABLE di atas.
```

### 5.4 Consumer: Order Book Reconstruction

**Tujuan:** maintain orderbook level-2 dari sequence Type 16 events.

| Aspek          | Detail                                                                             |
| -------------- | ---------------------------------------------------------------------------------- |
| Subscribe      | `idx.order.>`, `idx.resend.order.>`                                                |
| Source data    | Type 16 (Order), Type 26 (Resend Order)                                            |
| Logic          | Process bid/offer/cancel events → update sorted set per side                       |
| Output         | Redis sorted set `orderbook:<stock>:bid` (score=price) dan `orderbook:<stock>:ask` |
| Latency target | <100ms event masuk → Redis ter-update                                              |

**Catatan:**

- Order command `0=Bid`, `1=Offer`, `2=Cancel Bid`, `3=Cancel Offer`
- Field `Broker` saat live di RG board akan blank (`--`) karena Type 16 tidak include broker code — ini **regulasi BEI**, baru muncul di Type 27 setelah market close
- Top-10 level dari sorted set di-snapshot ke Redis hash terpisah `orderbook:<stock>:top10` untuk query cepat (full sorted set bisa puluhan ribu entry)

**Persist ke QuestDB juga?** Tergantung use case. Kalau Anda butuh historical orderbook (replay backtesting), ya — table `order_events` dengan partition by hour. Kalau cuma butuh current state, Redis cukup.

### 5.5 Consumer: News Indexer

**Tujuan:** simpan news searchable dengan full-text untuk dashboard berita.

| Aspek       | Detail                                                  |
| ----------- | ------------------------------------------------------- |
| Subscribe   | `idx.news.>`                                            |
| Source data | Type 36 (News)                                          |
| Logic       | Reassemble multi-packet (1 KB per packet) → simpan utuh |
| Output      | MongoDB collection `news` dengan text index             |
| Retention   | Permanent (atau sesuai kebijakan retensi konten)        |

**Kenapa MongoDB bukan PostgreSQL/QuestDB:**

- News punya field text panjang (ribuan karakter) — JSON flexible schema cocok
- Full-text search native (`$text` operator dengan stemming)
- Multi-packet reassembly cocok dengan dokument-oriented model
- Volume kecil dibanding tick data, jadi storage efficiency bukan concern utama

**Schema MongoDB:**

```javascript
{
  _id: "1640260352855278",          // news_id dari vendor
  date: ISODate("2021-12-23"),
  time: "18:53:06",
  category: "BIS",
  company_id: "TLKM",
  headline: "TLKM Laba Rp 18,9 T...",
  story: "<full reassembled text>",
  packets_received: 4,
  inserted_at: ISODate(...)
}
```

Plus text index: `db.news.createIndex({ headline: "text", story: "text" })`.

**Multi-packet handling:**

Type 36 di-deliver dalam beberapa packet (1 KB each). Field `Num_packet` total, `Current_packet` urutan. Consumer harus buffer di memory (atau Redis dengan TTL) sampai semua packet diterima, baru insert ke MongoDB.

```python
# Pseudocode
buffer = redis.get(f"news_buffer:{news_id}")
buffer[current_packet] = story_chunk
if len(buffer) == num_packet:
    full_story = "".join(buffer[i] for i in sorted(buffer))
    mongodb.news.insert({...full_story...})
    redis.delete(f"news_buffer:{news_id}")
```

### 5.6 Consumer: Top 20 Snapshot

**Tujuan:** tampilkan ranking saham/broker untuk dashboard.

| Aspek          | Detail                                                         |
| -------------- | -------------------------------------------------------------- |
| Subscribe      | `idx.top20.>`                                                  |
| Source data    | Type 17 (Top 20) — 17 kategori berbeda                         |
| Logic          | Replace snapshot terakhir per kategori (overwrite, not append) |
| Output         | Redis hash `top20:<category>` dengan list 20 simbol            |
| Output (audit) | MongoDB `top20_snapshots` dengan timestamp untuk historical    |

Top 20 datang sebagai **snapshot lengkap** (semua 20 simbol dalam satu record), bukan incremental. Setiap update menggantikan snapshot lama.

Kategori 14, 15, 16 (top 20 broker) dikirim ~2 jam setelah market close — tidak realtime sepanjang sesi.

### 5.7 Consumer: NBS (Net Buy Sell) Aggregator

**Tujuan:** analytics bandarmologi — pola net flow per broker per saham.

| Aspek               | Detail                                                                    |
| ------------------- | ------------------------------------------------------------------------- |
| Subscribe           | `idx.nbs.stock.>`, `idx.nbs.broker.>`                                     |
| Source data         | Type 58 (NBS Stock), Type 59 (NBS Broker) — **butuh permission**          |
| Logic               | Replace running total per (stock, broker) atau (broker, stock)            |
| Output (live)       | Redis hash `nbs:stock:<stock>:<broker>` dan `nbs:broker:<broker>:<stock>` |
| Output (historical) | QuestDB table `nbs_daily` snapshot di end-of-day                          |

Type 58 dan 59 sebenarnya redundant — informasi sama, beda urutan key. Pilih salah satu sebagai source of truth (saya rekomendasi Type 58 karena saham-centric lebih sering jadi entry point query).

### 5.8 Consumer: Trading Status Monitor

**Tujuan:** tracking session lifecycle untuk gating logic di consumer lain.

| Aspek       | Detail                                                    |
| ----------- | --------------------------------------------------------- |
| Subscribe   | `idx.status.session`, `idx.status.feed`                   |
| Source data | Type 57 (Trading Status), Type 13 (Control Messages)      |
| Logic       | Update current session state                              |
| Output      | Redis key `session:state` dan `session:lifecycle_history` |
| Side effect | Trigger pre-open setup, mid-day flush, post-close cleanup |

Status value yang penting (lihat spec halaman 11):

- `1` Begin sending records → reset semua sequence counter
- `8`/`9` Begin/End Pre-opening → mulai/stop akumulasi pre-market quote
- `3`/`4` Begin/End first session → main trading window
- `a`/`b` Begin/End Pre-closing → priority untuk closing price
- `c`/`d` Begin/End Post-trading → wait untuk Type 27 resend dengan broker code

Consumer lain (OHLCV, NBS) bisa subscribe ke status ini untuk handle transisi sesi dengan benar.

### 5.9 Consumer: Activity & Trading Summary

**Tujuan:** dashboard widget statistik market-wide.

| Aspek       | Detail                                                     |
| ----------- | ---------------------------------------------------------- |
| Subscribe   | `idx.activity.market`, `idx.summary.>`                     |
| Source data | Type 39 (Activity), Type 130 (Trading Summary)             |
| Logic       | Replace counter terbaru                                    |
| Output      | Redis key `activity:current` dan `summary:<stype>:<board>` |
| Historical  | PostgreSQL `daily_market_summary` untuk reporting          |

Type 39 = jumlah saham aktif/inaktif/up/down/no-change. Type 130 = total frequency/volume/value per board (RG/TN/NG) per stype.

Karena ini data agregat market-wide (volume kecil, beberapa update per menit), Redis cukup untuk live, dan archive harian ke PostgreSQL untuk laporan.

### 5.10 Consumer: Alert Engine (Temporal)

**Tujuan:** complex pattern detection untuk notifikasi user.

| Aspek       | Detail                                                                        |
| ----------- | ----------------------------------------------------------------------------- |
| Subscribe   | Multiple — tergantung rule (mostly `idx.trade.>`, `idx.nbs.>`)                |
| Source data | Trade, NBS, Best Quote                                                        |
| Logic       | Workflow Temporal dengan stateful pattern (mis. "BBRI naik 3% dalam 5 menit") |
| Output      | Webhook Discord, push notification, atau call ke n8n                          |
| State       | Workflow state di Temporal (PostgreSQL backing)                               |

Temporal worker subscribe ke NATS, tapi tidak simpan tick-nya — dia hanya evaluate pattern. State pattern ada di workflow Temporal sendiri (yang persisted di PostgreSQL Temporal backend).

---

## 6. Decision Matrix — Apa Disimpan Di Mana

Ini cara cepat menentukan storage tujuan untuk data dari record type baru atau use case baru:

| Karakteristik Data                                     | Storage                             |
| ------------------------------------------------------ | ----------------------------------- |
| Latest state, akses frequent, mutable, key-based       | **Redis**                           |
| Time-series, immutable, range query, OHLCV/aggregation | **QuestDB**                         |
| Multi-day snapshot, audit trail, reporting             | **QuestDB** atau **PostgreSQL**     |
| Document/text, full-text search, schema flexible       | **MongoDB**                         |
| Transaksional ACID, user/billing/permission            | **PostgreSQL**                      |
| Workflow state, retry-able operation                   | **Temporal** (backed by PostgreSQL) |

### 6.1 Mapping per record type ke storage akhir

| Type | Nama             | Live Storage                     | Historical Storage                        |
| ---- | ---------------- | -------------------------------- | ----------------------------------------- |
| 13   | Control Messages | Redis (current state)            | PostgreSQL (audit log)                    |
| 14   | Quote            | Redis (full FID hash)            | QuestDB (snapshot tiap N detik, optional) |
| 15   | Trade            | Redis (last trade)               | QuestDB (every tick)                      |
| 16   | Order            | Redis (orderbook sorted set)     | QuestDB (events, optional)                |
| 17   | Top 20           | Redis (current ranking)          | MongoDB (hourly snapshot)                 |
| 18   | Best Quote       | Redis (top of book)              | QuestDB (snapshot, optional)              |
| 26   | Resend Order     | (process untuk fill QuestDB gap) | QuestDB                                   |
| 27   | Resend Trade     | (process untuk fill QuestDB gap) | QuestDB (DEDUP overwrite)                 |
| 36   | News             | Redis (recent N items)           | MongoDB (full archive)                    |
| 39   | Activity         | Redis (current)                  | PostgreSQL (daily)                        |
| 40   | Trade Done       | Redis (volume profile current)   | QuestDB (price-volume map)                |
| 57   | Trading Status   | Redis (current state)            | PostgreSQL (event log)                    |
| 58   | NBS Stock        | Redis (running aggregate)        | QuestDB (EOD snapshot)                    |
| 59   | NBS Broker       | Redis (running aggregate)        | QuestDB (EOD snapshot)                    |
| 130  | Trading Summary  | Redis (current)                  | PostgreSQL (daily)                        |

### 6.2 Catatan tentang "optional" di kolom historical

Beberapa data tidak wajib disimpan tick-by-tick — hanya kalau Anda butuh replay atau backtest. Trade-off-nya:

**Wajib historical:**

- Type 15 (Trade) — basis untuk OHLCV historical, charting, backtest
- Type 36 (News) — content asset, jangan hilang
- Type 17 (Top 20) — analisa trend ranking

**Optional historical:**

- Type 14 (Quote) full snapshot — ukurannya besar, bisa di-skip kalau Anda hanya butuh OHLCV (yang derive dari Type 15)
- Type 18 (Best Quote) — derived dari Type 16 yang lebih granular
- Type 16 (Order) — sangat besar, hanya simpan kalau benar-benar butuh L2 backtest

Default rekomendasi awal: simpan **wajib** dulu, tambah **optional** kalau use case-nya muncul.

---

## 7. Catatan Praktis

### 7.1 Yang harus dikerjakan dulu (Phase 0/1)

Build pipeline minimal end-to-end pakai mode demo public IP yang sudah ada:

1. Go ingestion → connect ke demo IP, login, parse Type 15 saja
2. Single-node NATS JetStream lokal (Docker)
3. Single OHLCV consumer → write ke single-node QuestDB lokal
4. Single live state consumer → write ke single-node Redis lokal
5. WebSocket gateway → push 1 saham ke browser

Tujuan: verifikasi format vendor sama dengan spec, ukur latency end-to-end, validate skema. **Jangan loncat ke production setup sebelum ini works.**

### 7.2 Yang ditunda dulu

- Type 16 Order (volume sangat tinggi, tunda sampai use case-nya jelas)
- Type 36 News full archival (Phase 2 — pakai Redis recent N items dulu)
- Type 26/27 Resend (Phase 2 — kalau gap recovery sudah jadi pain point)
- Type 17 Top 20 broker historical (Phase 2)
- Multi-timeframe materialized view di QuestDB (Phase 2 — pakai 1m saja dulu)

### 7.3 Konfirmasi yang masih outstanding ke vendor

Sebelum daemon di-setup di FreeBSD VM:

1. Arah koneksi: outbound (kita ke mereka) atau inbound (mereka ke kita)?
2. Format output daemon ke aplikasi lokal: TCP localhost port berapa, atau Unix socket?
3. Format byte sama persis dengan demo public IP?
4. Aplikasi kita perlu login lagi ke daemon, atau langsung trusted local?
5. Trigger Type 26/27 resend: otomatis dari server, atau client request?
6. Heartbeat: apakah Type 13 dikirim periodik sebagai keepalive?
7. Concurrent connection limit di kontrak?

Tanpa jawaban ini, beberapa decision di dokumen ini berbasis asumsi yang bisa salah.

### 7.4 Monitoring critical metrics

Set alert ini dari Day 1, bukan Day 30:

- `iqplus_last_message_timestamp` — kalau >5 detik tidak update saat trading hour → Discord alert
- `iqplus_connection_state` — kalau != 2 (connected) saat trading hour → Discord + SMS
- NATS JetStream pending messages per stream — kalau >10rb → Discord (consumer lag)
- QuestDB insert latency p99 — kalau >1 detik → investigate disk I/O
- Redis memory usage — kalau >80% → Discord
- IQPlus Type 13 DOWN diterima → Discord + SMS

---

## Changelog

| Versi | Tanggal    | Catatan                                                             |
| ----- | ---------- | ------------------------------------------------------------------- |
| 1.0   | 2026-04-27 | Initial draft — flow lengkap dari IQPlus ke storage per record type |
