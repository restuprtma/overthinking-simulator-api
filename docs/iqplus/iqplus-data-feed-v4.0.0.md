# IQPlus Data Feed Service (SCF) — Technical Specification v4.0.0

Ringkasan & penjelasan dokumen `IQPlus-Data Feed Service-4.0.0_2025.pdf`. Dokumen sumber dibuat oleh IQPlus, mendeskripsikan protokol streaming data perdagangan saham IDX (Indonesia Stock Exchange) dan data terkait (regional index, commodity, futures, currency, news).

---

## 1. Pengantar

IQPlus Data Feed (SCF) adalah layanan streaming data trading dari Bursa Efek Indonesia (IDX) yang dikirim sebagai teks dengan protokol tertentu.

- **Protokol**: TCP/IP (client–server)
- **OS server**: UNIX FreeBSD
- **Encoding payload**: ASCII text, dipisahkan karakter `|` (pipe) dan `;` (semicolon)
- **Akhir record**: `CR/LF` (ASCII 13 + 10)

Tujuan: memudahkan client menerima data realtime dengan parsing sederhana.

---

## 2. Daftar Record Type

| Record Type | Nama                   | Keterangan                       |
|------------:|------------------------|----------------------------------|
| 13          | Control Messages       | Status koneksi UP/DOWN           |
| 14          | Quote                  | Snapshot data saham/index/dll    |
| 15          | Trade                  | Trade matched/withdrawn          |
| 16          | Order                  | Bid/Offer/Cancel order           |
| 17          | Top 20                 | Top 20 by berbagai kategori      |
| 18          | Best Quote             | Best bid/offer order book        |
| 26          | Resend Order           | Replay order (after market close)|
| 27          | Resend Trade           | Replay trade (incl. broker code) |
| 36          | News                   | Berita ekonomi & bisnis          |
| 39          | Activity               | Statistik agregat aktivitas saham|
| 40          | Trade Done             | Akumulasi trade per harga        |
| 57          | Trading Status         | Status sesi perdagangan          |
| 58          | NBS Stock              | Net Buy/Sell per saham (broker)  |
| 59          | NBS Broker             | Net Buy/Sell per broker (saham)  |
| 130         | Trading Summary        | Ringkasan harian per board       |
| 149 (0)     | Login                  | Autentikasi                      |
| 149 (1)     | Change Password        | Ganti password                   |

### Pemetaan record per kategori data

| Data               | Record Types yang diterima                                  | Permission |
|--------------------|-------------------------------------------------------------|------------|
| **IDX Stock**      | 13, 14, 15, 16, 17, 18, 26, 27, 39, 40, 57, 130, 149        | Required   |
| **IDX Broker**     | 15 (incl. broker code), 17 (Top 20 Broker), 27, 58, 59      | Required   |
| **Regional Index** | 13, 14, 149                                                 | Required   |
| **Commodity**      | 13, 14, 149                                                 | Required   |
| **Futures**        | 13, 14, 149                                                 | Required   |
| **Currency**       | 13, 14, 149                                                 | Required   |
| **News**           | 13, 36, 149                                                 | Required   |

---

## 3. Format Record Umum

```
IQP|Date|Time|Sequence#|RecordType|Data[CR/LF]
```

| Bagian        | Panjang   | Format / Aturan                                   |
|---------------|-----------|---------------------------------------------------|
| `IQP`         | 3 byte    | Selalu literal `IQP`                              |
| `Date`        | 8 byte    | `YYYYMMDD`                                        |
| `Time`        | 6 byte    | `HHMMSS` (waktu pengiriman)                       |
| `Sequence#`   | variabel  | Unique per hari, mulai `1` di awal hari          |
| `RecordType`  | 1 byte (logical) | Kode record type (lihat tabel di atas)     |
| `Data`        | variabel  | Field dipisah `|`; sub-field dipisah `;`         |
| `CR/LF`       | 2 byte    | ASCII 13 + 10                                     |

---

## 4. Login & Change Password (Record Type 149)

### 4.1 Login

**Request**

```
IQP|149|0|1|<user>|<md5(password)>[CR/LF]
```

- `149` = `auth_record_type`
- `0` = sub_type (0 = login, 1 = change password)
- `1` = `encryption_method` (selalu `1` = MD5)

Contoh:

```
IQP|149|0|1|demo|e368b9938746fa090d6afd3628355133[CR/LF]
```

