# IQPlus → JetStream — Edge Spool Topology

> Status: PROPOSED. Designed 2026-04-29 untuk mengatasi resend-loss yang
> terlihat 2026-04-26..28 di mana ~64% trade resend tidak sampai JetStream
> di hari pertama, turun bertahap setelah server resource di-scale up.
>
> Replaces flat topology di [topology.md](./topology.md) §4 untuk leg
> IQPlus → JetStream. Konsumen-konsumen downstream (running-trade-consumer,
> news-consumer, dll) tidak terpengaruh — subjects & stream names di main
> tetap sama.

---

## 1. Motivasi

### Pola lama

```
IQPlus TCP ──┐
             ▼
   iqplus-publisher (FreeBSD VM 10.10.8.1)
   ├─ TCP read loop                      (in-mem chan, 200k buffer)
   ├─ js.PublishMsgAsync per record      (in-mem, AsyncMaxPending 32k×4)
   └─ goroutine per ack                  (1 per record — millions selama burst)
             │
             ▼ network LAN
   main NATS cluster (10.10.8.2)
   └─ IDX_TICK / IDX_QUOTE / IDX_META / IDX_NEWS
```

### Failure mode yang terjadi

1. **EOD resend burst** → IQPlus push 5–10 juta record dalam beberapa menit
   (record type 26/27).
2. `PublishMsgAsync` returns `nats: max async pending` ketika 32k×4 in-flight
   penuh. Service log it as DEBUG dan **lanjut** ke record berikutnya =
   silent loss ([sebelum patch](../../internal/modules/stock/iqplus_publisher/service/service.go)).
3. `go p.trackAck(...)` spawn 1 goroutine per record. Pada burst 5M record,
   itu 5M goroutine = ~10 GB stack = OOM atau scheduler thrash.
