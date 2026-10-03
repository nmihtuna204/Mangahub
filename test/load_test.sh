#!/usr/bin/env bash
# Load test against the four running servers. Needs bash and curl (grpcurl
# optional); it used to need ab and nc, and printed "complete" even when they
# were missing or every request failed. Exits 1 if any request fails.
#
# The API allows 50 requests/s per client IP with bursts of 100, and this
# test stays inside that. For heavier runs set server.rate_limit: 0 first.
#
#   bash test/load_test.sh            # or: make load-test
#   MANGAHUB_HOST=10.0.0.5 bash test/load_test.sh
set -u

HOST="${MANGAHUB_HOST:-127.0.0.1}"
HTTP="http://$HOST:${HTTP_PORT:-8080}"
TCP_PORT="${TCP_PORT:-9090}"
UDP_PORT="${UDP_PORT:-9091}"
GRPC_ADDR="$HOST:${GRPC_PORT:-9092}"
FAILED=0
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass() { echo "[PASS] $*"; }
fail() { echo "[FAIL] $*"; FAILED=$((FAILED + 1)); }
now_ms() { date +%s%3N; }

# p50/p95/max of the numbers (seconds) in a file, printed in ms
latency() {
  sort -n "$1" | awk '{ v[NR] = $1 * 1000 } END {
    if (NR == 0) { print "no samples"; exit }
    printf "p50 %.1f ms, p95 %.1f ms, max %.1f ms", v[int((NR + 1) * 0.50)], v[int((NR + 1) * 0.95)], v[NR]
  }'
}

echo "=== MangaHub Load Testing ($HOST) ==="
echo ""
if ! curl -sf "$HTTP/health" > /dev/null; then
  echo "HTTP server not running at $HTTP"
  exit 1
fi

# Test 1: HTTP
echo "Test 1: 100 HTTP GET /manga?limit=20 requests, 10 at a time..."
start=$(now_ms)
for i in $(seq 1 100); do
  curl -s -o /dev/null -w "%{http_code} %{time_total}\n" "$HTTP/manga?limit=20" > "$TMP/http.$i" &
  if (( i % 10 == 0 )); then wait; fi
done
wait
elapsed=$(( $(now_ms) - start ))
cat "$TMP"/http.* > "$TMP/http"
ok=$(grep -c '^200 ' "$TMP/http")
limited=$(grep -c '^429 ' "$TMP/http")
awk '{ print $2 }' "$TMP/http" > "$TMP/http.times"
echo "  $ok/100 OK in ${elapsed} ms ($(( 100 * 1000 / (elapsed > 0 ? elapsed : 1) )) req/s); $(latency "$TMP/http.times")"
if (( ok == 100 )); then
  pass "HTTP: all 100 requests succeeded"
else
  fail "HTTP: $((100 - ok)) of 100 requests failed ($limited were rate limited: 429)"
fi
echo ""

# Test 2: TCP. The sync server relays every line to all clients (the sender
# included), so each client waits for its own update to come back.
echo "Test 2: 10 concurrent TCP sync clients..."
tcp_client() {
  local id="load-$1-$$" line got
  exec 3<> "/dev/tcp/$HOST/$TCP_PORT" || return 1
  line="{\"user_id\":\"$id\",\"manga_id\":\"load-test\",\"chapter\":$1,\"timestamp\":$(date +%s)}"
  printf '%s\n' "$line" >&3
  while IFS= read -r -t 5 got <&3; do
    if [[ "$got" == *"\"$id\""* ]]; then
      exec 3>&-
      return 0
    fi
  done
  exec 3>&-
  return 1
}
for i in $(seq 1 10); do
  ( tcp_client "$i" && touch "$TMP/tcp.ok.$i" ) 2> /dev/null &
done
wait
ok=$(ls "$TMP" | grep -c '^tcp\.ok\.')
if (( ok == 10 )); then
  pass "TCP: 10/10 clients connected and got their update relayed back"
else
  fail "TCP: only $ok/10 clients got their update relayed back"
fi
echo ""

# Test 3: gRPC
echo "Test 3: 20 concurrent gRPC SearchManga calls..."
GRPCURL="$(command -v grpcurl || true)"
[[ -z "$GRPCURL" && -x "$HOME/go/bin/grpcurl.exe" ]] && GRPCURL="$HOME/go/bin/grpcurl.exe"
[[ -z "$GRPCURL" && -x "$HOME/go/bin/grpcurl" ]] && GRPCURL="$HOME/go/bin/grpcurl"
if [[ -z "$GRPCURL" ]]; then
  echo "[SKIP] grpcurl not installed (go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)"
else
  start=$(now_ms)
  for i in $(seq 1 20); do
    ( "$GRPCURL" -plaintext -d '{"query":"one","limit":5}' "$GRPC_ADDR" mangahub.v1.MangaService/SearchManga 2> /dev/null \
        | grep -q '"manga"' && touch "$TMP/grpc.ok.$i" ) &
  done
  wait
  elapsed=$(( $(now_ms) - start ))
  ok=$(ls "$TMP" | grep -c '^grpc\.ok\.')
  if (( ok == 20 )); then
    pass "gRPC: 20/20 calls returned results (${elapsed} ms for all)"
  else
    fail "gRPC: only $ok/20 calls returned results"
  fi
fi
echo ""

# Test 4: UDP. bash's /dev/udp gives a connected socket; dd reads exactly one
# datagram (the reply).
echo "Test 4: 50 concurrent UDP subscribers (REGISTER, then UNREGISTER)..."
udp_client() {
  local reply
  exec 3<> "/dev/udp/$HOST/$UDP_PORT" || return 1
  printf 'REGISTER' >&3
  reply="$(timeout 3 dd bs=128 count=1 status=none <&3)"
  printf 'UNREGISTER' >&3
  exec 3>&-
  [[ "$reply" == "REGISTERED" ]]
}
for i in $(seq 1 50); do
  ( udp_client && touch "$TMP/udp.ok.$i" ) 2> /dev/null &
done
wait
ok=$(ls "$TMP" | grep -c '^udp\.ok\.')
if (( ok == 50 )); then
  pass "UDP: 50/50 subscribers got REGISTERED"
else
  fail "UDP: only $ok/50 subscribers got REGISTERED"
fi
echo ""

echo "=== Load Testing Complete ==="
if (( FAILED > 0 )); then
  echo "$FAILED test(s) FAILED"
  exit 1
fi
echo "All load tests passed"
