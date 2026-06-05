# tradedone-consumer — Kubernetes Deploy

Deploy of [`cmd/tradedone-consumer`](../../../cmd/tradedone-consumer/)
to the production RKE2 cluster, namespace `tuai`. The consumer
subscribes to `idx.tradedone.>` (IQPlus record type 40 — Trade Done) on
JetStream `IDX_TICK` and maintains a per-(stock, price) **volume
profile snapshot** in Redis DB 14, key prefix `tradedone`.

> Type 40 frames carry **cumulative** buy/sell volume + frequency per
> price level. Vendor resends with updated totals — sink overwrites
> per-field, not accumulates.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-tradedone-consumer:<tag>` (Docker Hub) |
| Replicas | **1** (single durable cursor; idempotent HSETs make scale-up safe) |
| Node affinity | `nodetype=worker` |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| Redis | `10.10.8.31:6379` (DB 14, password auth) |

### Subjects subscribed

- `idx.tradedone.>` — IQPlus Type 40 frames (cumulative buy/sell volume +
  frequency per price level, with foreign-flow breakdown)

### Redis layout

```
HASH tradedone:<stock>           ← one hash per stock
  field "<price>" → {"bvol":..,"svol":..,"bfreq":..,"sfreq":..,
                     "bvol_f":..,"bfreq_f":..,"svol_f":..,"sfreq_f":..}

HASH tradedone:<stock>:_meta
  last_price, last_updated_ts (consumer wall-clock), _seq, updates
```

TTL 25h on both keys. **Note**: TTL is refreshed on every update, so
keys never expire while market is active. A daily reset cron on the
Redis VM clears `tradedone:*` at 08:30 WIB Mon–Fri to avoid yesterday's
price levels bleeding into today's volume profile.

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
  -f deployments/docker/tradedone-consumer.Dockerfile \
  -t venturoid/tuai-be-tradedone-consumer:1.0.0 .
docker push venturoid/tuai-be-tradedone-consumer:1.0.0

# Prepare secret
cp deployments/kubernetes/tradedone-consumer/secret.yaml.example \
   deployments/kubernetes/tradedone-consumer/secret.yaml
# Edit it: fill in NATS_TOKEN, REDIS_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/tradedone-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/tradedone-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/tuai-be-tradedone-consumer --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/tuai-be-tradedone-consumer --tail=20
```

Expected on startup:

```
tradedone-consumer starting … nats_filter=idx.tradedone.> redis_addr=10.10.8.31:6379 redis_db=14
tradedone subscriber ready  … durable=tradedone-volume-profile
```

Stats lines appear every `STATS_INTERVAL=30s` with `received`, `acked`,
`parse_err`, and `dropped` counters. `dropped` counts frames where
`RecordType != 40` reaching this durable (should stay 0 because the
filter is exact).

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=tuai-be-tradedone-consumer

# Tail stats (active during market hours: ~09:00–16:00 WIB)
kubectl logs -n tuai deploy/tuai-be-tradedone-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/tuai-be-tradedone-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/tuai-be-tradedone-consumer

# JetStream durable position
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK tradedone-volume-profile
```

### Inspect Redis snapshots

```bash
# Volume profile for one stock (returns price → JSON pairs)
redis-cli -h 10.10.8.31 -a "$RP" -n 14 HGETALL tradedone:BBCA

# Just metadata
redis-cli -h 10.10.8.31 -a "$RP" -n 14 HGETALL tradedone:BBCA:_meta

# How many price levels currently tracked
redis-cli -h 10.10.8.31 -a "$RP" -n 14 HLEN tradedone:BBCA
```

### Verifying live ingest

```bash
# Should be > 0 during market hours
redis-cli -h 10.10.8.31 -a "$RP" -n 14 DBSIZE
```

If 0 during market hours, check JetStream consumer pending and pod
logs.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `nats subscribe failed: stream not found` | `IDX_TICK` stream not provisioned | Confirm with broker operator |
| stats stuck at zero during market hours | Wrong filter subject or token | Check `nats_filter` in startup log = `idx.tradedone.>`; verify NATS_TOKEN |
| `parse_err` > 0 | Vendor payload schema drift on Type 40 | Coordinate with iqplus-publisher version; payload must be 10 pipe-separated fields |
| `dropped` climbing | Filter accidentally widened (e.g. `idx.>`) and matching non-40 frames | Tighten `NATS_FILTER_SUBJECT` back to `idx.tradedone.>` |
| Pod restarts during traffic burst | Memory pressure | Bump `resources.limits.memory` (default 256Mi). Volume profiles per stock can grow to 100+ price levels each |
| Redis CPU spike | Too many HSETs per second on hot stocks | Inspect `redis-cli SLOWLOG GET 10` on the Redis VM. Pipeline already batches per message; check Redis-side contention |
| Yesterday's price levels still in `HGETALL` | TTL refreshed on every update — key never expires; daily reset cron didn't fire | Check `~/log/tradedone-reset.log` on Redis VM. Manual fix: `redis-cli -n 14 --scan --pattern 'tradedone:*' \| xargs redis-cli -n 14 DEL` |

---

## Initial deploy log

- **2026-04-28** — first deploy as `tradedone-consumer`, image `venturoid/tuai-tradedone-consumer-production`.
- **2026-05-08** — manifest standardized to match `running-order-consumer` style; renamed to `tuai-be-tradedone-consumer`, image repo `venturoid/tuai-be-tradedone-consumer`; `deploy.sh` automation added.

Image size: ~5 MB (scratch + static binary).
