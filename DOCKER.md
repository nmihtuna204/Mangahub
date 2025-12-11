# MangaHub Docker Setup

## 🐳 Quick Start với Docker

### Prerequisites
- Docker Desktop installed
- Docker Compose installed

### Build và Run Tất Cả Services

```bash
# Build images
docker-compose build

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

### Run Individual Services

```bash
# Chỉ chạy API server
docker-compose up -d api-server

# Chạy API + TCP + UDP + gRPC
docker-compose up -d
```

## 📦 Services và Ports

| Service | Container Name | Port | Protocol | Description |
|---------|---------------|------|----------|-------------|
| API Server | mangahub-api | 8080 | HTTP/WebSocket | REST API + Chat |
| TCP Server | mangahub-tcp | 9090 | TCP | Progress Sync |
| UDP Server | mangahub-udp | 9091 | UDP | Notifications |
| gRPC Server | mangahub-grpc | 9092 | gRPC | Internal Service |

## 🔧 Configuration

Config file được mount từ `./configs/development.yaml`. Có thể sửa file này để thay đổi cấu hình.

## 📊 Health Checks

API server có health check tự động. Kiểm tra status:

```bash
curl http://localhost:8080/health
```

## 🛠️ Development Commands

```bash
# Rebuild sau khi thay đổi code
docker-compose build
docker-compose up -d

# View logs của service cụ thể
docker-compose logs -f api-server
docker-compose logs -f tcp-server

# Restart service cụ thể
docker-compose restart api-server

# Stop và remove tất cả
docker-compose down -v
```

## 🧪 Testing với Docker

```bash
# Exec vào container
docker exec -it mangahub-api sh

# Run tests trong container
docker exec -it mangahub-api go test ./...

# Check database
docker exec -it mangahub-api ls -la /app/data
```

## 🌐 Access Services

Sau khi start:

- **API Docs**: http://localhost:8080
- **WebSocket Chat**: ws://localhost:8080/ws/chat?room_id=one-piece
- **TCP Client**: Connect to localhost:9090
- **UDP Client**: Send to localhost:9091
- **gRPC Client**: Connect to localhost:9092

## 📋 Production Deployment

### Build for Production

```bash
# Set production config
cp configs/production.yaml.example configs/production.yaml

# Build with production flag
docker-compose -f docker-compose.prod.yml build

# Run in production mode
docker-compose -f docker-compose.prod.yml up -d
```

### Using Standalone Dockerfile

```bash
# Build image
docker build -t mangahub:latest .

# Run API server
docker run -d -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/data:/app/data \
  --name mangahub-api \
  mangahub:latest /app/api-server

# Run TCP server
docker run -d -p 9090:9090 \
  -v $(pwd)/configs:/app/configs \
  --name mangahub-tcp \
  mangahub:latest /app/tcp-server

# Run UDP server
docker run -d -p 9091:9091/udp \
  -v $(pwd)/configs:/app/configs \
  --name mangahub-udp \
  mangahub:latest /app/udp-server

# Run gRPC server
docker run -d -p 9092:9092 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/data:/app/data \
  --name mangahub-grpc \
  mangahub:latest /app/grpc-server
```

## 🔍 Troubleshooting

### Services không start được

```bash
# Check logs
docker-compose logs api-server

# Check container status
docker-compose ps

# Restart all
docker-compose restart
```

### Port conflicts

Nếu port đã được sử dụng, sửa trong `docker-compose.yml`:

```yaml
ports:
  - "8081:8080"  # Map host 8081 to container 8080
```

### Database issues

```bash
# Reset database
docker-compose down -v
rm -rf data/*.db
docker-compose up -d
```

## 📝 Notes

- Database file được persist trong `./data` volume
- Config files được mount read-only
- Containers tự động restart khi crash
- Network isolation giữa containers
- Health checks cho API server

## 🚀 Next Steps

1. Run `docker-compose up -d`
2. Wait for health checks to pass
3. Test với CLI tool hoặc curl
4. Check logs với `docker-compose logs -f`
5. Demo với instructor!
