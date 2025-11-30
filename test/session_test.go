package test

import (
	"testing"
	"time"

	"github.com/ctagard/dap-mcp/internal/dap"
	"github.com/ctagard/dap-mcp/pkg/types"
)

// TestSessionManager_CreateSession verifies session creation.
func TestSessionManager_CreateSession(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	session, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session fields
	if session.ID == "" {
		t.Error("expected session ID to be set")
	}
	if session.Language != types.LanguagePython {
		t.Errorf("expected language %s, got %s", types.LanguagePython, session.Language)
	}
	if session.Program != "/path/to/program.py" {
		t.Errorf("expected program /path/to/program.py, got %s", session.Program)
	}
	if session.Status != types.SessionStatusInitializing {
		t.Errorf("expected status %s, got %s", types.SessionStatusInitializing, session.Status)
	}
	if session.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

// TestSessionManager_MaxSessions verifies max session limit enforcement.
func TestSessionManager_MaxSessions(t *testing.T) {
	sm := dap.NewSessionManager(2, 30*time.Minute) // Max 2 sessions
	defer sm.Close()

	// Create first session
	_, err := sm.CreateSession(types.LanguagePython, "/path/1.py")
	if err != nil {
		t.Fatalf("first session failed: %v", err)
	}

	// Create second session
	_, err = sm.CreateSession(types.LanguageGo, "/path/2.go")
	if err != nil {
		t.Fatalf("second session failed: %v", err)
	}

	// Third session should fail
	_, err = sm.CreateSession(types.LanguageJavaScript, "/path/3.js")
	if err == nil {
		t.Error("expected error when max sessions reached")
	}
}

// TestSessionManager_GetSession verifies session retrieval.
func TestSessionManager_GetSession(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	// Create a session
	created, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get by ID
	retrieved, err := sm.GetSession(created.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, retrieved.ID)
	}
}

// TestSessionManager_GetSession_NotFound verifies error for non-existent session.
func TestSessionManager_GetSession_NotFound(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	_, err := sm.GetSession("nonexistent-id")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

// TestSessionManager_ListSessions verifies listing all sessions.
func TestSessionManager_ListSessions(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	// Initially empty
	sessions := sm.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	// Create sessions
	_, _ = sm.CreateSession(types.LanguagePython, "/path/1.py")
	_, _ = sm.CreateSession(types.LanguageGo, "/path/2.go")
	_, _ = sm.CreateSession(types.LanguageJavaScript, "/path/3.js")

	sessions = sm.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

// TestSessionManager_TerminateSession verifies session termination.
func TestSessionManager_TerminateSession(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	// Create a session
	session, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Terminate it
	err = sm.TerminateSession(session.ID, true)
	if err != nil {
		t.Fatalf("TerminateSession failed: %v", err)
	}

	// Should no longer be retrievable
	_, err = sm.GetSession(session.ID)
	if err == nil {
		t.Error("expected error after termination")
	}

	// List should be empty
	sessions := sm.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after termination, got %d", len(sessions))
	}
}

// TestSessionManager_TerminateSession_NotFound verifies error for non-existent termination.
func TestSessionManager_TerminateSession_NotFound(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	err := sm.TerminateSession("nonexistent-id", true)
	if err == nil {
		t.Error("expected error for non-existent session termination")
	}
}

// TestSessionManager_UpdateSessionStatus verifies status updates.
func TestSessionManager_UpdateSessionStatus(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	session, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update status
	err = sm.UpdateSessionStatus(session.ID, types.SessionStatusRunning)
	if err != nil {
		t.Fatalf("UpdateSessionStatus failed: %v", err)
	}

	// Verify update
	retrieved, _ := sm.GetSession(session.ID)
	if retrieved.Status != types.SessionStatusRunning {
		t.Errorf("expected status %s, got %s", types.SessionStatusRunning, retrieved.Status)
	}
}

// TestSessionManager_UpdateSessionStatus_NotFound verifies error for non-existent status update.
func TestSessionManager_UpdateSessionStatus_NotFound(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	err := sm.UpdateSessionStatus("nonexistent-id", types.SessionStatusRunning)
	if err == nil {
		t.Error("expected error for non-existent session status update")
	}
}

// TestSessionManager_SetSessionProcess verifies process tracking.
func TestSessionManager_SetSessionProcess(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	session, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Set process info (nil cmd is fine for testing)
	err = sm.SetSessionProcess(session.ID, nil, 12345)
	if err != nil {
		t.Fatalf("SetSessionProcess failed: %v", err)
	}

	// Verify update
	retrieved, _ := sm.GetSession(session.ID)
	if retrieved.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", retrieved.PID)
	}
}

