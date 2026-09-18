#!/bin/bash
# Test script for Drone MCP Server
# Usage: DRONE_SERVER=https://your.drone.server DRONE_TOKEN=your_token ./test_env.sh

# Check if environment variables are set
if [ -z "$DRONE_SERVER" ] || [ -z "$DRONE_TOKEN" ]; then
    echo "Error: DRONE_SERVER and DRONE_TOKEN environment variables must be set"
    echo "Usage: DRONE_SERVER=https://your.drone.server DRONE_TOKEN=your_token ./test_env.sh"
    exit 1
fi

export DRONE_SERVER
export DRONE_TOKEN

# HTTP mode requires a bearer token; generate a throwaway one when absent.
if [ -z "$MCP_AUTH_TOKEN" ]; then
    MCP_AUTH_TOKEN=$(head -c 32 /dev/urandom | base64 | tr -d '\n')
    echo "Generated a temporary MCP_AUTH_TOKEN for this smoke test"
fi
export MCP_AUTH_TOKEN

echo "=== Testing Drone MCP Server ==="
echo "DRONE_SERVER: $DRONE_SERVER"

# Test help
echo -e "\n=== Testing help ==="
./drone-mcp-server --help

# Try to run in stdio mode; it exits as soon as stdin reaches EOF
echo -e "\n=== Testing stdio mode ==="
./drone-mcp-server </dev/null && echo "stdio mode exited cleanly"

# Test Streamable HTTP mode startup
echo -e "\n=== Testing Streamable HTTP mode startup ==="
./drone-mcp-server --http --host localhost --port 18080 &
SERVER_PID=$!
sleep 2
echo "Server PID: $SERVER_PID"

# Test the HTTP endpoint. An unauthenticated MCP request must be rejected with
# 401, while an authenticated one creates a session and returns 200.
echo -e "\n=== Testing HTTP endpoint ==="
INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}'

curl -s -o /dev/null -w "unauthenticated request: %{http_code} (expected 401)\n" \
    -X POST -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d "$INIT" http://localhost:18080/ || echo "Server not responding or not started"

curl -s -o /dev/null -w "authenticated request:   %{http_code} (expected 200)\n" \
    -X POST -H "Authorization: Bearer $MCP_AUTH_TOKEN" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d "$INIT" http://localhost:18080/ || echo "Server not responding or not started"

kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
echo "Test complete"