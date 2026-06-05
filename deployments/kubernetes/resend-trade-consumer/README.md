# resend-trade-consumer — Kubernetes Deploy

Deploy of [`cmd/resend-trade-consumer`](../../../cmd/resend-trade-consumer/) to the
production RKE2 cluster, namespace `tuai`. The handler subscribes to
`idx.resend.trade.>` on JetStream `IDX_TICK` and writes every Type 27
row into the QuestDB `trades` table.

IDX vendor emits Type 27 in two batches per session: mid-day around
~12:00 WIB (lunch break) and post-close (sometimes delivered late, past
midnight WIB). Both batches share the same broker-real schema and land
in the same table. DEDUP UPSERT KEYS on QuestDB protects against
duplicates if a row is re-processed.

The live `running_trades` table written by running-trade-consumer is **not**
updated by this handler — its broker codes stay masked as `"--"`. An
earlier design tried to overwrite the live row via DEDUP UPSERT
cross-table, but that path was too slow at IDX-scale.

> **Why single-table now?** A previous version routed rows into
> `mid_trades` vs `trades` based on the handler's wall-clock receipt
> hour in WIB. That proxy broke whenever the vendor delivered late
> (e.g. ~00:02 WIB) or JetStream replayed messages — post-close rows
> ended up in `mid_trades`. Single-table avoids the broken split.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-resend-trade-consumer:<tag>` (Docker Hub) |
| Replicas | **2** (JetStream durable load-balances across subscribers) |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| QuestDB | `10.10.8.51:9000` (basic auth) |

### Table required in QuestDB

The `trades` table MUST exist before pod starts (auto-create via ILP
works but the schema would be sub-optimal — no DEDUP, no PARTITION BY
HOUR). Canonical schema:

```sql
CREATE TABLE 'trades' (
    timestamp TIMESTAMP,
    stock SYMBOL INDEX CAPACITY 256,
    market SYMBOL,
    command LONG,
    price DOUBLE,
    volume LONG,
    buyer SYMBOL,
    seller SYMBOL,
    buyer_type SYMBOL,
    seller_type SYMBOL,
    buyer_order_no LONG,
    seller_order_no LONG,
    trade_no LONG
) timestamp(timestamp) PARTITION BY HOUR
DEDUP UPSERT KEYS(timestamp, stock, trade_no);
```

DEDUP keys ensure the same trade re-processed (e.g. JetStream redeliver
or vendor re-emit) won't duplicate.

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
  -f deployments/docker/resend-trade-consumer.Dockerfile \
  -t venturoid/tuai-be-resend-trade-consumer:1.0.0 .
docker push venturoid/tuai-be-resend-trade-consumer:1.0.0

# Prepare secret
cp deployments/kubernetes/resend-trade-consumer/secret.yaml.example \
   deployments/kubernetes/resend-trade-consumer/secret.yaml
# Edit it: fill in NATS_TOKEN, QUESTDB_AUTH_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/resend-trade-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/resend-trade-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/resend-trade-consumer --timeout=2m
```

The `dockerhub-secret-venturoid` imagePullSecret is shared across the
namespace — already provisioned during running-trade-consumer deploy.

### Verify

```bash
kubectl logs -n tuai deploy/resend-trade-consumer --tail=20
```

Expected on startup:

```
resend-trade-consumer starting … filter=idx.resend.trade.> questdb_table=trades
resend subscriber ready  … durable=resend-trade-backfill
```

Stats lines appear every `STATS_INTERVAL=30s` with a `backfilled` counter
showing rows written.

---

## Redeploy / update image

```bash
NEW_TAG=1.2.0

docker build \
  -f deployments/docker/resend-trade-consumer.Dockerfile \
  -t venturoid/tuai-be-resend-trade-consumer:$NEW_TAG .
docker push venturoid/tuai-be-resend-trade-consumer:$NEW_TAG

sed -i "s|tuai-be-resend-trade-consumer:[0-9.]*|tuai-be-resend-trade-consumer:$NEW_TAG|" \
  deployments/kubernetes/resend-trade-consumer/deployment.yaml

kubectl apply -f deployments/kubernetes/resend-trade-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/resend-trade-consumer --timeout=2m
```

### Update env / secrets only

```bash
kubectl apply -f deployments/kubernetes/resend-trade-consumer/secret.yaml
kubectl rollout restart -n tuai deploy/resend-trade-consumer
```

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=resend-trade-consumer

# Tail stats (active during ~12:00 WIB and ~17:00 WIB windows)
kubectl logs -n tuai deploy/resend-trade-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/resend-trade-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/resend-trade-consumer

# Verify the JetStream durable is positioned correctly
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK resend-trade-backfill
```

### Verifying a resend ran successfully

After each batch window, check row count:

```bash
curl -sf -G -u "tuai_tan:$QDB_PASS" \
  --data-urlencode "query=SELECT count() FROM trades WHERE timestamp IN today()" \
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

If pod is up but the `trades` table doesn't grow during the expected
window:

```bash
# Check JetStream consumer for backlog and last-delivered position
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  consumer info IDX_TICK resend-trade-backfill

# Did the publisher actually emit? Look for idx.resend.trade.* on the stream
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  stream subjects IDX_TICK | grep resend
```

### Cleaning up legacy `mid_trades`

If `mid_trades` exists from the previous dual-table version, you can
either drop it (data is duplicated in `trades` for any rows that arrived
realtime) or fold rows back into `trades` and then drop:

```sql
INSERT INTO trades
  SELECT * FROM mid_trades;
-- DEDUP keys handle conflicts.

DROP TABLE mid_trades;
```

---

## Initial deploy log

- **2026-05-04** — first dual-table version (`mid_trades` + `trades`).
- **2026-05-07** — collapsed to single `trades` table after observing
  late vendor delivery (~00:02 WIB) misroute post-close rows into
  `mid_trades`.

Image size: ~4 MB (scratch + static binary).
