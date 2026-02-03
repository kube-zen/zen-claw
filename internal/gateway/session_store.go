package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"

	_ "github.com/mattn/go-sqlite3"
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

// SessionStore manages persistent session storage with SQLite
type SessionStore struct {
	db          *sql.DB
	dbPath      string
	sessions    map[string]*SessionInfo // In-memory cache
	sessionsMu  sync.RWMutex
	maxSessions int
}

// SessionStoreConfig configuration for session store
type SessionStoreConfig struct {
	DBPath      string // Path to SQLite database file
	MaxSessions int    // Maximum concurrent sessions (0 = unlimited)
}

// DefaultSessionDBPath returns the default database path
func DefaultSessionDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".zen", "zen-claw", "data", "sessions.db")
}

// NewSessionStore creates a new session store with SQLite backend
func NewSessionStore(cfg *SessionStoreConfig) (*SessionStore, error) {
	if cfg.DBPath == "" {
		cfg.DBPath = DefaultSessionDBPath()
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 5
	}

	// Create directory if needed
	dir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	// Open SQLite database with WAL mode for better concurrency and crash safety
	db, err := sql.Open("sqlite3", cfg.DBPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Create tables
	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	store := &SessionStore{
		db:          db,
		dbPath:      cfg.DBPath,
		sessions:    make(map[string]*SessionInfo),
		maxSessions: cfg.MaxSessions,
	}

	// Load existing sessions into memory
	if err := store.loadSessions(); err != nil {
		db.Close()
		return nil, fmt.Errorf("load sessions: %w", err)
	}

	log.Printf("[SessionStore] Initialized with SQLite at %s", cfg.DBPath)
	return store, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		working_dir TEXT,
		message_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		seq INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT,
		tool_call_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		UNIQUE(session_id, seq)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, seq);
	`
	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *SessionStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetSession gets a session by ID (from memory cache)
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

// SaveSession saves a session to SQLite
func (s *SessionStore) SaveSession(session *agent.Session) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// Check if new session and at limit
	_, exists := s.sessions[session.ID]
	if !exists {
		activeCount := 0
		for _, info := range s.sessions {
			if info.State == SessionStateActive || info.State == SessionStateBackground {
				activeCount++
			}
		}
		if activeCount >= s.maxSessions {
			return fmt.Errorf("max sessions limit reached (%d)", s.maxSessions)
		}
	}

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	stats := session.GetStats()
	messages := session.GetMessages()

	// Upsert session
	_, err = tx.Exec(`
		INSERT INTO sessions (id, created_at, updated_at, working_dir, message_count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			updated_at = excluded.updated_at,
			working_dir = excluded.working_dir,
			message_count = excluded.message_count
	`, session.ID, stats.CreatedAt, now, stats.WorkingDir, len(messages))
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	// Delete old messages and insert new ones
	_, err = tx.Exec("DELETE FROM messages WHERE session_id = ?", session.ID)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO messages (session_id, seq, role, content, tool_calls, tool_call_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for i, msg := range messages {
		var toolCallsJSON []byte
		if len(msg.ToolCalls) > 0 {
			toolCallsJSON, _ = json.Marshal(msg.ToolCalls)
		}
		_, err = stmt.Exec(session.ID, i, msg.Role, msg.Content, toolCallsJSON, msg.ToolCallID)
		if err != nil {
			return fmt.Errorf("insert message %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Update in-memory cache
	if !exists {
		s.sessions[session.ID] = &SessionInfo{
			Session:  session,
			State:    SessionStateActive,
			LastUsed: now,
		}
	} else {
		s.sessions[session.ID].Session = session
		s.sessions[session.ID].LastUsed = now
	}

	return nil
}

// DeleteSession deletes a session
func (s *SessionStore) DeleteSession(sessionID string) bool {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// Delete from DB (cascade deletes messages)
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		log.Printf("[SessionStore] Delete error: %v", err)
		return false
	}

	// Remove from memory
	delete(s.sessions, sessionID)
	return true
}

// ListSessions returns all session stats
func (s *SessionStore) ListSessions() []agent.SessionStats {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	var stats []agent.SessionStats
	for _, info := range s.sessions {
		stats = append(stats, info.Session.GetStats())
	}
	return stats
}

// SessionListEntry contains session stats with state info
type SessionListEntry struct {
	Stats    agent.SessionStats
	State    SessionState
	ClientID string
	LastUsed time.Time
}

// ListSessionsWithState returns all sessions with state
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

// GetActiveSessions returns active sessions
func (s *SessionStore) GetActiveSessions() []*SessionInfo {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	var active []*SessionInfo
	for _, info := range s.sessions {
		if info.State == SessionStateActive {
			active = append(active, info)
		}
	}
	return active
}

// GetActiveSessionCount returns count of active sessions
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

// CanCreateSession checks if a new session can be created
func (s *SessionStore) CanCreateSession() bool {
	return s.GetActiveSessionCount() < s.maxSessions
}

// GetMaxSessions returns the max sessions limit
func (s *SessionStore) GetMaxSessions() int {
	return s.maxSessions
}

// BackgroundSession moves a session to background
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

	if info.State == SessionStateActive && info.ClientID != "" && info.ClientID != clientID {
		return fmt.Errorf("session %s is active for another client", sessionID)
	}

	info.State = SessionStateActive
	info.ClientID = clientID
	info.LastUsed = time.Now()
	return nil
}

// SetSessionState updates session state
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

// CreateSession creates a new session
func (s *SessionStore) CreateSession(sessionID string) (*agent.Session, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// Return existing
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
		return nil, fmt.Errorf("max sessions limit reached (%d)", s.maxSessions)
	}

	// Create new
	session := agent.NewSession(sessionID)
	s.sessions[sessionID] = &SessionInfo{
		Session:  session,
		State:    SessionStateActive,
		LastUsed: time.Now(),
	}

	return session, nil
}

// loadSessions loads all sessions from SQLite into memory
func (s *SessionStore) loadSessions() error {
	rows, err := s.db.Query(`
		SELECT id, created_at, updated_at, working_dir 
		FROM sessions 
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, workingDir string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &createdAt, &updatedAt, &workingDir); err != nil {
			continue
		}

		// Load messages for this session
		session := agent.NewSession(id)
		if workingDir != "" {
			session.SetWorkingDir(workingDir)
		}

		msgRows, err := s.db.Query(`
			SELECT role, content, tool_calls, tool_call_id
			FROM messages
			WHERE session_id = ?
			ORDER BY seq
		`, id)
		if err != nil {
			continue
		}

		for msgRows.Next() {
			var role, content string
			var toolCallsJSON sql.NullString
			var toolCallID sql.NullString

			if err := msgRows.Scan(&role, &content, &toolCallsJSON, &toolCallID); err != nil {
				continue
			}

			msg := ai.Message{
				Role:    role,
				Content: content,
			}
			if toolCallID.Valid {
				msg.ToolCallID = toolCallID.String
			}
			if toolCallsJSON.Valid && toolCallsJSON.String != "" {
				json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls)
			}
			session.AddMessage(msg)
		}
		msgRows.Close()

		msgCount := len(session.GetMessages())
		s.sessions[id] = &SessionInfo{
			Session:  session,
			State:    SessionStateIdle,
			LastUsed: updatedAt,
		}
		log.Printf("[SessionStore] Loaded session '%s' with %d messages", id, msgCount)
	}

	log.Printf("[SessionStore] Loaded %d sessions from %s", len(s.sessions), s.dbPath)
	return nil
}

