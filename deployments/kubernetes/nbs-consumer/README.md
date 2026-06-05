# nbs-consumer — Kubernetes Deploy

Deploy of [`cmd/nbs-consumer`](../../../cmd/nbs-consumer/), namespace `tuai`.
Subscribes to `idx.nbs.>` on JetStream `IDX_META` and writes every
Net Buy/Sell snapshot to QuestDB:

- **Type 58 (NBS Stock)** → table `nbs_stock`
- **Type 59 (NBS Broker)** → table `nbs_broker`

Both tables share the same column layout (split keeps the vendor's two
semantic views queryable as independent time series).

History: an earlier design wrote to Redis as a dual hash view
(`nbs:stock:<stock>` × broker, `nbs:broker:<broker>` × stock) but the
consumer was never deployed — the `nbs-aggregator` durable accumulated
~21M backlog without delivering. Moving to QuestDB aligns with the
broker-summary archive design noted in
[`stock` schema migration 000001](../../../internal/database/migrations/stock/000001_prices_daily.up.sql)
and lets analytics queries reuse the same store as `trades`.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Deployment | `tuai-be-nbs-consumer` |
| Image | `venturoid/tuai-be-nbs-consumer:<tag>` (Docker Hub, [`nbs-consumer.Dockerfile`](../../docker/nbs-consumer.Dockerfile)) |
| Replicas | **1** (no DEDUP yet, see §"Tables required") |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| QuestDB | `10.10.8.51:9000` (basic auth, **direct — bypass HAProxy**) |

### Subjects subscribed

`idx.nbs.>` from stream `IDX_META`:
- `idx.nbs.stock.<stockcode>` — Type 58 (stock-centric snapshot)
- `idx.nbs.broker.<brokercode>` — Type 59 (broker-centric snapshot)

Both record types carry the same 12 numeric fields per (stock, broker)
pair; only the order of the first two identifier fields differs (see
[parser/parser.go](../../../internal/modules/stock/nbs_consumer/parser/parser.go)).

---

## Tables required in QuestDB

Auto-create via ILP works but produces a sub-optimal schema (no INDEX,
no PARTITION). Create both tables explicitly **before** the pod starts:

```sql
CREATE TABLE 'nbs_stock' (
    timestamp       TIMESTAMP,
    stock           SYMBOL INDEX CAPACITY 1024,
    broker          SYMBOL INDEX CAPACITY 256,
    market          SYMBOL,
    b_freq          LONG,
    b_vol           LONG,
    b_lot           LONG,
    b_val           LONG,
    b_pct           DOUBLE,
    s_freq          LONG,
    s_vol           LONG,
    s_lot           LONG,
    s_val           LONG,
    s_pct           DOUBLE,
    sequence        LONG,
    date            STRING,
    time            STRING,
    last_updated_at TIMESTAMP
) timestamp(timestamp) PARTITION BY DAY
DEDUP UPSERT KEYS(timestamp, stock, broker);

CREATE TABLE 'nbs_broker' (
    -- same shape as nbs_stock
) timestamp(timestamp) PARTITION BY DAY
DEDUP UPSERT KEYS(timestamp, stock, broker);
```

`timestamp` = UTC midnight of trading day (day-bucket; derived from
`envelope.ReceivedAt` via `time.Truncate(24h)` in the sink). This makes
DEDUP UPSERT KEYS work — same (day, stock, broker) overwrites in place,
keeping only the latest cumulative state. Actual snapshot precision is
preserved in `last_updated_at`.

`stock` stores the full vendor code **including board suffix** (e.g.
`WBSANG` = `WBSA` on NG board). `market` is the derived suffix
(`RG`/`NG`/`TN`) — same convention as the `trades` table, computed
via [`trade.DeriveMarket`](../../../internal/modules/stock/running_trade_consumer/trade/).

`date` + `time` preserve vendor's raw wall-clock fields (envelope.Date
"20260518" and envelope.Time "171517") for display and audit — useful
when comparing "what time vendor claims it emitted" vs "what time we
actually received" (= `last_updated_at`).

`last_updated_at` mirrors `timestamp` for now (both = publisher
`ReceivedAt` UTC). It will diverge if we later enable DEDUP UPSERT
KEYS — then `timestamp` becomes the dedup bucket (truncated to day)
and `last_updated_at` tracks actual latest receive time.

**DEDUP UPSERT KEYS(timestamp, stock, broker)** enables ~95× storage
compression vs raw time-series. Vendor emits many cumulative snapshots
per (stock, broker) per session (observed ~50-200/pair/day); since
`timestamp` is day-bucket, all snapshots collapse into one row and the
latest INSERT wins. NBS counters are monotonically increasing during a
session, so latest = correct final state.

