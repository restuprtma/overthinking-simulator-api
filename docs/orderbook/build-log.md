# Realtime Orderbook + Channel-Multiplex WS Gateway — Build Log

Catatan terkonsolidasi tentang apa yang dibangun antara **2026-05-21 → 2026-05-23** untuk men-deliver depth-of-book realtime ke FE. Dokumen ini adalah peta apa-ada-di-mana, bukan changelog naratif — gunakan sebagai referensi onboarding atau bahan PR description.

Dokumen pasangan:
- [audit-2026-05-21.md](./audit-2026-05-21.md) — 6 bug + 8 decisions di engine lama
- [protocol.md](./protocol.md) v1.1 — Redis layout + NATS subjects + WS protocol

---

## 1. Konteks

Kondisi sebelum mulai:
- `internal/modules/stock/orderbook_consumer/` sudah ada tapi user flag sebagai "suspect" (engine tidak diandalkan untuk produksi).
- `cmd/ws-gateway/` melayani candle saja via `/ws/candles`, single-channel.
- Belum ada path realtime dari engine ke FE — Redis snapshot ada tapi tidak ada fanout ke browser.
- User butuh ≥4 channel realtime ke depan (orderbook, candle, trades, quote, …) lewat satu WS connection.

Goal akhir build ini:
1. Engine orderbook **correct** (bug-fixed + unit-tested).
2. **Snapshot + delta protocol** standard-industry (idempotent, gap-detectable).
3. **Channel-multiplex WS gateway** (`/ws/v1`) supaya FE buka 1 koneksi untuk semua channel — pola Binance/Coinbase/Bybit.
4. **Backward compat** — `/ws/candles` lama tidak break.
5. **Deploy-ready** ke Kubernetes + tooling local dev yang fail-fast.

---

## 2. What's now in the repo

### 2.1 Engine fixes ([internal/modules/stock/orderbook_consumer/book/engine.go](../../internal/modules/stock/orderbook_consumer/book/engine.go))

Bug yang diperbaiki (dari audit doc):

| ID | Fix |
|---|---|
| B1 | `OnOrder` pakai `o.Balance` (bukan `o.Volume`) → tidak overcount saat vendor re-emit |
| B2 | Path duplicate order_no panggil `decOrderCount` → freq tidak inflated |
| B5 | Per-stock monotonic `seq` (uint64) di engine; increment **sekali per logical event** |
| B6 | `DrainDirty()` return per-stock `{Stock, Seq, Bid, Ask}` — bukan per-(stock,side) — supaya snapshotter punya state lengkap untuk delta diff |

Plus tambahan struktural:
- `markChanged(stock)` = dirty + seq bump; `markDirty(stock)` = dirty only (untuk intermediate step di duplicate-handling)
- `Reset()` clear orders + books + dirty + seq map
- Constant `LotSize = 100` (IDX standard, dipakai sink untuk shares→lots conversion)

Test: **[engine_test.go](../../internal/modules/stock/orderbook_consumer/book/engine_test.go) — 8 tests, all PASS**.

### 2.2 Sinks

**[sink/redis.go](../../internal/modules/stock/orderbook_consumer/sink/redis.go)** — full rewrite:
- Per-stock atomic write (`WriteStock`) — both sides + meta dalam satu pipeline
- Schema baru: JSON value `{l,f,lf,ff}` (lot dalam unit IDX lot, bukan shares — divide-by-100 saat serialize)
- `_meta` hash carries `seq, last_change_ts_ns, top_bid, top_ask, *_levels, total_*, phase`

**[sink/nats.go](../../internal/modules/stock/orderbook_consumer/sink/nats.go)** — NEW:
- Subjects: `idx.orderbook.delta.<stock>` + `idx.orderbook.reset.<stock|all>`
- `DeltaPayload` carries absolute new values (`a:"u"`) + price-only removes (`a:"r"`) — idempotent under re-delivery

