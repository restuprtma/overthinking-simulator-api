# IDX Data Catalog — Apa Disimpan Di Mana, Cara Ambilnya

> Audience: developer aplikasi (frontend, n8n author, backend), analyst.
> Sumber kebenaran: doc ini + comment di kode consumer masing-masing.
>
> **TL;DR**: setiap jenis data IDX punya "rumah" yang berbeda
> (Redis / QuestDB / MongoDB) dengan pola akses berbeda. Dokumen ini
> peta lengkap-nya.

---

## 1. Overview Pipeline

```
IQPlus TCP feed
      ↓
cmd/iqplus-publisher  (parse + dispatch ke 4 stream berbeda)
      ↓
NATS JetStream (10.10.8.2:4222)
      ├─ IDX_TICK   (24h retention)
      ├─ IDX_QUOTE  (12h retention)
      ├─ IDX_META   (24h retention)
      └─ IDX_NEWS   (7d  retention)
      ↓
8 consumer service (kerja paralel, 1 service per data type)
      ↓
Storage layer
      ├─ Redis (10.10.8.10:6379)        — live state, fast key lookup
      ├─ QuestDB (10.10.8.10:9000)      — historical tick, time-series
      └─ MongoDB (10.10.8.10:27017)     — news content + full-text search
```

---

## 2. Storage Allocation — Cheat Sheet

### 2.1 Redis — DB index allocation

| DB | Service | Key pattern | Berisi apa |
|---:|---|---|---|
| 9 | orderbook-consumer | `orderbook:<stock>:{bid,ask,_meta}` | Best bid/ask depth (Type 18) |
| 10 | (api umum) | various | Permission cache, session, dll |
| 11 | running-trade-consumer | `candle:<stock>:1m` | Live 1-minute bar (open/high/low/close) |
| 12 | quote-consumer | `quote:<stock>` | Full quote state (~80 FID per saham) |
| 13 | meta-consumer | `market:*` | Trading status, activity, summary, top20 |
| 14 | tradedone-consumer | `tradedone:<stock>:*` | Volume profile per harga |
| 15 | nbs-consumer | `nbs:{stock,broker}:*` | Bandar / foreign flow per (stock, broker) |

### 2.2 QuestDB

| Database | Table | Berisi apa | Retention |
|---|---|---|---|
| `qdb` | `trades` | Setiap tick trade (raw, 11 kolom) | 90 hari (TBD cron) |

### 2.3 MongoDB

| Database | Collection | Berisi apa | Retention |
|---|---|---|---|
| `tuai` | `news` | Berita lengkap dengan full-text index | Permanent |

---

## 3. Per Data Type — Detail Lengkap

### 3.1 Trade Tick — `QuestDB.trades`

**Service**: [`cmd/running-trade-consumer`](../cmd/running-trade-consumer/) (live)
+ [`cmd/resend-handler`](../cmd/resend-handler/) (UPSERT broker code post-close)

**Sumber**: IQPlus Type 15 (Trade) + Type 27 (Resend Trade)

**Schema** (lihat [docs/QuestDB/schema.md](QuestDB/schema.md) untuk DDL):

| Kolom | Tipe | Catatan |
|---|---|---|
| `timestamp` | TIMESTAMP | Designated, partition key (UTC) |
| `stock` | SYMBOL INDEX | Kode saham |
| `buyer` | SYMBOL | `--` saat live, real broker post-close |
| `seller` | SYMBOL | sda |
| `buyer_type` | SYMBOL | `F` foreign / `D` domestic |
| `seller_type` | SYMBOL | sda |
| `price` | DOUBLE | Last price |
| `volume` | LONG | Last volume (shares) |
| `buyer_order_no` | LONG | Bagian DEDUP key |
| `seller_order_no` | LONG | Bagian DEDUP key |
| `trade_no` | LONG | Per-stock per-day sequence |

