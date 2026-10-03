# One image with every MangaHub server (and the CLI); docker-compose.yml runs
# one container per server from it.
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all servers. The SQLite driver (glebarez/go-sqlite) is pure Go, so no
# C toolchain is needed. (This used golang:1.21 with cgo, which can't build a
# module that requires Go 1.25.)
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -o /app/bin/api-server ./cmd/api-server
RUN go build -o /app/bin/tcp-server ./cmd/tcp-server
RUN go build -o /app/bin/udp-server ./cmd/udp-server
RUN go build -o /app/bin/grpc-server ./cmd/grpc-server
RUN go build -o /app/bin/mangahub-cli ./cmd/cli

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/* /app/
COPY --from=builder /app/configs ./configs

# The database is created and seeded here on first start; docker-compose.yml
# mounts ./data over it so it survives restarts. (Copying data/ from the
# build context failed in a fresh clone, which has no data/ directory.)
RUN mkdir -p /app/data

# Expose ports
# 8080: HTTP API + WebSocket
# 9090: TCP
# 9091: UDP
# 9092: gRPC
EXPOSE 8080 9090 9091/udp 9092

# Default to API server (can override with docker run command)
CMD ["/app/api-server"]
