#!/usr/bin/env python3
"""
Test harness for DAP-MCP server.

This script starts the MCP server and sends test requests to verify functionality.
"""

import json
import subprocess
import sys
import time
from typing import Any

class MCPClient:
    """Simple MCP client for testing."""

    def __init__(self, server_path: str):
        self.server_path = server_path
        self.process = None
        self.request_id = 0

    def start(self):
        """Start the MCP server process."""
        self.process = subprocess.Popen(
            [self.server_path, "--mode", "full"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=False,
        )
        print(f"Started MCP server (PID: {self.process.pid})")
        time.sleep(0.5)  # Give server time to initialize

    def stop(self):
        """Stop the MCP server process."""
        if self.process:
            self.process.terminate()
            self.process.wait(timeout=5)
            print("MCP server stopped")

    def send_request(self, method: str, params: dict = None) -> dict:
        """Send a JSON-RPC request and get the response."""
        self.request_id += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method,
        }
        if params:
            request["params"] = params

        # Encode as JSON-RPC message with Content-Length header
        body = json.dumps(request)
        message = f"Content-Length: {len(body)}\r\n\r\n{body}"

        # Send to server
        self.process.stdin.write(message.encode())
        self.process.stdin.flush()

        # Read response
        response = self._read_response()
        return response

    def _read_response(self) -> dict:
        """Read a JSON-RPC response from the server."""
        # Read Content-Length header
        headers = {}
        while True:
            line = self.process.stdout.readline().decode().strip()
            if not line:
                break
            if ":" in line:
                key, value = line.split(":", 1)
                headers[key.strip()] = value.strip()

        content_length = int(headers.get("Content-Length", 0))
        if content_length > 0:
            body = self.process.stdout.read(content_length).decode()
            return json.loads(body)
        return {}

    def initialize(self) -> dict:
        """Send MCP initialize request."""
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
        """Call a tool with arguments."""
        return self.send_request("tools/call", {
            "name": name,
            "arguments": arguments
        })


def print_json(data: Any, title: str = None):
    """Pretty print JSON data."""
    if title:
        print(f"\n{'='*60}")
        print(f"{title}")
        print('='*60)
    print(json.dumps(data, indent=2))


def main():
    """Run the test suite."""
    import os

    # Path to the MCP server binary
    script_dir = os.path.dirname(os.path.abspath(__file__))
    server_path = os.path.join(script_dir, "..", "bin", "dap-mcp")

    if not os.path.exists(server_path):
        print(f"Error: Server binary not found at {server_path}")
        print("Please run 'make build' first")
        sys.exit(1)

    client = MCPClient(server_path)

    try:
        # Start the server
        client.start()

        # Initialize
        print_json(client.initialize(), "Initialize Response")

        # List tools
        print_json(client.list_tools(), "Available Tools")

        # List sessions (should be empty)
        result = client.call_tool("debug_list_sessions", {})
        print_json(result, "List Sessions (empty)")

        # Launch a Python debug session
        python_file = os.path.join(script_dir, "python_project", "calculator.py")
        result = client.call_tool("debug_launch", {
            "language": "python",
            "program": python_file,
            "stopOnEntry": True
        })
        print_json(result, "Launch Python Debug Session")

        # Extract session ID from result
        if "result" in result and result["result"]:
            content = result["result"].get("content", [])
            if content:
                session_data = json.loads(content[0].get("text", "{}"))
                session_id = session_data.get("sessionId")

                if session_id:
                    print(f"\nSession ID: {session_id}")

                    # Wait for debugger to attach
                    time.sleep(1)

                    # Get threads
                    result = client.call_tool("inspect_threads", {
                        "sessionId": session_id
                    })
                    print_json(result, "Threads")

                    # Get debug snapshot
                    result = client.call_tool("debug_snapshot", {
                        "sessionId": session_id,
                        "maxStackDepth": 5,
                        "expandVariables": True
                    })
                    print_json(result, "Debug Snapshot")

                    # Disconnect
                    result = client.call_tool("debug_disconnect", {
                        "sessionId": session_id,
                        "terminateDebuggee": True
                    })
                    print_json(result, "Disconnect")

    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()
    finally:
        client.stop()


if __name__ == "__main__":
    main()
