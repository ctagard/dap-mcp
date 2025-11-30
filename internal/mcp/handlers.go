package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/ctagard/dap-mcp/internal/adapters"
	internaldap "github.com/ctagard/dap-mcp/internal/dap"
	"github.com/ctagard/dap-mcp/internal/errors"
	"github.com/ctagard/dap-mcp/internal/launchconfig"
	"github.com/ctagard/dap-mcp/pkg/types"
	"github.com/google/go-dap"
	"github.com/mark3labs/mcp-go/mcp"
)

// Session Management Handlers

func (s *Server) handleDebugLaunch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if this is a config-based launch
	configName, _ := request.RequireString("configName")
	if configName != "" {
		return s.handleConfigBasedLaunch(ctx, request, configName)
	}

	// Direct args launch (original behavior)
	langStr, err := request.RequireString("language")
	if err != nil {
		return mcp.NewToolResultError(errors.MissingParameter("language",
			"Specify the programming language: 'go', 'python', 'javascript', 'typescript', 'c', or 'rust'. Alternatively, use configName to load from launch.json.").Error()), nil
	}

	program, err := request.RequireString("program")
	if err != nil {
		return mcp.NewToolResultError(errors.MissingParameter("program",
			"Specify the path to the program to debug. For Go: path to main package directory. For Python/JS: path to the script file. Alternatively, use configName to load from launch.json.").Error()), nil
	}

	lang := types.Language(langStr)

	// Get the adapter for this language
	adapter, err := s.adapterReg.Get(lang)
	if err != nil {
		return mcp.NewToolResultError(errors.AdapterNotSupported(langStr, []string{"go", "python", "javascript", "typescript", "c", "rust"}).Error()), nil
	}

	// Create a new session
	session, err := s.sessionManager.CreateSession(lang, program)
	if err != nil {
		return mcp.NewToolResultError(errors.SessionLimitReached(10).Error()), nil // Uses default max; ideally would get actual max
	}

	// Build launch arguments from request
	args := make(map[string]interface{})
	if cwd, err := request.RequireString("cwd"); err == nil {
		args["cwd"] = cwd
	}
	if stopOnEntry := request.GetBool("stopOnEntry", false); stopOnEntry {
		args["stopOnEntry"] = true
	}
	// Browser debugging options
	if target, err := request.RequireString("target"); err == nil {
		args["target"] = target
	}
	if webRoot, err := request.RequireString("webRoot"); err == nil {
		args["webRoot"] = webRoot
	}
	// Python interpreter path for venv support (supports both "python" and "pythonPath")
	if pythonPath, err := request.RequireString("pythonPath"); err == nil {
		args["pythonPath"] = pythonPath
		args["python"] = pythonPath // Also set VS Code style for compatibility
	}
	if python, err := request.RequireString("python"); err == nil {
		args["python"] = python     // VS Code style takes precedence
		args["pythonPath"] = python // Also set debugpy style
	}

	// Spawn the debug adapter if allowed
	if !s.config.CanSpawn() {
		s.sessionManager.TerminateSession(session.ID, false)
		return mcp.NewToolResultError(errors.PermissionDenied("spawn", string(s.config.Mode)).Error()), nil
	}

	// SpawnAndConnect handles both TCP and stdio-based adapters
	client, cmd, err := adapters.SpawnAndConnect(ctx, adapter, program, args)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, false)
		return mcp.NewToolResultError(errors.AdapterSpawnFailed(langStr, err).Error()), nil
	}

	if cmd != nil && cmd.Process != nil {
		s.sessionManager.SetSessionProcess(session.ID, cmd, cmd.Process.Pid)
	}

	s.sessionManager.SetSessionClient(session.ID, client)

	// Initialize the debug adapter
	_, err = client.Initialize("dap-mcp", "DAP-MCP Server")
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(errors.DAPInitFailed(err).Error()), nil
	}

	// Launch the program asynchronously - debugpy won't respond until after configurationDone
	launchArgs := adapter.BuildLaunchArgs(program, args)
	launchRespCh, err := client.LaunchAsync(launchArgs)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(errors.DAPLaunchFailed(program, err).Error()), nil
	}

	// Wait for initialized event
	if err := client.WaitInitialized(10 * time.Second); err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(errors.DAPTimeout("waiting for initialized event", 10).Error()), nil
	}

	// Signal configuration done - debugpy needs this before it will send launch response
	if err := client.ConfigurationDone(); err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(errors.Wrap(errors.CodeDAPProtocolError, "configuration done failed", "The debug adapter rejected the configuration. Try launching with simpler options.", err).Error()), nil
	}

	// Now wait for the launch response
	_, err = client.WaitForLaunchResponse(launchRespCh, 10*time.Second)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(errors.DAPLaunchFailed(program, err).Error()), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusRunning)

	result := map[string]interface{}{
		"sessionId": session.ID,
		"status":    "launched",
		"language":  string(lang),
		"program":   program,
	}
	if cmd != nil && cmd.Process != nil {
		result["pid"] = cmd.Process.Pid
	}

	return jsonResult(result)
}

