# WebSocket Gateway — Live OHLCV Candles

**Version:** v1
**Endpoint:** `wss://ws.tuai.id/ws/candles`
**Module:** Stock — Live Streaming
**Last Updated:** 2026-04-28

---

## Overview

Gateway WebSocket untuk **streaming candle (OHLCV) per saham IDX** secara real-time. Cocok untuk komponen chart trading di FE (TradingView lightweight-charts, Highcharts, dll).

**Cara kerja singkat:**

1. Client connect ke `/ws/candles` (WS upgrade).
2. Client kirim `subscribe` — sebut saham + timeframe yang mau dipantau.
3. Server **langsung kirim snapshot** state saat ini dari Redis (1 pesan per pasangan stock × tf).
4. Server **push update** setiap kali ada trade baru (real-time, sub-detik).
5. Saat menit/jam berganti, server kirim `closed` untuk bar lama lalu mulai update bar baru.

> **`1m` itu ukuran bucket bar, BUKAN interval push.** Update tetap real-time setiap tick — yang `1m` itu rentang waktu yang di-agregat per bar.

---

## Endpoint

| | |
|---|---|
| URL | `wss://ws.tuai.id/ws/candles` |
| HTTP fallback | `ws://ws.tuai.id/ws/candles` (tidak direkomendasikan untuk produksi — Cloudflare auto-upgrade) |
| Health check | `GET https://ws.tuai.id/healthz` → `{"status":"ok"}` |
| Auth | **Tidak ada** (Phase 1) — endpoint terbuka. Production harus difront reverse-proxy yang validasi session. |
| Origin policy | Konfigurable. Saat ini allow-all. |

### Mixed-content rule (browser)

- Page `https://...` → wajib pakai `wss://` (Cloudflare provide TLS).
- Page `http://...` → boleh `ws://` atau `wss://`.

---

## Connection lifecycle

```
Client                                Server
  │                                     │
  │ 1. WS Upgrade (HTTP 101)            │
  ├────────────────────────────────────>│
  │<────────────────────────────────────┤
  │                                     │
  │ 2. {"action":"subscribe", ...}      │
  ├────────────────────────────────────>│
  │                                     │
  │ 3a. {"type":"snapshot", ...} (Redis)│
  │<────────────────────────────────────┤
  │                                     │
  │ 3b. {"type":"update", ...} (live)   │
  │<────────────────────────────────────┤  ← stream selama bucket aktif
  │<────────────────────────────────────┤
  │<────────────────────────────────────┤
  │                                     │
  │ 4. {"type":"closed", ...}            │  ← bucket selesai (mis. 09:00:59 → 09:01:00)
  │<────────────────────────────────────┤
  │                                     │
  │ 5. {"type":"update", ...} (new bar) │
  │<────────────────────────────────────┤
  │                                     │
  │ 6. {"action":"unsubscribe", ...}    │  (opsional — atau langsung close)
  ├────────────────────────────────────>│
  │                                     │
  │ 7. WS Close                         │
  ├────────────────────────────────────>│
  │<────────────────────────────────────┤
```

**Server tidak push apa-apa sampai client kirim `subscribe`** — kalau Anda hanya open lalu diam, Message Log akan kosong.

---

## Client → Server messages

Semua frame berbentuk **JSON text**. Satu frame = satu command.

### `subscribe`

Daftarkan kombinasi cartesian dari `stocks × timeframes`.

```json
{
  "action": "subscribe",
  "stocks": ["BBCA", "BMRI", "TLKM"],
  "timeframes": ["1m", "5m"]
}
```

Contoh di atas akan subscribe **6 stream** (BBCA-1m, BBCA-5m, BMRI-1m, BMRI-5m, TLKM-1m, TLKM-5m).

| Field | Type | Required | Catatan |
|---|---|---|---|
| `action` | string | ✓ | Harus `"subscribe"` |
| `stocks` | string[] | ✓ | IDX ticker uppercase (4 huruf), mis. `"BBCA"`. Min 1 item. |
| `timeframes` | string[] | ✓ | Salah satu dari `1m`, `5m`, `15m`, `1h` (tergantung config running-trade-consumer). Min 1 item. |

**Behavior:**
- Untuk setiap pasangan baru yang di-subscribe, server **langsung kirim 1 `snapshot`** (best-effort dari Redis).
- Subscribe ke pasangan yang sudah aktif = no-op (idempotent).
- Tidak ada balasan "ack" — kalau snapshot keluar, berarti subscribe sukses.

### `unsubscribe`

```json
{
  "action": "unsubscribe",
  "stocks": ["BBCA"],
  "timeframes": ["1m"]
}
```

Sama formatnya dengan `subscribe`. Hanya pasangan yang disebut yang akan di-unsubscribe — pasangan lain tetap streaming.

**Tip:** Kalau cuma mau berhenti total, tinggal `ws.close()` di client — tidak perlu kirim unsubscribe dulu.

