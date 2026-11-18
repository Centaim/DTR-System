# Production install

Needs one Linux server with Docker and openssl. For a public domain, ports 80 and 443 must be reachable and the domain must point at the server.

```sh
cd deploy

# Public domain, with a free trusted certificate
./install.sh dtr.agency.gov.ph it-admin@agency.gov.ph

# Private network, with a certificate issued by this server
./install.sh 192.168.1.50 internal
```

The installer generates the database password, signing key and backup passphrase into `deploy/.env` (readable only by its owner), builds the system, and starts it. Then create the first administrator:

```sh
docker compose -f compose.prod.yml exec api /app/api create-admin -no ADMIN-1 -name "Full Name"
```

The temporary PIN is printed once, and must be changed at first sign-in.

**Copy `BACKUP_PASSPHRASE` and `PHOTO_KEY` from `deploy/.env` to a safe place away from this server.** Backups cannot be restored without the passphrase, and restored photos cannot be opened without the photo key.

With `internal`, each device must trust the server's certificate, or the browser will warn and may block the camera. Export it with:

```sh
docker compose -f compose.prod.yml cp web:/data/caddy/pki/authorities/local/root.crt ./dtr-root.crt
```

## Updating

Get the new code, then run `./install.sh <same-address>` again. Data and secrets are kept, and database changes apply automatically. Take a backup first.

## Backups

One encrypted archive (AES-256) of the database and all photos is written to `deploy/backups/` every day at 1:00 AM Manila time, and kept for 30 days. Change this with `BACKUP_HOUR` and `BACKUP_KEEP_DAYS` in `deploy/.env`.

```sh
# Back up now
docker compose -f compose.prod.yml exec backup backup.sh

# Replace ALL current data with a backup (asks for confirmation)
./restore.sh backups/dtr-YYYYMMDD-HHMMSS.tar.gpg
```

Copy the archives to another machine regularly. A backup on the same disk does not survive a disk failure.

## Security notes

- Only ports 80 and 443 are open. The database and API are not reachable from outside.
- Photos are encrypted with AES-256-GCM before they are written to disk.
- Treat `deploy/.env` and kiosk tokens like passwords.
