#!/bin/bash
# Simple test of the DAP-MCP server

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER="$SCRIPT_DIR/../bin/dap-mcp"
PYTHON_FILE="$SCRIPT_DIR/python_project/calculator.py"
export PATH="$SCRIPT_DIR/venv/bin:$PATH"

# Function to send MCP request
send_request() {
    local id=$1
    local method=$2
    local params=$3

    local body
    if [ -z "$params" ]; then
        body="{\"jsonrpc\":\"2.0\",\"id\":$id,\"method\":\"$method\"}"
    else
        body="{\"jsonrpc\":\"2.0\",\"id\":$id,\"method\":\"$method\",\"params\":$params}"
    fi

    local length=${#body}
    printf "Content-Length: %d\r\n\r\n%s" "$length" "$body"
}

# Run tests
echo "=== Testing DAP-MCP Server ==="
echo ""

# Create a temporary file for server output
OUTFILE=$(mktemp)

# Start server with input piped
{
    # Initialize
    echo ">>> Sending initialize request..."
    send_request 1 "initialize" '{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}'
    sleep 0.5

    # List tools
    echo ">>> Sending tools/list request..."
    send_request 2 "tools/list"
    sleep 0.5

    # List sessions
    echo ">>> Listing sessions..."
    send_request 3 "tools/call" '{"name":"debug_list_sessions","arguments":{}}'
    sleep 0.5

    # Launch Python debugger
    echo ">>> Launching Python debugger..."
    send_request 4 "tools/call" "{\"name\":\"debug_launch\",\"arguments\":{\"language\":\"python\",\"program\":\"$PYTHON_FILE\",\"stopOnEntry\":true}}"
    sleep 3

    # Wait and collect
    sleep 2
} 2>&1 | timeout 15 "$SERVER" 2>&1

echo ""
echo "=== Test Complete ==="
