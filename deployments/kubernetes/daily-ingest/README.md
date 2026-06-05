# daily-ingest — Kubernetes Deploy

Deploy of [`cmd/daily-ingest`](../../../cmd/daily-ingest/) to the production
RKE2 cluster, namespace `tuai`. The daemon aggregates QuestDB raw ticks
from `trades` (broker-real archive populated by `resend-trade-consumer`)
into per-day OHLCV bars and UPSERTs into Postgres `stock.prices_daily`.

> Fires once per WIB day at `DAILY_INGEST_RUN_AT` (default 17:00 = post-close).
> Single replica with `Recreate` strategy — daemon has internal scheduler,
> two pods would both fire (UPSERT idempotent so output stays correct, but
> it would double Postgres load).

---

## Target

| Item | Value |
|---|---|
| Cluster | RKE2 v1.32.6 (Rancher proxy `10.10.1.1`) |
| Kubeconfig | [`deployments/kubernetes/production.yaml`](../production.yaml) |
| Namespace | `tuai` |
| Image | `venturoid/tuai-be-daily-ingest:<tag>` (Docker Hub) |
| Replicas | **1** (Recreate strategy — internal scheduler must not run twice) |
| Node affinity | `nodetype=worker` |
| Schedule | 17:00 WIB daily, `DAILY_INGEST_TARGET=today` |

### Network dependencies

| Service | Endpoint |
|---|---|
| Postgres | `10.10.8.10:5432` (DB `tuai`, user `tuai_tan`) |
| QuestDB | `10.10.8.10:9000` (HTTP `/exec`, basic auth) |

### Tables touched

**Source** — read-only via QuestDB HTTP `/exec`:

```
trades                   ← populated by resend-trade-consumer (Type 27, broker-real)
SELECT first(price), max, min, last, sum(volume), sum(price*volume), count()
SAMPLE BY 1d ALIGN TO CALENDAR TIME ZONE 'Asia/Jakarta'
```

**Destination** — UPSERT via pgx batch:

```sql
stock.prices_daily (
  stock_code TEXT,
  date       DATE,    -- WIB calendar
  market     TEXT,    -- RG/TN/NG (per-row, fallback DEFAULT_MARKET if NULL)
  open, high, low, close, volume, value, freq  BIGINT,
  PRIMARY KEY (stock_code, date, market)
);
```

ON CONFLICT (stock_code, date, market) DO UPDATE — re-runnable for the
same date (cron retry, manual backfill via `--once`).

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

## Prerequisites

1. **Postgres `stock` schema** with `stock.prices_daily` migrated:
   ```bash
   make DB_HOST=10.10.8.10 DB_PORT=5432 DB_USER=tuai_tan \
        DB_PASSWORD='...' DB_NAME=tuai DB_SSLMODE=disable \
        migrate-up
   ```

2. **QuestDB `trades` table populated** by `resend-trade-consumer` (which
   reads Type 27 frames after market close). The daemon SELECTs from there
   at fire time — if `resend-trade-consumer` hasn't drained yesterday's
   resends by 17:00 WIB, the daily bars will be incomplete.

---

## First-time deploy

```bash
export KUBECONFIG=$PWD/deployments/kubernetes/production.yaml

# Build & push
docker build \
  -f deployments/docker/daily-ingest.Dockerfile \
  -t venturoid/tuai-be-daily-ingest:1.0.0 .
docker push venturoid/tuai-be-daily-ingest:1.0.0

# Prepare secret
cp deployments/kubernetes/daily-ingest/secret.yaml.example \
   deployments/kubernetes/daily-ingest/secret.yaml
# Edit it: fill in DB_PASSWORD, QUESTDB_AUTH_PASSWORD

# Apply
kubectl apply -f deployments/kubernetes/daily-ingest/secret.yaml
kubectl apply -f deployments/kubernetes/daily-ingest/deployment.yaml
kubectl rollout status -n tuai deploy/tuai-be-daily-ingest --timeout=2m
```

### Verify