**Reply**

```
IQP|149|0|<status#>|<message>[CR/LF]
```

Contoh sukses:

```
IQP|149|0|0|OK[CR/LF]
```

### 4.2 Change Password

**Request**

```
IQP|149|1|1|<user>|<md5(new_password)>|<md5(old_password)>[CR/LF]
```

**Reply**

```
IQP|149|1|<status#>|<message>[CR/LF]
```

### 4.3 Status Code Reply

| Code | Arti                                       |
|-----:|--------------------------------------------|
| 0    | OK                                          |
| 1    | Invalid password                            |
| 2    | Expired                                     |
| 3    | Invalid user name                           |
| 4    | Change password: wrong old password         |
| 5    | Already login                               |
| 6    | Access denied from your IP address          |
| 7    | System error                                |
| 8    | Access denied (temporary)                   |
| 9    | Unauthorized user                           |
| 10   | Header `IQP` not found                      |

---

## 5. Detail Record per Type

### 5.1 Control Message — Type `13`

Indikator status koneksi ke server pusat IQPlus.

- Data `0` = `UP`
- Data `1` = `DOWN`

```
IQP|20211222|072222|1|13|0[CR/LF]
```

### 5.2 Trading Status — Type `57`

Menunjukkan fase sesi perdagangan saat ini.

| Status | Deskripsi                  |
|--------|----------------------------|
| `1`    | Begin sending records      |
| `3`    | Begin first session        |
| `4`    | End first session          |
| `5`    | Begin second session       |
| `6`    | End second session         |
| `7`    | End sending records        |
| `8`    | Begin Pre-opening          |
| `9`    | End Pre-opening            |
| `a`    | Begin Pre-closing          |
| `b`    | End Pre-closing            |
| `c`    | Begin Post-trading         |
| `d`    | End Post-trading           |
| `e`    | Trading suspension         |
| `f`    | Trading activation         |
| `g`    | Board suspension           |
| `h`    | Board activation           |
| `i`    | Instrument suspension      |
| `j`    | Instrument activation      |
| `k`    | Market suspension          |
| `l`    | Market activation          |

Contoh urutan satu hari trading:

```
IQP|20211223|040351|3536|57|1|Begin sending records[CR/LF]
IQP|20211223|084500|4244|57|8|Begin Pre-opening[CR/LF]
IQP|20211223|085900|11673|57|9|End Pre-opening[CR/LF]
IQP|20211223|090000|71431|57|3|Begin first session[CR/LF]
IQP|20211223|113000|13712445|57|4|End first session[CR/LF]
IQP|20211223|133000|23727277|57|5|Begin second session[CR/LF]
IQP|20211223|145000|35484731|57|a|Begin Pre-closing[CR/LF]
IQP|20211223|150000|45520461|57|b|End Pre-closing[CR/LF]
```

### 5.3 Quote — Type `14`

Database utama IQPlus. Tiap quote berisi banyak FID (Field Identification Number). Antar FID dipisah `|`, antara FID dan nilainya dipisah `;`.

#### FID Reference (Quote)

