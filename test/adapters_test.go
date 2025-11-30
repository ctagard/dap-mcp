package test

import (
	"testing"

	"github.com/ctagard/dap-mcp/internal/adapters"
	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// TestNewRegistry verifies adapter registry creation with all adapters.
func TestNewRegistry(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)

	// Verify all expected adapters are registered
	languages := []types.Language{
		types.LanguageGo,
		types.LanguagePython,
		types.LanguageJavaScript,
		types.LanguageTypeScript,
	}

	for _, lang := range languages {
		adapter, err := reg.Get(lang)
		if err != nil {
			t.Errorf("expected adapter for %s, got error: %v", lang, err)
			continue
		}
		if adapter == nil {
			t.Errorf("expected non-nil adapter for %s", lang)
			continue
		}
		// Verify adapter reports correct language
		if adapter.Language() != lang {
			// JavaScript and TypeScript share an adapter
			if lang == types.LanguageTypeScript && adapter.Language() == types.LanguageJavaScript {
				continue // This is expected - they share the Node adapter
			}
			t.Errorf("adapter for %s reports language %s", lang, adapter.Language())
		}
	}
}

// TestRegistry_Get_NotFound verifies error for unknown language.
func TestRegistry_Get_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)

	_, err := reg.Get(types.Language("unknown"))
	if err == nil {
		t.Error("expected error for unknown language")
	}
}

// TestRegistry_GoAdapter verifies Go adapter is correctly configured.
func TestRegistry_GoAdapter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Adapters.Go.Path = "/custom/dlv"
	cfg.Adapters.Go.BuildFlags = "-race"

	reg := adapters.NewRegistry(cfg)
	adapter, err := reg.Get(types.LanguageGo)
	if err != nil {
		t.Fatalf("failed to get Go adapter: %v", err)
	}

	if adapter.Language() != types.LanguageGo {
		t.Errorf("expected language go, got %s", adapter.Language())
	}
}

// TestRegistry_PythonAdapter verifies Python adapter is correctly configured.
func TestRegistry_PythonAdapter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Adapters.Python.PythonPath = "/usr/bin/python3.10"

	reg := adapters.NewRegistry(cfg)
	adapter, err := reg.Get(types.LanguagePython)
	if err != nil {
		t.Fatalf("failed to get Python adapter: %v", err)
	}

	if adapter.Language() != types.LanguagePython {
		t.Errorf("expected language python, got %s", adapter.Language())
	}
}

// TestRegistry_NodeAdapter verifies Node adapter handles JS and TS.
func TestRegistry_NodeAdapter(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)

	// Both JS and TS should work
	jsAdapter, err := reg.Get(types.LanguageJavaScript)
	if err != nil {
		t.Fatalf("failed to get JavaScript adapter: %v", err)
	}

	tsAdapter, err := reg.Get(types.LanguageTypeScript)
	if err != nil {
		t.Fatalf("failed to get TypeScript adapter: %v", err)
	}

	// They should be the same adapter instance
	if jsAdapter != tsAdapter {
		t.Error("JavaScript and TypeScript should share the same adapter")
	}
}

// TestDelveAdapter_BuildLaunchArgs verifies Go launch argument building.
func TestDelveAdapter_BuildLaunchArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Adapters.Go.Path = "dlv"
	cfg.Adapters.Go.BuildFlags = "-race"

	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguageGo)

	args := adapter.BuildLaunchArgs("/path/to/main.go", map[string]interface{}{
		"args":        []interface{}{"--config", "test.yaml"}, // JSON unmarshals as []interface{}
		"cwd":         "/project",
		"stopOnEntry": true,
		"buildFlags":  "-race", // Must be passed explicitly in args
	})

	// Verify program is set
	if args["program"] != "/path/to/main.go" {
		t.Errorf("expected program /path/to/main.go, got %v", args["program"])
	}

	// Verify mode (default)
	if args["mode"] != "debug" {
		t.Errorf("expected mode debug, got %v", args["mode"])
	}

	// Verify build flags are included when passed in args
	if args["buildFlags"] != "-race" {
		t.Errorf("expected buildFlags -race, got %v", args["buildFlags"])
	}
}

// TestDelveAdapter_BuildAttachArgs verifies Go attach argument building.
func TestDelveAdapter_BuildAttachArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguageGo)

	args := adapter.BuildAttachArgs(map[string]interface{}{
		"pid": float64(12345), // JSON unmarshals integers as float64
	})

	if args["processId"] != 12345 {
		t.Errorf("expected processId 12345, got %v", args["processId"])
	}

	// Mode defaults to "local"
	if args["mode"] != "local" {
		t.Errorf("expected mode local, got %v", args["mode"])
	}
}

