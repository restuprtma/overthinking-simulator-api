# running-trade-consumer — Kubernetes Deploy

Deploy of [`cmd/running-trade-consumer`](../../../cmd/running-trade-consumer/)
to the production RKE2 cluster, namespace `tuai`. The consumer subscribes
to `idx.trade.>` (IQPlus record type 15 — Trade Done live) on JetStream
`IDX_TICK`, writes raw ticks to QuestDB `running_trades`, and maintains
**live OHLCV bars** in Redis for the configured timeframes (1m/5m/15m/1h).

> Mirrors the `running_orders` half of `running-order-consumer` for trades.
> Live broker codes are masked as `"--"` (regulation); the broker-real
> archive lives in the `trades` table populated by
> [`resend-trade-consumer`](../resend-trade-consumer/).

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-running-trade-consumer:<tag>` (Docker Hub) |
| Replicas | **1** (in-memory aggregator state — see "Why one replica" below) |
| Node affinity | `nodetype=worker` |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| Redis | `10.10.8.31:6379` (DB 11, password auth) |
| QuestDB | `10.10.8.51:9000` (HTTP ILP, basic auth) |

### Subjects subscribed

- `idx.trade.>` — IQPlus Type 15 frames (live matched trades, broker masked `--`)

### Storage layout

**QuestDB `running_trades`** — raw tick stream, designated timestamp `timestamp` (UTC):

```sql
CREATE TABLE 'running_trades' (
    timestamp TIMESTAMP,
    stock SYMBOL INDEX CAPACITY 256,
    market SYMBOL,            -- RG/TN/NG (derived from suffix consumer-side)
    command LONG,             -- 0 matched, 1 withdrawn
    price DOUBLE,
    volume LONG,              -- shares (not lots)
    buyer SYMBOL,             -- "--" during live session
    seller SYMBOL,
    buyer_type SYMBOL,        -- F (foreign) / D (domestic)
    seller_type SYMBOL,
    buyer_order_no LONG,
    seller_order_no LONG,
    trade_no LONG
) timestamp(timestamp) PARTITION BY HOUR;
```

**Redis (DB 11) — live OHLCV bars**, one HASH per (stock, timeframe):

```
HASH candle:<stock>:<tf>     ← e.g. candle:BBCA:1m
  open, high, low, close      (DOUBLE)
  volume, trades              (LONG)
  open_ts, close_ts           (UNIX seconds)
  updated_ts                  (consumer wall-clock)
  status                      "live" | "closed"
  closed_at_ts                (set on bar Close)
```

TTL 25h on each bar, refreshed on every Update.

---

## Files

| File | Purpose |
|---|---|
| `README.md` | This document |
| `secret.yaml.example` | Env Secret template (committed) |
| `secret.yaml` | Real Secret with credentials (**gitignored**) |
| `deployment.yaml` | Workload spec |
| `deploy.sh` | Build + push + apply automation |

---

## First-time deploy

```bash
export KUBECONFIG=$PWD/deployments/kubernetes/production.yaml

# Build & push
docker build \
  -f deployments/docker/running-trade-consumer.Dockerfile \
  -t venturoid/tuai-be-running-trade-consumer:1.0.0 .
docker push venturoid/tuai-be-running-trade-consumer:1.0.0

# Prepare secret
cp deployments/kubernetes/running-trade-consumer/secret.yaml.example \
   deployments/kubernetes/running-trade-consumer/secret.yaml
# Edit it: fill in NATS_TOKEN, REDIS_PASSWORD, QUESTDB_AUTH_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/running-trade-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/running-trade-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/tuai-be-running-trade-consumer --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/tuai-be-running-trade-consumer --tail=20
```

Expected on startup:

```
running-trade-consumer starting … timeframes=[1m 5m 15m 1h] questdb_table=running_trades
running-trade subscriber ready  … durable=ohlcv-aggregator
running-trade consumer stats    … received=N acked=N parse_err=0 dropped=0
```

`durable=ohlcv-aggregator` is intentional — kept stable across the rename
to preserve the JetStream consumer cursor (renaming would force
DeliverAll replay of the entire stream).

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=tuai-be-running-trade-consumer

# Tail stats (active during market hours: ~09:00–16:00 WIB)
kubectl logs -n tuai deploy/tuai-be-running-trade-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/tuai-be-running-trade-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/tuai-be-running-trade-consumer

# JetStream durable position
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK ohlcv-aggregator
```