**Update pattern**:
- Live: `INSERT` per tick via ILP HTTP (auto-batch 1000 row / 500ms)
- Post-close (~17-18 WIB): Type 27 datang dengan broker code real → `DEDUP UPSERT KEYS(timestamp, stock, buyer_order_no, seller_order_no)` → row Type 15 ter-overwrite

**Cara ambil**:

```sql
-- Last trade BBCA
SELECT * FROM trades
WHERE stock = 'BBCA'
LATEST ON timestamp PARTITION BY stock;

-- OHLCV 1-menit dari raw tick (tidak butuh table candles!)
SELECT timestamp,
       first(price) AS open, max(price) AS high,
       min(price) AS low, last(price) AS close,
       sum(volume) AS volume, count(*) AS trades
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
SAMPLE BY 1m
ORDER BY timestamp DESC;

-- Multi-timeframe gratis: ganti SAMPLE BY 5m / 15m / 1h / 1d
```

**Akses dari Go** (Postgres wire port 8812):
```go
import "github.com/jackc/pgx/v5"
conn, _ := pgx.Connect(ctx, "postgres://tuai_tan:TuaiTan1407*@10.10.8.10:8812/qdb")
```

**Akses dari REST**:
```bash
curl -G -u "tuai_tan:TuaiTan1407*" \
  --data-urlencode "query=SELECT * FROM trades WHERE stock='BBCA' LIMIT 10" \
  "http://10.10.8.10:9000/exec"
```

**Lengkap**:
- DDL + retention: [docs/QuestDB/schema.md](QuestDB/schema.md)
- **Query cookbook (50+ contoh)**: [docs/QuestDB/queries.md](QuestDB/queries.md)

---

### 3.2 Live Bar (Candle) — `Redis.DB 11`

**Service**: [`cmd/running-trade-consumer`](../cmd/running-trade-consumer/)

**Sumber**: Type 15 Trade → aggregator → Redis

**Storage layout**:

```
HASH candle:<stock>:1m {
  open:        "5400",
  high:        "5450",
  low:         "5380",
  close:       "5425",
  volume:      "125000",
  trades:      "47",
  open_ts:     "1746012600",  ← Unix epoch UTC
  close_ts:    "1746012660",
  updated_ts:  "1746012635",
  status:      "live"          ← atau "closed" saat bucket roll over
}
```

TTL: 25 jam (auto-evict next pre-open).

**Update pattern**: HSET incremental setiap tick masuk. Status flip ke `closed` saat bar berikutnya dibuka.

**Cara ambil**:

```bash
redis-cli -h 10.10.8.10 -a 'TuaiTan1407*' -n 11

> HGETALL candle:BBCA:1m
> HMGET candle:BBCA:1m open high low close volume status
> KEYS candle:*                # semua saham aktif (hati-hati di prod)
> SCAN 0 MATCH candle:* COUNT 100
```

**Akses dari Go**:
```go
client := redis.NewClient(&redis.Options{Addr: "10.10.8.10:6379", Password: "...", DB: 11})
m, _ := client.HGetAll(ctx, "candle:BBCA:1m").Result()
```

**Akses dari n8n**:
```
Operation: Hash Get All
Key:       candle:BBCA:1m
Key Type:  Hash
```

**Phase 2**: multi-timeframe (5m/15m/1h/1d) — saat ini hanya 1m. Untuk historical multi-tf, query QuestDB pakai `SAMPLE BY`.

---

### 3.3 Quote State — `Redis.DB 12`

**Service**: [`cmd/quote-consumer`](../cmd/quote-consumer/)

**Sumber**: IQPlus Type 14 (Quote) — vendor kirim incremental FID updates

**Storage layout**:

```
HASH quote:<stock> {
  # raw FID (number → value)
  "0":          "BBCA",
  "1":          "Bank Central Asia",
  "60":         "5400",       ← prev close
  "56":         "5450",       ← last price
  ...

  # named alias (sama nilai, ergonomic name)
  code:         "BBCA",
  name:         "Bank Central Asia",
  prev_close:   "5400",
  last_price:   "5450",
  open_price:   "5425",
  high_price:   "5475",
  low_price:    "5410",
  volume:       "12500000",
  change:       "50",
  pct_change:   "0.93",
  bid_price:    "5440",
  offer_price:  "5450",
  bid_volume:   "100000",
  offer_volume: "150000",
  sector_name:  "Financials",
  industry_name: "Banks",
  ... (sekitar 80 field)

  _updated_ts:  "1746012635",
  _seq:         "12345"
}
```

