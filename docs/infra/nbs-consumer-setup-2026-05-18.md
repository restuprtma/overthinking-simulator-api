# NBS Consumer — Setup & Migration 2026-05-18

> **Status**: code merged-ready, awaiting CREATE TABLE + Docker image build + kubectl apply.
> Follow-up to [jetstream-disk-upgrade-2026-05-08.md §11](jetstream-disk-upgrade-2026-05-08.md) (IDX_META resize) and [iqplus-eod-investigation-2026-04-30.md](iqplus-eod-investigation-2026-04-30.md).

---

## 1. Context

Hari ini terdeteksi dua masalah saling terkait di pipeline NBS (Net Buy/Sell, juga dikenal sebagai "broker summary"):

1. **IDX_META overflow** — saat NBS EOD burst jam 17:15 WIB, NATS edge tolak 474,734 record dengan `err_code=10077 maximum bytes exceeded`. **SUDAH DIFIX** dengan bump `max_bytes` 5 GiB → 15 GiB (lihat doc disk-upgrade §11).
2. **NBS consumer dead** — durable `nbs-aggregator` di main NATS punya `Last Delivery: never` sejak created 27 April. Backlog 20.7M record menumpuk tanpa diproses. Root cause: **tidak ada deployment artifact di k8s** — binary code-nya ada tapi tidak pernah deployed.

Dokumen ini mencatat keputusan + perubahan code yang dilakukan untuk memperbaiki #2: migrasi sink dari Redis (original design) ke QuestDB dual-table, plus melengkapi deployment artifact agar match pattern dominant consumer-consumer lain.

---

## 2. Discovery — what NBS actually is

### 2.1 IQPlus Type 58 & 59 = broker summary, dual-view

Per spec IQPlus Technical Specs v4.0.0:
- **Type 58 (NBS Stock)**: format `Stock|Broker|Bfreq|Bvol|Blot|Bval|Bpct|Sfreq|Svol|Slot|Sval|Spct`
- **Type 59 (NBS Broker)**: same 12 field, cuma order field 0 & 1 di-swap → `Broker|Stock|...`

Kedua-nya sama content, beda entry-point. Industry / Invezgo MCP menyebutnya `broker_summary_stock` & `broker_summary_broker`.

### 2.2 Snapshot stream — bukan periodic dump

Empirical observation dari NATS stream `IDX_META`: pair `(XL, ZYRX)` muncul **7 kali dalam 18 milidetik** (jam 17:10:17 WIB):

| Vendor seq | bfreq | bvol | sfreq | svol | sval |
|---:|---:|---:|---:|---:|---:|
| 41327571 | 1 | 800 | 0 | 0 | 0 |
| 41327579 | 1 | 800 | 1 | 800 | 111,200 |
| 41327582 | 2 | 1,800 | 1 | 800 | 111,200 |
| 41327583 | 2 | 1,800 | 2 | 1,800 | 250,200 |
| 41327603 | 2 | 1,800 | 3 | 1,900 | 264,100 |
| 41327625 | 2 | 1,800 | 4 | 3,100 | 430,900 |
| 41327631 | 2 | 1,800 | 5 | 4,200 | 583,800 |

IQPlus emit snapshot baru **setiap kali angka cumulative berubah**, bukan periodic. Tiap snapshot adalah **state lengkap** (bukan delta). Total IDX_META ~1.4M NBS msg/hari = avg ~22 update per (stock, broker) pair per hari.

### 2.3 F/D (Foreign/Domestic) bukan bagian dari NBS

NBS feed cuma 12 field cumulative — tidak ada split F/D. Field `buyer_type` / `seller_type` ("F" atau "D") ada di **Type 15 (Trade)** dan **Type 27 (Resend Trade)**, sudah tersimpan di QuestDB `trades.buyer_type` / `seller_type`. NBS by F/D harus di-derive dari aggregate `trades`, bukan dari `nbs_*` tables.

### 2.4 NBS-versus-`trades` redundancy