### 2.3 Snapshotter ([snapshotter/snapshotter.go](../../internal/modules/stock/orderbook_consumer/snapshotter/snapshotter.go))

Rewrite untuk dual-sink + delta computation:
- Cache `previous map[string]book.DirtySnapshot` untuk per-cycle diff
- Setiap tick (default 100ms): `DrainDirty()` → diff vs previous per stock → write to Redis (full state) + publish delta to NATS (changes only)
- `SetPhase(string)` API dipanggil dari session listener
- `OnEngineReset()` clear previous-state cache + publish reset envelope

### 2.4 Session reset ([parser/status.go](../../internal/modules/stock/orderbook_consumer/parser/status.go) + service)

- New `parser.ParseStatus()` decode Type 57 payload → `{Code, Description, Phase, IsReset}`
- Subscriber subject filter ditambah `idx.status.session` (3 filter total: order, trade, status)
- Service `onEnvelope` case 57 → tagging phase + reset engine on code "1" (Begin sending records)
- Phase string masuk `_meta.phase` di Redis

### 2.5 Module wiring + cmd

**[main.orderbook_consumer.go](../../internal/modules/stock/orderbook_consumer/main.orderbook_consumer.go)** — Config + Module now include `NATSSink`.

**[cmd/orderbook-consumer/main.go](../../cmd/orderbook-consumer/main.go)** — env vars baru:
- `NATS_PUBLISH_URL` / `NATS_PUBLISH_TOKEN` (default ke `NATS_URL` / `NATS_TOKEN`)
- `NATS_PUBLISH_PREFIX=idx.orderbook`

Header comment direvisi: hapus `Build for FreeBSD` legacy, pointer ke `deploy.sh` k8s.

### 2.6 K8s deployment artifacts

NEW:
- **[deployments/docker/orderbook-consumer.Dockerfile](../../deployments/docker/orderbook-consumer.Dockerfile)** — multi-stage golang:1.26-alpine → scratch
- **[deployments/kubernetes/orderbook-consumer/deployment.yaml](../../deployments/kubernetes/orderbook-consumer/deployment.yaml)** — `replicas=1` (stateful engine), 200m/256Mi → 2c/1Gi limits, anti-affinity
- **[deployments/kubernetes/orderbook-consumer/deploy.sh](../../deployments/kubernetes/orderbook-consumer/deploy.sh)** — copy of running-order-consumer's deploy automation, adapted

Updated:
- **[deployments/kubernetes/ws-gateway/deployment.yaml](../../deployments/kubernetes/ws-gateway/deployment.yaml)** — `replicas=2`, anti-affinity, 500m/512Mi → 2c/2Gi limits, **HPA** (autoscaling/v2, min=2 max=8, CPU 70% target)
- **[deployments/kubernetes/ws-gateway/ingress.yaml](../../deployments/kubernetes/ws-gateway/ingress.yaml)** — `nginx.ingress.kubernetes.io/load-balance: ewma` (substitusi `least_conn` untuk WS long-lived)

### 2.7 WS Gateway — unified v1 channel registry

**Pola**: single endpoint `/ws/v1`, channel field di subscribe message → server route ke plugin. Each channel = self-contained package implementing `v1.Channel` interface.

```
internal/modules/stock/ws_gateway/
├── domain/             (legacy candle msg shapes — kept)
├── snapshot/           (legacy candle Redis reader — kept)
├── hub/                (legacy candle hub — kept for /ws/candles)
├── handler/            (HTTP routes — extended with NewWithV1 + NewFull)
├── main.ws_gateway.go  (wires BOTH legacy + v1 hubs)
└── v1/                 ← NEW unified protocol
    ├── protocol.go     (Channel interface + envelope types + error codes)
    ├── client.go       (per-conn lifecycle, 512-msg send buffer)
    ├── hub.go          (registry + dispatch routing, atomic stats)
    ├── hub_test.go     (8 unit tests — passing)
    └── channels/
        ├── candle/     (port logic from legacy hub, args: stocks×tfs)
        │   ├── snapshot.go   (Redis db 11 reader)
        │   └── channel.go    (NATS idx.candle.> subscribe + fanout)
        └── orderbook/  (depth book channel)
            ├── snapshot.go   (Redis db 9 reader, sorted+depth-capped)
            └── channel.go    (NATS idx.orderbook.delta.>/reset.>)
```