TTL: 25 jam.

**FID alias mapping**: lihat [`internal/modules/stock/quote_consumer/quote/quote.go`](../internal/modules/stock/quote_consumer/quote/quote.go) `FidAlias` map.

**Update pattern**: HSET incremental — Quote message hanya bawa FID yang berubah, sink tidak hapus FID lama.

**Cara ambil**:

```bash
redis-cli -h 10.10.8.10 -a 'TuaiTan1407*' -n 12

# Field tertentu — cara paling umum
> HGET quote:BBCA prev_close
> HGET quote:BBCA last_price
> HGET quote:BBCA pct_change

# Banyak field sekaligus
> HMGET quote:BBCA prev_close last_price open_price high_price low_price volume

# Semua field
> HGETALL quote:BBCA

# Raw FID number juga work
> HGET quote:BBCA 60        ← sama dengan prev_close
```

**Use case favorite — daily change %**:
```bash
> HMGET quote:BBCA prev_close last_price change pct_change
1) "5400"      ← close kemarin
2) "5450"      ← harga sekarang
3) "50"        ← change rupiah
4) "0.93"      ← change percent
```

---

### 3.4 Order Book — `Redis.DB 9`

**Service**: [`cmd/orderbook-consumer`](../cmd/orderbook-consumer/)

**Sumber**: IQPlus Type 18 (Best Quote) — vendor pre-aggregate per price level

**Storage layout**:

```
HASH orderbook:<stock>:bid {
  "5400": '{"lot":1000,"orders":5,"lot_f":300,"orders_f":2}',
  "5395": '{"lot":2500,"orders":12,"lot_f":0,"orders_f":0}',
  ...
}

HASH orderbook:<stock>:ask {
  "5410": '{"lot":800,"orders":4,"lot_f":100,"orders_f":1}',
  "5415": '{...}',
  ...
}

HASH orderbook:<stock>:_meta {
  top_bid:         "5400",     ← inside bid (max price di hash bid)
  top_ask:         "5410",     ← inside ask (min price di hash ask)
  bid_levels:      "5",
  ask_levels:      "8",
  last_updated_ts: "1746012635",
  _seq:            "12345",
  _bid_seq:        "12340",
  _ask_seq:        "12345"
}
```

TTL: 25 jam.

**Update pattern**: per-side atomic replace — saat update bid masuk, sink DEL+HSET semua level bid (ask tidak tersentuh). Sebaliknya untuk ask.

**Cara ambil**:

```bash
redis-cli -h 10.10.8.10 -a 'TuaiTan1407*' -n 9

# Top of book + spread (1 round-trip)
> HMGET orderbook:BBCA:_meta top_bid top_ask
1) "5400"
2) "5410"
# spread = top_ask - top_bid = 10 (compute di app)

# Full bid book
> HGETALL orderbook:BBCA:bid

# Full ask book
> HGETALL orderbook:BBCA:ask
```

**Use case — depth chart frontend**:
```javascript
const [bidRaw, askRaw, meta] = await Promise.all([
  redis.hgetall('orderbook:BBCA:bid'),
  redis.hgetall('orderbook:BBCA:ask'),
  redis.hgetall('orderbook:BBCA:_meta'),
]);

const bids = Object.entries(bidRaw)
  .map(([price, raw]) => ({ price: Number(price), ...JSON.parse(raw) }))
  .sort((a, b) => b.price - a.price);   // desc

const asks = Object.entries(askRaw)
  .map(([price, raw]) => ({ price: Number(price), ...JSON.parse(raw) }))
  .sort((a, b) => a.price - b.price);   // asc

const spread = Number(meta.top_ask) - Number(meta.top_bid);
```

