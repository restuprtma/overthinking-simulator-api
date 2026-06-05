# nats-edge — Durable JetStream Spool di FreeBSD VM

Edge nats-server yang jalan **bersebelahan dengan iqplus-publisher** di FreeBSD
VM (`10.10.8.1`). Tujuannya: TCP push dari IQPlus selalu berakhir di disk lokal
sebelum direplikasi ke main NATS cluster (`10.10.8.2`) — sehingga partisi LAN
atau outage di main tidak pernah menyebabkan data loss.

> Detail arsitektur & runbook migrasi: [docs/infra/iqplus-edge-topology.md](../../../docs/infra/iqplus-edge-topology.md).

---

## Target host

| Item | Value |
|---|---|
| Host | `10.10.8.1` |
| User | `landa` (uid 1001, no sudo) |
| OS | FreeBSD 14.x amd64 |
| Deploy root | `/home/landa/nats-edge/` |
| Listen | `0.0.0.0:4222` (NATS), `127.0.0.1:8222` (HTTP monitoring) |
| Storage | `/home/landa/nats-edge/data/jetstream/` |
| Supervisor | `/usr/sbin/daemon -r` (auto-restart) |

### Directory layout di host

```
/home/landa/nats-edge/
├── bin/
│   └── nats-server                  # binary statis FreeBSD/amd64 (mode 0700)
├── conf/
│   ├── nats-server.conf
│   └── nats-edge.env                # NATS_TOKEN (mode 0600)
├── data/
│   └── jetstream/                   # JetStream file store
├── log/
│   └── nats-server.log
├── run/
│   ├── daemon.pid
│   └── nats-server.pid
└── scripts/
    ├── install-binary.sh
    ├── start.sh
    ├── stop.sh
    ├── status.sh
    └── streams-add.sh
```

---

## First-time deploy

> Jalankan dari workstation. SSH password atau key sesuai existing setup.

### 1. Generate token & siapkan env file lokal

```bash
TOKEN=$(openssl rand -hex 32)
echo "NATS_TOKEN=$TOKEN" > deployments/freebsd/nats-edge/conf/nats-edge.env
chmod 600 deployments/freebsd/nats-edge/conf/nats-edge.env

# Save token-mu di password manager — perlu untuk:
#   - publisher .env (NATS_TOKEN)
#   - main NATS context untuk stream-source
echo "Edge token: $TOKEN"
```

### 2. Siapkan direktori di host

```bash
ssh landa@10.10.8.1 'sh -c "
  mkdir -p ~/nats-edge/bin \
           ~/nats-edge/conf \
           ~/nats-edge/data/jetstream \
           ~/nats-edge/log \
           ~/nats-edge/run \
           ~/nats-edge/scripts
"'
```

### 3. Upload config, scripts, dan env

```bash
# Config & scripts
scp deployments/freebsd/nats-edge/nats-server.conf \
    landa@10.10.8.1:~/nats-edge/conf/nats-server.conf

scp deployments/freebsd/nats-edge/scripts/*.sh \
    landa@10.10.8.1:~/nats-edge/scripts/

ssh landa@10.10.8.1 'chmod +x ~/nats-edge/scripts/*.sh'

# Env (mode 0600)
scp deployments/freebsd/nats-edge/conf/nats-edge.env \
    landa@10.10.8.1:~/nats-edge/conf/nats-edge.env
ssh landa@10.10.8.1 'chmod 600 ~/nats-edge/conf/nats-edge.env'
```

### 4. Install binary nats-server di host

```bash
ssh landa@10.10.8.1 'sh ~/nats-edge/scripts/install-binary.sh'
# Output: nats-server: v2.10.20
```

### 5. Start

```bash
ssh landa@10.10.8.1 '~/nats-edge/scripts/start.sh'
# Cek:
ssh landa@10.10.8.1 '~/nats-edge/scripts/status.sh'
```

### 6. Buat 4 stream IDX di edge

Jalankan dari workstation (tidak perlu di host — `nats` CLI cukup point ke
`nats://10.10.8.1:4222`):

```bash
nats context save iqplus-edge \
  --server nats://10.10.8.1:4222 \
  --token "$TOKEN"
nats context select iqplus-edge

sh deployments/freebsd/nats-edge/scripts/streams-add.sh

nats stream ls
# Expect: IDX_TICK, IDX_QUOTE, IDX_META, IDX_NEWS
```

### 7. Auto-start saat reboot (crontab)

Pakai pola yang sama seperti iqplus-publisher:

```bash
ssh landa@10.10.8.1 'crontab -l 2>/dev/null; echo "@reboot $HOME/nats-edge/scripts/start.sh >> $HOME/nats-edge/log/cron.log 2>&1" | crontab -'
```

> Penting: edge harus start **sebelum** iqplus-publisher. Cara aman: kasih
> sleep 5s sebelum @reboot publisher, atau biarkan publisher reconnect-loop
> (default 5s) yang akan handle race-nya.

---

## Operasional

### Restart

```bash
ssh landa@10.10.8.1 '~/nats-edge/scripts/stop.sh && sleep 2 && ~/nats-edge/scripts/start.sh'
```

### Cek streams

```bash
nats stream ls --context iqplus-edge
nats stream report --context iqplus-edge
nats stream info IDX_TICK --context iqplus-edge
```

### Cek lag (edge → main mirror)

```bash
# Di main NATS (10.10.8.2):
nats stream info IDX_TICK
# Lihat field "Source", harus active dan lag = 0 dalam kondisi normal.
```

### Disk usage

```bash
ssh landa@10.10.8.1 'du -sh ~/nats-edge/data/jetstream'
# Steady state: 5–15 GB tergantung retention & volume.
```

### Tail log

```bash
ssh landa@10.10.8.1 'tail -f ~/nats-edge/log/nats-server.log'
```

---

## Hardening (opsional, recommended)

- **Disk**: idealnya `data/jetstream/` di volume ZFS mirror dedicated. Kalau
  Proxmox host pakai stripe/RAID-0, **kurangi `--max-age` ke 6h** dan anggap
  main NATS source-of-truth.
- **Firewall**: hanya `10.10.8.2` yang perlu reach `10.10.8.1:4222`. Restrict
  via `ipfw` atau Proxmox firewall.
- **FreeBSD sysctl** (butuh root, lakukan saat maintenance window):
  ```
  sysctl -w kern.ipc.maxsockbuf=16777216
  sysctl -w net.inet.tcp.recvbuf_max=8388608
  sysctl -w net.inet.tcp.recvbuf_auto=1
  ```
  Persist di `/etc/sysctl.conf`. Ini tetap berguna untuk TCP socket
  iqplus-publisher saat opening burst.

---

## Troubleshooting

| Gejala | Cek |
|---|---|
| `start.sh` exit segera, no daemon | `cat ~/nats-edge/log/nats-server.log` — biasanya port conflict, config syntax, atau JetStream store path tidak writable |
| Publisher tidak konek | Token mismatch antara `nats-edge.env` dan publisher `.env` — pastikan sama persis |
| Main NATS tidak nge-source | Token mismatch antara main context dan edge token; atau firewall blokir 10.10.8.1:4222 dari 10.10.8.2 |
| Disk usage naik terus | Stream `--max-age` tidak ke-set, atau main mirror tidak jalan (data tidak dikonsumsi). Cek `nats stream info <name>` field "Subjects" dan "Messages" |
