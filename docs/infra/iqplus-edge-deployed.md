# IQPlus Edge Spool — As Deployed

> Status: **LIVE** since 2026-04-30 ~01:59 WIB
> Migration plan: [iqplus-edge-topology.md](./iqplus-edge-topology.md)
> This document captures the **actual deployed state** after migration completed.

---

## 1. Architecture (live)

```
┌──────────────────────────────────────────┐    ┌──────────────────────────────┐
│ FreeBSD VM 10.10.8.1 (user landa)        │    │ Ubuntu VM 10.10.8.2 (nats)   │
│                                          │    │                              │
│ ┌─────────────────┐                      │    │                              │
│ │ iqplus-publisher│                      │    │                              │
│ │ (Go 1.25)       │ NATS_URL=            │    │                              │
│ │                 │ 127.0.0.1:4222       │    │                              │
│ └────────┬────────┘                      │    │                              │
│          │ JetStream publish              │    │                              │
│          ▼                                │    │                              │
│ ┌─────────────────────────────────┐      │    │                              │
│ │ nats-edge (v2.10.20)            │      │    │ ┌──────────────────────────┐ │
│ │ JS domain "edge"                │ leaf │    │ │ nats-server (v2.12.7)    │ │
│ │ store: ~/nats-edge/data         │ ━━━━━┼━━━━┼▶│ JS domain "hub"          │ │
│ │ port 4222 (client)              │ 7422 │    │ │ store: /var/lib/nats     │ │
│ │ port 8222 (monitoring loopback) │      │    │ │ port 4222 (client)       │ │
│ │ Streams (replicas 1):           │      │    │ │ port 7422 (leaf listen)  │ │
│ │  - IDX_TICK   6h cap 12GB       │ ◄━━━━┼━━━━┼━│ port 8222 (monitor)      │ │
│ │  - IDX_QUOTE  6h cap 12GB       │ src  │    │ │ Streams (replicas 1):    │ │
│ │  - IDX_META  7d cap 15GB ⚐     │      │    │ │  - IDX_TICK 24h cap 30GB │ │
│ │  - IDX_NEWS  24h cap 12GB       │      │    │ │  - IDX_QUOTE 12h cap 15GB│ │
│ │  discard: NEW                   │      │    │ │  - IDX_META 24h cap 200M │ │
│ └─────────────────────────────────┘      │    │ │  - IDX_NEWS 7d cap 2GB   │ │
│                                          │    │ │  discard: OLD            │ │
└──────────────────────────────────────────┘    │ │  source: edge:<same name>│ │
                                                │ └────────┬─────────────────┘ │
                                                │          │ existing consumers│
                                                │          ▼                   │
                                                │ ohlcv-aggregator             │
                                                │ orderbook-events             │
                                                │ resend-trade-backfill        │
                                                │ tradedone-volume-profile     │
                                                │ orderbook-state-cache        │
                                                │ quote-state-cache            │
                                                │ meta-state-cache             │
                                                │ nbs-aggregator               │
                                                │ news-indexer                 │
                                                └──────────────────────────────┘
```

---

## 2. VM specs (final, after upgrade)

| | **Edge VM** (10.10.8.1) | **Main VM** (10.10.8.2) |
|---|---|---|
| OS | FreeBSD 14.4-RELEASE | Ubuntu 24.04.4 LTS |
| User | `landa` (uid 1001, **no sudo**) | `landa` (uid 1000, sudo) |
| CPU | 8 vCPU | 12 vCPU (Xeon Gold 6138) |
| RAM | 16 GB | 15 GB |
| Swap | — | **8 GB** (`/swapfile`) |
| Disk | 30 GB ZFS mirror (`zroot/home`) | **79 GB** ext4 (`/`) |
| FS path of relevance | `/home/landa/` | `/var/lib/nats/jetstream/` |

---

## 3. Software versions