**Trade-off**: lost intra-day evolution (cannot answer "kapan broker X
mulai akumulasi stock Y siang tadi"). For audit needs, `last_updated_at`
records when the most recent snapshot was received — but only that
single point, not the full timeline.

### Query patterns

```sql
-- Latest cumulative NBS per (stock, broker) today
SELECT stock, broker, market, b_vol, b_val, s_vol, s_val
FROM nbs_stock
WHERE timestamp >= today()
LATEST ON timestamp PARTITION BY stock, broker;

-- All brokers active on BBYB (regular board) today
SELECT broker, b_val, s_val
FROM nbs_stock
WHERE stock = 'BBYB' AND market = 'RG' AND timestamp >= today()
LATEST ON timestamp PARTITION BY broker
ORDER BY (b_val + s_val) DESC;

-- All stocks broker PD trades today (any board)
SELECT stock, market, b_val, s_val
FROM nbs_broker
WHERE broker = 'PD' AND timestamp >= today()
LATEST ON timestamp PARTITION BY stock
ORDER BY (b_val + s_val) DESC;

-- Aggregate across boards: broker PD total buy volume per base ticker today
-- (strips 2-char suffix; works for RG/NG/TN with 4+2 char codes)
SELECT
    CASE WHEN market != 'RG' THEN substring(stock, 1, length(stock)-2) ELSE stock END AS base_ticker,
    sum(b_vol) AS total_b_vol
FROM nbs_broker
WHERE broker = 'PD' AND timestamp >= today()
LATEST ON timestamp PARTITION BY stock
GROUP BY base_ticker
ORDER BY total_b_vol DESC;
```

---

## Files

| File | Purpose |
|---|---|
| `README.md` | This document |
| `secret.yaml.example` | Env Secret template (committed) |
| `secret.yaml` | Real Secret with credentials (**gitignored**) |
| `deployment.yaml` | Workload spec |
| `deploy.sh` | Build + validate + deploy automation (matches sibling consumers) |

---

## First-time deploy

```bash
# 1. Create the QuestDB tables (run once)
#    Web Console at http://10.10.8.51:9000 — paste the CREATE TABLE
#    statements from §"Tables required in QuestDB" above.

# 2. Apply secret (only first time, or when rotating)
cp secret.yaml.example secret.yaml
# Edit secret.yaml: fill NATS_TOKEN and QUESTDB_AUTH_PASSWORD
kubectl --kubeconfig ../production.yaml apply -f secret.yaml -n tuai

# 3. Deploy via deploy.sh (handles git push, autobuild wait, kubectl apply,
#    rollout, and post-deploy verification)
./deploy.sh 1.0.0

# 4. Watch initial drain (20M+ backlog from dead consumer days)
kubectl --kubeconfig ../production.yaml logs -n tuai \
  -l app.kubernetes.io/name=tuai-be-nbs-consumer -f
```

## Subsequent deploys

```bash
# Bump tag and roll
./deploy.sh 1.0.1

# Rollback to previous tag (no git push, just re-deploy older image)
./deploy.sh 1.0.0 --no-build
```

## Rollback

```bash
kubectl --kubeconfig ../production.yaml delete deploy tuai-be-nbs-consumer -n tuai
# Backlog stays in NATS — durable `nbs-aggregator` survives pod restart.
# Re-deploy resumes from last ack'd sequence.
```

## Operational notes

### Initial drain expectation

When first deployed, the consumer will start at the durable's
`ack_floor` (probably sequence 1 since `Last Delivery: never` historically).
Stream IDX_META has ~5.7M NBS messages on edge plus the main mirror —
expect a few minutes of intense write to QuestDB on cold start. Monitor:

```bash
kubectl logs -l app.kubernetes.io/name=tuai-be-nbs-consumer -n tuai -f \
  | grep "nbs consumer stats"
```

Look for `rate_per_sec` ramping to several thousand, then dropping to
the live rate (~tens/sec) once caught up.

### Verifying the dual-table split works

After 1 minute of runtime:

```sql
SELECT count(*) FROM nbs_stock;   -- Type 58 count
SELECT count(*) FROM nbs_broker;  -- Type 59 count
```

Both should be growing. Roughly equal numbers — vendor sends both
record types throughout the session at similar cadence.

### Why not Redis

Original design (see git log) used Redis dual-hash projection for O(1)
broker-summary lookup. That works but:
- Was never deployed (consumer dead since 27 April)
- Duplicates data that's derivable from `trades` table aggregation
- Requires extra cache invalidation on EOD overwrite

QuestDB time-series is the lower-friction archive: same store as raw
trades, native `LATEST ON` for current-state queries, partitioned by
day for natural EOD bucket queries.

### Source field deliberately dropped

`parser.NBS.Source` (58 or 59) is implicit in which table the row lives
in (`nbs_stock` vs `nbs_broker`). Not stored as a column to avoid
redundancy.
