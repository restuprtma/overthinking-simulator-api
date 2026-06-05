# IQPlus Data Feed Integration Guide

**Target Audience:** Venturo Go Engineering Team
**Version:** 1.0
**Last Updated:** 2026-04-24
**Spec Reference:** IQPlus Data Feed Service (SCF) v4.0.0

---

## 1. Overview

IQPlus adalah **layanan streaming data real-time** Bursa Efek Indonesia (IDX) yang dikirim via **TCP raw socket** dengan format text berstruktur. Berbeda dengan REST API, IQPlus adalah **push-based protocol** — client login sekali, server langsung terus-menerus stream data selama koneksi hidup.

### Karakteristik utama

| Aspek              | Detail                                        |
| ------------------ | --------------------------------------------- |
| **Transport**      | TCP raw socket (bukan HTTP)                   |
| **Endpoint**       | `103.114.143.237:8888`                        |
| **Protocol**       | Text-based, pipe-delimited, CRLF terminator   |
| **Paradigma**      | Push-based (server streams, client listens)   |
| **Authentication** | MD5-hashed password, sekali di awal koneksi   |
| **Rate**           | 1000-5000 record/detik saat peak market hours |
| **IP Restriction** | Whitelist-based (per IP subscribed)           |

### Kredensial saat ini

```
Host     : 103.114.143.237
Port     : 8888
Username : venturo
Password : ca0a9bbcfead18109abda813e3c03981  (MD5 hash, jangan di-hash lagi)
```

⚠️ **Akses hanya dari IP yang sudah di-whitelist ke IQPlus.** Saat ini akses berhasil dari VM Biznet (`10.10.1.15`). IP lain akan di-reject dengan error `Access denied from your IP Address`.

---

## 2. Protocol Specification

### 2.1. General Record Format

Setiap record dari server mengikuti format:

```
IQP|<date>|<time>|<sequence>|<record_type>|<data>[CR][LF]
```

| Field         | Size     | Format                 | Contoh                |
| ------------- | -------- | ---------------------- | --------------------- |
| `IQP`         | 3 bytes  | Fixed literal          | `IQP`                 |
| `date`        | 8 bytes  | `YYYYMMDD`             | `20260424`            |
| `time`        | 6 bytes  | `HHMMSS` (WIB / GMT+7) | `090015`              |
| `sequence`    | variable | Numeric, unik per hari | `71577`               |
| `record_type` | variable | Numeric (lihat §2.3)   | `15`                  |
| `data`        | variable | Depend on record type  | `WIKA\|20260424\|...` |
| terminator    | 2 bytes  | `\r\n` (`0x0D 0x0A`)   | —                     |

**Data separator:** karakter `|` (ASCII 124) memisahkan field. Dalam data FID-based (record type 14), karakter `;` (ASCII 59) memisahkan FID dengan value-nya.

### 2.2. Login Flow

#### Request Format

```
IQP|149|0|1|<username>|<md5_password>[CR][LF]
```

| Field            | Arti                                           |
| ---------------- | ---------------------------------------------- |
| `149`            | `auth_record_type` (fixed)                     |
| `0`              | `sub_type`: `0` = login, `1` = change password |
| `1`              | `encryption_method`: `1` = MD5                 |
| `<username>`     | Username (plaintext)                           |
| `<md5_password>` | Password sudah di-hash MD5 (32 char hex)       |

#### Response Format

```
IQP|149|0|<status_code>|<message>[CR][LF]
```

#### Status Codes

| Code | Meaning                            | Action                   |
| ---- | ---------------------------------- | ------------------------ |
| `0`  | OK                                 | Lanjut terima stream     |
| `1`  | Invalid password                   | Cek credential           |
| `2`  | Expire                             | Kontak provider          |
| `3`  | Invalid user name                  | Cek username             |
| `5`  | Already login                      | Tutup koneksi lain dulu  |
| `6`  | Access denied from your IP Address | IP belum whitelist       |
| `7`  | System error                       | Retry after delay        |
| `8`  | Access Denied for Temporary        | Retry after longer delay |
| `9`  | Unauthorized user                  | Kontak provider          |
| `10` | Header IQP not found               | Fix payload format       |

#### Contoh Success

**Request:**

```
IQP|149|0|1|venturo|ca0a9bbcfead18109abda813e3c03981\r\n
```

**Response:**

```
IQP|149|0|0|OK\r\n
```

Setelah ini, server langsung mulai streaming data real-time sesuai permission akun.

### 2.3. Record Types

