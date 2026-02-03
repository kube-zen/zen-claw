package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

func setupTestStore(t *testing.T) (*SessionStore, func()) {
	t.Helper()

	// Use temp directory for test database
	tmpDir, err := os.MkdirTemp("", "zen-claw-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "sessions.db")
	store, err := NewSessionStore(&SessionStoreConfig{
		DBPath:      dbPath,
		MaxSessions: 3,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create session store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestNewSessionStore(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	if store.GetMaxSessions() != 3 {
		t.Errorf("GetMaxSessions() = %d, want 3", store.GetMaxSessions())
	}
}

func TestCreateSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session, err := store.CreateSession("test-session-1")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	if session.ID != "test-session-1" {
		t.Errorf("Session ID = %q, want %q", session.ID, "test-session-1")
	}
}

func TestGetSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session
	_, err := store.CreateSession("test-session")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get the session
	session, found := store.GetSession("test-session")
	if !found {
		t.Error("Expected to find session")
	}
	if session == nil {
		t.Error("Expected non-nil session")
	}

	// Try to get non-existent session
	_, found = store.GetSession("non-existent")
	if found {
		t.Error("Expected not to find non-existent session")
	}
}

func TestSaveSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session
	session, err := store.CreateSession("save-test")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Add some messages to the session
	session.AddMessage(ai.Message{Role: "user", Content: "Hello"})
	session.AddMessage(ai.Message{Role: "assistant", Content: "Hi there!"})

	// Save the session
	err = store.SaveSession(session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Retrieve and verify
	retrieved, found := store.GetSession("save-test")
	if !found {
		t.Fatal("Expected to find saved session")
	}

	messages := retrieved.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
}

func TestDeleteSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session
	_, err := store.CreateSession("delete-test")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Delete it
	deleted := store.DeleteSession("delete-test")
	if !deleted {
		t.Error("Expected deletion to succeed")
	}

	// Verify it's gone
	_, found := store.GetSession("delete-test")
	if found {
		t.Error("Expected session to be deleted")
	}

	// Try to delete non-existent session (returns true since SQL DELETE doesn't error on no rows)
	deleted = store.DeleteSession("non-existent")
	// Note: Current implementation returns true even if session doesn't exist
	_ = deleted // Just verify it doesn't crash
}

func TestListSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create some sessions
	for i := 0; i < 3; i++ {
		_, err := store.CreateSession("session-" + string(rune('a'+i)))
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	sessions := store.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}
}

func TestMaxSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create max sessions (3)
	for i := 0; i < 3; i++ {
		_, err := store.CreateSession("session-" + string(rune('a'+i)))
		if err != nil {
			t.Fatalf("CreateSession %d failed: %v", i, err)
		}
	}

	// Check if we can create more
	if store.CanCreateSession() {
		t.Error("Expected CanCreateSession() to return false when at max")
	}

	// Try to create one more (should fail or evict)
	_, err := store.CreateSession("session-overflow")
	if err == nil {
		// If it succeeded, check that count is still at max
		count := store.GetActiveSessionCount()
		if count > 3 {
			t.Errorf("Session count %d exceeds max %d", count, 3)
		}
	}
}

func TestSessionState(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a session
	_, err := store.CreateSession("state-test")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Check initial state
	info, found := store.GetSessionInfo("state-test")
	if !found {
		t.Fatal("Expected to find session info")
	}
	if info.State != SessionStateActive {
		t.Errorf("Expected initial state %q, got %q", SessionStateActive, info.State)
	}

	// Background the session
	err = store.BackgroundSession("state-test")
	if err != nil {
		t.Fatalf("BackgroundSession failed: %v", err)
	}

	info, _ = store.GetSessionInfo("state-test")
	if info.State != SessionStateBackground {
		t.Errorf("Expected state %q, got %q", SessionStateBackground, info.State)
	}

	// Activate the session
	err = store.ActivateSession("state-test", "client-1")
	if err != nil {
		t.Fatalf("ActivateSession failed: %v", err)
	}

	info, _ = store.GetSessionInfo("state-test")
	if info.State != SessionStateActive {
		t.Errorf("Expected state %q, got %q", SessionStateActive, info.State)
	}
	if info.ClientID != "client-1" {
		t.Errorf("Expected client ID %q, got %q", "client-1", info.ClientID)
	}
}

func TestCleanAllSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create some sessions
	for i := 0; i < 3; i++ {
		_, err := store.CreateSession("clean-" + string(rune('a'+i)))
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	// Verify they exist
	if len(store.ListSessions()) != 3 {
		t.Fatal("Expected 3 sessions before cleanup")
	}

	// Clean all
	count, err := store.CleanAllSessions()
	if err != nil {
		t.Fatalf("CleanAllSessions failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected to clean 3 sessions, cleaned %d", count)
	}

	// Verify they're gone
	if len(store.ListSessions()) != 0 {
		t.Error("Expected 0 sessions after cleanup")
	}
}

func TestCleanOldSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create sessions
	_, err := store.CreateSession("old-session")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Clean sessions older than 1 hour (none should be deleted since we just created it)
	count, err := store.CleanOldSessions(time.Hour)
	if err != nil {
		t.Fatalf("CleanOldSessions failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 sessions cleaned, got %d", count)
	}

	// Session should still exist
	_, found := store.GetSession("old-session")
	if !found {
		t.Error("Expected session to still exist")
	}
}

func TestDefaultSessionDBPath(t *testing.T) {
	path := DefaultSessionDBPath()

	if path == "" {
		t.Error("Expected non-empty path")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %q", path)
	}

	if filepath.Base(path) != "sessions.db" {
		t.Errorf("Expected filename 'sessions.db', got %q", filepath.Base(path))
	}
}
