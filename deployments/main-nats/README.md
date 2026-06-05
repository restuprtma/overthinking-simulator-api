# Main NATS — Edge Source Migration

Konfigurasi & skrip untuk mengubah main NATS cluster (`10.10.8.2`) supaya
streams IDX_* nge-pull data dari edge nats-server (`10.10.8.1`) via leafnode +
stream-source.

> Lihat dulu: [docs/infra/iqplus-edge-topology.md](../../docs/infra/iqplus-edge-topology.md)
> untuk arsitektur lengkap & migration plan.

## File di folder ini

| File | Tujuan |
|---|---|
| `leafnode-server-additions.conf` | Snippet tambahan untuk `nats-server.conf` di main — mengaktifkan leafnode listener |
| `streams-with-edge-source.sh` | Recreate IDX_TICK/QUOTE/META/NEWS dengan `source` dari domain `edge` |

## Step ringkas

1. Edge sudah up, streams sudah dibuat (lihat `deployments/freebsd/nats-edge/README.md`).
2. Generate leaf token: `openssl rand -hex 32`. Simpan di password manager.
3. Tambahkan blok `leafnodes { ... }` ke `nats-server.conf` main, set `domain: hub` di JetStream section. Reload/restart main NATS.
4. Tambahkan `leafnodes.remotes` block ke edge `nats-server.conf`, set credentials = leaf token. Restart edge.
5. Verify leaf up:
   ```bash
   # Di main
   nats --context tuai-jetstream req '$JS.edge.API.INFO' '' --timeout 3s
   # Harus dapet response (bukan no-responder).
   ```
6. Jalankan `streams-with-edge-source.sh`:
   ```bash
   export MAIN_NATS_CONTEXT=tuai-jetstream
   sh deployments/main-nats/streams-with-edge-source.sh
   ```
7. Verify per-stream source:
   ```bash
   nats stream info IDX_TICK | grep -A3 Sources
   ```

## Rollback

Kalau ada masalah dan harus kembali ke arsitektur lama (publisher → main langsung):

1. Drop sourced streams:
   ```bash
   for s in IDX_TICK IDX_QUOTE IDX_META IDX_NEWS; do
     nats stream rm "$s" --force
   done
   ```
2. Recreate dengan config lama (tanpa `--source`):
   ```bash
   sh docs/JetStream/streams.md   # ada CLI commands lengkap di markdown
   ```
3. Update `cmd/iqplus-publisher/.env` (di FreeBSD VM) ubah `NATS_URL` balik ke `nats://10.10.8.2:4222`. Restart publisher.

> Edge nats-server bisa dibiarkan jalan sebagai dormant — tidak akan menerima publish kalau publisher tidak point ke sana. Stop kalau memang sudah pasti tidak butuh.
