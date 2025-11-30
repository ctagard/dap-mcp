package test

import (
	"encoding/json"
	"testing"

	"github.com/ctagard/dap-mcp/pkg/types"
)

// TestLanguageConstants verifies language constant values.
func TestLanguageConstants(t *testing.T) {
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
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.lang) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, string(tc.lang))
			}
		})
	}
}

// TestSessionStatusConstants verifies session status constant values.
func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		status   types.SessionStatus
		expected string
	}{
		{types.SessionStatusInitializing, "initializing"},
		{types.SessionStatusRunning, "running"},
		{types.SessionStatusStopped, "stopped"},
		{types.SessionStatusTerminated, "terminated"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.status) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, string(tc.status))
			}
		})
	}
}

// TestLaunchRequest_JSONSerialization verifies LaunchRequest JSON handling.
func TestLaunchRequest_JSONSerialization(t *testing.T) {
	req := types.LaunchRequest{
		Language:    types.LanguagePython,
		Program:     "/path/to/script.py",
		Args:        []string{"--verbose", "--config", "test.yaml"},
		Cwd:         "/project",
		Env:         map[string]string{"DEBUG": "1"},
		StopOnEntry: true,
	}

	// Serialize
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize
	var decoded types.LaunchRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify
	if decoded.Language != req.Language {
		t.Errorf("language mismatch: %v != %v", decoded.Language, req.Language)
	}
	if decoded.Program != req.Program {
		t.Errorf("program mismatch: %v != %v", decoded.Program, req.Program)
	}
	if len(decoded.Args) != len(req.Args) {
		t.Errorf("args length mismatch: %d != %d", len(decoded.Args), len(req.Args))
	}
	if decoded.Cwd != req.Cwd {
		t.Errorf("cwd mismatch: %v != %v", decoded.Cwd, req.Cwd)
	}
	if decoded.Env["DEBUG"] != "1" {
		t.Errorf("env mismatch: %v != %v", decoded.Env, req.Env)
	}
	if decoded.StopOnEntry != true {
		t.Error("stopOnEntry should be true")
	}
}

