#!/usr/bin/env python3
"""
Comprehensive test for all supported languages: Python, TypeScript, Go.

Tests the key debugging workflows:
1. Launch debugger with stopOnEntry
2. Run to a specific line
3. Get snapshot (stack, variables)
4. Batch evaluate expressions
5. Disconnect
"""

import json
import subprocess
import sys
import time
import os

class MCPClient:
    def __init__(self, server_path, mode="full"):
        self.request_id = 0
        # Set up environment with venv
        venv_bin = os.path.join(os.path.dirname(__file__), "venv", "bin")
        env = os.environ.copy()
        env["PATH"] = venv_bin + ":" + env.get("PATH", "")

        self.proc = subprocess.Popen(
            [server_path, "--mode", mode],
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
        return {"error": resp['error']}

    result = resp.get("result", {})
    content = result.get("content", [])
    if content:
        text = content[0].get("text", "")
        try:
            return json.loads(text)
        except:
            return {"text": text}
    return result

def test_language(client, language, program_path, target_line, expressions):
    """Test debugging workflow for a specific language."""
    print(f"\n{'='*70}")
    print(f"TESTING: {language.upper()}")
    print(f"Program: {program_path}")
    print(f"{'='*70}")

    results = {
        "language": language,
        "tests": {},
        "success": True
    }

    # Test 1: Launch debugger
    print(f"\n[1] Launching {language} debugger with stopOnEntry=true...")
    resp = client.call_tool("debug_launch", {
        "language": language,
        "program": program_path,
        "stopOnEntry": True
    })
    launch_result = format_response(resp)

    if "error" in launch_result:
        print(f"   FAILED: {launch_result['error']}")
        results["tests"]["launch"] = {"success": False, "error": str(launch_result['error'])}
        results["success"] = False
        return results

    session_id = launch_result.get("sessionId")
    if not session_id:
        print(f"   FAILED: No sessionId returned")
        results["tests"]["launch"] = {"success": False, "error": "No sessionId"}
        results["success"] = False
        return results

    print(f"   SUCCESS: sessionId={session_id}")
    results["tests"]["launch"] = {"success": True, "sessionId": session_id}

    # Give debugger time to start
    time.sleep(2)

    # Test 2: Run to specific line
    print(f"\n[2] Running to line {target_line}...")
    resp = client.call_tool("debug_run_to_line", {
        "sessionId": session_id,
        "path": program_path,
        "line": target_line
    })
    run_result = format_response(resp)

    if "error" in run_result:
        print(f"   FAILED: {run_result['error']}")
        results["tests"]["run_to_line"] = {"success": False, "error": str(run_result['error'])}
        results["success"] = False
    else:
        # Check we got a snapshot with stack info
        has_stack = "stacks" in run_result or "stack" in run_result
        stopped_line = run_result.get("stoppedAt", {}).get("line", "unknown")
        print(f"   SUCCESS: Stopped at line {stopped_line}")
        if has_stack:
            print(f"   Got stack trace in response")
        results["tests"]["run_to_line"] = {
            "success": True,
            "stoppedAt": stopped_line,
            "hasStack": has_stack
        }

    # Test 3: Get snapshot
    print(f"\n[3] Getting debug snapshot...")
    resp = client.call_tool("debug_snapshot", {
        "sessionId": session_id
    })
    snapshot_result = format_response(resp)

    if "error" in snapshot_result:
        print(f"   FAILED: {snapshot_result['error']}")
        results["tests"]["snapshot"] = {"success": False, "error": str(snapshot_result['error'])}
        results["success"] = False
    else:
        thread_count = len(snapshot_result.get("threads", []))
        has_scopes = "scopes" in snapshot_result
        has_variables = "variables" in snapshot_result
        print(f"   SUCCESS: {thread_count} thread(s), hasScopes={has_scopes}, hasVariables={has_variables}")
        results["tests"]["snapshot"] = {
            "success": True,
            "threadCount": thread_count,
            "hasScopes": has_scopes,
            "hasVariables": has_variables
        }

    # Test 4: Batch evaluate
    print(f"\n[4] Batch evaluating expressions: {expressions}")
    resp = client.call_tool("debug_batch_evaluate", {
        "sessionId": session_id,
        "expressions": json.dumps(expressions)
    })
    eval_result = format_response(resp)

    if "error" in eval_result:
        print(f"   FAILED: {eval_result['error']}")
        results["tests"]["batch_evaluate"] = {"success": False, "error": str(eval_result['error'])}
        results["success"] = False
    else:
        evaluations = eval_result.get("evaluations", [])
        success_count = sum(1 for e in evaluations if not e.get("error"))
        print(f"   SUCCESS: {success_count}/{len(expressions)} expressions evaluated")
        for i, expr in enumerate(expressions):
            if i < len(evaluations):
                val = evaluations[i].get("value", evaluations[i].get("error", "???"))
                print(f"      {expr} = {val}")
        results["tests"]["batch_evaluate"] = {
            "success": True,
            "successCount": success_count,
            "total": len(expressions)
        }

    # Test 5: Disconnect
    print(f"\n[5] Disconnecting...")
    resp = client.call_tool("debug_disconnect", {
        "sessionId": session_id,
        "terminateDebuggee": True
    })
    disconnect_result = format_response(resp)

    if "error" in disconnect_result:
        print(f"   FAILED: {disconnect_result['error']}")
        results["tests"]["disconnect"] = {"success": False, "error": str(disconnect_result['error'])}
    else:
        print(f"   SUCCESS")
        results["tests"]["disconnect"] = {"success": True}

    return results

def main():
    base_dir = os.path.dirname(os.path.abspath(__file__))
    server_path = os.path.join(base_dir, "..", "bin", "dap-mcp")

    # Test configurations for each language
    test_configs = [
        {
            "language": "python",
            "program": os.path.join(base_dir, "python_project", "calculator.py"),
            "target_line": 103,  # In main(), after basic operations
            "expressions": ["5 + 3", "len('hello')", "type(add)"]
        },
        {
            "language": "typescript",
            "program": os.path.join(base_dir, "typescript_project", "calculator.ts"),
            "target_line": 121,  # console.log in main()
            "expressions": ["5 + 3", "'hello'.length", "typeof add"]
        },
        {
            "language": "go",
            "program": os.path.join(base_dir, "go_project", "calculator.go"),
            "target_line": 114,  # In main(), basic operations
            "expressions": ["5 + 3", "10 * 2"]
        }
    ]

    print("="*70)
    print("DAP-MCP MULTI-LANGUAGE DEBUGGER TEST")
    print("="*70)

    all_results = []

    for config in test_configs:
        # Check if program exists
        if not os.path.exists(config["program"]):
            print(f"\nSKIPPING {config['language']}: {config['program']} not found")
            continue

        # Create new client for each language
        client = MCPClient(server_path)

        try:
            # Initialize MCP
            resp = client.send("initialize", {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "test-all-languages", "version": "1.0.0"}
            })

            result = test_language(
                client,
                config["language"],
                config["program"],
                config["target_line"],
                config["expressions"]
            )
            all_results.append(result)

        except Exception as e:
            print(f"\nERROR testing {config['language']}: {e}")
            all_results.append({
                "language": config["language"],
                "success": False,
                "error": str(e)
            })
        finally:
            client.close()

        # Brief pause between languages
        time.sleep(1)

    # Summary
    print("\n" + "="*70)
    print("SUMMARY")
    print("="*70)

    for result in all_results:
        lang = result["language"]
        success = result.get("success", False)
        status = "PASS" if success else "FAIL"
        print(f"  {lang.upper():12} [{status}]")

        if not success and "error" in result:
            print(f"               Error: {result['error']}")
        elif "tests" in result:
            for test_name, test_result in result["tests"].items():
                test_status = "OK" if test_result.get("success") else "FAIL"
                print(f"               - {test_name}: {test_status}")

    # Exit code
    all_passed = all(r.get("success", False) for r in all_results)
    if all_passed and len(all_results) > 0:
        print("\nAll tests PASSED!")
        return 0
    else:
        print(f"\nSome tests FAILED or were skipped.")
        return 1

if __name__ == "__main__":
    sys.exit(main())
