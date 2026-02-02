package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
)

// SessionStore manages persistent session storage
type SessionStore struct {
	config     *SessionStoreConfig
	sessions   map[string]*agent.Session
	sessionsMu sync.RWMutex
}

// SessionStoreConfig configuration for session store
type SessionStoreConfig struct {
	DataDir string
}

// NewSessionStore creates a new session store
func NewSessionStore(cfg *SessionStoreConfig) (*SessionStore, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "/tmp/zen-claw-sessions"
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create session data directory: %w", err)
	}

	store := &SessionStore{
		config:   cfg,
		sessions: make(map[string]*agent.Session),
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

	session, exists := s.sessions[sessionID]
	return session, exists
}

// SaveSession saves a session to disk
func (s *SessionStore) SaveSession(session *agent.Session) error {
	s.sessionsMu.Lock()
	s.sessions[session.ID] = session
	s.sessionsMu.Unlock()

	return s.persistSession(session)
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
	for _, session := range s.sessions {
		stats = append(stats, session.GetStats())
	}
	return stats
}

// persistSession saves a single session to disk
func (s *SessionStore) persistSession(session *agent.Session) error {
	filePath := filepath.Join(s.config.DataDir, session.ID+".json")
	
	// Create session data for persistence
	sessionData := PersistedSession{
		ID:        session.ID,
		CreatedAt: session.GetStats().CreatedAt,
		UpdatedAt: time.Now(),
		Messages:  session.GetMessages(),
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

		s.sessions[persisted.ID] = session
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