func (s *Server) handleDebugAttach(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	langStr, err := request.RequireString("language")
	if err != nil {
		return mcp.NewToolResultError(errors.MissingParameter("language",
			"Specify the programming language of the process to attach to: 'go', 'python', 'javascript', 'typescript'.").Error()), nil
	}

	if !s.config.CanAttach() {
		return mcp.NewToolResultError(errors.PermissionDenied("attach", string(s.config.Mode)).Error()), nil
	}

	lang := types.Language(langStr)

	adapter, err := s.adapterReg.Get(lang)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	session, err := s.sessionManager.CreateSession(lang, "attached")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get connection details
	host := "127.0.0.1"
	if h, err := request.RequireString("host"); err == nil {
		host = h
	}

	port, err := request.RequireFloat("port")
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, false)
		return mcp.NewToolResultError("port is required for attach"), nil
	}

	// Build attach args early to check target type
	args := map[string]interface{}{
		"host": host,
		"port": port,
	}
	if pid, err := request.RequireFloat("pid"); err == nil {
		args["pid"] = pid
	}

	// Browser debugging options
	target := ""
	if t, err := request.RequireString("target"); err == nil {
		target = t
		args["target"] = target
	}
	if url, err := request.RequireString("url"); err == nil {
		args["url"] = url
	}
	if webRoot, err := request.RequireString("webRoot"); err == nil {
		args["webRoot"] = webRoot
	}

	var client *internaldap.Client
	var address string

	// For browser targets (chrome/edge), we need to spawn vscode-js-debug first
	// because Chrome speaks CDP (Chrome DevTools Protocol), not DAP
	if target == "chrome" || target == "edge" {
		// Check if spawning is allowed (needed for vscode-js-debug)
		if !s.config.CanSpawn() {
			s.sessionManager.TerminateSession(session.ID, false)
			return mcp.NewToolResultError("spawning debug adapters is not allowed (required for browser attach)"), nil
		}

		// Spawn vscode-js-debug as the DAP-to-CDP translator
		// We pass empty program since we're attaching, not launching
		var cmd *exec.Cmd
		address, cmd, err = adapter.Spawn(ctx, "", args)
		if err != nil {
			s.sessionManager.TerminateSession(session.ID, false)
			return mcp.NewToolResultError(fmt.Sprintf("failed to spawn adapter: %v", err)), nil
		}

		if cmd != nil && cmd.Process != nil {
			s.sessionManager.SetSessionProcess(session.ID, cmd, cmd.Process.Pid)
		}

		// Connect to vscode-js-debug (not Chrome directly)
		client, err = adapters.Connect(address, 20)
		if err != nil {
			s.sessionManager.TerminateSession(session.ID, true)
			return mcp.NewToolResultError(fmt.Sprintf("failed to connect to adapter: %v", err)), nil
		}
	} else {
		// For Node.js attach, connect directly to the debug port
		// Node.js with --inspect speaks DAP-compatible protocol
		address = fmt.Sprintf("%s:%d", host, int(port))
		client, err = adapters.Connect(address, 10)
		if err != nil {
			s.sessionManager.TerminateSession(session.ID, false)
			return mcp.NewToolResultError(fmt.Sprintf("failed to connect: %v", err)), nil
		}
	}

	s.sessionManager.SetSessionClient(session.ID, client)

	// Initialize the DAP session
	_, err = client.Initialize("dap-mcp", "DAP-MCP Server")
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(fmt.Sprintf("failed to initialize: %v", err)), nil
	}

	// Build and send attach request
	attachArgs := adapter.BuildAttachArgs(args)

	// For browser attach, use async pattern like launch does
	if target == "chrome" || target == "edge" {
		attachRespCh, err := client.AttachAsync(attachArgs)
		if err != nil {
			s.sessionManager.TerminateSession(session.ID, true)
			return mcp.NewToolResultError(fmt.Sprintf("failed to attach: %v", err)), nil
		}

		// Wait for initialized event
		if err := client.WaitInitialized(10 * time.Second); err != nil {
			s.sessionManager.TerminateSession(session.ID, true)
			return mcp.NewToolResultError(fmt.Sprintf("failed waiting for initialized: %v", err)), nil
		}

		// Signal configuration done
		if err := client.ConfigurationDone(); err != nil {
			s.sessionManager.TerminateSession(session.ID, true)
			return mcp.NewToolResultError(fmt.Sprintf("configuration failed: %v", err)), nil
		}

		// Wait for attach response
		_, err = client.WaitForAttachResponse(attachRespCh, 10*time.Second)
		if err != nil {
			s.sessionManager.TerminateSession(session.ID, true)
			return mcp.NewToolResultError(fmt.Sprintf("attach failed: %v", err)), nil
		}
	} else {
		// For Node.js, use synchronous attach
		_, err = client.Attach(attachArgs)
		if err != nil {
			s.sessionManager.TerminateSession(session.ID, false)
			return mcp.NewToolResultError(fmt.Sprintf("failed to attach: %v", err)), nil
		}

		if err := client.ConfigurationDone(); err != nil {
			s.sessionManager.TerminateSession(session.ID, false)
			return mcp.NewToolResultError(fmt.Sprintf("configuration failed: %v", err)), nil
		}
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusRunning)

	return jsonResult(map[string]interface{}{
		"sessionId": session.ID,
		"status":    "attached",
		"language":  string(lang),
	})
}

func (s *Server) handleDebugDisconnect(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("sessionId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	terminateDebuggee := request.GetBool("terminateDebuggee", false)

	if err := s.sessionManager.TerminateSession(sessionID, terminateDebuggee); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]interface{}{
		"sessionId": sessionID,
		"status":    "disconnected",
	})
}

func (s *Server) handleDebugListSessions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessions := s.sessionManager.ListSessions()

	result := make([]map[string]interface{}, len(sessions))
	for i, session := range sessions {
		result[i] = map[string]interface{}{
			"sessionId": session.ID,
			"language":  string(session.Language),
			"status":    string(session.Status),
			"program":   session.Program,
		}
		if session.PID > 0 {
			result[i]["pid"] = session.PID
		}
	}

	return jsonResult(map[string]interface{}{
		"sessions": result,
	})
}

// Inspection Handlers

func (s *Server) handleInspectThreads(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	_ = session

	threads, err := client.Threads()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get threads: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(threads))
	for i, t := range threads {
		result[i] = map[string]interface{}{
			"id":   t.Id,
			"name": t.Name,
		}
	}

	return jsonResult(map[string]interface{}{
		"threads": result,
	})
}

