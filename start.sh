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

# Start FFmpeg Watchdog (kills orphan FFmpeg processes every 60s)
(
    sleep 30  # Wait for services to be ready
    echo "[Watchdog] FFmpeg zombie watchdog started"
    while true; do
        for pid in $(pgrep -f "ffmpeg.*rtmp://127.0.0.1/live/" 2>/dev/null); do
            # Extract stream key from FFmpeg command line
            STREAM_KEY=$(ps -p $pid -o args= 2>/dev/null | sed -n 's/.*live\/\([^ ]*\).*/\1/p')
            if [ -n "$STREAM_KEY" ]; then
                # Check if this stream is still in the active_streams table
                COUNT=$(mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -D "$DB_NAME" -N -s -e \
                    "SELECT COUNT(*) FROM active_streams WHERE stream_key='$STREAM_KEY'" 2>/dev/null)
                if [ "$COUNT" = "0" ]; then
                    echo "[Watchdog] Killing orphan FFmpeg PID $pid for stream '$STREAM_KEY'"
                    kill $pid 2>/dev/null
                    sleep 0.5
                    kill -9 $pid 2>/dev/null
                    rm -f /tmp/hls/${STREAM_KEY}.m3u8 /tmp/hls/${STREAM_KEY}-*.ts /tmp/hls/${STREAM_KEY}.pid
                fi
            fi
        done

        # Also detect duplicate FFmpeg for the same stream key (keep only newest)
        for SKEY in $(pgrep -a -f "ffmpeg.*rtmp://127.0.0.1/live/" 2>/dev/null | sed -n 's/.*live\/\([^ ]*\).*/\1/p' | sort -u); do
            PIDS=$(pgrep -f "ffmpeg.*live/${SKEY}" 2>/dev/null | sort -n)
            PID_COUNT=$(echo "$PIDS" | wc -l)
            if [ "$PID_COUNT" -gt 1 ]; then
                # Kill all except the newest (last PID)
                NEWEST=$(echo "$PIDS" | tail -1)
                for OLD_PID in $(echo "$PIDS" | head -n -1); do
                    echo "[Watchdog] Killing duplicate FFmpeg PID $OLD_PID for '$SKEY' (keeping $NEWEST)"
                    kill $OLD_PID 2>/dev/null
                    sleep 0.3
                    kill -9 $OLD_PID 2>/dev/null
                done
            fi
        done

        sleep 60
    done
) &
echo "  ✓ FFmpeg Watchdog started"

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
