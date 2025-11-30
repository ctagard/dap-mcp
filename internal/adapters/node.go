package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// NodeAdapter implements the Adapter interface for JavaScript/TypeScript via vscode-js-debug
type NodeAdapter struct {
	nodePath               string
	jsDebugPath            string
	inspectBrk             bool
	sourceMapPathOverrides map[string]string
}

// NewNodeAdapter creates a new Node.js adapter
func NewNodeAdapter(cfg config.NodeConfig) *NodeAdapter {
	nodePath := cfg.NodePath
	if nodePath == "" {
		nodePath = "node"
	}

	return &NodeAdapter{
		nodePath:               nodePath,
		jsDebugPath:            cfg.JsDebugPath,
		inspectBrk:             cfg.InspectBrk,
		sourceMapPathOverrides: cfg.SourceMapPathOverrides,
	}
}

// Language returns the language this adapter supports
func (n *NodeAdapter) Language() types.Language {
	return types.LanguageJavaScript
}

// Spawn starts the vscode-js-debug DAP server
// This spawns vscode-js-debug which provides a proper DAP interface and handles
// the translation to Chrome DevTools Protocol internally
func (n *NodeAdapter) Spawn(ctx context.Context, program string, args map[string]interface{}) (string, *exec.Cmd, error) {
	// Require jsDebugPath to be configured
	if n.jsDebugPath == "" {
		return "", nil, fmt.Errorf("jsDebugPath not configured: vscode-js-debug is required for JavaScript/TypeScript debugging. " +
			"Install from https://github.com/microsoft/vscode-js-debug/releases and set jsDebugPath in config")
	}

	port, err := findAvailablePort()
	if err != nil {
		return "", nil, fmt.Errorf("failed to find available port: %w", err)
	}

	address := fmt.Sprintf("127.0.0.1:%d", port)

	// Spawn vscode-js-debug DAP server
	// Usage: node dapDebugServer.js <port> [host]
	cmd := exec.CommandContext(ctx, n.nodePath, n.jsDebugPath, fmt.Sprintf("%d", port), "127.0.0.1")
	cmd.Env = os.Environ()
	// Explicitly disconnect stdin to prevent TTY issues when run as MCP server.
	cmd.Stdin = nil
	// Set platform-specific process attributes (procattr_unix.go / procattr_windows.go)
	setProcAttr(cmd)

	// Set working directory if specified
	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start vscode-js-debug: %w", err)
	}

	// Wait for the DAP server to start listening
	time.Sleep(500 * time.Millisecond)

	return address, cmd, nil
}

// BuildLaunchArgs builds the launch arguments for JavaScript/TypeScript debugging
// Supports both Node.js (pwa-node) and browser (pwa-chrome/pwa-msedge) debugging
func (n *NodeAdapter) BuildLaunchArgs(program string, args map[string]interface{}) map[string]interface{} {
	// Determine the debug target type
	target := "node" // default
	if t, ok := args["target"].(string); ok {
		target = t
	}

	var launchArgs map[string]interface{}

	switch target {
	case "chrome":
		// Browser debugging - Chrome
		launchArgs = n.buildBrowserLaunchArgs("pwa-chrome", program, args)
	case "edge":
		// Browser debugging - Edge
		launchArgs = n.buildBrowserLaunchArgs("pwa-msedge", program, args)
	default:
		// Node.js debugging
		launchArgs = n.buildNodeLaunchArgs(program, args)
	}

	return launchArgs
}