---

### 3.5 Volume Profile — `Redis.DB 14`

**Service**: [`cmd/tradedone-consumer`](../cmd/tradedone-consumer/)

**Sumber**: IQPlus Type 40 (Trade Done) — cumulative buy/sell per (stock, price)

**Storage layout**:

```
HASH tradedone:<stock> {
  "5400": '{"bvol":12500,"svol":11000,"bfreq":50,"sfreq":42,"bvol_f":3000,"bfreq_f":12,"svol_f":1500,"sfreq_f":8}',
  "5410": '{...}',
  "5420": '{...}',
  ...
}

HASH tradedone:<stock>:_meta {
  last_price:      "5410",
  last_updated_ts: "1746012635",
  _seq:            "12345",
  updates:         "847"            ← total update count today
}
```

TTL: 25 jam.

**Update pattern**: vendor kirim cumulative total per (stock, price) — sink HSET overwrites field. Price levels accumulate sepanjang hari (banyak field saat session penuh).

**Cara ambil**:

```bash
redis-cli -h 10.10.8.10 -a 'TuaiTan1407*' -n 14

# Volume profile lengkap BBCA (semua price level)
> HGETALL tradedone:BBCA

# Volume di harga tertentu
> HGET tradedone:BBCA 5400
'{"bvol":12500,"svol":11000,...}'

# Berapa price level ter-track
> HLEN tradedone:BBCA
```

**Use case — volume profile chart**:
```javascript
const profile = await redis.hgetall('tradedone:BBCA');
const data = Object.entries(profile)
  .map(([price, raw]) => ({ price: Number(price), ...JSON.parse(raw) }))
  .sort((a, b) => a.price - b.price);
// data[i].bvol + data[i].svol = total volume di price[i]
```

---

### 3.6 NBS (Net Buy/Sell) — `Redis.DB 15`

**Service**: [`cmd/nbs-consumer`](../cmd/nbs-consumer/)

**Sumber**: IQPlus Type 58 (NBS Stock) + Type 59 (NBS Broker)

**Storage layout** — dual view (stock-centric + broker-centric):

```
HASH nbs:stock:BBYB {                ← stock-centric
  "PD":  '{"b_freq":3989,"b_vol":13206300,"b_lot":132063,"b_val":35407613000,"b_pct":0.27195,"s_freq":3076,...,"source":58}',
  "YP":  '{...}',
  "CC":  '{...}',
  ...
}

HASH nbs:broker:PD {                  ← broker-centric
  "BBYB": '{"b_freq":3989,...}',
  "BBCA": '{...}',
  "TLKM": '{...}',
  ...
}

HASH nbs:stock:BBYB:_meta { last_updated_ts, _seq, last_broker, updates }
HASH nbs:broker:PD:_meta  { last_updated_ts, _seq, last_stock, updates }
```

TTL: 25 jam.

**Update pattern**: Type 58 dan Type 59 keduanya di-projek ke kedua view (selalu freshest data).

**Cara ambil**:

```bash
redis-cli -h 10.10.8.10 -a 'TuaiTan1407*' -n 15

# "Broker mana saja yang trading BBYB?"
> HKEYS nbs:stock:BBYB

# "Berapa net flow broker PD di BBYB?"
> HGET nbs:stock:BBYB PD
'{"b_val":35407613000,"s_val":28449851000,...}'   ← net = b_val - s_val

# "Saham apa saja yang di-trading broker PD?"
> HKEYS nbs:broker:PD
> HGETALL nbs:broker:PD
```

**Use case — top 10 saham yang dibeli PD hari ini**:
```javascript
const stocks = await redis.hgetall('nbs:broker:PD');
const ranking = Object.entries(stocks)
  .map(([stock, raw]) => {
    const v = JSON.parse(raw);
    return { stock, net_buy: v.b_val - v.s_val };
  })
  .sort((a, b) => b.net_buy - a.net_buy)
  .slice(0, 10);
```

---

### 3.7 Market Metadata — `Redis.DB 13`