### Verifying live ingest

```bash
# QuestDB row growth in the last minute
curl -sf -G -u "tuai_tan:$QDB_PASS" \
  --data-urlencode "query=SELECT count() FROM running_trades WHERE timestamp > dateadd('m', -1, now())" \
  http://10.10.8.51:9000/exec

# Redis live bar for one stock
redis-cli -h 10.10.8.31 -a "$RP" -n 11 HGETALL candle:BBCA:1m
```

If 0 during market hours, check JetStream consumer pending and pod logs.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `missing required env var` | Secret not applied or key typo | `kubectl get secret -n tuai tuai-be-running-trade-consumer-env -o yaml` |
| `redis: connection refused` | Redis password wrong / network ACL | Check `REDIS_PASSWORD` in secret; `nc -zv 10.10.8.31 6379` from a debug pod |
| `nats connect: timeout` | NATS unreachable from cluster CIDR | Verify routing — pods on RKE2 use `10.42.0.0/16` |
| `questdb auth failed` | `QUESTDB_AUTH_USER/PASSWORD` wrong | Test with `curl -u user:pw http://10.10.8.51:9000/exec?query=...` |
| stats stuck at zero during market hours | Wrong filter / token | Confirm `nats_filter=idx.trade.>`, NATS_TOKEN valid |
| `parse_err` > 0 | Vendor schema drift on Type 15 | Check publisher version; payload must be 13 pipe-separated fields |
| `dropped` climbing | Filter too wide and matching non-15 frames | Tighten back to `idx.trade.>` |
| Pod OOMKilled | In-memory aggregator state too large | Bump `resources.limits.memory` (default 512Mi) — backlog catch-up uses more than steady state |

### Pod runs but no stats line after 30s

`STATS_INTERVAL` defaults to 30s. If stats never appear, subscriber is stuck:

```bash
kubectl logs -n tuai deploy/tuai-be-running-trade-consumer | grep -iE 'subscriber|durable|consumer'
```

If `consumer already exists with conflicting filter`, the old durable has
different settings — fix via:
`nats consumer rm IDX_TICK ohlcv-aggregator` then redeploy.

---

## Why one replica

The aggregator holds **open OHLCV bars** in memory under
[`internal/modules/stock/running_trade_consumer/aggregator/`](../../../internal/modules/stock/running_trade_consumer/aggregator/).
With multiple replicas, JetStream's queue group would split trade
messages between pods, and each pod would only see partial bars for any
given stock.

If horizontal scale is ever needed:
- Shard by stock (separate durable per shard with subject routing), or
- Move bar state to Redis with optimistic locking so any replica can
  update any bar.

For current load (~10–13K msg/s peak, ~365 msg/s sustained, ~100MB RSS
during burst), one pod handles it comfortably.

---

## Observability

Stats line every `STATS_INTERVAL` (30s):

```json
{
  "level": "info",
  "msg": "running-trade consumer stats",
  "received": 110355, "acked": 110354,
  "naked": 0, "decode_err": 0, "parse_err": 0, "dropped": 0,
  "rate_per_sec": 367.85,
  "elapsed": 300
}
```

Suggested alerts:

| Metric | Threshold | Action |
|---|---|---|
| `naked` | > 0 | Look for parse / sink errors above the stats line |
| `decode_err`, `parse_err` | > 0 sustained | Upstream protocol change |
| `dropped` | > 0 | Backpressure on Redis or QuestDB sink |
| `rate_per_sec` during 09:00–15:00 WIB | < 100 | Subscriber stuck or stream empty |

---

## Initial deploy log

- **2026-04-28** — first deploy as `ohlcv-consumer`, image `venturoid/tuai-ohlcv-consumer-production`.
- **2026-05-08** — renamed to `running-trade-consumer`; manifest standardized to match `running-order-consumer` style; image repo `venturoid/tuai-be-running-trade-consumer`; `deploy.sh` automation added. JetStream durable preserved as `ohlcv-aggregator` for cursor continuity.

```
T+30s  : received=10468  acked=10467  err=0  rate=349/s
T+300s : received=110355 acked=110354 err=0  rate=368/s
```

Image size: ~5 MB (scratch + static binary).
