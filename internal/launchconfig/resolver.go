package launchconfig

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ResolvedConfiguration is a fully resolved configuration ready for use.
type ResolvedConfiguration struct {
	*DebugConfiguration

	// Resolved language (mapped from Type)
	Language string

	// Resolved target (for browser debugging)
	Target string
}

// ResolveConfiguration resolves all variables in a configuration.
func ResolveConfiguration(cfg *DebugConfiguration, ctx *ResolutionContext) (*ResolvedConfiguration, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	if ctx == nil {
		ctx = &ResolutionContext{}
	}

	// Check for missing input values first
	missingInputs := ValidateInputsProvided(cfg, ctx.InputValues)
	if len(missingInputs) > 0 {
		return nil, &MissingInputsError{Inputs: missingInputs}
	}

	// Create a copy of the configuration
	resolved := &DebugConfiguration{
		Type:           cfg.Type,
		Request:        cfg.Request,
		Name:           cfg.Name,
		StopOnEntry:    cfg.StopOnEntry,
		Port:           cfg.Port,
		ProcessID:      cfg.ProcessID,
		JustMyCode:     cfg.JustMyCode,
		Django:         cfg.Django,
		Jinja:          cfg.Jinja,
		RedirectOutput: cfg.RedirectOutput,
		SourceMaps:     cfg.SourceMaps,
		Presentation:   cfg.Presentation,
	}

	var err error

	// Resolve string fields
	resolved.Program, err = ResolveStringField(cfg.Program, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve program: %w", err)
	}

	resolved.Cwd, err = ResolveStringField(cfg.Cwd, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cwd: %w", err)
	}

	resolved.WebRoot, err = ResolveStringField(cfg.WebRoot, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve webRoot: %w", err)
	}

	resolved.URL, err = ResolveStringField(cfg.URL, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve url: %w", err)
	}

	resolved.Console, err = ResolveStringField(cfg.Console, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve console: %w", err)
	}

	resolved.Host, err = ResolveStringField(cfg.Host, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host: %w", err)
	}

	resolved.RuntimeExecutable, err = ResolveStringField(cfg.RuntimeExecutable, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtimeExecutable: %w", err)
	}

	resolved.Mode, err = ResolveStringField(cfg.Mode, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve mode: %w", err)
	}

	resolved.BuildFlags, err = ResolveStringField(cfg.BuildFlags, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve buildFlags: %w", err)
	}

	// Resolve python path (support both VS Code's "python" and debugpy's "pythonPath")
	// "python" takes precedence if both are provided
	resolved.Python, err = ResolveStringField(cfg.Python, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve python: %w", err)
	}
	resolved.PythonPath, err = ResolveStringField(cfg.PythonPath, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve pythonPath: %w", err)
	}

	resolved.Module, err = ResolveStringField(cfg.Module, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve module: %w", err)
	}

	resolved.DebugAdapterPath, err = ResolveStringField(cfg.DebugAdapterPath, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve debugAdapterPath: %w", err)
	}

	resolved.PreLaunchTask, err = ResolveStringField(cfg.PreLaunchTask, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preLaunchTask: %w", err)
	}

	resolved.PostDebugTask, err = ResolveStringField(cfg.PostDebugTask, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve postDebugTask: %w", err)
	}

	// Resolve array fields
	resolved.Args, err = ResolveStringSlice(cfg.Args, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve args: %w", err)
	}

	resolved.RuntimeArgs, err = ResolveStringSlice(cfg.RuntimeArgs, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtimeArgs: %w", err)
	}

	// Resolve map fields
	resolved.Env, err = ResolveStringMap(cfg.Env, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve env: %w", err)
	}

	resolved.SourceMapPathOverrides, err = ResolveStringMap(cfg.SourceMapPathOverrides, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve sourceMapPathOverrides: %w", err)
	}

	// Resolve Extra fields
	if cfg.Extra != nil {
		resolved.Extra, err = resolveExtraFields(cfg.Extra, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve extra fields: %w", err)
		}
	}

	return &ResolvedConfiguration{
		DebugConfiguration: resolved,
		Language:           resolved.GetLanguage(),
		Target:             resolved.GetTarget(),
	}, nil
}

// resolveExtraFields recursively resolves variables in extra fields.
func resolveExtraFields(extra map[string]interface{}, ctx *ResolutionContext) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(extra))

	for k, v := range extra {
		resolved, err := resolveValue(v, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve extra[%s]: %w", k, err)
		}
		result[k] = resolved
	}

	return result, nil
}