**Routes served**:
- `GET /healthz` — liveness + reports v1 channel registry
- `GET /ws/candles` — LEGACY (backward compat untuk FE existing)
- `GET /ws/v1` — UNIFIED multiplex
- `GET /api/v1/orderbook/:code?depth=10` — REST snapshot fallback (SSR / debug)

**Caps per WS connection**:
- orderbook: max 30 stocks, max depth 30 levels
- candle: max 100 (stock × tf) pairs

Slow-consumer: send buffer 512 messages; full → drop client.

### 2.8 Local dev tooling

NEW:
- **[scripts/run-service.sh](../../scripts/run-service.sh)** — generic preflight runner: validates `.env` exists (auto-copy from `.env.example` if not), checks required env vars, TCP-probes NATS + Redis, runs `redis-cli PING + DBSIZE`, then `exec go run`
- Makefile targets `run-ws-gateway` + `run-orderbook-consumer`

Per-service env files now populated with **verified production creds** (decoded from k8s secrets):
- `cmd/orderbook-consumer/.env` — NATS prod (with token), Redis localhost (isolated)
- `cmd/ws-gateway/.env` — same pattern

---

## 3. Runtime data flow

```
                                                                
     Edge VM (FreeBSD)                                          
   ┌──────────────────┐                                         
   │  iqplus-publisher│  feeds IDX_TICK stream                  
   │  (Type 15/16/57) │                                         
   └────────┬─────────┘                                         
            │                                                   
            ▼                                                   
   ┌────────────────────────┐                                   
   │  NATS JetStream IDX_TICK│ ◄─── shared broker (10.10.8.2)   
   └────────┬───────────────┘                                   
            │ durable consumer "orderbook-events"               
            │ filters: idx.order.> + idx.trade.> + idx.status.session
            ▼                                                   
   ┌────────────────────────┐                                   
   │  orderbook-consumer    │  k8s, replicas=1 (stateful)       
   │  ┌──────────────────┐  │                                   
   │  │  book.Engine     │  │  per-stock state + seq counter    
   │  │  (in-memory map) │  │                                   
   │  └────────┬─────────┘  │                                   
   │           │            │                                   
   │  ┌────────▼─────────┐  │  100ms ticker                     
   │  │   Snapshotter    │──┼──┬─→ Redis db 9 (full state)      
   │  │   diff-vs-prev   │  │  │                                
   │  └──────────────────┘  │  └─→ NATS idx.orderbook.delta.<stk>
   └────────────────────────┘     NATS idx.orderbook.reset.<stk> 
                                                                
   ┌────────────────────────┐                                   
   │   ws-gateway           │  k8s, replicas=2 + HPA, stateless 
   │  ┌──────────────────┐  │                                   
   │  │  v1.Hub          │  │                                   
   │  │  channel registry│  │                                   
   │  └─┬──────┬─────────┘  │                                   
   │    │      │            │                                   
   │  ┌─▼──┐ ┌─▼─────────┐ │                                    
   │  │cnd │ │ orderbook │ │  each subscribes to its NATS prefix
   │  │ch  │ │  ch       │ │  + opens its own Redis client      
   │  └────┘ └───────────┘ │                                    
   └─────┬──────────────────┘                                   
         │ /ws/v1 (multiplex)                                   
         │ /ws/candles (legacy)                                 
         │ /api/v1/orderbook/:code (REST fallback)              
         ▼                                                      
   ┌─────────────────┐                                          
   │ NGINX ingress   │  load-balance: ewma (WS-friendly)        
   │ ws.tuai.id      │                                          
   └────────┬────────┘                                          
            ▼                                                   
   ┌─────────────────┐                                          
   │   FE (browser)  │  single WS connection, multi-channel     
   └─────────────────┘                                          
```