**Service**: [`cmd/meta-consumer`](../cmd/meta-consumer/)

**Sumber**: 5 record type low-volume (Control 13, Status 57, Activity 39, Summary 130, Top20 17)

**Storage layout**:

```
HASH market:control                       { state: "UP|DOWN", updated_ts, _seq }

HASH market:session                       { code, label, description, updated_ts }
LIST market:session:history (max 100)     "<ts>|<code>|<label>|<desc>" entries

HASH market:activity                      { inactive, active, down, nochg, up, updated_ts }

HASH market:summary:<stype>:<board>       { frequency, volume, value, f_bought_*, f_sold_*, ... }
HASH market:summary:<label>:<board>       same data, friendly key (e.g. market:summary:ordi:RG)

LIST market:top20:<type_code>             20 stock/broker codes
LIST market:top20:<type_label>            same codes, friendly key (e.g. market:top20:volume_rg)
HASH market:top20:<type_code>:_meta       { label, size, updated_ts, _seq }
```

TTL: 25 jam.

**Top 20 alias** (snake_case):
| Code | Alias | Code | Alias |
|---|---|---|---|
| 0 | volume_rg | 9 | freq_nonrg |
| 1 | value_rg | 10 | gainer_nonrg |
| 2 | freq_rg | 11 | loser_nonrg |
| 3 | gainer_rg | 12 | pct_gainer_nonrg |
| 4 | loser_rg | 13 | pct_loser_nonrg |
| 5 | pct_gainer_rg | 14 | volume_broker |
| 6 | pct_loser_rg | 15 | value_broker |
| 7 | volume_nonrg | 16 | freq_broker |
| 8 | value_nonrg | | |

**Status code → label**: `1`=begin_sending, `3`=begin_first_session, `4`=end_first_session, `5`=begin_second_session, `6`=end_second_session, `8`=begin_pre_opening, `9`=end_pre_opening, `a`=begin_pre_closing, `b`=end_pre_closing, dst.

**Cara ambil**:

```bash
redis-cli -h 10.10.8.10 -a 'TuaiTan1407*' -n 13

# Status sesi sekarang
> HGETALL market:session
# code: "3", label: "begin_first_session", description: "Begin first session", ...

# Status feed
> HGETALL market:control
# state: "UP", updated_ts: "..."

# Activity counter
> HGETALL market:activity
# inactive: "2512", active: "140", down: "25", nochg: "46", up: "69"

# Top 20 gainer regular market
> LRANGE market:top20:gainer_rg 0 -1
1) "BBCA"
2) "TLKM"
...

# Trading summary regular board, ordinary share
> HGETALL market:summary:ordi:RG

# History transisi sesi (10 terakhir)
> LRANGE market:session:history 0 9
```

---

### 3.8 News — `MongoDB.tuai.news`

**Service**: [`cmd/news-consumer`](../cmd/news-consumer/)

**Sumber**: IQPlus Type 36 (News, multi-packet)

**Storage layout** (collection `news`):

```javascript
{
  _id:               "1640260352855278",          // news_id from vendor (idempotent UPSERT)
  date:              ISODate("2021-12-23T11:53:06Z"), // UTC
  date_str:          "20211223",
  time_str:          "185306",                    // raw WIB
  category:          "BIS",
  company_id:        "TLKM",
  headline:          "TLKM Laba Rp 18,9 T...",
  story:             "<full reassembled text>",
  packets_received:  4,
  num_packets:       4,
  inserted_at:       ISODate("..."),
  schema_version:    "v1"
}
```

**Indexes auto-created**:
- `{ date: -1 }` — recent first
- `{ company_id: 1, date: -1 }` — per-stock timeline
- `{ headline: "text", story: "text" }` — full-text search

**Retention**: permanent (kalau perlu trim, manual cron).

**Update pattern**: multi-packet di-buffer di memory consumer, insert ke Mongo saat semua packet diterima. UPSERT by `_id` — vendor resend / JetStream redeliver tidak duplikat.

**Cara ambil**:

```bash
mongosh "mongodb://tuai_tan:TuaiTan1407%2A@10.10.8.10:27017/?authSource=admin"
> use tuai

# 10 berita terbaru
> db.news.find().sort({date: -1}).limit(10);

# Berita per ticker, sorted by date
> db.news.find({company_id: "TLKM"}).sort({date: -1}).limit(20);

# Full-text search
> db.news.find({$text: {$search: "laba bersih dividen"}}).limit(20);

# Berita 24 jam terakhir
> db.news.find({date: {$gte: new Date(Date.now() - 86400000)}}).sort({date: -1});

# Count per ticker bulan ini
> db.news.aggregate([
    {$match: {date: {$gte: ISODate("2026-04-01")}}},
    {$group: {_id: "$company_id", total: {$sum: 1}}},
    {$sort: {total: -1}},
    {$limit: 20}
  ]);
```

**Akses dari Go**:
```go
import "go.mongodb.org/mongo-driver/mongo"
cli, _ := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://..."))
news := cli.Database("tuai").Collection("news")
```

---

## 4. Cheat Sheet Per Use Case

### 4.1 "Dashboard widget — current price BBCA"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 12 HGET quote:BBCA last_price
```

### 4.2 "Dashboard widget — daily change %"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 12 HMGET quote:BBCA prev_close last_price change pct_change
```

### 4.3 "Trading view — OHLCV chart 1-menit hari ini"

```sql
-- QuestDB
SELECT timestamp, first(price) open, max(price) high,
       min(price) low, last(price) close, sum(volume) volume
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
SAMPLE BY 1m;
```

### 4.4 "Trading view — current candle real-time (sub-detik)"

```bash
# Redis live bar update setiap tick
redis-cli -h 10.10.8.10 -a '...' -n 11 HGETALL candle:BBCA:1m
```

### 4.5 "Order book depth chart"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 9 HGETALL orderbook:BBCA:bid
redis-cli -h 10.10.8.10 -a '...' -n 9 HGETALL orderbook:BBCA:ask
redis-cli -h 10.10.8.10 -a '...' -n 9 HGETALL orderbook:BBCA:_meta
```

### 4.6 "Volume profile — di harga berapa banyak yang trading"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 14 HGETALL tradedone:BBCA
```

### 4.7 "Bandar tracker — broker mana akumulasi BBYB?"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 15 HGETALL nbs:stock:BBYB
# Sort by net_buy = b_val - s_val di app
```

### 4.8 "Top 20 gainer hari ini"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 13 LRANGE market:top20:gainer_rg 0 -1
# Cross-reference dengan Redis 12 quote:<stock> untuk dapat detail
```

### 4.9 "News dashboard — 50 berita terbaru"

```javascript
db.news.find().sort({date: -1}).limit(50).toArray();
```

### 4.10 "Market status — apakah market open sekarang?"

```bash
redis-cli -h 10.10.8.10 -a '...' -n 13 HGET market:session label
# Output: "begin_first_session" / "end_second_session" / dll
```

### 4.11 "Backtest — semua trade BBCA antara 09:00-10:00 hari ini"

```sql
-- QuestDB
SELECT * FROM trades
WHERE stock = 'BBCA'
  AND timestamp >= '2026-04-27T02:00:00Z'   -- 09:00 WIB
  AND timestamp <  '2026-04-27T03:00:00Z'   -- 10:00 WIB
ORDER BY timestamp;
```

### 4.12 "Foreign flow — net foreign buy hari ini per saham"

```sql
-- QuestDB (per tick computation)
SELECT
  stock,
  sum(CASE WHEN buyer_type  = 'F' THEN price * volume ELSE 0 END) AS f_buy,
  sum(CASE WHEN seller_type = 'F' THEN price * volume ELSE 0 END) AS f_sell,
  sum(CASE WHEN buyer_type  = 'F' THEN price * volume ELSE 0 END)
    - sum(CASE WHEN seller_type = 'F' THEN price * volume ELSE 0 END) AS net_foreign
FROM trades
WHERE timestamp IN today()
GROUP BY stock
ORDER BY net_foreign DESC
LIMIT 20;
```