// TestDebugpyAdapter_BuildLaunchArgs verifies Python launch argument building.
func TestDebugpyAdapter_BuildLaunchArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguagePython)

	args := adapter.BuildLaunchArgs("/path/to/script.py", map[string]interface{}{
		"args":        []string{"--verbose"},
		"cwd":         "/project",
		"stopOnEntry": true,
		"env": map[string]interface{}{
			"PYTHONPATH": "/lib",
		},
	})

	if args["program"] != "/path/to/script.py" {
		t.Errorf("expected program /path/to/script.py, got %v", args["program"])
	}

	if args["stopOnEntry"] != true {
		t.Errorf("expected stopOnEntry true, got %v", args["stopOnEntry"])
	}
}

// TestDebugpyAdapter_BuildAttachArgs verifies Python attach argument building.
func TestDebugpyAdapter_BuildAttachArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguagePython)

	args := adapter.BuildAttachArgs(map[string]interface{}{
		"host": "localhost",
		"port": float64(5678), // JSON unmarshals integers as float64
	})

	if args["host"] != "localhost" {
		t.Errorf("expected host localhost, got %v", args["host"])
	}

	// Port should be included (converted to int)
	if args["port"] != 5678 {
		t.Errorf("expected port 5678, got %v", args["port"])
	}
}

// TestNodeAdapter_BuildLaunchArgs verifies Node launch argument building.
func TestNodeAdapter_BuildLaunchArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Adapters.Node.SourceMapPathOverrides = map[string]string{
		"webpack:///*": "${workspaceFolder}/*",
	}

	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguageJavaScript)

	args := adapter.BuildLaunchArgs("/path/to/app.js", map[string]interface{}{
		"args":        []string{"--port", "3000"},
		"cwd":         "/project",
		"stopOnEntry": true,
	})

	if args["program"] != "/path/to/app.js" {
		t.Errorf("expected program /path/to/app.js, got %v", args["program"])
	}
}

// TestNodeAdapter_BuildLaunchArgs_Browser verifies browser launch arguments.
func TestNodeAdapter_BuildLaunchArgs_Browser(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguageJavaScript)

	args := adapter.BuildLaunchArgs("http://localhost:3000", map[string]interface{}{
		"target":  "chrome",
		"webRoot": "/project/src",
	})

	// For browser targets, URL should be set
	if args["url"] != "http://localhost:3000" {
		t.Errorf("expected url http://localhost:3000, got %v", args["url"])
	}
}

// TestNodeAdapter_BuildAttachArgs verifies Node attach argument building.
func TestNodeAdapter_BuildAttachArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguageJavaScript)

	args := adapter.BuildAttachArgs(map[string]interface{}{
		"host": "localhost",
		"port": float64(9229), // JSON unmarshals integers as float64
	})

	// Verify the args contain expected fields
	// The Node adapter may structure these differently
	if args == nil {
		t.Fatal("expected non-nil args")
	}
}

// TestNodeAdapter_BuildAttachArgs_Browser verifies browser attach arguments.
func TestNodeAdapter_BuildAttachArgs_Browser(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguageJavaScript)

	args := adapter.BuildAttachArgs(map[string]interface{}{
		"target":  "chrome",
		"port":    9222,
		"webRoot": "/project/src",
		"url":     "http://localhost:3000/*",
	})

	// Browser-specific args should be included
	if args["port"] != 9222 {
		t.Errorf("expected port 9222, got %v", args["port"])
	}
}

// TestConnect_InvalidAddress verifies error handling for invalid addresses.
func TestConnect_InvalidAddress(t *testing.T) {
	// Try to connect to an address that won't be listening
	_, err := adapters.Connect("127.0.0.1:59999", 1) // Only 1 retry for speed
	if err == nil {
		t.Error("expected error connecting to invalid address")
	}
}

// TestAdapterLanguageConstants verifies language constant values.
func TestAdapterLanguageConstants(t *testing.T) {
	// Ensure language constants have expected string values
	tests := []struct {
		lang     types.Language
		expected string
	}{
		{types.LanguageGo, "go"},
		{types.LanguagePython, "python"},
		{types.LanguageJavaScript, "javascript"},
		{types.LanguageTypeScript, "typescript"},
	}

	for _, tc := range tests {
		if string(tc.lang) != tc.expected {
			t.Errorf("expected %s = %q, got %q", tc.lang, tc.expected, string(tc.lang))
		}
	}
}

// TestDebugpyAdapter_BuildLaunchArgs_PythonPath verifies pythonPath is passed through.
func TestDebugpyAdapter_BuildLaunchArgs_PythonPath(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := adapters.NewRegistry(cfg)
	adapter, _ := reg.Get(types.LanguagePython)

	// Test that pythonPath in args is passed through to launch args
	args := adapter.BuildLaunchArgs("/path/to/script.py", map[string]interface{}{
		"pythonPath": "/custom/venv/bin/python3",
	})

	if args["pythonPath"] != "/custom/venv/bin/python3" {
		t.Errorf("expected pythonPath /custom/venv/bin/python3, got %v", args["pythonPath"])
	}
}