---

## Server → Client messages

Semua frame berbentuk JSON text dengan field `type`.

### Tipe pesan

| `type` | Kapan dikirim | Mengandung Bar? |
|---|---|---|
| `snapshot` | Setelah `subscribe` baru, satu kali per pasangan | Ya (bisa null kalau Redis belum punya data) |
| `update` | Setiap tick baru sambil bucket masih aktif | Ya |
| `closed` | Saat bucket finalised (rolled over ke bucket baru) | Ya, dengan `bar.status = "closed"` |
| `error` | Error per-command (subscribe invalid, JSON parse error, dll) | Tidak (pakai field `error`) |

### Schema umum

```ts
interface ServerMessage {
  type: "snapshot" | "update" | "closed" | "error";
  stock?: string;     // hanya untuk snapshot/update/closed
  tf?: string;        // hanya untuk snapshot/update/closed
  bar?: Bar | null;   // null hanya boleh di snapshot
  error?: string;     // hanya untuk error
}
```

### Schema `Bar`

```ts
interface Bar {
  stock:    string;   // mis. "BBCA"
  tf:       string;   // mis. "1m"
  open_ts:  number;   // unix epoch millis (UTC) — awal bucket
  close_ts: number;   // unix epoch millis (UTC) — akhir bucket (exclusive)
  o:        number;   // open price (rupiah, integer-valued tapi serialised float)
  h:        number;   // high
  l:        number;   // low
  c:        number;   // close — di update terus selama bucket live
  v:        number;   // total volume (lot × 100 = lembar) — int64
  trades:   number;   // jumlah trade (frequency)
  status:   "live" | "closed";
}
```

> **Catatan harga:** IDX quote whole rupiah, tapi field `o/h/l/c` bertipe float64 di JSON (untuk konsistensi dengan turunan/avg). Frontend boleh treat sebagai integer.

### Contoh `snapshot`

```json
{
  "type": "snapshot",
  "stock": "BBCA",
  "tf": "1m",
  "bar": {
    "stock": "BBCA",
    "tf": "1m",
    "open_ts": 1745824800000,
    "close_ts": 1745824860000,
    "o": 9650, "h": 9680, "l": 9650, "c": 9675,
    "v": 1900,
    "trades": 4,
    "status": "live"
  }
}
```

### Contoh `snapshot` kosong

```json
{
  "type": "snapshot",
  "stock": "ANTM",
  "tf": "1h",
  "bar": null
}
```

Artinya: belum ada bar di Redis untuk pasangan ini (saham yang baru saja IPO, market belum buka, dll). FE tidak perlu treat ini sebagai error — cukup tunggu `update` pertama.

### Contoh `update`

```json
{
  "type": "update",
  "stock": "BBCA",
  "tf": "1m",
  "bar": {
    "stock": "BBCA", "tf": "1m",
    "open_ts": 1745824800000, "close_ts": 1745824860000,
    "o": 9650, "h": 9700, "l": 9650, "c": 9695,
    "v": 2400, "trades": 5,
    "status": "live"
  }
}
```

### Contoh `closed`

```json
{
  "type": "closed",
  "stock": "BBCA",
  "tf": "1m",
  "bar": {
    "stock": "BBCA", "tf": "1m",
    "open_ts": 1745824800000, "close_ts": 1745824860000,
    "o": 9650, "h": 9700, "l": 9645, "c": 9690,
    "v": 3200, "trades": 7,
    "status": "closed"
  }
}
```

FE pattern yang aman: simpan bar dengan key `(stock, tf, open_ts)` — `update` & `closed` selalu pakai `open_ts` yang sama untuk bucket yang sama, jadi mudah replace-in-place. Saat `open_ts` ganti = bucket baru, push ke array.

### Contoh `error`

```json
{ "type": "error", "error": "subscribe requires non-empty stocks and timeframes" }
```

Kemungkinan pesan error:

| `error` value | Penyebab |
|---|---|
| `"invalid json: ..."` | Frame yang dikirim bukan JSON valid |
| `"unknown action: <x>"` | Field `action` bukan `subscribe`/`unsubscribe` |
| `"subscribe requires non-empty stocks and timeframes"` | Salah satu dari `stocks`/`timeframes` kosong |

Error tidak menyebabkan disconnect — koneksi tetap open, client bisa retry command yang benar.

---

## Disconnect scenarios

| Penyebab | Behavior server |
|---|---|
| Client `ws.close()` | Server clean-up subscription, log `ws client disconnected`. |
| Client diam terlalu lama | Server ping internal, tidak ada keep-alive aktif dari client. Cloudflare bisa close connection idle >100s — pakai keep-alive ping di client (lihat "Best practices" di bawah). |
| **Slow consumer** (FE telat baca) | Send buffer per-client penuh → server **paksa-disconnect**. Pesan log: `client send buffer full — disconnecting slow consumer`. |
| Server restart / rolling update | Connection close mendadak. Client harus implementasi auto-reconnect (lihat contoh JS di bawah). |

