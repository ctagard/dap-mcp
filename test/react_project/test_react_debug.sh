#!/bin/bash
# Test script for React/browser debugging with dap-mcp
# Prerequisites:
#   1. React dev server running on http://localhost:3000 (npm run dev)
#   2. vscode-js-debug installed at tools/js-debug/src/dapDebugServer.js

set -e

cd "$(dirname "$0")/../.."

CONFIG_PATH="$(pwd)/test/react_project/dap-mcp-config.json"
REACT_URL="http://localhost:3000"
WEB_ROOT="$(pwd)/test/react_project/src"

echo "=== React Browser Debugging Test ==="
echo "Config: $CONFIG_PATH"
echo "URL: $REACT_URL"
echo "WebRoot: $WEB_ROOT"
echo ""

# Check if the React dev server is running
if ! curl -s "$REACT_URL" > /dev/null 2>&1; then
    echo "ERROR: React dev server is not running on $REACT_URL"
    echo "Please start it first: cd test/react_project && npm run dev"
    exit 1
fi
echo "✓ React dev server is running"

# Check if js-debug is available
JS_DEBUG_PATH="$(pwd)/tools/js-debug/src/dapDebugServer.js"
if [ ! -f "$JS_DEBUG_PATH" ]; then
    echo "ERROR: vscode-js-debug not found at $JS_DEBUG_PATH"
    exit 1
fi
echo "✓ vscode-js-debug is available"

# Build the MCP server
make build
echo "✓ dap-mcp built"

# Create the test input that will be piped to dap-mcp
cat > /tmp/test_react_debug.json << EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"react-debug-test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"debug_launch","arguments":{"language":"javascript","program":"$REACT_URL","target":"chrome","webRoot":"$WEB_ROOT","stopOnEntry":false}}}
EOF

echo ""
echo "=== Launching Chrome debugger for React app ==="
echo "This will open Chrome and attach a debugger to your React app."
echo "Expected behavior:"
echo "  1. Chrome opens with your React app"
echo "  2. Debugger is attached (you can set breakpoints in Dev Tools)"
echo "  3. The MCP server returns the session ID"
echo ""
echo "Press Ctrl+C to stop the test after Chrome opens."
echo ""

# Run the test with a timeout (Chrome will stay open for interaction)
timeout 30 bash -c 'cat /tmp/test_react_debug.json | ./bin/dap-mcp --config test/react_project/dap-mcp-config.json' 2>&1 || true

echo ""
echo "=== Test Complete ==="
