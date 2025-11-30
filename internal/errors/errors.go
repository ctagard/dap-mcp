// Package errors provides structured error types for the DAP-MCP server.
// These errors include helpful hints and suggestions that guide the LLM
// to correct course when something goes wrong.
package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
)

// ErrorCode represents a category of error for programmatic handling
type ErrorCode string

const (
	// Session errors
	CodeSessionNotFound     ErrorCode = "SESSION_NOT_FOUND"
	CodeSessionLimitReached ErrorCode = "SESSION_LIMIT_REACHED"
	CodeSessionNoClient     ErrorCode = "SESSION_NO_CLIENT"
	CodeSessionTerminated   ErrorCode = "SESSION_TERMINATED"

	// Adapter errors
	CodeAdapterNotSupported  ErrorCode = "ADAPTER_NOT_SUPPORTED"
	CodeAdapterSpawnFailed   ErrorCode = "ADAPTER_SPAWN_FAILED"
	CodeAdapterConnectFailed ErrorCode = "ADAPTER_CONNECT_FAILED"

	// DAP protocol errors
	CodeDAPInitFailed    ErrorCode = "DAP_INIT_FAILED"
	CodeDAPLaunchFailed  ErrorCode = "DAP_LAUNCH_FAILED"
	CodeDAPAttachFailed  ErrorCode = "DAP_ATTACH_FAILED"
	CodeDAPTimeout       ErrorCode = "DAP_TIMEOUT"
	CodeDAPProtocolError ErrorCode = "DAP_PROTOCOL_ERROR"

	// Parameter errors
	CodeMissingParameter ErrorCode = "MISSING_PARAMETER"
	CodeInvalidParameter ErrorCode = "INVALID_PARAMETER"
	CodeInvalidJSON      ErrorCode = "INVALID_JSON"

	// Permission errors
	CodePermissionDenied ErrorCode = "PERMISSION_DENIED"

	// Configuration errors
	CodeConfigNotFound ErrorCode = "CONFIG_NOT_FOUND"
	CodeConfigInvalid  ErrorCode = "CONFIG_INVALID"
	CodeMissingInputs  ErrorCode = "MISSING_INPUTS"

	// Runtime errors
	CodeBreakpointFailed ErrorCode = "BREAKPOINT_FAILED"
	CodeEvaluationFailed ErrorCode = "EVALUATION_FAILED"
	CodeStepFailed       ErrorCode = "STEP_FAILED"
	CodeNoThreads        ErrorCode = "NO_THREADS"
)

// DebugError is a structured error type that includes helpful information
// for the LLM to understand what went wrong and how to fix it.
type DebugError struct {
	// Code is a machine-readable error category
	Code ErrorCode `json:"code"`

	// Message is a human/LLM-readable description of what went wrong
	Message string `json:"message"`

	// Hint provides actionable guidance on how to fix the error
	Hint string `json:"hint,omitempty"`

	// Details contains additional context (e.g., the invalid value, expected format)
	Details map[string]interface{} `json:"details,omitempty"`

	// Cause is the underlying error, if any
	Cause error `json:"-"`
}

// Error implements the error interface
func (e *DebugError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)

	if e.Hint != "" {
		sb.WriteString(" | Hint: ")
		sb.WriteString(e.Hint)
	}

	return sb.String()
}

// Unwrap returns the underlying error for error chaining
func (e *DebugError) Unwrap() error {
	return e.Cause
}

// WithDetails adds details to the error
func (e *DebugError) WithDetails(key string, value interface{}) *DebugError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithCause sets the underlying cause
func (e *DebugError) WithCause(err error) *DebugError {
	e.Cause = err
	return e
}

// --- Session Errors ---

// SessionNotFound creates an error for when a session ID doesn't exist
func SessionNotFound(sessionID string) *DebugError {
	return &DebugError{
		Code:    CodeSessionNotFound,
		Message: fmt.Sprintf("session '%s' not found", sessionID),
		Hint:    "Use debug_list_sessions to see active sessions, or use debug_launch to create a new session.",
		Details: map[string]interface{}{
			"sessionId": sessionID,
		},
	}
}

// SessionLimitReached creates an error when max sessions is reached
func SessionLimitReached(maxSessions int) *DebugError {
	return &DebugError{
		Code:    CodeSessionLimitReached,
		Message: fmt.Sprintf("maximum number of sessions (%d) reached", maxSessions),
		Hint:    "Use debug_disconnect to terminate an existing session before creating a new one.",
		Details: map[string]interface{}{
			"maxSessions": maxSessions,
		},
	}
}

// SessionNoClient creates an error when a session has no active client
func SessionNoClient(sessionID string) *DebugError {
	return &DebugError{
		Code:    CodeSessionNoClient,
		Message: fmt.Sprintf("session '%s' has no active debug client", sessionID),
		Hint:    "The session may have been terminated or failed to initialize. Use debug_disconnect to clean up and debug_launch to create a new session.",
		Details: map[string]interface{}{
			"sessionId": sessionID,
		},
	}
}

// --- Adapter Errors ---

