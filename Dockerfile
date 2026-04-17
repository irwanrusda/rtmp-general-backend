# Build Go Stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/rtmp_server main.go

# Run Stage
FROM tiangolo/nginx-rtmp
RUN apt-get update && apt-get install -y \
    default-mysql-client \
    ffmpeg \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/rtmp_server /usr/local/bin/rtmp_server
RUN chmod +x /usr/local/bin/rtmp_server

COPY nginx.conf /etc/nginx/nginx.conf
COPY migrate.sql /www/migrate.sql
COPY start.sh /start.sh
COPY transcode.sh /usr/local/bin/transcode.sh
RUN chmod +x /start.sh /usr/local/bin/transcode.sh

EXPOSE 1935 80 8080

CMD ["/start.sh"]