func (s *Server) handleInspectStack(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	startFrame := 0
	if sf, err := request.RequireFloat("startFrame"); err == nil {
		startFrame = int(sf)
	}

	levels := 20
	if l, err := request.RequireFloat("levels"); err == nil {
		levels = int(l)
	}

	frames, totalFrames, err := client.StackTrace(int(threadID), startFrame, levels)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get stack trace: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(frames))
	for i, f := range frames {
		frame := map[string]interface{}{
			"id":   f.Id,
			"name": f.Name,
			"line": f.Line,
		}
		if f.Column > 0 {
			frame["column"] = f.Column
		}
		if f.Source != nil {
			frame["source"] = map[string]interface{}{
				"name":            f.Source.Name,
				"path":            f.Source.Path,
				"sourceReference": f.Source.SourceReference,
			}
		}
		result[i] = frame
	}

	return jsonResult(map[string]interface{}{
		"stackFrames": result,
		"totalFrames": totalFrames,
	})
}

func (s *Server) handleInspectScopes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	frameID, err := request.RequireFloat("frameId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	scopes, err := client.Scopes(int(frameID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get scopes: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(scopes))
	for i, scope := range scopes {
		result[i] = map[string]interface{}{
			"name":               scope.Name,
			"variablesReference": scope.VariablesReference,
			"expensive":          scope.Expensive,
		}
		if scope.NamedVariables > 0 {
			result[i]["namedVariables"] = scope.NamedVariables
		}
		if scope.IndexedVariables > 0 {
			result[i]["indexedVariables"] = scope.IndexedVariables
		}
	}

	return jsonResult(map[string]interface{}{
		"scopes": result,
	})
}

func (s *Server) handleInspectVariables(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	varsRef, err := request.RequireFloat("variablesReference")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	filter := ""
	if f, err := request.RequireString("filter"); err == nil {
		filter = f
	}

	start := 0
	if st, err := request.RequireFloat("start"); err == nil {
		start = int(st)
	}

	count := 0
	if c, err := request.RequireFloat("count"); err == nil {
		count = int(c)
	}

	vars, err := client.Variables(int(varsRef), filter, start, count)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get variables: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(vars))
	for i, v := range vars {
		variable := map[string]interface{}{
			"name":               v.Name,
			"value":              v.Value,
			"variablesReference": v.VariablesReference,
		}
		if v.Type != "" {
			variable["type"] = v.Type
		}
		if v.NamedVariables > 0 {
			variable["namedVariables"] = v.NamedVariables
		}
		if v.IndexedVariables > 0 {
			variable["indexedVariables"] = v.IndexedVariables
		}
		result[i] = variable
	}

	return jsonResult(map[string]interface{}{
		"variables": result,
	})
}

func (s *Server) handleInspectEvaluate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.CanEvaluate() {
		return mcp.NewToolResultError("expression evaluation is not allowed"), nil
	}

	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	expression, err := request.RequireString("expression")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	frameID := 0
	if f, err := request.RequireFloat("frameId"); err == nil {
		frameID = int(f)
	}

	evalContext := "watch"
	if c, err := request.RequireString("context"); err == nil {
		evalContext = c
	}

	result, err := client.Evaluate(expression, frameID, evalContext)
	if err != nil {
		return mcp.NewToolResultError(errors.EvaluationFailed(expression, err).Error()), nil
	}

	return jsonResult(map[string]interface{}{
		"result":             result.Result,
		"type":               result.Type,
		"variablesReference": result.VariablesReference,
	})
}

func (s *Server) handleInspectSource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sourceRef := 0
	if sr, err := request.RequireFloat("sourceReference"); err == nil {
		sourceRef = int(sr)
	}

	path := ""
	if p, err := request.RequireString("path"); err == nil {
		path = p
	}

	content, mimeType, err := client.Source(sourceRef, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get source: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"content":  content,
		"mimeType": mimeType,
	})
}

func (s *Server) handleInspectModules(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	modules, total, err := client.Modules(0, 100)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get modules: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(modules))
	for i, m := range modules {
		result[i] = map[string]interface{}{
			"id":   m.Id,
			"name": m.Name,
		}
		if m.Path != "" {
			result[i]["path"] = m.Path
		}
		if m.Version != "" {
			result[i]["version"] = m.Version
		}
	}

	return jsonResult(map[string]interface{}{
		"modules":      result,
		"totalModules": total,
	})
}

// Control Handlers

func (s *Server) handleControlSetBreakpoints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bpsJSON, err := request.RequireString("breakpoints")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var bpRequests []struct {
		Line         int    `json:"line"`
		Condition    string `json:"condition,omitempty"`
		HitCondition string `json:"hitCondition,omitempty"`
		LogMessage   string `json:"logMessage,omitempty"`
	}

	if err := json.Unmarshal([]byte(bpsJSON), &bpRequests); err != nil {
		return mcp.NewToolResultError(errors.InvalidJSON("breakpoints", err, `[{"line": 10}, {"line": 20, "condition": "x > 5"}]`).Error()), nil
	}

	source := dap.Source{
		Path: path,
	}

	breakpoints := make([]dap.SourceBreakpoint, len(bpRequests))
	for i, bp := range bpRequests {
		breakpoints[i] = dap.SourceBreakpoint{
			Line:         bp.Line,
			Condition:    bp.Condition,
			HitCondition: bp.HitCondition,
			LogMessage:   bp.LogMessage,
		}
	}

	bps, err := client.SetBreakpoints(source, breakpoints)
	if err != nil {
		return mcp.NewToolResultError(errors.Wrap(errors.CodeBreakpointFailed, fmt.Sprintf("failed to set breakpoints in %s", path), "Ensure the file path is correct and the line numbers contain executable code.", err).Error()), nil
	}

	result := make([]map[string]interface{}, len(bps))
	for i, bp := range bps {
		result[i] = map[string]interface{}{
			"id":       bp.Id,
			"verified": bp.Verified,
			"line":     bp.Line,
		}
		if bp.Message != "" {
			result[i]["message"] = bp.Message
		}
	}

	return jsonResult(map[string]interface{}{
		"breakpoints": result,
	})
}