```bash
kubectl logs -n tuai deploy/tuai-be-daily-ingest --tail=20
```

Expected on startup:

```
Database connection pool initialized … max_conns=5
daily-ingest daemon starting … run_at=61200 target=today
daily-ingest daemon scheduled  … next_fire_wib=2026-05-08T17:00:00+0700
```

`run_at=61200` = seconds since WIB midnight (= 17:00:00).

---

## Manual one-shot ingest

For backfill or recovery, run the binary in `--once` mode via a one-off pod:

```bash
# Yesterday WIB (default for --once)
kubectl run daily-ingest-once --rm -i --restart=Never -n tuai \
  --image=venturoid/tuai-be-daily-ingest:latest \
  --image-pull-policy=Always \
  --overrides='{
    "spec": {
      "imagePullSecrets": [{"name":"dockerhub-secret-venturoid"}],
      "containers": [{
        "name": "daily-ingest-once",
        "image": "venturoid/tuai-be-daily-ingest:latest",
        "args": ["--once"],
        "envFrom": [{"secretRef": {"name": "tuai-be-daily-ingest-env"}}]
      }]
    }
  }' -- --once

# Specific date
... -- --once --date=2026-04-25

# Date range (inclusive)
... -- --once --from=2026-04-01 --to=2026-04-25
```

---

## Daily operations

```bash
# Status
kubectl get pods -n tuai -l app.kubernetes.io/name=tuai-be-daily-ingest

# Logs (mostly idle until run_at — 1 batch per day)
kubectl logs -n tuai deploy/tuai-be-daily-ingest --tail=50

# Tail live (useful when run_at is approaching)
kubectl logs -n tuai deploy/tuai-be-daily-ingest -f

# Errors / warnings
kubectl logs -n tuai deploy/tuai-be-daily-ingest | grep -E '"level":"(error|warn)"'

# Restart (re-reads secret, recomputes next_fire)
kubectl rollout restart -n tuai deploy/tuai-be-daily-ingest
```

### Verify ingest result in Postgres

```bash
PGPASSWORD='...' psql -h 10.10.8.10 -p 5432 -U tuai_tan -d tuai -c "
  SELECT date, market, count(*) AS bars, sum(volume) AS total_vol
  FROM stock.prices_daily
  WHERE date >= current_date - 7
  GROUP BY date, market
  ORDER BY date DESC, market;
"
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Pod runs but never ingests | `next_fire_wib` already past for today, daemon waits until tomorrow | Use `--once` form for manual backfill of today |
| Pod restarted after RUN_AT — daemon won't refire same day | Restart consumed today's slot | Manual `--once` for the missed day |
| `null value in column "stock_code"` | Legacy QuestDB row with NULL field other than `market` | Inspect QuestDB row; only `market` has fallback via `DAILY_INGEST_DEFAULT_MARKET` |
| Postgres connection errors | DB host/cred wrong | Verify Secret values; `kubectl run debug` with debug image to test connectivity (scratch image has no shell) |
| QuestDB query timeout | Heavy day with many resends | Bump `QUESTDB_HTTP_TIMEOUT` (default 60s) |
| Bars=0 logged at fire time | `trades` table empty for that day — `resend-trade-consumer` hasn't drained yet | Check `resend-trade-consumer` logs; rerun `--once` after resends finish |

---

## Initial deploy log

- **2026-04-28** — first deploy as `daily-ingest`, image `venturoid/tuai-daily-ingest-production`.
- **2026-05-08** — manifest standardized to match consumer pattern; renamed to `tuai-be-daily-ingest`, image repo `venturoid/tuai-be-daily-ingest`; dedicated `daily-ingest.Dockerfile` and `deploy.sh` automation added.

```
Database pool init … max_conns=5
Daemon starting    … run_at=61200 (17:00 WIB)
Next fire WIB      … 2026-04-28T17:00:00.000+0700
Sleep              … 56,687s (~15.7h)
```

Image size: ~12 MB (scratch + static binary; larger than streaming
consumers because daily-ingest links pgx).
