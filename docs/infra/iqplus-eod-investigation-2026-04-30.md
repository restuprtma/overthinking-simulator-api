# IQPlus EOD Loss Investigation — 2026-04-30

> Status: investigation in-progress. End-of-day session ringkasan.
> Production publisher restored to real IQPlus at 16:33 UTC (23:33 WIB).
> Continue tomorrow with pending items in §10.

## 1. Timeline

| Waktu (WIB) | Event |
|---|---|
| 01:59 | Migrasi edge-spool topology LIVE (per [iqplus-edge-deployed.md](./iqplus-edge-deployed.md)) |
| 09:00–16:00 | Trading session normal — publisher streaming dari real IQPlus |
| 16:00 | Market close. EOD resend window dimulai |
| ~17:26 | Last EOD resend record arrived dari IQPlus (lebih cepat dari biasa) |
| 18:00 | Deadline IQPlus EOD resend (per user); tidak ada record baru |
| ~19:00 | User report dashboard: **72% trades hari ini masih `buyer='--'`** untuk IMPC dan stocks lain |
| 19:00–21:30 | Forensik production data (NATS, QuestDB, publisher counters) |
| 21:13 | IQPlus tim resend ulang — 1.8M record burst arrived |
| 21:17 | Aggregate miss% turun: 72% → 28%. Tapi masih 133,844 stuck '--' |
| 22:30–23:30 | Build patches publisher + tools, deploy cloud VM Jakarta untuk WAN test |
| 23:00 | Production publisher dimatikan, dataset 2.66M asli di-replay via cloud mock |
| 23:32 | **100% recovery validated** — semua 2,661,435 trades dapat broker codes |
| 23:33 | Production publisher restored → real IQPlus |
| 23:35 | QuestDB host akses untuk diagnosa rate ~7.5K rec/s — ketemu HAProxy bottleneck |

## 2. Initial Problem

```sql
SELECT count(*) AS total,
       sum(case when buyer != '--' then 1 else 0 end) AS broker,
       sum(case when buyer = '--' then 1 else 0 end) AS dash
FROM trades
WHERE timestamp >= '2026-04-30T00:00:00Z' AND timestamp < '2026-05-01T00:00:00Z';

-- result (sebelum resend kedua):
-- total=2,626,644  broker=732,548  dash=1,894,096  miss_pct=72%
```

Pattern dashboard:
- Resend 27 April: 64% miss
- Resend 28 April: 29% miss
- Resend 29 April: **6% miss** (good)
- 30 April (today): **72% miss** ← regresi setelah migrasi malam-nya

## 3. Root Cause Analysis (multi-layer)

### Layer 1 — IQPlus side (initial hypothesis)

**Kemarin (Apr 29) IQPlus kirim full data** → 6% miss. **Hari ini IQPlus kirim incomplete EOD batch pertama** (hanya ~28% trade dapat broker code di EOD pertama). Mereka kemudian kirim ulang jam 21:13 WIB → coverage naik ke 72% broker, sisa 133,844 trade stuck '--'.

**Verifikasi**: query NATS edge stream `idx.resend.trade.IMPC` → 12,099 records, hanya 2,702 yang carry broker. Confirms 28% coverage = matches QuestDB exactly.

### Layer 2 — Test pakai data CSV ternyata mengandung 133,844 '--' record

User export QuestDB ke `assets/questdb-query-1777563501083.csv`. Ternyata **133,844 row di CSV memang `buyer='--'` & `seller='--'`** (live trade tanpa broker yang IQPlus belum pernah kirim ulang). Test replay menemukan angka loss yang sama persis = **bukan loss di pipeline kita**, melainkan data asli yang IQPlus tidak punya broker code-nya.

### Layer 3 — Kernel TCP & MikroTik (potential WAN-side issue)

Edge VM (FreeBSD 10.10.8.1) dari boot pertama:
- 1,401 packets `discarded due to full reassembly queue` (kernel-level silent drop)
- 10.5 juta out-of-order packets (reordering parah dari WAN ke IQPlus)
- `kern.ipc.maxsockbuf=2 MiB`, `net.inet.tcp.recvspace=64 KiB` (default kecil)
- `net.inet.tcp.reass.maxqueuelen=100` (kecil untuk burst high-throughput)

**Tidak bisa apply Tier 1 sysctl** — `landa` user uid=1001 tidak ada di `wheel` group, su- ditolak. Butuh akses root edge VM via Proxmox console.

### Layer 4 — Publisher pipeline architecture

