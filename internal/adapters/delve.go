package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// DelveAdapter implements the Adapter interface for Go/Delve
type DelveAdapter struct {
	dlvPath    string
	buildFlags string
}

// NewDelveAdapter creates a new Delve adapter
func NewDelveAdapter(cfg config.DelveConfig) *DelveAdapter {
	dlvPath := cfg.Path
	if dlvPath == "" {
		dlvPath = "dlv"
	}

	return &DelveAdapter{
		dlvPath:    dlvPath,
		buildFlags: cfg.BuildFlags,
	}
}

// Language returns the language this adapter supports
func (d *DelveAdapter) Language() types.Language {
	return types.LanguageGo
}

// Spawn starts a Delve debug adapter process
func (d *DelveAdapter) Spawn(ctx context.Context, program string, args map[string]interface{}) (string, *exec.Cmd, error) {
	port, err := findAvailablePort()
	if err != nil {
		return "", nil, fmt.Errorf("failed to find available port: %w", err)
	}

	address := fmt.Sprintf("127.0.0.1:%d", port)

	dlvArgs := []string{
		"dap",
		"--listen", address,
	}

	if d.buildFlags != "" {
		dlvArgs = append(dlvArgs, "--build-flags", d.buildFlags)
	}

	cmd := exec.CommandContext(ctx, d.dlvPath, dlvArgs...)
	cmd.Env = os.Environ()
	// Explicitly disconnect stdin to prevent TTY issues when run as MCP server.
	cmd.Stdin = nil
	// Capture stderr to help debug issues
	cmd.Stderr = os.Stderr
	// Set platform-specific process attributes (procattr_unix.go / procattr_windows.go)
	setProcAttr(cmd)

	// Set working directory if specified
	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start dlv: %w", err)
	}

	// Wait for the server to start
	time.Sleep(500 * time.Millisecond)

	return address, cmd, nil
}

// BuildLaunchArgs builds the launch arguments for Delve
func (d *DelveAdapter) BuildLaunchArgs(program string, args map[string]interface{}) map[string]interface{} {
	launchArgs := map[string]interface{}{
		"mode":    "debug",
		"program": program,
	}

	// Pass through common arguments
	if programArgs, ok := args["args"].([]interface{}); ok {
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

	// Delve-specific options
	if buildFlags, ok := args["buildFlags"].(string); ok {
		launchArgs["buildFlags"] = buildFlags
	}

	return launchArgs
}

// BuildAttachArgs builds the attach arguments for Delve
func (d *DelveAdapter) BuildAttachArgs(args map[string]interface{}) map[string]interface{} {
	attachArgs := map[string]interface{}{
		"mode": "local",
	}

	if pid, ok := args["pid"].(float64); ok {
		attachArgs["processId"] = int(pid)
	}

	return attachArgs
}
