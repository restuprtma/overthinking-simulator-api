# QuestDB — Snapshot Struktur Tabel

> Sumber: live introspection via HTTP `/exec` di `10.10.8.51:9000` pada 2026-05-21.
> Cara reproduce di bagian akhir dokumen.
>
> Dokumen ini adalah snapshot apa adanya — untuk DDL setup awal lihat
> [schema.md](schema.md). Untuk query patterns lihat [queries.md](queries.md).

12 tabel total: 1 lookup, 5 candles agregat, 2 NBS snapshot, 2 order log, 2 trade log.

| Tabel | Partition | Designated TS | WAL | DEDUP | Use case |
|---|---|---|---|---|---|
| `brokers` | NONE | — | ❌ | ❌ | Lookup `code` → `broker_name` |
| `candles_1m` | DAY | `timestamp` | ✅ | ❌ | OHLCV 1m bars |
| `candles_5m` | DAY | `timestamp` | ✅ | ❌ | OHLCV 5m bars |
| `candles_15m` | DAY | `timestamp` | ✅ | ❌ | OHLCV 15m bars |
| `candles_1h` | MONTH | `timestamp` | ✅ | ❌ | OHLCV 1h bars |
| `candles_4h` | MONTH | `timestamp` | ✅ | ❌ | OHLCV 4h bars |
| `nbs_stock` | DAY | `timestamp` | ✅ | ✅ | Net Buy/Sell snapshot per stock |
| `nbs_broker` | DAY | `timestamp` | ✅ | ✅ | Net Buy/Sell snapshot per broker |
| `orders` | DAY | `timestamp` | ✅ | ❌ | Post-close resend orders (broker codes real) |
| `running_orders` | DAY | `timestamp` | ✅ | ❌ | Live order events (broker codes masked) |
| `running_trades` | HOUR | `timestamp` | ✅ | ✅ | Live trades (broker codes masked) |
| `trades` | HOUR | `timestamp` | ✅ | ✅ | Post-close resend trades (broker codes real) |

Legend untuk kolom: `*` = designated timestamp, `+` = upsert key, `idx` = indexed SYMBOL, `cached` = symbol table cached.

---

## 1. `brokers`

Static lookup, ~89 baris. Tanpa timestamp, tanpa WAL, tanpa partisi.

| Column | Type | Notes |
|---|---|---|
| `code` | `SYMBOL` | `idx` — broker code (2 huruf), e.g. `AK`, `YP` |
| `broker_name` | `STRING` | Nama lengkap broker |

---

## 2. Candle tables — `candles_1m`, `candles_5m`, `candles_15m`, `candles_1h`, `candles_4h`

Skema identik untuk kelima timeframe. Sumber: aggregator di `cmd/running-trade-consumer`. Partition `DAY` untuk TF kecil (1m/5m/15m) dan `MONTH` untuk TF besar (1h/4h) — trade-off antara jumlah partisi dan ukuran per-partisi.

| Column | Type | Notes |
|---|---|---|
| `timestamp` | `TIMESTAMP` | `*` bucket start (UTC) |
| `stock` | `SYMBOL` | cap 4096, ~1782 unique ticker |
| `market` | `SYMBOL` | RG / NG / TN |
| `open` | `DOUBLE` | |
| `high` | `DOUBLE` | |
| `low` | `DOUBLE` | |
| `close` | `DOUBLE` | |
| `volume` | `LONG` | Shares |
| `value` | `DOUBLE` | Rupiah (price × volume sum) |
| `freq` | `LONG` | Trade count dalam bucket |

---

## 3. `nbs_stock` & `nbs_broker`

Skema identik. Vendor IDX kirim dua view: Type 58 (per stock) → `nbs_stock`, Type 59 (per broker) → `nbs_broker`. DEDUP UPSERT KEYS `(timestamp, stock, broker)` — timestamp di-truncate ke UTC midnight (day-bucket) sehingga snapshot kumulatif harian collapse ke satu baris per `(day, stock, broker)`. Detail lihat [internal/modules/stock/nbs_consumer/sink/questdb.go](../../internal/modules/stock/nbs_consumer/sink/questdb.go).