// buildNodeLaunchArgs builds launch arguments for Node.js debugging
func (n *NodeAdapter) buildNodeLaunchArgs(program string, args map[string]interface{}) map[string]interface{} {
	launchArgs := map[string]interface{}{
		"type":    "pwa-node",
		"request": "launch",
		"program": program,
		"console": "internalConsole",
	}

	// Pass through common arguments
	if programArgs, ok := args["args"].([]interface{}); ok {
		strArgs := make([]string, len(programArgs))
		for i, a := range programArgs {
			strArgs[i] = fmt.Sprint(a)
		}
		launchArgs["args"] = strArgs
	}

	if cwd, ok := args["cwd"].(string); ok {
		launchArgs["cwd"] = cwd
	}

	if env, ok := args["env"].(map[string]interface{}); ok {
		envMap := make(map[string]string)
		for k, v := range env {
			envMap[k] = fmt.Sprint(v)
		}
		launchArgs["env"] = envMap
	}

	if stopOnEntry, ok := args["stopOnEntry"].(bool); ok {
		launchArgs["stopOnEntry"] = stopOnEntry
	}

	// Node.js specific options
	if runtimeExecutable, ok := args["runtimeExecutable"].(string); ok {
		launchArgs["runtimeExecutable"] = runtimeExecutable
	}

	if runtimeArgs, ok := args["runtimeArgs"].([]interface{}); ok {
		strArgs := make([]string, len(runtimeArgs))
		for i, a := range runtimeArgs {
			strArgs[i] = fmt.Sprint(a)
		}
		launchArgs["runtimeArgs"] = strArgs
	}

	// TypeScript support via ts-node or similar
	if outFiles, ok := args["outFiles"].([]interface{}); ok {
		strFiles := make([]string, len(outFiles))
		for i, f := range outFiles {
			strFiles[i] = fmt.Sprint(f)
		}
		launchArgs["outFiles"] = strFiles
	}

	if sourceMaps, ok := args["sourceMaps"].(bool); ok {
		launchArgs["sourceMaps"] = sourceMaps
	} else {
		launchArgs["sourceMaps"] = true // Enable source maps by default
	}

	return launchArgs
}

// buildBrowserLaunchArgs builds launch arguments for browser debugging (Chrome/Edge)
// Used for debugging React, Svelte, Vue, and other frontend frameworks
func (n *NodeAdapter) buildBrowserLaunchArgs(debugType string, url string, args map[string]interface{}) map[string]interface{} {
	launchArgs := map[string]interface{}{
		"type":    debugType,
		"request": "launch",
		"url":     url,
	}

	// webRoot is important for source map resolution
	if webRoot, ok := args["webRoot"].(string); ok {
		launchArgs["webRoot"] = webRoot
	} else if cwd, ok := args["cwd"].(string); ok {
		// Fall back to cwd if webRoot not specified
		launchArgs["webRoot"] = cwd
	}

	// Source maps enabled by default for TypeScript/JSX
	if sourceMaps, ok := args["sourceMaps"].(bool); ok {
		launchArgs["sourceMaps"] = sourceMaps
	} else {
		launchArgs["sourceMaps"] = true
	}

	// resolveSourceMapLocations - critical for source map resolution
	// This tells the debugger where to look for source maps
	if webRoot, ok := launchArgs["webRoot"].(string); ok {
		launchArgs["resolveSourceMapLocations"] = []string{
			webRoot + "/**",
			"!**/node_modules/**",
		}

		// sourceMapPathOverrides - maps URLs in source maps to local files
		// Use custom overrides if provided, otherwise use defaults for common bundlers
		if len(n.sourceMapPathOverrides) > 0 {
			// Apply custom overrides, replacing ${webRoot} placeholder
			overrides := make(map[string]string)
			for pattern, replacement := range n.sourceMapPathOverrides {
				// Replace ${webRoot} with actual webRoot value
				finalReplacement := replacement
				if replacement == "${webRoot}/*" {
					finalReplacement = webRoot + "/*"
				} else if len(replacement) > 11 && replacement[:11] == "${webRoot}/" {
					finalReplacement = webRoot + replacement[10:]
				}
				overrides[pattern] = finalReplacement
			}
			launchArgs["sourceMapPathOverrides"] = overrides
		} else {
			// Default overrides for common bundlers: Vite, Webpack (CRA), and others
			launchArgs["sourceMapPathOverrides"] = map[string]string{
				// Vite serves files with their original paths
				"/*": webRoot + "/*",
				// Webpack/Create React App patterns
				"webpack:///src/*":  webRoot + "/src/*",
				"webpack:///./*":    webRoot + "/*",
				"webpack:///*":      "*",
				"webpack:///./~/*":  webRoot + "/node_modules/*",
				// Meteor pattern
				"meteor://💻app/*": webRoot + "/*",
			}
		}
	}

	// Enable pause on exceptions for debugging
	if pauseForSourceMap, ok := args["pauseForSourceMap"].(bool); ok {
		launchArgs["pauseForSourceMap"] = pauseForSourceMap
	}

	// User data directory for Chrome (to avoid conflicts with existing browser)
	launchArgs["userDataDir"] = true // Creates a temp profile

	return launchArgs
}