---

## 5. Connection Strings — One-Stop

### Redis
```
URL:         redis://10.10.8.10:6379
Password:    TuaiTan1407*
DB allocation:
  9  → orderbook
  10 → api
  11 → running-trade (candle)
  12 → quote
  13 → market metadata
  14 → tradedone (volume profile)
  15 → nbs
```

### QuestDB
```
HTTP REST/ILP:    http://10.10.8.10:9000
PostgreSQL wire:  postgres://tuai_tan:TuaiTan1407%2A@10.10.8.10:8812/qdb
TCP ILP:          10.10.8.10:9009
Web Console:      http://10.10.8.10:9000
Auth (basic):     tuai_tan / TuaiTan1407*
```

### MongoDB
```
URI:       mongodb://tuai_tan:TuaiTan1407%2A@10.10.8.10:27017/?authSource=admin
Database:  tuai
Collection: news
```

### NATS JetStream
```
URL:        nats://10.10.8.2:4222
Token:      Mrm25UYHeaMa19yHtGlWkFEtyoQ16lrU0uzs7CzFRNA
Monitoring: http://10.10.8.2:8222
Streams:    IDX_TICK, IDX_QUOTE, IDX_META, IDX_NEWS
```

---

## 6. Akses dari n8n — Pattern Umum

### Redis (Hash)
```
Operation: Hash Get All
Credential: Redis account (set DB index sesuai data)
Name:      bar / quote / book (output property name)
Key:       candle:BBCA:1m / quote:BBCA / orderbook:BBCA:bid
Key Type:  Hash
```

Output di node berikutnya: `{{ $json.bar.last_price }}` dst.

Untuk numeric cast: `{{ Number($json.bar.last_price) }}`.

### MongoDB
```
Operation: Find / Aggregate
Collection: news
Query:     { "company_id": "BBCA" }
Sort:      { "date": -1 }
Limit:     10
```

### QuestDB (HTTP REST)
```
Method: GET
URL:    http://10.10.8.10:9000/exec
Auth:   Basic (tuai_tan / TuaiTan1407*)
Query Parameters:
  query: SELECT timestamp, last(price) FROM trades WHERE stock='BBCA' SAMPLE BY 1m LIMIT 10
```

---

## 7. TTL & Lifecycle Summary

| Storage | Item | Lifetime |
|---|---|---|
| Redis 9–15 (semua) | All keys | 25 jam (auto-evict) |
| QuestDB `trades` | Hot tick | 90 hari (TBD cron) |
| MongoDB `news` | All news | Permanent |
| NATS IDX_TICK | Stream messages | 24 jam |
| NATS IDX_QUOTE | Stream messages | 12 jam |
| NATS IDX_META | Stream messages | 24 jam |
| NATS IDX_NEWS | Stream messages | 7 hari |

**Implikasi**:
- Restart consumer: tidak masalah, JetStream replay backlog (selama < retention)
- Restart Redis: data hilang sampai consumer process ulang JetStream backlog
- Restart QuestDB: data persistent (file storage)
- Restart MongoDB: data persistent

---

## 8. Operational Caveats

### 8.1 Trading hour vs off-hours

Data baru masuk hanya saat IDX trading hour (Senin-Jumat 09:00–15:00 WIB). Di luar itu:
- Redis live state stays (TTL 25h, valid sampai pre-open hari berikutnya)
- QuestDB tetap bisa di-query untuk historical
- News bisa muncul kapan saja (vendor announce)

### 8.2 Resend Trade post-close

Type 27 (resend trade dengan broker code asli) datang ~17-18 WIB:
- `cmd/resend-handler` overwrite QuestDB `trades` row via DEDUP UPSERT
- Sebelum jam ini, query `buyer/seller` di trade hari ini akan `--`
- Setelah jam ini, broker code asli muncul

### 8.3 Live vs Historical OHLCV

