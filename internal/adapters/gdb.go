package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/internal/dap"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// GDBAdapter implements the StdioAdapter interface for GDB's native DAP support.
// Requires GDB 14.1 or later which includes built-in DAP support via --interpreter=dap.
// Supports debugging C, C++, Rust, and other languages supported by GDB.
type GDBAdapter struct {
	gdbPath string
}

// NewGDBAdapter creates a new GDB adapter
func NewGDBAdapter(cfg config.GDBConfig) *GDBAdapter {
	path := cfg.Path
	if path == "" {
		path = "gdb"
	}

	return &GDBAdapter{
		gdbPath: path,
	}
}

// Language returns the language this adapter supports
func (g *GDBAdapter) Language() types.Language {
	// GDB supports multiple languages; the registry will register for each
	return types.LanguageC
}

// IsStdio returns true because GDB DAP uses stdio transport
func (g *GDBAdapter) IsStdio() bool {
	return true
}

// Spawn is implemented for interface compatibility but should not be called directly.
// Use SpawnStdio instead for stdio-based adapters.
func (g *GDBAdapter) Spawn(ctx context.Context, program string, args map[string]interface{}) (string, *exec.Cmd, error) {
	return "", nil, fmt.Errorf("gdb adapter uses stdio transport, use SpawnStdio instead")
}

// SpawnStdio starts GDB in DAP mode and returns a DAP client connected via stdin/stdout
func (g *GDBAdapter) SpawnStdio(ctx context.Context, program string, args map[string]interface{}) (*dap.Client, *exec.Cmd, error) {
	// Build GDB command with DAP interpreter
	gdbArgs := []string{
		"--interpreter=dap",
	}

	// Add pretty printing by default for better output
	gdbArgs = append(gdbArgs, "--eval-command", "set print pretty on")

	// Quiet mode to suppress startup messages that could interfere with DAP
	gdbArgs = append(gdbArgs, "--quiet")

	cmd := exec.CommandContext(ctx, g.gdbPath, gdbArgs...)
	cmd.Env = os.Environ()

	// Set platform-specific process attributes (procattr_unix.go / procattr_windows.go)
	setProcAttr(cmd)

	// Set working directory if specified
	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	// Get stdin pipe (we write to this)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Get stdout pipe (we read from this)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, fmt.Errorf("failed to start gdb: %w", err)
	}

	// Create transport using the process's stdio
	transport := dap.NewStdioTransport(stdin, stdout)
	client := dap.NewClient(transport)

	return client, cmd, nil
}

// BuildLaunchArgs builds the launch arguments for GDB DAP
func (g *GDBAdapter) BuildLaunchArgs(program string, args map[string]interface{}) map[string]interface{} {
	launchArgs := map[string]interface{}{
		"program": program,
	}

	// Pass through program arguments
	if programArgs, ok := args["args"].([]interface{}); ok {
		strArgs := make([]string, len(programArgs))
		for i, a := range programArgs {
			strArgs[i] = fmt.Sprint(a)
		}
		launchArgs["args"] = strArgs
	}

	// Working directory
	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		launchArgs["cwd"] = cwd
	}

	// Environment variables (GDB DAP expects object format)
	if env, ok := args["env"].(map[string]interface{}); ok {
		envMap := make(map[string]string)
		for k, v := range env {
			envMap[k] = fmt.Sprint(v)
		}
		launchArgs["env"] = envMap
	}

	// Stop on entry (first instruction)
	if stopOnEntry, ok := args["stopOnEntry"].(bool); ok {
		launchArgs["stopOnEntry"] = stopOnEntry
	}

	// GDB-specific: stop at beginning of main subprogram
	if stopAtMain, ok := args["stopAtBeginningOfMainSubprogram"].(bool); ok {
		launchArgs["stopAtBeginningOfMainSubprogram"] = stopAtMain
	}

	return launchArgs
}

// BuildAttachArgs builds the attach arguments for GDB DAP
func (g *GDBAdapter) BuildAttachArgs(args map[string]interface{}) map[string]interface{} {
	attachArgs := map[string]interface{}{}

	// Attach by process ID
	if pid, ok := args["pid"].(float64); ok {
		attachArgs["pid"] = int(pid)
	}

	// Program path (for symbol resolution)
	if program, ok := args["program"].(string); ok {
		attachArgs["program"] = program
	}

	// Remote target connection string (for gdbserver)
	// e.g., "localhost:1234" or "/dev/ttyUSB0"
	if target, ok := args["target"].(string); ok {
		attachArgs["target"] = target
	}

	return attachArgs
}
