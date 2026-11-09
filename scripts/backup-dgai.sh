#!/usr/bin/env bash
# Back up the dgai database and volumes to ./backups, keeping the last 14 days.
#
#   ./scripts/backup-dgai.sh
#
# Install as a daily cron on the server:
#   0 3 * * * cd /opt/dgai && ./scripts/backup-dgai.sh >> /var/log/dgai-backup.log 2>&1
set -euo pipefail

cd "$(dirname "$0")/.."

COMPOSE_FILE=${COMPOSE_FILE:-docker-compose.prod.yml}
ENV_FILE=${ENV_FILE:-.env.dgai}
BACKUP_DIR=${BACKUP_DIR:-backups}
STAMP=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"

docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" exec -T postgres \
  pg_dump -U dgai -d dgai --no-owner --clean \
  | gzip > "$BACKUP_DIR/dgai-$STAMP.sql.gz"

# Logs and uploaded artifacts live in the app volume; they are small at this
# scale and cheap to archive.
docker run --rm \
  -v new-api_dgai_data:/data:ro \
  -v "$(pwd)/$BACKUP_DIR":/backup \
  alpine:3.20 \
  tar czf "/backup/dgai-data-$STAMP.tar.gz" -C /data . 2>/dev/null || true

find "$BACKUP_DIR" -name 'dgai-*.gz' -mtime +14 -delete

echo "backup written: $BACKUP_DIR/dgai-$STAMP.sql.gz"