Secara matematis, NBS bisa diturunkan dari aggregate `trades`:
```sql
SELECT buyer AS broker, stock, count(*) AS bfreq, sum(volume) AS bvol, sum(volume*price) AS bval
FROM trades WHERE timestamp >= today() AND buyer != '--'
GROUP BY buyer, stock;
```

Tapi cross-check menunjukkan **trade-derived NBS lebih pendek dari NBS asli IQPlus** (sample XL|WBSANG: trades=74 vs NBS=161). Penyebab: (a) trades hari ini incomplete (489K record gap + 318K '--' broker per IQPlus EOD pattern, sama dengan 30 April), (b) NBS include negotiated/cash board lengkap.

Conclusion: keep NBS pipeline as a separate authoritative source — **bukan** sebagai cache redundant atas `trades`.

---

## 3. Decisions

### 3.1 Sink: Redis → QuestDB

**Original design** ([internal/modules/stock/nbs_consumer/sink/redis.go](../../internal/modules/stock/nbs_consumer/sink/redis.go) — *deleted*): Redis dual-hash `nbs:stock:<stock>` × broker dan `nbs:broker:<broker>` × stock. Tidak pernah deployed (dashboard kemungkinan baca dari Invezgo API atau tidak baca sama sekali).

**New design**: QuestDB dual-table — `nbs_stock` (Type 58) dan `nbs_broker` (Type 59). Alasan:
- Konsisten dengan archive design intent — CLAUDE.md & migration comment sudah menyebut `broker_summary live in QuestDB`.
- Same store sebagai `trades` — query lintas-table mudah (join broker activity vs raw trades).
- Time-series native — `LATEST ON timestamp` untuk current state, range scan untuk audit.
- Tidak butuh extra cache invalidation strategy.

### 3.2 Tabel terpisah (bukan satu tabel dengan kolom `source`)

Pertimbangan utama: routing per `n.Source` (58/59) sudah terjadi di sink, jadi nama table sendiri sudah encoded info `source`. Menyimpan `source LONG` column di tabel = redundan dengan nama table. Saved ~5M row/hari × 8 bytes = ~15 GB/tahun untuk info yang sudah implicit.

### 3.3 Designated timestamp = `envelope.ReceivedAt`

Bukan vendor `Date+Time` (HHMMSS WIB tanpa TZ, drift-prone). `ReceivedAt` adalah UTC dari publisher clock, di-stamp saat record sampai publisher dari IQPlus. Konsisten dengan resend-trade-consumer.

### 3.4 Tidak ada DEDUP UPSERT KEYS (untuk sekarang)

QuestDB constraint: designated timestamp HARUS masuk dedup keys. Pilihan praktis: `DEDUP UPSERT KEYS(timestamp, stock, broker)`.

Tapi NBS bersifat time-series snapshot — pair `(stock, broker)` muncul puluhan kali per hari dengan `ReceivedAt` berbeda di milidetik. DEDUP keys ini cuma kena saat exact replay JetStream (envelope identik). Trade-off:
- Pro DEDUP: replay-safe, multi-replica safe
- Kontra DEDUP: zero gain in normal operation, dapat overhead

**Keputusan**: skip dulu, evaluate setelah observe storage growth. **Konsekuensi: replicas harus 1** — multi-replica writes bisa duplicate kalau ada JetStream redeliver.

### 3.5 `market` column derived dari stock suffix

Pattern persis seperti `trades.market` — pakai `trade.DeriveMarket(stock)` dari [running_trade_consumer/trade](../../internal/modules/stock/running_trade_consumer/trade/):
- Code length ≥5 + suffix `NG` → market `NG`
- Code length ≥5 + suffix `TN` → market `TN`
- Else → market `RG`

`stock` column simpan **full vendor code** (`WBSANG`), `market` derived (`NG`). Konsisten 100% dengan `trades` table — operator gampang remember.

---

## 4. Schema

