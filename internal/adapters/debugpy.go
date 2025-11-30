package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// DebugpyAdapter implements the Adapter interface for Python/debugpy
type DebugpyAdapter struct {
	pythonPath string
}

// NewDebugpyAdapter creates a new debugpy adapter
func NewDebugpyAdapter(cfg config.DebugpyConfig) *DebugpyAdapter {
	pythonPath := cfg.PythonPath
	if pythonPath == "" {
		pythonPath = "python3"
	}

	return &DebugpyAdapter{
		pythonPath: pythonPath,
	}
}

// Language returns the language this adapter supports
func (d *DebugpyAdapter) Language() types.Language {
	return types.LanguagePython
}

// getPythonPath returns the Python interpreter path, checking args first for venv support.
// Supports both VS Code's "python" attribute and debugpy's "pythonPath" attribute.
func (d *DebugpyAdapter) getPythonPath(args map[string]interface{}) string {
	// VS Code uses "python" attribute
	if p, ok := args["python"].(string); ok && p != "" {
		return p
	}
	// debugpy traditionally uses "pythonPath"
	if p, ok := args["pythonPath"].(string); ok && p != "" {
		return p
	}
	// Fall back to config default
	return d.pythonPath
}

// detectVenvRoot checks if pythonPath is inside a venv and returns the root directory.
// Returns empty string if not a venv or venv cannot be detected.
func (d *DebugpyAdapter) detectVenvRoot(pythonPath string) string {
	// Python path is typically: /path/to/venv/bin/python -> venv root: /path/to/venv
	binDir := filepath.Dir(pythonPath)
	venvRoot := filepath.Dir(binDir)

	// Check for pyvenv.cfg (standard venv marker created by python -m venv)
	if _, err := os.Stat(filepath.Join(venvRoot, "pyvenv.cfg")); err == nil {
		return venvRoot
	}
	return ""
}

// Spawn starts a debugpy debug adapter process
func (d *DebugpyAdapter) Spawn(ctx context.Context, program string, args map[string]interface{}) (string, *exec.Cmd, error) {
	port, err := findAvailablePort()
	if err != nil {
		return "", nil, fmt.Errorf("failed to find available port: %w", err)
	}

	address := fmt.Sprintf("127.0.0.1:%d", port)

	// Get python path from args (supports venv) or fall back to config default
	pythonPath := d.getPythonPath(args)

	// debugpy --listen mode starts the adapter and waits for a connection
	cmdArgs := []string{
		"-m", "debugpy.adapter",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	}

	cmd := exec.CommandContext(ctx, pythonPath, cmdArgs...)
	cmd.Env = os.Environ()
	// Explicitly disconnect stdin to prevent TTY issues when run as MCP server.
	cmd.Stdin = nil
	// Set platform-specific process attributes (procattr_unix.go / procattr_windows.go)
	setProcAttr(cmd)

	// Auto-detect venv and set VIRTUAL_ENV environment variable
	if venvRoot := d.detectVenvRoot(pythonPath); venvRoot != "" {
		cmd.Env = append(cmd.Env, "VIRTUAL_ENV="+venvRoot)
		// Prepend venv bin to PATH for subprocess calls
		binDir := filepath.Dir(pythonPath)
		for i, env := range cmd.Env {
			if strings.HasPrefix(env, "PATH=") {
				cmd.Env[i] = "PATH=" + binDir + string(os.PathListSeparator) + env[5:]
				break
			}
		}
	}

	// Add custom environment variables (these override auto-detected values)
	if env, ok := args["env"].(map[string]any); ok {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, fmt.Sprint(v)))
		}
	}

	// Set working directory
	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	// Capture stderr to help debug issues
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start debugpy: %w", err)
	}

	// Wait for the server to start - debugpy can take a moment to initialize
	time.Sleep(1 * time.Second)

	// Verify the process is still running
	if cmd.Process == nil {
		return "", nil, fmt.Errorf("debugpy process failed to start")
	}

	return address, cmd, nil
}

// BuildLaunchArgs builds the launch arguments for debugpy
func (d *DebugpyAdapter) BuildLaunchArgs(program string, args map[string]interface{}) map[string]interface{} {
	launchArgs := map[string]any{
		"type":    "python",
		"request": "launch",
		"program": program,
		"console": "internalConsole",
	}

	// Pass through common arguments
	if programArgs, ok := args["args"].([]any); ok {
		strArgs := make([]string, len(programArgs))
		for i, a := range programArgs {
			strArgs[i] = fmt.Sprint(a)
		}
		launchArgs["args"] = strArgs
	}

	if cwd, ok := args["cwd"].(string); ok {
		launchArgs["cwd"] = cwd
	}

	if env, ok := args["env"].(map[string]interface{}); ok {
		envMap := make(map[string]string)
		for k, v := range env {
			envMap[k] = fmt.Sprint(v)
		}
		launchArgs["env"] = envMap
	}

	if stopOnEntry, ok := args["stopOnEntry"].(bool); ok {
		launchArgs["stopOnEntry"] = stopOnEntry
	}

	// Python-specific options
	if module, ok := args["module"].(string); ok {
		delete(launchArgs, "program")
		launchArgs["module"] = module
	}

	if pythonPath, ok := args["pythonPath"].(string); ok {
		launchArgs["pythonPath"] = pythonPath
	}

	return launchArgs
}

// BuildAttachArgs builds the attach arguments for debugpy
func (d *DebugpyAdapter) BuildAttachArgs(args map[string]interface{}) map[string]interface{} {
	attachArgs := map[string]interface{}{
		"type":    "python",
		"request": "attach",
	}

	// Connect to a debugpy server
	if host, ok := args["host"].(string); ok {
		attachArgs["host"] = host
	} else {
		attachArgs["host"] = "127.0.0.1"
	}

	if port, ok := args["port"].(float64); ok {
		attachArgs["port"] = int(port)
	}

	// Or attach to a process by PID
	if pid, ok := args["pid"].(float64); ok {
		attachArgs["processId"] = int(pid)
	}

	return attachArgs
}
