# QuestDB Schema — Setup yang Dibutuhkan untuk IDX Tick Storage

> Audience: tim yang setup QuestDB instance (10.10.8.10).
> Sumber: [docs/infra/topology.md](../infra/topology.md) §5.3.
>
> OHLCV consumer (`cmd/running-trade-consumer`) menulis raw tick ke table `trades`
> via ILP HTTP. Dokumen ini DDL yang harus di-apply sebelum consumer
> di-jalankan.

---

## 1. Kenapa harus pre-create (tidak biarkan ILP auto-create)

QuestDB ILP akan auto-create table kalau tidak ada, tapi:

- Tidak ada **DEDUP UPSERT KEYS** → resend trade (Type 27) akan jadi row
  duplikat alih-alih overwrite row dari Type 15 dengan broker code real.
- Tidak ada **PARTITION BY HOUR** → satu partisi membesar tanpa batas,
  query lambat, retention drop susah.
- Tidak ada **CAPACITY hint** untuk SYMBOL `stock` → memory overhead
  tinggi (default capacity 256 padahal IDX punya ~800 ticker).
- Tidak ada **WAL** → tidak bisa concurrent write dari multiple consumer.

Apply DDL berikut sebelum start consumer pertama.

---

## 2. DDL — copy-paste ke QuestDB web console atau via curl

```sql
CREATE TABLE IF NOT EXISTS trades (
  timestamp       TIMESTAMP,
  stock           SYMBOL CAPACITY 2048 INDEX,
  market          SYMBOL CAPACITY 8,    -- RG / TN / NG (derived dari suffix code di consumer)
  command         LONG,                 -- 0 = matched, 1 = withdrawn (vendor selalu 0; lihat §3.1)
  price           DOUBLE,
  volume          LONG,
  buyer           SYMBOL CAPACITY 256,
  seller          SYMBOL CAPACITY 256,
  buyer_type      SYMBOL CAPACITY 8,    -- F=foreign, D=domestic, '-' kalau tidak ada
  seller_type     SYMBOL CAPACITY 8,
  buyer_order_no  LONG,
  seller_order_no LONG,
  trade_no        LONG
) TIMESTAMP(timestamp)
PARTITION BY HOUR
WAL
DEDUP UPSERT KEYS(timestamp, stock, trade_no);
```

> **Dedup gotcha:** key dulu pernah pakai `(ts, stock, buyer_order_no, seller_order_no)` — tidak jalan untuk auction / single-sided trades karena `order_no = 0`. Sekarang pakai `trade_no` yang konsisten antara Type 15 (live) dan Type 27 (resend).

### Apply via web console

1. Buka `http://10.10.8.10:9000`
2. Login (kalau auth aktif)
3. Paste DDL di SQL editor
4. Klik Run (atau Ctrl+Enter)

### Apply via curl (HTTP API)

```bash
curl -G "http://10.10.8.10:9000/exec" \
  -u "tuai_tan:TuaiTan1407*" \
  --data-urlencode "query=CREATE TABLE IF NOT EXISTS trades (
    timestamp TIMESTAMP,
    stock SYMBOL CAPACITY 2048 INDEX,
    market SYMBOL CAPACITY 8,
    command LONG,
    price DOUBLE,
    volume LONG,
    buyer SYMBOL CAPACITY 256,
    seller SYMBOL CAPACITY 256,
    buyer_type SYMBOL CAPACITY 8,
    seller_type SYMBOL CAPACITY 8,
    buyer_order_no LONG,
    seller_order_no LONG,
    trade_no LONG
  ) TIMESTAMP(timestamp) PARTITION BY HOUR WAL
  DEDUP UPSERT KEYS(timestamp, stock, trade_no)"
```

---

## 3. Penjelasan Kolom