| Type  | Name             | Stream Behavior                                      | Broker Code?                     |
| ----- | ---------------- | ---------------------------------------------------- | -------------------------------- |
| `13`  | Control Messages | On server UP/DOWN                                    | —                                |
| `14`  | Quote            | Per harga/bid/offer berubah                          | —                                |
| `15`  | Trade            | Per transaksi match                                  | ❌ masked `--` saat market hours |
| `16`  | Order            | Per order book update                                | ❌ masked `--`                   |
| `17`  | Top 20           | Periodic                                             | —                                |
| `18`  | Best Quote       | Per best bid/offer change                            | —                                |
| `26`  | Resend Order     | **After market close**                               | ✅ dengan broker code            |
| `27`  | Resend Trade     | **After market close**                               | ✅ dengan broker code            |
| `36`  | News             | Per berita baru (permission required)                | —                                |
| `39`  | Activity         | Periodic (counter aktif/up/down)                     | —                                |
| `40`  | Trade Done       | Aggregate per price level                            | —                                |
| `57`  | Trading Status   | Saat sesi mulai/berakhir                             | —                                |
| `58`  | NBS Stock        | Real-time NBS per stock-broker (permission required) | ✅                               |
| `59`  | NBS Broker       | Real-time NBS per broker-stock (permission required) | ✅                               |
| `130` | Trading Summary  | Periodic aggregate                                   | —                                |
| `149` | Login response   | Sekali saja                                          | —                                |

⚠️ **Penting:** Selama market aktif, field broker code di Trade (type 15) dan Order (type 16) di-mask jadi `--`. Broker code asli baru dikirim via **Resend Trade (type 27)** setelah market close (~16:30 WIB).

### 2.4. Contoh Data Real

```
IQP|20211223|085900|69397|15|WIKA|20211208|085900|1|0|1225|200|--|D|--|D|48941|34504
IQP|20211223|173942|34650807|27|LPKR|20211223|101741|550511|0|146|1200|AK|F|YP|D|1295566|217378
IQP|20211223|104551|2940338|58|BBYB|PD|3989|13206300|132063|35407613000|0.271950|3076|10622300|106223|28449851000|0.218510
IQP|20211223|090000|71620|18|INDF|S|6400;251;1;0;0|6725;100;1;0;0
```

Untuk breakdown lengkap tiap FID & field, **lihat dokumen resmi IQPlus v4.0.0** yang sudah di-share di internal docs.

---

## 3. Connection Lifecycle

```
┌──────────┐   TCP dial    ┌──────────────┐
│  Client  │──────────────>│  IQPlus      │
│  (Go)    │               │  Server      │
└──────────┘<──────────────└──────────────┘
     │        connected          │
     │                           │
     │   Send login payload      │
     ├──────────────────────────>│
     │                           │
     │<──────────────────────────│
     │   IQP|149|0|0|OK          │
     │                           │
     │<──────────────────────────│
     │   IQP|...|14|...          │  ←  stream mengalir terus
     │<──────────────────────────│
     │   IQP|...|15|...          │
     │<──────────────────────────│
     │   ... (ribuan/detik)      │
     │                           │
     X   Connection dropped      X  ←  wajib auto-reconnect
```

### Karakteristik Penting

- **Persistent connection** — koneksi harus selalu terbuka. Kalau drop, data selama disconnect **hilang** (kecuali lewat Resend type 26/27 after market close).
- **No query, no filter** — server kirim SEMUA data sesuai permission. Filter harus di client side.
- **No heartbeat bawaan** — server tidak kirim ping/pong. Deteksi dead connection harus via timeout sendiri.
- **Order matters** — tetap urut berdasarkan `sequence` number yang increment dari `1` di awal hari.

---

## 4. Go Implementation Guide

### 4.1. Recommended Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    iqplus-ingestor (Go)                     │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │  Connector   │───>│   Parser     │───>│   Router     │ │
│  │  (TCP/auth)  │    │ (line→record)│    │ (by type)    │ │
│  └──────────────┘    └──────────────┘    └──────┬───────┘ │
│         ▲                                       │          │
│         │ auto-reconnect                        ▼          │
│  ┌──────────────┐                      ┌──────────────┐   │
│  │  Watchdog    │                      │   Sinks      │   │
│  │  (heartbeat) │                      │ ┌──────────┐ │   │
│  └──────────────┘                      │ │TimescaleDB│ │   │
│                                        │ │ Redis    │ │   │
│                                        │ │ NATS     │ │   │
│                                        │ │ Temporal │ │   │
│                                        │ └──────────┘ │   │
│                                        └──────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 4.2. Core Code Skeleton

