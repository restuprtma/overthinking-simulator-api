# JetStream Streams — Setup yang Dibutuhkan untuk IDX Data Feed

> Audience: tim yang manage NATS server `tuai-jetstream` (10.10.8.2).
> Sumber: [docs/infra/topology.md](../infra/topology.md) §4 dan
> [publisher.md](publisher.md).
>
> Publisher Go (`cmd/iqplus-publisher`) sudah mengirim record ke 4 stream
> berbeda. Dokumen ini menjelaskan **stream apa saja yang harus disiapkan
> di server side** sebelum publisher di-deploy ke FreeBSD VM.

---

## 1. Ringkasan

IDX Data Feed dipecah ke **4 stream** berdasarkan kelas data + retention.
Tidak satu stream `STOCKS` global — alasan: tick rate tinggi (24h cukup)
tidak boleh nyampur dengan news (perlu 7 hari).

| Stream      | Subjects                                                   | Retention | Estimasi rate |
| ----------- | ---------------------------------------------------------- | --------- | ------------- |
| `IDX_TICK`  | `idx.trade.>`, `idx.order.>`, `idx.tradedone.>`, `idx.resend.>` | 24h    | 5k–40k msg/s  |
| `IDX_QUOTE` | `idx.quote.>`, `idx.bestquote.>`                           | 12h       | 1k–10k msg/s  |
| `IDX_META`  | `idx.status.>`, `idx.activity.>`, `idx.summary.>`, `idx.top20.>`, `idx.nbs.>` | 24h | <100 msg/s |
| `IDX_NEWS`  | `idx.news.>`                                               | 7d        | <10 msg/s     |

Semua stream:

- **Storage**: file (bukan memory — tidak boleh hilang saat restart)
- **Replicas**: 3 (HA) — turunkan ke 1 kalau cluster cuma 1 node sementara
- **Discard policy**: `old` (drop data lama saat penuh, jangan reject publisher)
- **Duplicate window**: 2 menit (cocok dengan publisher Msg-Id `iqplus-<date>-<seq>`)
- **Max msg size**: 1 MB (default cukup; news terbesar ~5–10 KB setelah reassembly)

---

## 2. Setup via `nats` CLI

Asumsi `nats` CLI sudah ter-config dengan context yang point ke
`nats://10.10.8.2:4222` dan token admin.

```bash
# Check context dulu
nats context info

# IDX_TICK — high-volume tick data, 24h retention
nats stream add IDX_TICK \
  --subjects "idx.trade.>,idx.order.>,idx.tradedone.>,idx.resend.>" \
  --storage file \
  --replicas 3 \
  --retention limits \
  --max-age 24h \
  --max-msgs=-1 \
  --max-bytes=-1 \
  --max-msg-size 1MB \
  --discard old \
  --dupe-window 2m \
  --defaults

# IDX_QUOTE — quote + best quote, 12h retention
nats stream add IDX_QUOTE \
  --subjects "idx.quote.>,idx.bestquote.>" \
  --storage file \
  --replicas 3 \
  --retention limits \
  --max-age 12h \
  --max-msgs=-1 \
  --max-bytes=-1 \
  --max-msg-size 1MB \
  --discard old \
  --dupe-window 2m \
  --defaults

# IDX_META — session/activity/summary/top20/nbs, 24h retention
nats stream add IDX_META \
  --subjects "idx.status.>,idx.activity.>,idx.summary.>,idx.top20.>,idx.nbs.>" \
  --storage file \
  --replicas 3 \
  --retention limits \
  --max-age 24h \
  --max-msgs=-1 \
  --max-bytes=-1 \
  --max-msg-size 1MB \
  --discard old \
  --dupe-window 2m \
  --defaults

# IDX_NEWS — news content, 7 hari retention
nats stream add IDX_NEWS \
  --subjects "idx.news.>" \
  --storage file \
  --replicas 3 \
  --retention limits \
  --max-age 7d \
  --max-msgs=-1 \
  --max-bytes=-1 \
  --max-msg-size 1MB \
  --discard old \
  --dupe-window 2m \
  --defaults
```

> **Catatan replicas**: kalau cluster sekarang masih 1 node, ganti
> `--replicas 3` jadi `--replicas 1`. Naikkan ke 3 saat cluster sudah
> 3-node — bisa di-update via `nats stream edit IDX_TICK --replicas 3`.

---

## 3. Verifikasi setelah setup

```bash
# Daftar semua stream — harus muncul 4
nats stream ls

# Detail per stream
nats stream info IDX_TICK
nats stream info IDX_QUOTE
nats stream info IDX_META
nats stream info IDX_NEWS

# Verifikasi subjects map dengan benar (publish test message)
nats pub idx.trade.BBCA "test"
nats stream info IDX_TICK | grep -i message  # harus naik 1
```

---

## 4. Mapping Subject → Stream → Record Type

Publisher derive subject dari record type dan field di payload IQPlus.
Tabel ini sumber kebenaran (sinkron dengan
[`internal/modules/stock/iqplus_publisher/publisher/subjects.go`](../../internal/modules/stock/iqplus_publisher/publisher/subjects.go)
dan [topology.md §4.2](../infra/topology.md)):

