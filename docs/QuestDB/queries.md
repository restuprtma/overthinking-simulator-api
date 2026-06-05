# QuestDB — Query Cookbook untuk `trades` table

> Audience: backend dev, analyst, dashboard builder.
> Sumber data: table `trades` (lihat [schema.md](schema.md)).
>
> Akses: web console `http://10.10.8.10:9000`, atau via REST/Postgres/Go.
> Kolom timestamp namanya **`timestamp`** (bukan `ts`).

---

## 1. Health Check / Live Monitoring

### 1.1 Apakah data sedang masuk?

```sql
-- Tick rate per detik 1 menit terakhir
SELECT timestamp, count(*) AS ticks_per_sec
FROM trades
WHERE timestamp > dateadd('m', -1, now())
SAMPLE BY 1s
ORDER BY timestamp DESC;
```

Saat trading hour, harusnya angka tidak 0 di setiap detik.

### 1.2 Tick terakhir masuk kapan?

```sql
SELECT max(timestamp) AS last_tick,
       (now() - max(timestamp)) AS lag
FROM trades;
```

`lag` > 5 detik saat trading hour = ada masalah di publisher atau running-trade-consumer.

### 1.3 Total tick hari ini per saham (top 20)

```sql
SELECT stock, count(*) AS ticks, sum(volume) AS volume
FROM trades
WHERE timestamp IN today()
GROUP BY stock
ORDER BY ticks DESC
LIMIT 20;
```

### 1.4 10 trade terbaru (any stock)

```sql
SELECT timestamp, stock, price, volume, buyer, seller
FROM trades
ORDER BY timestamp DESC
LIMIT 10;
```

### 1.5 10 trade terbaru per saham tertentu

```sql
SELECT timestamp, price, volume, buyer, seller
FROM trades
WHERE stock = 'BBCA'
ORDER BY timestamp DESC
LIMIT 10;
```

---

## 2. OHLCV — Single Stock

### 2.1 1-menit bar BBCA hari ini

```sql
SELECT
  timestamp,
  first(price) AS open,
  max(price)   AS high,
  min(price)   AS low,
  last(price)  AS close,
  sum(volume)  AS volume,
  count(*)     AS trades
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
SAMPLE BY 1m
ORDER BY timestamp DESC;
```

### 2.2 Multi-timeframe (5m / 15m / 1h / 4h / 1d)

Tinggal ganti `SAMPLE BY`:

```sql
-- 5-menit
SELECT timestamp, first(price) open, max(price) high,
       min(price) low, last(price) close, sum(volume) volume
FROM trades
WHERE stock = 'BBCA' AND timestamp > dateadd('h', -4, now())
SAMPLE BY 5m;

-- 1-jam
SAMPLE BY 1h

-- Daily (1 hari)
SAMPLE BY 1d
```

### 2.3 OHLCV jam terakhir (window query)

```sql
SELECT timestamp, first(price) open, max(price) high,
       min(price) low, last(price) close, sum(volume) volume
FROM trades
WHERE stock = 'BBCA' AND timestamp > dateadd('h', -1, now())
SAMPLE BY 1m;
```

### 2.4 OHLCV custom date range (backtest)

```sql
SELECT timestamp, first(price) open, max(price) high,
       min(price) low, last(price) close, sum(volume) volume
FROM trades
WHERE stock = 'BBCA'
  AND timestamp >= '2026-04-20T02:00:00Z'   -- 09:00 WIB tgl 20
  AND timestamp <  '2026-04-25T08:00:00Z'   -- 15:00 WIB tgl 24
SAMPLE BY 1d;
```

---

## 3. OHLCV — All / Multi Stock

### 3.1 Snapshot 1-menit terbaru per saham

```sql
SELECT
  stock,
  last(timestamp) AS bar_time,
  first(price)    AS open,
  max(price)      AS high,
  min(price)      AS low,
  last(price)     AS close,
  sum(volume)     AS volume
FROM trades
WHERE timestamp > dateadd('m', -1, now())
GROUP BY stock
ORDER BY volume DESC;
```