// resolveValue resolves variables in a value of any type.
func resolveValue(v interface{}, ctx *ResolutionContext) (interface{}, error) {
	switch val := v.(type) {
	case string:
		return ResolveVariables(val, ctx)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			resolved, err := resolveValue(item, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, item := range val {
			resolved, err := resolveValue(item, ctx)
			if err != nil {
				return nil, err
			}
			result[k] = resolved
		}
		return result, nil
	default:
		// Non-string types pass through unchanged (numbers, bools, nil)
		return v, nil
	}
}

// MissingInputsError is returned when required ${input:} values are not provided.
type MissingInputsError struct {
	Inputs []string
}

func (e *MissingInputsError) Error() string {
	return fmt.Sprintf("missing input values: %v", e.Inputs)
}

// IsMissingInputsError checks if an error is a MissingInputsError.
func IsMissingInputsError(err error) (*MissingInputsError, bool) {
	var e *MissingInputsError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// ToLaunchArgs converts a resolved configuration to a map suitable for the DAP launch request.
func (r *ResolvedConfiguration) ToLaunchArgs() map[string]interface{} {
	args := make(map[string]interface{})

	// Common fields
	if r.Program != "" {
		args["program"] = r.Program
	}
	if len(r.Args) > 0 {
		args["args"] = r.Args
	}
	if r.Cwd != "" {
		args["cwd"] = r.Cwd
	}
	if r.Env != nil {
		args["env"] = r.Env
	}
	args["stopOnEntry"] = r.StopOnEntry
	if r.Console != "" {
		args["console"] = r.Console
	}

	// Browser fields
	if r.URL != "" {
		args["url"] = r.URL
	}
	if r.WebRoot != "" {
		args["webRoot"] = r.WebRoot
	}

	// Node.js fields
	if r.RuntimeExecutable != "" {
		args["runtimeExecutable"] = r.RuntimeExecutable
	}
	if len(r.RuntimeArgs) > 0 {
		args["runtimeArgs"] = r.RuntimeArgs
	}

	// Go/Delve fields
	if r.Mode != "" {
		args["mode"] = r.Mode
	}
	if r.BuildFlags != "" {
		args["buildFlags"] = r.BuildFlags
	}

	// Python fields - output both "python" (VS Code) and "pythonPath" (debugpy) for compatibility
	// "python" takes precedence if both are set
	pythonInterpreter := r.Python
	if pythonInterpreter == "" {
		pythonInterpreter = r.PythonPath
	}
	if pythonInterpreter != "" {
		args["python"] = pythonInterpreter     // VS Code style (checked first by debugpy adapter)
		args["pythonPath"] = pythonInterpreter // debugpy style (backward compatibility)
	}
	if r.Module != "" {
		args["module"] = r.Module
	}
	if r.JustMyCode != nil {
		args["justMyCode"] = *r.JustMyCode
	}
	if r.Django {
		args["django"] = r.Django
	}
	if r.Jinja {
		args["jinja"] = r.Jinja
	}
	if r.RedirectOutput {
		args["redirectOutput"] = r.RedirectOutput
	}
	if r.DebugAdapterPath != "" {
		args["debugAdapterPath"] = r.DebugAdapterPath
	}

	// Source maps
	if r.SourceMaps != nil {
		args["sourceMaps"] = *r.SourceMaps
	}
	if r.SourceMapPathOverrides != nil {
		args["sourceMapPathOverrides"] = r.SourceMapPathOverrides
	}

	// Add Extra fields
	for k, v := range r.Extra {
		args[k] = v
	}

	return args
}

// ToAttachArgs converts a resolved configuration to a map suitable for the DAP attach request.
func (r *ResolvedConfiguration) ToAttachArgs() map[string]interface{} {
	args := make(map[string]interface{})

	// Connection fields
	if r.Host != "" {
		args["host"] = r.Host
	}
	if r.Port != 0 {
		args["port"] = r.Port
	}
	if r.ProcessID != 0 {
		args["processId"] = r.ProcessID
	}

	// Browser fields
	if r.URL != "" {
		args["url"] = r.URL
	}
	if r.WebRoot != "" {
		args["webRoot"] = r.WebRoot
	}

	// Target for browser debugging
	if r.Target != "" {
		args["target"] = r.Target
	}

	// Source maps
	if r.SourceMaps != nil {
		args["sourceMaps"] = *r.SourceMaps
	}
	if r.SourceMapPathOverrides != nil {
		args["sourceMapPathOverrides"] = r.SourceMapPathOverrides
	}

	// Add Extra fields
	for k, v := range r.Extra {
		args[k] = v
	}

	return args
}

// Clone creates a deep copy of the configuration.
func (cfg *DebugConfiguration) Clone() *DebugConfiguration {
	// Use JSON round-trip for deep copy
	data, _ := json.Marshal(cfg)
	var clone DebugConfiguration
	_ = json.Unmarshal(data, &clone) // Error ignored: unmarshal of our own marshaled data should not fail
	return &clone
}

// MergeOverrides applies override values to a configuration.
// This allows tool arguments to override values from launch.json.
func MergeOverrides(cfg *DebugConfiguration, overrides map[string]interface{}) *DebugConfiguration {
	if len(overrides) == 0 {
		return cfg
	}

	// Clone the configuration first
	result := cfg.Clone()

	// Apply overrides
	for k, v := range overrides {
		switch k {
		case "program":
			if s, ok := v.(string); ok {
				result.Program = s
			}
		case "args":
			if arr, ok := v.([]interface{}); ok {
				args := make([]string, len(arr))
				for i, item := range arr {
					if s, ok := item.(string); ok {
						args[i] = s
					}
				}
				result.Args = args
			} else if arr, ok := v.([]string); ok {
				result.Args = arr
			}
		case "cwd":
			if s, ok := v.(string); ok {
				result.Cwd = s
			}
		case "env":
			if m, ok := v.(map[string]string); ok {
				result.Env = m
			} else if m, ok := v.(map[string]interface{}); ok {
				env := make(map[string]string)
				for k, v := range m {
					if s, ok := v.(string); ok {
						env[k] = s
					}
				}
				result.Env = env
			}
		case "stopOnEntry":
			if b, ok := v.(bool); ok {
				result.StopOnEntry = b
			}
		case "webRoot":
			if s, ok := v.(string); ok {
				result.WebRoot = s
			}
		case "url":
			if s, ok := v.(string); ok {
				result.URL = s
			}
		default:
			// Add to Extra for unknown fields
			if result.Extra == nil {
				result.Extra = make(map[string]interface{})
			}
			result.Extra[k] = v
		}
	}

	return result
}
