// Package mcp provides the Model Context Protocol (MCP) server implementation.
//
// This package exposes debugging capabilities through MCP tools that can be used
// by AI assistants and other MCP clients. It provides a consolidated 12-tool API:
//
// Session Management (always available):
//   - debug_launch: Launch a new debug session
//   - debug_attach: Attach to an existing process or browser
//   - debug_disconnect: Disconnect from a session
//   - debug_list_sessions: List active sessions
//
// Inspection (always available):
//   - debug_snapshot: Get complete debug state (threads, stacks, variables)
//   - debug_evaluate: Evaluate expressions in debug context
//
// Control (full mode only):
//   - debug_breakpoints: Set/clear breakpoints
//   - debug_step: Step over/into/out
//   - debug_continue: Resume execution
//   - debug_pause: Pause execution
//   - debug_set_variable: Modify variable values
//   - debug_run_to_line: Run to a specific line
package mcp

import (
	"context"

	"github.com/ctagard/dap-mcp/internal/adapters"
	"github.com/ctagard/dap-mcp/internal/config"
	"github.com/ctagard/dap-mcp/internal/dap"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with debugging capabilities
type Server struct {
	mcpServer      *server.MCPServer
	sessionManager *dap.SessionManager
	adapterReg     *adapters.Registry
	config         *config.Config
}

// NewServer creates a new DAP-MCP server
func NewServer(cfg *config.Config) *Server {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"dap-mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Create session manager
	sessionManager := dap.NewSessionManager(cfg.MaxSessions, cfg.SessionTimeout)

	// Create adapter registry
	adapterReg := adapters.NewRegistry(cfg)

	s := &Server{
		mcpServer:      mcpServer,
		sessionManager: sessionManager,
		adapterReg:     adapterReg,
		config:         cfg,
	}

	// Register all tools
	s.registerTools()

	return s
}

// registerTools is defined in tools.go with the consolidated 12-tool API

// ServeStdio starts the server using stdio transport
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

// Close shuts down the server
func (s *Server) Close() {
	s.sessionManager.Close()
}

// GetSessionManager returns the session manager
func (s *Server) GetSessionManager() *dap.SessionManager {
	return s.sessionManager
}

// GetAdapterRegistry returns the adapter registry
func (s *Server) GetAdapterRegistry() *adapters.Registry {
	return s.adapterReg
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *config.Config {
	return s.config
}

// Helper function to get context from handler
func contextFromHandler() context.Context {
	return context.Background()
}
