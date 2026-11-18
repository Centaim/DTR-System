#!/bin/sh
# Runs one encrypted backup a day at BACKUP_HOUR (Asia/Manila).
set -u
last=""
while true; do
	today=$(date +%F)
	hour=$(date +%H)
	if [ "$hour" = "$(printf %02d "${BACKUP_HOUR:-1}")" ] && [ "$today" != "$last" ]; then
		if backup.sh; then last=$today; else echo "backup failed, retrying in 10 minutes" >&2; sleep 540; fi
	fi
	sleep 60
done
