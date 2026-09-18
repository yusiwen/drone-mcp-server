#!/usr/bin/env bash
#
# Security smoke test for drone-mcp-server.
#
# Every check below is decided before the server talks to Drone (authentication,
# Host/Origin validation, request limits, tool registration, input validation),
# so a dummy DRONE_SERVER and DRONE_TOKEN are enough. No real Drone instance is
# contacted and nothing is written anywhere except a temporary directory.
#
# Usage:
#   scripts/security-smoke-test.sh                 # build, then run every check
#   BIN=./drone-mcp-server scripts/security-smoke-test.sh
#   GO=/opt/go/bin/go scripts/security-smoke-test.sh
#   BASE_PORT=19200 scripts/security-smoke-test.sh # when the default ports are busy
#
# Exit status 0 means every check passed. The script is intended for local use
# and for CI; it never mutates a Drone instance.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO="${GO:-go}"
BIN="${BIN:-}"
BASE_PORT="${BASE_PORT:-18200}"
TOKEN="smoke-test-token-0123456789abcdef"
WORK_DIR="$(mktemp -d)"
SRV_PID=""

PASS=0
FAIL=0

INIT_BODY='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke-test","version":"1.0"}}}'
ACCEPT_HEADER='Accept: application/json, text/event-stream'
JSON_HEADER='Content-Type: application/json'

cleanup() {
	stop_server
	rm -rf "$WORK_DIR"
}
trap cleanup EXIT

section() { printf '\n== %s\n' "$1"; }
ok() { PASS=$((PASS + 1)); printf '   PASS  %s\n' "$1"; }
bad() { FAIL=$((FAIL + 1)); printf '   FAIL  %s (expected %s, got %s)\n' "$1" "$2" "$3"; }

check_eq() {
	if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "$2" "$3"; fi
}
# grep -F keeps brackets, quotes and dots in the expected value literal.
check_contains() {
	if printf '%s' "$3" | grep -qF -- "$2"; then ok "$1"; else bad "$1" "to contain '$2'" "$3"; fi
}
check_not_contains() {
	if printf '%s' "$3" | grep -qF -- "$2"; then bad "$1" "not to contain '$2'" "$3"; else ok "$1"; fi
}

build_binary() {
	if [ -n "$BIN" ]; then
		[ -x "$BIN" ] || { printf 'BIN=%s is not executable\n' "$BIN" >&2; exit 2; }
		return
	fi
	BIN="$WORK_DIR/drone-mcp-server"
	printf 'Building the server with %s ...\n' "$GO"
	if ! (cd "$REPO_ROOT" && "$GO" build -o "$BIN" .); then
		printf 'Build failed. Set GO=/path/to/go or BIN=/path/to/binary.\n' >&2
		exit 2
	fi
}

start_server() { # <port> <logfile> -- <server args...>
	local port="$1" logfile="$2"
	shift 2
	[ "$1" = "--" ] && shift

	DRONE_SERVER=https://drone.invalid \
		DRONE_TOKEN=smoke-test-drone-token \
		MCP_AUTH_TOKEN="$TOKEN" \
		"$BIN" "$@" >"$logfile" 2>&1 &
	SRV_PID=$!

	local i=0
	while [ "$i" -lt 60 ]; do
		if ! kill -0 "$SRV_PID" 2>/dev/null; then
			printf 'Server exited early, log follows:\n' >&2
			cat "$logfile" >&2
			return 1
		fi
		if curl -s -o /dev/null -m 1 "http://127.0.0.1:$port/"; then
			return 0
		fi
		i=$((i + 1))
		sleep 0.1
	done
	printf 'Server did not become ready on port %s\n' "$port" >&2
	return 1
}

stop_server() {
	if [ -n "$SRV_PID" ] && kill -0 "$SRV_PID" 2>/dev/null; then
		kill "$SRV_PID" 2>/dev/null
		wait "$SRV_PID" 2>/dev/null
	fi
	SRV_PID=""
}

