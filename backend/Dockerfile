# --- Stage 1: build ---
# Pakai Alpine-based image untuk kompatibilitas musl (sqlite3 cgo).
FROM golang:1.25-alpine AS builder

# build deps untuk go-sqlite3 (cgo)
RUN apk add --no-cache build-base

WORKDIR /src

# Cache deps
COPY go.mod go.sum* ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# --- Stage 2: runtime ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/server /app/server

# Data directory untuk SQLite audit log (mount volume di production)
RUN mkdir -p /app/data && chown -R app:app /app

USER app
EXPOSE 8080

# Healthcheck pakai endpoint /health
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/server"]