### 3.2 Daily OHLC semua saham yang trading hari ini

```sql
SELECT stock,
       first(price) AS open,
       max(price)   AS high,
       min(price)   AS low,
       last(price)  AS close,
       sum(volume)  AS volume,
       count(*)     AS trades
FROM trades
WHERE timestamp IN today()
GROUP BY stock;
```

### 3.3 Compare last price beberapa saham

```sql
SELECT stock, last(price) AS last_px, last(timestamp) AS last_trade_at
FROM trades
WHERE stock IN ('BBCA', 'BBRI', 'BMRI', 'BBNI')
  AND timestamp > dateadd('m', -10, now())
GROUP BY stock;
```

---

## 4. Top Movers / Ranking

### 4.1 Top 20 most traded by value (price × volume)

```sql
SELECT stock,
       sum(price * volume) AS value_traded,
       sum(volume)         AS total_volume,
       count(*)            AS trade_count
FROM trades
WHERE timestamp IN today()
GROUP BY stock
ORDER BY value_traded DESC
LIMIT 20;
```

### 4.2 Top gainer hari ini (% change open vs current last)

> QuestDB tidak support `HAVING` — pakai subquery + outer `WHERE` untuk filter.

```sql
SELECT * FROM (
  SELECT stock,
         first(price) AS open,
         last(price)  AS last_px,
         round(((last(price) - first(price)) / first(price)) * 100, 2) AS pct_change,
         count(*)     AS trade_count
  FROM trades
  WHERE timestamp IN today()
  GROUP BY stock
)
WHERE trade_count > 10                -- skip saham yang baru sedikit trade
ORDER BY pct_change DESC
LIMIT 20;
```

### 4.3 Top loser hari ini

```sql
SELECT * FROM (
  SELECT stock,
         first(price) AS open,
         last(price)  AS last_px,
         round(((last(price) - first(price)) / first(price)) * 100, 2) AS pct_change,
         count(*)     AS trade_count
  FROM trades
  WHERE timestamp IN today()
  GROUP BY stock
)
WHERE trade_count > 10
ORDER BY pct_change ASC
LIMIT 20;
```

### 4.4 Most active by frequency 5 menit terakhir

```sql
SELECT stock, count(*) AS trades, sum(volume) AS volume
FROM trades
WHERE timestamp > dateadd('m', -5, now())
GROUP BY stock
ORDER BY trades DESC
LIMIT 20;
```

### 4.5 Stock dengan volume terbesar 1 jam terakhir

```sql
SELECT stock, sum(volume) AS volume, count(*) AS trades
FROM trades
WHERE timestamp > dateadd('h', -1, now())
GROUP BY stock
ORDER BY volume DESC
LIMIT 20;
```

---

## 5. Latest Tick (Snapshot)

### 5.1 Last tick per saham (semua saham)

```sql
SELECT timestamp, stock, price, volume, buyer, seller
FROM trades
LATEST ON timestamp PARTITION BY stock;
```

`LATEST ON ... PARTITION BY` itu QuestDB-specific shortcut yang lebih cepat dari `MAX(timestamp) GROUP BY`.

### 5.2 Last tick satu saham

```sql
SELECT timestamp, price, volume, buyer, seller
FROM trades
WHERE stock = 'BBCA'
LATEST ON timestamp PARTITION BY stock;
```

### 5.3 Last tick beberapa saham terpilih

```sql
SELECT timestamp, stock, price, volume
FROM trades
WHERE stock IN ('BBCA', 'BBRI', 'TLKM')
LATEST ON timestamp PARTITION BY stock;
```

---

## 6. Foreign vs Domestic Flow

### 6.1 Net foreign buy per saham hari ini

