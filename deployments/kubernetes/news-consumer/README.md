# news-consumer — Kubernetes Deploy

Deploy of [`cmd/news-consumer`](../../../cmd/news-consumer/), namespace `tuai`.
Subscribes to `idx.news.>` on JetStream `IDX_NEWS`, reassembles multi-packet
news frames (IDX news Type 25 splits one article across many TCP packets),
and inserts complete articles into MongoDB.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-news-consumer-production:<tag>` (Docker Hub, `consumer.Dockerfile`) |
| Replicas | **1** (multi-packet assembler is in-memory — splitting causes half-assembled frames) |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| MongoDB | `10.10.8.10:27017` (auth via `tuai_tan`, authSource `admin`, DB `tuai`, collection `news`) |

> URL-encode `*` in `MONGO_URI` password as `%2A`. The driver doesn't accept
> raw `*` in the userinfo segment.

---

## Files

| File | Purpose |
|---|---|
| `README.md` | This document |
| `secret.yaml.example` | Env Secret template (committed) |
| `secret.yaml` | Real Secret (**gitignored**) |
| `deployment.yaml` | Workload spec |

---

## First-time deploy

```bash
export KUBECONFIG=$PWD/deployments/kubernetes/production.yaml

docker build -f deployments/docker/consumer.Dockerfile \
  --build-arg SERVICE=news-consumer \
  -t venturoid/tuai-news-consumer-production:1.0.0 .
docker push venturoid/tuai-news-consumer-production:1.0.0

cp deployments/kubernetes/news-consumer/secret.yaml.example \
   deployments/kubernetes/news-consumer/secret.yaml
# Edit secret.yaml — fill NATS_TOKEN and replace <PASSWORD> in MONGO_URI

kubectl apply -f deployments/kubernetes/news-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/news-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/news-consumer --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/news-consumer --tail=20
```

Expected on startup:

```
news-consumer starting … nats_filter=idx.news.>  mongo_collection=news
mongo sink ready       … database=tuai collection=news
news subscriber ready  … durable=news-indexer
news consumer stats    … packets_received=0 news_inserted=0 (idle outside news hours)
```

IDX news is low-frequency (corp announcements, market commentary) — `packets_received=0` for long stretches is normal.

---

## Redeploy

```bash
NEW_TAG=1.0.1
docker build -f deployments/docker/consumer.Dockerfile \
  --build-arg SERVICE=news-consumer \
  -t venturoid/tuai-news-consumer-production:$NEW_TAG .
docker push venturoid/tuai-news-consumer-production:$NEW_TAG

sed -i "s|tuai-news-consumer-production:[0-9.]*|tuai-news-consumer-production:$NEW_TAG|" \
  deployments/kubernetes/news-consumer/deployment.yaml

kubectl apply -f deployments/kubernetes/news-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/news-consumer --timeout=2m
```

### Update env / secrets only

```bash
kubectl apply -f deployments/kubernetes/news-consumer/secret.yaml
kubectl rollout restart -n tuai deploy/news-consumer
```

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=news-consumer

# Tail stats
kubectl logs -n tuai deploy/news-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/news-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/news-consumer
```

### Verify articles landed in MongoDB

From a debug pod (scratch image has no shell):

```bash
kubectl run mongocli --rm -i --restart=Never \
  --image=mongo:7 -n tuai --command -- \
  mongosh "$MONGO_URI" --quiet --eval '
    db.news.countDocuments({});
    db.news.find({}, {title:1, published_at:1}).sort({_id:-1}).limit(5).toArray();
  '
```

(Pass MONGO_URI via `--env` so the password isn't in argv. See nats subject naming for the full schema.)

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `mongo sink ready` log appears, then errors | Wrong DB / collection / authSource | Check `mongoexec` from a debug pod; verify `authSource=admin` in MONGO_URI |
| Many `dup_packets` in stats | News re-broadcast by IDX during reconnect | Harmless — assembler dedups; insert is upsert by `news_id` |
| Many `buffers_evicted` | Stale partial frames swept after `NEWS_BUFFER_STALE_AFTER` (10m default) | Indicates upstream gaps. Check publisher logs for missing packets in the news range. |
| `decode_err` > 0 | Publisher emitting malformed Type 25 envelope | Coordinate with iqplus-publisher — version mismatch |
| Pod CrashLoop on first start | `MONGO_URI` userinfo not URL-encoded (`*` literal) | Encode `*` → `%2A`, restart |

---

## Initial deploy log

First successful deploy: **2026-04-28**.

```
T+0   : pod scheduled, image pulled (~3.9 MB scratch)
T+10s : mongo sink ready, news subscriber ready
T+30s : first stats — packets_received=0 (after-hours, no news flow)
```

The durable `news-indexer` was created fresh on this deploy. Position
starts from `now` (NATS default) — historical news in the JetStream
backlog won't be replayed unless the durable is recreated with
`DeliverPolicy=All`. Adjust at the broker if backfill is needed.