# http_code <port> <curl args...>
http_code() {
	local port="$1"
	shift
	curl -s -o /dev/null -w '%{http_code}' -m 5 "http://127.0.0.1:$port/" "$@"
}

# mcp_session <port> ; prints the session id, or nothing on failure
mcp_session() {
	local port="$1"
	curl -sS -D "$WORK_DIR/hdr.txt" -o /dev/null -m 5 -X POST \
		-H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" \
		--data "$INIT_BODY" "http://127.0.0.1:$port/"
	grep -i '^mcp-session-id' "$WORK_DIR/hdr.txt" | tr -d '\r' | awk '{print $2}'
}

# mcp_call <port> <session> <json body>
mcp_call() {
	curl -sS -m 5 -X POST -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" \
		-H "$ACCEPT_HEADER" -H "Mcp-Session-Id: $2" --data "$3" "http://127.0.0.1:$1/"
}

build_binary

# ---------------------------------------------------------------------------
section "1. HTTP mode fails closed without a token"
no_token_out=$(DRONE_SERVER=https://drone.invalid DRONE_TOKEN=dummy "$BIN" --http --host 127.0.0.1 --port "$BASE_PORT" 2>&1)
no_token_rc=$?
check_eq "exits non-zero" "1" "$no_token_rc"
check_contains "says why" "refusing to start HTTP mode without MCP_AUTH_TOKEN" "$no_token_out"

# ---------------------------------------------------------------------------
section "2. Authentication, host/origin validation and request limits"
LOG1="$WORK_DIR/server-default.log"
start_server "$BASE_PORT" "$LOG1" -- --http --host 127.0.0.1 --port "$BASE_PORT" || exit 2

check_eq "unauthenticated request is rejected" "401" \
	"$(http_code "$BASE_PORT" -X POST -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_eq "wrong bearer token is rejected" "401" \
	"$(http_code "$BASE_PORT" -X POST -H "Authorization: Bearer wrong-token" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_eq "non-bearer scheme is rejected" "401" \
	"$(http_code "$BASE_PORT" -X POST -H "Authorization: Basic $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_eq "valid bearer token is accepted" "200" \
	"$(http_code "$BASE_PORT" -X POST -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"

check_eq "forged Host header is rejected (DNS rebinding)" "403" \
	"$(http_code "$BASE_PORT" -X POST -H 'Host: attacker.example' -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_eq "cross-site Origin is rejected (CSRF)" "403" \
	"$(http_code "$BASE_PORT" -X POST -H 'Origin: http://evil.example' -H 'Sec-Fetch-Site: cross-site' -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_eq "wrong Content-Type is rejected" "415" \
	"$(http_code "$BASE_PORT" -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: text/plain' -H "$ACCEPT_HEADER" --data "$INIT_BODY")"

oversize_pad() { head -c 1200000 /dev/zero | tr '\0' 'a'; }
{
	printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"pad":"'
	oversize_pad
	printf '"}}'
} >"$WORK_DIR/oversize.json"
check_eq "oversized request body is rejected" "413" \
	"$(http_code "$BASE_PORT" -X POST -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data-binary "@$WORK_DIR/oversize.json")"
check_eq "DELETE without session is rejected" "400" \
	"$(http_code "$BASE_PORT" -X DELETE -H "Authorization: Bearer $TOKEN")"

SESSION="$(mcp_session "$BASE_PORT")"
if [ -n "$SESSION" ]; then ok "initialize returns a session id"; else bad "initialize returns a session id" "a session id" "empty"; fi
mcp_call "$BASE_PORT" "$SESSION" '{"jsonrpc":"2.0","method":"notifications/initialized"}' >/dev/null

stop_server

# ---------------------------------------------------------------------------
section "3. Read-only tool surface by default"
start_server "$BASE_PORT" "$LOG1" -- --http --host 127.0.0.1 --port "$BASE_PORT" || exit 2
SESSION="$(mcp_session "$BASE_PORT")"
mcp_call "$BASE_PORT" "$SESSION" '{"jsonrpc":"2.0","method":"notifications/initialized"}' >/dev/null
TOOLS_READONLY="$(mcp_call "$BASE_PORT" "$SESSION" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')"

readonly_count=$(printf '%s' "$TOOLS_READONLY" | grep -o '"name":"[a-z_]*"' | wc -l | tr -d ' ')
readonly_flags=$(printf '%s' "$TOOLS_READONLY" | grep -o '"readOnlyHint":true' | wc -l | tr -d ' ')
check_eq "registers 18 tools" "18" "$readonly_count"
check_eq "all of them are annotated read-only" "18" "$readonly_flags"
check_not_contains "delete_user is not exposed" '"name":"delete_user"' "$TOOLS_READONLY"

hidden_call="$(mcp_call "$BASE_PORT" "$SESSION" '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"delete_user","arguments":{"login":"someone"}}}')"
check_contains "calling a hidden write tool fails" 'unknown tool \"delete_user\"' "$hidden_call"

traversal_call="$(mcp_call "$BASE_PORT" "$SESSION" '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_repo","arguments":{"owner":"../users","repo":"drone"}}}')"
check_contains "path traversal in tool arguments is rejected" '"isError":true' "$traversal_call"
check_contains "the rejection explains the rule" 'may only contain letters, digits, dots, underscores and hyphens' "$traversal_call"

# Log hygiene: a newline smuggled through the URL path must not forge a log line.
LOG_INJECT_MARKER="1999/01/01"
curl -s -o /dev/null -m 3 -H "Authorization: Bearer $TOKEN" \
	"http://127.0.0.1:$BASE_PORT/%0a${LOG_INJECT_MARKER}%2000:00:00%20[AUTH]%20Token%20validated" || true
# Forwarded headers are ignored unless --trust-proxy is set.
curl -s -o /dev/null -m 3 -H "Authorization: Bearer $TOKEN" -H 'X-Forwarded-For: 203.0.113.9' \
	-X POST -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY" "http://127.0.0.1:$BASE_PORT/" || true
sleep 0.3
LOG_CONTENT="$(cat "$LOG1")"
forged_lines=$(printf '%s\n' "$LOG_CONTENT" | grep -c "^${LOG_INJECT_MARKER}" || true)
check_eq "log injection is neutralised" "0" "$forged_lines"
check_not_contains "X-Forwarded-For is ignored by default" "203.0.113.9" "$LOG_CONTENT"
check_contains "real client address is logged" "(client: 127.0.0.1)" "$LOG_CONTENT"

# Token values must never appear in the log.
check_not_contains "the bearer token is not logged" "$TOKEN" "$LOG_CONTENT"

kill -TERM "$SRV_PID" 2>/dev/null
wait "$SRV_PID" 2>/dev/null
SRV_PID=""
check_contains "SIGTERM shuts the server down gracefully" "HTTP server stopped" "$(cat "$LOG1")"

# ---------------------------------------------------------------------------
section "4. Host allowlist replaces the loopback heuristic"
PORT=$((BASE_PORT + 1))
LOG2="$WORK_DIR/server-allowlist.log"
start_server "$PORT" "$LOG2" -- --http --host 127.0.0.1 --port "$PORT" --allowed-hosts mcp.example.com || exit 2
check_eq "allowlisted Host is accepted" "200" \
	"$(http_code "$PORT" -X POST -H 'Host: mcp.example.com' -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_eq "other Host is rejected by the allowlist" "403" \
	"$(http_code "$PORT" -X POST -H 'Host: evil.example' -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
check_contains "the policy change is logged" "instead of the transport's loopback Host heuristic" "$(cat "$LOG2")"
check_contains "the rejection is audited" "[SECURITY] Rejected request with disallowed Host header" "$(cat "$LOG2")"
stop_server

# ---------------------------------------------------------------------------
section "5. Explicit --localhost-protection=true stacks both checks"
PORT=$((BASE_PORT + 2))
LOG3="$WORK_DIR/server-stacked.log"
start_server "$PORT" "$LOG3" -- --http --host 127.0.0.1 --port "$PORT" --allowed-hosts mcp.example.com --localhost-protection=true || exit 2
check_eq "public Host is rejected again" "403" \
	"$(http_code "$PORT" -X POST -H 'Host: mcp.example.com' -H "Authorization: Bearer $TOKEN" -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY")"
stop_server

# ---------------------------------------------------------------------------
section "6. Disabling the loopback check alone warns"
PORT=$((BASE_PORT + 3))
LOG4="$WORK_DIR/server-noprotection.log"
start_server "$PORT" "$LOG4" -- --http --host 127.0.0.1 --port "$PORT" --localhost-protection=false || exit 2
check_contains "a warning is printed" "no --allowed-hosts allowlist is configured" "$(cat "$LOG4")"
stop_server

# ---------------------------------------------------------------------------
section "7. Write tools require an explicit opt-in"
PORT=$((BASE_PORT + 4))
LOG5="$WORK_DIR/server-write.log"
start_server "$PORT" "$LOG5" -- --http --host 127.0.0.1 --port "$PORT" --enable-write-tools || exit 2
SESSION="$(mcp_session "$PORT")"
mcp_call "$PORT" "$SESSION" '{"jsonrpc":"2.0","method":"notifications/initialized"}' >/dev/null
TOOLS_WRITE="$(mcp_call "$PORT" "$SESSION" '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')"
write_count=$(printf '%s' "$TOOLS_WRITE" | grep -o '"name":"[a-z_]*"' | wc -l | tr -d ' ')
check_eq "registers 45 tools" "45" "$write_count"
check_contains "destructive tools are annotated" '"destructiveHint":true' "$TOOLS_WRITE"
stop_server

# ---------------------------------------------------------------------------
section "8. Forwarding headers are trusted only with --trust-proxy"
PORT=$((BASE_PORT + 5))
LOG6="$WORK_DIR/server-trustproxy.log"
start_server "$PORT" "$LOG6" -- --http --host 127.0.0.1 --port "$PORT" --trust-proxy || exit 2
curl -s -o /dev/null -m 3 -H "Authorization: Bearer $TOKEN" -H 'X-Forwarded-For: 203.0.113.9' \
	-X POST -H "$JSON_HEADER" -H "$ACCEPT_HEADER" --data "$INIT_BODY" "http://127.0.0.1:$PORT/" || true
sleep 0.3
check_contains "the forwarded address is used" "203.0.113.9" "$(cat "$LOG6")"
stop_server

# ---------------------------------------------------------------------------
section "9. Removed --sse flag and stdio mode"
sse_out=$(DRONE_SERVER=https://drone.invalid DRONE_TOKEN=dummy MCP_AUTH_TOKEN="$TOKEN" "$BIN" --sse --host 127.0.0.1 --port "$BASE_PORT" 2>&1)
sse_rc=$?
check_eq "--sse is rejected" "2" "$sse_rc"
check_contains "the parser explains" "flag provided but not defined" "$sse_out"

stdio_out=$(DRONE_SERVER=https://drone.invalid DRONE_TOKEN=dummy "$BIN" </dev/null 2>&1)
stdio_rc=$?
check_eq "stdio mode exits cleanly on EOF" "0" "$stdio_rc"
check_contains "stdio mode announces read-only mode" "write/destructive tools are hidden" "$stdio_out"

# ---------------------------------------------------------------------------
printf '\n%s\n' "----------------------------------------"
printf 'passed: %d   failed: %d\n' "$PASS" "$FAIL"
if [ "$FAIL" -ne 0 ]; then
	printf 'SECURITY SMOKE TEST FAILED\n'
	exit 1
fi
printf 'SECURITY SMOKE TEST PASSED\n'