| Pertanyaan | Source |
|---|---|
| "Current 1-min bar BBCA sekarang" (live, sub-detik) | Redis `candle:BBCA:1m` |
| "1-min OHLC BBCA 1 jam terakhir" (historical) | QuestDB `SAMPLE BY 1m` |
| "5-min / 15-min / 1-jam OHLC" | QuestDB `SAMPLE BY 5m/15m/1h` (tidak ada di Redis) |
| "Daily OHLC kemarin" | QuestDB `SAMPLE BY 1d` |

### 8.4 Cardinality limits

| Storage | Item | Worst case | Note |
|---|---|---|---|
| Redis quote: | ~80 FID × 900 stock × 2 (raw+alias) | ~144k field total | Aman |
| Redis tradedone: | ~100 price × 900 stock | ~90k field hash | Aman, bisa membengkak akhir sesi |
| Redis nbs: | 100 broker × 900 stock × 2 view | ~180k field | Aman |
| QuestDB trades | ~10k tick/s × 6h × 250 hari | ~54B row/tahun | Partition by HOUR/DAY help |

---

## 9. Troubleshooting Cepat

### "Redis kosong padahal trading hour"
1. Cek consumer hidup: `ps aux | grep -consumer`
2. Cek consumer pending: `nats consumer info IDX_TICK ohlcv-aggregator`
3. Cek log consumer: `tail -f /tmp/<service>.log | grep -i error`

### "QuestDB kosong padahal Redis ok"
1. Cek `MEMORY USAGE` untuk redis (cek running-trade-consumer hidup)
2. Test ILP curl: `docs/QuestDB/schema.md` §2 cara apply DDL
3. Cek `/jsz` untuk ack lag

### "News tidak masuk Mongo"
1. Cek connection: `mongosh "mongodb://..."` ping
2. Cek consumer log: `news inserted` line setiap news complete
3. Outside trading hour news rate rendah, wajar kosong

### "Berapa lag consumer?"
```bash
nats consumer info <STREAM> <DURABLE>
# Look at: Pending Messages, Last Delivery
```

---

## 10. Referensi Kode

| Service | Internal package | cmd |
|---|---|---|
| Publisher | `internal/modules/stock/iqplus_publisher/` | [cmd/iqplus-publisher](../cmd/iqplus-publisher/) |
| OHLCV | `internal/modules/stock/running_trade_consumer/` | [cmd/running-trade-consumer](../cmd/running-trade-consumer/) |
| Quote | `internal/modules/stock/quote_consumer/` | [cmd/quote-consumer](../cmd/quote-consumer/) |
| Meta | `internal/modules/stock/meta_consumer/` | [cmd/meta-consumer](../cmd/meta-consumer/) |
| Tradedone | `internal/modules/stock/tradedone_consumer/` | [cmd/tradedone-consumer](../cmd/tradedone-consumer/) |
| NBS | `internal/modules/stock/nbs_consumer/` | [cmd/nbs-consumer](../cmd/nbs-consumer/) |
| News | `internal/modules/stock/news_consumer/` | [cmd/news-consumer](../cmd/news-consumer/) |
| Resend | `internal/modules/stock/resend_handler/` | [cmd/resend-handler](../cmd/resend-handler/) |
| Orderbook | `internal/modules/stock/orderbook_consumer/` | [cmd/orderbook-consumer](../cmd/orderbook-consumer/) |

Shared:
- `internal/modules/stock/iqplus_envelope/` — JSON envelope contract
- [docs/iqplus/iqplus-data-feed-v4.0.0.md](iqplus/iqplus-data-feed-v4.0.0.md) — protocol spec
- [docs/infra/topology.md](infra/topology.md) — full pipeline plan
- [docs/JetStream/streams.md](JetStream/streams.md) — server stream setup
- [docs/QuestDB/schema.md](QuestDB/schema.md) — DDL + retention

---

## Changelog

| Versi | Tanggal | Catatan |
|---|---|---|
| 1.0 | 2026-04-27 | Initial — 8 consumer service complete (semua subject IDX ter-konsumsi) |
