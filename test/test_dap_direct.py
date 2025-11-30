#!/usr/bin/env python3
"""
Direct test of debugpy DAP connection.
This tests that we can spawn debugpy and connect to it via DAP.
"""

import json
import socket
import subprocess
import sys
import time
import os

# Get the path to the venv python
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
VENV_PYTHON = os.path.join(SCRIPT_DIR, "venv", "bin", "python")

def find_free_port():
    """Find a free port."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]


def send_dap_request(sock, seq, command, arguments=None):
    """Send a DAP request."""
    request = {
        "seq": seq,
        "type": "request",
        "command": command,
    }
    if arguments:
        request["arguments"] = arguments

    body = json.dumps(request)
    message = f"Content-Length: {len(body)}\r\n\r\n{body}"
    sock.sendall(message.encode())
    print(f">>> Sent: {command}")


def recv_dap_message(sock):
    """Receive a DAP message."""
    # Read headers
    headers = b""
    while b"\r\n\r\n" not in headers:
        chunk = sock.recv(1)
        if not chunk:
            return None
        headers += chunk

    header_str = headers.decode()
    content_length = 0
    for line in header_str.split("\r\n"):
        if line.startswith("Content-Length:"):
            content_length = int(line.split(":")[1].strip())

    # Read body
    body = b""
    while len(body) < content_length:
        chunk = sock.recv(content_length - len(body))
        if not chunk:
            return None
        body += chunk

    message = json.loads(body.decode())
    msg_type = message.get("type", "unknown")
    if msg_type == "response":
        print(f"<<< Response: {message.get('command')} success={message.get('success')}")
    elif msg_type == "event":
        print(f"<<< Event: {message.get('event')}")
    return message


def main():
    """Run the test."""
    port = find_free_port()
    print(f"Using port: {port}")

    # Start debugpy adapter
    print("\n1. Starting debugpy adapter...")
    proc = subprocess.Popen(
        [VENV_PYTHON, "-m", "debugpy.adapter",
         "--host", "127.0.0.1",
         "--port", str(port)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    print(f"   Started debugpy adapter (PID: {proc.pid})")
    time.sleep(1)

    # Connect to the adapter
    print("\n2. Connecting to debugpy adapter...")
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(10)

    try:
        sock.connect(("127.0.0.1", port))
        print("   Connected!")

        seq = 1

        # Initialize
        print("\n3. Sending initialize request...")
        send_dap_request(sock, seq, "initialize", {
            "clientID": "test-client",
            "clientName": "Test Client",
            "adapterID": "python",
            "pathFormat": "path",
            "linesStartAt1": True,
            "columnsStartAt1": True,
            "supportsVariableType": True,
            "supportsVariablePaging": True,
        })
        seq += 1

        # Read response and any events
        while True:
            msg = recv_dap_message(sock)
            if not msg:
                break
            if msg.get("type") == "response" and msg.get("command") == "initialize":
                print(f"   Capabilities: {list(msg.get('body', {}).keys())}")
                break

        # Launch the calculator program
        print("\n4. Launching calculator.py...")
        calculator_path = os.path.join(SCRIPT_DIR, "python_project", "calculator.py")
        send_dap_request(sock, seq, "launch", {
            "type": "python",
            "request": "launch",
            "program": calculator_path,
            "console": "internalConsole",
            "stopOnEntry": True,
        })
        seq += 1

        # Read responses/events - wait for initialized event then send configurationDone
        thread_id = 1
        got_initialized = False
        sent_config_done = False
        got_stopped = False

        while not got_stopped:
            msg = recv_dap_message(sock)
            if not msg:
                break

            # Handle initialized event - send configurationDone immediately
            if msg.get("type") == "event" and msg.get("event") == "initialized":
                got_initialized = True
                print("   Got initialized event")
                # Send configurationDone right away
                print("\n5. Sending configurationDone...")
                send_dap_request(sock, seq, "configurationDone", {})
                seq += 1
                sent_config_done = True

            if msg.get("type") == "response" and msg.get("command") == "launch":
                if msg.get("success"):
                    print("   Launch successful!")
                else:
                    print(f"   Launch failed: {msg.get('message')}")
                    return

            if msg.get("type") == "response" and msg.get("command") == "configurationDone":
                print("   configurationDone acknowledged")
                continue

            if msg.get("type") == "event" and msg.get("event") == "stopped":
                print(f"\n6. Stopped! Reason: {msg.get('body', {}).get('reason')}")
                thread_id = msg.get('body', {}).get('threadId', 1)
                got_stopped = True
                break

            if msg.get("type") == "event" and msg.get("event") == "thread":
                print(f"   Thread event: {msg.get('body', {})}")
                continue
            if msg.get("type") == "event" and msg.get("event") == "process":
                print(f"   Process event: started")
                continue

        # Get threads
        print("\n7. Getting threads...")
        send_dap_request(sock, seq, "threads", {})
        seq += 1
        while True:
            msg = recv_dap_message(sock)
            if msg.get("type") == "response" and msg.get("command") == "threads":
                threads = msg.get("body", {}).get("threads", [])
                print(f"   Threads: {[t['name'] for t in threads]}")
                if threads:
                    thread_id = threads[0]["id"]
                break

        # Get stack trace
        print("\n8. Getting stack trace...")
        send_dap_request(sock, seq, "stackTrace", {
            "threadId": thread_id,
            "startFrame": 0,
            "levels": 20,
        })
        seq += 1
        while True:
            msg = recv_dap_message(sock)
            if msg.get("type") == "response" and msg.get("command") == "stackTrace":
                frames = msg.get("body", {}).get("stackFrames", [])
                print(f"   Stack frames: {len(frames)}")
                for f in frames[:5]:
                    source = f.get("source", {}).get("name", "?")
                    print(f"      - {f['name']} at {source}:{f.get('line', '?')}")
                if frames:
                    frame_id = frames[0]["id"]
                break

        # Get scopes
        print("\n9. Getting scopes...")
        send_dap_request(sock, seq, "scopes", {"frameId": frame_id})
        seq += 1
        while True:
            msg = recv_dap_message(sock)
            if msg.get("type") == "response" and msg.get("command") == "scopes":
                scopes = msg.get("body", {}).get("scopes", [])
                print(f"   Scopes: {[s['name'] for s in scopes]}")
                for s in scopes:
                    if s.get("variablesReference", 0) > 0:
                        vars_ref = s["variablesReference"]
                        break
                break

        # Get variables
        print("\n10. Getting variables...")
        send_dap_request(sock, seq, "variables", {"variablesReference": vars_ref})
        seq += 1
        while True:
            msg = recv_dap_message(sock)
            if msg.get("type") == "response" and msg.get("command") == "variables":
                variables = msg.get("body", {}).get("variables", [])
                print(f"   Variables: {len(variables)}")
                for v in variables[:10]:
                    print(f"      - {v['name']}: {v['value'][:50] if len(v.get('value', '')) > 50 else v.get('value', '')}")
                break

        # Continue execution
        print("\n11. Continuing execution...")
        send_dap_request(sock, seq, "continue", {"threadId": thread_id})
        seq += 1

        # Wait a moment for program to run
        time.sleep(0.5)

        # Read any remaining messages
        sock.settimeout(1)
        try:
            while True:
                msg = recv_dap_message(sock)
                if not msg:
                    break
        except socket.timeout:
            pass

        # Disconnect
        print("\n12. Disconnecting...")
        send_dap_request(sock, seq, "disconnect", {"terminateDebuggee": True})

        print("\n" + "="*60)
        print("TEST COMPLETED SUCCESSFULLY!")
        print("="*60)

    except Exception as e:
        print(f"\nError: {e}")
        import traceback
        traceback.print_exc()
    finally:
        sock.close()
        proc.terminate()
        proc.wait(timeout=5)


if __name__ == "__main__":
    main()
