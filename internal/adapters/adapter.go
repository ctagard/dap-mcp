// Package adapters provides language-specific debug adapter implementations.
//
// This package defines the Adapter interface that all language-specific debuggers
// must implement, and provides concrete implementations for:
//   - Go (via Delve)
//   - Python (via debugpy)
//   - JavaScript/TypeScript (via vscode-js-debug for both Node.js and browser targets)
//
// The Registry type manages the collection of available adapters and provides
// lookup by language. Adapters handle spawning debug adapter processes and
// building the appropriate launch/attach arguments for each debugger.
package adapters

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/internal/dap"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// Adapter defines the interface for language-specific debug adapters
type Adapter interface {
	// Language returns the language this adapter supports
	Language() types.Language

	// Spawn starts a debug adapter process and returns the address to connect to
	// For TCP-based adapters, returns the address to connect to.
	// For stdio-based adapters (IsStdio() == true), returns empty address.
	Spawn(ctx context.Context, program string, args map[string]interface{}) (address string, cmd *exec.Cmd, err error)

	// BuildLaunchArgs builds the launch arguments for the debug adapter
	BuildLaunchArgs(program string, args map[string]interface{}) map[string]interface{}

	// BuildAttachArgs builds the attach arguments for the debug adapter
	BuildAttachArgs(args map[string]interface{}) map[string]interface{}
}

// StdioAdapter extends Adapter for adapters that communicate via stdin/stdout
// instead of TCP sockets (e.g., lldb-dap, gdb --interpreter=dap)
type StdioAdapter interface {
	Adapter

	// IsStdio returns true if this adapter uses stdio transport
	IsStdio() bool

	// SpawnStdio starts a debug adapter process and returns a DAP client
	// connected via the process's stdin/stdout pipes
	SpawnStdio(ctx context.Context, program string, args map[string]interface{}) (client *dap.Client, cmd *exec.Cmd, err error)
}

// Registry holds all registered adapters
type Registry struct {
	adapters map[types.Language]Adapter
}

// NewRegistry creates a new adapter registry with all supported adapters
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		adapters: make(map[types.Language]Adapter),
	}

	// Register Go adapter
	r.adapters[types.LanguageGo] = NewDelveAdapter(cfg.Adapters.Go)

	// Register Python adapter
	r.adapters[types.LanguagePython] = NewDebugpyAdapter(cfg.Adapters.Python)

	// Register JavaScript/TypeScript adapters
	nodeAdapter := NewNodeAdapter(cfg.Adapters.Node)
	r.adapters[types.LanguageJavaScript] = nodeAdapter
	r.adapters[types.LanguageTypeScript] = nodeAdapter

	// Register LLDB adapter for native languages (C, C++, Rust)
	// LLDB is preferred on macOS and also works well on Linux
	lldbAdapter := NewLLDBAdapter(cfg.Adapters.LLDB)
	r.adapters[types.LanguageC] = lldbAdapter
	r.adapters[types.LanguageCpp] = lldbAdapter
	r.adapters[types.LanguageRust] = lldbAdapter

	// GDB adapter is available as an alternative via explicit configuration
	// Users can override the default LLDB adapter by specifying gdb in launch.json
	// or by modifying the registry after creation

	return r
}

// Get returns the adapter for a language
func (r *Registry) Get(lang types.Language) (Adapter, error) {
	adapter, ok := r.adapters[lang]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for language: %s", lang)
	}
	return adapter, nil
}

// Register registers an adapter for a language, overriding any existing adapter
func (r *Registry) Register(lang types.Language, adapter Adapter) {
	r.adapters[lang] = adapter
}

// GetGDBAdapter returns a GDB adapter (useful when user explicitly wants GDB over LLDB)
func (r *Registry) GetGDBAdapter(cfg config.GDBConfig) *GDBAdapter {
	return NewGDBAdapter(cfg)
}

// GetLLDBAdapter returns an LLDB adapter
func (r *Registry) GetLLDBAdapter(cfg config.LLDBConfig) *LLDBAdapter {
	return NewLLDBAdapter(cfg)
}

// Connect creates a DAP client connected to the given address via TCP
func Connect(address string, maxRetries int) (*dap.Client, error) {
	var transport *dap.Transport
	var err error

	for i := 0; i < maxRetries; i++ {
		transport, err = dap.NewTCPTransport(address)
		if err == nil {
			break
		}
		// Increase delay between retries to give the adapter more time
		time.Sleep(200 * time.Millisecond)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to debug adapter at %s: %w", address, err)
	}

	client := dap.NewClient(transport)
	return client, nil
}

// SpawnAndConnect spawns an adapter and returns a connected client.
// For stdio-based adapters, it connects via stdin/stdout pipes.
// For TCP-based adapters, it connects via the returned address.
func SpawnAndConnect(ctx context.Context, adapter Adapter, program string, args map[string]interface{}) (*dap.Client, *exec.Cmd, error) {
	// Check if this is a stdio-based adapter
	if stdioAdapter, ok := adapter.(StdioAdapter); ok && stdioAdapter.IsStdio() {
		return stdioAdapter.SpawnStdio(ctx, program, args)
	}

	// TCP-based adapter
	address, cmd, err := adapter.Spawn(ctx, program, args)
	if err != nil {
		return nil, nil, err
	}

	// Connect to the adapter (20 retries * 200ms = 4 seconds max wait)
	client, err := Connect(address, 20)
	if err != nil {
		// Kill the spawned process if we can't connect
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill() // Error ignored: best-effort cleanup
		}
		return nil, nil, err
	}

	return client, cmd, nil
}

// findAvailablePort finds an available TCP port
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