| Record Type | Nama IQPlus      | Subject pattern                | Stream      |
| ----------- | ---------------- | ------------------------------ | ----------- |
| 13          | Control          | `idx.status.feed`              | `IDX_META`  |
| 14          | Quote (saham)    | `idx.quote.<stockcode>`        | `IDX_QUOTE` |
| 14          | Quote (regional) | `idx.quote.regional.<symbol>`  | `IDX_QUOTE` |
| 15          | Trade            | `idx.trade.<stockcode>`        | `IDX_TICK`  |
| 16          | Order            | `idx.order.<stockcode>`        | `IDX_TICK`  |
| 17          | Top 20           | `idx.top20.<category_code>`    | `IDX_META`  |
| 18          | Best Quote       | `idx.bestquote.<stockcode>`    | `IDX_QUOTE` |
| 26          | Resend Order     | `idx.resend.order.<stockcode>` | `IDX_TICK`  |
| 27          | Resend Trade     | `idx.resend.trade.<stockcode>` | `IDX_TICK`  |
| 36          | News             | `idx.news.<category>`          | `IDX_NEWS`  |
| 39          | Activity         | `idx.activity.market`          | `IDX_META`  |
| 40          | Trade Done       | `idx.tradedone.<stockcode>`    | `IDX_TICK`  |
| 57          | Trading Status   | `idx.status.session`           | `IDX_META`  |
| 58          | NBS Stock        | `idx.nbs.stock.<stockcode>`    | `IDX_META`  |
| 59          | NBS Broker       | `idx.nbs.broker.<brokercode>`  | `IDX_META`  |
| 130         | Trading Summary  | `idx.summary.<stype>.<board>`  | `IDX_META`  |

`<stockcode>` UPPERCASE persis dengan kode IDX (BBCA, BBRI, WIKA, KBAG-W).
`<symbol>` regional/komoditi/currency dengan `-` prefix dilucuti
(`-FTSE` → `FTSE`, `AUD-USD` → `AUD-USD`).

---

## 5. Yang Publisher Harapkan dari Server

Publisher pakai `WithExpectStream(<stream>)` di setiap publish (kontrol
via env `NATS_ENFORCE_STREAM=true`, default ON). Dampaknya:

- Kalau stream **belum dibuat**, publish gagal eksplisit dengan error
  `expected stream does not match`. Publisher akan log error dan increment
  `unrouted` counter — tidak silently drop.
- Kalau stream **dihapus** in-flight, publisher langsung tahu lewat error
  ack yang sama.

**Jadi urutan yang aman saat first deploy:**

1. Server side: bikin 4 stream pakai command di section 2 di atas.
2. Verifikasi: `nats stream ls` harus tampil keempatnya.
3. Publisher side: jalankan dengan `NATS_ENFORCE_STREAM=true`.

Kalau mau bring-up bertahap (publisher dulu sebelum stream selesai),
publisher bisa di-set `NATS_ENFORCE_STREAM=false` sementara, tapi message
akan **lenyap** kalau tidak ada stream yang capture (NATS Core fire-and-
forget). Hanya pakai mode ini untuk smoke test, jangan production.

---

## 6. Dedup Window — Cocok dengan Publisher Msg-Id

Setiap publish mengirim header `Nats-Msg-Id: iqplus-<date>-<seq>` (mis.
`iqplus-20260427-1234567`). Stream punya `--dupe-window 2m`, jadi:

- Network blip → publisher retry → server reject ack kedua sebagai
  duplicate. Stream tetap punya 1 entry. Counter `duplicates` di publisher
  naik (info, bukan error).
- Window 2 menit cukup untuk retry layer publisher (max ~5–10 detik) dan
  reasonable buffer untuk ack lag saat network spike.

Kalau ke depan retry window perlu lebih besar (mis. 5 menit), edit di
server tanpa perubahan publisher: `nats stream edit IDX_TICK --dupe-window 5m`.

---

## 7. Monitoring yang Direkomendasikan

Endpoint server: `http://10.10.8.2:8222/jsz` (JSON).

Field yang menarik per stream:

| Field           | Arti                                       | Alert ambang     |
| --------------- | ------------------------------------------ | ---------------- |
| `messages`      | Total entries di stream                    | drift cek harian |
| `bytes`         | Disk pakai per stream                      | >70% disk → alert|
| `num_subjects`  | Jumlah subject unik (≈ jumlah ticker)      | sanity check     |
| `consumer_count`| Berapa consumer subscribe                  | <expected → alert|

Topology.md §7.4 minta alert minimal:

- NATS pending messages per stream > 10.000 → Discord (consumer lag)
- Publisher koneksi disconnect saat trading hour → Discord + SMS
- Stream `bytes` per disk usage > 80% → Discord

---

## 8. Capacity Sizing Awal

Estimasi cepat untuk planning disk (sustained trading hour ~6 jam/hari):

| Stream      | Rate avg     | Avg msg size | 24h volume |
| ----------- | ------------ | ------------ | ---------- |
| `IDX_TICK`  | 10k msg/s    | 200 byte     | ~17 GB     |
| `IDX_QUOTE` | 3k msg/s     | 300 byte     | ~7 GB      |
| `IDX_META`  | 50 msg/s     | 200 byte     | ~85 MB     |
| `IDX_NEWS`  | 5 msg/s      | 5 KB         | ~2 GB/7hari|

Total ~25–30 GB hot disk dengan retention default. Multiply by replicas.
Provision NVMe minimal 200 GB free per node untuk margin (peak day, OS
overhead, snapshot).

---

## 9. Kontak / Eskalasi

- Publisher code owner: lihat `git log internal/modules/stock/iqplus_publisher`
- Topology owner: tim infra (lihat `docs/infra/topology.md` changelog)
- Vendor IQPlus: kontak di kontrak (untuk verify feed dari source)

---

## Changelog

| Versi | Tanggal    | Catatan                                                   |
| ----- | ---------- | --------------------------------------------------------- |
| 1.0   | 2026-04-27 | Initial — 4 stream setup, sync dengan topology.md v1.0    |