// AdapterNotSupported creates an error for unsupported languages
func AdapterNotSupported(language string, supported []string) *DebugError {
	return &DebugError{
		Code:    CodeAdapterNotSupported,
		Message: fmt.Sprintf("no debug adapter available for language: %s", language),
		Hint:    fmt.Sprintf("Supported languages are: %s. Check that the language parameter is correct.", strings.Join(supported, ", ")),
		Details: map[string]interface{}{
			"requestedLanguage":  language,
			"supportedLanguages": supported,
		},
	}
}

// AdapterSpawnFailed creates an error when adapter spawn fails
func AdapterSpawnFailed(language string, err error) *DebugError {
	return &DebugError{
		Code:    CodeAdapterSpawnFailed,
		Message: fmt.Sprintf("failed to spawn debug adapter for %s: %v", language, err),
		Hint:    "Ensure the debug adapter is installed. For Go: install Delve (go install github.com/go-delve/delve/cmd/dlv@latest). For Python: install debugpy (pip install debugpy). For JavaScript: vscode-js-debug should be bundled.",
		Cause:   err,
		Details: map[string]interface{}{
			"language": language,
		},
	}
}

// AdapterConnectFailed creates an error when connecting to adapter fails
func AdapterConnectFailed(address string, err error) *DebugError {
	return &DebugError{
		Code:    CodeAdapterConnectFailed,
		Message: fmt.Sprintf("failed to connect to debug adapter at %s: %v", address, err),
		Hint:    "The debug adapter may have failed to start or crashed. Check that the program path is correct and the file exists.",
		Cause:   err,
		Details: map[string]interface{}{
			"address": address,
		},
	}
}

// --- DAP Protocol Errors ---

// DAPInitFailed creates an error for DAP initialization failures
func DAPInitFailed(err error) *DebugError {
	return &DebugError{
		Code:    CodeDAPInitFailed,
		Message: fmt.Sprintf("debug adapter initialization failed: %v", err),
		Hint:    "The debug adapter may be incompatible or crashed during startup. Try disconnecting and launching a new session.",
		Cause:   err,
	}
}

// DAPLaunchFailed creates an error for launch failures
func DAPLaunchFailed(program string, err error) *DebugError {
	return &DebugError{
		Code:    CodeDAPLaunchFailed,
		Message: fmt.Sprintf("failed to launch program: %v", err),
		Hint:    "Check that the program path is correct and the file exists. For compiled languages, ensure the program compiles without errors.",
		Cause:   err,
		Details: map[string]interface{}{
			"program": program,
		},
	}
}

// DAPAttachFailed creates an error for attach failures
func DAPAttachFailed(err error) *DebugError {
	return &DebugError{
		Code:    CodeDAPAttachFailed,
		Message: fmt.Sprintf("failed to attach to process: %v", err),
		Hint:    "Ensure the target process is running and listening on the specified port. For Node.js, the process should be started with --inspect flag.",
		Cause:   err,
	}
}

// DAPTimeout creates an error for DAP timeouts
func DAPTimeout(operation string, timeoutSeconds int) *DebugError {
	return &DebugError{
		Code:    CodeDAPTimeout,
		Message: fmt.Sprintf("%s timed out after %d seconds", operation, timeoutSeconds),
		Hint:    "The operation took too long. The program may be stuck, in an infinite loop, or waiting for input. Try using debug_pause to interrupt execution.",
		Details: map[string]interface{}{
			"operation":      operation,
			"timeoutSeconds": timeoutSeconds,
		},
	}
}

// --- Parameter Errors ---

// MissingParameter creates an error for missing required parameters
func MissingParameter(paramName, description string) *DebugError {
	return &DebugError{
		Code:    CodeMissingParameter,
		Message: fmt.Sprintf("required parameter '%s' is missing", paramName),
		Hint:    description,
		Details: map[string]interface{}{
			"parameter": paramName,
		},
	}
}

// InvalidParameter creates an error for invalid parameter values
func InvalidParameter(paramName string, value interface{}, expected string) *DebugError {
	return &DebugError{
		Code:    CodeInvalidParameter,
		Message: fmt.Sprintf("invalid value for parameter '%s': %v", paramName, value),
		Hint:    fmt.Sprintf("Expected: %s", expected),
		Details: map[string]interface{}{
			"parameter": paramName,
			"value":     value,
			"expected":  expected,
		},
	}
}

// InvalidJSON creates an error for JSON parsing failures
func InvalidJSON(paramName string, err error, example string) *DebugError {
	return &DebugError{
		Code:    CodeInvalidJSON,
		Message: fmt.Sprintf("invalid JSON in parameter '%s': %v", paramName, err),
		Hint:    fmt.Sprintf("Provide valid JSON. Example: %s", example),
		Cause:   err,
		Details: map[string]interface{}{
			"parameter": paramName,
			"example":   example,
		},
	}
}

// --- Permission Errors ---

