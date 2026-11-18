#!/bin/sh
# Replaces ALL current data with one backup.
# Usage: ./restore.sh backups/dtr-YYYYMMDD-HHMMSS.tar.gpg [--yes]
set -eu
cd "$(dirname "$0")"

file=${1:-}
[ -n "$file" ] && [ -f "$file" ] || { echo "Usage: ./restore.sh backups/<file>.tar.gpg [--yes]" >&2; exit 1; }
if [ "${2:-}" != "--yes" ]; then
	printf 'This replaces ALL current data with %s. Type RESTORE to continue: ' "$(basename "$file")"
	read -r answer
	[ "$answer" = RESTORE ] || { echo "Cancelled."; exit 1; }
fi

c="docker compose -f compose.prod.yml"
$c stop web api backup
status=0
$c run --rm --build restore "$(basename "$file")" || status=$?
$c up -d --wait
[ "$status" -eq 0 ] && echo "Restore finished." || { echo "Restore FAILED. Check the output above." >&2; exit "$status"; }
