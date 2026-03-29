#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/"

exec xcaddy run -- --config Caddyfile.dev
