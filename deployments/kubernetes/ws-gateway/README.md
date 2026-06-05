# ws-gateway — Kubernetes Deploy (NOT YET DEPLOYED)

Manifests for [`cmd/ws-gateway`](../../../cmd/ws-gateway/), the WebSocket
fan-out for browser-side trading apps. Subscribes to `idx.candle.>` on
NATS and pushes per-stock candle updates to connected WS clients. On
subscribe, it also reads the current bar snapshot from Redis (written by
running-trade-consumer's RedisSink) and sends it as a backfill.

> **Status: artifacts only — not yet applied to the cluster.** Deploy when
> the prerequisites below are met and a deploy is explicitly requested.

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-ws-gateway-production:<tag>` (Docker Hub, build with `consumer.Dockerfile`) |
| Domain | **`ws.tuai.id`** (HTTP/80 via nginx ingress, no TLS at ingress level) |
| Service type | ClusterIP `service-ws-gateway-production:80` → pod `:8081` |
| Replicas | 1 (scale horizontally — fan-out is stateless per-message) |

---

## Prerequisites — read before deploying

1. **Candle publisher must be enabled on running-trade-consumer.** Without it, the
   `idx.candle.>` subjects have no traffic and ws-gateway will accept WS
   connections but never push messages. To enable:
   - Add `ENABLE_CANDLE_PUBLISHER: "true"` to the running-trade-consumer Secret
     ([../running-trade-consumer/secret.yaml](../running-trade-consumer/secret.yaml)).
   - Optionally override `CANDLE_NATS_URL`, `CANDLE_NATS_TOKEN`,
     `CANDLE_SUBJECT_PREFIX` (defaults reuse the main NATS connection).
   - `kubectl rollout restart -n tuai deploy/running-trade-consumer`.
   - Verify candles flowing:
     ```bash
     nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
       sub 'idx.candle.>' --count=5
     ```

2. **DNS**: `ws.tuai.id` must resolve to one of the cluster ingress IPs
   (10.10.2.110–115, 10.10.5.11–13, 10.10.5.21, 10.10.5.31). Pick one and
   create an A record (or CNAME to an existing entry).

3. **No authentication.** ws-gateway has zero auth — anyone who can reach
   the endpoint can subscribe to live candles. Source code calls this out:
   *"production must front it with a reverse proxy that validates the
   session"*. Until auth is added, treat this as **read-only public** data.
   If that's not acceptable, add one of:
   - Basic auth via nginx ingress annotation
   - JWT validation in a sidecar / external auth
   - Custom middleware in `cmd/ws-gateway/main.go` before
     `mod.Handler.Register`.

4. **Origin allowlist.** `WS_ALLOWED_ORIGINS` is empty in the secret
   (allow all). Once a production frontend domain exists, fill it in to
   block cross-origin abuse. Example:
   ```yaml
   WS_ALLOWED_ORIGINS: "https://app.tuai.id,https://tuai.id"
   ```

---

## Files in this directory

| File | Purpose |
|---|---|
| `README.md` | This document |
| `secret.yaml.example` | Env Secret template (committed) |
| `secret.yaml` | Real Secret (**gitignored**) |
| `deployment.yaml` | Pod spec with `/healthz` probes |
| `service.yaml` | ClusterIP — `service-ws-gateway-production:80 → :8081` |
| `ingress.yaml` | nginx ingress for `ws.tuai.id` with WebSocket-friendly timeouts |

---

## Deploy procedure (when authorized)

```bash
export KUBECONFIG=$PWD/deployments/kubernetes/production.yaml

# 1. Build & push image (uses the generic consumer.Dockerfile)
docker build \
  -f deployments/docker/consumer.Dockerfile \
  --build-arg SERVICE=ws-gateway \
  -t venturoid/tuai-ws-gateway-production:1.0.0 .
docker push venturoid/tuai-ws-gateway-production:1.0.0

# 2. Prepare the Secret (gitignored)
cp deployments/kubernetes/ws-gateway/secret.yaml.example \
   deployments/kubernetes/ws-gateway/secret.yaml
# Edit it: fill in NATS_TOKEN, REDIS_PASSWORD, optionally WS_ALLOWED_ORIGINS

# 3. Apply manifests
kubectl apply -f deployments/kubernetes/ws-gateway/secret.yaml
kubectl apply -f deployments/kubernetes/ws-gateway/deployment.yaml
kubectl apply -f deployments/kubernetes/ws-gateway/service.yaml
kubectl apply -f deployments/kubernetes/ws-gateway/ingress.yaml

# 4. Verify rollout
kubectl rollout status -n tuai deploy/ws-gateway --timeout=2m
kubectl logs -n tuai deploy/ws-gateway --tail=20

# 5. Smoke-test the WS endpoint (after DNS resolves)
# Health:
curl -i http://ws.tuai.id/healthz

# WS handshake (should return HTTP 101 Switching Protocols):
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: $(head -c 16 /dev/urandom | base64)" \
  "http://ws.tuai.id/ws/candles?stock=BBCA&tf=1m"
```

---

## Redeploy / update image

```bash
NEW_TAG=1.0.1
docker build -f deployments/docker/consumer.Dockerfile \
  --build-arg SERVICE=ws-gateway \
  -t venturoid/tuai-ws-gateway-production:$NEW_TAG .
docker push venturoid/tuai-ws-gateway-production:$NEW_TAG

sed -i "s|tuai-ws-gateway-production:[0-9.]*|tuai-ws-gateway-production:$NEW_TAG|" \
  deployments/kubernetes/ws-gateway/deployment.yaml

kubectl apply -f deployments/kubernetes/ws-gateway/deployment.yaml
kubectl rollout status -n tuai deploy/ws-gateway --timeout=2m
```

Strategy is `RollingUpdate (maxSurge=1, maxUnavailable=0)`, so existing WS
connections drain on the old pod while the new pod handles new connects.
Clients with persistent connections need to reconnect after their pod
terminates (default `terminationGracePeriodSeconds: 30s`).

### Update env / secrets only

```bash
kubectl apply -f deployments/kubernetes/ws-gateway/secret.yaml
kubectl rollout restart -n tuai deploy/ws-gateway
```

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=ws-gateway

# Tail logs
kubectl logs -n tuai deploy/ws-gateway -f

# Active WS connections (visible in zap log lines)
kubectl logs -n tuai deploy/ws-gateway | grep -iE 'subscribe|unsubscribe|connection'

# Scale
kubectl scale -n tuai deploy/ws-gateway --replicas=2

# Restart
kubectl rollout restart -n tuai deploy/ws-gateway

# Ingress check
kubectl get ingress -n tuai ws-gateway-production
```

### Verify WS upgrade reaching the pod

```bash
# Show recent ingress access from inside an ingress controller pod
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx --tail=50 \
  | grep ws.tuai.id
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `404 Not Found` from ingress | DNS resolves but `host: ws.tuai.id` mismatch | Check `kubectl get ingress -n tuai` and DNS A record |
| WS handshake hangs / 502 | Service selector / port mismatch | `kubectl get endpoints -n tuai service-ws-gateway-production` should list pod IPs |
| `Origin not allowed` in WS handshake | `WS_ALLOWED_ORIGINS` set but doesn't include the calling domain | Add the origin to the secret, restart deploy |
| Connection drops every 60s | nginx default timeouts kicked in | Confirm `proxy-read-timeout` annotation is applied (`kubectl describe ingress -n tuai ws-gateway-production`) |
| WS connects but no messages | Candle publisher disabled on running-trade-consumer | See "Prerequisites #1" above |
| `redis: connection refused` in logs | Wrong `REDIS_PASSWORD` or DB index | Must match running-trade-consumer's RedisSink (DB 11, prefix `candle`) |

### When candles aren't flowing

```bash
# 1. Is anything publishing?
nats --server nats://10.10.8.2:4222 --token "$NATS_TOKEN" \
  sub 'idx.candle.>' --count=3
# If silent → running-trade-consumer's ENABLE_CANDLE_PUBLISHER is off.

# 2. Is the gateway subscribed?
kubectl logs -n tuai deploy/ws-gateway | grep -i 'subscriber\|candle'

# 3. Is Redis returning snapshots?
kubectl exec -n tuai deploy/ws-gateway -- /bin/sh -c \
  "redis-cli -h 10.10.8.10 -a $REDIS_PASSWORD -n 11 KEYS 'candle:*' | head"
# Note: scratch image has no shell; use a debug pod instead:
kubectl run debug --rm -it --image=redis:7-alpine -n tuai -- \
  redis-cli -h 10.10.8.10 -a "$REDIS_PASSWORD" -n 11 KEYS 'candle:*' | head
```

---

## Why the ingress has no TLS

The cluster's existing convention (Hayyu, etc.) terminates TLS at the
upstream load balancer / Cloudflare and routes plain HTTP to the ingress
on port 80. If you need cluster-level TLS, add a `spec.tls` block with a
cert-manager-issued certificate, e.g.:

```yaml
spec:
  tls:
    - hosts: [ws.tuai.id]
      secretName: ws-tuai-id-tls
```

(Requires cert-manager + a `ClusterIssuer` configured on the cluster — not
checked at the time of writing.)
