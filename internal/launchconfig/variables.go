package launchconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Variable pattern matches ${...} expressions
var variablePattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ResolveVariables replaces all ${...} variables in the given text.
func ResolveVariables(text string, ctx *ResolutionContext) (string, error) {
	if ctx == nil {
		ctx = &ResolutionContext{}
	}

	var lastErr error
	result := variablePattern.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the variable expression (without ${ and })
		expr := match[2 : len(match)-1]

		resolved, err := resolveVariable(expr, ctx)
		if err != nil {
			lastErr = err
			return match // Keep original if error
		}
		return resolved
	})

	return result, lastErr
}

// resolveVariable resolves a single variable expression.
func resolveVariable(expr string, ctx *ResolutionContext) (string, error) {
	// Handle different variable types
	switch {
	case expr == "workspaceFolder":
		return ctx.WorkspaceFolder, nil

	case expr == "workspaceFolderBasename":
		return filepath.Base(ctx.WorkspaceFolder), nil

	case expr == "file":
		return ctx.CurrentFile, nil

	case expr == "fileBasename":
		return filepath.Base(ctx.CurrentFile), nil

	case expr == "fileDirname":
		return filepath.Dir(ctx.CurrentFile), nil

	case expr == "fileBasenameNoExtension":
		base := filepath.Base(ctx.CurrentFile)
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext), nil

	case expr == "fileExtname":
		return filepath.Ext(ctx.CurrentFile), nil

	case expr == "relativeFile":
		if ctx.WorkspaceFolder != "" && ctx.CurrentFile != "" {
			rel, err := filepath.Rel(ctx.WorkspaceFolder, ctx.CurrentFile)
			if err == nil {
				return rel, nil
			}
		}
		return ctx.CurrentFile, nil

	case expr == "relativeFileDirname":
		if ctx.WorkspaceFolder != "" && ctx.CurrentFile != "" {
			dir := filepath.Dir(ctx.CurrentFile)
			rel, err := filepath.Rel(ctx.WorkspaceFolder, dir)
			if err == nil {
				return rel, nil
			}
		}
		return filepath.Dir(ctx.CurrentFile), nil

	case expr == "lineNumber":
		return strconv.Itoa(ctx.LineNumber), nil

	case expr == "selectedText":
		return ctx.SelectedText, nil

	case expr == "userHome":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home: %w", err)
		}
		return home, nil

	case expr == "cwd":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get cwd: %w", err)
		}
		return cwd, nil

	case expr == "pathSeparator":
		return string(os.PathSeparator), nil

	case expr == "execPath":
		// Return the executable path (not typically useful in MCP context)
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("failed to get executable path: %w", err)
		}
		return exe, nil

	case strings.HasPrefix(expr, "env:"):
		// ${env:VAR_NAME}
		varName := strings.TrimPrefix(expr, "env:")
		// Check context overrides first
		if ctx.EnvOverrides != nil {
			if val, ok := ctx.EnvOverrides[varName]; ok {
				return val, nil
			}
		}
		return os.Getenv(varName), nil

	case strings.HasPrefix(expr, "config:"):
		// ${config:SETTING_ID} - VS Code setting
		// Limited support: try to read from .vscode/settings.json
		settingID := strings.TrimPrefix(expr, "config:")
		return resolveConfigVariable(settingID, ctx.WorkspaceFolder)

	case strings.HasPrefix(expr, "command:"):
		// ${command:COMMAND_ID} - Execute command and capture output
		commandID := strings.TrimPrefix(expr, "command:")
		return resolveCommandVariable(commandID, ctx)

	case strings.HasPrefix(expr, "input:"):
		// ${input:INPUT_ID} - User input
		inputID := strings.TrimPrefix(expr, "input:")
		if ctx.InputValues != nil {
			if val, ok := ctx.InputValues[inputID]; ok {
				return val, nil
			}
		}
		return "", fmt.Errorf("missing input value for ${input:%s}", inputID)

	default:
		return "", fmt.Errorf("unknown variable: ${%s}", expr)
	}
}

// resolveConfigVariable attempts to read a VS Code setting.
func resolveConfigVariable(settingID, workspaceFolder string) (string, error) {
	if workspaceFolder == "" {
		return "", fmt.Errorf("workspaceFolder required for ${config:} variables")
	}

	// Try to read .vscode/settings.json
	settingsPath := filepath.Join(workspaceFolder, ".vscode", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		// Settings file not found, return empty (VS Code would use default)
		return "", nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", fmt.Errorf("failed to parse settings.json: %w", err)
	}

	// Navigate the setting ID (e.g., "python.defaultInterpreterPath")
	parts := strings.Split(settingID, ".")
	var current interface{} = settings

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return "", nil // Setting not found
		}
	}

	if current == nil {
		return "", nil
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		// Return JSON representation for complex types
		data, _ := json.Marshal(v)
		return string(data), nil
	}
}