---

## Examples

### Browser JavaScript

```js
const ws = new WebSocket('wss://ws.tuai.id/ws/candles');

ws.onopen = () => {
  ws.send(JSON.stringify({
    action: 'subscribe',
    stocks: ['BBCA', 'BMRI'],
    timeframes: ['1m'],
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case 'snapshot':
    case 'update':
    case 'closed':
      handleBar(msg.stock, msg.tf, msg.bar, msg.type);
      break;
    case 'error':
      console.warn('ws-gateway error:', msg.error);
      break;
  }
};

ws.onclose = (e) => {
  console.log('disconnected', e.code, e.reason);
  // implement exponential-backoff reconnect here
};
```

### Reconnect-pattern (production)

```js
function connect(url, onMsg) {
  let ws, retry = 0;
  function open() {
    ws = new WebSocket(url);
    ws.onopen    = () => { retry = 0; ws.send(JSON.stringify(subscribePayload)); };
    ws.onmessage = (e) => onMsg(JSON.parse(e.data));
    ws.onclose   = () => { setTimeout(open, Math.min(30000, 1000 * 2 ** retry++)); };
    ws.onerror   = () => ws.close();
  }
  open();
  return () => ws && ws.close();
}
```

**Server-side application ping** belum ada di Phase 1 — Cloudflare close idle 100s. Solusi sederhana: kirim subscribe-ulang lightweight tiap ~60s, atau pakai TCP-level keep-alive (browser otomatis).

### websocat CLI

```bash
# install: brew install websocat   atau   cargo install websocat
websocat wss://ws.tuai.id/ws/candles
# lalu ketik:
{"action":"subscribe","stocks":["BBCA"],"timeframes":["1m"]}
```

### wscat CLI

```bash
npm install -g wscat
wscat -c wss://ws.tuai.id/ws/candles
> {"action":"subscribe","stocks":["BBCA"],"timeframes":["1m"]}
```

### Python

```python
import asyncio, json
import websockets

async def main():
    async with websockets.connect('wss://ws.tuai.id/ws/candles') as ws:
        await ws.send(json.dumps({
            'action': 'subscribe',
            'stocks': ['BBCA'],
            'timeframes': ['1m'],
        }))
        async for raw in ws:
            msg = json.loads(raw)
            print(msg['type'], msg.get('stock'), msg.get('bar', {}).get('c'))

asyncio.run(main())
```

### Health check

```bash
curl https://ws.tuai.id/healthz
# {"status":"ok"}
```

---

## Operational notes

### Frame size & rate

- Bar JSON ~250–300 bytes per pesan.
- Saat pasar sibuk (open auction 09:00:00 WIB), satu saham besar (BBCA, BMRI) bisa generate **5–20 update/detik** untuk timeframe 1m.
- Total throughput tergantung jumlah saham yang Anda subscribe. 50 saham × 1m × 10 tick/s ≈ 500 msg/s ≈ 150 KB/s. WS handle ini tanpa masalah.

### Per-client send buffer

Server-side buffer per koneksi terbatas. Kalau FE proses message lebih lambat dari incoming rate (mis. tab di background, render thread saturated), buffer akan penuh dan server **disconnect klien itu** (klien lain tidak terdampak).

Kalau Anda lihat koneksi sering close padahal jaringan OK, kurangi jumlah subscription atau batch render di FE.

### Backfill historical bars

ws-gateway **hanya kasih bar saat ini + live updates**. Untuk historical bar (mis. chart 100 bar terakhir), pakai HTTP API ke `stock.prices_daily` di Postgres atau langsung query QuestDB (lihat [docs/QuestDB/](../../QuestDB/)).

Pattern FE chart yang umum: load history dari HTTP REST → connect WS → pakai `snapshot` + `update` untuk continue dari history terakhir.

---

## Status & rencana ke depan

- **Phase 1 (saat ini)**: WS streaming dari `idx.candle.>` + Redis snapshot. Tanpa auth, tanpa per-client rate limit, tanpa application-level ping.
- **Phase 2 (planned)**: JWT auth via query param atau header, ping/pong frame, per-user subscription quota.
- **Belum support**: tick-by-tick raw trade stream (cuma candle), depth/order book stream (pakai service lain), history backfill via WS.

### Catatan saat ini (2026-04-28)

`idx.candle.>` belum ada publisher karena running-trade-consumer masih `ENABLE_CANDLE_PUBLISHER=false`. Akibatnya:
- `subscribe` ke pasangan yang ada di Redis → dapat `snapshot` saja.
- Tidak ada `update` / `closed` sampai candle publisher diaktifkan.

Untuk aktifkan: update Secret `running-trade-consumer-env` di namespace `tuai`, set `ENABLE_CANDLE_PUBLISHER=true`, restart deploy.