func (s *Server) handleControlSetFunctionBreakpoints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bpsJSON, err := request.RequireString("breakpoints")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var bpRequests []struct {
		Name      string `json:"name"`
		Condition string `json:"condition,omitempty"`
	}

	if err := json.Unmarshal([]byte(bpsJSON), &bpRequests); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid breakpoints JSON: %v", err)), nil
	}

	breakpoints := make([]dap.FunctionBreakpoint, len(bpRequests))
	for i, bp := range bpRequests {
		breakpoints[i] = dap.FunctionBreakpoint{
			Name:      bp.Name,
			Condition: bp.Condition,
		}
	}

	bps, err := client.SetFunctionBreakpoints(breakpoints)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set function breakpoints: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(bps))
	for i, bp := range bps {
		result[i] = map[string]interface{}{
			"id":       bp.Id,
			"verified": bp.Verified,
		}
		if bp.Message != "" {
			result[i]["message"] = bp.Message
		}
	}

	return jsonResult(map[string]interface{}{
		"breakpoints": result,
	})
}

func (s *Server) handleControlContinue(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	allContinued, err := client.Continue(int(threadID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("continue failed: %v", err)), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusRunning)

	return jsonResult(map[string]interface{}{
		"allThreadsContinued": allContinued,
	})
}

func (s *Server) handleControlStepOver(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.Next(int(threadID)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("step over failed: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"status": "stepped",
	})
}

func (s *Server) handleControlStepInto(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.StepIn(int(threadID)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("step into failed: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"status": "stepped",
	})
}

func (s *Server) handleControlStepOut(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.StepOut(int(threadID)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("step out failed: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"status": "stepped",
	})
}

func (s *Server) handleControlPause(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.Pause(int(threadID)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("pause failed: %v", err)), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusStopped)

	return jsonResult(map[string]interface{}{
		"status": "paused",
	})
}

func (s *Server) handleControlSetVariable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.CanModifyVariables() {
		return mcp.NewToolResultError("variable modification is not allowed"), nil
	}

	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	varsRef, err := request.RequireFloat("variablesReference")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	value, err := request.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := client.SetVariable(int(varsRef), name, value)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set variable failed: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"value":              result.Value,
		"type":               result.Type,
		"variablesReference": result.VariablesReference,
	})
}

// Consolidated Control Handlers

// handleDebugStep consolidates step_over, step_into, step_out into one tool with type parameter
func (s *Server) handleDebugStep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	stepType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	switch stepType {
	case "over":
		if err := client.Next(int(threadID)); err != nil {
			return mcp.NewToolResultError(errors.StepFailed("over", err).Error()), nil
		}
	case "into":
		if err := client.StepIn(int(threadID)); err != nil {
			return mcp.NewToolResultError(errors.StepFailed("into", err).Error()), nil
		}
	case "out":
		if err := client.StepOut(int(threadID)); err != nil {
			return mcp.NewToolResultError(errors.StepFailed("out", err).Error()), nil
		}
	default:
		return mcp.NewToolResultError(errors.InvalidParameter("type", stepType, "'over', 'into', or 'out'").Error()), nil
	}

	return jsonResult(map[string]interface{}{
		"status": "stepped",
		"type":   stepType,
	})
}

// handleDebugEvaluate consolidates single and batch expression evaluation
func (s *Server) handleDebugEvaluate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.CanEvaluate() {
		return mcp.NewToolResultError(errors.PermissionDenied("evaluate", string(s.config.Mode)).Error()), nil
	}

	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Check for batch mode first
	expressionsJSON, _ := request.RequireString("expressions")
	if expressionsJSON != "" {
		var expressions []string
		if err := json.Unmarshal([]byte(expressionsJSON), &expressions); err != nil {
			return mcp.NewToolResultError(errors.InvalidJSON("expressions", err, `["x", "y", "len(arr)"]`).Error()), nil
		}

		frameID := 0
		if f, err := request.RequireFloat("frameId"); err == nil {
			frameID = int(f)
		} else {
			// Try to get the top frame automatically
			threads, err := client.Threads()
			if err == nil && len(threads) > 0 {
				frames, _, err := client.StackTrace(threads[0].Id, 0, 1)
				if err == nil && len(frames) > 0 {
					frameID = frames[0].Id
				}
			}
		}

		results := make([]map[string]interface{}, len(expressions))
		for i, expr := range expressions {
			result, err := client.Evaluate(expr, frameID, "watch")
			if err != nil {
				results[i] = map[string]interface{}{
					"expression": expr,
					"error":      err.Error(),
				}
			} else {
				results[i] = map[string]interface{}{
					"expression":         expr,
					"result":             result.Result,
					"type":               result.Type,
					"variablesReference": result.VariablesReference,
				}
			}
		}

		return jsonResult(map[string]interface{}{
			"evaluations": results,
			"frameId":     frameID,
		})
	}

	// Single expression mode
	expression, err := request.RequireString("expression")
	if err != nil {
		return mcp.NewToolResultError(errors.MissingParameter("expression",
			"Provide either 'expression' for a single evaluation (e.g., \"x + y\") or 'expressions' for batch evaluation (e.g., [\"x\", \"y\"]).").Error()), nil
	}

	frameID := 0
	if f, err := request.RequireFloat("frameId"); err == nil {
		frameID = int(f)
	}

	evalContext := "watch"
	if c, err := request.RequireString("context"); err == nil {
		evalContext = c
	}

	result, err := client.Evaluate(expression, frameID, evalContext)
	if err != nil {
		return mcp.NewToolResultError(errors.EvaluationFailed(expression, err).Error()), nil
	}

	return jsonResult(map[string]interface{}{
		"result":             result.Result,
		"type":               result.Type,
		"variablesReference": result.VariablesReference,
	})
}

