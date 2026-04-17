#!/bin/bash
set -e

# Handle transcode command from Nginx exec_publish
if [ "$1" == "transcode" ]; then
    shift
    exec /usr/local/bin/transcode.sh "$@"
fi

echo "=========================================="
echo "  Multi RTMP Server - Starting Up (Go)"
echo "=========================================="

# ─── Default environment variables ────────────────────────
export DB_HOST="${DB_HOST:-nanobyte_nanodb}"
export DB_PORT="${DB_PORT:-3306}"
export DB_USER="${DB_USER:-root}"
export DB_PASSWORD="${DB_PASSWORD:-${DB_PASS:-506cef66db0e2af91eac}}"
export DB_NAME="${DB_NAME:-rtmp}"
export RTMP_SECRET="${RTMP_SECRET:-}"
export ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
export ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

# ─── [1/4] Wait for MySQL ────────────────────────────────
echo "[1/4] Waiting for MySQL at ${DB_HOST}:${DB_PORT}..."
for i in $(seq 1 30); do
    if mysqladmin ping -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" --silent 2>/dev/null; then
        echo "  ✓ MySQL is ready!"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "  ⚠ MySQL not ready after 30 attempts, continuing anyway..."
    fi
    echo "  Attempt $i/30..."
    sleep 2
done

# ─── [2/4] Run database migrations ───────────────────────
echo "[2/4] Auto metadata database migrations skipped."
echo "  Run /migrate manually after container starts."

# ─── [3/4] Services are now seeded automatically by the Go binary ───
echo "[3/4] Skipping legacy bash seeding"
echo "  ✓ Seeder ready"

# ─── [4/4] Start services ────────────────────────────────
echo "[4/4] Starting services..."

# Start Golang API Backend in background
/usr/local/bin/rtmp_server &
echo "  ✓ Go API Server started"

# Start Nginx (foreground)
echo "=========================================="
echo "  ✓ Server READY!"
echo "  RTMP  → rtmp://host:1935/live?key=${RTMP_SECRET}"
echo "  HTTP  → http://host/"
echo "  Admin → ${ADMIN_USERNAME} / ${ADMIN_PASSWORD}"
echo "=========================================="

nginx -g "daemon off;"