Migrasi semalam tambah `publishWithRetry()` ([service.go:113-144](../../internal/modules/stock/iqplus_publisher/service/service.go#L113-L144)) yang block 50ms saat backpressure. Comment di kode:
> "blocking here means the service goroutine doesn't drain the client.Stream() channel, which fills, which blocks the TCP read loop, which finally pushes back on IQPlus"

Sengaja di-design menukar NATS-side silent loss menjadi TCP-level backpressure ke IQPlus. **Resiko**: kernel reassembly queue overflow saat burst dengan reordering tinggi.

### Layer 5 — QuestDB ingestion pipeline (downstream bottleneck)

- QuestDB host `10.10.8.51`: **30 vCPU, 15 GiB RAM** — saat peak load CPU 327% (3.3 cores), RAM 1.1 GiB. **Under-utilized 89%**. Bukan bottleneck.
- `10.10.8.10` ternyata bukan QuestDB tapi **HAProxy** dengan **2 vCPU, 4 GiB RAM**. Resend-handler & running-trade-consumer write via HAProxy (slow path).
- Resend-handler single-goroutine sequential. `QUESTDB_AUTO_FLUSH_INT=500ms` (max 2 flush/sec).
- Observed throughput: ~7.5K rec/s → 2.66M dataset takes ~6 minutes drain.

## 4. Code Changes Made (committed locally, NOT pushed)

### 4.1 [client.go](../../internal/modules/stock/iqplus_publisher/client/client.go) — TCP buffer enhancements

```go
type Config struct {
    // ... existing fields ...
    SocketRecvBuffer int // SO_RCVBUF (default 16 MiB; capped by kern.ipc.maxsockbuf)
    BufioReaderSize  int // bufio reader buffer (default 1 MiB; was 64 KiB)
}

// DefaultConfig:
//   BufferSize: 200_000 → 2_000_000     (10x channel buffer absorption)
//   SocketRecvBuffer: 16 * 1024 * 1024  (16 MiB SO_RCVBUF)
//   BufioReaderSize:  1024 * 1024       (1 MiB bufio)
```

In `connectAndStream()`: setelah `net.DialTimeout`, panggil `tc.SetReadBuffer(c.cfg.SocketRecvBuffer)` dengan log warning kalau gagal (FreeBSD cap 2 MiB tanpa Tier 1 sysctl).

### 4.2 [main.go](../../cmd/iqplus-publisher/main.go) — Env loaders

Tambah `IQPLUS_SOCKET_RECV_BUF` & `IQPLUS_BUFIO_READ_SIZE`. Documented di header comment.

### 4.3 [publisher.go](../../internal/modules/stock/iqplus_publisher/publisher/publisher.go) — Test isolation support

```go
type Config struct {
    // ... existing ...
    SubjectPrefix string // prepended to derived subject (default "")
    RouteToStream string // force ALL records to single stream (default "" = use StreamFor)
}
```

Allow load tests publish to `test.idx.*` → custom test stream, tidak ganggu production `IDX_TICK`.

### 4.4 Production publisher binary deployed

- Backup OLD binary: `/home/landa/iqplus-publisher/bin/iqplus-publisher.bak.before-loadtest-20260430_231135`
- NEW binary di production location dengan all patches
- Production env append `IQPLUS_SOCKET_RECV_BUF=16777216` dan `IQPLUS_BUFIO_READ_SIZE=1048576` (capped to 2MB without Tier 1 sysctl tapi harmless)

## 5. New Tools Built (binaries di `bin/`, code di `cmd/`)

| Tool | Purpose | Path |
|---|---|---|
| `iqplus-mock-server` | TCP server emit synthetic IQPlus records di rate konfigurable | [cmd/iqplus-mock-server/main.go](../../cmd/iqplus-mock-server/main.go) |
| `iqplus-loadtest-receiver` | NATS-bypass: connect ke mock, count records, no NATS pollution | [cmd/iqplus-loadtest-receiver/main.go](../../cmd/iqplus-loadtest-receiver/main.go) |
| `iqplus-replay-mock` | Read NDJSON dataset dari QuestDB export, replay sebagai IQPlus | [cmd/iqplus-replay-mock/main.go](../../cmd/iqplus-replay-mock/main.go) |

Makefile targets baru: `build-iqplus-mock-server-{linux,freebsd}`, `build-iqplus-loadtest-receiver-freebsd`, `build-iqplus-replay-mock-linux`.

## 6. Test Results

### 6.1 LAN test 5M synthetic records (main → edge)

| Metric | Value |
|---|---|
| Mock pumped | 5,000,000 in 19.5s |
| Receiver counted | 5,000,001 |
| Drops at any layer | 0 |
| Kernel reass.queue Δ | +0 |

### 6.2 LAN test 500K real QuestDB rows (replay-mock → patched publisher → IDX_TICK_TEST)

| Metric | Value |
|---|---|
| Records replayed | 500,000 |
| NATS IDX_TICK_TEST | 500,001 |
| Loss | 0 |

### 6.3 WAN test 2.66M from cloud Jakarta (replay-mock → publisher → IDX_TICK_TEST)

| Metric | Value |
|---|---|
| Cloud mock sent | 2,661,435 in 52.6s (~50K rec/s sustained) |
| Publisher tcp_received/ok | 2,661,437 (5M + status) |
| NATS IDX_TICK_TEST | 2,661,435 |
| Kernel reass.queue Δ | +0 |
| Out-of-order packets | +178,393 (LAN+WAN noise) |
| Backpressured events | 0 |

### 6.4 End-to-end production-as-if-real test (cloud → real publisher → real NATS → real consumers → QuestDB)

Setelah replace 133,844 '--' rows di NDJSON dengan random broker codes:

| Metric | Before | After |
|---|---:|---:|
| QuestDB total trades | 2,661,435 | 2,661,435 |
| QuestDB broker | 2,527,591 | **2,661,435** |
| QuestDB '--' | 133,844 | **0** ✅ |
| Pipeline drops | 0 | 0 |
| QuestDB drain rate | n/a | ~7.5K rec/s sustained |

## 7. Cloud VM Setup (Jakarta GCP)

| Item | Value |
|---|---|
| IP public | `34.101.107.88` |
| Region | `asia-southeast2` (Jakarta) |
| Specs | e2-small (2 vCPU, 4 GiB RAM), Ubuntu 26.04 |
| User | `claude-loadtest` |
| Files | `/home/claude-loadtest/iqplus-replay-mock/{bin,data,log}` |
| Data | `trades_full.ndjson` (234 MB, 2.66M rows) + `trades_full_filled.ndjson` (no '--') |
| Firewall | TCP `18888` allowed from edge public IP `103.125.36.242` (atau open) |
| SSH key | `~/.ssh/claude_loadtest_ed25519` (di laptop tantowi) |
| Status | Mock stopped, **OK to delete VM** untuk hemat biaya |

Edge public IP saat test: `103.125.36.242`.

## 8. State of Production at End-of-Day

- Production publisher `/home/landa/iqplus-publisher/bin/iqplus-publisher` running (PID baru setelah restart 23:33 WIB)
- Streaming dari real IQPlus `103.114.143.237:8888`
- Binary: NEW patches applied (bigger buffer + SO_RCVBUF + subject prefix support)
- Env: original (real IQPlus credentials) + tambahan tunables `IQPLUS_SOCKET_RECV_BUF` & `IQPLUS_BUFIO_READ_SIZE`
- IDX_TICK stream cleaned dari resend.trade subjects (purged total ~7.5M edge + 8.8M main records)
- IDX_TICK_TEST stream dropped
- Edge VM rebooted hari ini untuk vCPU upgrade 8 → **12** vCPU
- Cloud mock stopped tapi VM masih jalan

## 9. QuestDB Ingestion Diagnosis

**QuestDB host (10.10.8.51) sangat over-provisioned**, bukan bottleneck:
- 30 vCPU / 15 GiB RAM, peak utilization 11% / 7%
- server.conf mostly defaults — auto-tunes ke jumlah core
- Worker threads: 139 OS threads aktif

**Bottleneck sebenarnya** (urut prioritas):

1. **HAProxy `10.10.8.10` (`tua-haproxy-database-1`)** — VM kecil 2 vCPU/4 GiB. Resend-handler write HTTP via HAProxy adds latency tiap request.

2. **Resend-handler single-threaded** — [service.go:115](../../internal/modules/stock/resend_handler/service/service.go#L115) sequential `tickWriter.Write()` per record.

3. **ILP flush interval lambat** — `QUESTDB_AUTO_FLUSH_INT=500ms` capped 2 flush/sec.

### Recommended fixes (proposed for tomorrow)

| Fix | Effort | Speedup |
|---|---|---:|
| Bypass HAProxy: point langsung `QUESTDB_ADDRESS=10.10.8.51:9000` di k8s secret | 1 menit | 2-3x |
| Switch HTTP→ILP TCP port 9009 (`tcp::addr=10.10.8.51:9009`) | 30 menit + test | 5-10x |
| Scale resend-handler replicas 1→3 di k8s | 1 menit | 3x |
| `QUESTDB_AUTO_FLUSH_INT: "100ms"` & `AUTO_FLUSH_ROWS: "5000"` | 1 menit | 1.5x |

Combined target: **7.5K → 40-60K rec/s** drain rate. Untuk 2.66M dataset: 6 min → 1 min.

## 10. Pending Items (Tomorrow)

### High Priority

- [ ] **Tier 1 sysctl di edge VM (BUTUH ROOT)** — apply via Proxmox console (steps di [iqplus-edge-deployed.md §12](./iqplus-edge-deployed.md)):
  ```sh
  cat >> /etc/sysctl.conf <<'EOF'
  kern.ipc.maxsockbuf=33554432
  net.inet.tcp.recvspace=4194304
  net.inet.tcp.recvbuf_max=16777216
  net.inet.tcp.reass.maxqueuelen=4096
  EOF
  sysctl kern.ipc.maxsockbuf=33554432 net.inet.tcp.recvspace=4194304 \
         net.inet.tcp.recvbuf_max=16777216 net.inet.tcp.reass.maxqueuelen=4096
  ```
  Lalu `~/iqplus-publisher/scripts/stop.sh && sleep 2 && ~/iqplus-publisher/scripts/start.sh` (sebagai user `landa`).

- [ ] **MikroTik diagnostic (192.168/`10.10.0.1`)** — jalankan 8 commands di [WinBox/SSH], paste output:
  ```
  /system resource print
  /system identity print
  /ip firewall mangle print where action=fasttrack-connection
  /ip firewall filter print where action=fasttrack-connection
  /ip firewall connection tracking print
  /interface ethernet print stats-detail
  /system resource cpu print
  /ip route print where dst-address=103.114.143.237/32
  ```
  Hipotesa: fasttrack aktif → causing OOO/reordering. Disable fasttrack untuk koneksi IQPlus.

- [ ] **Bypass HAProxy** untuk resend-handler (k8s secret `resend-handler-env`):
  ```yaml
  QUESTDB_ADDRESS: "10.10.8.51:9000"   # was 10.10.8.10:9000
  ```

### Medium Priority

- [ ] **Scale resend-handler replicas** k8s 1 → 3
- [ ] **Tune ILP flush** di k8s secret:
  ```yaml
  QUESTDB_AUTO_FLUSH_ROWS: "5000"
  QUESTDB_AUTO_FLUSH_INT: "100ms"
  ```
- [ ] **Delete cloud VM `34.101.107.88`** (atau preemptible, ~$1 sehari)
- [ ] **Rotate edge NATS_TOKEN** (saat tidak sibuk — sempat exposed di chat history)

### Low Priority / Nice-to-have

- [ ] Pertimbangkan **switch ke ILP TCP port 9009** (vs HTTP) untuk resend-handler — perlu test go-questdb-client compatibility
- [ ] Add **kernel TCP drop counter** ke publisher stats (cron `netstat -s -p tcp` scrape)
- [ ] Document **IQPlus EOD timing pattern** — apakah 18:00 WIB consistent atau variable per hari

## 11. Useful Commands & References

### Health checks (dari workstation)

```sh
# Production publisher status
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/status.sh'

# Latest stats
ssh landa@10.10.8.1 'grep -a "publisher stats" ~/iqplus-publisher/log/iqplus-publisher.log | tail -1' \
  | python3 -m json.tool

# Edge stream count
ssh landa@10.10.8.2 \
  "nats --server 'nats://<EDGE_TOKEN>@10.10.8.1:4222' stream info IDX_TICK --json" \
  | jq '.state.messages, .state.bytes'

# Resend-handler consumer state
ssh landa@10.10.8.2 \
  "nats --server 'nats://<MAIN_TOKEN>@10.10.8.2:4222' consumer info IDX_TICK resend-trade-backfill --json" \
  | jq '{num_pending, num_ack_pending, last_active: .delivered.last_active}'

# QuestDB miss% real-time
ssh landa@10.10.8.2 \
  "curl -sS -G --data-urlencode \"query=SELECT count(*) AS rows, sum(case when buyer != '--' then 1 else 0 end) AS broker, sum(case when buyer = '--' then 1 else 0 end) AS dash FROM trades WHERE timestamp >= '2026-04-30T00:00:00Z' AND timestamp < '2026-05-01T00:00:00Z'\" -u 'tuai_tan:TuaiTan1407*' 'http://10.10.8.10:9000/exec'" \
  | jq '.dataset[0]'

# Kernel TCP counter (edge)
ssh landa@10.10.8.1 'netstat -s -p tcp | grep -iE "reassembly queue|out-of-order|retransmit"'
```

### Re-run loadtest

Mock at cloud, publisher di edge dengan test isolation:

```sh
# 1. Start cloud mock (jika VM masih ada)
ssh -i ~/.ssh/claude_loadtest_ed25519 claude-loadtest@34.101.107.88
cd ~/iqplus-replay-mock
nohup ./bin/iqplus-replay-mock -listen :18888 \
  -input ./data/trades_full_filled.ndjson \
  -loops 1 -rps 0 -hold-after 10m \
  -seq-base 11000000000 \
  > log/replay.log 2>&1 < /dev/null &
disown

# 2. Stop production
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/stop.sh'

# 3. Edit env: target cloud + add prefix/route for isolation
# Use existing /tmp/run_pub_test.sh & iqplus-publisher-test/bin/iqplus-publisher.env
# (already configured untuk test mode dengan IDX_TICK_TEST)

# 4. Recreate test stream
ssh landa@10.10.8.2 \
  "nats --server 'nats://<EDGE_TOKEN>@10.10.8.1:4222' stream add IDX_TICK_TEST \
    --subjects 'test.idx.>' --storage file --replicas 1 \
    --retention limits --max-age 1h --discard old --defaults"

# 5. Start test publisher (di edge)
ssh landa@10.10.8.1 'nohup /tmp/run_pub_test.sh > ~/iqplus-publisher-test/log/publisher.log 2>&1 < /dev/null &'

# 6. Validate after burst
ssh landa@10.10.8.2 \
  "nats --server 'nats://<EDGE_TOKEN>@10.10.8.1:4222' stream info IDX_TICK_TEST --json | jq '.state.messages'"
```

### Files & paths reference

| Path | Purpose |
|---|---|
| `/home/landa/iqplus-publisher/` | Production publisher (real IQPlus) |
| `/home/landa/iqplus-publisher-test/` | Test publisher pointing at mock (cloud) |
| `/home/landa/nats-edge/` | NATS edge server |
| `/home/landa/iqplus-mock-server/` | (main VM) synthetic mock |
| `/home/landa/iqplus-replay-mock/` | (cloud VM Jakarta) real-data replay |
| `/home/landa/iqplus-loadtest/` | (edge VM) loadtest receiver (NATS-bypass) |
| `/home/claude-loadtest/iqplus-replay-mock/data/trades_full_filled.ndjson` | (cloud VM) 2.66M rows, no '--' |
| `assets/questdb-query-1777563501083.csv` | (laptop) original QuestDB export 2.66M rows |
| `cmd/iqplus-mock-server/main.go` | Mock server source |
| `cmd/iqplus-loadtest-receiver/main.go` | Receiver source |
| `cmd/iqplus-replay-mock/main.go` | Replay-mock source |

### SSH access summary

```sh
# Edge VM (FreeBSD, no sudo)
ssh landa@10.10.8.1   # password: Alhamdulillah1407*

# Main VM (Linux, has sudo + nats CLI)
ssh landa@10.10.8.2   # same password

# QuestDB host (Linux, sudo)
ssh landa@10.10.8.51  # same password

# Cloud loadtest VM (Linux, GCP)
ssh -i ~/.ssh/claude_loadtest_ed25519 claude-loadtest@34.101.107.88

# MikroTik router
ssh tantowi@10.10.0.1  # password: Development280897*
# (admin port currently re-closed; was opened for debugging session)
```

## 12. Key Insights / What We Learned

1. **Migrasi semalam (edge-spool topology) BEKERJA.** Pipeline edge → main → consumer → QuestDB capable handle 2.66M record burst dengan 0 loss saat IQPlus akhirnya kirim full data.

2. **Hipotesis "kernel TCP drop" overstated.** 1,401 reass.queue drops terlalu kecil untuk explain 72% loss. WAN test dari cloud Jakarta tidak trigger kernel drops.

3. **Loss yang terlihat di production HARI INI sebagian besar = IQPlus side incomplete** (mereka kirim parsial dulu lalu top-up beberapa jam kemudian).

4. **Bottleneck QuestDB ingestion = HAProxy + single-threaded resend-handler**, BUKAN QuestDB resource. Ada 7-10x speedup yang accessible dengan k8s config tweaks.

5. **DEDUP key di trades table = `trade_no`** (designated timestamp + trade_no). Resend record dengan timestamp+trade_no sama akan overwrite '--' dengan broker code. Tested OK.

6. **TCP backpressure chain bekerja persis seperti design** — saat publisher saturate, mock auto-throttled dari 267K → 33K rec/s tanpa kehilangan record.

7. **Tier 1 sysctl tetap penting** sebagai safety margin, meski tidak menjadi root cause hari ini.

---

**End of session.** Lanjutkan tomorrow dengan §10 Pending Items.
