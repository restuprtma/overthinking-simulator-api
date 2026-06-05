# Orderbook — Protocol & Data Contract

Sumber tunggal kebenaran untuk:
- Bentuk data orderbook di Redis db 9 (cold-start source)
- Subject NATS `idx.orderbook.*` (realtime fanout)
- WebSocket message protocol antara ws-gateway dan FE
- REST snapshot endpoint shape

Versi: v1 (2026-05-21). Backward-incompatible perubahan harus bump versi major + dokumentasikan di section "Changelog" bawah.

---

## Konsep umum

```
┌────────────┐   Type 16   ┌────────────────┐  delta+snap   ┌──────────────┐
│   IQPlus   ├────────────►│  orderbook-    ├─────NATS─────►│  ws-gateway  │
│ (Type 15+  │   Type 15   │   consumer     │               │              │
│  16 stream)│             │  (engine)      │  full state   │              │
└────────────┘             └────────┬───────┘  Redis db 9   └──────┬───────┘
                                    │                              │
                                    │ snapshot                     │ WS
                                    ▼                              ▼
                              ┌──────────┐                  ┌──────────┐
                              │ Redis db │◄─cold-start──────┤ FE Client│
                              │   9      │  via WS          │          │
                              └──────────┘                  └──────────┘
                                                                  ▲
                                                            REST GET (fallback)
```

**Sumber kebenaran**:
- In-memory engine = live truth
- Redis db 9 = persisted snapshot (untuk cold-start ws-gateway saat subscribe atau API GET)
- NATS subject = delivery channel ke ws-gateway

---

## 1. Redis Layout (db 9)

```
HASH  orderbook:<stock>:bid       field=<price-str>  value=JSON LevelStats
HASH  orderbook:<stock>:ask       field=<price-str>  value=JSON LevelStats
HASH  orderbook:<stock>:_meta     fields lihat di bawah
```

### `LevelStats` JSON (value di field hash bid/ask)

```json
{ "l": 11418, "f": 117, "lf": 0, "ff": 0 }
```

| Field | Type | Arti |
|---|---|---|
| `l`  | int64 | total lot di level harga ini (lot = 1 = 100 shares untuk most IDX stocks) |
| `f`  | int64 | jumlah order aktif di level (frequency) |
| `lf` | int64 | lot foreign (investor=F) — bisa 0 untuk data masked |
| `ff` | int64 | freq foreign — bisa 0 untuk data masked |

Catatan: di engine internal disimpan dalam **shares**; serializer ke JSON divide-by-100 → output dalam **lot**. Lot size default 100 (`pkg/idx.LotSize`); future per-stock override jika ada non-standard lots.

### `_meta` hash fields

```
seq                 monotonic per-stock counter (string, parsed as int64)
last_change_ts_ns   nanosecond unix ts of last mutation
last_flush_ts_ns    nanosecond unix ts dari snapshotter cycle terakhir
top_bid             best bid price (string)
top_ask             best ask price (string)
bid_levels          number of price levels currently on bid side
ask_levels          idem ask
total_bid_lot       Σ lot all bid levels
total_ask_lot       Σ lot all ask levels
total_bid_freq      Σ freq
total_ask_freq      idem ask
phase               trading phase: "preopen" | "open" | "preclose" | "post" | "closed" | "unknown"
```

TTL: 25 jam (per session); auto-refresh setiap snapshotter cycle.

### Contoh read (FE atau ws-gateway)

```bash
# Top of book (best bid + best ask)
redis-cli -n 9 HMGET orderbook:BBCA:_meta top_bid top_ask seq

# Full depth, sort di client side karena HASH tidak preserve order
redis-cli -n 9 HGETALL orderbook:BBCA:bid   # → [price, JSON, price, JSON, ...]
redis-cli -n 9 HGETALL orderbook:BBCA:ask
```

---

## 2. NATS Subject Layout

```
idx.orderbook.snapshot.<stock>     full state, periodic (5s heartbeat) + on-demand
idx.orderbook.delta.<stock>        incremental, throttled 100ms
idx.orderbook.reset.<stock>        engine reset signal (session restart / midnight)
```

Stream: `IDX_BOOK` (recommend separate stream agar retention berbeda dari `IDX_TICK`). Retention: 1 jam, MaxBytes 500MB. Tujuan: ws-gateway sebagai consumer bisa replay last second saja jika reconnect.

### Snapshot payload