```sql
CREATE TABLE 'nbs_stock' (
    timestamp       TIMESTAMP,
    stock           SYMBOL INDEX CAPACITY 1024,
    broker          SYMBOL INDEX CAPACITY 256,
    market          SYMBOL,
    b_freq          LONG,
    b_vol           LONG,
    b_lot           LONG,
    b_val           LONG,
    b_pct           DOUBLE,
    s_freq          LONG,
    s_vol           LONG,
    s_lot           LONG,
    s_val           LONG,
    s_pct           DOUBLE,
    sequence        LONG,
    date            STRING,
    time            STRING,
    last_updated_at TIMESTAMP
) timestamp(timestamp) PARTITION BY DAY;

CREATE TABLE 'nbs_broker' (
    -- same shape as nbs_stock
);
```

**Column provenance** (added 2026-05-18 evening after observing snapshot redundancy):

| Column | Source | Type | Purpose |
|---|---|---|---|
| `timestamp` | `envelope.ReceivedAt` UTC | TIMESTAMP designated | Ordering + PARTITION |
| `date` | `envelope.Date` raw (e.g. `"20260518"`) | STRING | Vendor wall-clock day for display/audit |
| `time` | `envelope.Time` raw (e.g. `"171517"`) | STRING | Vendor wall-clock HHMMSS WIB for display |
| `last_updated_at` | `envelope.ReceivedAt` UTC | TIMESTAMP | Dashboard-friendly alias for `timestamp`; will diverge if DEDUP enabled later (then `timestamp` becomes day bucket) |

Catatan:
- **Tidak ada `source` column** — implicit dari nama table.
- **Tidak ada DEDUP** — lihat §3.4.
- **`sequence`** = `envelope.Sequence` (per-day unique IQPlus sequence) untuk audit / debugging traceability.
- **Volume estimate**: ~1.4M row/hari per table × 2 tables × 365 = ~1B row/tahun, ~100 GB/tahun (compressed lebih kecil di QuestDB).

---

## 5. Code changes

| File | Action |
|---|---|
| [internal/modules/stock/nbs_consumer/sink/questdb.go](../../internal/modules/stock/nbs_consumer/sink/questdb.go) | **NEW** — `Writer` interface, `QuestDBSink` dengan single LineSender. Routing per-row: `Source==58→StockTable`, `Source==59→BrokerTable`. `Apply(ctx, n, market, ts, seq)`. |
| `internal/modules/stock/nbs_consumer/sink/redis.go` | **DELETED** |
| [internal/modules/stock/nbs_consumer/service/service.go](../../internal/modules/stock/nbs_consumer/service/service.go) | Import `running_trade_consumer/trade`, call `trade.DeriveMarket(n.Stock)`, pass to sink. Tambah `FlushOnTick` + `FlushTimeout` pattern dari resend-trade. |
| [internal/modules/stock/nbs_consumer/main.nbs_consumer.go](../../internal/modules/stock/nbs_consumer/main.nbs_consumer.go) | `Sink sink.Writer` (interface), `Initialize` call `sink.NewQuestDB`. |
| [cmd/nbs-consumer/main.go](../../cmd/nbs-consumer/main.go) | Replace `REDIS_*` env vars → `QUESTDB_ADDRESS/STOCK_TABLE/BROKER_TABLE/AUTH_*/AUTO_FLUSH_*`. Tambah `envBool` helper untuk `FLUSH_ON_TICK`. |
| [cmd/nbs-consumer/.env.example](../../cmd/nbs-consumer/.env.example) | Sync ke env baru, default `QUESTDB_ADDRESS=10.10.8.51:9000` (bypass HAProxy). |

Subscriber TIDAK diubah — tetap pakai `FilterSubject` singular (`idx.nbs.>`) seperti sebelumnya. Comment di [subscriber.go:2-3](../../internal/modules/stock/nbs_consumer/subscriber/subscriber.go) sudah ack tech debt "6th near-identical subscriber, extract to shared `iqplus_subscriber`" — di-skip untuk PR ini.

`go vet` clean, `go build ./cmd/...` OK.

---

## 6. Deployment artifacts

Initial draft saya pakai pattern minoritas (meta-consumer-style: bare name `nbs-consumer`, image `:1.0.0` dengan suffix `-production`). User correct → re-aligned ke **pattern dominant** yang dipakai 7 dari 9 consumer (`tuai-be-` prefix, image `:latest`, dengan `deploy.sh`).