// handleDebugBreakpoints handles setting breakpoints (renamed from control_set_breakpoints)
func (s *Server) handleDebugBreakpoints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bpsJSON, err := request.RequireString("breakpoints")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var bpRequests []struct {
		Line         int    `json:"line"`
		Condition    string `json:"condition,omitempty"`
		HitCondition string `json:"hitCondition,omitempty"`
		LogMessage   string `json:"logMessage,omitempty"`
	}

	if err := json.Unmarshal([]byte(bpsJSON), &bpRequests); err != nil {
		return mcp.NewToolResultError(errors.InvalidJSON("breakpoints", err, `[{"line": 10}, {"line": 20, "condition": "x > 5"}]`).Error()), nil
	}

	source := dap.Source{
		Path: path,
	}

	breakpoints := make([]dap.SourceBreakpoint, len(bpRequests))
	for i, bp := range bpRequests {
		breakpoints[i] = dap.SourceBreakpoint{
			Line:         bp.Line,
			Condition:    bp.Condition,
			HitCondition: bp.HitCondition,
			LogMessage:   bp.LogMessage,
		}
	}

	bps, err := client.SetBreakpoints(source, breakpoints)
	if err != nil {
		return mcp.NewToolResultError(errors.Wrap(errors.CodeBreakpointFailed, fmt.Sprintf("failed to set breakpoints in %s", path), "Ensure the file path is correct and the line numbers contain executable code.", err).Error()), nil
	}

	result := make([]map[string]interface{}, len(bps))
	for i, bp := range bps {
		result[i] = map[string]interface{}{
			"id":       bp.Id,
			"verified": bp.Verified,
			"line":     bp.Line,
		}
		if bp.Message != "" {
			result[i]["message"] = bp.Message
		}
	}

	return jsonResult(map[string]interface{}{
		"breakpoints": result,
	})
}

// handleDebugContinue handles continuing execution (renamed from control_continue)
func (s *Server) handleDebugContinue(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	allContinued, err := client.Continue(int(threadID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("continue failed: %v", err)), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusRunning)

	return jsonResult(map[string]interface{}{
		"allThreadsContinued": allContinued,
	})
}

// handleDebugPause handles pausing execution (renamed from control_pause)
func (s *Server) handleDebugPause(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threadID, err := request.RequireFloat("threadId")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.Pause(int(threadID)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("pause failed: %v", err)), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusStopped)

	return jsonResult(map[string]interface{}{
		"status": "paused",
	})
}

// handleDebugSetVariable handles modifying variables (renamed from control_set_variable)
func (s *Server) handleDebugSetVariable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.CanModifyVariables() {
		return mcp.NewToolResultError("variable modification is not allowed"), nil
	}

	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	varsRef, err := request.RequireFloat("variablesReference")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	value, err := request.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := client.SetVariable(int(varsRef), name, value)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set variable failed: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"value":              result.Value,
		"type":               result.Type,
		"variablesReference": result.VariablesReference,
	})
}

// Convenience Handlers

func (s *Server) handleDebugSnapshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	maxStackDepth := 10
	if d, err := request.RequireFloat("maxStackDepth"); err == nil {
		maxStackDepth = int(d)
	}

	expandVariables := request.GetBool("expandVariables", true)

	// Get all threads
	threads, err := client.Threads()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get threads: %v", err)), nil
	}

	// Filter to specific thread if requested
	var targetThreadID *int
	if tid, err := request.RequireFloat("threadId"); err == nil {
		t := int(tid)
		targetThreadID = &t
	}

	snapshot := map[string]interface{}{
		"sessionId": session.ID,
		"status":    string(session.Status),
	}

	threadsInfo := make([]map[string]interface{}, 0)
	stacks := make(map[string]interface{})
	scopes := make(map[string]interface{})
	variables := make(map[string]interface{})

	for _, thread := range threads {
		if targetThreadID != nil && thread.Id != *targetThreadID {
			continue
		}

		threadsInfo = append(threadsInfo, map[string]interface{}{
			"id":   thread.Id,
			"name": thread.Name,
		})

		// Get stack trace
		frames, _, err := client.StackTrace(thread.Id, 0, maxStackDepth)
		if err != nil {
			continue
		}

		framesList := make([]map[string]interface{}, len(frames))
		for i, f := range frames {
			frame := map[string]interface{}{
				"id":   f.Id,
				"name": f.Name,
				"line": f.Line,
			}
			if f.Source != nil {
				frame["source"] = map[string]interface{}{
					"path": f.Source.Path,
					"name": f.Source.Name,
				}
			}
			framesList[i] = frame

			// Get scopes for top frames
			if i < 3 {
				frameScopes, err := client.Scopes(f.Id)
				if err == nil {
					scopesList := make([]map[string]interface{}, len(frameScopes))
					for j, scope := range frameScopes {
						scopesList[j] = map[string]interface{}{
							"name":               scope.Name,
							"variablesReference": scope.VariablesReference,
						}

						// Expand variables if requested
						if expandVariables && scope.VariablesReference > 0 && !scope.Expensive {
							vars, err := client.Variables(scope.VariablesReference, "", 0, 50)
							if err == nil {
								varsList := make([]map[string]interface{}, len(vars))
								for k, v := range vars {
									varsList[k] = map[string]interface{}{
										"name":               v.Name,
										"value":              v.Value,
										"type":               v.Type,
										"variablesReference": v.VariablesReference,
									}
								}
								variables[fmt.Sprintf("%d", scope.VariablesReference)] = varsList
							}
						}
					}
					scopes[fmt.Sprintf("%d", f.Id)] = scopesList
				}
			}
		}
		stacks[fmt.Sprintf("%d", thread.Id)] = framesList
	}

	snapshot["threads"] = threadsInfo
	snapshot["stacks"] = stacks
	snapshot["scopes"] = scopes
	if expandVariables {
		snapshot["variables"] = variables
	}

	return jsonResult(snapshot)
}