| Column | Type | Notes |
|---|---|---|
| `timestamp` | `TIMESTAMP` | `*` `+` day-bucket UTC midnight |
| `stock` | `SYMBOL` | `+` `idx` cached, cap 4096 |
| `broker` | `SYMBOL` | `+` `idx` cached, cap 256 |
| `market` | `SYMBOL` | cached, RG/NG/TN |
| `b_freq` | `LONG` | Buy frequency |
| `b_vol` | `LONG` | Buy volume (shares) |
| `b_lot` | `LONG` | Buy lot |
| `b_val` | `LONG` | Buy value (rupiah) |
| `b_pct` | `DOUBLE` | Buy percentage |
| `s_freq` | `LONG` | Sell frequency |
| `s_vol` | `LONG` | Sell volume |
| `s_lot` | `LONG` | Sell lot |
| `s_val` | `LONG` | Sell value |
| `s_pct` | `DOUBLE` | Sell percentage |
| `sequence` | `LONG` | Envelope sequence (audit) |
| `date` | `STRING` | Vendor raw `YYYYMMDD` (WIB) |
| `time` | `STRING` | Vendor raw `HHMMSS` (WIB) |
| `last_updated_at` | `TIMESTAMP` | Receive time UTC, snapshot precision |

---

## 4. `orders` & `running_orders`

Skema identik. `running_orders` = Type 16 live events (broker masked `--`), `orders` = Type 26 resend post-close (broker code real). Keduanya tanpa DEDUP saat ini meski `command` includes cancel events — verify dengan tim consumer kalau intent dedup berubah.

| Column | Type | Notes |
|---|---|---|
| `stock` | `SYMBOL` | cached, cap 4096 |
| `market` | `SYMBOL` | cached, RG/NG/TN |
| `broker` | `SYMBOL` | cached, `--` di running_orders, real di orders |
| `investor` | `SYMBOL` | cached, `F` / `D` |
| `command` | `LONG` | 0=bid, 1=offer, 2=cancel-bid, 3=cancel-offer |
| `order_no` | `LONG` | |
| `price` | `DOUBLE` | |
| `volume` | `LONG` | |
| `balance` | `LONG` | Remaining order size (vendor-reported) |
| `no_reference` | `LONG` | |
| `timestamp` | `TIMESTAMP` | `*` UTC |
| `date` | `STRING` | Vendor raw |
| `time` | `STRING` | Vendor raw |

> Catatan: `running_orders` masih kosong dedup walaupun consumer doc menyebut intent `DEDUP UPSERT KEYS(timestamp, stock, order_no, command)`. Worth cross-check (lihat [internal/modules/stock/resend_order_consumer/sink/questdb.go](../../internal/modules/stock/resend_order_consumer/sink/questdb.go)).

---

## 5. `running_trades` & `trades`

Skema identik. `running_trades` = Type 15 live ticks (buyer/seller broker = `--`), `trades` = Type 27 resend (broker code real, mid-day + post-close batch). Keduanya pakai `DEDUP UPSERT KEYS(timestamp, stock, trade_no)` sehingga resend overwrite live row di tempat. Detail strategi dedup → [memory: trades dedup key](../../README.md) (catatan: live `running_trades` saat ini tetap pakai `--` masked, broker real hanya muncul di `trades`).

| Column | Type | Notes |
|---|---|---|
| `timestamp` | `TIMESTAMP` | `*` `+` UTC |
| `stock` | `SYMBOL` | `+` `idx` cached, cap 4096 |
| `market` | `SYMBOL` | cached, cap 256 (`trades`) / 256 (`running_trades`) — RG/NG/TN |
| `command` | `LONG` | 0=matched, 1=withdrawn |
| `price` | `DOUBLE` | |
| `volume` | `LONG` | |
| `buyer` | `SYMBOL` | cached, `--` di running, real di trades (~95 unique) |
| `seller` | `SYMBOL` | cached, idem |
| `buyer_type` | `SYMBOL` | cached, `F` / `D` |
| `seller_type` | `SYMBOL` | cached, `F` / `D` |
| `buyer_order_no` | `LONG` | |
| `seller_order_no` | `LONG` | |
| `trade_no` | `LONG` | `+` Vendor unique trade ID |
| `date` | `STRING` | Vendor raw |
| `time` | `STRING` | Vendor raw |

---

## Cara reproduce

Daftar tabel + metadata:

```bash
curl -sS -u "$QUESTDB_AUTH_USER:$QUESTDB_AUTH_PASSWORD" \
  -G 'http://10.10.8.51:9000/exec' \
  --data-urlencode "query=SELECT table_name, designatedTimestamp, partitionBy, dedup, walEnabled FROM tables() ORDER BY table_name"
```

Describe satu tabel:

```bash
curl -sS -u "$QUESTDB_AUTH_USER:$QUESTDB_AUTH_PASSWORD" \
  -G 'http://10.10.8.51:9000/exec' \
  --data-urlencode 'query=SHOW COLUMNS FROM trades'
```

Atau via Go pakai shared client baru:

```go
import "tuai/pkg/questdb"

cfg, _ := questdb.LoadFromEnv("")
qc := questdb.NewQueryClient(cfg)
resp, _ := qc.Exec(ctx, "SHOW COLUMNS FROM trades")
```
