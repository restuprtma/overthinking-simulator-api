# quote-consumer — Kubernetes Deploy

Deploy of [`cmd/quote-consumer`](../../../cmd/quote-consumer/) to the
production RKE2 cluster, namespace `tuai`. The consumer subscribes to
`idx.quote.>` (IQPlus record type 14 — Quote) on JetStream `IDX_QUOTE`
and maintains the **latest quote snapshot per stock** (FID 0..79) in
Redis as a HASH.

> Type 14 Quote is the canonical scalar state for each instrument:
> last/open/high/low/prev/avg, foreign buy/sell totals, top-of-book
> bid/ask, base price (for ARA/ARB), and ~80 other FIDs. UI stock
> detail pages read from `quote:<stock>` directly via Redis.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-quote-consumer:<tag>` (Docker Hub) |
| Replicas | **1** (stateless; can scale via durable load-balance if needed) |
| Node affinity | `nodetype=worker` |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| Redis | `10.10.8.31:6379` (DB 12, password auth) |

### Subjects subscribed

- `idx.quote.>` — IQPlus Type 14 Quote frames for IDX stocks, regional
  indices, commodities, futures, and currency pairs

### Redis layout

```
HASH quote:<stock>
  field "<FID>" → "<value>"
  e.g. quote:BBCA
       0   → "BBCA"           (Code)
       1   → "Bank Central Asia Tbk."
       11  → "9000"           (Base Price)
       54  → "9050"           (Open)
       56  → "9100"           (Last traded)
       57  → "9125"           (High)
       59  → "9050"           (Low)
       60  → "9000"           (CLOSE / prev)
       67  → "100"            (CHANGE)
       73  → "1234500"        (SHARELOT)
       74  → "12345600000"    (FRGBOUGHTVAL)
       75  → "8901230000"     (FRGSOLDVAL)
       78  → "9075"           (AVG)
       79  → "1.11"           (PCTCHANGE)
       …
```

Vendor sends partial frames (only the FIDs that changed) — sink HSETs
each present FID, untouched FIDs keep their previous value. Reading
`HGETALL quote:BBCA` returns the full latest state.

TTL 25h on each key (refreshed on update). FID reference: see
[`docs/iqplus/iqplus-data-feed-v4.0.0.md` §5.3](../../../docs/iqplus/iqplus-data-feed-v4.0.0.md#L191-L284).

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
  -f deployments/docker/quote-consumer.Dockerfile \
  -t venturoid/tuai-be-quote-consumer:1.0.0 .
docker push venturoid/tuai-be-quote-consumer:1.0.0

# Prepare secret
cp deployments/kubernetes/quote-consumer/secret.yaml.example \
   deployments/kubernetes/quote-consumer/secret.yaml
# Edit it: fill in NATS_TOKEN, REDIS_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/quote-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/quote-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/tuai-be-quote-consumer --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/tuai-be-quote-consumer --tail=20
```

Expected on startup:

```
quote-consumer starting … nats_filter=idx.quote.> redis_addr=10.10.8.31:6379 redis_db=12
quote subscriber ready  … durable=quote-state-cache
quote consumer stats    … received=N acked=N parse_err=0
```

Stats line every `STATS_INTERVAL=30s`.

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=tuai-be-quote-consumer

# Tail stats (active during market hours: ~09:00–16:00 WIB)
kubectl logs -n tuai deploy/tuai-be-quote-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/tuai-be-quote-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/tuai-be-quote-consumer

# JetStream durable position
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_QUOTE quote-state-cache
```

### Inspect Redis snapshots

```bash
# Full snapshot for one stock
redis-cli -h 10.10.8.31 -a "$RP" -n 12 HGETALL quote:BBCA

# Specific FIDs (e.g. last traded + change + pct)
redis-cli -h 10.10.8.31 -a "$RP" -n 12 HMGET quote:BBCA 56 67 79

# How many stocks tracked
redis-cli -h 10.10.8.31 -a "$RP" -n 12 DBSIZE
```

### Verifying live ingest

```bash
# DBSIZE during market hours should be > 800 (all listed instruments)
redis-cli -h 10.10.8.31 -a "$RP" -n 12 DBSIZE

# Check freshness — updated_ts should be within last few seconds
# (FID-based schema doesn't have an explicit updated_ts; rely on rate)
kubectl logs -n tuai deploy/tuai-be-quote-consumer | grep stats | tail -1
```

If rate=0 during market hours, check JetStream consumer pending and
pod logs.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `nats subscribe failed: stream not found` | `IDX_QUOTE` stream not provisioned | Confirm with broker operator |
| stats stuck at zero during market hours | Wrong filter / token | Confirm `nats_filter=idx.quote.>`, NATS_TOKEN valid |
| `parse_err` > 0 | Vendor schema drift on Type 14 (FID format) | Inspect warn log; payload should be `<FID>;<value>` groups separated by `\|` |
| Pod restarts during burst | Memory pressure | Bump `resources.limits.memory` (default 256Mi) |
| Redis CPU spike | Burst HSET hits on hot stocks | Check Redis SLOWLOG; pipeline already batches per message |
| Old FID values stuck | Vendor only sends changed FIDs — old values are stale by design | Compare `updated_ts` if added; or trust last vendor frame as truth |

### Pod runs but no stats line after 30s

```bash
kubectl logs -n tuai deploy/tuai-be-quote-consumer | grep -iE 'subscriber|durable|consumer'
```

If `consumer already exists with conflicting filter`, the durable has
different settings — fix via:
`nats consumer rm IDX_QUOTE quote-state-cache` then redeploy.

---

## Why one replica

Stateless service — Redis HSET per FID is idempotent, so two replicas
sharing the same JetStream durable would just split the load (no
correctness risk). Single replica is enough for current ~800 stocks
× moderate FID-update rate.

If horizontal scale is ever needed:
- Bump `replicas` to 2 or 3 — durable load-balances new pods in.
- No state migration needed.

---

## Observability

Stats line every `STATS_INTERVAL` (30s):

```json
{
  "level": "info",
  "msg": "quote consumer stats",
  "received": 12345, "acked": 12344,
  "naked": 0, "decode_err": 0, "parse_err": 0,
  "rate_per_sec": 41.15,
  "elapsed": 300
}
```

Suggested alerts:

| Metric | Threshold | Action |
|---|---|---|
| `naked` | > 0 | Look for parse / sink errors above the stats line |
| `decode_err`, `parse_err` | > 0 sustained | Upstream protocol change |
| `rate_per_sec` during 09:00–15:00 WIB | < 5 | Subscriber stuck or stream empty |

---

## Initial deploy log

- **2026-05-08** — first deploy with standardized pattern; image `venturoid/tuai-be-quote-consumer`, dedicated `quote-consumer.Dockerfile`, `deploy.sh` automation.

Image size: ~5 MB (scratch + static binary).
