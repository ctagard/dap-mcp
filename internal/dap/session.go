package dap

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ctagard/dap-mcp/pkg/types"
)

// Session represents an active debug session
type Session struct {
	ID        string
	Language  types.Language
	Status    types.SessionStatus
	Client    *Client
	Process   *exec.Cmd
	PID       int
	Program   string
	CreatedAt time.Time

	mu sync.RWMutex
}

// CompoundSession tracks a group of sessions launched together
type CompoundSession struct {
	Name       string
	SessionIDs []string
	StopAll    bool
}

// SessionManager manages multiple debug sessions
type SessionManager struct {
	sessions          map[string]*Session
	compoundSessions  map[string]*CompoundSession // compound name -> compound session
	sessionToCompound map[string]string           // session ID -> compound name
	mu                sync.RWMutex

	maxSessions    int
	sessionTimeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSessionManager creates a new session manager
func NewSessionManager(maxSessions int, sessionTimeout time.Duration) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())
	sm := &SessionManager{
		sessions:          make(map[string]*Session),
		compoundSessions:  make(map[string]*CompoundSession),
		sessionToCompound: make(map[string]string),
		maxSessions:       maxSessions,
		sessionTimeout:    sessionTimeout,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Start cleanup goroutine
	go sm.cleanupLoop()

	return sm
}

// cleanupLoop periodically cleans up expired sessions
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions removes sessions that have exceeded the timeout
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		if now.Sub(session.CreatedAt) > sm.sessionTimeout {
			sm.terminateSessionLocked(id)
		}
	}
}

// CreateSession creates a new debug session
func (sm *SessionManager) CreateSession(language types.Language, program string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.sessions) >= sm.maxSessions {
		return nil, fmt.Errorf("maximum number of sessions (%d) reached", sm.maxSessions)
	}

	session := &Session{
		ID:        uuid.New().String(),
		Language:  language,
		Status:    types.SessionStatusInitializing,
		Program:   program,
		CreatedAt: time.Now(),
	}

	sm.sessions[session.ID] = session
	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// ListSessions returns all active sessions
func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// TerminateSession terminates a session and cleans up resources
func (sm *SessionManager) TerminateSession(id string, terminateDebuggee bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// Check if this session is part of a compound with stopAll enabled
	if compoundName, ok := sm.sessionToCompound[id]; ok {
		if compound, ok := sm.compoundSessions[compoundName]; ok && compound.StopAll {
			// Terminate all sibling sessions in the compound
			for _, siblingID := range compound.SessionIDs {
				if siblingID != id {
					sm.terminateSessionLocked(siblingID)
					delete(sm.sessionToCompound, siblingID)
				}
			}
			// Clean up compound tracking
			delete(sm.compoundSessions, compoundName)
		}
		delete(sm.sessionToCompound, id)
	}

	// Disconnect from the debug adapter
	if session.Client != nil {
		if err := session.Client.Disconnect(terminateDebuggee); err != nil {
			log.Printf("Warning: failed to disconnect session %s: %v (continuing cleanup)", id, err)
		}
		if err := session.Client.Close(); err != nil {
			log.Printf("Warning: failed to close client for session %s: %v (continuing cleanup)", id, err)
		}
	}

	// Kill the spawned process group if any
	// Uses platform-specific implementation (process_unix.go / process_windows.go)
	if err := killProcessGroup(session.PID, session.Process); err != nil {
		log.Printf("Warning: failed to kill process group for session %s (PID %d): %v", id, session.PID, err)
	}

	session.Status = types.SessionStatusTerminated
	delete(sm.sessions, id)

	return nil
}

// terminateSessionLocked terminates a session (must be called with lock held)
func (sm *SessionManager) terminateSessionLocked(id string) {
	session, ok := sm.sessions[id]
	if !ok {
		return
	}

	if session.Client != nil {
		if err := session.Client.Disconnect(true); err != nil {
			log.Printf("Warning: failed to disconnect session %s during cleanup: %v", id, err)
		}
		if err := session.Client.Close(); err != nil {
			log.Printf("Warning: failed to close client for session %s during cleanup: %v", id, err)
		}
	}

	// Kill the spawned process group
	// Uses platform-specific implementation (process_unix.go / process_windows.go)
	if err := killProcessGroup(session.PID, session.Process); err != nil {
		log.Printf("Warning: failed to kill process group for session %s (PID %d) during cleanup: %v", id, session.PID, err)
	}

	session.Status = types.SessionStatusTerminated
	delete(sm.sessions, id)
}

// TrackCompoundSession registers a group of sessions as a compound session.
// If stopAll is true, terminating any session in the compound will terminate all of them.
func (sm *SessionManager) TrackCompoundSession(compoundName string, sessionIDs []string, stopAll bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	compound := &CompoundSession{
		Name:       compoundName,
		SessionIDs: sessionIDs,
		StopAll:    stopAll,
	}

	sm.compoundSessions[compoundName] = compound

	// Map each session to this compound
	for _, sessionID := range sessionIDs {
		sm.sessionToCompound[sessionID] = compoundName
	}
}

// GetCompoundSession returns information about a compound session
func (sm *SessionManager) GetCompoundSession(compoundName string) (*CompoundSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	compound, ok := sm.compoundSessions[compoundName]
	return compound, ok
}