| FID | Type       | Deskripsi                              |
|----:|------------|----------------------------------------|
| 0   | String     | Code (kode saham/index/instrument)     |
| 1   | String     | Name                                   |
| 2   | Numeric    | XGroup (bitmask index, lihat Appendix) |
| 3   | Byte       | Status (0 = Active)                    |
| 4   | Numeric    | Listing Date                           |
| 5   | Numeric    | Group                                  |
| 6   | Numeric    | ISSUERID                               |
| 7   | Numeric    | Listed Shares                          |
| 8   | Numeric    | Tradable Shares                        |
| 9   | Numeric    | IPO                                    |
| 10  | Numeric    | ORDERBOOKID                            |
| 11  | Numeric    | Base Price                             |
| 12  | String     | CURRENCY                               |
| 13  | String     | Remark                                 |
| 14  | Numeric    | Earning per share (EPS)                |
| 15  | String     | ISIN                                   |
| 16  | Numeric    | FOREIGNLIMIT                           |
| 17  | Numeric    | SECTORNAME (lihat Appendix)            |
| 18  | Numeric    | INDUSTRYNAME (lihat Appendix)          |
| 19  | Numeric    | EXPIRYDATE                             |
| 20  | String     | UNDERLYING                             |
| 21  | Numeric    | NTA (Net Tangible Asset)               |
| 22  | Numeric    | CONTRACTSIZE                           |
| 23  | String     | Underwriter                            |
| 24  | Numeric    | Bid price                              |
| 25  | Byte       | VERB                                   |
| 26  | Numeric    | STRIKEPRICE                            |
| 27  | Numeric    | High bid price                         |
| 28  | Numeric    | WEIGHT                                 |
| 29  | Numeric    | Low bid price                          |
| 30  | Numeric    | MATURITYDATE                           |
| 31  | Numeric    | Bid Volume                             |
| 32  | Numeric    | XGROUP1                                |
| 33  | String     | SECURITYTYPE                           |
| 34  | Numeric    | FLAG                                   |
| 35  | Numeric    | INDICATOR                              |
| 36  | Numeric    | RSI (Relative Strength Index)          |
| 37  | Numeric    | HIGH5                                  |
| 38  | Numeric    | Number of offer orders                 |
| 39  | Numeric    | Offer price                            |
| 40  | Numeric    | LOW5                                   |
| 41  | Numeric    | INDEX                                  |
| 42  | Numeric    | High offer price                       |
| 43  | Numeric    | FRGBOUGHTFREQ                          |
| 44  | Numeric    | Low offer price                        |
| 45  | Numeric    | DOMBOUGHTFREQ                          |
| 46  | Numeric    | Offer Volume                           |
| 47  | Numeric    | FRGSOLDFREQ                            |
| 48  | Numeric    | DOMSOLDFREQ                            |
| 49  | Numeric    | FRGBOUGHTVOL                           |
| 50  | Numeric    | DOMBOUGHTVOL                           |
| 51  | Numeric    | FRGSOLDVOL                             |
| 52  | Numeric    | DOMSOLDVOL                             |
| 53  | Numeric    | Number of bid orders                   |
| 54  | Numeric    | Open price                             |
| 55  | Numeric    | THEORETICALPRC                         |
| 56  | Numeric    | Last traded price                      |
| 57  | Numeric    | High traded price                      |
| 58  | Numeric    | THEORETICALVOL                         |
| 59  | Numeric    | Low trade price                        |
| 60  | Numeric    | CLOSE                                  |
| 61  | Numeric    | Close Date                             |
| 62  | Numeric    | (not available)                        |
| 63  | Numeric    | (not available)                        |
| 64  | Numeric    | (not available)                        |
| 65  | Numeric    | XBASEVAL                               |
| 66  | Numeric    | XMARKETVAL                             |
| 67  | Numeric    | CHANGE                                 |
| 68  | Numeric    | RATIO                                  |
| 69  | Numeric    | RECDATE                                |
| 70  | String     | BOARD (lihat Appendix)                 |
| 71  | Subst      | SOURCE                                 |
| 72  | Numeric    | VOL                                    |
| 73  | Numeric    | SHARELOT                               |
| 74  | Numeric    | FRGBOUGHTVAL                           |
| 75  | Numeric    | FRGSOLDVAL                             |
| 76  | Numeric    | DOMBOUGHTVAL                           |
| 77  | Numeric    | DOMSOLDVAL                             |
| 78  | Numeric    | AVG                                    |
| 79  | Numeric    | PCTCHANGE                              |

#### 5.3.1 Quote IDX Stock — contoh

```
IQP|20211223|173844|34049877|14|0;AALI|1;Astra Agro Lestari Tbk.|2;13793380360|...|79;1.02[CR/LF]
```

#### 5.3.2 Quote untuk Regional Index, Futures, Commodity, Currency

Hanya FID terbatas (umumnya 0, 24, 31, 39, 46, 56, 67, 79).

```
# Regional Index
IQP|20211223|150000|5520466|14|0;-FTSE|39;0|31;0|46;3084658944[CR/LF]

# Commodity
IQP|20211223|150000|5520487|14|0;-LGD|24;0|39;0|56;0|67;4234561|79;0|31;0|46;439563072[CR/LF]

# Currency
IQP|20211223|150000|5520497|14|0;AUD-USD|24;0|39;3490062592|56;0.00|67;2956977920|31;0|46;2205895168[CR/LF]
```

#### 5.3.3 Quote dengan Indikator (RSI, High5, Low5)

FID yang relevan: `34` FLAG, `35` INDICATOR, `36` RSI, `37` HIGH5, `40` LOW5.