// PermissionDenied creates an error for permission denied
func PermissionDenied(operation, mode string) *DebugError {
	var hint string
	switch operation {
	case "spawn":
		hint = "The server is configured to disallow spawning debug adapters. Ask the administrator to enable 'allowSpawn' in the configuration."
	case "attach":
		hint = "The server is configured to disallow attaching to processes. Ask the administrator to enable 'allowAttach' in the configuration."
	case "evaluate":
		hint = "Expression evaluation is disabled in the current server mode. This may be intentional for security reasons."
	case "modify":
		hint = "Variable modification is disabled in the current server mode. The server may be in read-only mode."
	default:
		hint = fmt.Sprintf("This operation is not allowed in '%s' mode.", mode)
	}

	return &DebugError{
		Code:    CodePermissionDenied,
		Message: fmt.Sprintf("%s is not allowed in current server mode", operation),
		Hint:    hint,
		Details: map[string]interface{}{
			"operation": operation,
			"mode":      mode,
		},
	}
}

// --- Configuration Errors ---

// ConfigNotFound creates an error for missing launch.json configurations
func ConfigNotFound(configName string, availableConfigs []string) *DebugError {
	var hint string
	if len(availableConfigs) > 0 {
		hint = fmt.Sprintf("Available configurations: %s", strings.Join(availableConfigs, ", "))
	} else {
		hint = "No configurations found in launch.json. Create a launch configuration first."
	}

	return &DebugError{
		Code:    CodeConfigNotFound,
		Message: fmt.Sprintf("configuration '%s' not found in launch.json", configName),
		Hint:    hint,
		Details: map[string]interface{}{
			"configName":       configName,
			"availableConfigs": availableConfigs,
		},
	}
}

// ConfigInvalid creates an error for invalid configuration
func ConfigInvalid(configName, reason string) *DebugError {
	return &DebugError{
		Code:    CodeConfigInvalid,
		Message: fmt.Sprintf("configuration '%s' is invalid: %s", configName, reason),
		Hint:    "Check the launch.json file for syntax errors and ensure all required fields are present.",
		Details: map[string]interface{}{
			"configName": configName,
			"reason":     reason,
		},
	}
}

// MissingInputs creates an error for missing input values
func MissingInputs(inputs []string) *DebugError {
	return &DebugError{
		Code:    CodeMissingInputs,
		Message: fmt.Sprintf("missing required input values: %s", strings.Join(inputs, ", ")),
		Hint:    "Provide the missing values via the inputValues parameter as a JSON object, e.g., {\"inputName\": \"value\"}",
		Details: map[string]interface{}{
			"missingInputs": inputs,
		},
	}
}

// --- Runtime Errors ---

// BreakpointFailed creates an error for breakpoint failures
func BreakpointFailed(path string, line int, reason string) *DebugError {
	return &DebugError{
		Code:    CodeBreakpointFailed,
		Message: fmt.Sprintf("could not set breakpoint at %s:%d", path, line),
		Hint:    fmt.Sprintf("Reason: %s. Ensure the file path is correct and the line number contains executable code (not comments or blank lines).", reason),
		Details: map[string]interface{}{
			"path":   path,
			"line":   line,
			"reason": reason,
		},
	}
}

// EvaluationFailed creates an error for expression evaluation failures
func EvaluationFailed(expression string, err error) *DebugError {
	return &DebugError{
		Code:    CodeEvaluationFailed,
		Message: fmt.Sprintf("failed to evaluate expression '%s': %v", expression, err),
		Hint:    "Check that the expression syntax is correct for the target language and that referenced variables are in scope.",
		Cause:   err,
		Details: map[string]interface{}{
			"expression": expression,
		},
	}
}

// StepFailed creates an error for step failures
func StepFailed(stepType string, err error) *DebugError {
	var hint string
	switch stepType {
	case "over":
		hint = "Step over failed. The program may have terminated or hit an error. Use debug_snapshot to check the current state."
	case "into":
		hint = "Step into failed. There may be no function call on the current line, or the program has terminated."
	case "out":
		hint = "Step out failed. You may already be at the top of the call stack, or the program has terminated."
	default:
		hint = "The step operation failed. Use debug_snapshot to check the current program state."
	}

	return &DebugError{
		Code:    CodeStepFailed,
		Message: fmt.Sprintf("step %s failed: %v", stepType, err),
		Hint:    hint,
		Cause:   err,
		Details: map[string]interface{}{
			"stepType": stepType,
		},
	}
}

// NoThreads creates an error when no threads are available
func NoThreads() *DebugError {
	return &DebugError{
		Code:    CodeNoThreads,
		Message: "no threads available",
		Hint:    "The program may have terminated or not started yet. Use debug_snapshot to check the session status.",
	}
}

// --- Helper for wrapping generic errors ---

// Wrap wraps a generic error with context
func Wrap(code ErrorCode, message string, hint string, err error) *DebugError {
	return &DebugError{
		Code:    code,
		Message: message,
		Hint:    hint,
		Cause:   err,
	}
}

// FromError creates a DebugError from a generic error, attempting to preserve any existing structure
func FromError(err error) *DebugError {
	var de *DebugError
	if stderrors.As(err, &de) {
		return de
	}
	return &DebugError{
		Code:    "UNKNOWN_ERROR",
		Message: err.Error(),
		Hint:    "An unexpected error occurred. Please check the error message for details.",
		Cause:   err,
	}
}