// BuildAttachArgs builds the attach arguments for JavaScript/TypeScript debugging
// Supports both Node.js and browser (Chrome/Edge) attach
func (n *NodeAdapter) BuildAttachArgs(args map[string]interface{}) map[string]interface{} {
	// Determine the debug target type
	target := "node" // default
	if t, ok := args["target"].(string); ok {
		target = t
	}

	switch target {
	case "chrome":
		return n.buildBrowserAttachArgs("pwa-chrome", args)
	case "edge":
		return n.buildBrowserAttachArgs("pwa-msedge", args)
	default:
		return n.buildNodeAttachArgs(args)
	}
}

// buildNodeAttachArgs builds attach arguments for Node.js debugging
func (n *NodeAdapter) buildNodeAttachArgs(args map[string]interface{}) map[string]interface{} {
	attachArgs := map[string]interface{}{
		"type":    "pwa-node",
		"request": "attach",
	}

	// Connect to a Node.js inspector
	if host, ok := args["host"].(string); ok {
		attachArgs["address"] = host
	} else {
		attachArgs["address"] = "127.0.0.1"
	}

	if port, ok := args["port"].(float64); ok {
		attachArgs["port"] = int(port)
	} else {
		attachArgs["port"] = 9229 // Default Node.js inspector port
	}

	// Process ID attachment
	if pid, ok := args["pid"].(float64); ok {
		attachArgs["processId"] = int(pid)
	}

	return attachArgs
}

// buildBrowserAttachArgs builds attach arguments for browser debugging
// Use this to attach to a Chrome/Edge instance launched with --remote-debugging-port
func (n *NodeAdapter) buildBrowserAttachArgs(debugType string, args map[string]interface{}) map[string]interface{} {
	attachArgs := map[string]interface{}{
		"type":    debugType,
		"request": "attach",
	}

	// URL pattern to match (can be used to find the right browser tab)
	if url, ok := args["url"].(string); ok {
		attachArgs["url"] = url
	}

	// webRoot for source map resolution
	if webRoot, ok := args["webRoot"].(string); ok {
		attachArgs["webRoot"] = webRoot

		// resolveSourceMapLocations - tells debugger where to look for source maps
		attachArgs["resolveSourceMapLocations"] = []string{
			webRoot + "/**",
			"!**/node_modules/**",
		}

		// sourceMapPathOverrides - maps URLs in source maps to local files
		// Use custom overrides if provided, otherwise use defaults for common bundlers
		if len(n.sourceMapPathOverrides) > 0 {
			// Apply custom overrides, replacing ${webRoot} placeholder
			overrides := make(map[string]string)
			for pattern, replacement := range n.sourceMapPathOverrides {
				// Replace ${webRoot} with actual webRoot value
				finalReplacement := replacement
				if replacement == "${webRoot}/*" {
					finalReplacement = webRoot + "/*"
				} else if len(replacement) > 11 && replacement[:11] == "${webRoot}/" {
					finalReplacement = webRoot + replacement[10:]
				}
				overrides[pattern] = finalReplacement
			}
			attachArgs["sourceMapPathOverrides"] = overrides
		} else {
			// Default overrides for common bundlers: Vite, Webpack (CRA), and others
			attachArgs["sourceMapPathOverrides"] = map[string]string{
				// Vite serves files with their original paths
				"/*": webRoot + "/*",
				// Webpack/Create React App patterns
				"webpack:///src/*":  webRoot + "/src/*",
				"webpack:///./*":    webRoot + "/*",
				"webpack:///*":      "*",
				"webpack:///./~/*":  webRoot + "/node_modules/*",
				// Meteor pattern
				"meteor://💻app/*": webRoot + "/*",
			}
		}
	}

	// Port for Chrome DevTools Protocol
	if port, ok := args["port"].(float64); ok {
		attachArgs["port"] = int(port)
	} else {
		attachArgs["port"] = 9222 // Default Chrome remote debugging port
	}

	// Enable source maps
	attachArgs["sourceMaps"] = true

	return attachArgs
}