```
IQP|20211223|150000|5520514|14|0;WIKA|1;Wijaya Karya (Persero) Tbk.|34;32768|35;5|36;80|37;1410|38;9|39;1410|40;1235[CR/LF]
```

### 5.4 Trade — Type `15`

| Field             | Type           | Deskripsi                                        |
|-------------------|----------------|--------------------------------------------------|
| Code              | Alphanumeric   | Stock Code                                       |
| Date              | Date           | YYYYMMDD                                         |
| Time              | Time           | HHMMSS                                           |
| Trade number      | Numeric        | Order Number                                     |
| Trade command     | Numeric        | `0` = matched, `1` = withdrawn                   |
| Price             | Numeric        | Last Price                                       |
| Volume            | Numeric        | Last Volume                                      |
| Buyer             | Alphanumeric   | `--` + 2 spasi (broker code disembunyikan)       |
| Buyer type        | Character      | `F` = foreign, `D` = domestic                    |
| Seller            | Alphanumeric   | `--` + 2 spasi                                   |
| Seller type       | Character      | `F` = foreign, `D` = domestic                    |
| Buyer order num   | Numeric        | Buyer order number                               |
| Seller order num  | Numeric        | Seller order number                              |

Contoh:

```
IQP|20211223|085900|69397|15|WIKA|20211208|085900|1|0|1225|200|--|D|--|D|48941|34504[CR/LF]
IQP|20211223|085900|69400|15|UNVR|20211208|085900|61|0|4300|100|--|F|--|F|70387|68326[CR/LF]
```

> **Catatan**: pada feed live, broker code disembunyikan menjadi `--`. Broker code asli baru muncul saat **Resend Trade** (lihat 5.5).

### 5.5 Resend Trade — Type `27`

Format identik dengan Trade (15), tapi field Buyer/Seller berisi broker code asli (2 huruf). Dikirim setelah market close.

```
IQP|20211223|173942|34650805|27|BOGA|20211223|101741|550509|0|1355|28900|FS|D|SH|D|1293549|1295557[CR/LF]
IQP|20211223|173942|34650807|27|LPKR|20211223|101741|550511|0|146|1200|AK|F|YP|D|1295566|217378[CR/LF]
```

### 5.6 Order — Type `16`

| Field          | Type           | Deskripsi                                           |
|----------------|----------------|-----------------------------------------------------|
| Code           | Alphanumeric   | Stock Code                                          |
| Time           | Time           | HHMMSS                                              |
| Order command  | Numeric        | `0`=Bid, `1`=Offer, `2`=Cancel Bid, `3`=Cancel Offer|
| Order number   | Numeric        | Order Number                                        |
| Price          | Numeric        | Price                                               |
| Volume         | Numeric        | Volume                                              |
| Broker         | Alphanumeric   | `--` (kosong jika board RG)                         |
| Balance        | Numeric        | Sisa order                                          |
| Investor       | Character      | `F` = foreign, `D` = domestic                       |
| No. Reference  | Numeric        | Reference number                                    |

Contoh:

```
IQP|20211223|090000|71577|16|BBCA|084500|0|1|6900|100|--|100|D|0[CR/LF]
IQP|20211223|090000|71580|16|INDF|084500|1|2|6725|10000|--|10000|D|0[CR/LF]
```

### 5.7 Resend Order — Type `26`

Format mengikuti Order (16). Dikirim setelah market close.

```
IQP|20211223|174028|35993438|26|ASII|090924|1|474168|5800|100|--|2415919103|D|0[CR/LF]
```

### 5.8 Top 20 — Type `17`

Daftar 20 kode (saham atau broker) berdasarkan kategori tertentu.

#### Kategori (Top 20 type)

| Tipe | Deskripsi                |
|-----:|--------------------------|
| 0    | Top 20 volume RG         |
| 1    | Top 20 value RG          |
| 2    | Top 20 frequency RG      |
| 3    | Top 20 gainer RG         |
| 4    | Top 20 loser RG          |
| 5    | Top 20 % gainer RG       |
| 6    | Top 20 % loser RG        |
| 7    | Top 20 volume non-RG     |
| 8    | Top 20 value non-RG      |
| 9    | Top 20 frequency non-RG  |
| 10   | Top 20 gainer non-RG     |
| 11   | Top 20 loser non-RG      |
| 12   | Top 20 % gainer non-RG   |
| 13   | Top 20 % loser non-RG    |
| 14   | Top 20 volume broker     |
| 15   | Top 20 value broker      |
| 16   | Top 20 frequency broker  |