| Component | Version | Where |
|---|---|---|
| nats-server (edge) | 2.10.20 | `/home/landa/nats-edge/bin/nats-server` |
| nats-server (main) | 2.12.7 | `/usr/local/bin/nats-server` |
| nats CLI | 0.3.2 | `/usr/local/bin/nats` (di main VM) |
| iqplus-publisher | Go 1.25, current branch + ack-semaphore patch | `/home/landa/iqplus-publisher/bin/iqplus-publisher` |

---

## 4. Configuration values (live)

### 4.1 Edge nats-server (`~/nats-edge/conf/nats-server.conf`)

```hocon
server_name: iqplus-edge
host: 0.0.0.0
port: 4222
http: 127.0.0.1:8222

authorization {
    token: $NATS_TOKEN          # rendered at start time from env
    timeout: 2
}

jetstream {
    store_dir: /home/landa/nats-edge/data/jetstream
    max_memory_store: 256MB
    max_file_store: 28GB        # was 20GB — bumped 2026-04-30 untuk fit 24h retention
    domain: edge
    sync_interval: 2s
}

leafnodes {
    remotes = [
        { url: "nats-leaf://leaf:$LEAF_TOKEN@10.10.8.2:7422", no_randomize: true }
    ]
}

max_payload: 8MB
max_pending: 256MB
max_connections: 64
```

### 4.2 Main nats-server (`/etc/nats/nats-server.conf`)

```hocon
server_name: tuai-jetstream
host: 10.10.8.2
port: 4222
http: 10.10.8.2:8222

authorization {
    token: "<MAIN_TOKEN>"       # nilai sebenarnya; rotate periodically
    timeout: 2
}

jetstream {
    store_dir: /var/lib/nats/jetstream
    max_memory_store: 4GB
    max_file_store: 60GB        # was 20GB before upgrade
    sync_interval: "2m"
    domain: hub                 # added during migration
}

leafnodes {
    host: 10.10.8.2
    port: 7422
    authorization {
        user: "leaf"
        password: "<LEAF_TOKEN>"   # rotate periodically
        timeout: 5
    }
}
```

### 4.3 Stream configs (live)

> ⚐ **Partial update 2026-05-18**: only the `IDX_META` row reflects current
> runtime values (bumped from 5 GiB → 15 GiB per
> [jetstream-disk-upgrade-2026-05-08.md §11](jetstream-disk-upgrade-2026-05-08.md)).
> Other rows still show 30-April values — the 2026-05-08 disk upgrade
> superseded server-level + per-stream limits broadly; that doc is the
> source of truth for current sizing. Re-sync this whole table at the
> next infra touchup.

| Stream | Subjects | Edge max-age / max-bytes | Main max-age / max-bytes | Discard |
|---|---|---|---|---|
| `IDX_TICK`  | `idx.trade.>`, `idx.order.>`, `idx.tradedone.>`, `idx.resend.>` | **24h** / 12 GiB | 24h / **30 GiB** | edge: new / main: old |
| `IDX_QUOTE` | `idx.quote.>`, `idx.bestquote.>` | **24h** / 12 GiB | 12h / **15 GiB** | edge: new / main: old |
| `IDX_META`  | `idx.status.>`, `idx.activity.>`, `idx.summary.>`, `idx.top20.>`, `idx.nbs.>` | **7d / 15 GiB** ⚐ | 14d / 30 GiB | edge: new / main: old |
| `IDX_NEWS`  | `idx.news.>` | 24h / 12 GiB | 7d / 2 GiB | edge: new / main: old |

Main streams have `sources: [{ name: "<same>", external: { api: "$JS.edge.API" } }]` — pulls from edge via leaf.

### 4.4 Publisher config (`~/iqplus-publisher/bin/iqplus-publisher.env`)

Yang berubah dari sebelumnya:

```diff
-NATS_URL=nats://10.10.8.2:4222
+NATS_URL=nats://127.0.0.1:4222
```

Sisanya tidak berubah. NATS_TOKEN tetap pakai token main (di-share dengan edge supaya publisher gak perlu ganti token).

