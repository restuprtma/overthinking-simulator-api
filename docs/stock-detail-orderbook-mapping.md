# Stock Detail Page — Orderbook UI Data Mapping

**Status**: design / planning — implementation TBD
**Last updated**: 2026-05-08

This document maps every visual element of the typical IDX stock-detail
orderbook page (header / stats grid / bid-ask ladder / done tape) to its
upstream IQPlus record type and the in-stack data source that already
contains it (or needs to be added).

Reference UI screenshot: IMPC live orderbook, full depth chart with
8-level ladder, foreign-buy / sell stats, and recent trade tape.

---

## TL;DR — data sources by section

```
┌──────────────────────────────────────────────────────────────────────┐
│ UI section                Record Type   Source aktif                 │
├──────────────────────────────────────────────────────────────────────┤
│ Header (price + Δ%)       14  Quote     quote:<stock>     (Redis 12) │
│ Stats grid                14  Quote     quote:<stock> + compute      │
│   └── ARA / ARB           (derived)     base_price × IDX tier        │
│   └── Freq                130 Summary   market:summary  (Redis 13)   │
│ Order book ladder         16  Order     orderbook:<stock>:bid/:ask   │
│   (alt source)            18  BestQuote (no consumer yet — orphan)   │
│ Avg / Lot / Avail strip   (computed)    derived from ladder          │
│ Done column (tape)        15  Trade     running_trades QuestDB + WS  │
│ Volume profile (optional) 40  TradeDone tradedone:<stock> (Redis 14) │
└──────────────────────────────────────────────────────────────────────┘
```

All record-type numbers refer to IQPlus
[Data Feed v4.0.0 spec](iqplus/iqplus-data-feed-v4.0.0.md).

---

## 1. Header strip — price & change

```
IMPC   2,380   ↗ 100 (+4.39%)
```

**Source**: Type 14 Quote → `cmd/quote-consumer` →
`HASH quote:<stock>` in Redis DB 12.

| UI field         | Quote FID | Notes |
|------------------|----------:|-------|
| Last price       | 56        | Last traded price |
| Change           | 67        | CHANGE (raw); can also derive `last − prev` |
| % change         | 79        | PCTCHANGE |

```
HGET quote:IMPC 56     # last
HGET quote:IMPC 67     # change
HGET quote:IMPC 79     # pct
```

---

## 2. Stats grid (Open / High / Low / Prev / ARA / ARB / F Buy / F Sell / Lot / Val / Avg / Freq)

| UI field         | Source                   | FID / key            |
|------------------|--------------------------|----------------------|
| Open             | Quote                    | 54                   |
| High             | Quote                    | 57                   |
| Low              | Quote                    | 59                   |
| Prev (close)     | Quote                    | 60 (CLOSE)           |
| **ARA**          | **derived**              | base_price × tier    |
| **ARB**          | **derived**              | base_price × tier    |
| F Buy (value)    | Quote                    | 74 (FRGBOUGHTVAL)    |
| F Sell (value)   | Quote                    | 75 (FRGSOLDVAL)      |
| Lot (total)      | Quote                    | 73 (SHARELOT) or VOL/100 |
| Val (total IDR)  | Quote                    | 66 (XMARKETVAL) or sum |
| Avg              | Quote                    | 78 (AVG)             |
| Freq             | Trading Summary (Type 130) | meta-consumer → `market:summary:0:RG.frequency` |

### IDX auto-rejection (ARA / ARB) tier rules

ARA/ARB tidak ada di feed — derive client-side dari `base_price`
(Quote FID 11) + tier resmi IDX:

| Base price range | ARA % | ARB % |
|------------------|------:|------:|
| `< 200`          | +35%  | −35%  |
| `200 – 5,000`    | +25%  | −25%  |
| `> 5,000`        | +20%  | −20%  |

```
ara = round(base_price × (1 + tier_pct))   // round to nearest fraction
arb = round(base_price × (1 − tier_pct))
```

Fraction (round step) juga punya tier IDX: 1 / 2 / 5 / 10 / 25 rupiah
sesuai harga. Reference: IDX Trading Rule Pasal Auto Rejection.

---

## 3. Order book ladder (8-level bid + 8-level offer)

```
       Bid                  │              Offer
Freq │ Lot  │ Bid (price)   │ Price │ Lot   │ Freq
─────┼──────┼───────────────┼───────┼───────┼─────
   8 │  567 │ 2,360         │ 2,370 │ 1,286 │   9
  26 │  719 │ 2,350         │ 2,380 │ 1,633 │   6
  …  │ …    │ …             │ …     │ …     │ …
─────┴──────┴───────────────┴───────┴───────┴─────
TOTAL  67,616                       126,405
```