// CleanAllSessions deletes all sessions (for CLI clean command)
func (s *SessionStore) CleanAllSessions() (int, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	count := len(s.sessions)

	// Delete from DB
	_, err := s.db.Exec("DELETE FROM messages")
	if err != nil {
		return 0, err
	}
	_, err = s.db.Exec("DELETE FROM sessions")
	if err != nil {
		return 0, err
	}

	// Vacuum to reclaim space
	_, _ = s.db.Exec("VACUUM")

	// Clear memory
	s.sessions = make(map[string]*SessionInfo)

	return count, nil
}

// CleanOldSessions deletes sessions older than the given duration
func (s *SessionStore) CleanOldSessions(olderThan time.Duration) (int, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	count := 0

	// Find old sessions
	var toDelete []string
	for id, info := range s.sessions {
		if info.LastUsed.Before(cutoff) && info.State != SessionStateActive {
			toDelete = append(toDelete, id)
		}
	}

	// Delete from DB
	for _, id := range toDelete {
		_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
		if err == nil {
			delete(s.sessions, id)
			count++
		}
	}

	if count > 0 {
		_, _ = s.db.Exec("VACUUM")
	}

	return count, nil
}

// GetDBPath returns the database file path
func (s *SessionStore) GetDBPath() string {
	return s.dbPath
}

// GetDBSize returns the database file size in bytes
func (s *SessionStore) GetDBSize() int64 {
	info, err := os.Stat(s.dbPath)
	if err != nil {
		return 0
	}
	return info.Size()
}