| Kolom             | Tipe            | Sumber (IQPlus Type 15)        | Catatan                                                                |
| ----------------- | --------------- | ------------------------------ | ---------------------------------------------------------------------- |
| `timestamp`       | TIMESTAMP       | Field Date + Time (WIB → UTC)  | Designated timestamp; partition key; bagian DEDUP                      |
| `stock`           | SYMBOL          | Field 0 (Code)                 | Indexed untuk filter `WHERE stock IN (...)`; bagian DEDUP              |
| `market`          | SYMBOL          | _(not in wire)_                | RG / TN / NG — derived dari suffix code di [trade.DeriveMarket()](../../internal/modules/stock/running_trade_consumer/trade/trade.go) |
| `command`         | LONG            | Field 4 (TradeCommand)         | 0 = matched, 1 = withdrawn. Lihat §3.1                                 |
| `price`           | DOUBLE          | Field 5                        | Last trade price                                                       |
| `volume`          | LONG            | Field 6                        | Last trade volume (shares, bukan lot)                                  |
| `buyer`           | SYMBOL          | Field 7 (Buyer)                | `--` saat live, real broker pada resend (Type 27)                      |
| `seller`          | SYMBOL          | Field 9 (Seller)               | sda                                                                    |
| `buyer_type`      | SYMBOL          | Field 8                        | `F` (foreign) / `D` (domestic) / `-` (tidak ada, mis. NG/post-trading) |
| `seller_type`     | SYMBOL          | Field 10                       | sda                                                                    |
| `buyer_order_no`  | LONG            | Field 11                       | Bisa `0` di auction / single-sided trade — JANGAN dipakai untuk DEDUP  |
| `seller_order_no` | LONG            | Field 12                       | sda                                                                    |
| `trade_no`        | LONG            | Field 3                        | Per-stock per-day trade sequence; bagian DEDUP — konsisten Type 15/27  |

### 3.1 Soal kolom `command` (withdrawn flag)

Spec IQPlus v4.0.0 §5.4 mendefinisikan `0 = matched`, `1 = withdrawn`. Tapi di-verifikasi 2026-04-29 lewat scan **8.8 juta+** trade record (1.27M Type 15 di NATS + 1.57M Type 27 di NATS + 6.7M row QuestDB all-time): **vendor IDX/IQPlus tidak pernah emit `command=1`** untuk Type 15 maupun Type 27. Semua row punya `command=0`.

Implikasi:
- Jangan filter `WHERE command = 0` — redundant, dataset memang sudah semua matched.
- Code [questdb.go:Write()](../../internal/modules/stock/running_trade_consumer/sink/questdb.go) menulis semua `command` value tanpa filter (no `IsMatched()` guard di sink), jadi kalaupun suatu hari vendor mulai kirim `1`, akan otomatis terekam.
- `IsMatched()` di [aggregator.go](../../internal/modules/stock/running_trade_consumer/aggregator/aggregator.go) hanya skip withdrawn untuk **bar OHLCV di Redis + broadcast NATS**, bukan untuk QuestDB write.

---

## 4. Verifikasi Setelah Apply

```sql
-- Cek struktur table
SHOW COLUMNS FROM trades;

-- Cek partisi
SELECT * FROM table_partitions('trades');

-- Cek WAL aktif (harus return 1 row)
SELECT * FROM wal_tables() WHERE name = 'trades';

-- Insert test row manual (harus berhasil tanpa error)
-- Column order: timestamp, stock, market, command, price, volume,
--               buyer, seller, buyer_type, seller_type,
--               buyer_order_no, seller_order_no, trade_no
INSERT INTO trades VALUES (
  now(), 'TEST', 'RG', 0, 1000.0, 100,
  '--', '--', 'D', 'D',
  1, 2, 99
);

-- Cleanup test row
DELETE FROM trades WHERE stock = 'TEST';
```

---

## 5. Verifikasi End-to-End (setelah consumer jalan)