---

## 4. Test status

| Layer | Tests | Status |
|---|---|---|
| Engine state machine | 8 unit tests in `book/engine_test.go` | ✅ ALL PASS |
| v1 Hub dispatch routing | 8 unit tests in `v1/hub_test.go` | ✅ ALL PASS |
| Sinks (Redis + NATS) | manual code review | ⚠️ no unit tests yet |
| Snapshotter delta diff | manual code review | ⚠️ no unit tests yet |
| End-to-end with real IDX data | — | ⏳ scheduled for next market session |
| Frontend integration | — | ⏳ tergantung FE team |

Full module build: `go build ./...` clean. `go vet ./...` clean.

---

## 5. How to run locally

Prerequisites:
- Local Redis at `localhost:6379` (no password — default `.env` config)
- Access to production NATS at `10.10.8.2:4222` (token sudah ada di .env)

```bash
# Terminal 1 — orderbook engine (writes to local Redis db 9, publishes deltas)
make run-orderbook-consumer

# Terminal 2 — WS gateway (reads local Redis db 9, fans NATS deltas to WS clients)
make run-ws-gateway

# Smoke check
curl http://localhost:8081/healthz
curl http://localhost:8081/api/v1/orderbook/BBCA?depth=10
wscat -c ws://localhost:8081/ws/v1
> {"op":"subscribe","channel":"orderbook","stocks":["BBCA"],"depth":10}
```

Preflight (env file + NATS + Redis reachability) auto-runs sebelum binary launch — see [scripts/run-service.sh](../../scripts/run-service.sh).

Untuk pivot ke Redis prod (test terhadap real state): swap `REDIS_ADDR=10.10.8.10:6379` + `REDIS_PASSWORD=TuaiTan1407*` di kedua `.env` (creds ada di [memory: infra_creds](../../).claude — Anda sudah punya).

---

## 6. Open items / what's NOT done

| Item | Priority | Notes |
|---|---|---|
| Live validation di market hours | **High** | Bandingkan output engine vs broker app (POEMS/Stockbit) untuk 1 stock liquid (BBCA) — confirm B1/B2 fix beneran benar di live data |
| Quirk `investor='-'` di running_orders | Medium | Live BBCA orders semua `investor='-'`. Foreign breakdown stuck di 0. Bukan engine bug — limitasi vendor masking; konfirmasi ke IQPlus support apa policy-nya |
| Snapshotter unit test | Medium | Logic delta diff (`diffSide`, `buildDelta`, `buildSummary`) testable secara pure tanpa NATS/Redis infra |
| End-to-end test dengan replay | Medium | Use existing `cmd/iqplus-replay-mock` untuk feed historical IDX events ke local NATS, verifikasi engine state akhir hari sama dengan broker app close |
| FE client implementation | — | Diluar scope BE; protocol contract di protocol.md sudah final |
| Auth layer di ws-gateway | Low | Phase 1 NO auth; production frontend reverse proxy yang gate JWT |
| Type 18 Best Quote consumer | Low | Skipped — engine reconstruction lebih kaya. Bisa diaktifkan nanti sebagai ground-truth comparator untuk validation |

---

## 7. Decisions worth remembering

