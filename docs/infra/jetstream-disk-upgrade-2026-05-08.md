# JetStream Disk Upgrade & Capacity Tuning — 2026-05-08

**Status**: complete
**Affected**: edge VM `10.10.8.1`, main VM `10.10.8.2`
**Outcome**: publisher `503 insufficient resources` errors resolved; capacity headroom increased ~3× (edge) / ~4× (main); all per-stream limits made explicit.

---

## 1. Incident

### Symptom

Publisher (running on edge VM `landa@10.10.8.1`) emitting sustained warn-level
errors:

```
[warn] jetstream async ack error
       subject="idx.<various>"
       error="nats: API error: code=503 err_code=10023 description=insufficient resources"
```

Publisher stats showed **18% async-ack failure** rate over 36-hour runtime:

```
consumed=111,417,985
ok      =101,308,207
err     = 20,215,348   (= err_ack — server actively rejecting)
err_ack_timeout=0
backpressured=0
tcp_received=consumed   (= no TCP-level loss)
```

Subject affected: `idx.nbs.*`, `idx.resend.trade.*`, `idx.quote.*` —
mapping ke 3 dari 4 streams (`IDX_META`, `IDX_TICK`, `IDX_QUOTE`).

### Root cause

Edge JetStream server-level `max_file_store: 28GB` reached **99.9993%
capacity** (30,047,978,428 / 30,064,771,072 bytes). With `discard: new`
on edge streams (intentional — see [iqplus-edge-topology.md](iqplus-edge-topology.md)),
JetStream rejects new publishes when full instead of evicting old data.

Server log confirmed:

```
[ERR] JetStream resource limits exceeded for server
```

Per-stream `max_bytes` were **not** the bottleneck (each stream within
its individual limit). The exhausted resource was the **server-level
storage pool**.

Sum of stream usage at incident time:

| Stream | Messages | Bytes |
|---|---:|---:|
| IDX_TICK | 30,030,691 | 13.4 GiB |
| IDX_QUOTE | 26,200,913 | 12.1 GiB |
| IDX_META | 5,414,867 | 2.5 GiB |
| IDX_NEWS | 0 | 0 |
| **Sum** | **61,646,471** | **28.0 GiB** ← exactly hits 28GB cap |

Comment in original config (`~/nats-edge/conf/nats-server.conf`)
underestimated steady-state usage: `"~18-19 GB steady state with
headroom for burst"` — actual reality was ~28 GB.

### Contributing factor — main replication lag

Main JetStream was ~16 minutes behind edge during the incident:

```
Edge  61,614,921 messages
Main  60,799,655 messages
Lag      815,266 messages  (~16 min at 855 msg/s sustained rate)
```

`discard: new` at edge means edge cannot evict messages until they're
sourced by main. Replication lag sustained → edge pool fills.

---

## 2. Hardware upgrade

User upgraded SSD on both VMs prior to remediation (1 TB NVMe).

### Edge `10.10.8.1` (FreeBSD + ZFS)

```
zpool zroot   148 GiB total   8 GiB used   139 GiB free
/home/landa   140 GiB         5 GiB used   135 GiB free
ZFS compression observed: 5.6:1
```

(Pre-upgrade: ~25 GiB total.)

### Main `10.10.8.2` (Ubuntu + ext4)

```
/dev/sda1   290 GiB total   17 GiB used   273 GiB free
```

(Pre-upgrade: smaller, exact prior unknown.)

---

## 3. Sizing methodology

Observed per-day data volume:

| Stream | Msgs/day | Logical size/day | Avg bytes/msg |
|---|---:|---:|---:|
| IDX_TICK | 30M | 13.4 GiB | ~450 |
| IDX_QUOTE | 26M | 12.1 GiB | ~470 |
| IDX_META | 5.4M | 2.5 GiB | ~470 |
| IDX_NEWS | <100 | < 1 MiB | — |
| **Daily total** | **~61M** | **~28 GiB** | |

User's quoted volume estimates match (3M trade + 3M resend trade + 5M
order + 5M resend order + ~14M Trade Done = ~30M IDX_TICK).

