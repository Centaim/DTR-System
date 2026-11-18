#!/bin/sh
# Installs or updates the DTR system on one server.
# Usage: ./install.sh <domain-or-ip> [admin-email|internal]
set -eu
cd "$(dirname "$0")"

site=${1:-}
tls=${2:-internal}
if [ -z "$site" ]; then
	echo "Usage: ./install.sh <domain-or-ip> [admin-email|internal]" >&2
	echo "  admin-email  public domain, with a free trusted certificate" >&2
	echo "  internal     private network, with a certificate from this server" >&2
	exit 1
fi
command -v docker >/dev/null || { echo "Docker is required." >&2; exit 1; }
command -v openssl >/dev/null || { echo "openssl is required." >&2; exit 1; }

new=no
if [ ! -f .env ]; then
	new=yes
	umask 077
	cat > .env <<ENV
SITE_ADDRESS=$site
CADDY_TLS=$tls
HTTP_PORT=${HTTP_PORT:-80}
HTTPS_PORT=${HTTPS_PORT:-443}
DB_PASSWORD=$(openssl rand -hex 24)
JWT_SECRET=$(openssl rand -hex 32)
BACKUP_PASSPHRASE=$(openssl rand -hex 32)
BACKUP_HOUR=1
BACKUP_KEEP_DAYS=30
ENV
fi
mkdir -p backups

docker compose -f compose.prod.yml up -d --build --wait

echo
echo "The system is running at https://$site"
if [ "$new" = yes ]; then
	echo
	echo "IMPORTANT: copy BACKUP_PASSPHRASE from deploy/.env to a safe place away from"
	echo "this server. Backups cannot be restored without it."
	echo
	echo "Create the first administrator:"
	echo "  docker compose -f compose.prod.yml exec api /app/api create-admin -no ADMIN-1 -name \"Full Name\""
fi