func (s *Server) handleDebugRunToLine(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	line, err := request.RequireFloat("line")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Set a temporary breakpoint
	source := dap.Source{Path: path}
	bps, err := client.SetBreakpoints(source, []dap.SourceBreakpoint{{Line: int(line)}})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set breakpoint: %v", err)), nil
	}

	if len(bps) == 0 || !bps[0].Verified {
		return mcp.NewToolResultError("could not set breakpoint at specified line"), nil
	}

	// Get threads and continue the first stopped one
	threads, err := client.Threads()
	if err != nil {
		return mcp.NewToolResultError(errors.Wrap(errors.CodeDAPProtocolError, "failed to get threads", "The program may have terminated. Use debug_snapshot to check session status.", err).Error()), nil
	}

	if len(threads) == 0 {
		return mcp.NewToolResultError(errors.NoThreads().Error()), nil
	}

	// Continue and wait for stop (30 second timeout)
	stoppedInfo, err := client.ContinueAndWait(threads[0].Id, 30*time.Second)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run to line failed: %v", err)), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusStopped)

	// Build a snapshot of current state
	snapshot := map[string]interface{}{
		"sessionId":  session.ID,
		"status":     "stopped",
		"stoppedAt":  bps[0].Line,
		"reason":     stoppedInfo.Reason,
		"path":       path,
	}

	// Get stack trace for stopped thread
	frames, _, err := client.StackTrace(stoppedInfo.ThreadID, 0, 5)
	if err == nil && len(frames) > 0 {
		framesList := make([]map[string]interface{}, len(frames))
		for i, f := range frames {
			frame := map[string]interface{}{
				"id":   f.Id,
				"name": f.Name,
				"line": f.Line,
			}
			if f.Source != nil {
				frame["source"] = f.Source.Path
			}
			framesList[i] = frame
		}
		snapshot["stack"] = framesList

		// Get variables for top frame
		if len(frames) > 0 {
			scopes, err := client.Scopes(frames[0].Id)
			if err == nil {
				for _, scope := range scopes {
					if scope.Name == "Locals" && scope.VariablesReference > 0 {
						vars, err := client.Variables(scope.VariablesReference, "", 0, 20)
						if err == nil {
							varsList := make([]map[string]interface{}, len(vars))
							for i, v := range vars {
								varsList[i] = map[string]interface{}{
									"name":  v.Name,
									"value": v.Value,
									"type":  v.Type,
								}
							}
							snapshot["locals"] = varsList
						}
						break
					}
				}
			}
		}
	}

	return jsonResult(snapshot)
}

func (s *Server) handleDebugBatchEvaluate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.CanEvaluate() {
		return mcp.NewToolResultError("expression evaluation is not allowed"), nil
	}

	_, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	expressionsJSON, err := request.RequireString("expressions")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var expressions []string
	if err := json.Unmarshal([]byte(expressionsJSON), &expressions); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid expressions JSON: %v", err)), nil
	}

	frameID := 0
	if f, err := request.RequireFloat("frameId"); err == nil {
		frameID = int(f)
	} else {
		// Try to get the top frame automatically
		threads, err := client.Threads()
		if err == nil && len(threads) > 0 {
			frames, _, err := client.StackTrace(threads[0].Id, 0, 1)
			if err == nil && len(frames) > 0 {
				frameID = frames[0].Id
			}
		}
	}

	results := make([]map[string]interface{}, len(expressions))
	for i, expr := range expressions {
		result, err := client.Evaluate(expr, frameID, "watch")
		if err != nil {
			results[i] = map[string]interface{}{
				"expression": expr,
				"error":      err.Error(),
			}
		} else {
			results[i] = map[string]interface{}{
				"expression":         expr,
				"result":             result.Result,
				"type":               result.Type,
				"variablesReference": result.VariablesReference,
			}
		}
	}

	return jsonResult(map[string]interface{}{
		"evaluations": results,
		"frameId":     frameID,
	})
}

// handleDebugExecuteCommand executes a native debugger CLI command (GDB/LLDB only)
func (s *Server) handleDebugExecuteCommand(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, client, err := s.getSessionClient(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate this is a GDB or LLDB session (C, C++, Rust, etc.)
	lang := session.Language
	if lang != types.LanguageC && lang != types.LanguageRust {
		return mcp.NewToolResultError(fmt.Sprintf(
			"debug_execute_command only works with GDB/LLDB sessions (C, C++, Rust). "+
				"Current session language: %s. Use debug_evaluate for Go/Python/JavaScript.", lang)), nil
	}

	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get frame ID for context, default to finding the top frame
	frameID := 0
	if f, err := request.RequireFloat("frameId"); err == nil {
		frameID = int(f)
	} else {
		// Try to get the top frame automatically
		threads, err := client.Threads()
		if err == nil && len(threads) > 0 {
			frames, _, err := client.StackTrace(threads[0].Id, 0, 1)
			if err == nil && len(frames) > 0 {
				frameID = frames[0].Id
			}
		}
	}

	// For LLDB, use backtick prefix to ensure command mode
	// lldb-dap with --repl-mode=auto will execute this as a command
	evalCommand := "`" + command

	// Execute the command using the repl context
	result, err := client.Evaluate(evalCommand, frameID, "repl")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("command execution failed: %v", err)), nil
	}

	return jsonResult(map[string]interface{}{
		"output":             result.Result,
		"type":               result.Type,
		"variablesReference": result.VariablesReference,
	})
}

// Helper functions

func (s *Server) getSessionClient(request mcp.CallToolRequest) (*internaldap.Session, *internaldap.Client, error) {
	sessionID, err := request.RequireString("sessionId")
	if err != nil {
		return nil, nil, errors.MissingParameter("sessionId", "Provide the sessionId returned from debug_launch or debug_attach. Use debug_list_sessions to see active sessions.")
	}

	session, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, nil, errors.SessionNotFound(sessionID)
	}

	if session.Client == nil {
		return nil, nil, errors.SessionNoClient(sessionID)
	}

	return session, session.Client, nil
}

func jsonResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// Launch.json Configuration Handlers

// handleConfigBasedLaunch handles launching a debug session from a launch.json configuration
func (s *Server) handleConfigBasedLaunch(ctx context.Context, request mcp.CallToolRequest, configName string) (*mcp.CallToolResult, error) {
	// Get workspace and config path
	workspace, _ := request.RequireString("workspace")
	configPath, _ := request.RequireString("configPath")

	// Load launch.json
	var lj *launchconfig.LaunchJSON
	var err error

	if configPath != "" {
		lj, err = launchconfig.LoadFromPath(configPath)
	} else if workspace != "" {
		lj, configPath, err = launchconfig.LoadAndDiscover(workspace)
	} else {
		return mcp.NewToolResultError("workspace or configPath is required when using configName"), nil
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load launch.json: %v", err)), nil
	}

	// Find the configuration
	cfg, err := launchconfig.FindConfiguration(lj, configName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("configuration not found: %v", err)), nil
	}

	// Validate it's a launch configuration
	if !cfg.IsLaunchRequest() {
		return mcp.NewToolResultError(fmt.Sprintf("configuration %q is an attach configuration, use debug_attach instead", configName)), nil
	}

	// Build resolution context
	resCtx := &launchconfig.ResolutionContext{
		WorkspaceFolder: workspace,
	}

	// If workspace not provided, derive from configPath
	if resCtx.WorkspaceFolder == "" && configPath != "" {
		resCtx.WorkspaceFolder = launchconfig.GetWorkspaceFolder(configPath)
	}

	// Parse input values if provided
	if inputValuesJSON, err := request.RequireString("inputValues"); err == nil && inputValuesJSON != "" {
		var inputValues map[string]string
		if err := json.Unmarshal([]byte(inputValuesJSON), &inputValues); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid inputValues JSON: %v", err)), nil
		}
		resCtx.InputValues = inputValues
	}

	// Check for program override (can be used as ${file})
	if program, err := request.RequireString("program"); err == nil && program != "" {
		resCtx.CurrentFile = program
	}

	// Resolve the configuration
	resolved, err := launchconfig.ResolveConfiguration(cfg, resCtx)
	if err != nil {
		// Check if it's a missing inputs error
		if missingErr, ok := launchconfig.IsMissingInputsError(err); ok {
			return mcp.NewToolResultError(fmt.Sprintf("missing input values: %v. Provide them via inputValues parameter.", missingErr.Inputs)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve configuration: %v", err)), nil
	}

	// Get the language
	lang := types.Language(resolved.Language)

	// Get the adapter for this language
	adapter, err := s.adapterReg.Get(lang)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Create a new session
	session, err := s.sessionManager.CreateSession(lang, resolved.Program)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Build launch arguments from resolved configuration
	args := resolved.ToLaunchArgs()

	// Add target if browser debugging
	if resolved.Target != "" {
		args["target"] = resolved.Target
	}

	// Spawn the debug adapter if allowed
	if !s.config.CanSpawn() {
		s.sessionManager.TerminateSession(session.ID, false)
		return mcp.NewToolResultError("spawning debug adapters is not allowed"), nil
	}

	// SpawnAndConnect handles both TCP and stdio-based adapters
	client, cmd, err := adapters.SpawnAndConnect(ctx, adapter, resolved.Program, args)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, false)
		return mcp.NewToolResultError(fmt.Sprintf("failed to spawn/connect adapter: %v", err)), nil
	}

	if cmd != nil && cmd.Process != nil {
		s.sessionManager.SetSessionProcess(session.ID, cmd, cmd.Process.Pid)
	}

	s.sessionManager.SetSessionClient(session.ID, client)

	// Initialize the debug adapter
	_, err = client.Initialize("dap-mcp", "DAP-MCP Server")
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(fmt.Sprintf("failed to initialize: %v", err)), nil
	}

	// Launch the program asynchronously
	launchArgs := adapter.BuildLaunchArgs(resolved.Program, args)
	launchRespCh, err := client.LaunchAsync(launchArgs)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(fmt.Sprintf("failed to launch: %v", err)), nil
	}

	// Wait for initialized event
	if err := client.WaitInitialized(10 * time.Second); err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(fmt.Sprintf("failed waiting for initialized: %v", err)), nil
	}

	// Signal configuration done
	if err := client.ConfigurationDone(); err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(fmt.Sprintf("configuration failed: %v", err)), nil
	}

	// Wait for the launch response
	_, err = client.WaitForLaunchResponse(launchRespCh, 10*time.Second)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return mcp.NewToolResultError(fmt.Sprintf("launch failed: %v", err)), nil
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusRunning)

	result := map[string]interface{}{
		"sessionId":  session.ID,
		"status":     "launched",
		"language":   string(lang),
		"program":    resolved.Program,
		"configName": configName,
	}
	if cmd != nil && cmd.Process != nil {
		result["pid"] = cmd.Process.Pid
	}

	return jsonResult(result)
}

// handleDebugListConfigs lists available configurations from a launch.json file
func (s *Server) handleDebugListConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workspace, _ := request.RequireString("workspace")
	configPath, _ := request.RequireString("configPath")

	var lj *launchconfig.LaunchJSON
	var err error
	var foundPath string

	if configPath != "" {
		lj, err = launchconfig.LoadFromPath(configPath)
		foundPath = configPath
	} else if workspace != "" {
		lj, foundPath, err = launchconfig.LoadAndDiscover(workspace)
	} else {
		// Try current directory
		lj, foundPath, err = launchconfig.LoadAndDiscover("")
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load launch.json: %v", err)), nil
	}

	// Validate the configuration
	validationErrors := launchconfig.ValidateLaunchJSON(lj)

	// Build response
	result := map[string]interface{}{
		"configPath":     foundPath,
		"configurations": launchconfig.ListConfigurations(lj),
	}

	if len(lj.Compounds) > 0 {
		result["compounds"] = launchconfig.ListCompounds(lj)
	}

	if len(validationErrors) > 0 {
		errStrings := make([]string, len(validationErrors))
		for i, e := range validationErrors {
			errStrings[i] = e.Error()
		}
		result["validationWarnings"] = errStrings
	}

	return jsonResult(result)
}

