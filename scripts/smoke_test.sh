#!/usr/bin/env bash
# End-to-end smoke test: builds the proxy, starts a mock shiori.ai upstream,
# and checks health, auth, model listing, and streaming/non-streaming chat.
#
# Usage:  ./scripts/smoke_test.sh
# Exit code 0 means every check passed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROXY_PORT="${PROXY_PORT:-18081}"
MOCK_PORT="${MOCK_PORT:-19192}"
WORK="$(mktemp -d)"
trap 'kill "${PROXY_PID:-}" "${MOCK_PID:-}" 2>/dev/null || true; rm -rf "$WORK"' EXIT

echo "==> building proxy"
go build -o "$WORK/shioriai2api" .

echo "==> building mock upstream"
go build -o "$WORK/mockshiori" ./internal/mockshiori

echo "==> starting mock upstream on :$MOCK_PORT"
"$WORK/mockshiori" -addr "127.0.0.1:$MOCK_PORT" >"$WORK/mock.log" 2>&1 &
MOCK_PID=$!
sleep 0.5

echo "==> starting proxy on :$PROXY_PORT"
SHIORI_REFRESH_TOKEN="test-refresh-token" \
PROXY_API_KEY="test-key" \
SHIORI_BASE_URL="http://127.0.0.1:$MOCK_PORT" \
PORT="$PROXY_PORT" \
  "$WORK/shioriai2api" >"$WORK/proxy.log" 2>&1 &
PROXY_PID=$!
sleep 1

fail() { echo "FAIL: $1" >&2; cat "$WORK/proxy.log" >&2; exit 1; }

health="$(curl -fsS "http://127.0.0.1:$PROXY_PORT/health")"
[ "$health" = '{"status":"ok"}' ] || fail "health returned $health"
echo "  health ok"

code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PROXY_PORT/v1/models")"
[ "$code" = "401" ] || fail "unauthenticated /v1/models returned $code, want 401"
echo "  auth enforced (401)"

models="$(curl -fsS -H 'Authorization: Bearer test-key' "http://127.0.0.1:$PROXY_PORT/v1/models")"
echo "$models" | grep -q glm-5.3-flash || fail "models list missing glm-5.3-flash"
echo "  models listed (live catalog)"

completion="$(curl -fsS -H 'Authorization: Bearer test-key' -H 'Content-Type: application/json' \
  -d '{"model":"glm-5.3-flash","messages":[{"role":"user","content":"hi"}]}' \
  "http://127.0.0.1:$PROXY_PORT/v1/chat/completions")"
echo "$completion" | grep -q 'Hello from mock!' || fail "non-streaming completion: $completion"
echo "$completion" | grep -q '"total_tokens":16' || fail "usage not reported: $completion"
echo "  non-streaming completion ok (usage reported)"

stream="$(curl -fsS -N -H 'Authorization: Bearer test-key' -H 'Content-Type: application/json' \
  -d '{"model":"glm-5.3-flash","messages":[{"role":"user","content":"hi"}],"stream":true}' \
  "http://127.0.0.1:$PROXY_PORT/v1/chat/completions")"
echo "$stream" | grep -q 'data: \[DONE\]' || fail "streaming completion missing [DONE]"
echo "$stream" | grep -q 'Hello ' || fail "streaming completion missing text delta"
echo "  streaming completion ok"

echo "ALL CHECKS PASSED"