// TestSessionManager_SetSessionProcess_NotFound verifies error handling.
func TestSessionManager_SetSessionProcess_NotFound(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	err := sm.SetSessionProcess("nonexistent-id", nil, 12345)
	if err == nil {
		t.Error("expected error for non-existent session process update")
	}
}

// TestSessionManager_CompoundSessions verifies compound session tracking.
func TestSessionManager_CompoundSessions(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	// Create sessions
	s1, _ := sm.CreateSession(types.LanguagePython, "/path/1.py")
	s2, _ := sm.CreateSession(types.LanguageGo, "/path/2.go")

	// Track as compound
	sm.TrackCompoundSession("Full Stack", []string{s1.ID, s2.ID}, true)

	// Verify compound exists
	compound, ok := sm.GetCompoundSession("Full Stack")
	if !ok {
		t.Fatal("compound session not found")
	}
	if compound.Name != "Full Stack" {
		t.Errorf("expected name 'Full Stack', got %s", compound.Name)
	}
	if !compound.StopAll {
		t.Error("expected StopAll to be true")
	}
	if len(compound.SessionIDs) != 2 {
		t.Errorf("expected 2 session IDs, got %d", len(compound.SessionIDs))
	}
}

// TestSessionManager_CompoundSessions_StopAll verifies stopAll behavior.
func TestSessionManager_CompoundSessions_StopAll(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	// Create sessions
	s1, _ := sm.CreateSession(types.LanguagePython, "/path/1.py")
	s2, _ := sm.CreateSession(types.LanguageGo, "/path/2.go")

	// Track as compound with stopAll=true
	sm.TrackCompoundSession("Full Stack", []string{s1.ID, s2.ID}, true)

	// Terminate one session - should terminate both due to stopAll
	err := sm.TerminateSession(s1.ID, true)
	if err != nil {
		t.Fatalf("TerminateSession failed: %v", err)
	}

	// Both sessions should be terminated
	_, err = sm.GetSession(s1.ID)
	if err == nil {
		t.Error("s1 should be terminated")
	}

	_, err = sm.GetSession(s2.ID)
	if err == nil {
		t.Error("s2 should be terminated due to stopAll")
	}
}

// TestSessionManager_ListCompoundSessions verifies listing compounds.
func TestSessionManager_ListCompoundSessions(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	// Create sessions
	s1, _ := sm.CreateSession(types.LanguagePython, "/path/1.py")
	s2, _ := sm.CreateSession(types.LanguageGo, "/path/2.go")

	// Initially empty
	compounds := sm.ListCompoundSessions()
	if len(compounds) != 0 {
		t.Errorf("expected 0 compounds, got %d", len(compounds))
	}

	// Track compound
	sm.TrackCompoundSession("Full Stack", []string{s1.ID, s2.ID}, true)

	compounds = sm.ListCompoundSessions()
	if len(compounds) != 1 {
		t.Errorf("expected 1 compound, got %d", len(compounds))
	}
}

// TestSession_GetInfo verifies session info retrieval.
func TestSession_GetInfo(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)
	defer sm.Close()

	session, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	info := session.GetInfo()

	if info.SessionID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, info.SessionID)
	}
	if info.Language != types.LanguagePython {
		t.Errorf("expected language %s, got %s", types.LanguagePython, info.Language)
	}
	if info.Program != "/path/to/program.py" {
		t.Errorf("expected program /path/to/program.py, got %s", info.Program)
	}
	if info.Status != types.SessionStatusInitializing {
		t.Errorf("expected status %s, got %s", types.SessionStatusInitializing, info.Status)
	}
}

// TestSessionManager_ConcurrentAccess verifies thread safety.
func TestSessionManager_ConcurrentAccess(t *testing.T) {
	sm := dap.NewSessionManager(100, 30*time.Minute)
	defer sm.Close()

	// Create sessions concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			_, err := sm.CreateSession(types.LanguagePython, "/path/to/program.py")
			if err != nil {
				t.Errorf("concurrent CreateSession failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all sessions created
	sessions := sm.ListSessions()
	if len(sessions) != 10 {
		t.Errorf("expected 10 sessions, got %d", len(sessions))
	}
}

// TestSessionManager_Close verifies cleanup on close.
func TestSessionManager_Close(t *testing.T) {
	sm := dap.NewSessionManager(10, 30*time.Minute)

	// Create sessions
	_, _ = sm.CreateSession(types.LanguagePython, "/path/1.py")
	_, _ = sm.CreateSession(types.LanguageGo, "/path/2.go")

	// Close manager
	sm.Close()

	// Sessions should be cleaned up
	sessions := sm.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after close, got %d", len(sessions))
	}
}
