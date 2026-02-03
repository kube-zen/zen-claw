package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
)

// SessionState tracks whether a session is active or backgrounded
type SessionState string

const (
	SessionStateActive     SessionState = "active"     // Currently being used by a client
	SessionStateBackground SessionState = "background" // Running in background, not actively used
	SessionStateIdle       SessionState = "idle"       // Loaded but not running
)

// SessionInfo tracks session metadata for multi-session management
type SessionInfo struct {
	Session  *agent.Session
	State    SessionState
	ClientID string    // Which client is using this session (empty if background/idle)
	LastUsed time.Time // Last time this session was interacted with
}

// SessionStore manages persistent session storage
type SessionStore struct {
	config      *SessionStoreConfig
	sessions    map[string]*SessionInfo
	sessionsMu  sync.RWMutex
	maxSessions int
}

// SessionStoreConfig configuration for session store
type SessionStoreConfig struct {
	DataDir     string
	MaxSessions int // Maximum concurrent sessions (0 = unlimited)
}

// NewSessionStore creates a new session store
func NewSessionStore(cfg *SessionStoreConfig) (*SessionStore, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "/tmp/zen-claw-sessions"
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 5 // Default max sessions
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create session data directory: %w", err)
	}

	store := &SessionStore{
		config:      cfg,
		sessions:    make(map[string]*SessionInfo),
		maxSessions: cfg.MaxSessions,
	}

	// Migrate old sessions from /tmp if needed
	if cfg.DataDir != "/tmp/zen-claw-sessions" {
		if err := store.migrateOldSessions(); err != nil {
			log.Printf("Warning: Failed to migrate old sessions: %v", err)
		}
	}

	// Load existing sessions
	if err := store.loadSessions(); err != nil {
		return nil, fmt.Errorf("load sessions: %w", err)
	}

	return store, nil
}

// GetSession gets a session by ID
func (s *SessionStore) GetSession(sessionID string) (*agent.Session, bool) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return nil, false
	}
	return info.Session, true
}

// GetSessionInfo gets full session info by ID
func (s *SessionStore) GetSessionInfo(sessionID string) (*SessionInfo, bool) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	info, exists := s.sessions[sessionID]
	return info, exists
}

// SetSessionState updates the state of a session
func (s *SessionStore) SetSessionState(sessionID string, state SessionState, clientID string) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	info.State = state
	info.ClientID = clientID
	info.LastUsed = time.Now()
	return nil
}

// GetActiveSessions returns all sessions that are active or background (not idle)
func (s *SessionStore) GetActiveSessions() []*SessionInfo {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	var active []*SessionInfo
	for _, info := range s.sessions {
		if info.State == SessionStateActive || info.State == SessionStateBackground {
			active = append(active, info)
		}
	}
	return active
}

// GetActiveSessionCount returns the count of active/background sessions
func (s *SessionStore) GetActiveSessionCount() int {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := 0
	for _, info := range s.sessions {
		if info.State == SessionStateActive || info.State == SessionStateBackground {
			count++
		}
	}
	return count
}

// CanCreateSession checks if a new session can be created (within max limit)
func (s *SessionStore) CanCreateSession() bool {
	return s.GetActiveSessionCount() < s.maxSessions
}

// GetMaxSessions returns the max sessions limit
func (s *SessionStore) GetMaxSessions() int {
	return s.maxSessions
}

// BackgroundSession moves a session to background state
func (s *SessionStore) BackgroundSession(sessionID string) error {
	return s.SetSessionState(sessionID, SessionStateBackground, "")
}

// ActivateSession activates a session for a client
func (s *SessionStore) ActivateSession(sessionID, clientID string) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Check if session is already active for another client
	if info.State == SessionStateActive && info.ClientID != "" && info.ClientID != clientID {
		return fmt.Errorf("session %s is active for another client", sessionID)
	}

	info.State = SessionStateActive
	info.ClientID = clientID
	info.LastUsed = time.Now()
	return nil
}

// SaveSession saves a session to disk
func (s *SessionStore) SaveSession(session *agent.Session) error {
	s.sessionsMu.Lock()
	info, exists := s.sessions[session.ID]
	if !exists {
		// New session - check if we can create it (count inline to avoid deadlock)
		activeCount := 0
		for _, info := range s.sessions {
			if info.State == SessionStateActive || info.State == SessionStateBackground {
				activeCount++
			}
		}
		if activeCount >= s.maxSessions {
			s.sessionsMu.Unlock()
			return fmt.Errorf("max sessions limit reached (%d), background or close a session first", s.maxSessions)
		}
		info = &SessionInfo{
			Session:  session,
			State:    SessionStateActive,
			LastUsed: time.Now(),
		}
		s.sessions[session.ID] = info
	} else {
		info.Session = session
		info.LastUsed = time.Now()
	}
	s.sessionsMu.Unlock()

	return s.persistSession(session)
}

