# meta-consumer — Kubernetes Deploy

Deploy of [`cmd/meta-consumer`](../../../cmd/meta-consumer/), namespace `tuai`.
Subscribes to the **low-volume "market metadata" subjects** on JetStream
`IDX_META` (status / activity / summary / top20) and maintains the latest
snapshot per type in Redis DB 13 with key prefix `market`.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-meta-consumer-production:<tag>` (Docker Hub, `consumer.Dockerfile`) |
| Replicas | **1** |

### Network dependencies

| Service | Endpoint |
|---|---|
| NATS JetStream | `nats://10.10.8.2:4222` (token auth) |
| Redis | `10.10.8.10:6379` (DB 13, password auth) |

### Subjects subscribed (default)

- `idx.status.>` — market open/close status, halt notifications
- `idx.activity.>` — total market activity counters
- `idx.summary.>` — index/sector summary
- `idx.top20.>` — top movers tables

Override `NATS_FILTER_SUBJECTS` in the Secret if you want a narrower set
(comma-separated).

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
  --build-arg SERVICE=meta-consumer \
  -t venturoid/tuai-meta-consumer-production:1.0.0 .
docker push venturoid/tuai-meta-consumer-production:1.0.0

cp deployments/kubernetes/meta-consumer/secret.yaml.example \
   deployments/kubernetes/meta-consumer/secret.yaml
# Edit secret.yaml — fill NATS_TOKEN and REDIS_PASSWORD

kubectl apply -f deployments/kubernetes/meta-consumer/secret.yaml
kubectl apply -f deployments/kubernetes/meta-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/meta-consumer --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/meta-consumer --tail=20
```

Expected on startup:

```
meta-consumer starting … nats_filters=[idx.status.> idx.activity.> idx.summary.> idx.top20.>]
meta subscriber ready
meta consumer stats     … received=N acked=N control=N
```

The stats line breaks out per category: `control`, `status`, `activity`,
`summary`, `top20`. Outside trading hours these counters stay flat.

---

## Redeploy

```bash
NEW_TAG=1.0.1
docker build -f deployments/docker/consumer.Dockerfile \
  --build-arg SERVICE=meta-consumer \
  -t venturoid/tuai-meta-consumer-production:$NEW_TAG .
docker push venturoid/tuai-meta-consumer-production:$NEW_TAG

sed -i "s|tuai-meta-consumer-production:[0-9.]*|tuai-meta-consumer-production:$NEW_TAG|" \
  deployments/kubernetes/meta-consumer/deployment.yaml

kubectl apply -f deployments/kubernetes/meta-consumer/deployment.yaml
kubectl rollout status -n tuai deploy/meta-consumer --timeout=2m
```

### Update env / secrets only

```bash
kubectl apply -f deployments/kubernetes/meta-consumer/secret.yaml
kubectl rollout restart -n tuai deploy/meta-consumer
```

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=meta-consumer

# Tail stats
kubectl logs -n tuai deploy/meta-consumer -f | grep stats

# Errors
kubectl logs -n tuai deploy/meta-consumer | grep -E '"level":"(error|warn)"'

# Restart
kubectl rollout restart -n tuai deploy/meta-consumer
```

### Inspect Redis snapshots

From a debug pod (scratch image has no shell):

```bash
kubectl run rediscli --rm -i --restart=Never --image=redis:7-alpine -n tuai \
  --command -- sh -c 'redis-cli -h 10.10.8.10 -a "$RP" --no-auth-warning -n 13 KEYS "market:*"' \
  --env="RP=$REDIS_PASSWORD"
```

Expected key shapes (subject to publisher version):
- `market:status:<board>` — current market session state
- `market:activity:<board>` — running counters
- `market:summary:<index>` — index-level summary blob
- `market:top20:<category>` — top mover tables

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `nats subscribe failed: stream not found` | `IDX_META` stream not provisioned in NATS | Ask broker operator; in dev set `NATS_ENFORCE_STREAM=false` upstream |
| stats stuck at zero during market hours | Wrong filter subjects | Check `nats_filters` in startup log — should match publisher subjects |
| `decode_err` > 0 | Publisher emitting payload that doesn't match meta envelope | Coordinate with iqplus-publisher version |
| Pod restarts during traffic burst | Memory pressure | Bump `resources.limits.memory` (default 128Mi). Meta is low-volume so this is rare. |

---

## Initial deploy log

First successful deploy: **2026-04-28**.

```
T+0   : pod scheduled
T+10s : meta subscriber ready
T+30s : first stats — received=8 (control frames), 0 errors
        — outside trading hours so no status/activity/summary/top20
```

Image size: ~3.9 MB (scratch + static binary).
