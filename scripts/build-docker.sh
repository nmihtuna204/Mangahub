#!/usr/bin/env bash
# Builds the MangaHub Docker image used by docker-compose.yml (all four
# servers and the CLI). Run from the repository root:
#   bash scripts/build-docker.sh && docker compose up -d
# (This file used to be an unfinished stub that couldn't run.)
set -euo pipefail
cd "$(dirname "$0")/.."

echo "================================"
echo "Building MangaHub Docker Images"
echo "================================"
docker compose build
echo ""
echo "Done. Start everything with: docker compose up -d"
echo "Then check: curl http://localhost:8080/health"
