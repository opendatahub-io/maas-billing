#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/create-sa-token.sh"

###############################################################################
# MaaS rate-limit load-tester
#
# Examples
#   ./test-request-limits.sh
#     → uses default simulator: http://simulator.maas.local:8000
#
#   ./test-request-limits.sh --host qwen3.maas.local --model qwen3-0-6b-instruct
#     → test qwen3 model: http://qwen3.maas.local:8000
#
#   ./test-request-limits.sh -H granite.maas.local -m granite-8b-code-instruct-128k
#     → test granite model when available
###############################################################################

########################
# Defaults & CLI opts
########################a
API_HOST="simulator.maas.local"          # default hostname (domain routing)
MODEL_ID="simulator-model"               # default model for simulator
# MODEL_ID="qwen3-0-6b-instruct"         # alt: qwen3 model

usage() {
  cat <<EOF
Usage: $(basename "$0") [--host <hostname>] [-m|--model <model-id>] [--help]

Options
  -H, --host   <hostname>  API host (default: $API_HOST)
  -m, --model  <id>        Override model ID (default: $MODEL_ID)
  -h, --help               Show this help and exit
EOF
}

# ─────────────── argument parsing ────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -H|--host)   API_HOST="$2"; shift 2 ;;
    -m|--model)  MODEL_ID="$2"; shift 2 ;;
    -h|--help)   usage; exit 0 ;;
    *) echo "❌ Unknown option: $1"; usage; exit 1 ;;
  esac
done

BASE_URL="http://${API_HOST}:8000/v1/chat/completions"

echo "📡  Host    : $API_HOST"
echo "🤖  Model ID: $MODEL_ID"
echo

########################
# Helper
########################
call_api () {
  local key="$1" prompt="$2" max_toks="$3"
  curl -s -o /dev/null -w "%{http_code}\n" \
       -X POST "$BASE_URL" \
       -H "Authorization: Bearer ${key}" \
       -H "Content-Type: application/json" \
       -d "{\"model\":\"${MODEL_ID}\",\"messages\":[{\"role\":\"user\",\"content\":\"${prompt}\"}],\"max_tokens\":${max_toks}}"
}

###############################################################################
# Usage tests - Rate limits: Free=5/2min, Premium=20/2min
###############################################################################
echo "=== Free User (5 requests per 2min) ==="
for i in {1..7}; do
  printf "Free req #%-2s -> " "$i"
  call_api "$(get_access_token "freeuser1")" "Free user request $i" 10
done

echo
echo "=== Premium User 1 (20 requests per 2min) ==="
for i in {1..22}; do
  printf "Premium1 req #%-2s -> " "$i"
  call_api "$(get_access_token "premiumuser1")" "Premium user 1 request $i" 15
done

echo
echo "=== Premium User 2 (20 requests per 2min) ==="
for i in {1..22}; do
  printf "Premium2 req #%-2s -> " "$i"
  call_api "$(get_access_token "premiumuser2")" "Premium user 2 request $i" 15
done

echo
echo "=== Second Free User (5 requests per 2min) ==="
for i in {1..7}; do
  printf "Free2 req #%-2s -> " "$i"
  call_api "$(get_access_token "freeuser2")" "Second free user request $i" 10
done

echo
echo "=== Enterprise user (50 requests per 2min) ==="
for i in {1..52}; do
  printf "Enterprise req #%-2s -> " "$i"
  call_api "$(get_access_token "enterpriseuser1")" "Enterprise user request $i" 10
done
