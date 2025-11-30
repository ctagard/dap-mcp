// Package launchconfig provides support for VS Code launch.json debug configurations.
package launchconfig

import (
	"encoding/json"
)

// LaunchJSON represents a VS Code launch.json file structure.
type LaunchJSON struct {
	Version        string               `json:"version"`
	Configurations []DebugConfiguration `json:"configurations"`
	Compounds      []CompoundConfig     `json:"compounds,omitempty"`
	Inputs         []InputConfig        `json:"inputs,omitempty"`
}

// DebugConfiguration represents a single debug configuration in launch.json.
type DebugConfiguration struct {
	// Required fields
	Type    string `json:"type"`    // e.g., "python", "go", "node", "chrome"
	Request string `json:"request"` // "launch" or "attach"
	Name    string `json:"name"`    // Human-readable name

	// Common optional fields
	Program     string            `json:"program,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	StopOnEntry bool              `json:"stopOnEntry,omitempty"`
	Console     string            `json:"console,omitempty"`

	// Attach-specific fields
	Port      int    `json:"port,omitempty"`
	Host      string `json:"host,omitempty"`
	ProcessID int    `json:"processId,omitempty"`

	// Browser debugging fields
	URL     string `json:"url,omitempty"`
	WebRoot string `json:"webRoot,omitempty"`

	// Node.js specific
	RuntimeExecutable string   `json:"runtimeExecutable,omitempty"`
	RuntimeArgs       []string `json:"runtimeArgs,omitempty"`

	// Go/Delve specific
	Mode       string `json:"mode,omitempty"`
	BuildFlags string `json:"buildFlags,omitempty"`

	// LLDB/lldb-dap specific
	InitCommands    []string          `json:"initCommands,omitempty"`    // Commands run before target creation
	PreRunCommands  []string          `json:"preRunCommands,omitempty"`  // Commands run before launch/attach
	StopCommands    []string          `json:"stopCommands,omitempty"`    // Commands run after each stop
	ExitCommands    []string          `json:"exitCommands,omitempty"`    // Commands run when program exits
	AttachCommands  []string          `json:"attachCommands,omitempty"`  // Custom attach commands
	LaunchCommands  []string          `json:"launchCommands,omitempty"`  // Custom launch commands
	CoreFile        string            `json:"coreFile,omitempty"`        // Core dump file for post-mortem debugging
	SourceMap       [][]string        `json:"sourceMap,omitempty"`       // Source path remapping [[from, to], ...]
	WaitFor         bool              `json:"waitFor,omitempty"`         // Wait for process to launch

	// GDB specific
	StopAtBeginningOfMainSubprogram bool   `json:"stopAtBeginningOfMainSubprogram,omitempty"` // Stop at main()
	MIMode                          string `json:"MIMode,omitempty"`                          // "gdb" or "lldb" for cppdbg
	MIDebuggerPath                  string `json:"miDebuggerPath,omitempty"`                  // Path to debugger
	TargetRemote                    string `json:"target,omitempty"`                          // Remote target for gdbserver

	// Python/debugpy specific
	Python          string `json:"python,omitempty"`     // VS Code style (preferred)
	PythonPath      string `json:"pythonPath,omitempty"` // debugpy style (legacy)
	Module          string `json:"module,omitempty"`
	JustMyCode      *bool  `json:"justMyCode,omitempty"`
	Django          bool   `json:"django,omitempty"`
	Jinja           bool   `json:"jinja,omitempty"`
	RedirectOutput  bool   `json:"redirectOutput,omitempty"`
	DebugAdapterPath string `json:"debugAdapterPath,omitempty"`

	// Source map configuration
	SourceMaps             *bool             `json:"sourceMaps,omitempty"`
	SourceMapPathOverrides map[string]string `json:"sourceMapPathOverrides,omitempty"`

	// Task integration
	PreLaunchTask  string `json:"preLaunchTask,omitempty"`
	PostDebugTask  string `json:"postDebugTask,omitempty"`

	// Presentation hints
	Presentation *PresentationConfig `json:"presentation,omitempty"`

	// All other properties not explicitly defined (language-specific extras)
	Extra map[string]interface{} `json:"-"`
}

// CompoundConfig represents a compound configuration that launches multiple debug sessions.
type CompoundConfig struct {
	Name           string   `json:"name"`
	Configurations []string `json:"configurations"`
	PreLaunchTask  string   `json:"preLaunchTask,omitempty"`
	StopAll        bool     `json:"stopAll,omitempty"`
	Presentation   *PresentationConfig `json:"presentation,omitempty"`
}

// InputConfig represents a user input variable definition.
type InputConfig struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`        // "promptString", "pickString", "command"
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"` // For pickString
	Command     string   `json:"command,omitempty"` // For command type
	Args        interface{} `json:"args,omitempty"`    // For command type
}

