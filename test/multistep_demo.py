#!/usr/bin/env python3
"""
Demonstrates EFFICIENT multi-step debugging workflow using the new tools.

This demo shows the token-efficient approach:
1. Launch debugger
2. Use debug_run_to_line (one call = breakpoint + continue + wait + snapshot)
3. Use debug_batch_evaluate (one call = evaluate multiple expressions)
4. Compare to the verbose step-by-step approach
"""

import json
import subprocess
import sys
import time
import os

class MCPClient:
    def __init__(self, server_path):
        self.request_id = 0
        # Set up environment with venv
        venv_bin = os.path.join(os.path.dirname(__file__), "venv", "bin")
        env = os.environ.copy()
        env["PATH"] = venv_bin + ":" + env.get("PATH", "")

        self.proc = subprocess.Popen(
            [server_path, "--mode", "full"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
            text=True,
            bufsize=1
        )
        time.sleep(0.5)

    def send(self, method, params=None):
        self.request_id += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method
        }
        if params:
            request["params"] = params

        msg = json.dumps(request) + "\n"
        self.proc.stdin.write(msg)
        self.proc.stdin.flush()

        # Read response (skip non-JSON lines)
        while True:
            line = self.proc.stdout.readline()
            if not line:
                raise Exception("Server closed connection")
            try:
                resp = json.loads(line)
                if resp.get("id") is not None:
                    return resp
            except json.JSONDecodeError:
                continue

    def call_tool(self, name, arguments):
        return self.send("tools/call", {"name": name, "arguments": arguments})

    def close(self):
        self.proc.terminate()
        self.proc.wait()

def format_response(resp):
    """Extract and format the tool response content."""
    if resp.get("error"):
        return f"ERROR: {resp['error']}"

    result = resp.get("result", {})
    content = result.get("content", [])
    if content:
        text = content[0].get("text", "")
        try:
            return json.loads(text)
        except:
            return text
    return result

def print_step(step_num, action, response):
    """Pretty print a debugging step."""
    print(f"\n{'='*60}")
    print(f"STEP {step_num}: {action}")
    print('='*60)
    formatted = format_response(response)
    if isinstance(formatted, dict):
        print(json.dumps(formatted, indent=2))
    else:
        print(formatted)

def main():
    server_path = os.path.join(os.path.dirname(__file__), "..", "bin", "dap-mcp")
    calculator_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "python_project", "calculator.py"))

    print("="*70)
    print("EFFICIENT DEBUGGING DEMO - New Token-Saving Tools")
    print("="*70)
    print("\nCompare: OLD way (many calls) vs NEW way (few calls)")
    print()

    client = MCPClient(server_path)

    try:
        # Initialize MCP
        resp = client.send("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "efficient-demo", "version": "1.0.0"}
        })
        print("MCP initialized.\n")

        # STEP 1: Launch debugger
        resp = client.call_tool("debug_launch", {
            "language": "python",
            "program": calculator_path,
            "stopOnEntry": True
        })
        launch_result = format_response(resp)
        print_step(1, "Launch debugger", resp)
        session_id = launch_result["sessionId"]

        time.sleep(1)

        # ============================================
        # NEW EFFICIENT WAY: debug_run_to_line
        # ============================================
        print("\n" + "="*70)
        print("NEW: debug_run_to_line (1 call = breakpoint + continue + wait + snapshot)")
        print("="*70)

        print_step(2, "Run to line 103 (ONE CALL does it all!)",
            client.call_tool("debug_run_to_line", {
                "sessionId": session_id,
                "path": calculator_path,
                "line": 103
            }))

        # ============================================
        # NEW EFFICIENT WAY: debug_batch_evaluate
        # ============================================
        print("\n" + "="*70)
        print("NEW: debug_batch_evaluate (1 call = evaluate multiple expressions)")
        print("="*70)

        print_step(3, "Batch evaluate multiple expressions (ONE CALL)",
            client.call_tool("debug_batch_evaluate", {
                "sessionId": session_id,
                "expressions": json.dumps([
                    "5 + 3",
                    "10 * 2",
                    "len('hello')",
                    "type(add)"
                ])
            }))

        # Run to another line
        print_step(4, "Run to line 109 (factorial call)",
            client.call_tool("debug_run_to_line", {
                "sessionId": session_id,
                "path": calculator_path,
                "line": 109
            }))

        # Batch evaluate at new location
        print_step(5, "Batch evaluate at factorial call",
            client.call_tool("debug_batch_evaluate", {
                "sessionId": session_id,
                "expressions": json.dumps([
                    "5",           # arg to factorial
                    "factorial",   # the function itself
                    "1+2+3+4+5"   # what factorial(5) should return
                ])
            }))

        # ============================================
        # CONDITIONAL BREAKPOINT EXAMPLE
        # ============================================
        print("\n" + "="*70)
        print("CONDITIONAL BREAKPOINT (DAP standard feature)")
        print("="*70)

        print_step(6, "Set conditional breakpoint (line 37, condition: i > 3)",
            client.call_tool("control_set_breakpoints", {
                "sessionId": session_id,
                "path": calculator_path,
                "breakpoints": json.dumps([{
                    "line": 37,
                    "condition": "i > 3"  # Only stop when i > 3
                }])
            }))

        # Disconnect
        print_step(7, "Disconnect",
            client.call_tool("debug_disconnect", {
                "sessionId": session_id,
                "terminateDebuggee": True
            }))

        # ============================================
        # TOKEN COMPARISON
        # ============================================
        print("\n" + "="*70)
        print("TOKEN EFFICIENCY COMPARISON")
        print("="*70)
        print("""
OLD WAY to reach line 103 and check variables:
  1. control_set_breakpoints   (~100 tokens)
  2. control_continue          (~50 tokens)
  3. [wait manually]
  4. debug_snapshot            (~1500 tokens)
  Total: 3+ calls, ~1650 tokens

NEW WAY with debug_run_to_line:
  1. debug_run_to_line         (~800 tokens - includes snapshot!)
  Total: 1 call, ~800 tokens

  SAVINGS: 50%+ fewer tokens, 3x fewer API calls

OLD WAY to evaluate 4 expressions:
  4x inspect_evaluate          (~400 tokens)
  Total: 4 calls, ~400 tokens

NEW WAY with debug_batch_evaluate:
  1. debug_batch_evaluate      (~200 tokens)
  Total: 1 call, ~200 tokens

  SAVINGS: 50% fewer tokens, 4x fewer API calls
""")

    finally:
        client.close()

if __name__ == "__main__":
    main()