```go
package iqplus

import (
    "bufio"
    "context"
    "fmt"
    "net"
    "strings"
    "time"
)

const (
    host           = "103.114.143.237"
    port           = 8888
    dialTimeout    = 10 * time.Second
    readTimeout    = 60 * time.Second // anggap dead kalau gak ada data >60s
    reconnectDelay = 5 * time.Second
)

type Credentials struct {
    Username string
    MD5Hash  string // password sudah di-MD5, JANGAN hash lagi
}

type Client struct {
    creds  Credentials
    out    chan<- Record
    conn   net.Conn
}

type Record struct {
    Date       string // YYYYMMDD
    Time       string // HHMMSS
    Sequence   int64
    RecordType int
    RawData    string   // field data mentah
    Fields     []string // sudah di-split by |
    ReceivedAt time.Time
}

func NewClient(creds Credentials, out chan<- Record) *Client {
    return &Client{creds: creds, out: out}
}

// Run adalah main loop dengan auto-reconnect.
// Blocking sampai ctx cancelled.
func (c *Client) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if err := c.connectAndStream(ctx); err != nil {
            // log error, tunggu, reconnect
            fmt.Printf("[iqplus] connection error: %v, reconnecting in %s\n",
                err, reconnectDelay)
        }

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(reconnectDelay):
        }
    }
}

func (c *Client) connectAndStream(ctx context.Context) error {
    addr := fmt.Sprintf("%s:%d", host, port)
    conn, err := net.DialTimeout("tcp", addr, dialTimeout)
    if err != nil {
        return fmt.Errorf("dial: %w", err)
    }
    defer conn.Close()
    c.conn = conn

    // Login
    loginPayload := fmt.Sprintf("IQP|149|0|1|%s|%s\r\n",
        c.creds.Username, c.creds.MD5Hash)
    if _, err := conn.Write([]byte(loginPayload)); err != nil {
        return fmt.Errorf("write login: %w", err)
    }

    // Baca response login (1 baris pertama)
    reader := bufio.NewReaderSize(conn, 64*1024)
    conn.SetReadDeadline(time.Now().Add(10 * time.Second))

    loginResp, err := reader.ReadString('\n')
    if err != nil {
        return fmt.Errorf("read login response: %w", err)
    }

    if !strings.HasPrefix(loginResp, "IQP|149|0|0|OK") {
        return fmt.Errorf("login failed: %s", strings.TrimSpace(loginResp))
    }

    fmt.Println("[iqplus] login success, streaming...")

    // Stream loop
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        conn.SetReadDeadline(time.Now().Add(readTimeout))
        line, err := reader.ReadString('\n')
        if err != nil {
            return fmt.Errorf("read stream: %w", err)
        }

        line = strings.TrimRight(line, "\r\n")
        if line == "" {
            continue
        }

        rec, err := parseRecord(line)
        if err != nil {
            // log tapi jangan stop streaming, mungkin cuma 1 line corrupt
            fmt.Printf("[iqplus] parse error: %v, line: %q\n", err, line)
            continue
        }

        select {
        case c.out <- rec:
        case <-ctx.Done():
            return ctx.Err()
        default:
            // channel full — strategi tergantung kebutuhan
            // Opsi 1: drop record (log warning)
            // Opsi 2: block (risk backpressure ke TCP)
            // Opsi 3: grow buffer (risk OOM)
            fmt.Println("[iqplus] WARNING: output channel full, dropping record")
        }
    }
}

func parseRecord(line string) (Record, error) {
    if !strings.HasPrefix(line, "IQP|") {
        return Record{}, fmt.Errorf("missing IQP header")
    }

    parts := strings.SplitN(line, "|", 6)
    if len(parts) < 6 {
        return Record{}, fmt.Errorf("insufficient fields: got %d", len(parts))
    }

    var seq int64
    fmt.Sscanf(parts[3], "%d", &seq)

    var rt int
    fmt.Sscanf(parts[4], "%d", &rt)

    return Record{
        Date:       parts[1],
        Time:       parts[2],
        Sequence:   seq,
        RecordType: rt,
        RawData:    parts[5],
        Fields:     strings.Split(parts[5], "|"),
        ReceivedAt: time.Now(),
    }, nil
}
```