```sql
SELECT
  stock,
  sum(CASE WHEN buyer_type  = 'F' THEN price * volume ELSE 0 END) AS foreign_buy,
  sum(CASE WHEN seller_type = 'F' THEN price * volume ELSE 0 END) AS foreign_sell,
  sum(CASE WHEN buyer_type  = 'F' THEN price * volume ELSE 0 END)
    - sum(CASE WHEN seller_type = 'F' THEN price * volume ELSE 0 END) AS net_foreign
FROM trades
WHERE timestamp IN today()
GROUP BY stock
ORDER BY net_foreign DESC
LIMIT 20;
```

### 6.2 Foreign flow timeline 5-menit untuk satu saham

```sql
SELECT timestamp,
       sum(CASE WHEN buyer_type  = 'F' THEN volume ELSE 0 END) AS f_buy_vol,
       sum(CASE WHEN seller_type = 'F' THEN volume ELSE 0 END) AS f_sell_vol,
       sum(CASE WHEN buyer_type  = 'F' THEN volume ELSE 0 END)
         - sum(CASE WHEN seller_type = 'F' THEN volume ELSE 0 END) AS net_vol
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
SAMPLE BY 5m;
```

### 6.3 Top foreign net seller hari ini

```sql
SELECT stock,
       sum(CASE WHEN buyer_type  = 'F' THEN price * volume ELSE 0 END)
         - sum(CASE WHEN seller_type = 'F' THEN price * volume ELSE 0 END) AS net_foreign
FROM trades
WHERE timestamp IN today()
GROUP BY stock
ORDER BY net_foreign ASC          -- ASC untuk yang paling negatif (foreign jualan)
LIMIT 20;
```

---

## 7. Volume Profile (Per Harga)

### 7.1 Volume per price level — satu saham hari ini

```sql
SELECT price, sum(volume) AS total_volume, count(*) AS trades
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
GROUP BY price
ORDER BY price;
```

### 7.2 Volume per price level — historical 30 hari

```sql
SELECT price, sum(volume) AS total_volume
FROM trades
WHERE stock = 'BBCA' AND timestamp > dateadd('d', -30, now())
GROUP BY price
ORDER BY total_volume DESC
LIMIT 20;                         -- top 20 price level paling sering trading
```

---

## 8. Broker Activity (post-close, butuh resend-handler running)

### 8.1 Broker yang paling aktif di BBCA hari ini

```sql
SELECT buyer AS broker,
       sum(volume) AS bought_vol,
       sum(price * volume) AS bought_val,
       count(*) AS bought_trades
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today() AND buyer != '--'
GROUP BY buyer
ORDER BY bought_val DESC
LIMIT 20;
```

### 8.2 Net flow broker PD hari ini per saham

```sql
SELECT stock,
       sum(CASE WHEN buyer  = 'PD' THEN price * volume ELSE 0 END) AS pd_bought,
       sum(CASE WHEN seller = 'PD' THEN price * volume ELSE 0 END) AS pd_sold,
       sum(CASE WHEN buyer  = 'PD' THEN price * volume ELSE 0 END)
         - sum(CASE WHEN seller = 'PD' THEN price * volume ELSE 0 END) AS net_pd
FROM trades
WHERE timestamp IN today()
  AND (buyer = 'PD' OR seller = 'PD')
GROUP BY stock
ORDER BY net_pd DESC;
```

### 8.3 Cross-broker matching — broker mana sering trade dengan PD?

```sql
SELECT seller AS counterpart,
       count(*) AS trade_count,
       sum(price * volume) AS total_value
FROM trades
WHERE buyer = 'PD' AND timestamp IN today() AND seller != '--'
GROUP BY seller
ORDER BY trade_count DESC
LIMIT 20;
```

---

## 9. Time-Series Analysis

### 9.1 VWAP (Volume-Weighted Average Price) hari ini

```sql
SELECT stock,
       sum(price * volume) / sum(volume) AS vwap,
       sum(volume) AS total_volume
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
GROUP BY stock;
```

### 9.2 VWAP per 30-menit (intraday VWAP)

```sql
SELECT timestamp,
       sum(price * volume) / sum(volume) AS vwap,
       sum(volume) AS volume
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
SAMPLE BY 30m;
```

### 9.3 Tick velocity — berapa cepat harga bergerak

