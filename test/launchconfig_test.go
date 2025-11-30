package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctagard/dap-mcp/internal/launchconfig"
)

// TestLoadFromPath verifies that launch.json files can be loaded and parsed correctly.
func TestLoadFromPath(t *testing.T) {
	// Create a temporary launch.json
	tmpDir := t.TempDir()
	vscodeDir := filepath.Join(tmpDir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatalf("failed to create .vscode dir: %v", err)
	}

	launchJSON := `{
		"version": "0.2.0",
		"configurations": [
			{
				"type": "python",
				"request": "launch",
				"name": "Python: Current File",
				"program": "${file}",
				"console": "integratedTerminal"
			},
			{
				"type": "go",
				"request": "launch",
				"name": "Go: Debug",
				"program": "${workspaceFolder}",
				"mode": "debug"
			}
		],
		"compounds": [
			{
				"name": "Full Stack",
				"configurations": ["Python: Current File", "Go: Debug"],
				"stopAll": true
			}
		]
	}`

	launchPath := filepath.Join(vscodeDir, "launch.json")
	if err := os.WriteFile(launchPath, []byte(launchJSON), 0644); err != nil {
		t.Fatalf("failed to write launch.json: %v", err)
	}

	// Test loading
	lj, err := launchconfig.LoadFromPath(launchPath)
	if err != nil {
		t.Fatalf("LoadFromPath failed: %v", err)
	}

	if lj.Version != "0.2.0" {
		t.Errorf("expected version 0.2.0, got %s", lj.Version)
	}

	if len(lj.Configurations) != 2 {
		t.Errorf("expected 2 configurations, got %d", len(lj.Configurations))
	}

	if len(lj.Compounds) != 1 {
		t.Errorf("expected 1 compound, got %d", len(lj.Compounds))
	}
}

// TestLoadFromPath_InvalidJSON verifies error handling for malformed JSON.
func TestLoadFromPath_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	launchPath := filepath.Join(tmpDir, "launch.json")

	// Write invalid JSON
	if err := os.WriteFile(launchPath, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("failed to write launch.json: %v", err)
	}

	_, err := launchconfig.LoadFromPath(launchPath)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestLoadFromPath_NonExistent verifies error handling for missing files.
func TestLoadFromPath_NonExistent(t *testing.T) {
	_, err := launchconfig.LoadFromPath("/nonexistent/path/launch.json")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

// TestDiscover verifies that launch.json files can be discovered in parent directories.
func TestDiscover(t *testing.T) {
	// Create nested directory structure
	tmpDir := t.TempDir()
	vscodeDir := filepath.Join(tmpDir, ".vscode")
	nestedDir := filepath.Join(tmpDir, "src", "components")

	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatalf("failed to create .vscode dir: %v", err)
	}
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	launchJSON := `{"version": "0.2.0", "configurations": []}`
	launchPath := filepath.Join(vscodeDir, "launch.json")
	if err := os.WriteFile(launchPath, []byte(launchJSON), 0644); err != nil {
		t.Fatalf("failed to write launch.json: %v", err)
	}

	// Test discovery from nested directory
	foundPath, err := launchconfig.Discover(nestedDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if foundPath != launchPath {
		t.Errorf("expected %s, got %s", launchPath, foundPath)
	}
}

// TestDiscover_NotFound verifies error handling when no launch.json exists.
func TestDiscover_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := launchconfig.Discover(tmpDir)
	if err == nil {
		t.Error("expected error when launch.json not found, got nil")
	}
}

// TestFindConfiguration verifies configuration lookup by name.
func TestFindConfiguration(t *testing.T) {
	lj := &launchconfig.LaunchJSON{
		Version: "0.2.0",
		Configurations: []launchconfig.DebugConfiguration{
			{Type: "python", Request: "launch", Name: "Python Debug"},
			{Type: "go", Request: "launch", Name: "Go Debug"},
		},
	}

	// Test finding existing config
	cfg, err := launchconfig.FindConfiguration(lj, "Python Debug")
	if err != nil {
		t.Fatalf("FindConfiguration failed: %v", err)
	}
	if cfg.Type != "python" {
		t.Errorf("expected type python, got %s", cfg.Type)
	}

	// Test not found
	_, err = launchconfig.FindConfiguration(lj, "Not Found")
	if err == nil {
		t.Error("expected error for non-existent config")
	}
}