**Bagian terpenting** dari halaman ini.

### Opsi A (recommended) — pakai `orderbook-consumer` yang sudah ada

[`internal/modules/stock/orderbook_consumer/`](../internal/modules/stock/orderbook_consumer/)
sudah running, build orderbook real-time per event dari Type 16 (Order
add/cancel) + Type 15 (Trade decrement).

Redis layout:

```
HASH orderbook:<stock>:bid   field="<price>" → JSON {lot, freq, foreign, ...}
HASH orderbook:<stock>:ask   field="<price>" → JSON {lot, freq, foreign, ...}
HASH orderbook:<stock>:_meta { top_bid, top_ask, ... }
```

Frontend tinggal:

```
HGETALL orderbook:IMPC:bid
HGETALL orderbook:IMPC:ask
HGETALL orderbook:IMPC:_meta
```

Sort 8 teratas per side (bid: highest price first, ask: lowest first).

**Pro**: real-time per event, akurat sub-detik, sudah deployed.
**Con**: ~1–2 menit warmup window setelah consumer (re)start sebelum
order book penuh.

### Opsi B (belum jadi) — bikin `bestquote-consumer` dari Type 18

Type 18 (Best Quote) adalah snapshot order-book pre-aggregated dari
vendor. Spec [§5.9](iqplus/iqplus-data-feed-v4.0.0.md#L410-L431):

```
IQP|...|18|<stock>|<B|S>|price1;lot1;#order1;lotF1;#orderF1|price2;...
```

Per level: `price ; lot ; #order ; lot_foreign ; #order_foreign`
(separator: `;` antar field dalam level, `|` antar level).

Kalau dibikin consumer-nya, sink ke:

```
HASH bestquote:<stock>:bid   field="<price>" → JSON
HASH bestquote:<stock>:ask   field="<price>" → JSON
```

Mirror dari `orderbook:*` layout supaya frontend bisa swap source.

**Pro**: vendor-aggregated, beban consumer ringan, foreign breakdown
sudah dipisah.
**Con**: lag detik, less-active stocks bisa missing best bid/ask.

### Keputusan default

**Pakai Opsi A**. Type 18 dibiarkan orphan (atau di-drop di publisher
[`subjects.go:70-71`](../internal/modules/stock/iqplus_publisher/publisher/subjects.go#L70-L71)).
Bestquote-consumer tambahan hanya kalau muncul use case spesifik
(mobile lite client / cross-validation engine).

---

## 4. Avg / Lot / Avail strip (di atas ladder)

```
Avg 2,397.25   Lot 13   Avail 13
```

**Client-side computed** dari isi ladder yang sudah di-render. Tidak
butuh data tambahan dari feed.

```
avg   = Σ(price × lot) / Σ(lot)        // weighted-avg di sisi bid
lot   = jumlah price level visible
avail = sama dengan `lot` di mode default UI
```

UI biasanya cap depth = 13 atau 10 level — `avail` show actual count
yang punya order, `lot` show max-depth setting.

---

## 5. Done column (tape recent trades)

```
5
13
1
0,15
1
12
1.015
…
```

Itu **list lot trade terakhir** running tape.
- `0,15` = 150 lot (Indonesian decimal notation, comma = decimal point)
- `1.015` = 1,015 lot (Indonesian thousands, period = thousands separator)
- Plain `13` = 13 lot

**Source**: Type 15 Trade → `cmd/running-trade-consumer` →
QuestDB `running_trades`.

### Snapshot query (last N)

```sql
SELECT
  to_str(to_timezone(timestamp, 'Asia/Jakarta'), 'HH:mm:ss') AS time,
  price,
  volume / 100 AS lot,
  CASE
    WHEN buyer_order_no > seller_order_no THEN 'Buy'
    WHEN seller_order_no > buyer_order_no THEN 'Sell'
    ELSE 'Cross'
  END AS action
FROM running_trades
WHERE stock = 'IMPC'
  AND timestamp > dateadd('h', -1, now())
ORDER BY timestamp DESC
LIMIT 30;
```

Color coding: `Buy` → green / up arrow, `Sell` → red / down arrow,
`Cross` → neutral. Logic mirror dari
[`cmd/tradedone-consumer/readme.md`](../cmd/tradedone-consumer/readme.md).

### Real-time push

Subscribe NATS subject `idx.trade.IMPC` via `ws-gateway` —
fan-out ke browser WS. Setiap matched trade push satu event.

---

## 6. Volume profile (optional add-on di sisi kanan)

Banyak depth UI menampilkan volume profile chart bersebelahan dengan
ladder. Source: Type 40 Trade Done → `cmd/tradedone-consumer` →
`HASH tradedone:<stock>` (Redis DB 14).

```
HGETALL tradedone:IMPC
# Returns: price → JSON {bvol, svol, bfreq, sfreq, bvol_f, ...}
```

Render sebagai horizontal bar chart aligned by price levels — POC
(point of control) = price dengan total volume tertinggi.

**Bug penting yang sudah dibahas**: TTL refreshed tiap update →
key tidak pernah expire kalau market aktif terus → harga stale dari
hari sebelumnya bisa nyangkut. Mitigation: cron `~/bin/tradedone-reset.sh`
di Redis VM (10.10.8.31), fires 08:30 WIB Mon-Fri, DEL `tradedone:*`.

---

## 7. Backend API yang perlu disediakan

Untuk satu page lengkap, minimal 3 endpoint REST + 1 WS:

```
GET  /core/v1/stock/:code/quote
       → header + stats grid
       → 1 Redis HGETALL quote:<code> + 1 HGET market:summary:0:RG.frequency

GET  /core/v1/stock/:code/orderbook
       → ladder bid + ask 8 level
       → 2 Redis HGETALL (orderbook:<code>:bid, :ask) + 1 HGETALL :_meta
       → response sort + slice top-8 server-side

GET  /core/v1/stock/:code/trades?n=30
       → done tape
       → 1 QuestDB query (running_trades, ORDER BY ts DESC LIMIT n)

WS   /core/v1/stock/:code/ws
       → multiplex push:
           idx.quote.<code>      → header/stats update
           idx.order.<code>      → ladder delta (atau orderbook engine snapshot)
           idx.trade.<code>      → tape append
           idx.tradedone.<code>  → volume profile update (opsional)
```

[`cmd/ws-gateway`](../cmd/ws-gateway/) sudah ada — concept-nya cocok
untuk fan-out. Tinggal wire 4 subject di atas ke client connection.

### Composite endpoint (opsi)

Alternative: satu endpoint `/core/v1/stock/:code/snapshot` yang
combine quote + orderbook + recent trades (top-30) → satu JSON
response. Trade-off: payload lebih besar tapi cuma 1 round-trip
saat first load. Setelah itu subscribe WS untuk delta.

---

## 8. Yang masih perlu dikerjakan

1. **ARA/ARB calculator helper** — `pkg/idx/auto_rejection.go` dengan
   tier table + `Compute(basePrice) (ara, arb int64)`.
2. **Backend endpoint orderbook composite** — combine 3 Redis read
   jadi satu response. Cepat (Redis sub-ms × 3).
3. **Done-tape color logic** — di backend (`action` field di response)
   atau frontend.
4. **WebSocket fanout di ws-gateway** — wire ke 4 subject above per
   stock subscription.
5. **(Opsional)** `bestquote-consumer` kalau butuh fallback / mobile
   lite path.
6. **(Opsional)** Volume profile widget di kanan ladder.

---

## 9. Reference

- IQPlus Data Feed spec: [`docs/iqplus/iqplus-data-feed-v4.0.0.md`](iqplus/iqplus-data-feed-v4.0.0.md)
- Quote consumer: [`cmd/quote-consumer`](../cmd/quote-consumer/) → Redis DB 12
- Orderbook consumer: [`cmd/orderbook-consumer`](../cmd/orderbook-consumer/) → `orderbook:<stock>:*`
- Running trade consumer: [`cmd/running-trade-consumer`](../cmd/running-trade-consumer/) → QuestDB `running_trades` + Redis DB 11 OHLCV bars
- Trade done consumer: [`cmd/tradedone-consumer`](../cmd/tradedone-consumer/) → Redis DB 14
- Meta consumer: [`cmd/meta-consumer`](../cmd/meta-consumer/) → Redis DB 13 (`market:*` keys)
- WebSocket gateway: [`cmd/ws-gateway`](../cmd/ws-gateway/)
- IQPlus publisher subject mapping: [`internal/modules/stock/iqplus_publisher/publisher/subjects.go`](../internal/modules/stock/iqplus_publisher/publisher/subjects.go)
