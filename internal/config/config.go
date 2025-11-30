// Package config provides configuration management for the DAP-MCP server.
//
// Configuration controls:
//   - Capability mode (readonly vs full): determines which tools are available
//   - Permission flags: control spawn, attach, modify, and execute operations
//   - Language-specific adapter settings: paths and flags for each debugger
//   - Safety limits: maximum sessions and session timeout
//
// Configuration can be loaded from a JSON file or use sensible defaults.
// The readonly mode exposes only inspection tools, while full mode enables
// all debugging capabilities including execution control.
package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"time"
)

// CapabilityMode defines the level of debugging capabilities exposed
type CapabilityMode string

const (
	ModeReadOnly CapabilityMode = "readonly" // Only inspect_* tools
	ModeFull     CapabilityMode = "full"     // All tools enabled
)

// Config holds the server configuration
type Config struct {
	// Capability levels
	Mode         CapabilityMode `json:"mode"`
	AllowSpawn   bool           `json:"allowSpawn"`
	AllowAttach  bool           `json:"allowAttach"`
	AllowModify  bool           `json:"allowModify"`
	AllowExecute bool           `json:"allowExecute"`

	// Language-specific adapter configs
	Adapters AdapterConfigs `json:"adapters"`

	// Limits for safety
	MaxSessions    int           `json:"maxSessions"`
	SessionTimeout time.Duration `json:"sessionTimeout"`
}

// AdapterConfigs holds configuration for each language adapter
type AdapterConfigs struct {
	Go     DelveConfig   `json:"go"`
	Python DebugpyConfig `json:"python"`
	Node   NodeConfig    `json:"node"`
	LLDB   LLDBConfig    `json:"lldb"`
	GDB    GDBConfig     `json:"gdb"`
}

// DelveConfig holds Delve-specific configuration
type DelveConfig struct {
	Path       string `json:"path"`
	BuildFlags string `json:"buildFlags"`
}

// DebugpyConfig holds debugpy-specific configuration
type DebugpyConfig struct {
	PythonPath string `json:"pythonPath"`
}

// NodeConfig holds Node.js-specific configuration
type NodeConfig struct {
	NodePath               string            `json:"nodePath"`
	JsDebugPath            string            `json:"jsDebugPath"` // Path to vscode-js-debug's dapDebugServer.js
	InspectBrk             bool              `json:"inspectBrk"`
	SourceMapPathOverrides map[string]string `json:"sourceMapPathOverrides"` // Custom source map path overrides for bundlers
}

// LLDBConfig holds LLDB-specific configuration
type LLDBConfig struct {
	Path string `json:"path"` // Path to lldb-dap binary (formerly lldb-vscode)
}

// GDBConfig holds GDB-specific configuration
type GDBConfig struct {
	Path string `json:"path"` // Path to gdb binary (requires GDB 14.1+ for DAP support)
}

// findLLDBDap searches for lldb-dap in common locations across platforms
func findLLDBDap() string {
	// Check PATH first
	if path, err := exec.LookPath("lldb-dap"); err == nil {
		return path
	}

	// Platform-specific search locations
	locations := []string{
		// macOS - Xcode Command Line Tools and Xcode.app
		"/Library/Developer/CommandLineTools/usr/bin/lldb-dap",
		"/Applications/Xcode.app/Contents/Developer/usr/bin/lldb-dap",
		"/opt/homebrew/bin/lldb-dap", // Homebrew on Apple Silicon
		"/usr/local/bin/lldb-dap",    // Homebrew on Intel Mac or manual install

		// Linux - LLVM/Clang package installations
		"/usr/bin/lldb-dap",
		"/usr/bin/lldb-dap-18", // Versioned binaries (Debian/Ubuntu)
		"/usr/bin/lldb-dap-17",
		"/usr/bin/lldb-dap-16",
		"/usr/lib/llvm-18/bin/lldb-dap",
		"/usr/lib/llvm-17/bin/lldb-dap",
		"/usr/lib/llvm-16/bin/lldb-dap",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Check for lldb-vscode (older name, pre-LLVM 16)
	if path, err := exec.LookPath("lldb-vscode"); err == nil {
		return path
	}

	// Fall back to default name (will fail if not in PATH, but provides clear error)
	return "lldb-dap"
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Mode:           ModeFull,
		AllowSpawn:     true,
		AllowAttach:    true,
		AllowModify:    true,
		AllowExecute:   true,
		MaxSessions:    10,
		SessionTimeout: 30 * time.Minute,
		Adapters: AdapterConfigs{
			Go: DelveConfig{
				Path: "dlv",
			},
			Python: DebugpyConfig{
				PythonPath: "python3",
			},
			Node: NodeConfig{
				NodePath:   "node",
				InspectBrk: true,
			},
			LLDB: LLDBConfig{
				Path: findLLDBDap(),
			},
			GDB: GDBConfig{
				Path: "gdb",
			},
		},
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// CanUseControlTools returns true if control tools are enabled
func (c *Config) CanUseControlTools() bool {
	return c.Mode == ModeFull
}

// CanSpawn returns true if spawning debug adapters is allowed
func (c *Config) CanSpawn() bool {
	return c.AllowSpawn
}

// CanAttach returns true if attaching to debug adapters is allowed
func (c *Config) CanAttach() bool {
	return c.AllowAttach
}

// CanModifyVariables returns true if variable modification is allowed
func (c *Config) CanModifyVariables() bool {
	return c.Mode == ModeFull && c.AllowModify
}

// CanEvaluate returns true if expression evaluation is allowed
func (c *Config) CanEvaluate() bool {
	return c.AllowExecute
}