```json
{
  "type": "snapshot",
  "stock": "BBCA",
  "seq": 12345,
  "ts": 1716284123456,
  "phase": "open",
  "bids": [{"p":1480,"l":11418,"f":117,"lf":0,"ff":0}, ...],
  "asks": [{"p":1485,"l":4333, "f":4,  "lf":0,"ff":0}, ...],
  "summary": {
    "top_bid": 1480, "top_ask": 1485,
    "total_bid_lot": 211240, "total_ask_lot": 404202,
    "total_bid_freq": 1240,  "total_ask_freq": 3328
  }
}
```

- `bids` sorted descending by price; `asks` sorted ascending.
- Server-capped at 30 levels per side (FE bisa minta `depth=10` di subscribe — server crop pada saat kirim).

### Delta payload

```json
{
  "type": "delta",
  "stock": "BBCA",
  "seq": 12346,
  "prev_seq": 12345,
  "ts": 1716284123556,
  "bids": [
    {"p":1465, "l":120475, "f":698, "a":"u"},
    {"p":1460, "a":"r"}
  ],
  "asks": [
    {"p":1505, "l":7346, "f":27,  "a":"u"}
  ],
  "summary": { ... }   // optional — kirim setiap N delta atau setiap perubahan top-of-book
}
```

Field `a` (action):
- `"u"` = upsert (set level to absolute values di `l`, `f`, `lf`, `ff`)
- `"r"` = remove (price level hilang dari book; `l/f` absent)

**Idempotent**: apply 2× = same state. FE bisa apply duplikat tanpa korup.

### Reset payload

```json
{ "type": "reset", "stock": "BBCA", "ts": ..., "reason": "session_begin" }
```

FE harus clear local state dan tunggu snapshot berikutnya.

---

## 3. WebSocket Protocol (FE ↔ ws-gateway)

Endpoint: `wss://ws.tuai.id/ws/v1` — **unified channel-multiplexed** WebSocket. One connection per browser tab; FE subscribes to N channels (orderbook, candle, future: trades, quote, …) over the same socket. See [internal/modules/stock/ws_gateway/v1/protocol.go](../../internal/modules/stock/ws_gateway/v1/protocol.go).

> **Legacy**: `wss://ws.tuai.id/ws/candles` is still served (candle-only, old protocol shape) so existing FE candle clients don't break. New code MUST target `/ws/v1`.

### Lifecycle

1. FE opens WS to `/ws/v1`.
2. Server immediately sends `{"type":"hello","server":"<podname>","ts":...}`.
3. FE sends subscribe per channel (see below).
4. For channels with snapshot semantics (orderbook, candle), server reads Redis and emits a `snapshot` message; thereafter live `delta` (orderbook) / `update` (candle) messages stream from NATS fan-out.
5. Heartbeat: server pings every 30s via WS control frame. FE can also send application-level `{"op":"ping"}`.

### Client → Server (envelope shared across all channels)

```jsonc
// orderbook
{ "op":"subscribe",   "channel":"orderbook", "stocks":["BBCA","BMRI"], "depth":10 }
{ "op":"unsubscribe", "channel":"orderbook", "stocks":["BBCA"] }
{ "op":"resync",      "channel":"orderbook", "stocks":["BBCA"] }

// candle (cross-product: stocks × tfs)
{ "op":"subscribe",   "channel":"candle", "stocks":["BBCA"], "tfs":["1m","5m"] }
{ "op":"unsubscribe", "channel":"candle", "stocks":["BBCA"], "tfs":["1m"] }
{ "op":"resync",      "channel":"candle", "stocks":["BBCA"], "tfs":["1m"] }

// application-level keepalive (independent of WS control ping)
{ "op":"ping" }
```

### Server → Client

