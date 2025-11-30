package test

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type MCPClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	reader    *bufio.Reader
	requestID int
}

func NewMCPClient(serverPath string) (*MCPClient, error) {
	cmd := exec.Command(serverPath, "--mode", "full")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	cmd.Stderr = os.Stderr

	// Inherit the environment with venv PATH
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)

	return &MCPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReader(stdout),
	}, nil
}

func (c *MCPClient) Close() {
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	_ = c.cmd.Wait()
}

func (c *MCPClient) SendRequest(method string, params interface{}) (map[string]interface{}, error) {
	c.requestID++

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.requestID,
		"method":  method,
	}
	if params != nil {
		request["params"] = params
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// mcp-go uses newline-delimited JSON
	message := string(body) + "\n"
	if _, err := c.stdin.Write([]byte(message)); err != nil {
		return nil, err
	}

	return c.readResponse()
}

func (c *MCPClient) readResponse() (map[string]interface{}, error) {
	// mcp-go uses newline-delimited JSON - read lines until we get a valid response
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = line[:len(line)-1] // Remove \n
		if len(line) == 0 {
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			// Skip non-JSON lines (like log messages)
			continue
		}

		// Skip error responses without an ID (these are parse errors from bad input)
		if result["id"] == nil && result["error"] != nil {
			continue
		}

		return result, nil
	}
}

func TestMCPServer(t *testing.T) {
	// Find server binary
	serverPath := filepath.Join("..", "bin", "dap-mcp")
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		t.Skip("Server binary not found. Run 'make build' first.")
	}

	// Start client
	client, err := NewMCPClient(serverPath)
	if err != nil {
		t.Fatalf("Failed to start MCP client: %v", err)
	}
	defer client.Close()

	// Test initialize
	t.Run("Initialize", func(t *testing.T) {
		resp, err := client.SendRequest("initialize", map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test",
				"version": "1.0.0",
			},
		})
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		if resp["error"] != nil {
			t.Fatalf("Initialize returned error: %v", resp["error"])
		}

		result := resp["result"].(map[string]interface{})
		if result["serverInfo"] == nil {
			t.Error("Missing serverInfo in response")
		}
		t.Logf("Server: %v", result["serverInfo"])
	})

	// Test list tools
	t.Run("ListTools", func(t *testing.T) {
		resp, err := client.SendRequest("tools/list", nil)
		if err != nil {
			t.Fatalf("List tools failed: %v", err)
		}

		if resp["error"] != nil {
			t.Fatalf("List tools returned error: %v", resp["error"])
		}

		result := resp["result"].(map[string]interface{})
		tools := result["tools"].([]interface{})
		t.Logf("Found %d tools", len(tools))

		// 12 tools total: 6 core + 6 control (full mode)
		if len(tools) < 12 {
			t.Errorf("Expected at least 12 tools, got %d", len(tools))
		}

		// Check for key tools
		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolMap := tool.(map[string]interface{})
			toolNames[toolMap["name"].(string)] = true
		}

		expectedTools := []string{
			"debug_launch",
			"debug_attach",
			"debug_disconnect",
			"debug_list_sessions",
			"debug_snapshot",
			"debug_evaluate",
			"debug_breakpoints",
			"debug_step",
			"debug_continue",
		}

		for _, name := range expectedTools {
			if !toolNames[name] {
				t.Errorf("Missing expected tool: %s", name)
			}
		}
	})

	// Test list sessions (should be empty)
	t.Run("ListSessions_Empty", func(t *testing.T) {
		resp, err := client.SendRequest("tools/call", map[string]interface{}{
			"name":      "debug_list_sessions",
			"arguments": map[string]interface{}{},
		})
		if err != nil {
			t.Fatalf("List sessions failed: %v", err)
		}

		if resp["error"] != nil {
			t.Fatalf("List sessions returned error: %v", resp["error"])
		}

		result := resp["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		if len(content) == 0 {
			t.Fatal("No content in response")
		}

		textContent := content[0].(map[string]interface{})
		text := textContent["text"].(string)

		var sessions map[string]interface{}
		if err := json.Unmarshal([]byte(text), &sessions); err != nil {
			t.Fatalf("Failed to parse sessions: %v", err)
		}

		sessionList := sessions["sessions"].([]interface{})
		if len(sessionList) != 0 {
			t.Errorf("Expected empty sessions, got %d", len(sessionList))
		}
		t.Log("Sessions list is empty as expected")
	})
}