// CreateSession creates a new session if within limits
func (s *SessionStore) CreateSession(sessionID string) (*agent.Session, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// Check existing
	if info, exists := s.sessions[sessionID]; exists {
		info.LastUsed = time.Now()
		return info.Session, nil
	}

	// Check limit
	activeCount := 0
	for _, info := range s.sessions {
		if info.State == SessionStateActive || info.State == SessionStateBackground {
			activeCount++
		}
	}
	if activeCount >= s.maxSessions {
		return nil, fmt.Errorf("max sessions limit reached (%d/%d), use /sessions to manage", activeCount, s.maxSessions)
	}

	// Create new session
	session := agent.NewSession(sessionID)
	s.sessions[sessionID] = &SessionInfo{
		Session:  session,
		State:    SessionStateActive,
		LastUsed: time.Now(),
	}

	return session, nil
}

// DeleteSession deletes a session
func (s *SessionStore) DeleteSession(sessionID string) bool {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		delete(s.sessions, sessionID)

		// Delete from disk
		filePath := filepath.Join(s.config.DataDir, sessionID+".json")
		os.Remove(filePath)

		return true
	}
	return false
}

// ListSessions returns all sessions
func (s *SessionStore) ListSessions() []agent.SessionStats {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	var stats []agent.SessionStats
	for _, info := range s.sessions {
		stats = append(stats, info.Session.GetStats())
	}
	return stats
}

// ListSessionsWithState returns all sessions with their state info
func (s *SessionStore) ListSessionsWithState() []SessionListEntry {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	var entries []SessionListEntry
	for _, info := range s.sessions {
		entries = append(entries, SessionListEntry{
			Stats:    info.Session.GetStats(),
			State:    info.State,
			ClientID: info.ClientID,
			LastUsed: info.LastUsed,
		})
	}
	return entries
}

// SessionListEntry contains session stats with state info
type SessionListEntry struct {
	Stats    agent.SessionStats
	State    SessionState
	ClientID string
	LastUsed time.Time
}

// persistSession saves a single session to disk
func (s *SessionStore) persistSession(session *agent.Session) error {
	filePath := filepath.Join(s.config.DataDir, session.ID+".json")

	// Create session data for persistence
	sessionData := PersistedSession{
		ID:         session.ID,
		CreatedAt:  session.GetStats().CreatedAt,
		UpdatedAt:  time.Now(),
		Messages:   session.GetMessages(),
		WorkingDir: session.GetStats().WorkingDir,
	}

	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	// Write to temp file first, then rename (atomic write)
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("rename session file: %w", err)
	}

	return nil
}

// migrateOldSessions migrates sessions from old /tmp location to new location
func (s *SessionStore) migrateOldSessions() error {
	oldDir := "/tmp/zen-claw-sessions"
	
	// Check if old directory exists
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}
	
	// Read old sessions
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return fmt.Errorf("read old sessions directory: %w", err)
	}
	
	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		
		oldPath := filepath.Join(oldDir, entry.Name())
		newPath := filepath.Join(s.config.DataDir, entry.Name())
		
		// Check if already exists in new location
		if _, err := os.Stat(newPath); err == nil {
			continue // Already migrated
		}
		
		// Copy file
		data, err := os.ReadFile(oldPath)
		if err != nil {
			log.Printf("Warning: Failed to read old session %s: %v", entry.Name(), err)
			continue
		}
		
		// Write to new location (atomic write)
		tmpPath := newPath + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0644); err != nil {
			log.Printf("Warning: Failed to write new session %s: %v", entry.Name(), err)
			continue
		}
		
		if err := os.Rename(tmpPath, newPath); err != nil {
			log.Printf("Warning: Failed to rename session %s: %v", entry.Name(), err)
			continue
		}
		
		migrated++
		log.Printf("Migrated session: %s", entry.Name())
	}
	
	if migrated > 0 {
		log.Printf("Successfully migrated %d sessions from %s to %s", migrated, oldDir, s.config.DataDir)
	}
	
	return nil
}

// loadSessions loads all sessions from disk
func (s *SessionStore) loadSessions() error {
	entries, err := os.ReadDir(s.config.DataDir)
	if err != nil {
		// Directory might not exist yet
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.config.DataDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip corrupted files
		}

		var persisted PersistedSession
		if err := json.Unmarshal(data, &persisted); err != nil {
			continue // Skip invalid JSON
		}

		// Create session from persisted data
		session := agent.NewSession(persisted.ID)
		for _, msg := range persisted.Messages {
			session.AddMessage(msg)
		}
		if persisted.WorkingDir != "" {
			session.SetWorkingDir(persisted.WorkingDir)
		}

		// Loaded sessions start as idle (not actively running)
		s.sessions[persisted.ID] = &SessionInfo{
			Session:  session,
			State:    SessionStateIdle,
			LastUsed: persisted.UpdatedAt,
		}
	}

	return nil
}
// PersistedSession represents a session saved to disk
type PersistedSession struct {
	ID         string       `json:"id"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	Messages   []ai.Message `json:"messages"`
	WorkingDir string       `json:"working_dir,omitempty"`
}