### 4.3. Entry Point

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/venturo/iqplus-ingestor/iqplus"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    out := make(chan iqplus.Record, 100_000) // buffer besar untuk handle burst

    client := iqplus.NewClient(iqplus.Credentials{
        Username: os.Getenv("IQPLUS_USER"),
        MD5Hash:  os.Getenv("IQPLUS_PASS_MD5"),
    }, out)

    // Connector runs in background
    go func() {
        if err := client.Run(ctx); err != nil {
            log.Printf("[main] client exited: %v", err)
        }
    }()

    // Consumer — route by record type
    for rec := range out {
        switch rec.RecordType {
        case 14:
            handleQuote(rec)
        case 15:
            handleTrade(rec)
        case 27:
            handleResendTrade(rec) // ini yg ada broker code!
        case 58:
            handleNBSStock(rec)
        case 57:
            handleTradingStatus(rec)
        // ... handlers lain
        }
    }
}
```

### 4.4. Parser Per Record Type

Contoh parser untuk **Resend Trade (type 27)** — paling penting untuk BDM:

```go
// Format: CODE|DATE|TIME|TRADE_NUM|CMD|PRICE|VOL|BUYER|BUYER_TYPE|SELLER|SELLER_TYPE|BUYER_ORDER|SELLER_ORDER
type ResendTrade struct {
    Code         string    // Stock code
    TradeDate    string
    TradeTime    string
    TradeNumber  int64
    Command      int       // 0=matched, 1=withdrawn
    Price        int64
    Volume       int64
    Buyer        string    // Broker code (mis. "YP", "AK")
    BuyerType    string    // F=foreign, D=domestic
    Seller       string
    SellerType   string
    BuyerOrder   int64
    SellerOrder  int64
}

func parseResendTrade(fields []string) (*ResendTrade, error) {
    if len(fields) < 13 {
        return nil, fmt.Errorf("resend trade: expected 13 fields, got %d", len(fields))
    }

    rt := &ResendTrade{
        Code:        fields[0],
        TradeDate:   fields[1],
        TradeTime:   fields[2],
        Buyer:       fields[7],
        BuyerType:   fields[8],
        Seller:      fields[9],
        SellerType:  fields[10],
    }
    fmt.Sscanf(fields[3], "%d", &rt.TradeNumber)
    fmt.Sscanf(fields[4], "%d", &rt.Command)
    fmt.Sscanf(fields[5], "%d", &rt.Price)
    fmt.Sscanf(fields[6], "%d", &rt.Volume)
    fmt.Sscanf(fields[11], "%d", &rt.BuyerOrder)
    fmt.Sscanf(fields[12], "%d", &rt.SellerOrder)

    return rt, nil
}
```

Contoh parser untuk **Quote (type 14)** yang pakai FID-based format:

```go
// Format data: "0;AALI|1;Astra Agro Lestari|24;9925|39;9950|56;9925|..."
func parseQuoteFIDs(data string) map[int]string {
    fids := make(map[int]string)
    for _, field := range strings.Split(data, "|") {
        parts := strings.SplitN(field, ";", 2)
        if len(parts) != 2 {
            continue
        }
        var fid int
        if _, err := fmt.Sscanf(parts[0], "%d", &fid); err != nil {
            continue
        }
        fids[fid] = parts[1]
    }
    return fids
}