func TestPythonDebugSession(t *testing.T) {
	// Find server binary
	serverPath := filepath.Join("..", "bin", "dap-mcp")
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		t.Skip("Server binary not found. Run 'make build' first.")
	}

	// Check for venv with debugpy
	venvPython := filepath.Join(".", "venv", "bin", "python")
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		t.Skip("Python venv not found. Run 'python3 -m venv test/venv && test/venv/bin/pip install debugpy' first.")
	}

	// Set up environment with venv
	origPath := os.Getenv("PATH")
	venvBin := filepath.Join(".", "venv", "bin")
	absVenvBin, _ := filepath.Abs(venvBin)
	os.Setenv("PATH", absVenvBin+":"+origPath)
	defer os.Setenv("PATH", origPath)

	// Start client
	client, err := NewMCPClient(serverPath)
	if err != nil {
		t.Fatalf("Failed to start MCP client: %v", err)
	}
	defer client.Close()

	// Initialize
	resp, err := client.SendRequest("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test",
			"version": "1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if resp["error"] != nil {
		t.Fatalf("Initialize error: %v", resp["error"])
	}

	// Launch Python debugger
	t.Run("LaunchPython", func(t *testing.T) {
		calculatorPath, _ := filepath.Abs(filepath.Join(".", "python_project", "calculator.py"))
		resp, err := client.SendRequest("tools/call", map[string]interface{}{
			"name": "debug_launch",
			"arguments": map[string]interface{}{
				"language":    "python",
				"program":     calculatorPath,
				"stopOnEntry": true,
			},
		})
		if err != nil {
			t.Fatalf("Launch failed: %v", err)
		}

		if resp["error"] != nil {
			t.Fatalf("Launch returned error: %v", resp["error"])
		}

		result := resp["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		if len(content) == 0 {
			t.Fatal("No content in launch response")
		}

		textContent := content[0].(map[string]interface{})
		text := textContent["text"].(string)
		t.Logf("Launch response: %s", text)

		var launchResult map[string]interface{}
		if err := json.Unmarshal([]byte(text), &launchResult); err != nil {
			t.Fatalf("Failed to parse launch result: %v", err)
		}

		sessionID, ok := launchResult["sessionId"].(string)
		if !ok || sessionID == "" {
			t.Fatalf("No session ID in launch result: %v", launchResult)
		}
		t.Logf("Session ID: %s", sessionID)

		// Wait for debugger to settle
		time.Sleep(2 * time.Second)

		// Get debug snapshot
		t.Run("GetSnapshot", func(t *testing.T) {
			resp, err := client.SendRequest("tools/call", map[string]interface{}{
				"name": "debug_snapshot",
				"arguments": map[string]interface{}{
					"sessionId":       sessionID,
					"maxStackDepth":   5,
					"expandVariables": true,
				},
			})
			if err != nil {
				t.Fatalf("Snapshot failed: %v", err)
			}

			if resp["error"] != nil {
				t.Fatalf("Snapshot returned error: %v", resp["error"])
			}

			result := resp["result"].(map[string]interface{})
			content := result["content"].([]interface{})
			if len(content) == 0 {
				t.Fatal("No content in snapshot response")
			}

			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Logf("Snapshot: %s", text[:min(500, len(text))])
		})

		// Disconnect
		t.Run("Disconnect", func(t *testing.T) {
			resp, err := client.SendRequest("tools/call", map[string]interface{}{
				"name": "debug_disconnect",
				"arguments": map[string]interface{}{
					"sessionId":         sessionID,
					"terminateDebuggee": true,
				},
			})
			if err != nil {
				t.Fatalf("Disconnect failed: %v", err)
			}

			if resp["error"] != nil {
				t.Fatalf("Disconnect returned error: %v", resp["error"])
			}

			t.Log("Disconnected successfully")
		})
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