// handleDebugLaunchCompound launches a compound configuration (multiple sessions)
func (s *Server) handleDebugLaunchCompound(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	compoundName, err := request.RequireString("compoundName")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	workspace, err := request.RequireString("workspace")
	if err != nil {
		return mcp.NewToolResultError("workspace is required for compound configurations"), nil
	}

	configPath, _ := request.RequireString("configPath")

	// Load launch.json
	var lj *launchconfig.LaunchJSON
	if configPath != "" {
		lj, err = launchconfig.LoadFromPath(configPath)
	} else {
		lj, configPath, err = launchconfig.LoadAndDiscover(workspace)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load launch.json: %v", err)), nil
	}

	// Find the compound configuration
	compound, err := launchconfig.FindCompound(lj, compoundName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("compound not found: %v", err)), nil
	}

	// Parse input values if provided
	var inputValues map[string]string
	if inputValuesJSON, err := request.RequireString("inputValues"); err == nil && inputValuesJSON != "" {
		if err := json.Unmarshal([]byte(inputValuesJSON), &inputValues); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid inputValues JSON: %v", err)), nil
		}
	}

	// Launch each configuration in the compound
	var sessionIDs []string
	var launchResults []map[string]interface{}

	for _, cfgName := range compound.Configurations {
		cfg, err := launchconfig.FindConfiguration(lj, cfgName)
		if err != nil {
			// Clean up any sessions we already created
			for _, sid := range sessionIDs {
				s.sessionManager.TerminateSession(sid, true)
			}
			return mcp.NewToolResultError(fmt.Sprintf("configuration %q not found: %v", cfgName, err)), nil
		}

		// Build resolution context
		resCtx := &launchconfig.ResolutionContext{
			WorkspaceFolder: workspace,
			InputValues:     inputValues,
		}

		// Resolve the configuration
		resolved, err := launchconfig.ResolveConfiguration(cfg, resCtx)
		if err != nil {
			for _, sid := range sessionIDs {
				s.sessionManager.TerminateSession(sid, true)
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to resolve %q: %v", cfgName, err)), nil
		}

		// Launch based on request type
		if cfg.IsLaunchRequest() {
			sessionID, pid, err := s.launchSession(ctx, resolved)
			if err != nil {
				for _, sid := range sessionIDs {
					s.sessionManager.TerminateSession(sid, true)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to launch %q: %v", cfgName, err)), nil
			}
			sessionIDs = append(sessionIDs, sessionID)
			launchResults = append(launchResults, map[string]interface{}{
				"configName": cfgName,
				"sessionId":  sessionID,
				"status":     "launched",
				"pid":        pid,
			})
		} else {
			// TODO: Handle attach configurations in compounds
			launchResults = append(launchResults, map[string]interface{}{
				"configName": cfgName,
				"status":     "skipped",
				"reason":     "attach configurations not yet supported in compounds",
			})
		}
	}

	// Track the compound session if stopAll is enabled
	if compound.StopAll && len(sessionIDs) > 0 {
		s.sessionManager.TrackCompoundSession(compoundName, sessionIDs, compound.StopAll)
	}

	return jsonResult(map[string]interface{}{
		"compoundName": compoundName,
		"sessions":     launchResults,
		"stopAll":      compound.StopAll,
	})
}

// launchSession is a helper that launches a single session from a resolved configuration
func (s *Server) launchSession(ctx context.Context, resolved *launchconfig.ResolvedConfiguration) (string, int, error) {
	lang := types.Language(resolved.Language)

	adapter, err := s.adapterReg.Get(lang)
	if err != nil {
		return "", 0, err
	}

	session, err := s.sessionManager.CreateSession(lang, resolved.Program)
	if err != nil {
		return "", 0, err
	}

	args := resolved.ToLaunchArgs()
	if resolved.Target != "" {
		args["target"] = resolved.Target
	}

	if !s.config.CanSpawn() {
		s.sessionManager.TerminateSession(session.ID, false)
		return "", 0, fmt.Errorf("spawning debug adapters is not allowed")
	}

	address, cmd, err := adapter.Spawn(ctx, resolved.Program, args)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, false)
		return "", 0, fmt.Errorf("failed to spawn adapter: %w", err)
	}

	var pid int
	if cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
		s.sessionManager.SetSessionProcess(session.ID, cmd, pid)
	}

	client, err := adapters.Connect(address, 20)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return "", 0, fmt.Errorf("failed to connect to adapter: %w", err)
	}

	s.sessionManager.SetSessionClient(session.ID, client)

	_, err = client.Initialize("dap-mcp", "DAP-MCP Server")
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return "", 0, fmt.Errorf("failed to initialize: %w", err)
	}

	launchArgs := adapter.BuildLaunchArgs(resolved.Program, args)
	launchRespCh, err := client.LaunchAsync(launchArgs)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return "", 0, fmt.Errorf("failed to launch: %w", err)
	}

	if err := client.WaitInitialized(10 * time.Second); err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return "", 0, fmt.Errorf("failed waiting for initialized: %w", err)
	}

	if err := client.ConfigurationDone(); err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return "", 0, fmt.Errorf("configuration failed: %w", err)
	}

	_, err = client.WaitForLaunchResponse(launchRespCh, 10*time.Second)
	if err != nil {
		s.sessionManager.TerminateSession(session.ID, true)
		return "", 0, fmt.Errorf("launch failed: %w", err)
	}

	s.sessionManager.UpdateSessionStatus(session.ID, types.SessionStatusRunning)

	return session.ID, pid, nil
}
