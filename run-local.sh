#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

export APP_STORE="${APP_STORE:-mysql}"
export APP_ADDR="${APP_ADDR:-:8080}"
export MYSQL_DSN="${MYSQL_DSN:-redops:redops@tcp(127.0.0.1:3306)/redops?parseTime=true&charset=utf8mb4&loc=Local}"
export AUTO_MIGRATE="${AUTO_MIGRATE:-true}"
export SEED_DATA="${SEED_DATA:-true}"
export SCHEDULER_ENABLED="${SCHEDULER_ENABLED:-true}"

go run .