> **Catatan**: Top 20 Broker (14–16) dikirim sekitar **2 jam setelah market close**.

Format: `<TopType>|Code1|Code2|...|Code20`

```
IQP|20211223|173842|34049833|17|0|CPRO|KBAG-W|KUAS|KBAG|BUKA|BGTG|BVIC|BIPI|BABP|DNAR|CARE|BBKP|LUCK|ZINC|YELO|FREN|ADRO|MLPL|NATO|BULL[CR/LF]
IQP|20211223|173842|34049847|17|14|YP|CC|AK|PD|YU|CP|MG|XA|KK|YB|AP|NI|AZ|XC|ZP|BK|EP|YJ|SQ|BQ[CR/LF]
```

### 5.9 Best Quote — Type `18`

Best bid / offer untuk satu saham; bisa berisi beberapa level harga.

| Field              | Type           | Deskripsi                            |
|--------------------|----------------|--------------------------------------|
| Code               | Alphanumeric   | Stock Code                           |
| Order book type    | Character      | `B` = Bid, `S` = Offer               |
| Price              | Numeric        | Price                                |
| Lot                | Numeric        | Order volume in lot                  |
| #order             | Numeric        | Total of orders                      |
| Lot Foreign        | Numeric        | Foreign order volume in lot          |
| #order Foreign     | Numeric        | Total of Foreign orders              |

Setiap level harga ditulis sebagai grup field dipisah `;`, antar level dipisah `|`.

```
IQP|20211223|090000|71620|18|INDF|S|6400;251;1;0;0|6725;100;1;0;0[CR/LF]
IQP|20211223|090000|71630|18|INDF|B|6375;25;1;0;0[CR/LF]
```

> **Update v4.0.0**: ditambahkan field `Lot Foreign` dan `#order Foreign` (dua field terakhir) — dukungan order foreign per level.

### 5.10 Trade Done — Type `40`

Akumulasi trade per harga (per stock per price level) sepanjang sesi.

| Field   | Deskripsi                |
|---------|--------------------------|
| Code    | Stock Code               |
| Price   | Price level              |
| Bvol    | Buy Volume               |
| Svol    | Sell Volume              |
| bfreq   | Buy Frequency            |
| Sfreq   | Sell Frequency           |
| bfreqF  | Foreign Buy Frequency    |
| BvolF   | Foreign Buy Volume       |
| SfreqF  | Foreign Sell Frequency   |
| SvolF   | Foreign Sell Volume      |

```
IQP|20211223|173847|34053332|40|MDKA|3710|736200|290100|317|50|12|5800|15|104900[CR/LF]
IQP|20211223|173847|34053336|40|MDKA|3750|6696900|1400600|1319|286|35|159000|20|172000[CR/LF]
```

### 5.11 Trading Summary — Type `130`

Ringkasan harian per kombinasi `Stype` × `Board`.

| Field       | Deskripsi / Nilai                                           |
|-------------|-------------------------------------------------------------|
| Stype       | `0`=S_ORDI, `1`=S_PREOP, `2`=S_RIGHT, `3`=S_WARAN, `4`=S_MUTI, `5`=S_ACCEL, `6`=S_SWARAN, `7`=S_PREFEREN, `8`=S_WATCHLIST |
| Board       | `RG`=Regular, `TN`=Tunai, `NG`=Negosiasi                    |
| Frequency   | Total transaction count                                     |
| Volume      | Total share volume                                          |
| Value       | Total transaction value                                     |
| FBfreq      | Foreign Bought Frequency                                    |
| FBvol       | Foreign Bought Volume                                       |
| FBval       | Foreign Bought Value                                        |
| FSfreq      | Foreign Sold Frequency                                      |
| FSvol       | Foreign Sold Volume                                         |
| FSval       | Foreign Sold Value                                          |

```
IQP|20211223|173843|34049850|130|0|RG|13563|109001000|25679409800|240|3301800|1160696000|162|1637200|623024700[CR/LF]
IQP|20211223|173843|34049853|130|1|RG|1382019|21129279300|12329785458700|130361|1680723500|2842014351600|154943|1831372200|2738172587900[CR/LF]
```

