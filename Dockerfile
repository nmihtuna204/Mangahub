# Dockerfile for API Server
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all servers
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/api-server ./cmd/api-server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/tcp-server ./cmd/tcp-server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/udp-server ./cmd/udp-server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/grpc-server ./cmd/grpc-server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/mangahub-cli ./cmd/cli

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/* /app/
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/data ./data

# Create data directory
RUN mkdir -p /app/data

# Expose ports
# 8080: HTTP API + WebSocket
# 9090: TCP
# 9091: UDP  
# 9092: gRPC
EXPOSE 8080 9090 9091 9092

# Default to API server (can override with docker run command)
CMD ["/app/api-server"]