// PresentationConfig controls how the configuration appears in VS Code UI.
type PresentationConfig struct {
	Hidden bool   `json:"hidden,omitempty"`
	Group  string `json:"group,omitempty"`
	Order  int    `json:"order,omitempty"`
}

// ResolutionContext provides context for variable resolution.
type ResolutionContext struct {
	WorkspaceFolder string            // Root folder of the workspace
	CurrentFile     string            // Currently active file (for ${file} variables)
	LineNumber      int               // Current line number (for ${lineNumber})
	SelectedText    string            // Currently selected text (for ${selectedText})
	InputValues     map[string]string // Pre-provided values for ${input:} variables
	EnvOverrides    map[string]string // Override environment variables
}

// UnmarshalJSON implements custom unmarshaling to capture unknown fields.
func (c *DebugConfiguration) UnmarshalJSON(data []byte) error {
	// First unmarshal into a map to capture all fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Define known fields for type aliasing trick
	type Alias DebugConfiguration
	var alias Alias

	// Unmarshal into the alias (handles known fields)
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	*c = DebugConfiguration(alias)

	// Known fields to exclude from Extra
	knownFields := map[string]bool{
		"type": true, "request": true, "name": true,
		"program": true, "args": true, "cwd": true, "env": true,
		"stopOnEntry": true, "console": true,
		"port": true, "host": true, "processId": true,
		"url": true, "webRoot": true,
		"runtimeExecutable": true, "runtimeArgs": true,
		"mode": true, "buildFlags": true,
		// LLDB/lldb-dap specific
		"initCommands": true, "preRunCommands": true, "stopCommands": true,
		"exitCommands": true, "attachCommands": true, "launchCommands": true,
		"coreFile": true, "sourceMap": true, "waitFor": true,
		// GDB specific
		"stopAtBeginningOfMainSubprogram": true, "MIMode": true,
		"miDebuggerPath": true, "target": true,
		// Python/debugpy specific
		"python": true, "pythonPath": true, "module": true, "justMyCode": true,
		"django": true, "jinja": true, "redirectOutput": true,
		"debugAdapterPath": true,
		"sourceMaps": true, "sourceMapPathOverrides": true,
		"preLaunchTask": true, "postDebugTask": true,
		"presentation": true,
	}

	// Capture unknown fields into Extra
	c.Extra = make(map[string]interface{})
	for key, value := range raw {
		if !knownFields[key] {
			var v interface{}
			if err := json.Unmarshal(value, &v); err != nil {
				return err
			}
			c.Extra[key] = v
		}
	}

	return nil
}

// MarshalJSON implements custom marshaling to include Extra fields.
func (c DebugConfiguration) MarshalJSON() ([]byte, error) {
	type Alias DebugConfiguration
	alias := Alias(c)

	// Marshal the known fields
	data, err := json.Marshal(alias)
	if err != nil {
		return nil, err
	}

	// If no extra fields, return as-is
	if len(c.Extra) == 0 {
		return data, nil
	}

	// Merge extra fields
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	for k, v := range c.Extra {
		m[k] = v
	}

	return json.Marshal(m)
}

