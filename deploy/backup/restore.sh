#!/bin/sh
# Replaces the database and photos with the contents of one backup archive.
set -eu
umask 077
file="/backups/$(basename "$1")"
[ -f "$file" ] || { echo "backup not found: $1" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
printf %s "$BACKUP_PASSPHRASE" > "$work/key"

# Decrypt fully first, so a wrong passphrase or damaged file changes nothing.
gpg --batch --yes --quiet --pinentry-mode loopback --passphrase-file "$work/key" -d "$file" | tar -C "$work" -xf -
[ -s "$work/db.dump" ] && [ -f "$work/photos.tar" ] || { echo "archive is incomplete" >&2; exit 1; }

dropdb -h db -U dtr --maintenance-db=postgres --force --if-exists dtr
createdb -h db -U dtr --maintenance-db=postgres dtr
pg_restore -h db -U dtr -d dtr --no-owner --exit-on-error "$work/db.dump"

find /data/photos -mindepth 1 -delete
tar -C /data/photos -xpf "$work/photos.tar"
echo "restored from $(basename "$file")"
