#!/bin/sh
# Writes one encrypted archive holding the database and every photo.
set -eu
umask 077
work=$(mktemp -d)
record() { # keeps a line in the database so HR can see whether backups are working
	psql -h db -U dtr -d dtr -q -c "INSERT INTO backup_runs (file, bytes, ok) VALUES ('$1', $2, $3)" >/dev/null 2>&1 || true
}
finish() {
	status=$?
	rm -rf "$work"
	[ "$status" -eq 0 ] || record "" 0 false
}
trap finish EXIT

stamp=$(date +%Y%m%d-%H%M%S)
out="/backups/dtr-$stamp.tar.gpg"

pg_dump -h db -U dtr -d dtr -Fc -f "$work/db.dump"
tar -C /data/photos -cf "$work/photos.tar" .
printf %s "$BACKUP_PASSPHRASE" > "$work/key"

tar -C "$work" -cf - db.dump photos.tar |
	gpg --batch --yes --quiet --pinentry-mode loopback --passphrase-file "$work/key" \
		--symmetric --cipher-algo AES256 -o "$out.part"
# Encrypted, inside a private folder: readable so the operator can copy it off the server.
chmod 644 "$out.part"
mv "$out.part" "$out"

find /backups -name 'dtr-*.tar.gpg' -mtime "+${BACKUP_KEEP_DAYS:-30}" -delete
record "$(basename "$out")" "$(wc -c < "$out")" true
echo "backup written: $(basename "$out") ($(du -h "$out" | cut -f1))"
