#!/usr/bin/env python3
"""
Integration test for DAP-MCP server.
Tests the MCP server by sending JSON-RPC messages via stdin/stdout.
"""

import json
import subprocess
import sys
import time
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SERVER_PATH = os.path.join(SCRIPT_DIR, "..", "bin", "dap-mcp")
VENV_PYTHON = os.path.join(SCRIPT_DIR, "venv", "bin", "python")
CALCULATOR_PATH = os.path.join(SCRIPT_DIR, "python_project", "calculator.py")


class MCPServerTest:
    """Test harness for MCP server."""

    def __init__(self):
        self.process = None
        self.request_id = 0

    def start_server(self):
        """Start the MCP server."""
        # Set up environment with venv python
        env = os.environ.copy()
        env["PATH"] = os.path.dirname(VENV_PYTHON) + ":" + env.get("PATH", "")

        self.process = subprocess.Popen(
            [SERVER_PATH, "--mode", "full"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
        )
        print(f"Started MCP server (PID: {self.process.pid})")
        time.sleep(0.5)

    def stop_server(self):
        """Stop the MCP server."""
        if self.process:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
            print("MCP server stopped")

    def send_request(self, method: str, params: dict = None) -> dict:
        """Send a JSON-RPC request."""
        self.request_id += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method,
        }
        if params:
            request["params"] = params

        body = json.dumps(request)
        message = f"Content-Length: {len(body)}\r\n\r\n{body}"

        self.process.stdin.write(message.encode())
        self.process.stdin.flush()

        return self._read_response()

    def _read_response(self) -> dict:
        """Read a JSON-RPC response."""
        # Read headers
        headers = b""
        while b"\r\n\r\n" not in headers:
            char = self.process.stdout.read(1)
            if not char:
                return {}
            headers += char

        # Parse Content-Length
        header_str = headers.decode()
        content_length = 0
        for line in header_str.split("\r\n"):
            if line.lower().startswith("content-length:"):
                content_length = int(line.split(":")[1].strip())

        # Read body
        if content_length > 0:
            body = self.process.stdout.read(content_length)
            return json.loads(body.decode())
        return {}

    def initialize(self) -> dict:
        """Initialize the MCP connection."""
        return self.send_request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            }
        })

    def list_tools(self) -> dict:
        """List available tools."""
        return self.send_request("tools/list")

    def call_tool(self, name: str, arguments: dict) -> dict:
        """Call a tool."""
        return self.send_request("tools/call", {
            "name": name,
            "arguments": arguments
        })


def print_result(result: dict, title: str):
    """Print a result with formatting."""
    print(f"\n{'='*60}")
    print(f"  {title}")
    print('='*60)

    if "error" in result:
        print(f"ERROR: {result['error']}")
        return

    if "result" in result:
        res = result["result"]
        if isinstance(res, dict) and "content" in res:
            for content in res["content"]:
                if content.get("type") == "text":
                    try:
                        data = json.loads(content["text"])
                        print(json.dumps(data, indent=2))
                    except:
                        print(content["text"])
        else:
            print(json.dumps(res, indent=2))


def main():
    """Run the test."""
    test = MCPServerTest()

    try:
        # Start server
        test.start_server()

        # Initialize
        result = test.initialize()
        print_result(result, "Initialize")

        # List tools
        result = test.list_tools()
        print_result(result, "List Tools")

        # Count tools
        if "result" in result and "tools" in result["result"]:
            tools = result["result"]["tools"]
            print(f"\nFound {len(tools)} tools:")
            for tool in tools:
                print(f"  - {tool['name']}: {tool.get('description', '')[:60]}...")

        # List sessions (should be empty)
        result = test.call_tool("debug_list_sessions", {})
        print_result(result, "List Sessions (empty)")

        # Launch Python debug session
        print("\n" + "="*60)
        print("  Launching Python debug session...")
        print("="*60)
        result = test.call_tool("debug_launch", {
            "language": "python",
            "program": CALCULATOR_PATH,
            "stopOnEntry": True
        })
        print_result(result, "Launch Result")

        # Extract session ID
        session_id = None
        if "result" in result and "content" in result["result"]:
            for content in result["result"]["content"]:
                if content.get("type") == "text":
                    try:
                        data = json.loads(content["text"])
                        session_id = data.get("sessionId")
                    except:
                        pass

        if session_id:
            print(f"\n>>> Session ID: {session_id}")

            # Wait a moment for the debugger to settle
            time.sleep(1)

            # Get threads
            result = test.call_tool("inspect_threads", {
                "sessionId": session_id
            })
            print_result(result, "Threads")

            # Get debug snapshot
            result = test.call_tool("debug_snapshot", {
                "sessionId": session_id,
                "maxStackDepth": 5,
                "expandVariables": True
            })
            print_result(result, "Debug Snapshot")

            # List sessions again
            result = test.call_tool("debug_list_sessions", {})
            print_result(result, "List Sessions (with session)")

            # Disconnect
            result = test.call_tool("debug_disconnect", {
                "sessionId": session_id,
                "terminateDebuggee": True
            })
            print_result(result, "Disconnect")

        print("\n" + "="*60)
        print("  TEST COMPLETED!")
        print("="*60)

    except Exception as e:
        print(f"\nError: {e}")
        import traceback
        traceback.print_exc()

        # Print stderr if available
        if test.process:
            stderr = test.process.stderr.read()
            if stderr:
                print(f"\nServer stderr:\n{stderr.decode()}")

    finally:
        test.stop_server()


if __name__ == "__main__":
    main()