| File | Status | Purpose |
|---|---|---|
| [deployments/kubernetes/nbs-consumer/deployment.yaml](../../deployments/kubernetes/nbs-consumer/deployment.yaml) | NEW | Deployment `tuai-be-nbs-consumer`, image `venturoid/tuai-be-nbs-consumer:latest`, replicas=1, podAntiAffinity, hardened securityContext. Resources 100m/128Mi req → 1000m/512Mi limit (match resend-trade — NBS volume + EOD burst similar). |
| [deployments/kubernetes/nbs-consumer/secret.yaml.example](../../deployments/kubernetes/nbs-consumer/secret.yaml.example) | NEW | Secret `tuai-be-nbs-consumer-env`, includes NATS + QuestDB + service knobs. Two field harus diisi: `NATS_TOKEN`, `QUESTDB_AUTH_PASSWORD`. |
| [deployments/kubernetes/nbs-consumer/deploy.sh](../../deployments/kubernetes/nbs-consumer/deploy.sh) | NEW (+x) | 7-step automation: validate prereqs → git state → git push (Autobuild trigger) → in-cluster pull probe → bump tag → kubectl apply + rollout → verify QuestDB sink in startup log. Adapted from `quote-consumer/deploy.sh`. |
| [deployments/kubernetes/nbs-consumer/README.md](../../deployments/kubernetes/nbs-consumer/README.md) | NEW | Ops doc — target, network deps, CREATE TABLE DDL, query patterns (LATEST ON, market filter, board-aggregate), first-time deploy, rollback. |
| [deployments/docker/nbs-consumer.Dockerfile](../../deployments/docker/nbs-consumer.Dockerfile) | NEW | Per-consumer dedicated Dockerfile (bukan generic `consumer.Dockerfile`) — match pattern quote/tradedone/resend-*. |

### 6.1 Pattern compliance

| Aspek | quote-consumer (reference) | nbs-consumer |
|---|---|---|
| Deployment name | `tuai-be-quote-consumer` | `tuai-be-nbs-consumer` ✅ |
| Image | `venturoid/tuai-be-quote-consumer:latest` | `venturoid/tuai-be-nbs-consumer:latest` ✅ |
| imagePullPolicy | `Always` | `Always` ✅ |
| Secret name | `tuai-be-quote-consumer-env` | `tuai-be-nbs-consumer-env` ✅ |
| Strategy | RollingUpdate maxSurge=1, maxUnavailable=0 | sama ✅ |
| podAntiAffinity | weight 100, hostname | sama ✅ |
| nodeSelector | `nodetype: worker` | sama ✅ |
| securityContext | non-root 65532, RO rootfs, drop ALL | sama ✅ |
| Files | README + deploy.sh + deployment.yaml + secret.yaml.example | sama ✅ |
| Dockerfile | per-consumer dedicated | per-consumer dedicated ✅ |
| deploy.sh structure | 7-step (validate → git → push → wait → bump → apply → verify) | sama ✅ |

Justified differences:
- **Resources lebih besar** dari quote (100m/128Mi vs 50m/64Mi req) — match resend-trade karena NBS volume + EOD burst lebih heavy.
- **deploy.sh verify step** cek `questdb_addr` + `questdb_stock_table` + `questdb_broker_table` di startup log (instead of `redis_addr` + `redis_db`).

---

## 7. Outstanding — what's left

### High priority

- [ ] **Run CREATE TABLE di QuestDB** — DDL di §4 atau di README. Cara: paste di Web Console `http://10.10.8.51:9000` atau `curl -G --data-urlencode "query=..." -u "tuai_tan:<pwd>" http://10.10.8.51:9000/exec`.
- [ ] **Commit & push** code changes ke branch `production` — trigger Docker Hub Autobuild untuk image `venturoid/tuai-be-nbs-consumer:1.0.0`.
- [ ] **Apply secret + deployment** ke k8s:
  ```bash
  cd deployments/kubernetes/nbs-consumer
  cp secret.yaml.example secret.yaml
  # edit secret.yaml: NATS_TOKEN, QUESTDB_AUTH_PASSWORD
  kubectl --kubeconfig ../production.yaml apply -f secret.yaml -n tuai
  ./deploy.sh 1.0.0
  ```