// ListCompoundSessions returns all active compound sessions
func (sm *SessionManager) ListCompoundSessions() []*CompoundSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	compounds := make([]*CompoundSession, 0, len(sm.compoundSessions))
	for _, compound := range sm.compoundSessions {
		compounds = append(compounds, compound)
	}
	return compounds
}

// SetSessionClient sets the DAP client for a session
func (sm *SessionManager) SetSessionClient(id string, client *Client) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.Client = client
	return nil
}

// SetSessionProcess sets the spawned process for a session
func (sm *SessionManager) SetSessionProcess(id string, cmd *exec.Cmd, pid int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.Process = cmd
	session.PID = pid
	return nil
}

// UpdateSessionStatus updates the status of a session
func (sm *SessionManager) UpdateSessionStatus(id string, status types.SessionStatus) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.mu.Lock()
	session.Status = status
	session.mu.Unlock()

	return nil
}

// Close shuts down the session manager and all sessions
func (sm *SessionManager) Close() {
	sm.cancel()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id := range sm.sessions {
		sm.terminateSessionLocked(id)
	}
}

// GetSessionInfo returns session info for a session
func (s *Session) GetInfo() types.SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return types.SessionInfo{
		SessionID: s.ID,
		Language:  s.Language,
		Status:    s.Status,
		PID:       s.PID,
		Program:   s.Program,
	}
}

// SpawnAdapter spawns a debug adapter process and returns the address to connect to
type AdapterSpawner interface {
	Spawn(ctx context.Context, session *Session, args map[string]interface{}) (address string, cmd *exec.Cmd, err error)
}

// DelveSpawner spawns a Delve debug adapter
type DelveSpawner struct {
	DlvPath    string
	BuildFlags string
}

func (d *DelveSpawner) Spawn(ctx context.Context, session *Session, args map[string]interface{}) (string, *exec.Cmd, error) {
	port := findAvailablePort()
	address := fmt.Sprintf("127.0.0.1:%d", port)

	dlvArgs := []string{
		"dap",
		"--listen", address,
	}

	if d.BuildFlags != "" {
		dlvArgs = append(dlvArgs, "--build-flags", d.BuildFlags)
	}

	//nolint:gosec // G204: This is a debug adapter that intentionally spawns subprocesses
	cmd := exec.CommandContext(ctx, d.DlvPath, dlvArgs...)
	cmd.Env = os.Environ()
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start dlv: %w", err)
	}

	// Wait a bit for the server to start
	time.Sleep(500 * time.Millisecond)

	return address, cmd, nil
}

// DebugpySpawner spawns a debugpy debug adapter
type DebugpySpawner struct {
	PythonPath string
}

func (d *DebugpySpawner) Spawn(ctx context.Context, session *Session, args map[string]interface{}) (string, *exec.Cmd, error) {
	port := findAvailablePort()
	address := fmt.Sprintf("127.0.0.1:%d", port)

	program, _ := args["program"].(string)
	programArgs, _ := args["args"].([]string)

	cmdArgs := []string{
		"-m", "debugpy",
		"--listen", address,
		"--wait-for-client",
	}

	if program != "" {
		cmdArgs = append(cmdArgs, program)
		cmdArgs = append(cmdArgs, programArgs...)
	}

	//nolint:gosec // G204: This is a debug adapter that intentionally spawns subprocesses
	cmd := exec.CommandContext(ctx, d.PythonPath, cmdArgs...)
	cmd.Env = os.Environ()
	setProcAttr(cmd)

	// Add any custom environment variables
	if env, ok := args["env"].(map[string]string); ok {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start debugpy: %w", err)
	}

	// Wait a bit for the server to start
	time.Sleep(500 * time.Millisecond)

	return address, cmd, nil
}

// NodeSpawner spawns a Node.js debug adapter
type NodeSpawner struct {
	NodePath   string
	InspectBrk bool
}

func (n *NodeSpawner) Spawn(ctx context.Context, session *Session, args map[string]interface{}) (string, *exec.Cmd, error) {
	port := findAvailablePort()
	address := fmt.Sprintf("127.0.0.1:%d", port)

	program, _ := args["program"].(string)
	programArgs, _ := args["args"].([]string)

	inspectFlag := "--inspect"
	if n.InspectBrk {
		inspectFlag = "--inspect-brk"
	}

	cmdArgs := []string{
		fmt.Sprintf("%s=%s", inspectFlag, address),
	}

	if program != "" {
		cmdArgs = append(cmdArgs, program)
		cmdArgs = append(cmdArgs, programArgs...)
	}

	//nolint:gosec // G204: This is a debug adapter that intentionally spawns subprocesses
	cmd := exec.CommandContext(ctx, n.NodePath, cmdArgs...)
	cmd.Env = os.Environ()
	setProcAttr(cmd)

	if env, ok := args["env"].(map[string]string); ok {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start node: %w", err)
	}

	// Wait a bit for the server to start
	time.Sleep(500 * time.Millisecond)

	return address, cmd, nil
}

// findAvailablePort finds an available TCP port by binding to port 0
func findAvailablePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Fallback to a default port range if binding fails
		return 38000
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 38000
	}
	return addr.Port
}