---

## 5. Tokens & credentials

> Nilai sebenarnya BUKAN di-commit. Disimpan di env files (mode 0600) di host masing-masing.

| Token | Purpose | Lokasi nilainya |
|---|---|---|
| `MAIN_TOKEN` | Client auth ke main NATS (10.10.8.2:4222) | `/etc/nats/nats-server.conf` (pertama kali di-set), juga di `~/iqplus-publisher/bin/iqplus-publisher.env` di edge |
| `EDGE_TOKEN` | Client auth ke edge NATS (10.10.8.1:4222) | `~/nats-edge/conf/nats-edge.env` di edge VM. **Sama dengan MAIN_TOKEN** untuk simplification — boleh dipisah nanti |
| `LEAF_TOKEN` | Leafnode auth: edge → main:7422 | `~/nats-edge/conf/nats-edge.env` (edge side) **DAN** `/etc/nats/nats-server.conf` (`leafnodes.authorization.password`) |

> ⚠️ **Rotation reminder**: Password VM (landa user di kedua VM) sempat di-share di chat history; harus rotate. NATS tokens & leaf token belum di-share di chat — relatif aman, tapi rotation tetap good practice setiap 90 hari.

---

## 6. Filesystem layout

### 6.1 Edge VM (10.10.8.1)

```
/home/landa/
├── iqplus-publisher/              # existing
│   ├── bin/
│   │   ├── iqplus-publisher       # patched binary (ack-semaphore + retry)
│   │   ├── iqplus-publisher.bak.20260430
│   │   ├── iqplus-publisher.env   # NATS_URL=nats://127.0.0.1:4222
│   │   └── iqplus-publisher.env.bak.20260430
│   ├── log/iqplus-publisher.log
│   ├── run/{daemon,publisher}.pid
│   └── scripts/{start,stop,status,rotate-log}.sh
│
└── nats-edge/                     # NEW
    ├── bin/nats-server            # v2.10.20 freebsd-amd64
    ├── conf/
    │   ├── nats-server.conf       # template ($NATS_TOKEN, $LEAF_TOKEN placeholders)
    │   └── nats-edge.env          # mode 0600 — NATS_TOKEN + LEAF_TOKEN
    ├── data/jetstream/            # JetStream file store
    ├── log/{nats-server.log,cron.log}
    ├── run/
    │   ├── daemon.pid
    │   ├── nats-server.pid
    │   └── nats-server.rendered.conf  # auto-generated tiap start (token substitusi)
    └── scripts/{install-binary,start,stop,status,streams-add}.sh
```

### 6.2 Main VM (10.10.8.2)

```
/etc/nats/
├── nats-server.conf                  # current (v2 with leafnode + domain hub)
├── nats-server.conf.bak.20260429-193153   # pre-migration
├── nats-server.conf.bak.20260430          # mid-migration backup
└── nats-server.conf.bak.20260430b         # before max_file_store 60GB

/var/lib/nats/jetstream/             # owner: nats:nats
/var/log/nats/nats-server.log

/etc/systemd/system/nats-server.service   # User=nats, ExecReload=SIGHUP

/swapfile                            # 8 GB, mode 0600, in /etc/fstab
```

---

## 7. Operational commands

### 7.1 Health check (run from workstation)

```bash
# Publisher health
ssh landa@10.10.8.1 'tail -1 ~/iqplus-publisher/log/iqplus-publisher.log' \
  | grep "publisher stats" \
  | python3 -c 'import json,sys; print(json.dumps(json.loads(next(sys.stdin)), indent=2))'

# Edge stream state
nats --server "nats://<EDGE_TOKEN>@10.10.8.1:4222" stream report

# Main stream state + replication lag
nats --server "nats://<MAIN_TOKEN>@10.10.8.2:4222" stream report

# Leafnode connection from main side
ssh landa@10.10.8.2 'curl -sS http://10.10.8.2:8222/leafz | python3 -m json.tool | head -10'
```