### 5.12 NBS Stock — Type `58`

Net Buy/Sell view per **stock**, dipecah per broker.

| Field        | Deskripsi                |
|--------------|--------------------------|
| Stock_code   | Stock Code               |
| Broker_code  | Broker Code              |
| Bfreq        | Buy Frequency            |
| Bvol         | Buy Volume               |
| Blot         | Buy Lot                  |
| Bval         | Buy Value                |
| Bpct         | Buy Value (Percentage)   |
| Sfreq        | Sell Frequency           |
| Svol         | Sell Volume              |
| Slot         | Sell Lot                 |
| Sval         | Sell Value               |
| Spct         | Sell Value (Percentage)  |

```
IQP|20211223|104551|2940338|58|BBYB|PD|3989|13206300|132063|35407613000|0.271950|3076|10622300|106223|28449851000|0.218510[CR/LF]
```

### 5.13 NBS Broker — Type `59`

Net Buy/Sell view per **broker**, dipecah per saham. Field identik dengan NBS Stock kecuali urutan: `Broker_code` dulu lalu `Stock_code`.

```
IQP|20211223|090000|71642|59|PD|BBYB|3988|13206000|132060|35406836000|0.271944|3076|10622300|106223|28449851000|0.218510[CR/LF]
IQP|20211223|090000|71644|59|GR|ANTM|83|217500|2175|509767000|0.003915|23|200700|2007|470538000|0.003614[CR/LF]
```

### 5.14 News — Type `36`

Berita ekonomi & bisnis (kategori `BIS`).

| Field            | Deskripsi                                                |
|------------------|----------------------------------------------------------|
| News_type        | `1` = OK                                                 |
| Num_packet       | Total paket (max 1024 char per paket)                    |
| Current_packet   | Nomor paket saat ini, mulai dari 1                       |
| News_id          | ID berita                                                |
| Date / Time      | YYYYMMDD / HHMMSS                                        |
| Category         | `BIS` (Ekonomi Bisnis)                                   |
| Company_id       | Subjek berita / kode emiten                              |
| Headline         | Judul berita                                             |
| Story            | Konten berita (chunked sesuai `Num_packet`)              |

> Headline tetap sama selama `News_id` sama; story dipotong jadi paket-paket.

Contoh paket pertama:

```
IQP|20211223|185306|13|36|1|4|1|1640260352855278|20211223|185306|BIS|TLKM|TLKM Laba Rp 18,9 T, IndiHome & Layanan Digital Jadi Andalan|<konten paket 1>
```

**Delete News**:

```
IQP|<News_delete_Msg>|<news_id>[CR/LF]

# contoh
IQP|157|65102011[CR/LF]
```

### 5.15 Stock Activity — Type `39`

Statistik agregat aktivitas saham di market.

| Field      | Deskripsi                          |
|------------|------------------------------------|
| inactive   | Stock not active                   |
| active     | Stock active                       |
| down       | Stock active, harga turun          |
| nochg      | Stock active, harga tidak berubah  |
| up         | Stock active, harga naik           |

```
IQP|20211223|090000|71638|39|2512|140|25|46|69[CR/LF]
IQP|20211223|090000|71639|39|2430|222|31|82|109[CR/LF]
```

---

## 6. Appendix

### 6.1 XGroup (FID 2 di Quote) — Bitmask Index Membership

Nilai FID 2 adalah **bitmask** (penjumlahan `2^n`). Untuk cek keanggotaan sebuah index, lakukan `XGroup AND 2^n`. Bila hasilnya = `2^n`, saham termasuk konstituen index tersebut.

