#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/"

exec xcaddy build --with github.com/ericls/certmatic=.