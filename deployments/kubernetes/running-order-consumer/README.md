# running-order-consumer — Kubernetes Deploy

Deploy of [`cmd/running-order-consumer`](../../../cmd/running-order-consumer/)
to the production RKE2 cluster, namespace `tuai`. The consumer
subscribes to `idx.order.>` (Type 16, live) on JetStream `IDX_TICK` and
writes every order event to the QuestDB `running_orders` table.

Mirrors the `running_trades` half of `running-trade-consumer` for orders.
Live broker codes are masked as `"--"` (regulation); the broker-real
archive lives in the `orders` table populated by
[`resend-order-consumer`](../resend-order-consumer/).

> Type 16 is **high-frequency** (every order add/cancel is a frame).
> Expect millions of rows per active trading day. DEDUP UPSERT KEYS
> guards against re-processed messages.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-running-order-consumer:<tag>` (Docker Hub) |
| Replicas | **2** (durable load-balances; sized for live throughput) |
| Node affinity | `nodetype=worker` |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| QuestDB | `10.10.8.51:9000` (basic auth) |

### Table required in QuestDB

```sql
CREATE TABLE 'running_orders' (
    timestamp TIMESTAMP,
    stock SYMBOL INDEX CAPACITY 256,
    market SYMBOL,
    command LONG,        -- 0 bid, 1 offer, 2 cancel-bid, 3 cancel-offer
    order_no LONG,
    price DOUBLE,
    volume LONG,
    broker SYMBOL,       -- always "--" during live session
    balance LONG,
    investor SYMBOL,
    no_reference LONG
) timestamp(timestamp) PARTITION BY HOUR
DEDUP UPSERT KEYS(timestamp, stock, order_no, command);
```

Identical schema to `orders`, just a different name. DEDUP key
includes `command` because the same `order_no` legitimately appears
with different commands (add then cancel) at different timestamps.

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
  -f deployments/docker/running-order-consumer.Dockerfile \
  -t venturoid/tuai-be-running-order-consumer:1.0.0 .
docker push venturoid/tuai-be-running-order-consumer:1.0.0

# Prepare secret
cp deployments/kubernetes/running-order-consumer/secret.yaml.example \
   deployments/kubernetes/running-order-consumer/secret.yaml
# Edit it: fill in NATS_TOKEN, QUESTDB_AUTH_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/running-order-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/running-order-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/tuai-be-running-order-consumer --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/tuai-be-running-order-consumer --tail=20
```

Expected on startup:

```
running-order-consumer starting … filter=idx.order.> questdb_table=running_orders
running-order subscriber ready  … durable=running-order-consumer
```

Stats lines appear every `STATS_INTERVAL=30s` with `written` (rows
written) and `ts_err` (timestamp resolution failures) counters.

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=tuai-be-running-order-consumer

# Tail stats (active during market hours: ~09:00–16:00 WIB)
kubectl logs -n tuai deploy/tuai-be-running-order-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/tuai-be-running-order-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/tuai-be-running-order-consumer

# JetStream durable position
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK running-order-consumer
```

### Verifying live ingest

```bash
# Last 1 minute of order rows
curl -sf -G -u "tuai_tan:$QDB_PASS" \
  --data-urlencode "query=SELECT count() FROM running_orders WHERE timestamp > dateadd('m', -1, now())" \
  http://10.10.8.51:9000/exec
```

If 0 during market hours, check JetStream consumer pending and pod
logs.

---

## Troubleshooting

### Backlog growing (consumer can't keep up)

Symptoms: `consumer info` shows large `num_pending`; QuestDB write
rate plateaus.

- Bump `replicas` to 3 or 4 — durable load-balances new pods in.
- Increase `QUESTDB_AUTO_FLUSH_ROWS` (default 2000 → 5000) for larger
  batches per HTTP round trip.
- Bump container memory limit if pods OOMKilled.

### `ts_err` rising

Means `envelope.Date` / `envelope.Time` / `order.Time` has bad format.
Inspect the warn log line — it includes `seq`, `env_date`, `env_time`,
`order_time`. Rare if vendor is healthy; usually publisher bug.

### Same `order_no` appears multiple times in `running_orders`

Expected — that's the order lifecycle (add at T1, cancel at T2 = two
rows). DEDUP key `(timestamp, stock, order_no, command)` only collapses
exact reprocessing of the same event, not legitimate state changes.

---

## Initial deploy log

- **2026-05-07** — first deploy, single-table `running_orders`.

Image size: ~4 MB (scratch + static binary).