// TypeToLanguage maps VS Code debug types to dap-mcp language identifiers.
var TypeToLanguage = map[string]string{
	// Python
	"python":  "python",
	"debugpy": "python",

	// Go
	"go": "go",

	// JavaScript/TypeScript/Node.js
	"node":     "javascript",
	"pwa-node": "javascript",

	// Browser debugging
	"chrome":     "javascript",
	"pwa-chrome": "javascript",
	"msedge":     "javascript",
	"pwa-msedge": "javascript",

	// C/C++/Rust via LLDB
	"lldb":     "c",     // Generic LLDB
	"lldb-dap": "c",     // LLDB DAP server
	"codelldb": "c",     // CodeLLDB extension

	// C/C++/Rust via GDB
	"gdb":    "c",       // Native GDB DAP (GDB 14.1+)
	"cppdbg": "cpp",     // Microsoft cpptools (GDB/LLDB via MI)

	// Explicit language types
	"c":    "c",
	"cpp":  "cpp",
	"rust": "rust",
}

// IsLaunchRequest returns true if this is a launch configuration (not attach).
func (c *DebugConfiguration) IsLaunchRequest() bool {
	return c.Request == "launch"
}

// IsAttachRequest returns true if this is an attach configuration.
func (c *DebugConfiguration) IsAttachRequest() bool {
	return c.Request == "attach"
}

// IsBrowserTarget returns true if this targets a browser (Chrome/Edge).
func (c *DebugConfiguration) IsBrowserTarget() bool {
	switch c.Type {
	case "chrome", "pwa-chrome", "msedge", "pwa-msedge":
		return true
	}
	return false
}

// GetLanguage returns the dap-mcp language identifier for this configuration.
func (c *DebugConfiguration) GetLanguage() string {
	if lang, ok := TypeToLanguage[c.Type]; ok {
		return lang
	}
	// Default to the type itself if not mapped
	return c.Type
}

// GetTarget returns the debug target type (node, chrome, edge) for browser configurations.
func (c *DebugConfiguration) GetTarget() string {
	switch c.Type {
	case "chrome", "pwa-chrome":
		return "chrome"
	case "msedge", "pwa-msedge":
		return "edge"
	case "node", "pwa-node":
		return "node"
	}
	return ""
}

// IsNativeLanguage returns true if this configuration targets a native language (C, C++, Rust).
func (c *DebugConfiguration) IsNativeLanguage() bool {
	switch c.Type {
	case "lldb", "lldb-dap", "codelldb", "gdb", "cppdbg", "c", "cpp", "rust":
		return true
	}
	return false
}

// IsLLDBType returns true if this configuration uses LLDB-based debugging.
func (c *DebugConfiguration) IsLLDBType() bool {
	switch c.Type {
	case "lldb", "lldb-dap", "codelldb":
		return true
	}
	// cppdbg can use either GDB or LLDB based on MIMode
	if c.Type == "cppdbg" && c.MIMode == "lldb" {
		return true
	}
	return false
}

// IsGDBType returns true if this configuration uses GDB-based debugging.
func (c *DebugConfiguration) IsGDBType() bool {
	switch c.Type {
	case "gdb":
		return true
	}
	// cppdbg uses GDB by default or when MIMode is "gdb"
	if c.Type == "cppdbg" && (c.MIMode == "" || c.MIMode == "gdb") {
		return true
	}
	return false
}

// GetNativeDebugger returns the preferred debugger for native language configurations.
// Returns "lldb", "gdb", or empty string if not a native configuration.
func (c *DebugConfiguration) GetNativeDebugger() string {
	if c.IsLLDBType() {
		return "lldb"
	}
	if c.IsGDBType() {
		return "gdb"
	}
	// For explicit language types without a specified debugger, prefer LLDB
	switch c.Type {
	case "c", "cpp", "rust":
		return "lldb"
	}
	return ""
}
