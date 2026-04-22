#!/bin/bash

# transcode.sh - Dynamic RTMP Copy/Reject Wrapper
# Usage: ./transcode.sh <stream_key>
# 
# Flow:
# 1. ffprobe detects incoming stream resolution
# 2. Query DB for user's allowed_quality_id
# 3. If resolution exceeds limit → REJECT + log to stream_logs
# 4. If resolution is within limit → copy mode (-c copy, 0% CPU)

STREAM_KEY=$1
RTMP_INPUT="rtmp://127.0.0.1/live/$STREAM_KEY"
RTMP_OUTPUT_BASE="rtmp://127.0.0.1/hls_out/$STREAM_KEY"

# Wait briefly for stream to become available
sleep 0.5

# Hapus cache m3u8 lama secara otomatis agar Ghost Stream tidak terjadi
echo "[Transcoder] Membersihkan cache HLS sisa untuk $STREAM_KEY..."
rm -f /tmp/hls/${STREAM_KEY}.m3u8 /tmp/hls/${STREAM_KEY}-*.ts

# 1. Detect incoming stream resolution using ffprobe (Retry up to 5 times)
RESOLUTION=""
for i in {1..5}; do
    echo "[Transcoder] Probing stream resolution for $STREAM_KEY (Attempt $i)..."
    RESOLUTION=$(ffprobe -v error -select_streams v:0 \
        -show_entries stream=height \
        -of default=noprint_wrappers=1:nokey=1 \
        -analyzeduration 1500000 -probesize 100000 \
        "$RTMP_INPUT" 2>/dev/null)
        
    if [ -n "$RESOLUTION" ] && [ "$RESOLUTION" -gt 0 ] 2>/dev/null; then
        break
    fi
    sleep 1
done

if [ -z "$RESOLUTION" ] || [ "$RESOLUTION" -eq 0 ] 2>/dev/null; then
    echo "[Transcoder] WARNING: Could not detect resolution after 5 attempts, defaulting to 0"
    RESOLUTION=0
fi

echo "[Transcoder] Detected height: ${RESOLUTION}p"

# 2. Look up allowed quality for this stream key, prioritizing stream-specific override
# IFNULL allows us to fall back to user's allowed_quality_id if override_quality_id is NULL
RESULT=$(mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -D "$DB_NAME" -N -s -e \
    "SELECT IFNULL(sk.override_quality_id, u.allowed_quality_id), u.id FROM stream_keys sk JOIN users u ON sk.user_id = u.id WHERE sk.stream_key='$STREAM_KEY'")

QUALITY_ID=$(echo "$RESULT" | awk '{print $1}')
USER_ID=$(echo "$RESULT" | awk '{print $2}')

# Fallback to 480p if not found
if [ -z "$QUALITY_ID" ]; then
    QUALITY_ID=2
fi

# 3. Map quality_id to max allowed height
case $QUALITY_ID in
    1) MAX_HEIGHT=360 ;;
    3) MAX_HEIGHT=720 ;;
    4) MAX_HEIGHT=1080 ;;
    *) MAX_HEIGHT=480 ;;
esac

echo "[Transcoder] User ID: $USER_ID | Quality limit: ${MAX_HEIGHT}p | Stream: ${RESOLUTION}p"

# 4. Compare: reject if stream resolution exceeds allowed limit
if [ "$RESOLUTION" -gt "$MAX_HEIGHT" ] 2>/dev/null; then
    echo "[Transcoder] REJECTED: Stream ${RESOLUTION}p exceeds limit ${MAX_HEIGHT}p for $STREAM_KEY"
    
    # Log rejection to database
    if [ -n "$USER_ID" ]; then
        mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -D "$DB_NAME" -e \
            "INSERT INTO stream_logs (user_id, stream_key, event_type, message) VALUES ($USER_ID, '$STREAM_KEY', 'rejected', 'Stream resolution ${RESOLUTION}p exceeds limit ${MAX_HEIGHT}p. Please change Output Resolution in OBS.')"
    fi
    
    # Kill the stream - exit non-zero so nginx stops the exec
    curl -s "http://127.0.0.1/control/drop/publisher?app=live&name=$STREAM_KEY"
    exit 1
fi

# 5. Log successful connection
if [ -n "$USER_ID" ]; then
    mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -D "$DB_NAME" -e \
        "INSERT INTO stream_logs (user_id, stream_key, event_type, message) VALUES ($USER_ID, '$STREAM_KEY', 'connected', 'Stream ${RESOLUTION}p started (copy mode, no transcode)')"
fi

# 6. Execute FFmpeg in COPY mode -> Direct to HLS
echo "[Transcoder] APPROVED: Starting copy-mode relay for $STREAM_KEY (${RESOLUTION}p → ${MAX_HEIGHT}p limit) and Direct HLS Segmenting"

# Pastikan folder tmp/hls ada
mkdir -p /tmp/hls

ffmpeg -fflags nobuffer -flags low_delay -i "$RTMP_INPUT" \
    -c:v copy \
    -bsf:v h264_mp4toannexb \
    -c:a copy \
    -f hls \
    -hls_time 2 \
    -hls_list_size 3 \
    -hls_flags delete_segments+temp_file+independent_segments \
    -hls_segment_filename "/tmp/hls/${STREAM_KEY}-%d.ts" \
    "/tmp/hls/${STREAM_KEY}.m3u8"

# 7. Log disconnection when ffmpeg exits
if [ -n "$USER_ID" ]; then
    mysql -u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -D "$DB_NAME" -e \
        "INSERT INTO stream_logs (user_id, stream_key, event_type, message) VALUES ($USER_ID, '$STREAM_KEY', 'disconnected', 'Stream disconnected')"
fi