- [ ] **Monitor initial drain** — backlog 20.7M record. Watch:
  ```bash
  kubectl logs -n tuai -l app.kubernetes.io/name=tuai-be-nbs-consumer -f | grep "nbs consumer stats"
  ```
  Expect `rate_per_sec` ramping ke beberapa ribu, lalu turun ke live rate (~puluhan/sec) setelah catch-up.
- [ ] **Sanity check row counts** setelah ~5 menit drain:
  ```sql
  SELECT count(*) FROM nbs_stock;   -- Type 58 count
  SELECT count(*) FROM nbs_broker;  -- Type 59 count
  ```
  Harus naik dan kira-kira balanced (vendor kirim kedua type sama frequency).

### Medium priority

- [ ] **Evaluate DEDUP** setelah observe storage growth (~1 minggu). Kalau replay duplicate menjadi masalah (atau mau enable multi-replica), DROP TABLE + recreate dengan `DEDUP UPSERT KEYS(timestamp, stock, broker)`. Note: tidak bisa di-ALTER, must recreate.
- [ ] **Bandingkan trade-derived NBS vs nbs_* table** — kalau angkanya mirip, validate IQPlus NBS accurate. Kalau beda jauh, investigate IQPlus delivery quality.

### Low priority / future

- [ ] **Extract shared `iqplus_subscriber`** package — sudah 6 subscriber near-identical, tech debt yang sudah di-acknowledge.
- [ ] **Materialized view atau cron aggregator** untuk pre-computed broker summary kalau dashboard performance bermasalah dengan `LATEST ON` query pattern.
- [ ] **F/D split broker summary** — derive dari `trades` table, expose sebagai endpoint terpisah (out of NBS pipeline scope).

---

## 8. Decision log / what we did NOT do

1. **Tidak drop NBS subscribe entirely** — sempat dipertimbangkan (consumer dead 3 minggu = bukti tidak ada yang baca), tapi user mau revive sebagai authoritative source bukan derived dari trades.
2. **Tidak pakai Redis (original design)** — Redis dual-hash bagus untuk O(1) read tapi: (a) duplicate state yang sudah ada di QuestDB by design, (b) tidak ada history time-series, (c) butuh extra cache invalidation strategy.
3. **Tidak split jadi 3 stream** — `nbs.>` tetap di IDX_META (sesama low-volume meta subjects). Sudah cukup dengan IDX_META resize hari ini.
4. **Tidak pakai shared `iqplus_subscriber`** — refactor tech debt, di-skip untuk PR ini supaya tetap fokus.
5. **Tidak set replicas=2** — wajib pair dengan DEDUP. Bisa di-bump nanti kalau drain time bermasalah, tapi require DDL recreate dulu.
6. **Tidak include `source` column** — implicit dari nama table, save ~15 GB/year storage redundant.

---

## 9. References

- [iqplus-eod-investigation-2026-04-30.md §9](iqplus-eod-investigation-2026-04-30.md) — QuestDB ingestion bottleneck (HAProxy), bypass dengan `10.10.8.51:9000` direct.
- [jetstream-disk-upgrade-2026-05-08.md §11](jetstream-disk-upgrade-2026-05-08.md) — IDX_META resize 5 → 15 GiB hari ini.
- [topology.md §5.7](topology.md) — NBS Aggregator architecture (need re-sync after this change — currently still says "Redis hash" + "QuestDB EOD snapshot phase 2").
- [iqplus-edge-deployed.md](iqplus-edge-deployed.md) — `nbs-aggregator` listed sebagai existing consumer (sudah lama, tapi tidak pernah aktif sampai hari ini).
- IQPlus Technical Spec v4.0.0 §1.12 (NBS Stock) / §1.13 (NBS Broker) — vendor docs, soft copy di local share.
