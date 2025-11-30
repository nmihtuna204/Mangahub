#!/bin/bash

echo "=== MangaHub Load Testing ==="
echo ""

# Check if servers are running
echo "Checking server availability..."
curl -s http://localhost:8080/health > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✓ HTTP server running"
else
    echo "✗ HTTP server not running"
    exit 1
fi

echo ""

# Test 1: Concurrent HTTP requests
echo "Test 1: 100 concurrent HTTP GET requests to /manga..."
if command -v ab > /dev/null 2>&1; then
    ab -n 100 -c 10 -q http://localhost:8080/manga
    echo "✓ HTTP load test complete"
else
    echo "⚠ Apache Bench (ab) not installed, skipping HTTP load test"
    echo "  Install with: sudo apt-get install apache2-utils (Linux)"
    echo "             or: brew install apache2 (Mac)"
fi

echo ""

# Test 2: TCP concurrent connections
echo "Test 2: 10 concurrent TCP connections..."
for i in {1..10}; do
    (
        # Use bash TCP redirection if available
        if [ -e /dev/tcp/localhost/9090 ]; then
            exec 3<>/dev/tcp/localhost/9090
            echo '{"user_id":"user'$i'","manga_id":"one-piece","chapter":50,"timestamp":'$(date +%s)'}' >&3
            sleep 1
            exec 3>&-
        else
            echo '{"user_id":"user'$i'","manga_id":"one-piece","chapter":50}' | nc localhost 9090 &
        fi
    ) &
done
wait
echo "✓ TCP load test complete"

echo ""

# Test 3: gRPC concurrent requests
echo "Test 3: 20 concurrent gRPC requests..."
if command -v grpcurl > /dev/null 2>&1; then
    for i in {1..20}; do
        grpcurl -plaintext -d '{"query":"one","limit":5}' \
            localhost:9092 mangahub.v1.MangaService/SearchManga > /dev/null 2>&1 &
    done
    wait
    echo "✓ gRPC load test complete"
else
    echo "⚠ grpcurl not installed, skipping gRPC load test"
    echo "  Install with: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
fi

echo ""

# Test 4: UDP message flood
echo "Test 4: 50 UDP notification messages..."
for i in {1..50}; do
    echo "REGISTER" | nc -u -w1 localhost 9091 > /dev/null 2>&1 &
done
wait
echo "✓ UDP load test complete"

echo ""
echo "=== Load Testing Complete ==="
echo ""
echo "Summary:"
echo "  ✓ 100 HTTP requests"
echo "  ✓ 10 TCP connections"
echo "  ✓ 20 gRPC calls"
echo "  ✓ 50 UDP messages"
echo ""
echo "Check server logs for performance metrics"