// Usage:
// fids[0]  = Stock code
// fids[24] = Bid price
// fids[39] = Offer price
// fids[56] = Last traded price
// fids[72] = Volume (total)
// fids[35] = Indicator (1=BUY confidence, 7=SELL confidence)
```

---

## 5. Operational Considerations

### 5.1. Volume & Storage

| Item                  | Estimasi          |
| --------------------- | ----------------- |
| Record rate saat peak | 1,000–5,000 rec/s |
| Ukuran rata-rata      | 100–300 bytes     |
| Raw log harian        | 500 MB – 2 GB     |
| 1 bulan (30 hari)     | 15 – 60 GB        |
| 1 tahun               | 180 – 720 GB      |

**Rekomendasi:** jangan simpan raw log semua. Parse → normalize → simpan ke **TimescaleDB** atau **ClickHouse** (cocok untuk time-series finansial). Drop raw log setelah parsing sukses.

### 5.2. Reconnection Strategy

Koneksi bisa drop karena: network blip, server restart, firewall timeout, idle detection. Strategi:

1. **Infinite retry** dengan exponential backoff (max 30s)
2. **Log sequence number terakhir** — biar tau ada gap saat reconnect
3. **Alert** kalau gagal reconnect > 3x berturut-turut
4. **Jangan spam login** — kalau error `5 = Already login`, tunggu 60s baru retry

### 5.3. Heartbeat / Dead Connection Detection

Server tidak kirim heartbeat, jadi kita deteksi manual:

```go
// Kalau >60 detik gak ada data sama sekali (di jam market), anggap dead
conn.SetReadDeadline(time.Now().Add(60 * time.Second))
```

⚠️ Set timeout lebih lama di luar jam market (malam/weekend) karena memang traffic sepi.

### 5.4. Backpressure Handling

Kalau downstream (DB, Redis, dll) lambat, channel bisa penuh. Strategi:

- **Drop oldest** untuk data non-kritis (Quote bisa di-drop, Trade tidak)
- **Persist ke disk queue** (BadgerDB / BoltDB) sebagai buffer kalau DB down
- **Circuit breaker** ke downstream sink

### 5.5. Monitoring & Alerting

Metric yang wajib di-expose (Prometheus):

- `iqplus_records_received_total{type="15"}` — counter per record type
- `iqplus_records_dropped_total{reason="..."}` — backpressure drops
- `iqplus_connection_state` — 1=connected, 0=disconnected
- `iqplus_reconnects_total` — counter reconnect
- `iqplus_last_record_timestamp` — timestamp record terakhir (buat deteksi stale)
- `iqplus_parse_errors_total{type="..."}` — parse failure counter
- `iqplus_sequence_gap_total` — deteksi lost records (sequence tidak kontinu)

Alert rules:

- Stale feed: `time() - iqplus_last_record_timestamp > 120` saat jam market
- Reconnect loop: `rate(iqplus_reconnects_total[5m]) > 3`
- High drop rate: `rate(iqplus_records_dropped_total[1m]) > 100`

### 5.6. Deployment

- **Single instance** — koneksi IQPlus tidak boleh duplikat (error `5 = Already login`). Pakai leader election kalau deploy HA.
- **Network egress** — pastikan deploy di VM yang IP-nya sudah di-whitelist IQPlus (saat ini hanya VM Biznet `10.10.1.15`).
- **Resource** — cukup ringan: 100-500 MB RAM, 1 vCPU. Bottleneck di jaringan & downstream DB.
- **Graceful shutdown** — pas terima SIGTERM, drain channel dulu sebelum exit.

---

## 6. Key Use Cases untuk Venturo

### 6.1. Real-time Bandarmologi (BDM)

**Source:** Resend Trade (type 27) — dikirim setelah market close dengan broker code lengkap.

**Alur:**

1. Capture semua type 27 saat mulai masuk (~16:30 WIB)
2. Aggregate per `(stock_code, buyer_broker)` dan `(stock_code, seller_broker)`
3. Hasil = broker summary EOD yang setara Invezgo `invezgo_get_broker_summary_stock`

### 6.2. Intraday Broker Flow Monitoring

**Source:** NBS Stock (type 58) — real-time update sepanjang hari (butuh permission IDX Broker).

**Alur:**

1. Track accumulation/distribution broker besar (YP, CC, AK, MG, dll)
2. Deteksi pola bandarmologi live
3. Trigger alert kalau net buy/sell broker tertentu melebihi threshold

### 6.3. Indicator-based Alerts

**Source:** Quote (type 14) FID 35 = Indicator.

Values:

- `1` = BUY with confidence (strong buy signal)
- `7` = SELL with confidence (strong sell signal)

Trigger notifikasi Discord `analisa-claude` otomatis saat muncul, gabungkan dengan broker flow untuk decision.

### 6.4. Top 20 Tracker

**Source:** Top 20 (type 17) — periodic update selama jam market.

Monitor perubahan top gainer/loser/volume untuk deteksi rotasi sektor real-time.

### 6.5. Integrasi dengan Sistem Existing

- **Temporal workflow** — orchestrate daily EOD aggregation job
- **n8n** — alerting ke Discord/WhatsApp saat trigger match
- **Orderbook automation** — combine dengan Best Quote (type 18) untuk algo execution

---

## 7. Pitfalls & Gotchas

1. **Password HARUS sudah ter-hash MD5.** Jangan hash lagi di client. Password `ca0a9bbcfead18109abda813e3c03981` itu udah MD5 result.

2. **CRLF mandatory (`\r\n`), bukan cuma `\n`.** Pakai `printf` dengan `\r\n` explicit atau gunakan `fmt.Sprintf(...+"\r\n")` di Go. Kalau pakai heredoc shell atau `echo`, hasilnya LF doang → server reject dengan error code `10 = Header IQP not found`.

3. **Jangan pakai `telnet` untuk testing.** Telnet kirim IAC option negotiation bytes (0xFF) yang bisa bikin server IQPlus confuse. Pakai `nc` (netcat) atau Go TCP client langsung.

4. **Broker code masked saat live trading.** Jangan cari broker code di type 15 saat jam market, cari di type 27 setelah market close.

5. **Permission-based streaming.** Akun `venturo` mungkin tidak punya akses ke semua record type (misal NBS butuh permission tambahan). Kalau type tertentu gak muncul di stream, kemungkinan bukan subscribed — konfirmasi ke provider IQPlus.

6. **Time zone.** Timestamp di payload adalah **WIB (GMT+7)**. Wajib konversi saat simpan ke DB yang pakai UTC.

7. **Sequence number reset di awal hari.** Sequence mulai dari `1` tiap hari baru, bukan monotonic forever. Jangan jadikan primary key.

8. **Single connection per account.** Kalau ada 2 proses login bersamaan dengan credential sama, yang kedua akan ditolak dengan error `5 = Already login`.

9. **No query, no replay.** Kalau service down saat market aktif, data selama offline **hilang**. Satu-satunya recovery adalah Resend (type 26/27) after market close. Tidak ada mekanisme "replay dari sequence X".

10. **Pendanaan FTP.** Dokumentasi tidak menyebut fallback FTP/batch download. Pastikan konfirmasi ke provider apakah ada mekanisme EOD download file untuk backup.

---

## 8. Testing & Validation

### 8.1. Manual Testing (dari VM Biznet)

```bash
# 1. Siapkan payload login
printf "IQP|149|0|1|venturo|ca0a9bbcfead18109abda813e3c03981\r\n" > user.login