### 7.2 Edge restart (no production impact karena edge buffer leaf)

```bash
ssh landa@10.10.8.1 '~/nats-edge/scripts/stop.sh && sleep 2 && ~/nats-edge/scripts/start.sh'
ssh landa@10.10.8.1 '~/nats-edge/scripts/status.sh'
```

### 7.3 Main restart (~5-10s downtime, leaf reconnect, publisher buffer)

```bash
ssh landa@10.10.8.2 'sudo systemctl restart nats-server'
sleep 8
ssh landa@10.10.8.2 'sudo systemctl status nats-server --no-pager'
```

### 7.4 Reload main config tanpa restart (kalau ada perubahan config kecil)

```bash
ssh landa@10.10.8.2 'sudo systemctl reload nats-server'
ssh landa@10.10.8.2 'sudo tail -5 /var/log/nats/nats-server.log'
```

### 7.5 Publisher rebuild & cutover

```bash
# Dari workstation
make build-iqplus-publisher-freebsd
scp bin/iqplus-publisher-freebsd-amd64 landa@10.10.8.1:~/iqplus-publisher/bin/iqplus-publisher.new

ssh landa@10.10.8.1 '
cd ~/iqplus-publisher/bin
mv iqplus-publisher iqplus-publisher.bak.$(date +%Y%m%d)
mv iqplus-publisher.new iqplus-publisher
chmod +x iqplus-publisher
~/iqplus-publisher/scripts/stop.sh
sleep 2
~/iqplus-publisher/scripts/start.sh
'
```

### 7.6 Auto-start saat reboot (sudah ke-set)

Di crontab `landa@10.10.8.1`:
```
@reboot /home/landa/nats-edge/scripts/start.sh >> /home/landa/nats-edge/log/cron.log 2>&1
@reboot sleep 5 && /home/landa/iqplus-publisher/scripts/start.sh >> /home/landa/iqplus-publisher/log/cron.log 2>&1
5 0 * * * /home/landa/iqplus-publisher/scripts/rotate-log.sh >> /home/landa/iqplus-publisher/log/cron.log 2>&1
```

Edge start dulu, sleep 5s, baru publisher (untuk handle race condition saat boot).

---

## 8. Code patches (deployed)

Versus pre-migration. Lihat git diff untuk detail.

### 8.1 [publisher.go](../../internal/modules/stock/iqplus_publisher/publisher/publisher.go)

- Sentinel errors: `ErrPublishBackpressure`, `ErrPublishPermanent`
- Ack tracker pakai semaphore (`8 × AsyncMaxPending` slot, default 256k) — sebelumnya 1 goroutine per message → 5M+ goroutine saat resend burst
- Stat counter baru: `AckUntracked`
- Error categorization: `nats.ErrMaxPayload` = permanent, `ErrConnectionClosed` + queue full = backpressure (retriable)

### 8.2 [service.go](../../internal/modules/stock/iqplus_publisher/service/service.go)

- New `publishWithRetry()` method dengan loop retry 50ms saat `ErrPublishBackpressure`
- Permanent error → drop & log (no infinite loop)
- Stat counter baru: `backpressured`, `svc_dropped`

### 8.3 Result (compared to pre-patch over 3h cumulative)

| Counter | OLD (3h) | NEW (~20min steady-state) |
|---|---|---|
| `err_queue` (silent loss) | 2,977 | **0** |
| `err_ack_timeout` | 15,636 | **0** |
| `dropped` | 2,080 | **0** |
| `backpressured` (new metric) | n/a | 0 (off-hour) |

Real test = market opening burst & EOD resend.

---

## 9. Issues encountered during migration & resolutions

