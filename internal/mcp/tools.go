package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// registerTools registers the consolidated 12-tool debug API
func (s *Server) registerTools() {
	// Session Management (4 tools - both modes)
	s.registerDebugLaunch()
	s.registerDebugAttach()
	s.registerDebugDisconnect()
	s.registerDebugListSessions()

	// Inspection (2 tools - both modes)
	s.registerDebugSnapshot()
	s.registerDebugEvaluate()

	// Control (6 tools - full mode only)
	if s.config.CanUseControlTools() {
		s.registerDebugBreakpoints()
		s.registerDebugStep()
		s.registerDebugContinue()
		s.registerDebugPause()
		s.registerDebugSetVariable()
		s.registerDebugRunToLine()
		s.registerDebugExecuteCommand()
	}
}

// Session Management Tools

func (s *Server) registerDebugLaunch() {
	tool := mcp.NewTool("debug_launch",
		mcp.WithDescription("Launch a new debug session. Can use direct arguments OR reference a VS Code launch.json configuration. Returns sessionId needed for all other tools. Use stopOnEntry=true to pause at first line."),
		mcp.WithString("language",
			mcp.Description("Programming language: go, python, javascript, or typescript. Not required if configName is provided."),
		),
		mcp.WithString("program",
			mcp.Description("Path to the program to debug, OR URL for browser debugging. Not required if configName is provided."),
		),
		mcp.WithString("target",
			mcp.Description("Debug target: 'node' (default for JS/TS), 'chrome', or 'edge'. Use chrome/edge for React, Svelte, Vue apps"),
		),
		mcp.WithString("cwd",
			mcp.Description("Working directory for the program"),
		),
		mcp.WithString("webRoot",
			mcp.Description("Root of web app source files (for browser debugging source maps)"),
		),
		mcp.WithBoolean("stopOnEntry",
			mcp.Description("Stop on entry point (default: false)"),
		),
		// Python venv support
		mcp.WithString("pythonPath",
			mcp.Description("Path to Python interpreter (for venv support). Use this to specify a virtualenv Python, e.g., '/path/to/venv/bin/python'. Also accepts 'python' as an alias."),
		),
		// Launch.json configuration support
		mcp.WithString("configPath",
			mcp.Description("Path to launch.json file. Auto-discovers from workspace if not provided."),
		),
		mcp.WithString("configName",
			mcp.Description("Name of configuration in launch.json to use. If provided, loads settings from launch.json."),
		),
		mcp.WithString("workspace",
			mcp.Description("Workspace root for variable resolution (e.g., ${workspaceFolder}) and config discovery."),
		),
		mcp.WithString("inputValues",
			mcp.Description("JSON object with values for ${input:} variables in launch.json. Example: {\"testFile\": \"test_main.py\"}"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugLaunch)
}

func (s *Server) registerDebugAttach() {
	tool := mcp.NewTool("debug_attach",
		mcp.WithDescription("Attach to an existing debug adapter, process, or browser. Can use direct arguments OR reference a VS Code launch.json configuration."),
		mcp.WithString("language",
			mcp.Description("Programming language: go, python, javascript, or typescript. Not required if configName is provided."),
		),
		mcp.WithString("target",
			mcp.Description("Debug target: 'node' (default), 'chrome', or 'edge'. Use chrome/edge for React, Svelte, Vue apps"),
		),
		mcp.WithString("host",
			mcp.Description("Host address of the debug adapter (default: 127.0.0.1)"),
		),
		mcp.WithNumber("port",
			mcp.Description("Port of the debug adapter (default: 9229 for Node, 9222 for Chrome/Edge)"),
		),
		mcp.WithNumber("pid",
			mcp.Description("Process ID to attach to (Node.js only)"),
		),
		mcp.WithString("url",
			mcp.Description("URL pattern to match for browser tab selection"),
		),
		mcp.WithString("webRoot",
			mcp.Description("Root of web app source files (for source maps)"),
		),
		// Launch.json configuration support
		mcp.WithString("configPath",
			mcp.Description("Path to launch.json file. Auto-discovers from workspace if not provided."),
		),
		mcp.WithString("configName",
			mcp.Description("Name of attach configuration in launch.json to use."),
		),
		mcp.WithString("workspace",
			mcp.Description("Workspace root for variable resolution and config discovery."),
		),
		mcp.WithString("inputValues",
			mcp.Description("JSON object with values for ${input:} variables in launch.json."),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugAttach)
}

func (s *Server) registerDebugDisconnect() {
	tool := mcp.NewTool("debug_disconnect",
		mcp.WithDescription("Disconnect from a debug session"),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID to disconnect from"),
		),
		mcp.WithBoolean("terminateDebuggee",
			mcp.Description("Terminate the debugged process (default: false)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugDisconnect)
}

func (s *Server) registerDebugListSessions() {
	tool := mcp.NewTool("debug_list_sessions",
		mcp.WithDescription("List all active debug sessions"),
	)
	s.mcpServer.AddTool(tool, s.handleDebugListSessions)
}

// Inspection Tools

func (s *Server) registerDebugSnapshot() {
	tool := mcp.NewTool("debug_snapshot",
		mcp.WithDescription("Get complete debug state in ONE call: all threads, stack traces, scopes, and variables. This is the primary inspection tool - use it instead of making multiple individual calls. Returns: {threads, stacks, scopes, variables}."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithNumber("threadId",
			mcp.Description("Specific thread ID, or omit for all threads"),
		),
		mcp.WithNumber("maxStackDepth",
			mcp.Description("Maximum stack depth to return (default: 10)"),
		),
		mcp.WithBoolean("expandVariables",
			mcp.Description("Expand first level of complex variables (default: true)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugSnapshot)
}

func (s *Server) registerDebugEvaluate() {
	tool := mcp.NewTool("debug_evaluate",
		mcp.WithDescription("Evaluate one or more expressions in current debug context. Supports single expression OR batch mode for multiple expressions at once."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithString("expression",
			mcp.Description("Single expression to evaluate (e.g., 'len(my_list)', 'x + y')"),
		),
		mcp.WithString("expressions",
			mcp.Description("JSON array of expressions for batch evaluation: [\"x\", \"y\", \"len(arr)\"]"),
		),
		mcp.WithNumber("frameId",
			mcp.Description("Stack frame ID for context (default: top frame)"),
		),
		mcp.WithString("context",
			mcp.Description("Evaluation context: 'watch', 'hover', or 'repl' (default: 'watch')"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugEvaluate)
}

// Control Tools (Full mode only)

func (s *Server) registerDebugBreakpoints() {
	tool := mcp.NewTool("debug_breakpoints",
		mcp.WithDescription("Set breakpoints in a source file. Supports conditional breakpoints with 'condition' field. Note: This REPLACES all breakpoints in the file - include all desired breakpoints in each call."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("The source file path"),
		),
		mcp.WithString("breakpoints",
			mcp.Required(),
			mcp.Description("JSON array of breakpoints: [{line: number, condition?: string, hitCondition?: string, logMessage?: string}]"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugBreakpoints)
}

func (s *Server) registerDebugStep() {
	tool := mcp.NewTool("debug_step",
		mcp.WithDescription("Execute a step command. Use type='over' to step to next line, 'into' to enter function calls, 'out' to exit current function. Follow with debug_snapshot to see new state."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithNumber("threadId",
			mcp.Required(),
			mcp.Description("The thread ID to step"),
		),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Step type: 'over' (next line), 'into' (enter function), 'out' (exit function)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugStep)
}

func (s *Server) registerDebugContinue() {
	tool := mcp.NewTool("debug_continue",
		mcp.WithDescription("Continue program execution until next breakpoint or program end. Returns immediately - use debug_snapshot to check state after stopping. For 'run to line X', use debug_run_to_line instead."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithNumber("threadId",
			mcp.Required(),
			mcp.Description("The thread ID to continue"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugContinue)
}

func (s *Server) registerDebugPause() {
	tool := mcp.NewTool("debug_pause",
		mcp.WithDescription("Pause program execution. Use when program is running and you need to inspect state."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithNumber("threadId",
			mcp.Required(),
			mcp.Description("The thread ID to pause"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugPause)
}

func (s *Server) registerDebugSetVariable() {
	tool := mcp.NewTool("debug_set_variable",
		mcp.WithDescription("Modify the value of a variable during debugging. Use variablesReference from debug_snapshot to identify the scope."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithNumber("variablesReference",
			mcp.Required(),
			mcp.Description("The variables reference containing the variable (from debug_snapshot)"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The variable name to modify"),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("The new value to set"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugSetVariable)
}

func (s *Server) registerDebugRunToLine() {
	tool := mcp.NewTool("debug_run_to_line",
		mcp.WithDescription("Run until execution reaches a specific line. Sets temp breakpoint, continues, waits for stop, and returns a snapshot with stack and local variables. More efficient than set breakpoint + continue + snapshot."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("The source file path"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("The line number to run to"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugRunToLine)
}

func (s *Server) registerDebugExecuteCommand() {
	tool := mcp.NewTool("debug_execute_command",
		mcp.WithDescription("Execute a native debugger CLI command. ONLY for GDB/LLDB sessions (C, C++, Rust, Objective-C, Swift). "+
			"Supports Python scripting via 'script' (LLDB) or 'python' (GDB) commands. "+
			"Examples: 'disassemble main', 'memory read 0x1000', 'script print(lldb.frame)'. "+
			"NOT available for Go, Python, JavaScript/TypeScript - use debug_evaluate for those."),
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("The session ID (must be a GDB or LLDB session)"),
		),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The debugger command to execute"),
		),
		mcp.WithNumber("frameId",
			mcp.Description("Stack frame ID for context (default: top frame of first thread)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDebugExecuteCommand)
}
