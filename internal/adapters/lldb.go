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

// LLDBAdapter implements the StdioAdapter interface for LLDB via lldb-dap
// (formerly lldb-vscode). It supports debugging C, C++, Rust, Objective-C, and Swift.
type LLDBAdapter struct {
	lldbDapPath string
}

// NewLLDBAdapter creates a new LLDB adapter
func NewLLDBAdapter(cfg config.LLDBConfig) *LLDBAdapter {
	path := cfg.Path
	if path == "" {
		path = "lldb-dap"
	}

	return &LLDBAdapter{
		lldbDapPath: path,
	}
}

// Language returns the language this adapter supports
func (l *LLDBAdapter) Language() types.Language {
	// LLDB supports multiple languages; we use it for C/C++/Rust
	// The registry will register this adapter for multiple languages
	return types.LanguageC
}

// IsStdio returns true because lldb-dap uses stdio transport
func (l *LLDBAdapter) IsStdio() bool {
	return true
}

// Spawn is implemented for interface compatibility but should not be called directly.
// Use SpawnStdio instead for stdio-based adapters.
func (l *LLDBAdapter) Spawn(ctx context.Context, program string, args map[string]interface{}) (string, *exec.Cmd, error) {
	return "", nil, fmt.Errorf("lldb adapter uses stdio transport, use SpawnStdio instead")
}

// SpawnStdio starts lldb-dap and returns a DAP client connected via stdin/stdout
func (l *LLDBAdapter) SpawnStdio(ctx context.Context, program string, args map[string]interface{}) (*dap.Client, *exec.Cmd, error) {
	// Enable auto REPL mode to support both expression evaluation and command execution
	// In auto mode, lldb-dap uses heuristics to determine if input is a command or expression
	// Commands can also be explicitly prefixed with backtick (`)
	//nolint:gosec // G204: This is a debug adapter that intentionally spawns subprocesses
	cmd := exec.CommandContext(ctx, l.lldbDapPath, "--repl-mode=auto")
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
		_ = stdin.Close()
		return nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, fmt.Errorf("failed to start lldb-dap: %w", err)
	}

	// Create transport using the process's stdio
	transport := dap.NewStdioTransport(stdin, stdout)
	client := dap.NewClient(transport)

	return client, cmd, nil
}

// BuildLaunchArgs builds the launch arguments for lldb-dap
func (l *LLDBAdapter) BuildLaunchArgs(program string, args map[string]interface{}) map[string]interface{} {
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

	// Environment variables
	if env, ok := args["env"].(map[string]interface{}); ok {
		envList := make([]string, 0, len(env))
		for k, v := range env {
			envList = append(envList, fmt.Sprintf("%s=%v", k, v))
		}
		launchArgs["env"] = envList
	}

	// Stop on entry
	if stopOnEntry, ok := args["stopOnEntry"].(bool); ok {
		launchArgs["stopOnEntry"] = stopOnEntry
	}

	// LLDB-specific: initCommands run before target is created
	if initCommands, ok := args["initCommands"].([]interface{}); ok {
		cmds := make([]string, len(initCommands))
		for i, c := range initCommands {
			cmds[i] = fmt.Sprint(c)
		}
		launchArgs["initCommands"] = cmds
	}

	// LLDB-specific: preRunCommands run after target is created but before launch
	if preRunCommands, ok := args["preRunCommands"].([]interface{}); ok {
		cmds := make([]string, len(preRunCommands))
		for i, c := range preRunCommands {
			cmds[i] = fmt.Sprint(c)
		}
		launchArgs["preRunCommands"] = cmds
	}

	// LLDB-specific: stopCommands run after each stop
	if stopCommands, ok := args["stopCommands"].([]interface{}); ok {
		cmds := make([]string, len(stopCommands))
		for i, c := range stopCommands {
			cmds[i] = fmt.Sprint(c)
		}
		launchArgs["stopCommands"] = cmds
	}

	// Source path mapping for relocated binaries
	if sourceMap, ok := args["sourceMap"].([]interface{}); ok {
		launchArgs["sourceMap"] = sourceMap
	}

	return launchArgs
}

// BuildAttachArgs builds the attach arguments for lldb-dap
func (l *LLDBAdapter) BuildAttachArgs(args map[string]interface{}) map[string]interface{} {
	attachArgs := map[string]interface{}{}

	// Attach by process ID
	if pid, ok := args["pid"].(float64); ok {
		attachArgs["pid"] = int(pid)
	}

	// Wait for process to launch (useful for debugging startup)
	if waitFor, ok := args["waitFor"].(bool); ok {
		attachArgs["waitFor"] = waitFor
	}

	// Attach to core dump
	if coreFile, ok := args["coreFile"].(string); ok {
		attachArgs["coreFile"] = coreFile
	}

	// Program path (for symbol resolution)
	if program, ok := args["program"].(string); ok {
		attachArgs["program"] = program
	}

	// Remote debugging via gdb-server protocol
	if port, ok := args["gdb-remote-port"].(float64); ok {
		attachArgs["gdb-remote-port"] = int(port)
	}
	if hostname, ok := args["gdb-remote-hostname"].(string); ok {
		attachArgs["gdb-remote-hostname"] = hostname
	}

	// LLDB-specific: attachCommands
	if attachCommands, ok := args["attachCommands"].([]interface{}); ok {
		cmds := make([]string, len(attachCommands))
		for i, c := range attachCommands {
			cmds[i] = fmt.Sprint(c)
		}
		attachArgs["attachCommands"] = cmds
	}

	return attachArgs
}
