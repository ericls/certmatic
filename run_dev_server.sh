#!/usr/bin/env bash
set -euo pipefail

# Change to repository root
cd "$(dirname "$0")/"

echo "Building dev certmatic executable..."
go build -o certmatic ./cmd/certmatic

echo "Starting dev certmatic executable with dev config..."
exec ./certmatic run --config Caddyfile.dev --adapter caddyfile