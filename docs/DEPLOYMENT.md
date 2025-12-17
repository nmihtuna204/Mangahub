# Deployment Guide

## Development Environment

### Setup

```bash
git clone https://github.com/yourusername/mangahub.git
cd mangahub
go mod tidy
cp configs/development.yaml.example configs/development.yaml
```

### Configuration

Edit `configs/development.yaml`:

```yaml
server:
  host: 0.0.0.0
  http_port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s

tcp:
  host: 0.0.0.0
  port: 9090

udp:
  host: 0.0.0.0
  port: 9091

grpc:
  host: 0.0.0.0
  port: 9092

database:
  path: ./data/mangahub.db
  max_open_conns: 25
  max_idle_conns: 5

jwt:
  secret: your-secret-key-change-this
  issuer: mangahub
  expiration: 86400

logging:
  level: info
  format: json
  output: stdout
```

### Running Services

**All-in-One (Development)**
```bash
make run-all
```

**Individual Services**
```bash
# Terminal 1
go run cmd/api-server/main.go

# Terminal 2
go run cmd/tcp-server/main.go

# Terminal 3
go run cmd/udp-server/main.go

# Terminal 4
go run cmd/grpc-server/main.go
```

## Docker Deployment (Optional)

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bin/mangahub ./cmd/api-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/mangahub .
COPY configs/development.yaml .
EXPOSE 8080 9090 9091 9092
CMD ["./mangahub"]
```

Build and run:
```bash
docker build -t mangahub .
docker run -p 8080:8080 -p 9090:9090 -p 9091:9091 -p 9092:9092 mangahub
```

## Production Checklist

- [ ] Change JWT secret
- [ ] Set up proper logging
- [ ] Configure database backups
- [ ] Enable HTTPS/TLS
- [ ] Set up monitoring
- [ ] Configure firewall rules
- [ ] Set environment variables
- [ ] Test failover scenarios