```sql
SELECT timestamp,
       max(price) - min(price) AS price_range,
       (max(price) - min(price)) / first(price) * 100 AS volatility_pct
FROM trades
WHERE stock = 'BBCA' AND timestamp > dateadd('h', -1, now())
SAMPLE BY 5m;
```

### 9.4 Trade size distribution

```sql
SELECT
  CASE
    WHEN volume < 100    THEN '<100'
    WHEN volume < 1000   THEN '100-1k'
    WHEN volume < 10000  THEN '1k-10k'
    WHEN volume < 100000 THEN '10k-100k'
    ELSE '>100k'
  END AS size_bucket,
  count(*) AS trade_count,
  sum(volume) AS total_volume
FROM trades
WHERE stock = 'BBCA' AND timestamp IN today()
GROUP BY size_bucket;
```

---

## 10. Operasional / Maintenance

### 10.1 Total disk usage table `trades`

```sql
SELECT
  sum(diskSize) / 1024 / 1024 AS size_mb,
  sum(numRows)               AS total_rows
FROM table_partitions('trades');
```

### 10.2 Daftar partisi (urutan terbaru)

```sql
SELECT name, minTimestamp, maxTimestamp, numRows, diskSize
FROM table_partitions('trades')
ORDER BY minTimestamp DESC
LIMIT 24;
```

### 10.3 Kolom & metadata table

```sql
SHOW COLUMNS FROM trades;
```

### 10.4 WAL status

```sql
SELECT * FROM wal_tables() WHERE name = 'trades';
-- walEnabled harus true untuk DEDUP UPSERT bekerja
```

### 10.5 Drop partisi >90 hari (retention)

```sql
ALTER TABLE trades DROP PARTITION
  WHERE timestamp < dateadd('d', -90, now());
```