4. Network hiccup antara FreeBSD VM ↔ main NATS = TCP socket di-backpressure
   ke IQPlus → IQPlus disconnect → reconnect → tidak ada replay live (cuma
   resend EOD yang juga terkena masalah #1).

### Pola baru (target)

```
IQPlus TCP ──┐
             ▼
   FreeBSD VM 10.10.8.1
   ├─ iqplus-publisher
   │    └─ publish ke 127.0.0.1:4222            (loopback, sub-ms)
   │
   └─ nats-edge (jetstream domain "edge")
        ├─ store_dir: /home/landa/nats-edge/data/jetstream
        ├─ streams: IDX_TICK / QUOTE / META / NEWS (24h retention, discard NEW)
        └─ leafnode → main hub
                          │
                          ▼ network LAN
   main NATS cluster (10.10.8.2, jetstream domain "hub")
   └─ IDX_TICK / IDX_QUOTE / IDX_META / IDX_NEWS
       └─ source: $JS.edge.API → edge:IDX_TICK (etc)
```

### Dampak

| Properti | Sebelum | Sesudah |
|---|---|---|
| Hot-path publish latency | network LAN (sub-ms s/d 10ms) | loopback + fsync (~1ms) |
| Toleransi LAN partition | 0 (publisher silent-drop saat queue full) | tahan jam-an (edge tampung di disk) |
| Toleransi main NATS outage | publisher mati / silent-drop | tahan jam-an (edge tampung) |
| Goroutine count saat burst | unbounded (~5M) | bounded (`8 × AsyncMaxPending`) |
| AsyncQueueError handling | log debug + drop | retry dengan backpressure ke TCP |
| Disk storage | NATS cluster only | NATS cluster + edge spool (~5–15 GB/day) |

---

## 2. Komponen

### 2.1 Edge nats-server (NEW)

| Item | Value |
|---|---|
| Lokasi | FreeBSD VM 10.10.8.1, user `landa` |
| Deploy root | `/home/landa/nats-edge/` |
| Listen | `0.0.0.0:4222` (publisher loopback + admin) |
| Monitor | `127.0.0.1:8222` (HTTP) |
| Storage | `/home/landa/nats-edge/data/jetstream/` (~30 GB cap) |
| JetStream domain | `edge` |
| Leafnode | outbound → `nats-leaf://10.10.8.2:7422` |

Stream config di edge — **berbeda dari main**:

| Stream | Subjects | max_age | discard | replicas |
|---|---|---|---|---|
| `IDX_TICK`  | `idx.trade.>`, `idx.order.>`, `idx.tradedone.>`, `idx.resend.>` | 6h | **new** | 1 |
| `IDX_QUOTE` | `idx.quote.>`, `idx.bestquote.>`                                | 6h | **new** | 1 |
| `IDX_META`  | `idx.status.>`, `idx.activity.>`, `idx.summary.>`, `idx.top20.>`, `idx.nbs.>` | 24h | **new** | 1 |
| `IDX_NEWS`  | `idx.news.>`                                                    | 24h | **new** | 1 |

Catatan:
- **`discard: new`** (BUKAN `old`). Kalau stream penuh → reject publisher →
  dapat `ErrPublishBackpressure` → retry. Tidak boleh diam-diam buang data lama.
- **Replicas 1**: edge single-node. Durability cukup dari fsync + leafnode
  replicate ke main (yang punya replicas 3).
- **Retention pendek**: edge cuma transit. Source-of-truth tetap di main.

### 2.2 Main NATS cluster (existing, modifikasi minor)

Tambahan ke `nats-server.conf`:

```hocon
leafnodes {
  port: 7422
  no_tls: true                              # LAN private
  authorization { token: $LEAF_TOKEN }
}

jetstream {
  # ... existing config ...
  domain: hub                               # WAJIB di-set untuk cross-domain source
}
```

Stream config di main — **subjects sama**, tapi sekarang **sourced dari edge**:

| Stream | Subjects (sama) | max_age | discard | replicas | source |
|---|---|---|---|---|---|
| `IDX_TICK`  | (sama) | 24h | old | 3 | `IDX_TICK` @ domain `edge` |
| `IDX_QUOTE` | (sama) | 12h | old | 3 | `IDX_QUOTE` @ domain `edge` |
| `IDX_META`  | (sama) | 24h | old | 3 | `IDX_META` @ domain `edge` |
| `IDX_NEWS`  | (sama) | 7d  | old | 3 | `IDX_NEWS` @ domain `edge` |

> Konsumen downstream (running-trade-consumer, news-consumer, meta-consumer, dll)
> **tidak perlu diubah**. Mereka subscribe ke subjects `idx.*.>` yang sama,
> di stream IDX_* yang sama, di server NATS yang sama. Data datangnya beda
> (via stream-source dari edge alih-alih langsung dari publisher) — itu
> transparan.

### 2.3 iqplus-publisher (modifikasi)

Perubahan code di repo ini:

- [publisher.go](../../internal/modules/stock/iqplus_publisher/publisher/publisher.go) — sentinel errors `ErrPublishBackpressure` & `ErrPublishPermanent`; ack-tracker dibatasi semaphore (`8 × AsyncMaxPending` slot, ~256k untuk default config).
- [service.go](../../internal/modules/stock/iqplus_publisher/service/service.go) — `publishWithRetry()` loop: retry 50ms backoff selama dapat `ErrPublishBackpressure`. Permanent error (oversize/marshal) → drop & log.

Dampak observability — ada metric baru di stats line:
- `ack_untracked`: berapa publish ack-nya tidak ditrack karena semaphore penuh (ack tetap akan datang, di-handle di async err handler — bukan loss).
- `backpressured`: berapa kali kita retry karena queue full. Spike di sini = signal bahwa edge atau main tidak keep up dengan publisher.
- `svc_dropped`: berapa record dropped karena permanent error.

Perubahan env (`.env`):

```diff
-NATS_URL=nats://10.10.8.2:4222
+NATS_URL=nats://127.0.0.1:4222
-NATS_TOKEN=<main-token>
+NATS_TOKEN=<edge-token>
```

Token edge di-generate sekali saat deploy edge, share dengan publisher dan
main (untuk leaf auth). Lihat `deployments/freebsd/nats-edge/README.md`.

---

## 3. Setup leafnode

Bagian paling tricky karena melibatkan dua server. Urutan eksekusi penting.

### 3.1 Generate token leaf

```bash
LEAF_TOKEN=$(openssl rand -hex 32)
# Simpan di password manager. Akan dipakai DUA tempat:
#  - main: `leafnodes.authorization.token` → $LEAF_TOKEN
#  - edge: `leafnodes.remotes[0].url` → nats-leaf://$LEAF_TOKEN@10.10.8.2:7422
```

### 3.2 Apply ke main NATS

1. Backup `nats-server.conf` di main.
2. Append blok dari `deployments/main-nats/leafnode-server-additions.conf`.
3. Pastikan ada `domain: hub` di section `jetstream {}`.
4. Set env `LEAF_TOKEN` di service unit / start script main.
5. Reload NATS:
   ```bash
   # Cara 1: signal reload (tidak putus koneksi, kalau supported)
   ssh root@10.10.8.2 'pkill -SIGHUP nats-server'

   # Cara 2: restart penuh
   ssh root@10.10.8.2 'systemctl restart nats-server'
   ```
6. Verify leafnode listener:
   ```bash
   nats --context tuai-jetstream server check connections
   curl -s http://10.10.8.2:8222/varz | jq .leafnodes  # harus > 0 setelah edge up
   ```

### 3.3 Apply ke edge

Edge config sudah include `leafnodes.remotes` block (lihat
[`nats-server.conf`](../../deployments/freebsd/nats-edge/nats-server.conf)).
Tinggal isi `LEAF_TOKEN` di env file:

```bash
ssh landa@10.10.8.1 'cat >> ~/nats-edge/conf/nats-edge.env <<EOF
LEAF_TOKEN=<token-yang-sama>
EOF'
ssh landa@10.10.8.1 '~/nats-edge/scripts/stop.sh && sleep 2 && ~/nats-edge/scripts/start.sh'
```

### 3.4 Verify leaf up dari sisi main

```bash
# Dari workstation, pakai context tuai-jetstream:
nats --context tuai-jetstream req '$JS.edge.API.INFO' '' --timeout 3s

# Sukses kalau dapat JSON response berisi info JetStream edge.
# "no responder" = leaf belum up; cek log kedua server.
```

### 3.5 Apply main streams dengan source

```bash
export EDGE_TOKEN=<edge-token>           # token client edge, bukan leaf
export MAIN_NATS_CONTEXT=tuai-jetstream
sh deployments/main-nats/streams-with-edge-source.sh
```

Verify:

```bash
nats --context tuai-jetstream stream info IDX_TICK | grep -A5 Sources
# Harus terlihat:
#   Sources:
#     IDX_TICK
#       Stream: IDX_TICK
#       Active: <recent>
#       Lag: 0
```

---

## 4. Migration dari pola lama

> **Asumsi**: existing publisher sudah jalan di `landa@10.10.8.1`,
> publish ke main NATS (`10.10.8.2`).

### Strategi: cutover dengan window singkat (~5 menit downtime)

1. **Off-hour** (idealnya weekend / setelah 16:00 WIB).
2. Deploy edge nats-server (lihat `deployments/freebsd/nats-edge/README.md`).
   Setup leafnode (3.1–3.4 di atas).
3. **Stop publisher** existing:
   ```bash
   ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/stop.sh'
   ```
4. Drop & recreate main streams sebagai sourced (3.5 di atas).
5. Build & deploy publisher baru (yang sudah include patch
   [publisher.go](../../internal/modules/stock/iqplus_publisher/publisher/publisher.go) +
   [service.go](../../internal/modules/stock/iqplus_publisher/service/service.go)):
   ```bash
   make build-iqplus-publisher-freebsd
   scp bin/iqplus-publisher-freebsd-amd64 \
       landa@10.10.8.1:~/iqplus-publisher/bin/iqplus-publisher
   ```
6. Update `iqplus-publisher.env` di host:
   ```diff
   -NATS_URL=nats://10.10.8.2:4222
   +NATS_URL=nats://127.0.0.1:4222
   -NATS_TOKEN=<main-token>
   +NATS_TOKEN=<edge-token>
   ```
7. Start publisher:
   ```bash
   ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/start.sh'
   ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/status.sh'
   ```
8. Validate end-to-end dalam 30 detik pertama:
   - publisher log: `iqplus publisher stats consumed=...,ok=...,err_queue=0`
   - edge `nats stream info IDX_TICK` (dari workstation lewat
     `nats://10.10.8.1:4222`): messages naik
   - main `nats stream info IDX_TICK`: messages naik dengan lag minimal
   - downstream consumer (running-trade-consumer, dll): lag tidak balon

### Rollback (kalau ada masalah)

Lihat `deployments/main-nats/README.md` bagian "Rollback".

---

## 5. Tradeoffs & batasan

### Apa yang BERHASIL diatasi

- ✅ Resend burst silent-loss → producer dapat backpressure, retry sampai bisa.
- ✅ LAN partition main ↔ edge → edge tampung sampai disk penuh (24h+).
- ✅ Main NATS restart/maintenance → edge tampung; resync setelah leaf reconnect.
- ✅ Goroutine explosion saat burst → semaphore-bounded.

### Apa yang TIDAK diatasi

- ❌ Edge VM (10.10.8.1) crash hilang power → record yang masih di OS write
  cache (≤ `sync_interval=2s` dari config) hilang. Mitigasi: kalau penting,
  set `sync_interval: 200ms` (latency hit).
- ❌ Disk edge mati → spool hilang. Mitigasi: pakai ZFS mirror untuk
  `data/jetstream/`. Kalau Proxmox disk pakai stripe/RAID-0, kurangi
  retention edge ke 6h (loss bounded).
- ❌ IQPlus side outage → tetap perlu menunggu resend EOD. Edge tidak bisa
  fabricate data yang tidak diterima dari socket.
- ❌ Token leaf bocor → siapapun yang punya token bisa konek ke main sebagai
  leaf. Mitigasi: rotate token periodically; restrict 10.10.8.2:7422 ke
  10.10.8.1 saja via firewall.

### Disk planning

Untuk Proxmox storage layout:

| Mode | Aman untuk edge spool? | Recommended retention |
|---|---|---|
| ZFS mirror / RAIDZ | ✅ | 24-48h (default) |
| ZFS stripe / LVM stripe / linear | ⚠️ | **6h** (loss bounded) |
| Single disk SSD | ⚠️ | 6h (loss bounded) |

Cek mode di Proxmox host:
```bash
zpool status        # ZFS
lvs -o +stripes     # LVM
```

---

## 6. Observability checklist

Setelah migration, monitor 3 layer:

| Layer | Cek | Healthy state |
|---|---|---|
| Publisher | `iqplus-publisher` stats log tiap 30s | `err_queue=0` (atau low), `backpressured` flat saat non-burst |
| Edge | `nats stream info IDX_TICK` di edge | messages naik, `state.first_ts` rolling 6h, `consumer_count=1` (leaf) |
| Main | `nats stream info IDX_TICK` di main | messages naik, `Sources.Lag = 0` saat steady, naik saat burst tapi turun balik |

Alert kalau:
- Publisher `backpressured` rate > 100/min sustained → main NATS slow / disk issue.
- Main `Sources.Lag` > 30s sustained → leafnode bermasalah atau main tidak keep up.
- Edge disk usage > 80% → retention tidak jalan, atau leaf down lama.

---

## 7. References

- [deployments/freebsd/nats-edge/README.md](../../deployments/freebsd/nats-edge/README.md) — deploy guide edge
- [deployments/main-nats/README.md](../../deployments/main-nats/README.md) — main NATS modifications
- [docs/JetStream/streams.md](../JetStream/streams.md) — pre-migration streams config (deprecated setelah migration)
- [docs/infra/topology.md](./topology.md) — overall topology, subjects naming
- [NATS Leaf Nodes](https://docs.nats.io/running-a-nats-service/configuration/leafnodes) — official docs
- [JetStream Stream Sources](https://docs.nats.io/nats-concepts/jetstream/streams#sources) — official docs