| # | Issue | Cause | Resolution |
|---|---|---|---|
| 1 | NATS gagal restart setelah config edit | `no_tls` bukan field valid di NATS HOCON; `token` tidak support di leafnodes auth | Hapus `no_tls`, ganti `token` → `user/password` |
| 2 | Edge leaf gagal auth (Authorization Violation berulang) | `$LEAF_TOKEN` tidak ke-expand di dalam quoted URL string oleh HOCON parser | Render config saat start time pakai `sed` substitution di `start.sh` |
| 3 | `nats stream update` gagal di IDX_TICK & IDX_QUOTE (context deadline) | Default 5s timeout tidak cukup untuk stream besar (22M+, 9M msg) | Pakai `--timeout 60s` |
| 4 | `nats stream update` minta interactive confirmation | Default behavior CLI | Pakai `--force` flag |
| 5 | Publisher ENV `LEAF_TOKEN` not used (start.sh) | Kelupaan validation | Tambah check di start.sh + render template |

---

## 10. Validation results (post-migration, 2026-04-30 ~02:00 WIB, off-hour)

| Item | Status |
|---|---|
| Publisher `consumed = ok` (0 drop) | ✅ |
| Publisher `err_queue = 0` | ✅ |
| Publisher `err_ack_timeout = 0` | ✅ (was 15k+ in old binary) |
| Edge → Main replication lag | ✅ 0 across all 4 streams |
| Existing 9 consumers state preserved | ✅ |
| Leaf bridge active | ✅ RTT 1.5ms |
| Edge disk usage | ✅ 23 MB / 20 GB cap |

Real test: 2026-05-01 09:00 WIB opening burst.

---

## 11. Capacity headroom (post-upgrade)

### Daily projected workload
- IDX_TICK: ~12 GB/day
- IDX_QUOTE: ~6 GB/day
- IDX_META: <0.5 GB/day
- IDX_NEWS: KB/day
- **Total: ~18 GB/day**

### Main VM capacity
| Layer | Capacity | Steady-state usage | Headroom |
|---|---|---|---|
| Disk `/` | 79 GB | ~25 GB (JetStream + OS) | 54 GB free |
| `max_file_store` | 60 GB | ~25 GB | 35 GB |
| Sum stream `max_bytes` | 47 GB (30+15+0.2+2) | ~25 GB | 22 GB |
| RAM | 15 GB | 1.7 GB nats peak | 13 GB |
| Swap | 8 GB | 0 | 8 GB |

### Edge VM capacity
| Layer | Capacity | Steady-state (24h spool) | Headroom |
|---|---|---|---|
| Disk `zroot/home` | 30 GB | ~19 GB | 11 GB |
| `max_file_store` | 28 GB | ~19 GB | 9 GB |
| RAM | 16 GB | 70 MB nats + 38 MB pub | 15.9 GB |

---

## 12. Pending follow-ups

| Priority | Item | Why | Estimated effort |
|---|---|---|---|
| 🟡 Soon | Setup Prometheus + Grafana monitoring | Visibility ke disk usage / lag / err_queue trends | 1-2 hari |
| 🟡 Soon | Daily `nats stream backup` cronjob | Disaster recovery untuk consumer state | 30 menit |
| 🟡 Soon | Rotate password landa@VM (kedua VM) | Sempat di-share di chat history | 10 menit |
| 🟢 Nice | Edge disk expand 30→50 GB + retention TICK/QUOTE 6h→24h | Buffer kalau main outage panjang | 30 menit |
| 🟢 Nice | FreeBSD sysctl tuning (butuh root edge) | TCP socket buffer untuk opening burst | 5 menit (kalau ada akses root) |
| 🟢 Nice | ZFS tuning di edge (`atime=off`, lz4 compression) | Marginal write speedup | 5 menit (butuh root) |

---

## 13. References

- Migration design: [iqplus-edge-topology.md](./iqplus-edge-topology.md)
- Edge deploy guide: [../deployments/freebsd/nats-edge/README.md](../../deployments/freebsd/nats-edge/README.md)
- Main config additions: [../deployments/main-nats/README.md](../../deployments/main-nats/README.md)
- Pre-migration streams (deprecated): [../JetStream/streams.md](../JetStream/streams.md)
- Topology overview: [./topology.md](./topology.md)