```jsonc
// session
{ "type":"hello", "server":"ws-gateway-7c8d", "ts":1716284123456 }
{ "type":"pong",  "ts":1716284123456 }
{ "type":"error", "code":"UNKNOWN_CHANNEL"|"BAD_JSON"|"BAD_ARGS"|"TOO_MANY_SUBSCRIPTIONS"|"SNAPSHOT_FAIL"|"UNKNOWN_OP",
                  "msg":"...", "channel":"orderbook" }

// orderbook channel
{ "type":"snapshot", "channel":"orderbook", "stock":"BBCA",
  "seq":12345, "ts":1716284123456, "phase":"open",
  "bids":[{"p":1480,"l":11418,"f":117,"lf":0,"ff":0}, ...],
  "asks":[...],
  "summary":{"top_bid":1480,"top_ask":1485,"total_bid_lot":211240,...} }

{ "type":"delta", "channel":"orderbook", "stock":"BBCA",
  "seq":12346, "prev_seq":12345, "ts":1716284123556,
  "bids":[{"p":1465,"l":120475,"f":698,"a":"u"}, {"p":1460,"a":"r"}],
  "asks":[...],
  "summary":{...} }

{ "type":"reset", "channel":"orderbook", "stock":"BBCA",
  "reason":"session_1", "ts":... }

// candle channel
{ "type":"snapshot", "channel":"candle", "stock":"BBCA", "tf":"1m",
  "bar":{"o":1480,"h":1500,"l":1475,"c":1490,"v":5000,
         "open_ts":...,"close_ts":...,"status":"live"} }

{ "type":"update", "channel":"candle", "stock":"BBCA", "tf":"1m",
  "bar":{...,"status":"live"|"closed"} }
```

### FE algorithm (per channel)

```
on connect:
    open WS to /ws/v1, await hello
    for each (channel, args) the user wants:
        send {op:"subscribe", channel, ...args}
        expect snapshot → store {seq, state}, render

    on delta (orderbook):
        if msg.prev_seq != local[stock].seq:
            send {op:"resync", channel:"orderbook", stocks:[stock]}
            drop incoming deltas until next snapshot
            return
        apply delta to local state, bump local[stock].seq
        render

    on update (candle):
        replace/upsert bar in local cache, render

    on reset:
        clear local state for the affected stock, await snapshot

on disconnect:
    exponential backoff reconnect (1s → 30s, max)
    on reconnect: replay every active subscribe — server starts each fresh
```

### Caps & rate limit (Phase 1 defaults)

Per channel, per WS connection:

| Channel | Max subscriptions | Max depth |
|---|---|---|
| orderbook | 30 stocks | 30 levels |
| candle | 100 (stock × tf) pairs | — |

Slow-consumer policy: send buffer is **512 messages**. Full buffer → server drops the message and logs; sustained pressure → server unregisters the client (FE reconnects). Realtime market data prioritizes freshness over delivery guarantee.

---

## 4. REST Snapshot Endpoint (fallback)

`GET https://ws.tuai.id/api/v1/orderbook/:code?depth=10`

Served by the same ws-gateway pod, reads the same Redis db 9. See [internal/modules/stock/ws_gateway/handler/handler.go](../../internal/modules/stock/ws_gateway/handler/handler.go) (`orderbookSnapshot`).

Use case:
- Server-side rendering (Next.js `getServerSideProps`)
- Client cold-start / debug / quick lookup
- `curl` smoke check during deployment validation

### Response

`200 OK` — same shape as the WS snapshot message:
```json
{
  "type": "snapshot",
  "channel": "orderbook",
  "stock": "BBCA",
  "seq": 12345,
  "ts": 1716284123456,
  "phase": "open",
  "bids": [{"p":1480,"l":11418,"f":117,"lf":0,"ff":0}, ...],
  "asks": [...],
  "summary": {...}
}
```

`404 Not Found` — stock has no state in Redis (engine never saw it / TTL expired).
`400 Bad Request` — missing/empty `:code`.
`502 Bad Gateway` — Redis read failed.

`Cache-Control: public, max-age=1` — 1s edge cache absorbs poll storms cheaply without making the data noticeably stale.

---

## 5. Sequence number semantics

- Engine maintains `seq[stock] atomic.Uint64`. Increment by 1 setiap perubahan state (add/cancel/fill yang affecting that stock).
- Snapshot serialize `seq` as monotonic value (post-flush).
- Delta carries `seq + prev_seq`. `seq - prev_seq` bisa > 1 (banyak event sebelum throttle), tetap dianggap valid delta (state setelah delta = `seq`).
- Reset → `seq` reset ke 1.

Gap detection di FE:
- Bila terima delta `prev_seq != local.seq` → gap → resync.
- Bila terima snapshot `seq < local.seq` → stale snapshot dari Redis lag → discard (jarang terjadi tapi mungkin).

---

## Changelog

- 2026-05-21 v1.1: WebSocket endpoint pivoted to **unified `/ws/v1`** with channel-registry pattern (per-channel plugins, single connection multiplexes orderbook + candle + future channels). REST snapshot path corrected to `/api/v1/orderbook/:code`. Legacy `/ws/candles` retained for backward compat.
- 2026-05-21 v1.0: initial draft. Lihat [audit-2026-05-21.md](./audit-2026-05-21.md) untuk konteks.