| #  | Index        | Bit  | Value    |
|---:|--------------|------|---------:|
| 1  | IDXENERGY    | 2^0  | 1        |
| 2  | IDXBASIC     | 2^1  | 2        |
| 3  | IDXINDUST    | 2^2  | 4        |
| 4  | IDXNONCYC    | 2^3  | 8        |
| 5  | IDXCYCLIC    | 2^4  | 16       |
| 6  | IDXHEALTH    | 2^5  | 32       |
| 7  | IDXFINANCE   | 2^6  | 64       |
| 8  | IDXPROPERT   | 2^7  | 128      |
| 9  | IDXTECHNO    | 2^8  | 256      |
| 10 | IDXINFRA     | 2^9  | 512      |
| 11 | IDXTRANS     | 2^10 | 1024     |
| 12 | MBX          | 2^11 | 2048     |
| 13 | DBX          | 2^12 | 4096     |
| 14 | ABX          | 2^13 | 8192     |
| 15 | COMPOSITE    | 2^14 | 16384    |
| 16 | LQ45         | 2^15 | 32768    |
| 17 | JII          | 2^16 | 65536    |
| 18 | KOMPAS100    | 2^17 | 131072   |
| 19 | BISNIS-27    | 2^18 | 262144   |
| 20 | PEFINDO25    | 2^19 | 524288   |
| 21 | SRI-KEHATI   | 2^20 | 1048576  |
| 22 | ISSI         | 2^21 | 2097152  |
| 23 | IDX30        | 2^22 | 4194304  |
| 24 | INFOBANK15   | 2^23 | 8388608  |
| 25 | SMinfra18    | 2^24 | 16777216 |
| 26 | MNC36        | 2^25 | 33554432 |
| 27 | INVESTOR33   | 2^26 |          |
| 28 | I-GRADE      | 2^27 |          |
| 29 | IDXSMC-COM   | 2^28 |          |
| 30 | IDXSMC-LIQ   | 2^29 |          |
| 31 | IDXHIDIV20   | 2^30 |          |
| 32 | IDXBUMN20    | 2^31 |          |
| 33 | JII70        | 2^32 |          |
| 34 | IDX80        | 2^33 |          |
| 35 | IDXV30       | 2^34 |          |
| 36 | IDXG30       | 2^35 |          |
| 37 | IDXQ30       | 2^36 |          |
| 38 | IDXESGL      | 2^37 |          |
| 39 | IDXMESBUMN   | 2^38 |          |
| 40 | ESGSKEHATI   | 2^39 |          |
| 41 | ESGQKEHATI   | 2^40 |          |

> Index baru dari IDX akan otomatis ditambahkan dengan formula `2^n`.

**Contoh**: SILO punya `XGroup = 270551072`. Cek IDXHEALTH (bit `2^5 = 32`):
`270551072 AND 32 = 32` ⇒ termasuk konstituen IDXHEALTH.

### 6.2 Status (FID 3 di Quote)

| Value | Arti     |
|------:|----------|
| 0     | Active   |

### 6.3 SectorName (FID 17 di Quote)

Nilai string sektor sesuai klasifikasi IDX-IC:

- Energy
- Basic Materials
- Industrials
- Consumer Non-Cyclicals
- Consumer Cyclicals
- Healthcare
- Financials
- Properties & Real Estate
- Technology
- Infrastructures
- Transportation & Logistic
- Listed Investment Product

### 6.4 IndustryName (FID 18 di Quote)

String industri (subset dari IDX-IC), antara lain:

```
Oil & Gas Production & Refinery, Oil & Gas Storage & Distribution,
Coal Production, Coal Distribution, Oil & Gas Drilling Service,
Alternative Energy Equipment, Alternative Fuels,
Basic Chemicals, Agricultural Chemicals, Specialty Chemicals,
Construction Materials, Containers & Packaging,
Aluminum, Cooper, Gold, Iron & Steel, Precious Metals & Minerals,
Diversified Metals & Minerals, Mining Equipment & Services,
Timber, Paper, Diversified Forest,
Aerospace & Defense, Building Products & Fixtures,
Electrical Components & Equipment, Heavy Electrical Equipment,
Construction Machinery & Heavy Vehicles, Agricultural & Farm Machinery,
Industrial Machinery & Components, Diversified Industrial Trading,
Commercial Printing, Environmental & Facilities Services,
Office Supplies, Business Support Services,
Human Resource & Employment Services, Research & Consulting Services,
Multi-sector Holdings,
Drug Retail & Distributors, Food Retail & Distributors,
Supermarket & Convenience Store,
Liquors, Soft Drinks, Dairy Products, Processed Foods,
Fish/Meat & Poultry, Plantation & Crops, Tobacco,
Household Products, Personal Care Products,
Auto Parts & Equipment, Tires, Car Manufacturers, Motorcycle Manufacturers,
Home Furnishings, Household Appliance, Housewares & Specialties,
Consumer Electronics, Sport Equipment & Hobbies Goods,
Clothing/Accessories & Bags, Footwear, Textiles,
Gaming Venue, Hotels/Resorts & Cruise Lines, Travel Agencies,
Recreational & Sports Facilities, Restaurants,
Education Services, Consumer Support Service,
Media Advertising, Media Broadcasting, Media Cables & Satellite,
Media Consumer Publishing, Entertainment & Movie Production,
Consumer Distributors, Internet & Homeshop Retail,
Department Stores, Apparel & Textile Retail, Electronics Retail,
Home Improvement Retail, Specialty Store, Automotive Retail,
Healthcare Equipment, Healthcare Supplies & Distributors,
Healthcare Providers, Pharmaceuticals, Healthcare Research,
Banks, Consumer Financing, Venture Capital,
Specialize Business Financing, Investment Management,
Investment Banking & Brokerage Service, Market Operators,
Investment Services Support, Insurance Brokers,
General Insurance, Life Insurance, Reinsurance,
Financial Holdings, Investment Companies,
Real Estate Development & Management, Real Estate Service,
Online Applications & Services, IT Services & Consulting,
Software, Networking Equipment, Computer Hardware,
Electronic Equipment & Instruments, Electronic Components & Semiconductors,
Airport Operators, Highway & Railtracks, Marine Ports & Services,
Heavy Constructions & Civil Engineering,
Wired Telecommunication Service, Integrated Telecommunication Service,
Wireless Telecommunication Services,
Electric Utilities, Gas Utilities, Water Utilities,
Airlines, Passenger Marine Transportation, Rail, Road Transportation,
Logistics & Deliveries,
Mutual Fund / ETFs, Real Estate Investment Trusts,
Infrastructure Investment Trusts, Government Bonds, Corporate Bonds
```

### 6.5 Board (FID 70 di Quote)

| Code | Arti                |
|------|---------------------|
| `RG` | Regular Market      |
| `NG` | Negotiation Market  |
| `TN` | Cash Market         |
| `00` | Broker              |
| `01` | Currency            |
| `02` | Regional Index      |
| `03` | IDX Index           |
| `04` | Commodity           |

---

## 7. Changelog (rangkuman)

### v3.8.20-PreClosing → v4.0.0

1. **Record Type baru**:
   - `149` Login (Record for Login) — sebelumnya implisit
   - `58` NBS Stock (Net Buy/Sell Stock) — *Permission Required*
   - `59` NBS Broker (Net Buy/Sell Broker) — *Permission Required*
2. **Best Quote (Type 18)** mendapat 2 field tambahan: `Lot Foreign`, `#order Foreign`.

### v3.8.20 → v3.8.20-PreClosing

- Broker code di Trade & Order disembunyikan menjadi flag `--` selama market jam (broker code asli tetap tersedia di Resend Trade / Resend Order setelah market close).

### v3.7.35 → v3.8.20

- Order (16) ditambah field `Investor` dan `No. Reference`.
- Best Quote (18) ditambah field `#order`.

---

## 8. Catatan Implementasi (untuk integrator Tuai)

Hal-hal penting saat membangun client TCP IQPlus:

1. **Banner sebelum login**: server kirim binary banner sebelum response login (lihat memory `IQPlus TCP banner prefix`).
2. **Encoding**: Plain text ASCII; pastikan parsing strict pada delimiter `|` dan `;`.
3. **Sequence**: `Sequence#` reset tiap hari mulai dari `1` — jangan diandalkan untuk dedup lintas hari.
4. **Time fields**: Tidak ada timezone di payload; default WIB (Asia/Jakarta) sesuai jam IDX.
5. **Best Quote multi-level**: Satu record bisa berisi >1 level harga (terlihat dari multiple group `;-separated` yang dipisah `|`).
6. **Trade vs Resend Trade**: Selama market hours pakai Trade (15) dengan broker `--`; setelah close konsumsi Resend Trade (27) untuk dapat broker code asli (audit, NBS reconciliation).
7. **Top 20 Broker**: Tipe 14–16 dikirim ~2 jam setelah market close — jangan polling siang hari.
8. **News chunking**: Story bisa lebih dari 1 paket (lihat `Num_packet` & `Current_packet`); concat berurutan untuk dapat full story.
9. **XGroup bitmask**: Untuk filter konstituen index gunakan bitwise AND, bukan equality.
10. **Permission**: Hampir semua data type butuh permission khusus (kontrak dengan IQPlus). Perhatikan error `9 Unauthorized user` saat login.
