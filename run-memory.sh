#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

export APP_STORE=memory
export APP_ADDR="${APP_ADDR:-:8080}"
export SEED_DATA="${SEED_DATA:-true}"
export SCHEDULER_ENABLED="${SCHEDULER_ENABLED:-true}"

go run .