// TestFindCompound verifies compound configuration lookup by name.
func TestFindCompound(t *testing.T) {
	lj := &launchconfig.LaunchJSON{
		Version: "0.2.0",
		Compounds: []launchconfig.CompoundConfig{
			{Name: "Full Stack", Configurations: []string{"A", "B"}, StopAll: true},
		},
	}

	compound, err := launchconfig.FindCompound(lj, "Full Stack")
	if err != nil {
		t.Fatalf("FindCompound failed: %v", err)
	}
	if !compound.StopAll {
		t.Error("expected stopAll to be true")
	}

	_, err = launchconfig.FindCompound(lj, "Not Found")
	if err == nil {
		t.Error("expected error for non-existent compound")
	}
}

// TestResolveVariables verifies VS Code variable expansion.
func TestResolveVariables(t *testing.T) {
	ctx := &launchconfig.ResolutionContext{
		WorkspaceFolder: "/home/user/project",
		CurrentFile:     "/home/user/project/src/main.py",
		EnvOverrides: map[string]string{
			"MY_VAR": "test_value",
		},
		InputValues: map[string]string{
			"port": "3000",
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"workspaceFolder", "${workspaceFolder}", "/home/user/project"},
		{"workspaceFolderBasename", "${workspaceFolderBasename}", "project"},
		{"file", "${file}", "/home/user/project/src/main.py"},
		{"fileBasename", "${fileBasename}", "main.py"},
		{"fileDirname", "${fileDirname}", "/home/user/project/src"},
		{"fileBasenameNoExtension", "${fileBasenameNoExtension}", "main"},
		{"fileExtname", "${fileExtname}", ".py"},
		{"relativeFile", "${relativeFile}", "src/main.py"},
		{"env variable", "${env:MY_VAR}", "test_value"},
		{"input variable", "${input:port}", "3000"},
		{"pathSeparator", "${pathSeparator}", string(os.PathSeparator)},
		{"mixed text", "Program: ${fileBasename} in ${workspaceFolder}", "Program: main.py in /home/user/project"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := launchconfig.ResolveVariables(tc.input, ctx)
			if err != nil {
				t.Errorf("ResolveVariables(%q) error: %v", tc.input, err)
				return
			}
			if result != tc.expected {
				t.Errorf("ResolveVariables(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestResolveVariables_MissingInput verifies error handling for missing input values.
func TestResolveVariables_MissingInput(t *testing.T) {
	ctx := &launchconfig.ResolutionContext{
		WorkspaceFolder: "/home/user/project",
	}

	_, err := launchconfig.ResolveVariables("${input:missing}", ctx)
	if err == nil {
		t.Error("expected error for missing input")
	}
}

// TestResolveVariables_EmptyEnv verifies behavior with undefined environment variables.
func TestResolveVariables_EmptyEnv(t *testing.T) {
	ctx := &launchconfig.ResolutionContext{
		WorkspaceFolder: "/home/user/project",
	}

	// Environment variable not set - should return empty string
	result, err := launchconfig.ResolveVariables("${env:UNDEFINED_VAR}", ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for undefined env var, got %q", result)
	}
}

// TestResolveConfiguration verifies full configuration resolution with variables.
func TestResolveConfiguration(t *testing.T) {
	cfg := &launchconfig.DebugConfiguration{
		Type:    "python",
		Request: "launch",
		Name:    "Python Debug",
		Program: "${workspaceFolder}/main.py",
		Cwd:     "${workspaceFolder}",
		Args:    []string{"--arg1", "${env:MY_ARG}"},
		Env: map[string]string{
			"PYTHONPATH": "${workspaceFolder}/lib",
		},
	}

	ctx := &launchconfig.ResolutionContext{
		WorkspaceFolder: "/home/user/project",
		EnvOverrides: map[string]string{
			"MY_ARG": "value1",
		},
	}

	resolved, err := launchconfig.ResolveConfiguration(cfg, ctx)
	if err != nil {
		t.Fatalf("ResolveConfiguration failed: %v", err)
	}

	if resolved.Program != "/home/user/project/main.py" {
		t.Errorf("expected program /home/user/project/main.py, got %s", resolved.Program)
	}

	if resolved.Cwd != "/home/user/project" {
		t.Errorf("expected cwd /home/user/project, got %s", resolved.Cwd)
	}

	if len(resolved.Args) != 2 || resolved.Args[1] != "value1" {
		t.Errorf("expected args [--arg1, value1], got %v", resolved.Args)
	}

	if resolved.Env["PYTHONPATH"] != "/home/user/project/lib" {
		t.Errorf("expected PYTHONPATH /home/user/project/lib, got %s", resolved.Env["PYTHONPATH"])
	}

	if resolved.Language != "python" {
		t.Errorf("expected language python, got %s", resolved.Language)
	}
}

// TestDebugConfigurationHelpers verifies IsLaunchRequest and IsAttachRequest.
func TestDebugConfigurationHelpers(t *testing.T) {
	launchCfg := &launchconfig.DebugConfiguration{
		Type:    "python",
		Request: "launch",
	}
	if !launchCfg.IsLaunchRequest() {
		t.Error("expected IsLaunchRequest to be true")
	}
	if launchCfg.IsAttachRequest() {
		t.Error("expected IsAttachRequest to be false")
	}

	attachCfg := &launchconfig.DebugConfiguration{
		Type:    "node",
		Request: "attach",
	}
	if attachCfg.IsLaunchRequest() {
		t.Error("expected IsLaunchRequest to be false")
	}
	if !attachCfg.IsAttachRequest() {
		t.Error("expected IsAttachRequest to be true")
	}
}

// TestTypeToLanguageMapping verifies GetLanguage returns correct language for each type.
func TestTypeToLanguageMapping(t *testing.T) {
	tests := []struct {
		cfgType  string
		expected string
	}{
		{"python", "python"},
		{"go", "go"},
		{"node", "javascript"},
		{"pwa-node", "javascript"},
		{"chrome", "javascript"},
		{"pwa-chrome", "javascript"},
		{"msedge", "javascript"},
		{"unknown", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.cfgType, func(t *testing.T) {
			cfg := &launchconfig.DebugConfiguration{Type: tc.cfgType}
			lang := cfg.GetLanguage()
			if lang != tc.expected {
				t.Errorf("GetLanguage for type %q = %q, want %q", tc.cfgType, lang, tc.expected)
			}
		})
	}
}

// TestIsBrowserTarget verifies browser target detection.
func TestIsBrowserTarget(t *testing.T) {
	browserTypes := []string{"chrome", "pwa-chrome", "msedge", "pwa-msedge"}
	nonBrowserTypes := []string{"node", "pwa-node", "python", "go"}

	for _, cfgType := range browserTypes {
		t.Run(cfgType+"_isBrowser", func(t *testing.T) {
			cfg := &launchconfig.DebugConfiguration{Type: cfgType}
			if !cfg.IsBrowserTarget() {
				t.Errorf("expected IsBrowserTarget to be true for %s", cfgType)
			}
		})
	}

	for _, cfgType := range nonBrowserTypes {
		t.Run(cfgType+"_notBrowser", func(t *testing.T) {
			cfg := &launchconfig.DebugConfiguration{Type: cfgType}
			if cfg.IsBrowserTarget() {
				t.Errorf("expected IsBrowserTarget to be false for %s", cfgType)
			}
		})
	}
}

// TestToLaunchArgs verifies conversion to launch arguments map.
func TestToLaunchArgs(t *testing.T) {
	resolved := &launchconfig.ResolvedConfiguration{
		DebugConfiguration: &launchconfig.DebugConfiguration{
			Program:     "/app/main.py",
			Args:        []string{"--debug"},
			Cwd:         "/app",
			Env:         map[string]string{"DEBUG": "1"},
			StopOnEntry: true,
			Mode:        "debug",
		},
		Language: "python",
	}

	args := resolved.ToLaunchArgs()

	if args["program"] != "/app/main.py" {
		t.Errorf("expected program /app/main.py, got %v", args["program"])
	}

	argsSlice, ok := args["args"].([]string)
	if !ok || len(argsSlice) != 1 || argsSlice[0] != "--debug" {
		t.Errorf("expected args [--debug], got %v", args["args"])
	}

	if args["stopOnEntry"] != true {
		t.Errorf("expected stopOnEntry true, got %v", args["stopOnEntry"])
	}

	if args["mode"] != "debug" {
		t.Errorf("expected mode debug, got %v", args["mode"])
	}
}

// TestToAttachArgs verifies conversion to attach arguments map.
func TestToAttachArgs(t *testing.T) {
	resolved := &launchconfig.ResolvedConfiguration{
		DebugConfiguration: &launchconfig.DebugConfiguration{
			Host:      "localhost",
			Port:      9229,
			ProcessID: 1234,
		},
		Language: "javascript",
		Target:   "node",
	}

	args := resolved.ToAttachArgs()

	if args["host"] != "localhost" {
		t.Errorf("expected host localhost, got %v", args["host"])
	}

	if args["port"] != 9229 {
		t.Errorf("expected port 9229, got %v", args["port"])
	}

	if args["processId"] != 1234 {
		t.Errorf("expected processId 1234, got %v", args["processId"])
	}
}

// TestMergeOverrides verifies configuration override merging.
func TestMergeOverrides(t *testing.T) {
	cfg := &launchconfig.DebugConfiguration{
		Type:    "python",
		Request: "launch",
		Program: "/original/path.py",
		Args:    []string{"--original"},
	}

	overrides := map[string]interface{}{
		"program":  "/override/path.py",
		"args":     []interface{}{"--new1", "--new2"},
		"newField": "value",
	}

	merged := launchconfig.MergeOverrides(cfg, overrides)

	if merged.Program != "/override/path.py" {
		t.Errorf("expected overridden program, got %s", merged.Program)
	}

	if len(merged.Args) != 2 || merged.Args[0] != "--new1" {
		t.Errorf("expected overridden args, got %v", merged.Args)
	}

	if merged.Extra["newField"] != "value" {
		t.Errorf("expected extra field, got %v", merged.Extra)
	}

	// Original should be unchanged
	if cfg.Program != "/original/path.py" {
		t.Error("original cfg was modified")
	}
}

// TestFindAllRequiredInputsInConfig verifies input variable detection.
func TestFindAllRequiredInputsInConfig(t *testing.T) {
	cfg := &launchconfig.DebugConfiguration{
		Program: "${input:programPath}",
		Args:    []string{"--port", "${input:port}"},
		Env: map[string]string{
			"API_KEY": "${input:apiKey}",
		},
		Extra: map[string]interface{}{
			"customField": "${input:customValue}",
		},
	}

	inputs := launchconfig.FindAllRequiredInputsInConfig(cfg)

	expected := map[string]bool{
		"programPath": true,
		"port":        true,
		"apiKey":      true,
		"customValue": true,
	}

	if len(inputs) != len(expected) {
		t.Errorf("expected %d inputs, got %d: %v", len(expected), len(inputs), inputs)
	}

	for _, input := range inputs {
		if !expected[input] {
			t.Errorf("unexpected input: %s", input)
		}
	}
}

// TestValidateInputsProvided verifies input validation.
func TestValidateInputsProvided(t *testing.T) {
	cfg := &launchconfig.DebugConfiguration{
		Program: "${input:programPath}",
		Args:    []string{"${input:arg1}"},
	}

	// Test with missing inputs
	missing := launchconfig.ValidateInputsProvided(cfg, nil)
	if len(missing) != 2 {
		t.Errorf("expected 2 missing inputs, got %d", len(missing))
	}

	// Test with partial inputs
	missing = launchconfig.ValidateInputsProvided(cfg, map[string]string{"programPath": "/app"})
	if len(missing) != 1 {
		t.Errorf("expected 1 missing input, got %d", len(missing))
	}

	// Test with all inputs provided
	missing = launchconfig.ValidateInputsProvided(cfg, map[string]string{
		"programPath": "/app",
		"arg1":        "value",
	})
	if len(missing) != 0 {
		t.Errorf("expected 0 missing inputs, got %d", len(missing))
	}
}

// TestValidateConfiguration verifies configuration validation rules.
func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *launchconfig.DebugConfiguration
		wantErr bool
	}{
		{
			name: "valid launch config",
			cfg: &launchconfig.DebugConfiguration{
				Type:    "python",
				Request: "launch",
				Name:    "Python",
				Program: "main.py",
			},
			wantErr: false,
		},
		{
			name: "valid attach config",
			cfg: &launchconfig.DebugConfiguration{
				Type:    "node",
				Request: "attach",
				Name:    "Node Attach",
				Port:    9229,
			},
			wantErr: false,
		},
		{
			name: "missing type",
			cfg: &launchconfig.DebugConfiguration{
				Request: "launch",
				Name:    "No Type",
				Program: "main.py",
			},
			wantErr: true,
		},
		{
			name: "missing request",
			cfg: &launchconfig.DebugConfiguration{
				Type: "python",
				Name: "No Request",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			cfg: &launchconfig.DebugConfiguration{
				Type:    "python",
				Request: "launch",
				Program: "main.py",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := launchconfig.ValidateConfiguration(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateConfiguration() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestListConfigurations verifies listing all configurations.
func TestListConfigurations(t *testing.T) {
	lj := &launchconfig.LaunchJSON{
		Configurations: []launchconfig.DebugConfiguration{
			{Type: "python", Request: "launch", Name: "Python Debug"},
			{Type: "go", Request: "attach", Name: "Go Attach"},
		},
	}

	configs := launchconfig.ListConfigurations(lj)

	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}

	// Check first config
	if configs[0].Name != "Python Debug" {
		t.Errorf("expected name Python Debug, got %v", configs[0].Name)
	}
	if configs[0].Type != "python" {
		t.Errorf("expected type python, got %v", configs[0].Type)
	}
	if configs[0].Request != "launch" {
		t.Errorf("expected request launch, got %v", configs[0].Request)
	}
}

// TestListCompounds verifies listing all compound configurations.
func TestListCompounds(t *testing.T) {
	lj := &launchconfig.LaunchJSON{
		Compounds: []launchconfig.CompoundConfig{
			{Name: "Full Stack", Configurations: []string{"A", "B"}, StopAll: true},
			{Name: "Backend Only", Configurations: []string{"C"}, StopAll: false},
		},
	}

	compounds := launchconfig.ListCompounds(lj)

	if len(compounds) != 2 {
		t.Errorf("expected 2 compounds, got %d", len(compounds))
	}

	if compounds[0].Name != "Full Stack" {
		t.Errorf("expected name Full Stack, got %v", compounds[0].Name)
	}
	if compounds[0].StopAll != true {
		t.Errorf("expected stopAll true, got %v", compounds[0].StopAll)
	}
}

// TestGetWorkspaceFolder verifies workspace folder extraction from launch.json path.
func TestGetWorkspaceFolder(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/home/user/project/.vscode/launch.json", "/home/user/project"},
		{"/project/.vscode/launch.json", "/project"},
		{"/.vscode/launch.json", "/"}, // Root directory case
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			workspace := launchconfig.GetWorkspaceFolder(tc.input)
			if workspace != tc.expected {
				t.Errorf("GetWorkspaceFolder(%q) = %q, want %q", tc.input, workspace, tc.expected)
			}
		})
	}
}

// TestMissingInputsError verifies the MissingInputsError type.
func TestMissingInputsError(t *testing.T) {
	err := &launchconfig.MissingInputsError{Inputs: []string{"a", "b"}}
	if err.Error() != "missing input values: [a b]" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	e, ok := launchconfig.IsMissingInputsError(err)
	if !ok {
		t.Error("expected IsMissingInputsError to return true")
	}
	if len(e.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(e.Inputs))
	}

	_, ok = launchconfig.IsMissingInputsError(os.ErrNotExist)
	if ok {
		t.Error("expected IsMissingInputsError to return false for other error types")
	}
}

// TestResolveExtraFields verifies resolution of variables in Extra fields.
func TestResolveExtraFields(t *testing.T) {
	ctx := &launchconfig.ResolutionContext{
		WorkspaceFolder: "/home/user/project",
	}

	cfg := &launchconfig.DebugConfiguration{
		Type:    "custom",
		Request: "launch",
		Name:    "Custom",
		Program: "main",
		Extra: map[string]interface{}{
			"stringField": "${workspaceFolder}/data",
			"arrayField":  []interface{}{"${workspaceFolder}/a", "${workspaceFolder}/b"},
			"nestedField": map[string]interface{}{
				"inner": "${workspaceFolder}/nested",
			},
			"numberField": 42,
			"boolField":   true,
		},
	}

	resolved, err := launchconfig.ResolveConfiguration(cfg, ctx)
	if err != nil {
		t.Fatalf("ResolveConfiguration failed: %v", err)
	}

	if resolved.Extra["stringField"] != "/home/user/project/data" {
		t.Errorf("expected resolved string field, got %v", resolved.Extra["stringField"])
	}

	arr, ok := resolved.Extra["arrayField"].([]interface{})
	if !ok || len(arr) != 2 {
		t.Errorf("expected resolved array, got %v", resolved.Extra["arrayField"])
	} else if arr[0] != "/home/user/project/a" {
		t.Errorf("expected resolved array element, got %v", arr[0])
	}

	nested, ok := resolved.Extra["nestedField"].(map[string]interface{})
	if !ok {
		t.Errorf("expected nested map, got %v", resolved.Extra["nestedField"])
	} else if nested["inner"] != "/home/user/project/nested" {
		t.Errorf("expected resolved nested field, got %v", nested["inner"])
	}

	// Non-string types should pass through unchanged
	if resolved.Extra["numberField"] != 42 {
		t.Errorf("expected number to pass through, got %v", resolved.Extra["numberField"])
	}
	if resolved.Extra["boolField"] != true {
		t.Errorf("expected bool to pass through, got %v", resolved.Extra["boolField"])
	}
}
