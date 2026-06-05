# iqplus-publisher — FreeBSD Deploy

User-level deploy of [`cmd/iqplus-publisher`](../../cmd/iqplus-publisher/) to a
FreeBSD host without root privileges. Service runs under `daemon(8)` supervisor,
auto-starts on reboot via `crontab @reboot`, and logs are rotated daily.

---

## Target host

| Item | Value |
|---|---|
| Host | `10.10.8.1` |
| User | `landa` (uid 1001, no sudo/doas, not in `wheel`) |
| OS | FreeBSD 14.4-RELEASE amd64 |
| Deploy root | `/home/landa/iqplus-publisher/` |
| Auto-start | `crontab @reboot` |
| Supervisor | `/usr/sbin/daemon -r` (auto-restart on crash, 5s delay) |

### Directory layout on host

```
/home/landa/iqplus-publisher/
├── bin/
│   ├── iqplus-publisher            # binary (mode 0700)
│   └── iqplus-publisher.env        # secrets (mode 0600)
├── scripts/
│   ├── start.sh
│   ├── stop.sh
│   ├── status.sh
│   └── rotate-log.sh
├── log/
│   ├── iqplus-publisher.log        # current (stdout+stderr from daemon -o)
│   ├── iqplus-publisher.log.YYYYMMDD.gz  # rotated archives (7 days)
│   └── cron.log                    # cron job stdout/stderr
└── run/
    ├── daemon.pid                  # supervisor pid
    └── publisher.pid               # publisher pid
```

---

## First-time deploy

> Run from repo root on the dev workstation. SSH password authentication is
> assumed; for production, switch to SSH keys (see "Hardening" below).

### 1. Cross-compile the binary

```bash
make build-iqplus-publisher-freebsd
# Produces: bin/iqplus-publisher-freebsd-amd64 (static, ~7 MB)
```