// TestLaunchRequest_OmitEmpty verifies optional fields are omitted.
func TestLaunchRequest_OmitEmpty(t *testing.T) {
	req := types.LaunchRequest{
		Language: types.LanguagePython,
		Program:  "/path/to/script.py",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify optional fields are omitted
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := m["args"]; ok {
		t.Error("args should be omitted when empty")
	}
	if _, ok := m["cwd"]; ok {
		t.Error("cwd should be omitted when empty")
	}
	if _, ok := m["env"]; ok {
		t.Error("env should be omitted when empty")
	}
}

// TestAttachRequest_JSONSerialization verifies AttachRequest JSON handling.
func TestAttachRequest_JSONSerialization(t *testing.T) {
	req := types.AttachRequest{
		Language: types.LanguageJavaScript,
		Host:     "localhost",
		Port:     9229,
		PID:      12345,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.AttachRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Language != req.Language {
		t.Errorf("language mismatch")
	}
	if decoded.Host != req.Host {
		t.Errorf("host mismatch: %v != %v", decoded.Host, req.Host)
	}
	if decoded.Port != req.Port {
		t.Errorf("port mismatch: %v != %v", decoded.Port, req.Port)
	}
	if decoded.PID != req.PID {
		t.Errorf("pid mismatch: %v != %v", decoded.PID, req.PID)
	}
}

// TestSessionInfo_JSONSerialization verifies SessionInfo JSON handling.
func TestSessionInfo_JSONSerialization(t *testing.T) {
	info := types.SessionInfo{
		SessionID: "abc-123",
		Language:  types.LanguagePython,
		Status:    types.SessionStatusRunning,
		PID:       54321,
		Program:   "/path/to/script.py",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.SessionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.SessionID != info.SessionID {
		t.Errorf("sessionId mismatch")
	}
	if decoded.Status != info.Status {
		t.Errorf("status mismatch")
	}
}

// TestThreadInfo_JSONSerialization verifies ThreadInfo JSON handling.
func TestThreadInfo_JSONSerialization(t *testing.T) {
	info := types.ThreadInfo{
		ID:     1,
		Name:   "MainThread",
		Status: "stopped",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.ThreadInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != info.ID {
		t.Errorf("ID mismatch: %v != %v", decoded.ID, info.ID)
	}
	if decoded.Name != info.Name {
		t.Errorf("Name mismatch")
	}
}

// TestStackFrame_JSONSerialization verifies StackFrame JSON handling.
func TestStackFrame_JSONSerialization(t *testing.T) {
	frame := types.StackFrame{
		ID:        1,
		Name:      "main",
		Line:      42,
		Column:    10,
		EndLine:   45,
		EndColumn: 20,
		Source: &types.SourceInfo{
			Name: "main.py",
			Path: "/path/to/main.py",
		},
	}

	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.StackFrame
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != frame.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.Name != frame.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.Line != frame.Line {
		t.Errorf("Line mismatch: %v != %v", decoded.Line, frame.Line)
	}
	if decoded.Source == nil {
		t.Fatal("Source should not be nil")
	}
	if decoded.Source.Path != frame.Source.Path {
		t.Errorf("Source.Path mismatch")
	}
}

// TestSourceInfo_JSONSerialization verifies SourceInfo JSON handling.
func TestSourceInfo_JSONSerialization(t *testing.T) {
	info := types.SourceInfo{
		Name:            "script.py",
		Path:            "/full/path/to/script.py",
		SourceReference: 5,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.SourceInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != info.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.Path != info.Path {
		t.Errorf("Path mismatch")
	}
	if decoded.SourceReference != info.SourceReference {
		t.Errorf("SourceReference mismatch")
	}
}

// TestScope_JSONSerialization verifies Scope JSON handling.
func TestScope_JSONSerialization(t *testing.T) {
	scope := types.Scope{
		Name:               "Locals",
		VariablesReference: 10,
		NamedVariables:     5,
		IndexedVariables:   3,
		Expensive:          false,
	}

	data, err := json.Marshal(scope)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.Scope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != scope.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.VariablesReference != scope.VariablesReference {
		t.Errorf("VariablesReference mismatch")
	}
}

// TestVariable_JSONSerialization verifies Variable JSON handling.
func TestVariable_JSONSerialization(t *testing.T) {
	v := types.Variable{
		Name:               "items",
		Value:              "[1, 2, 3]",
		Type:               "list",
		VariablesReference: 15,
		NamedVariables:     0,
		IndexedVariables:   3,
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.Variable
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Name != v.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.Value != v.Value {
		t.Errorf("Value mismatch")
	}
	if decoded.Type != v.Type {
		t.Errorf("Type mismatch")
	}
}

// TestBreakpoint_JSONSerialization verifies Breakpoint JSON handling.
func TestBreakpoint_JSONSerialization(t *testing.T) {
	bp := types.Breakpoint{
		ID:           1,
		Verified:     true,
		Line:         42,
		Column:       0,
		Condition:    "x > 10",
		HitCondition: "5",
		LogMessage:   "Value: {x}",
		Source: &types.SourceInfo{
			Path: "/path/to/script.py",
		},
	}

	data, err := json.Marshal(bp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.Breakpoint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != bp.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.Verified != bp.Verified {
		t.Errorf("Verified mismatch")
	}
	if decoded.Condition != bp.Condition {
		t.Errorf("Condition mismatch")
	}
}

// TestBreakpointRequest_JSONSerialization verifies BreakpointRequest JSON handling.
func TestBreakpointRequest_JSONSerialization(t *testing.T) {
	req := types.BreakpointRequest{
		Line:         100,
		Condition:    "i == 5",
		HitCondition: "10",
		LogMessage:   "Iteration: {i}",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.BreakpointRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Line != req.Line {
		t.Errorf("Line mismatch")
	}
	if decoded.Condition != req.Condition {
		t.Errorf("Condition mismatch")
	}
}

// TestEvaluateResult_JSONSerialization verifies EvaluateResult JSON handling.
func TestEvaluateResult_JSONSerialization(t *testing.T) {
	result := types.EvaluateResult{
		Result:             "42",
		Type:               "int",
		VariablesReference: 0,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.EvaluateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Result != result.Result {
		t.Errorf("Result mismatch")
	}
	if decoded.Type != result.Type {
		t.Errorf("Type mismatch")
	}
}

// TestDebugSnapshot_JSONSerialization verifies DebugSnapshot JSON handling.
func TestDebugSnapshot_JSONSerialization(t *testing.T) {
	snapshot := types.DebugSnapshot{
		SessionID: "session-123",
		Status:    types.SessionStatusStopped,
		Threads: []types.ThreadInfo{
			{ID: 1, Name: "MainThread"},
		},
		Stacks: map[int][]types.StackFrame{
			1: {{ID: 1, Name: "main", Line: 10}},
		},
		Scopes: map[int][]types.Scope{
			1: {{Name: "Locals", VariablesReference: 5}},
		},
		Variables: map[int][]types.Variable{
			5: {{Name: "x", Value: "42", Type: "int"}},
		},
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.DebugSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.SessionID != snapshot.SessionID {
		t.Errorf("SessionID mismatch")
	}
	if decoded.Status != snapshot.Status {
		t.Errorf("Status mismatch")
	}
	if len(decoded.Threads) != 1 {
		t.Errorf("Threads length mismatch")
	}
	if len(decoded.Stacks) != 1 {
		t.Errorf("Stacks length mismatch")
	}
	if len(decoded.Scopes) != 1 {
		t.Errorf("Scopes length mismatch")
	}
	if len(decoded.Variables) != 1 {
		t.Errorf("Variables length mismatch")
	}
}

// TestModuleInfo_JSONSerialization verifies ModuleInfo JSON handling.
func TestModuleInfo_JSONSerialization(t *testing.T) {
	info := types.ModuleInfo{
		ID:             1,
		Name:           "main",
		Path:           "/path/to/main.so",
		Version:        "1.0.0",
		SymbolStatus:   "loaded",
		SymbolFilePath: "/path/to/symbols",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded types.ModuleInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != info.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.Name != info.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.Path != info.Path {
		t.Errorf("Path mismatch")
	}
}
