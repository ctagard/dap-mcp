package test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ctagard/dap-mcp/internal/adapters"
	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// TestLLDBAdapterRegistry verifies that LLDB adapter is registered for native languages
func TestLLDBAdapterRegistry(t *testing.T) {
	cfg := config.DefaultConfig()
	registry := adapters.NewRegistry(cfg)

	// Test that C, C++, and Rust all have adapters registered
	languages := []types.Language{types.LanguageC, types.LanguageCpp, types.LanguageRust}
	for _, lang := range languages {
		adapter, err := registry.Get(lang)
		if err != nil {
			t.Errorf("Expected adapter for language %s, got error: %v", lang, err)
			continue
		}

		// Verify it's a StdioAdapter
		stdioAdapter, ok := adapter.(adapters.StdioAdapter)
		if !ok {
			t.Errorf("Expected StdioAdapter for language %s, got %T", lang, adapter)
			continue
		}

		if !stdioAdapter.IsStdio() {
			t.Errorf("Expected IsStdio() to return true for language %s", lang)
		}
	}
}

// TestLLDBAdapterSpawnStdio tests spawning the LLDB adapter (requires lldb-dap)
func TestLLDBAdapterSpawnStdio(t *testing.T) {
	// Check if lldb-dap is available
	lldbDapPath := findLLDBDap()
	if lldbDapPath == "" {
		t.Skip("lldb-dap not found, skipping test")
	}

	// Create adapter with the found path
	cfg := config.LLDBConfig{Path: lldbDapPath}
	adapter := adapters.NewLLDBAdapter(cfg)

	// Create a simple C test program
	testDir := t.TempDir()
	srcFile := filepath.Join(testDir, "main.c")
	binFile := filepath.Join(testDir, "main")

	// Write test program
	err := os.WriteFile(srcFile, []byte(`
#include <stdio.h>
int main() {
    int x = 42;
    printf("x = %d\n", x);
    return 0;
}
`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test program: %v", err)
	}

	// Compile with debug symbols
	compileCmd := exec.Command("clang", "-g", "-O0", "-o", binFile, srcFile)
	if output, err := compileCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to compile test program: %v\nOutput: %s", err, output)
	}

	// Spawn the adapter
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, cmd, err := adapter.SpawnStdio(ctx, binFile, map[string]interface{}{
		"cwd": testDir,
	})
	if err != nil {
		t.Fatalf("Failed to spawn LLDB adapter: %v", err)
	}
	defer func() {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Initialize the debug adapter
	_, err = client.Initialize("test", "LLDB Test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Build launch args
	launchArgs := adapter.BuildLaunchArgs(binFile, map[string]interface{}{
		"stopOnEntry": true,
	})

	// Launch the program
	launchRespCh, err := client.LaunchAsync(launchArgs)
	if err != nil {
		t.Fatalf("Failed to launch: %v", err)
	}

	// Wait for initialized event
	if err := client.WaitInitialized(10 * time.Second); err != nil {
		t.Fatalf("Failed waiting for initialized: %v", err)
	}

	// Signal configuration done
	if err := client.ConfigurationDone(); err != nil {
		t.Fatalf("Configuration done failed: %v", err)
	}

	// Wait for the launch response
	_, err = client.WaitForLaunchResponse(launchRespCh, 10*time.Second)
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	// Get threads to verify session is working
	threads, err := client.Threads()
	if err != nil {
		t.Fatalf("Failed to get threads: %v", err)
	}

	if len(threads) == 0 {
		t.Error("Expected at least one thread")
	}

	t.Logf("Successfully launched LLDB debug session with %d threads", len(threads))

	// Cleanup
	client.Disconnect(true)
	client.Close()
}

// TestGDBAdapterRegistry verifies GDB adapter can be created
func TestGDBAdapterRegistry(t *testing.T) {
	cfg := config.GDBConfig{Path: "gdb"}
	adapter := adapters.NewGDBAdapter(cfg)

	if !adapter.IsStdio() {
		t.Error("Expected GDB adapter to be stdio-based")
	}
}

// findLLDBDap searches for lldb-dap in common locations
func findLLDBDap() string {
	// Check PATH first
	if path, err := exec.LookPath("lldb-dap"); err == nil {
		return path
	}

	// Check common macOS locations
	locations := []string{
		"/Library/Developer/CommandLineTools/usr/bin/lldb-dap",
		"/Applications/Xcode.app/Contents/Developer/usr/bin/lldb-dap",
		"/usr/local/bin/lldb-dap",
		"/opt/homebrew/bin/lldb-dap",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Check for lldb-vscode (older name)
	if path, err := exec.LookPath("lldb-vscode"); err == nil {
		return path
	}

	return ""
}