The Makefile target uses `GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0`
([Makefile:157-162](../../Makefile#L157-L162)). No libc dependency — the binary
runs on any FreeBSD/amd64 host.

### 2. Prepare directories on the host

```bash
ssh landa@10.10.8.1 'sh -c "
  mkdir -p ~/iqplus-publisher/bin \
           ~/iqplus-publisher/log \
           ~/iqplus-publisher/run \
           ~/iqplus-publisher/scripts
"'
```

> The default shell for `landa` is `csh`, which does **not** expand
> `mkdir -p ~/foo/{a,b,c}` braces. Always wrap multi-arg commands in `sh -c "…"`.

### 3. Prepare local env from template

```bash
cp deployments/freebsd/iqplus-publisher.env.example \
   deployments/freebsd/iqplus-publisher.env
# Edit it: fill in IQPLUS_USER, IQPLUS_PASS_MD5, NATS_TOKEN
```

The real `iqplus-publisher.env` is in `.gitignore`; only the `.example`
template is committed.

### 4. Upload binary + env + scripts

```bash
scp bin/iqplus-publisher-freebsd-amd64 \
    landa@10.10.8.1:/home/landa/iqplus-publisher/bin/iqplus-publisher

scp deployments/freebsd/iqplus-publisher.env \
    landa@10.10.8.1:/home/landa/iqplus-publisher/bin/iqplus-publisher.env

scp deployments/freebsd/scripts/start.sh \
    deployments/freebsd/scripts/stop.sh \
    deployments/freebsd/scripts/status.sh \
    deployments/freebsd/scripts/rotate-log.sh \
    landa@10.10.8.1:/home/landa/iqplus-publisher/scripts/
```

### 5. Set permissions

```bash
ssh landa@10.10.8.1 'sh -c "
  chmod 0700 ~/iqplus-publisher/bin/iqplus-publisher
  chmod 0600 ~/iqplus-publisher/bin/iqplus-publisher.env
  chmod 0700 ~/iqplus-publisher/scripts/*.sh
"'
```

`iqplus-publisher.env` contains `NATS_TOKEN` and `IQPLUS_PASS_MD5` — the
`0600` perms are mandatory.

### 6. Install crontab (auto-start + log rotation)

Run **once** per host:

```bash
ssh landa@10.10.8.1 'sh -c "
  ( crontab -l 2>/dev/null | grep -v iqplus-publisher ; cat <<EOF
@reboot /home/landa/iqplus-publisher/scripts/start.sh >> /home/landa/iqplus-publisher/log/cron.log 2>\&1
5 0 * * * /home/landa/iqplus-publisher/scripts/rotate-log.sh >> /home/landa/iqplus-publisher/log/cron.log 2>\&1
EOF
) | crontab -
  crontab -l
"'
```

### 7. Start the service

```bash
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/start.sh'
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/status.sh'
```

Expected first log lines (in `log/iqplus-publisher.log`):

```
loaded env from /home/landa/iqplus-publisher/bin/iqplus-publisher.env
{"level":"info","msg":"iqplus-publisher starting", ...}
{"level":"info","msg":"nats jetstream ready","streams":["IDX_TICK","IDX_QUOTE","IDX_META","IDX_NEWS"]}
{"level":"info","msg":"iqplus dialing","addr":"103.114.143.237:8888"}
{"level":"info","msg":"iqplus login response","line":"IQP|149|0|0|OK"}
{"level":"info","msg":"iqplus login success, streaming","user":"venturo"}
```

After 30s a stats line should appear (`STATS_INTERVAL=30s`):

```
{"level":"info","msg":"iqplus publisher stats","consumed":...,"rate_per_sec":...,"ok":...,"err":0,"dropped":0}
```

---

## Redeploy / update binary

When the publisher code changes, only the binary needs to be replaced — env
file, scripts, and crontab stay the same.

```bash
# 1. Build new binary on dev machine
make build-iqplus-publisher-freebsd

# 2. Stop running service (graceful drain ~ up to 30s)
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/stop.sh'

# 3. Upload new binary
scp bin/iqplus-publisher-freebsd-amd64 \
    landa@10.10.8.1:/home/landa/iqplus-publisher/bin/iqplus-publisher

# 4. Make sure it stays executable
ssh landa@10.10.8.1 'chmod 0700 ~/iqplus-publisher/bin/iqplus-publisher'

# 5. Start
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/start.sh'

# 6. Verify
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/status.sh'
```

### Atomic update (zero-downtime not needed but cleaner)

```bash
scp bin/iqplus-publisher-freebsd-amd64 \
    landa@10.10.8.1:/home/landa/iqplus-publisher/bin/iqplus-publisher.new
ssh landa@10.10.8.1 'sh -c "
  chmod 0700 ~/iqplus-publisher/bin/iqplus-publisher.new
  ~/iqplus-publisher/scripts/stop.sh
  mv ~/iqplus-publisher/bin/iqplus-publisher.new ~/iqplus-publisher/bin/iqplus-publisher
  ~/iqplus-publisher/scripts/start.sh
"'
```

### Update env / config only

```bash
# Edit deployments/freebsd/iqplus-publisher.env locally, then:
scp deployments/freebsd/iqplus-publisher.env \
    landa@10.10.8.1:/home/landa/iqplus-publisher/bin/iqplus-publisher.env
ssh landa@10.10.8.1 'sh -c "
  chmod 0600 ~/iqplus-publisher/bin/iqplus-publisher.env
  ~/iqplus-publisher/scripts/stop.sh
  ~/iqplus-publisher/scripts/start.sh
"'
```

> The publisher reads env once at startup. Any env change requires a restart.

### Update helper scripts

```bash
scp deployments/freebsd/scripts/*.sh \
    landa@10.10.8.1:/home/landa/iqplus-publisher/scripts/
ssh landa@10.10.8.1 'chmod 0700 ~/iqplus-publisher/scripts/*.sh'
```

(No restart needed — `start.sh` etc. are read on next invocation.)

---

## Daily operations

### Check status

```bash
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/status.sh'
```

Output includes: running state, daemon + publisher pids, RSS/VSZ, last 5 stats
lines, last 10 ERROR/WARN log lines.

### Tail live log

```bash
ssh landa@10.10.8.1 'tail -f ~/iqplus-publisher/log/iqplus-publisher.log'
```

Filter for stats only:

```bash
ssh landa@10.10.8.1 'tail -f ~/iqplus-publisher/log/iqplus-publisher.log | grep stats'
```

### Search for errors / warnings

```bash
ssh landa@10.10.8.1 'grep -iE "\"level\":\"(error|warn)\"" ~/iqplus-publisher/log/iqplus-publisher.log | tail -50'
```

Search archived logs too:

```bash
ssh landa@10.10.8.1 'sh -c "
  zgrep -hE \"level.:.error\" ~/iqplus-publisher/log/iqplus-publisher.log.*.gz | tail -50
"'
```

### Stop / start / restart

```bash
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/stop.sh'
ssh landa@10.10.8.1 '~/iqplus-publisher/scripts/start.sh'

# Restart in one shot:
ssh landa@10.10.8.1 'sh -c "~/iqplus-publisher/scripts/stop.sh && ~/iqplus-publisher/scripts/start.sh"'
```

`stop.sh` sends `SIGTERM` to the daemon supervisor, which forwards to the
publisher. Publisher's main() handles `SIGTERM` and drains NATS for up to
`NATS_DRAIN_TIMEOUT` (30s default). After 35s without exit, `stop.sh`
escalates to `SIGKILL`.

---

## How log rotation works

`daemon(8)` does not reopen its `-o` log on `SIGHUP`, so we use **copy +
truncate** instead of `mv + signal`. `rotate-log.sh` runs daily at 00:05:

1. `cp current.log archive.YYYYMMDD`
2. `: > current.log` (truncate in place — the daemon's open fd keeps writing)
3. `gzip archive.YYYYMMDD`
4. Keep newest 7 archives, delete the rest.

The truncate-in-place keeps the supervisor's file descriptor valid; the
publisher continues writing to the same inode without restart.

If you ever change `daemon -o` to a different log path, also update
`rotate-log.sh` and the path in `status.sh`.

---

## Error log handling

Currently `start.sh` merges stdout + stderr into one file
(`log/iqplus-publisher.log`) using `daemon -o`. Errors are emitted by the Zap
logger as JSON lines with `"level":"error"` or `"level":"warn"`, so:

- `status.sh` greps the merged log for `"level":"(error|warn)"` and shows the
  last 10.
- For a long-term error-only feed, run:
  ```bash
  ssh landa@10.10.8.1 'sh -c "
    grep -E \"level.:.(error|warn)\" ~/iqplus-publisher/log/iqplus-publisher.log >> ~/iqplus-publisher/log/iqplus-publisher.error.log
  "'
  ```
  (Or wire that into a separate cron entry if you want a curated error log.)

If you instead want stdout vs stderr split, change `start.sh` to use
`daemon -o stdout.log -e stderr.log` (FreeBSD `daemon` supports `-e` since
13.0). Then update `rotate-log.sh` to handle both files and `status.sh` to
read `stderr.log` directly.

---

## Auto-start on reboot

Provided by:

```cron
@reboot /home/landa/iqplus-publisher/scripts/start.sh >> /home/landa/iqplus-publisher/log/cron.log 2>&1
```

Verify after a reboot:

```bash
ssh landa@10.10.8.1 'sh -c "
  uptime
  ~/iqplus-publisher/scripts/status.sh
  tail -20 ~/iqplus-publisher/log/cron.log
"'
```

**Note**: the user-level cron only fires when `cron(8)` is running on the
host (it normally is — base FreeBSD enables it by default). To confirm:

```bash
ssh landa@10.10.8.1 'service cron status 2>/dev/null || pgrep -lf cron'
```

---

## Troubleshooting

### Service won't start

```bash
ssh landa@10.10.8.1 'sh -c "
  # 1. Run binary in foreground to see immediate errors
  ~/iqplus-publisher/bin/iqplus-publisher
"'
# Ctrl-C to abort. Common issues:
#   - missing required env var → IQPLUS_USER, IQPLUS_PASS_MD5, NATS_URL
#   - NATS unreachable        → check VPN / network ACL
#   - stream not found        → set NATS_ENFORCE_STREAM=false during initial bringup
```

### Stale pidfile

If `start.sh` says "Already running" but `ps` shows nothing:

```bash
ssh landa@10.10.8.1 'sh -c "
  rm -f ~/iqplus-publisher/run/daemon.pid ~/iqplus-publisher/run/publisher.pid
  ~/iqplus-publisher/scripts/start.sh
"'
```

### Connection drops to IQPlus

Look for `tcp_reconnects` in stats lines. The client auto-reconnects after
`IQPLUS_RECONNECT_DELAY=5s`. Persistent drops usually indicate:

- IQPlus server-side rate-limit or session cap (multiple clients with same
  user) — only one publisher per `IQPLUS_USER` should run.
- Network MTU / firewall idle-timeout — increase `IQPLUS_READ_TIMEOUT` if
  needed.

### NATS publish errors

`err_queue` (async pending overflow) → bump `NATS_ASYNC_MAX_PENDING`.
`err_ack_timeout` → bump `NATS_PUBLISH_TIMEOUT` or check JetStream disk I/O.

### Buffer drops during opening burst

`tcp_dropped > 0` in stats → bump `IQPLUS_BUFFER_SIZE` (default 200000 sized
for IDX 09:00:00 burst). Also verify `IQPLUS_SEND_TIMEOUT=0` so TCP back-pressures
instead of dropping.

---

## Hardening (production checklist)

- [ ] **Switch from password to SSH key auth** — generate a key on the dev
  workstation, add the public key to `~landa/.ssh/authorized_keys` on the host,
  then disable password SSH for `landa` (requires root, ask sysadmin). The
  password used during initial bring-up should be rotated afterwards.
- [ ] **Rotate `IQPLUS_PASS_MD5` and `NATS_TOKEN`** if anyone outside the
  current operator has seen them — the env file `chmod 0600` is the only thing
  protecting them on the host.
- [ ] **Move to system-wide rc.d** if you ever get root: `/usr/local/etc/rc.d/iqplus_publisher`
  with a dedicated `iqplus` user is cleaner than `crontab @reboot`. The current
  user-level setup is fine but tied to `landa`'s account.
- [ ] **Monitor `tcp_dropped`, `err`, `dropped`** in the stats line via NATS
  consumer / Grafana — alert on non-zero values.

---

## Build matrix

If the host arch ever changes, override `GOARCH`:

```bash
GOOS=freebsd GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o bin/iqplus-publisher-freebsd-arm64 ./cmd/iqplus-publisher
```

Same pattern for the seven other consumer binaries — they all have
`-freebsd` Makefile targets ([Makefile:157-266](../../Makefile#L157-L266)).

---

## Files in this directory

| File | Purpose |
|---|---|
| `README.md` | This document |
| `iqplus-publisher.env` | Production env (committed copy, also uploaded to host) |
| `scripts/start.sh` | Launch under `daemon(8)` supervisor |
| `scripts/stop.sh` | Graceful shutdown with 35s SIGTERM-then-KILL fallback |
| `scripts/status.sh` | Process state + recent stats + recent errors |
| `scripts/rotate-log.sh` | Daily logrotate (cp + truncate, keep 7 archives) |

---

## Initial deploy log (for reference)

First successful deploy: **2026-04-28**. Smoke-test results from the first
60s after `start.sh`:

```
T+30s : consumed=84702   rate=2823/s   ok=84696  err=0  tcp_dropped=0
T+60s : consumed=801328  rate=13355/s  ok=800862 err=0  tcp_dropped=0
```

Two harmless `iqplus parse error: missing IQP header` warnings (unparseable
heartbeat-style lines from IQPlus), zero NATS errors, zero TCP drops.