// resolveCommandVariable executes a shell command and captures its output.
func resolveCommandVariable(commandID string, ctx *ResolutionContext) (string, error) {
	// The commandID might be:
	// - A simple command name (e.g., "python.interpreterPath")
	// - A shell command to execute

	// For VS Code compatibility, certain commands have known behaviors
	switch commandID {
	case "python.interpreterPath":
		// Try common methods to find Python
		return findPythonPath(ctx)
	}

	// For other commands, execute as shell command
	cmd := exec.Command("sh", "-c", commandID)
	if ctx.WorkspaceFolder != "" {
		cmd.Dir = ctx.WorkspaceFolder
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command %q failed: %w (stderr: %s)", commandID, err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// findPythonPath attempts to locate the Python interpreter.
func findPythonPath(ctx *ResolutionContext) (string, error) {
	// Check for virtual environment in workspace
	if ctx.WorkspaceFolder != "" {
		venvPaths := []string{
			filepath.Join(ctx.WorkspaceFolder, "venv", "bin", "python"),
			filepath.Join(ctx.WorkspaceFolder, "venv", "bin", "python3"),
			filepath.Join(ctx.WorkspaceFolder, ".venv", "bin", "python"),
			filepath.Join(ctx.WorkspaceFolder, ".venv", "bin", "python3"),
		}
		for _, p := range venvPaths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	// Fall back to system Python
	for _, cmd := range []string{"python3", "python"} {
		if path, err := exec.LookPath(cmd); err == nil {
			return path, nil
		}
	}

	return "python3", nil // Default fallback
}

// ResolveStringField resolves variables in a single string field.
func ResolveStringField(value string, ctx *ResolutionContext) (string, error) {
	if value == "" {
		return "", nil
	}
	return ResolveVariables(value, ctx)
}

// ResolveStringSlice resolves variables in all strings in a slice.
func ResolveStringSlice(values []string, ctx *ResolutionContext) ([]string, error) {
	result := make([]string, len(values))
	for i, v := range values {
		resolved, err := ResolveVariables(v, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve element %d: %w", i, err)
		}
		result[i] = resolved
	}
	return result, nil
}

// ResolveStringMap resolves variables in all values (not keys) of a string map.
func ResolveStringMap(values map[string]string, ctx *ResolutionContext) (map[string]string, error) {
	if values == nil {
		return nil, nil
	}
	result := make(map[string]string, len(values))
	for k, v := range values {
		resolved, err := ResolveVariables(v, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve value for key %q: %w", k, err)
		}
		result[k] = resolved
	}
	return result, nil
}

// FindRequiredInputs scans a text for ${input:...} variables and returns their IDs.
func FindRequiredInputs(text string) []string {
	var inputs []string
	seen := make(map[string]bool)

	matches := variablePattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			expr := match[1]
			if strings.HasPrefix(expr, "input:") {
				inputID := strings.TrimPrefix(expr, "input:")
				if !seen[inputID] {
					seen[inputID] = true
					inputs = append(inputs, inputID)
				}
			}
		}
	}
	return inputs
}

// FindAllRequiredInputsInConfig scans all string fields in a configuration for ${input:} variables.
func FindAllRequiredInputsInConfig(cfg *DebugConfiguration) []string {
	var inputs []string
	seen := make(map[string]bool)

	addInputs := func(text string) {
		for _, id := range FindRequiredInputs(text) {
			if !seen[id] {
				seen[id] = true
				inputs = append(inputs, id)
			}
		}
	}

	// Check all string fields
	addInputs(cfg.Program)
	addInputs(cfg.Cwd)
	addInputs(cfg.WebRoot)
	addInputs(cfg.URL)
	addInputs(cfg.Console)
	addInputs(cfg.Module)
	addInputs(cfg.PythonPath)
	addInputs(cfg.Mode)
	addInputs(cfg.BuildFlags)
	addInputs(cfg.RuntimeExecutable)
	addInputs(cfg.Host)
	addInputs(cfg.DebugAdapterPath)

	// Check array fields
	for _, arg := range cfg.Args {
		addInputs(arg)
	}
	for _, arg := range cfg.RuntimeArgs {
		addInputs(arg)
	}

	// Check map fields
	for _, v := range cfg.Env {
		addInputs(v)
	}
	for _, v := range cfg.SourceMapPathOverrides {
		addInputs(v)
	}

	// Check Extra fields (convert to JSON and scan)
	if len(cfg.Extra) > 0 {
		if data, err := json.Marshal(cfg.Extra); err == nil {
			addInputs(string(data))
		}
	}

	return inputs
}

// ValidateInputsProvided checks if all required inputs are provided.
func ValidateInputsProvided(cfg *DebugConfiguration, inputValues map[string]string) []string {
	required := FindAllRequiredInputsInConfig(cfg)
	var missing []string
	for _, id := range required {
		if _, ok := inputValues[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}
