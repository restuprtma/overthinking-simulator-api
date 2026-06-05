# resend-order-consumer — Kubernetes Deploy

Deploy of [`cmd/resend-order-consumer`](../../../cmd/resend-order-consumer/) to
the production RKE2 cluster, namespace `tuai`. The handler subscribes to
`idx.resend.order.>` on JetStream `IDX_TICK` and writes every Type 26
row into the QuestDB `orders` table.

IDX vendor emits Type 26 (Resend Order) after market close with the
real broker codes that were masked as `"--"` in live Type 16 events.
DEDUP UPSERT KEYS on QuestDB protects against duplicates if a row is
re-processed.

> **Timestamp note.** The Type 26 wire payload only carries HHMMSS; the
> date is taken from the envelope's frame Date stamp (vendor server
> clock at frame emit). Late post-midnight deliveries are auto-rolled
> back one day so orders land on the actual trading day — see
> [service.resolveTimestamp](../../../internal/modules/stock/resend_order_consumer/service/service.go).

The live order events are processed by `orderbook-consumer` for the
sub-second order book reconstruction in Redis; that path uses Type 16
(masked broker codes). This handler is the broker-real archive,
queried separately from QuestDB.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-resend-order-consumer:<tag>` (Docker Hub) |
| Replicas | **1** (Type 26 is bursty, JetStream durable cursor exclusive) |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| QuestDB | `10.10.8.51:9000` (basic auth) |

### Table required in QuestDB

The `orders` table MUST exist before pod starts (auto-create via ILP
works but the schema would be sub-optimal — no DEDUP, no PARTITION BY
HOUR). Canonical schema:

```sql
CREATE TABLE 'orders' (
    timestamp TIMESTAMP,
    stock SYMBOL INDEX CAPACITY 256,
    market SYMBOL,
    command LONG,        -- 0 bid, 1 offer, 2 cancel-bid, 3 cancel-offer
    order_no LONG,
    price DOUBLE,
    volume LONG,
    broker SYMBOL,
    balance LONG,
    investor SYMBOL,
    no_reference LONG
) timestamp(timestamp) PARTITION BY HOUR
DEDUP UPSERT KEYS(timestamp, stock, order_no, command);
```

DEDUP key includes `command` because the same `order_no` legitimately
appears with different commands (add then cancel) at different
timestamps — including command guards against any same-second collision.

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
  -f deployments/docker/resend-order-consumer.Dockerfile \
  -t venturoid/tuai-be-resend-order-consumer:1.0.0 .
docker push venturoid/tuai-be-resend-order-consumer:1.0.0

# Prepare secret
cp deployments/kubernetes/resend-order-consumer/secret.yaml.example \
   deployments/kubernetes/resend-order-consumer/secret.yaml
# Edit it: fill in NATS_TOKEN, QUESTDB_AUTH_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/resend-order-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/resend-order-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/resend-order-consumer --timeout=2m
```

The `dockerhub-secret-venturoid` imagePullSecret is shared across the
namespace — already provisioned during running-trade-consumer deploy.

### Verify

```bash
kubectl logs -n tuai deploy/resend-order-consumer --tail=20
```

Expected on startup:

```
resend-order-consumer starting … filter=idx.resend.order.> questdb_table=orders
resend-order subscriber ready  … durable=resend-order-backfill
```

Stats lines appear every `STATS_INTERVAL=30s` with `backfilled` (rows
written) and `ts_err` (timestamp resolution failures) counters.

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=resend-order-consumer

# Tail stats (active during ~17:00 WIB and any late-delivery windows)
kubectl logs -n tuai deploy/resend-order-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/resend-order-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/resend-order-consumer

# Verify the JetStream durable is positioned correctly
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK resend-order-backfill
```

### Verifying a resend ran successfully

After the post-close window (typically ~18:00 WIB onwards), check row
count:

```bash
curl -sf -G -u "tuai_tan:$QDB_PASS" \
  --data-urlencode "query=SELECT count() FROM orders WHERE timestamp IN today()" \
  http://10.10.8.51:9000/exec
```

If a window passes with no row growth, the resend either didn't run
upstream or our handler missed it — check pod logs and JetStream durable
position.

---

## Troubleshooting

### Backfill not landing in QuestDB

Same connectivity checklist as running-trade-consumer — see
[../running-trade-consumer/README.md](../running-trade-consumer/README.md#troubleshooting).

If pod is up but the `orders` table doesn't grow during the expected
window:

```bash
# Check JetStream consumer for backlog and last-delivered position
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK resend-order-backfill

# Did the publisher actually emit? Look for idx.resend.order.* on the stream
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  stream subjects IDX_TICK | grep resend.order
```

### `ts_err` counter rising

Means envelope.Date / envelope.Time / order.Time has bad format.
Inspect the warn log line — it includes `seq`, `env_date`, `env_time`,
`order_time`. If the vendor data is genuinely malformed, those rows are
dropped (no nak — same shape as `parse_err`).

---

## Initial deploy log

- **2026-05-07** — first deploy, single-table `orders`.

Image size: ~4 MB (scratch + static binary).
