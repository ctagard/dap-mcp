package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ctagard/dap-mcp/internal/config"
)

// TestDefaultConfig verifies that DefaultConfig returns sensible defaults.
func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	// Verify mode defaults
	if cfg.Mode != config.ModeFull {
		t.Errorf("expected mode %s, got %s", config.ModeFull, cfg.Mode)
	}

	// Verify permission defaults
	if !cfg.AllowSpawn {
		t.Error("expected AllowSpawn to be true by default")
	}
	if !cfg.AllowAttach {
		t.Error("expected AllowAttach to be true by default")
	}
	if !cfg.AllowModify {
		t.Error("expected AllowModify to be true by default")
	}
	if !cfg.AllowExecute {
		t.Error("expected AllowExecute to be true by default")
	}

	// Verify safety limits
	if cfg.MaxSessions != 10 {
		t.Errorf("expected MaxSessions 10, got %d", cfg.MaxSessions)
	}
	if cfg.SessionTimeout != 30*time.Minute {
		t.Errorf("expected SessionTimeout 30m, got %v", cfg.SessionTimeout)
	}

	// Verify adapter defaults
	if cfg.Adapters.Go.Path != "dlv" {
		t.Errorf("expected Go adapter path 'dlv', got %s", cfg.Adapters.Go.Path)
	}
	if cfg.Adapters.Python.PythonPath != "python3" {
		t.Errorf("expected Python path 'python3', got %s", cfg.Adapters.Python.PythonPath)
	}
	if cfg.Adapters.Node.NodePath != "node" {
		t.Errorf("expected Node path 'node', got %s", cfg.Adapters.Node.NodePath)
	}
}

// TestLoadConfig_EmptyPath verifies that empty path returns defaults.
func TestLoadConfig_EmptyPath(t *testing.T) {
	cfg, err := config.LoadConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be same as defaults
	defaults := config.DefaultConfig()
	if cfg.Mode != defaults.Mode {
		t.Errorf("expected default mode, got %s", cfg.Mode)
	}
	if cfg.MaxSessions != defaults.MaxSessions {
		t.Errorf("expected default MaxSessions, got %d", cfg.MaxSessions)
	}
}

// TestLoadConfig_FromFile verifies loading configuration from JSON file.
func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"mode": "readonly",
		"allowSpawn": false,
		"allowAttach": true,
		"allowModify": false,
		"allowExecute": false,
		"maxSessions": 5,
		"adapters": {
			"go": {
				"path": "/custom/dlv",
				"buildFlags": "-race"
			},
			"python": {
				"pythonPath": "/usr/bin/python3.10"
			},
			"node": {
				"nodePath": "/usr/local/bin/node",
				"inspectBrk": false
			}
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify loaded values
	if cfg.Mode != config.ModeReadOnly {
		t.Errorf("expected mode readonly, got %s", cfg.Mode)
	}
	if cfg.AllowSpawn {
		t.Error("expected AllowSpawn to be false")
	}
	if !cfg.AllowAttach {
		t.Error("expected AllowAttach to be true")
	}
	if cfg.AllowModify {
		t.Error("expected AllowModify to be false")
	}
	if cfg.MaxSessions != 5 {
		t.Errorf("expected MaxSessions 5, got %d", cfg.MaxSessions)
	}
	if cfg.Adapters.Go.Path != "/custom/dlv" {
		t.Errorf("expected Go adapter path '/custom/dlv', got %s", cfg.Adapters.Go.Path)
	}
	if cfg.Adapters.Go.BuildFlags != "-race" {
		t.Errorf("expected Go buildFlags '-race', got %s", cfg.Adapters.Go.BuildFlags)
	}
}

// TestLoadConfig_NonExistent verifies error handling for missing files.
func TestLoadConfig_NonExistent(t *testing.T) {
	_, err := config.LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// TestLoadConfig_InvalidJSON verifies error handling for malformed JSON.
func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	if err := os.WriteFile(configPath, []byte(`{invalid}`), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := config.LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestLoadConfig_PartialOverrides verifies that partial JSON merges with defaults.
func TestLoadConfig_PartialOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Only override mode, leave everything else as defaults
	configJSON := `{"mode": "readonly"}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Mode should be overridden
	if cfg.Mode != config.ModeReadOnly {
		t.Errorf("expected mode readonly, got %s", cfg.Mode)
	}

	// Other fields should retain defaults
	if !cfg.AllowSpawn {
		t.Error("expected AllowSpawn to retain default (true)")
	}
	if cfg.MaxSessions != 10 {
		t.Errorf("expected MaxSessions to retain default (10), got %d", cfg.MaxSessions)
	}
}

// TestCanUseControlTools verifies control tool permission checking.
func TestCanUseControlTools(t *testing.T) {
	tests := []struct {
		name     string
		mode     config.CapabilityMode
		expected bool
	}{
		{"full mode allows control", config.ModeFull, true},
		{"readonly mode denies control", config.ModeReadOnly, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Mode = tc.mode
			if cfg.CanUseControlTools() != tc.expected {
				t.Errorf("CanUseControlTools() = %v, want %v", cfg.CanUseControlTools(), tc.expected)
			}
		})
	}
}

// TestCanSpawn verifies spawn permission checking.
func TestCanSpawn(t *testing.T) {
	cfg := config.DefaultConfig()

	// Default should allow
	if !cfg.CanSpawn() {
		t.Error("default config should allow spawn")
	}

	// Disabled should deny
	cfg.AllowSpawn = false
	if cfg.CanSpawn() {
		t.Error("disabled spawn should not allow spawn")
	}
}

// TestCanAttach verifies attach permission checking.
func TestCanAttach(t *testing.T) {
	cfg := config.DefaultConfig()

	// Default should allow
	if !cfg.CanAttach() {
		t.Error("default config should allow attach")
	}

	// Disabled should deny
	cfg.AllowAttach = false
	if cfg.CanAttach() {
		t.Error("disabled attach should not allow attach")
	}
}

// TestCanModifyVariables verifies variable modification permission checking.
func TestCanModifyVariables(t *testing.T) {
	tests := []struct {
		name        string
		mode        config.CapabilityMode
		allowModify bool
		expected    bool
	}{
		{"full mode with modify", config.ModeFull, true, true},
		{"full mode without modify", config.ModeFull, false, false},
		{"readonly mode with modify", config.ModeReadOnly, true, false},
		{"readonly mode without modify", config.ModeReadOnly, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Mode = tc.mode
			cfg.AllowModify = tc.allowModify
			if cfg.CanModifyVariables() != tc.expected {
				t.Errorf("CanModifyVariables() = %v, want %v", cfg.CanModifyVariables(), tc.expected)
			}
		})
	}
}

// TestCanEvaluate verifies expression evaluation permission checking.
func TestCanEvaluate(t *testing.T) {
	cfg := config.DefaultConfig()

	// Default should allow
	if !cfg.CanEvaluate() {
		t.Error("default config should allow evaluate")
	}

	// Disabled should deny
	cfg.AllowExecute = false
	if cfg.CanEvaluate() {
		t.Error("disabled execute should not allow evaluate")
	}
}

// TestCapabilityModes verifies the capability mode constants.
func TestCapabilityModes(t *testing.T) {
	if config.ModeReadOnly != "readonly" {
		t.Errorf("expected ModeReadOnly='readonly', got %s", config.ModeReadOnly)
	}
	if config.ModeFull != "full" {
		t.Errorf("expected ModeFull='full', got %s", config.ModeFull)
	}
}