> Pasang sebagai cron 03:00 WIB (di luar trading hour). Lihat
> [schema.md §6](schema.md#6-retention--drop-partisi-lama-otomatis).

### 10.6 Cek apakah resend-handler sudah backfill broker code

```sql
SELECT
  CASE WHEN buyer = '--' THEN 'masked' ELSE 'real' END AS state,
  count(*) AS rows
FROM trades
WHERE timestamp > dateadd('h', -24, now())
GROUP BY state;
```

Setelah resend window (~17-18 WIB), `real` count harusnya jauh lebih banyak dari `masked`.

---

## 11. Akses Programmatic

### 11.1 HTTP REST (curl, n8n, Postman, dll)

```bash
# GET /exec → JSON response
curl -G -u "tuai_tan:TuaiTan1407*" \
  --data-urlencode "query=SELECT timestamp, last(price) FROM trades WHERE stock='BBCA' SAMPLE BY 1m LIMIT 10" \
  "http://10.10.8.10:9000/exec" | jq

# Response format:
# {
#   "query": "SELECT ...",
#   "columns": [{"name":"timestamp","type":"TIMESTAMP"}, ...],
#   "dataset": [["2026-04-27T01:00:00.000000Z", 5400], ...],
#   "count": 10
# }
```

### 11.2 Postgres wire (Go pgx)

QuestDB expose Postgres protocol di port **8812**:

```go
import "github.com/jackc/pgx/v5"

conn, _ := pgx.Connect(ctx,
  "postgres://tuai_tan:TuaiTan1407%2A@10.10.8.10:8812/qdb")

rows, _ := conn.Query(ctx,
  "SELECT timestamp, price, volume FROM trades WHERE stock=$1 ORDER BY timestamp DESC LIMIT $2",
  "BBCA", 10)

for rows.Next() {
  var ts time.Time
  var price float64
  var volume int64
  rows.Scan(&ts, &price, &volume)
  // ...
}
```

Pakai `pgxpool` untuk connection pooling — sama pattern dengan main Postgres connection di project.

### 11.3 n8n HTTP Request node

```
Method: GET
URL:    http://10.10.8.10:9000/exec
Authentication: Basic Auth
  Username: tuai_tan
  Password: TuaiTan1407*
Query Parameters:
  query: SELECT timestamp, last(price) FROM trades WHERE stock='BBCA' SAMPLE BY 1m LIMIT 10
```

Output ada di `$json.dataset` (array of arrays). Pakai Code node untuk transform ke object kalau perlu.

### 11.4 Python

```python
import requests
from requests.auth import HTTPBasicAuth

resp = requests.get(
    "http://10.10.8.10:9000/exec",
    params={"query": "SELECT * FROM trades WHERE stock='BBCA' LIMIT 10"},
    auth=HTTPBasicAuth("tuai_tan", "TuaiTan1407*"),
)
data = resp.json()
for row in data["dataset"]:
    print(row)
```

---

## 12. Tips & Gotchas

### 12.1 Pakai `SAMPLE BY` bukan manual time-bucketing

❌ Slow:
```sql
SELECT date_trunc('minute', timestamp), sum(volume)
FROM trades GROUP BY 1;
```

✅ Fast (10-100× lebih cepat di QuestDB):
```sql
SELECT timestamp, sum(volume)
FROM trades SAMPLE BY 1m;
```

### 12.2 Filter timestamp DULU (partition pruning)

❌ Scan semua partisi:
```sql
SELECT count(*) FROM trades WHERE volume > 100000;
```

✅ Skip partisi tidak relevan:
```sql
SELECT count(*) FROM trades
WHERE timestamp IN today() AND volume > 100000;
```

### 12.3 Hindari JOIN

QuestDB optimized untuk single-table time-series. Kalau perlu lookup, denormalize di consumer side (mis. ambil last price dari Redis dulu, baru query trades).

### 12.4 `LATEST ON ... PARTITION BY` >> `MAX(...) GROUP BY`

```sql
-- Lambat
SELECT stock, max(timestamp) AS last_ts FROM trades GROUP BY stock;

-- Cepat (gunakan partition skip)
SELECT timestamp, stock FROM trades LATEST ON timestamp PARTITION BY stock;
```

### 12.5 Date shortcuts QuestDB

| Shortcut | Arti |
|---|---|
| `today()` | Hari ini (UTC) |
| `yesterday()` | Kemarin |
| `IN today()` | `WHERE ts >= today() AND ts < tomorrow()` |
| `IN '2026-04-27'` | Range satu hari penuh |
| `dateadd('m', -5, now())` | 5 menit yang lalu |
| `dateadd('h', -1, now())` | 1 jam yang lalu |
| `dateadd('d', -30, now())` | 30 hari yang lalu |

Unit: `s` (detik), `m` (menit), `h` (jam), `d` (hari), `w` (minggu), `M` (bulan), `y` (tahun).

### 12.6 `HAVING` tidak didukung — pakai subquery

Error `unexpected token [HAVING]` muncul karena QuestDB tidak punya
`HAVING` clause. Wrap aggregate di subquery, lalu `WHERE` di outer:

```sql
-- ❌ ERROR
SELECT stock, count(*) AS cnt FROM trades GROUP BY stock HAVING cnt > 10;

-- ✅ FIX
SELECT * FROM (
  SELECT stock, count(*) AS cnt FROM trades GROUP BY stock
) WHERE cnt > 10;
```

### 12.7 Timestamp di QuestDB selalu UTC

Vendor IQPlus kirim WIB, publisher convert ke UTC sebelum write. Jadi semua query timestamp harus thinking UTC, atau convert eksplisit:

```sql
-- WIB display (UTC + 7)
SELECT to_timezone(timestamp, 'Asia/Jakarta') AS ts_wib, stock, price
FROM trades WHERE stock='BBCA'
LIMIT 5;
```

---

## 13. Lengkap

- Schema lengkap & DDL: [docs/QuestDB/schema.md](schema.md)
- Pipeline overview: [docs/data-catalog.md §3.1](../data-catalog.md#31-trade-tick--questdbtrades)
- Topology asli: [docs/infra/topology.md §5.3](../infra/topology.md)
