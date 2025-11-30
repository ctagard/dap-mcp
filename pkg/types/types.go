// Package types defines shared data types used across the DAP-MCP server.
//
// This package provides type definitions for:
//   - Language: Supported programming languages (Go, Python, JavaScript, TypeScript)
//   - SessionStatus: Debug session states (initializing, running, stopped, terminated)
//   - Request types: LaunchRequest, AttachRequest, BreakpointRequest
//   - Info types: SessionInfo, ThreadInfo, StackFrame, Variable, Scope, etc.
//   - DebugSnapshot: Complete debug state for inspection
//
// These types are used throughout the codebase to maintain type safety
// and provide clear contracts between components.
package types

// Language represents a supported programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageRust       Language = "rust"
	LanguageC          Language = "c"
	LanguageCpp        Language = "cpp"
)

// SessionStatus represents the status of a debug session
type SessionStatus string

const (
	SessionStatusInitializing SessionStatus = "initializing"
	SessionStatusRunning      SessionStatus = "running"
	SessionStatusStopped      SessionStatus = "stopped"
	SessionStatusTerminated   SessionStatus = "terminated"
)

// LaunchRequest represents a request to launch a debug session
type LaunchRequest struct {
	Language    Language          `json:"language"`
	Program     string            `json:"program"`
	Args        []string          `json:"args,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	StopOnEntry bool              `json:"stopOnEntry,omitempty"`
}

// AttachRequest represents a request to attach to a debug session
type AttachRequest struct {
	Language Language `json:"language"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	PID      int      `json:"pid,omitempty"`
}

// SessionInfo represents information about a debug session
type SessionInfo struct {
	SessionID string        `json:"sessionId"`
	Language  Language      `json:"language"`
	Status    SessionStatus `json:"status"`
	PID       int           `json:"pid,omitempty"`
	Program   string        `json:"program,omitempty"`
}

// ThreadInfo represents information about a thread
type ThreadInfo struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// StackFrame represents a stack frame
type StackFrame struct {
	ID        int         `json:"id"`
	Name      string      `json:"name"`
	Source    *SourceInfo `json:"source,omitempty"`
	Line      int         `json:"line"`
	Column    int         `json:"column,omitempty"`
	EndLine   int         `json:"endLine,omitempty"`
	EndColumn int         `json:"endColumn,omitempty"`
}

// SourceInfo represents source file information
type SourceInfo struct {
	Name            string `json:"name,omitempty"`
	Path            string `json:"path,omitempty"`
	SourceReference int    `json:"sourceReference,omitempty"`
}

// Scope represents a variable scope
type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
	Expensive          bool   `json:"expensive,omitempty"`
}

// Variable represents a variable
type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
}

// Breakpoint represents a breakpoint
type Breakpoint struct {
	ID           int         `json:"id,omitempty"`
	Verified     bool        `json:"verified"`
	Message      string      `json:"message,omitempty"`
	Source       *SourceInfo `json:"source,omitempty"`
	Line         int         `json:"line,omitempty"`
	Column       int         `json:"column,omitempty"`
	EndLine      int         `json:"endLine,omitempty"`
	EndColumn    int         `json:"endColumn,omitempty"`
	Condition    string      `json:"condition,omitempty"`
	HitCondition string      `json:"hitCondition,omitempty"`
	LogMessage   string      `json:"logMessage,omitempty"`
}

// BreakpointRequest represents a request to set a breakpoint
type BreakpointRequest struct {
	Line         int    `json:"line"`
	Condition    string `json:"condition,omitempty"`
	HitCondition string `json:"hitCondition,omitempty"`
	LogMessage   string `json:"logMessage,omitempty"`
}

// EvaluateResult represents the result of evaluating an expression
type EvaluateResult struct {
	Result             string `json:"result"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
}

// DebugSnapshot represents a complete snapshot of debug state
type DebugSnapshot struct {
	SessionID string               `json:"sessionId"`
	Status    SessionStatus        `json:"status"`
	Threads   []ThreadInfo         `json:"threads"`
	Stacks    map[int][]StackFrame `json:"stacks"`              // threadId -> stack frames
	Scopes    map[int][]Scope      `json:"scopes"`              // frameId -> scopes
	Variables map[int][]Variable   `json:"variables,omitempty"` // variablesReference -> variables
}

// ModuleInfo represents information about a loaded module
type ModuleInfo struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path,omitempty"`
	Version        string `json:"version,omitempty"`
	SymbolStatus   string `json:"symbolStatus,omitempty"`
	SymbolFilePath string `json:"symbolFilePath,omitempty"`
}