```sql
-- Berapa tick masuk total
SELECT count(*) FROM trades;

-- Per-stock 5 menit terakhir
SELECT stock, count(*) AS ticks
FROM trades
WHERE timestamp > dateadd('m', -5, now())
GROUP BY stock
ORDER BY ticks DESC
LIMIT 20;

-- OHLCV 1-menit dari raw tick (tanpa table tambahan!)
SELECT
  timestamp,
  first(price)  AS open,
  max(price)    AS high,
  min(price)    AS low,
  last(price)   AS close,
  sum(volume)   AS volume,
  count(*)      AS trades
FROM trades
WHERE stock = 'BBCA' AND timestamp > dateadd('h', -1, now())
SAMPLE BY 1m
ORDER BY timestamp;
```

`SAMPLE BY` adalah QuestDB feature andalan — OHLCV multi-timeframe bisa
di-derive on-the-fly tanpa pre-compute. Kalau query lambat saat banyak
client, baru pertimbangkan **materialized view** (Phase 2).

---

## 6. Retention — Drop Partisi Lama Otomatis

Topology.md §5.3: "90 hari hot (NVMe), >90 hari archive ke Parquet".

Untuk MVP, cukup drop partisi >90 hari via cron harian:

```sql
-- Drop semua partisi lebih tua dari 90 hari
ALTER TABLE trades DROP PARTITION
  WHERE timestamp < dateadd('d', -90, now());
```

Pasang di cron 03:00 WIB (di luar trading hour):

```bash
# /etc/cron.d/questdb-retention
0 3 * * * questdb curl -s -G "http://localhost:9000/exec" \
  -u "$QDB_USER:$QDB_PASS" \
  --data-urlencode "query=ALTER TABLE trades DROP PARTITION WHERE timestamp < dateadd('d', -90, now())"
```

Untuk Phase 2: tambahkan export ke Parquet sebelum drop (pakai
`COPY trades TO 's3://...' WITH FORMAT PARQUET PARTITION BY DAY`).

---

## 7. Sizing Awal

Estimasi disk untuk planning (per topology.md §8 dan IDX volume):

| Period | Sustained tick | Avg row size | Disk |
| ------ | -------------- | ------------ | ---- |
| 1 jam trading peak | ~10k tick/s | ~80 byte | ~3 GB |
| 1 hari trading (~6 jam) | average ~5k tick/s | ~80 byte | ~9 GB |
| 90 hari hot | — | — | ~600 GB |

Provision NVMe 1 TB minimum untuk margin + WAL overhead.

---

## 8. Index Maintenance

`SYMBOL` columns dengan `INDEX` (kolom `stock`) di-update otomatis. Tidak
ada VACUUM seperti Postgres. Yang perlu monitoring:

- `sys.column_versions_purge_log` — kalau menumpuk, ada index churn
- `sys.transactions` — pending transactions
- Disk free space — alert pada 80%

---

## 9. Mapping ke OHLCV Consumer

Kontrak antara consumer dan schema ada di
[`internal/modules/stock/running_trade_consumer/sink/questdb.go`](../../internal/modules/stock/running_trade_consumer/sink/questdb.go)
fungsi `Write()`. Bila skema berubah:

1. Update DDL di dokumen ini
2. Update `Write()` field list (urutan `Symbol`/`Float64Column`/...)
3. Migrasi data lama kalau perlu (`ALTER TABLE ADD COLUMN` non-destructive)
4. Bump deployment running-trade-consumer

---

## Changelog

| Versi | Tanggal    | Catatan                                                                                                                                                                                |
| ----- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1.0   | 2026-04-27 | Initial — schema `trades` + verification + retention                                                                                                                                   |
| 1.1   | 2026-04-29 | Sync DDL ke production: rename `ts` → `timestamp`, tambah `market` & `command`, dedup keys jadi `(timestamp, stock, trade_no)`. Hapus klaim salah "Command=1 tidak ditulis" — lihat §3.1. |
