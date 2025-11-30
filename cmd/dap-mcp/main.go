package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/internal/mcp"
	"github.com/ctagard/dap-mcp/internal/version"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	mode := flag.String("mode", "full", "Capability mode: 'readonly' or 'full'")
	showVersion := flag.Bool("version", false, "Show version and exit")
	checkUpdate := flag.Bool("check-update", false, "Check for updates and exit")
	help := flag.Bool("help", false, "Show help and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("dap-mcp version %s\n", version.Version)
		os.Exit(0)
	}

	if *checkUpdate {
		checker := version.NewChecker()
		info := checker.CheckForUpdates(nil)
		if info.Error != "" {
			fmt.Printf("Error checking for updates: %s\n", info.Error)
			os.Exit(1)
		}
		if info.UpdateAvailable {
			fmt.Printf("Update available: v%s -> v%s\n", info.CurrentVersion, info.LatestVersion)
			fmt.Printf("Release: %s\n", info.ReleaseURL)
			fmt.Printf("\nTo update, run:\n")
			fmt.Printf("  curl -sSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash\n", version.GitHubRepo)
		} else {
			fmt.Printf("You are running the latest version (v%s)\n", info.CurrentVersion)
		}
		os.Exit(0)
	}

	if *help {
		printHelp()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override mode from command line
	if *mode == "readonly" {
		cfg.Mode = config.ModeReadOnly
	} else if *mode == "full" {
		cfg.Mode = config.ModeFull
	}

	// Start version check in background
	versionChecker := version.NewChecker()
	versionChecker.CheckForUpdatesAsync()

	// Create and start the server
	server := mcp.NewServer(cfg, versionChecker)

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		server.Close()
		os.Exit(0)
	}()

	// Start serving via stdio
	log.Println("DAP-MCP server starting...")
	if err := server.ServeStdio(); err != nil {
		server.Close()
		log.Fatalf("Server error: %v", err)
	}
	server.Close()
}

func printHelp() {
	fmt.Println(`DAP-MCP: Debug Adapter Protocol MCP Server

A Model Context Protocol (MCP) server that exposes Debug Adapter Protocol (DAP)
functionality to LLMs, enabling AI agents to introspect and debug code.

USAGE:
    dap-mcp [OPTIONS]

OPTIONS:
    -config <path>     Path to configuration file (JSON)
    -mode <mode>       Capability mode: 'readonly' or 'full' (default: full)
    -version           Show version and exit
    -help              Show this help message

SUPPORTED LANGUAGES:
    - Go (via Delve)
    - Python (via debugpy)
    - JavaScript/TypeScript (via Node.js inspector)

CONFIGURATION:
    Create a JSON configuration file to customize behavior:

    {
        "mode": "full",
        "allowSpawn": true,
        "allowAttach": true,
        "allowModify": true,
        "allowExecute": true,
        "maxSessions": 10,
        "sessionTimeout": "30m",
        "adapters": {
            "go": {
                "path": "dlv",
                "buildFlags": ""
            },
            "python": {
                "pythonPath": "python3"
            },
            "node": {
                "nodePath": "node",
                "inspectBrk": true
            }
        }
    }

MCP INTEGRATION:
    Add to your MCP client configuration:

    Claude Code (~/.claude.json):
    {
        "mcpServers": {
            "dap-mcp": {
                "command": "dap-mcp",
                "args": ["--mode", "full"]
            }
        }
    }

TOOLS:
    Session Management:
        debug_launch          Launch a new debug session
        debug_attach          Attach to an existing adapter
        debug_disconnect      End a debug session
        debug_list_sessions   List active sessions

    Inspection (read-only):
        inspect_threads       Get all threads
        inspect_stack         Get call stack
        inspect_scopes        Get variable scopes
        inspect_variables     Get variables
        inspect_evaluate      Evaluate expressions
        inspect_source        Get source code
        inspect_modules       Get loaded modules

    Control (full mode only):
        control_set_breakpoints           Set source breakpoints
        control_set_function_breakpoints  Set function breakpoints
        control_continue                  Continue execution
        control_step_over                 Step over
        control_step_into                 Step into
        control_step_out                  Step out
        control_pause                     Pause execution
        control_set_variable              Modify variables

    Convenience:
        debug_snapshot        Get complete debug state
        debug_run_to_line     Run to a specific line

For more information, visit: https://github.com/ctagard/dap-mcp`)
}
