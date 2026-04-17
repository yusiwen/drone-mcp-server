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

echo "=== Testing Drone MCP Server ==="
echo "DRONE_SERVER: $DRONE_SERVER"

# Test help
echo -e "\n=== Testing help ==="
./drone-mcp-server --help

# Try to run in stdio mode (will wait for stdin)
echo -e "\n=== Testing stdio mode (will timeout after 2s) ==="
timeout 2 ./drone-mcp-server || echo "Timeout as expected - waiting for MCP client connection"

# Test SSE mode startup
echo -e "\n=== Testing SSE mode startup ==="
./drone-mcp-server --sse --host localhost --port 18080 &
SERVER_PID=$!
sleep 2
echo "Server PID: $SERVER_PID"

# Test if server is responding
echo -e "\n=== Testing HTTP endpoint ==="
curl -s -I http://localhost:18080/ || echo "Server not responding or not started"

kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
echo "Test complete"