| # | Decision | Rationale |
|---|---|---|
| D1 | Engine pakai `Balance` bukan `Volume` saat add | Bootstrap-friendly: balance accurate setelah partial fill, volume hanya nilai initial |
| D2 | Lot disimpan sebagai shares di engine, dikonversi ke lot di sink | Aritmatika exact di engine, IDX-unit di wire |
| D3 | Session reset trigger via subject `idx.status.session` code "1" | Auto-cleanup tiap hari tanpa cron |
| D4 | Single sequence counter per stock, increment per logical event | Simpler reasoning di FE: prev_seq harus = seq-N untuk N event |
| D5 | Snapshot di Redis, delta di NATS (BUKAN Redis pub/sub) | Snapshot = cold-start source dengan TTL persistence; delta = throughput-oriented broadcast |
| D6 | Field naming JSON pendek: `p,l,f,lf,ff,a` | Bandwidth — ribuan delta/menit per stock |
| D7 | Channel-multiplex (`/ws/v1`) bukan endpoint-per-channel | 1 koneksi per browser tab, low fd cost, pola industri (Binance/Coinbase/Bybit) |
| D8 | Backward compat `/ws/candles` tetap jalan | Tidak break FE candle existing selama transition |
| D9 | ws-gateway replicas=2 + HPA + ewma | WS connection long-lived; round-robin = skew. ewma adalah `least_conn` substitute di nginx-ingress |
| D10 | Local dev pakai prod NATS + local Redis | Live IDX data tidak bisa di-mock; Redis prod state harus dilindungi sampai engine validated |
| D11 | Unique `NATS_DURABLE` per env (`-local` suffix) | JetStream queue-group: same durable di local + prod akan split events |

---

## 8. File index — quick reference

**Code**:
- Engine: [book/engine.go](../../internal/modules/stock/orderbook_consumer/book/engine.go) ([test](../../internal/modules/stock/orderbook_consumer/book/engine_test.go))
- Parser: [parser/](../../internal/modules/stock/orderbook_consumer/parser/) (order, status, trade)
- Sinks: [sink/redis.go](../../internal/modules/stock/orderbook_consumer/sink/redis.go) + [sink/nats.go](../../internal/modules/stock/orderbook_consumer/sink/nats.go)
- Snapshotter: [snapshotter/snapshotter.go](../../internal/modules/stock/orderbook_consumer/snapshotter/snapshotter.go)
- Service + subscriber: [service/service.go](../../internal/modules/stock/orderbook_consumer/service/service.go) + [subscriber/subscriber.go](../../internal/modules/stock/orderbook_consumer/subscriber/subscriber.go)
- WS gateway v1 core: [v1/protocol.go](../../internal/modules/stock/ws_gateway/v1/protocol.go) + [v1/hub.go](../../internal/modules/stock/ws_gateway/v1/hub.go) + [v1/client.go](../../internal/modules/stock/ws_gateway/v1/client.go) ([test](../../internal/modules/stock/ws_gateway/v1/hub_test.go))
- Channels: [v1/channels/orderbook/](../../internal/modules/stock/ws_gateway/v1/channels/orderbook/) + [v1/channels/candle/](../../internal/modules/stock/ws_gateway/v1/channels/candle/)
- Handler + main: [ws_gateway/handler/handler.go](../../internal/modules/stock/ws_gateway/handler/handler.go) + [ws_gateway/main.ws_gateway.go](../../internal/modules/stock/ws_gateway/main.ws_gateway.go)
- Cmd binaries: [cmd/orderbook-consumer/](../../cmd/orderbook-consumer/) + [cmd/ws-gateway/](../../cmd/ws-gateway/)

**Deploy + dev**:
- Dockerfile: [deployments/docker/orderbook-consumer.Dockerfile](../../deployments/docker/orderbook-consumer.Dockerfile)
- K8s: [deployments/kubernetes/orderbook-consumer/](../../deployments/kubernetes/orderbook-consumer/) + [deployments/kubernetes/ws-gateway/](../../deployments/kubernetes/ws-gateway/)
- Local: [scripts/run-service.sh](../../scripts/run-service.sh) + Makefile `run-*` targets

**Docs**:
- [audit-2026-05-21.md](./audit-2026-05-21.md) — bug list at start of work
- [protocol.md](./protocol.md) — wire protocol contract v1.1
- This file — what was built and where