### Role separation

| VM | Role | Retention philosophy |
|---|---|---|
| Edge | Short-term spool until main sources data | 48–72h, sized to survive main outage |
| Main | Operational backup behind QuestDB | 5–14d, replay buffer for consumer downtime |

QuestDB is the **durable archive** for tick data (Type 15 → `running_trades`,
Type 16 → `running_orders`, Type 26 → `orders`, Type 27 → `trades`).
JetStream main is **not** an archive — only operational replay buffer.

Originally drafted 800 GB main allocation; reduced to 250 GB after
realizing QuestDB removes the long-retention need.

---

## 4. Configuration changes applied

### Edge `10.10.8.1`

**`~/nats-edge/conf/nats-server.conf`** + rendered:

```diff
- max_file_store:   28GB
+ max_file_store:   80GB
  max_memory_store: 256MB
  sync_interval:    2s
```

**Restart required**: `max_file_store` is **not** SIGHUP-reloadable
([source error log](../../docs/infra/jetstream-disk-upgrade-2026-05-08.md#sighup-not-supported)).
Used `~/nats-edge/scripts/{stop,start}.sh`. Total disconnect window: ~7 seconds
(observed via leafnode reconnect log).

### Main `10.10.8.2`

**`/etc/nats/nats-server.conf`**:

```diff
-     max_file_store: 60GB
+     max_file_store: 250GB
      max_memory_store: 4GB
-     sync_interval: "2m"
+     sync_interval: "2s"
```

`sync_interval` tightened from 2 minutes to 2 seconds — NVMe IOPS
handles the increased fsync rate easily. Before: up to 2 min of
data loss on crash. After: ≤2s.

Restart via `sudo systemctl restart nats-server`.

### Per-stream limits (made explicit)

Previously per-stream `max_bytes` and `max_age` were defaults. Now
explicit, applied via `nats stream update --max-bytes=X --max-age=Y --force --timeout 60s`:

#### Edge (discard: New — reject when full)

| Stream | max_bytes | max_age |
|---|---:|---:|
| IDX_TICK | 30 GiB | 72h |
| IDX_QUOTE | 25 GiB | 48h |
| IDX_META | 5 GiB | 7d |
| IDX_NEWS | 2 GiB | 14d |
| **Sum** | **62 GiB** | |
| Server cap | **80 GiB** (30% headroom above sum) | |

#### Main (discard: Old — auto-evict)

| Stream | max_bytes | max_age |
|---|---:|---:|
| IDX_TICK | 100 GiB | 7d |
| IDX_QUOTE | 50 GiB | 4d |
| IDX_META | 30 GiB | 14d |
| IDX_NEWS | 10 GiB | 30d |
| **Sum** | **190 GiB** | |
| Server cap | **250 GiB** (32% headroom above sum) | |

---

## 5. Verification

### Edge post-restart (immediately)

```
max_storage:  80 GiB
storage:      28.0 GiB (35% used)
messages:     61,614,921 — preserved across restart
```

Leafnode reconnect to main confirmed clean:

```
17:58:38  Leafnode → main: closed (main also restarted)
17:58:39  Connection refused (attempt 1) ← main not yet up
17:58:42  Connection refused (attempt 2)
17:58:43  Connection refused (attempt 3)
17:58:45  Leafnode reconnected ✓
```

### Main post-restart

```
max_storage:    250 GiB
sync_interval:  2s
current:        27.7 GiB (11% used)
messages:       60,799,655
```

### Publisher recovery

Last 503 error timestamp: `2026-05-08T10:21:01.582Z` (UTC) =
`17:21 WIB` — before restarts at `17:57 / 17:58`.
**No new errors after restart.**

Publisher stats reset (rate=0.023/s due to market closed at this point):

```
consumed=18, err=0, err_ack=0, tcp_reconnects=0
```

---

## 6. Operational notes

### NATS CLI usage from main

Edge VM does not have `nats` CLI installed (FreeBSD, no curl/python3
either — see [`iqplus-edge-ops` memory](../../.claude/projects/-Users-tantowilathif-Logika-tuai-tuai-be/memory/iqplus_edge_ops.md)). Use main's CLI to manage edge streams:

```bash
# Main streams
nats --server "nats://${TOKEN}@10.10.8.2:4222" stream report

# Edge streams (cross-server connect)
nats --server "nats://${TOKEN}@10.10.8.1:4222" stream report
```

### Byte unit gotcha

`nats` CLI 0.3.2 requires unit suffix (e.g. `100GB`, not raw bytes):

```
nats: error: invalid bytes specification 10737418240:
       bytes must end in B, K, KB, M, MB, G, GB, T or TB
```

Always use `--max-bytes=100GB` form.

### Timeout flag essential

Per existing topology doc, large stream operations need `--timeout 60s`
override (default 5s insufficient):

```bash
nats stream update IDX_TICK \
  --max-bytes=100GB --max-age=7d --force --timeout 60s
```

### SIGHUP not supported for storage limits {#sighup-not-supported}

Server log when SIGHUP attempted:

```
[ERR] Failed to reload server configuration:
      config reload not supported for jetstream max memory and store
```

`max_file_store` and `max_memory_store` require **process restart**.
Reloadable parameters: most other config (logging, leafnode, etc.).

---

## 7. Backup files

Rollback files created during the upgrade:

```
Edge:  ~/nats-edge/conf/nats-server.conf.bak.20260508_175659
Edge:  ~/nats-edge/run/nats-server.rendered.conf.bak.20260508_175659
Main:  /etc/nats/nats-server.conf.bak.20260508_175823
```

To rollback (worst case):

```bash
# Edge
cp ~/nats-edge/conf/nats-server.conf.bak.20260508_175659 \
   ~/nats-edge/conf/nats-server.conf
# (also restore rendered or re-render via start.sh)
~/nats-edge/scripts/{stop,start}.sh

# Main
sudo cp /etc/nats/nats-server.conf.bak.20260508_175823 \
        /etc/nats/nats-server.conf
sudo systemctl restart nats-server

# Per-stream rollback if needed (no automatic backup):
nats stream update IDX_TICK --max-bytes=<old> --max-age=<old> --force --timeout 60s
```

---

## 8. Disk utilization summary

| Resource | Limit | Used now | Free | % used |
|---|---:|---:|---:|---:|
| Edge `/home/landa` (physical) | 140 GiB | 5 GiB | 135 GiB | 4% |
| Edge JS `max_storage` (logical) | 80 GiB | 29.5 GiB | 50.5 GiB | 37% |
| Edge JS sum stream max_bytes | 62 GiB | 29.5 GiB | 32.5 GiB | 48% |
| Main `/dev/sda1` (physical) | 290 GiB | 17 GiB | 273 GiB | 6% |
| Main JS `max_storage` (logical) | 250 GiB | 28 GiB | 222 GiB | 11% |
| Main JS sum stream max_bytes | 190 GiB | 28 GiB | 162 GiB | 15% |

ZFS compression on edge means 80 GiB logical → ~14 GiB physical → very
comfortable. Could go to ~150 GiB logical without disk pressure.

---

## 9. Open follow-ups

### Replication lag investigation

Lag of 815k messages observed during incident. After fix, gap is
closing (~5M msg/h source rate). Need to monitor next high-traffic
period (open at 09:00 WIB) and verify lag stays manageable.

If lag persists during burst, investigate:
- Leafnode bandwidth between edge and main
- `Sources.MaxLag` config on main
- Network latency / packet loss between VMs

### Alerting setup (not yet done)

Suggested thresholds:

| Metric | Threshold | Source |
|---|---|---|
| `storage / max_storage` per server | > 80% | `/jsz` |
| `sources[].lag` per stream on main | > 1M messages | `nats stream info` |
| Publisher `err_ack` rate | > 0 sustained | publisher stats log |
| stream `messages` count drop > 10% in 1 min | (suggests evict event) | `nats stream info` |

### Token rotation

`Mrm25UYHeaMa19yHtGlWkFEtyoQ16lrU0uzs7CzFRNA` was incidentally captured
in operator chat history during diagnosis. Rotation:

```bash
# 1. Generate new token (32+ char random)
NEW_TOKEN=$(openssl rand -hex 32)

# 2. Update edge config + env file
ssh landa@10.10.8.1 'sed -i "s/Mrm25UYHeaMa19yHtGlWkFEtyoQ16lrU0uzs7CzFRNA/${NEW_TOKEN}/" \
  ~/nats-edge/conf/nats-server.conf \
  ~/nats-edge/run/nats-server.rendered.conf \
  ~/iqplus-publisher/bin/iqplus-publisher.env'

# 3. Update main config
ssh landa@10.10.8.2 'sudo sed -i "s/Mrm25UYHeaMa19yHtGlWkFEtyoQ16lrU0uzs7CzFRNA/${NEW_TOKEN}/" \
  /etc/nats/nats-server.conf'

# 4. Restart nats-edge then nats-server (main first so leafnode reconnects)
ssh landa@10.10.8.2 'sudo systemctl restart nats-server'
ssh landa@10.10.8.1 '~/nats-edge/scripts/stop.sh && ~/nats-edge/scripts/start.sh'

# 5. Update K8s consumer Secrets (NATS_TOKEN field) and rollout-restart pods
kubectl get secret -n tuai -o name | grep tuai-be- | \
  while read s; do kubectl patch "$s" -n tuai -p '{"stringData":{"NATS_TOKEN":"'"$NEW_TOKEN"'"}}'; done
kubectl get deploy -n tuai -o name | xargs -I{} kubectl rollout restart {} -n tuai
```

### Optional — further optimize edge

Disk supports it. Current usage 37% of cap. Two upgrade paths if desired:

| Option | server cap | per-stream sum | use case |
|---|---:|---:|---|
| Current | 80 GiB | 62 GiB | comfortable headroom, current setting |
| Moderate | 120 GiB | 97 GiB (TICK→50, QUOTE→40) | 5d edge buffer for main outages |
| Aggressive | 150 GiB | 145 GiB (TICK→80, QUOTE→50) | 7d edge buffer (mostly redundant w/ main) |

No urgent need to bump — current sizing covers normal operation with
margin. Consider if main outages of 3+ days become a concern.

---

## 10. Lessons / what to bake in

1. **Set per-stream `max_bytes` explicit, sum with margin under server cap**.
   Default per-stream limits + server-only cap obscures what's actually
   binding when 503 errors happen.

2. **`discard: new` on edge** is correct architecturally (catches
   problems explicitly), but **must be paired with monitoring** that
   alerts before publisher hits the wall.

3. **Replication lag is the silent killer** for `discard: new` streams.
   If main can't drain edge fast enough, edge fills regardless of cap.
   Future: alert on `Sources.Lag > N` on main streams.

4. **NVMe sync_interval=2s** is safe and gives much better durability
   than 2-minute spin-disk era settings. Worth tightening across the
   fleet.

5. **QuestDB is the durable archive** — JetStream main retention should
   be sized for "operational replay window after consumer outage", not
   "permanent record". This avoided ~500 GB over-allocation.

---

## 11. Follow-up — IDX_META resize 2026-05-18

NBS EOD burst pada 17:15 WIB menolak **474,734 record** dengan
`err_code=10077 description=maximum bytes exceeded`. Subjects affected
murni `idx.nbs.broker.*` & `idx.nbs.stock.*` (record types 58/59 dari
IQPlus). Other IDX_META subjects (status/activity/summary/top20) tidak
terdampak.

Root cause: konfigurasi 5 GiB max_bytes (set di §4) terlalu tight kombinasi
dengan `max_age 7d`. Per-day NBS volume ~0.7 GB × 7 hari = ~5 GB baseline,
tidak menyisakan ruang untuk EOD burst (~225 MB tambahan per hari).

**Change**:
```bash
nats --server 'nats://<TOKEN>@10.10.8.1:4222' \
  stream update IDX_META --max-bytes=15GB --force --timeout 60s
```
(executed dari main VM `10.10.8.2`; no server restart needed — `max_bytes`
per-stream hot-updatable, hanya `max_file_store` yang butuh restart.)

**New sizing edge**:

| Stream | max_bytes (was) | max_bytes (now) | max_age |
|---|---:|---:|---:|
| IDX_TICK | 30 GiB | 30 GiB | 72h |
| IDX_QUOTE | 25 GiB | 25 GiB | 48h |
| **IDX_META** | **5 GiB** | **15 GiB** | 7d |
| IDX_NEWS | 2 GiB | 2 GiB | 14d |
| **Sum** | **62 GiB** | **72 GiB** | |
| Server cap | 80 GiB | 80 GiB (10% headroom) | |

Pre-change state: 2.87 GiB used (53% of old cap). Post-change: same data
preserved (5,739,153 messages, 1,509 subjects), now 18% of new cap. `discard:new`
unchanged — preserving edge "fail loud over silent drop" philosophy per
[iqplus-edge-topology.md §4](iqplus-edge-topology.md).

**Note — orphaned consumer**: `nbs-aggregator` di main sudah dead sejak
2026-04-27 (0 deliveries, 20.7M backlog growing). Stream resize ini hanya
mencegah publisher reject; NBS data tetap tidak diolah ke Redis sampai
consumer di-revive. Revival tracked separately.

**Note — NBS == broker summary**: data record types 58/59 = broker summary
dual-view (per-stock × broker, per-broker × stock). Bisa diturunkan dari
QuestDB `trades` table via `GROUP BY buyer/seller, stock`. Kalau dashboard
sudah pakai Invezgo API, pipeline NBS internal kita arguably redundant —
worth evaluating saat revive consumer.

---

## 12. Follow-up — IDX_TICK saturation & maximize sizing 2026-05-20

Recurrence dari pattern §1 dan §11. Edge IDX_TICK saturasi penuh, EOD finalization burst dan sebagian sesi pagi di-reject. **567,046 jetstream async ack errors** di publisher log hari ini saja; cumulative `err_ack` counter (publisher process up since 2026-05-17) mencapai **16,811,402 record lost** — tidak recoverable dari NATS karena publisher tidak retry on 10077.

### Timeline (UTC)

| Waktu | Event |
|---|---|
| 2026-05-19 17:47 | First `err_code=10077` pada `idx.quote.*` — saturasi mulai pasca-market |
| 2026-05-20 04:00–07:00 | Pre-market burst: 217K + 102K + 113K errors per jam |
| 09:00 (16:00 WIB) | Market close |
| 09:37 | Edge IDX_TICK last accepted write — stream officially capped |
| 09:37–10:44 | EOD `idx.tradedone.*` burst di-reject sepanjang window ini |
| 10:44 onwards | Error stop, bukan karena fix tapi karena traffic alami habis pasca-EOD |

### Root cause

Sama persis dengan §1 & §11: **server-level `max_file_store` adalah binding constraint, bukan per-stream cap**. Setelah §11 menaikkan IDX_META ke 15 GiB, sum stream max_bytes naik ke 72 GiB. Edge disk sebenarnya sudah di-upgrade ke 140 GiB (§2) — yang ketinggalan di-tune adalah `max_file_store` itu sendiri.

Stream usage saat saturasi terdeteksi:
```
IDX_TICK   30 GiB / 30 GiB max  (99.99%)  ← culprit
IDX_QUOTE  25 GiB / 25 GiB max  (99.68%)  ← juga at risk
IDX_META    6 GiB / 15 GiB max  (43%)
IDX_NEWS    0 GiB /  2 GiB max
Sum         61 GiB / 80 GB max_file_store
```

`discard: new` di edge bekerja sesuai design (§10 #2) — fail loud, bukan silent eviction. Itu sebabnya publisher `err_ack` counter naik agresif, bukan diam-diam kehilangan data tanpa signal.

### Configuration changes

**Step 1** — initial conservative bump:
```diff
- max_file_store:   80GB
+ max_file_store:   110GB
```
Restart edge NATS via `~/nats-edge/scripts/{stop,start}.sh`. Stream restore <10ms (mmap; data on disk preserved). Per-stream: TICK 30→50, QUOTE 25→35, META 15→12 GiB (rebalance dalam budget 110 GB).

**Step 2** — maximize headroom setelah konfirmasi disk lega + ZFS compression 5.57x:
```diff
- max_file_store:   110GB
+ max_file_store:   250GB
```
Second restart, juga clean. Final per-stream caps:

| Stream | §11 (was) | step 1 | **step 2 (final)** | max_age |
|---|---:|---:|---:|---:|
| IDX_TICK | 30 GiB | 50 GiB | **120 GiB** | 72h |
| IDX_QUOTE | 25 GiB | 35 GiB | **80 GiB** | 48h |
| IDX_META | 15 GiB | 12 GiB | **25 GiB** | 7d |
| IDX_NEWS | 2 GiB | 2 GiB | **5 GiB** | 14d |
| **Sum** | **72 GiB** | 99 GiB | **230 GiB** | |
| **Server cap** | 80 GiB | 110 GiB | **250 GiB** | |

**Main rebalance** (no restart — `max_bytes` hot-updatable, sebagaimana dicatat §11):
```bash
nats --server 'nats://<MAIN_TOKEN>@10.10.8.2:4222' \
  stream edit IDX_QUOTE --max-bytes=80G --force --timeout=60s
```
Main IDX_QUOTE 50 → 80 GiB. Pre-resize 77% (mendekati threshold alert), post-resize **48%**.

### Survival window setelah change

Edge buffer kalau leafnode ke main putus:

| Stream | daily rate | max_bytes baru | survival days |
|---|---:|---:|---:|
| IDX_TICK | ~13 GiB/day | 120 GiB | **~9 days** |
| IDX_QUOTE | ~12.5 GiB/day | 80 GiB | **~6 days** |
| IDX_META | ~1 GiB/day | 25 GiB | **~25 days** |

Sebelumnya: ~2.5 days untuk TICK/QUOTE. Sekarang tahan multi-hari outage main NATS tanpa drop publisher.

### Disk impact (post-change)

ZFS compression 5.57x masih efektif:

| Resource | Limit | Used now | Free |
|---|---:|---:|---:|
| Edge `/home/landa` (physical) | 133 GiB | **11 GiB** | **122 GiB** |
| Edge JS `max_storage` (logical) | 250 GiB | 29 GiB | 221 GiB |
| Edge JS sum stream max_bytes | 230 GiB | 29 GiB | 201 GiB |
| Edge logical-used (zfs `logicalused`) | — | 89.6 GiB | (5.57x compress ratio) |
| Main `/dev/sda1` (physical) | 290 GiB | 27 GiB | 263 GiB |
| Main JS `max_storage` (logical) | 250 GiB | 15 GiB | 235 GiB |
| Main JS sum stream max_bytes | 220 GiB | ~86 GiB | 134 GiB |

Worst-case (semua edge stream hit cap simultan): 230 GiB logical × 1/5.57 ≈ 41 GiB physical → masih 80+ GiB free di disk edge. Aman.

### Capacity alerting deployed

§9 "Alerting setup (not yet done)" — sekarang aktif via Discord webhook.

| Item | Path / Value |
|---|---|
| Script | `/home/landa/scripts/nats-capacity-alert.sh` (bash + `nats` CLI + `jq` + `curl`) |
| Env (mode 0600) | `/home/landa/scripts/.nats-alert.env` — `EDGE_TOKEN`, `MAIN_TOKEN`, `DISCORD_WEBHOOK_URL` |
| State (anti-spam) | `/home/landa/scripts/.nats-alert-state.json` (per-stream level cache) |
| Log | `/home/landa/scripts/nats-capacity-alert.log` |
| Cron | `*/5 * * * *` di `landa@10.10.8.2` crontab |
| Coverage | 8 streams: edge + main × {IDX_TICK, IDX_QUOTE, IDX_META, IDX_NEWS} |
| Thresholds | Escalate at **80 / 85 / 90 / 95 / 99%**; recovery message di <75% |
| Output | Discord webhook (channel internal `tuai`) |

Alert logic ringkas (file ini implementasi referensi — sesuaikan kalau dibutuhkan):

```
for each (server, stream):
  pct = bytes / max_bytes
  level_new = bucket(pct)   # 0/80/85/90/95/99
  if level_new > level_prev:
    notify Discord (escalation)
  elif pct < 75 and level_prev > 0:
    notify Discord (recovery)
  save state
```

Manual debug:
```bash
ssh landa@10.10.8.2 '/home/landa/scripts/nats-capacity-alert.sh'
# stdout: "OK \"edge:IDX_TICK=24% edge:IDX_QUOTE=31% ... main:IDX_NEWS=0%\""
```

### Loss summary

**16,811,402 record lost from NATS pipeline since publisher start.** Most heavily affected window: 2026-05-20 09:37–10:44 UTC (= 16:37–17:44 WIB) yang overlap dengan EOD `idx.tradedone.*` finalization burst — implikasi langsung: kemungkinan banyak baris di QuestDB `trades` untuk 2026-05-20 yang `buyer='--'` atau `seller='--'`.

Recovery path partial:
- `idx.trade.*`, `idx.tradedone.*`, `idx.resend.*` — *mungkin* recoverable kalau IQPlus kirim ulang dan stream sudah ada room (post-fix sekarang punya 91 GiB headroom)
- `idx.quote.*` — **tidak recoverable** (IQPlus tidak punya resend history untuk quote snapshot)
- `idx.order.*` — depends pada IQPlus side

Untuk audit konkret, query QuestDB:
```sql
SELECT count(*) AS total,
       sum(case when buyer != '--' then 1 else 0 end) AS broker,
       sum(case when buyer  = '--' then 1 else 0 end) AS dash
FROM trades
WHERE timestamp >= '2026-05-20T00:00:00Z'
  AND timestamp <  '2026-05-21T00:00:00Z';
```
(Same pattern seperti di [iqplus-eod-investigation-2026-04-30.md §2](iqplus-eod-investigation-2026-04-30.md#2-initial-problem).)

### Backup files (this session)

```
Edge: ~/nats-edge/conf/nats-server.conf.bak.20260520-224317  (pre step 1)
Edge: ~/nats-edge/conf/nats-server.conf.bak.20260520-230806  (pre step 2)
```

Rollback prosedur sama dengan §7.

### Lessons (tambahan ke §10)

6. **Audit `max_file_store` setiap kali disk di-upgrade**. SSD di-upgrade jadi 140 GiB di §2 (2026-05-08) tapi server cap tetap 80 GB sampai hari ini — 12 hari unused capacity. Setelah disk upgrade fisik, selalu re-evaluate NATS server-level caps.

7. **Per-stream sizing untuk `discard:new` spool stream harus pakai survival-day budget, bukan steady-state-only headroom**. §4 original sizing (~30% headroom) cocok untuk stream `discard:old`, tapi untuk spool dengan fail-loud semantic, sizing dengan multi-day burst buffer (3x+ steady) memberikan margin yang jauh lebih bermakna terhadap incident main outage.

8. **ZFS compression bikin physical-cost dari logical max_bytes sangat murah** — 5.57x ratio observed di edge berarti 1 GiB max_bytes ≈ 0.18 GiB physical. Tidak ada alasan menyizakan headroom besar selama disk fisik mendukung.

9. **Publisher tidak retry on 10077 ack error**. Setiap saturasi = data loss langsung pada subject yang terlalu volume-heavy untuk fit di cap. Alerting (deployed sekarang) cuma deteksi awal — perbaikan fundamentalnya adalah memastikan stream cap selalu jauh di atas steady-state. Future work: tambah retry-on-10077 di [iqplus-publisher service](../../internal/modules/stock/iqplus_publisher/service/service.go) dengan exponential backoff, supaya transient saturation tidak langsung jadi data loss.