# 2. Verify format (harus ada 0d 0a di akhir)
xxd user.login | tail -1

# 3. Test koneksi & capture sample
(cat user.login; sleep 30) | nc -v 103.114.143.237 8888 | tee sample.log

# 4. Analisis sample
grep -c "^IQP" sample.log                # total records
awk -F'|' '{print $5}' sample.log | sort -u  # list record types yg masuk
```

### 8.2. Unit Test Target

- Parse valid record → struct benar
- Parse malformed record → return error, tidak panic
- Parse record dengan field kosong → handle graceful
- FID parser untuk type 14 → handle missing FIDs
- Edge case: sequence number rollover, timestamp invalid, dll

### 8.3. Integration Test

- Mock TCP server yang kirim sample data → verify parser & routing
- Test reconnection: kill connection di tengah stream → verify auto-reconnect
- Test backpressure: downstream slow → verify drop/buffer behavior

---

## 9. Next Steps (Saran Implementasi)

**Phase 1 — MVP (1-2 minggu):**

- Go service dengan connector + parser untuk type 14, 15, 27, 57
- Sink ke TimescaleDB
- Basic monitoring (logs + 1 health endpoint)
- Deploy di VM Biznet, running 24/7

**Phase 2 — BDM Pipeline (2-3 minggu):**

- Parser lengkap untuk type 27, 58, 59
- Aggregation workflow di Temporal (daily EOD broker summary)
- Data retention policy di TimescaleDB
- Prometheus metrics + Grafana dashboard

**Phase 3 — Real-time Analytics (3-4 minggu):**

- Streaming aggregation (live broker flow)
- Alert engine → Discord/WhatsApp via n8n
- Integration dengan sistem trading automation existing

**Phase 4 — Advanced (later):**

- Replay dari TimescaleDB untuk backtesting strategi
- Machine learning pipeline (training dari historical feed)
- Integrasi dengan Invezgo MCP untuk cross-validation data

---

## 10. References

- **IQPlus Data Feed Service Technical Specification v4.0.0** — dokumen resmi, field definitions lengkap
- **IDX Trading Hours** — https://www.idx.co.id (referensi jam sesi market)
- **MD5 hash reference** — pastikan encoding lowercase hex 32 char
- **Similar systems** — KOFX, OMnet (protocol design reference)

---

## 11. Contacts & Escalation

- **Provider IQPlus** — kontak untuk whitelist IP, renewal, permission upgrade
- **Network admin Venturo** — firewall/routing issues dari VM Biznet
- **Tantowi (CTO)** — arsitektur & strategic decisions

---

_Dokumen ini living document. Silakan update kalau ada temuan baru saat implementasi._
