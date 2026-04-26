# Build Go Stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download || true

# Cache-bust: GIT_SHA changes every commit, forcing rebuild from here
ARG GIT_SHA=unknown
RUN echo "Cache bust: $GIT_SHA"

COPY . .
RUN echo "Building commit: $GIT_SHA" && CGO_ENABLED=0 GOOS=linux go build -o /app/rtmp_server main.go

# Run Stage
FROM tiangolo/nginx-rtmp
RUN apt-get update && apt-get install -y \
    default-mysql-client \
    ffmpeg \
    gettext-base \
    procps \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/rtmp_server /usr/local/bin/rtmp_server
RUN chmod +x /usr/local/bin/rtmp_server

# Cache-bust for config/scripts too
ARG GIT_SHA=unknown
RUN echo "Deploy: $GIT_SHA"

COPY nginx.conf /etc/nginx/nginx.conf
COPY migrate.sql /www/migrate.sql
COPY start.sh /start.sh
COPY transcode.sh /usr/local/bin/transcode.sh
RUN chmod +x /start.sh /usr/local/bin/transcode.sh

EXPOSE 1935 80 8080 8888

CMD ["/start.sh"]
