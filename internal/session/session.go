package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Config struct {
	Workspace string
	Model     string
}

type Session struct {
	config     Config
	id         string
	createdAt  time.Time
	transcript []Message
}

type Message struct {
	Role    string // "user", "assistant", "system", "tool"
	Content string
	Time    time.Time
}

func New(config Config) *Session {
	return &Session{
		config:    config,
		id:        generateID(),
		createdAt: time.Now(),
	}
}

func (s *Session) AddMessage(role, content string) {
	s.transcript = append(s.transcript, Message{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	})
}

func (s *Session) GetTranscript() []Message {
	return s.transcript
}

func (s *Session) Save() error {
	// Create session directory
	sessionDir := filepath.Join(s.config.Workspace, ".zen-claw", "sessions", s.id)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	// Save transcript
	transcriptFile := filepath.Join(sessionDir, "transcript.json")
	data, err := json.MarshalIndent(s.transcript, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal transcript: %w", err)
	}

	// Save metadata
	metaFile := filepath.Join(sessionDir, "meta.json")
	meta := map[string]interface{}{
		"id":            s.id,
		"created_at":    s.createdAt.Format(time.RFC3339),
		"model":         s.config.Model,
		"message_count": len(s.transcript),
		"last_updated":  time.Now().Format(time.RFC3339),
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	
	if err := os.WriteFile(metaFile, metaData, 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return os.WriteFile(transcriptFile, data, 0644)
}

// ListSessions returns all saved sessions
func ListSessions(workspace string) ([]map[string]interface{}, error) {
	sessionsDir := filepath.Join(workspace, ".zen-claw", "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return []map[string]interface{}{}, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	var sessions []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		metaFile := filepath.Join(sessionsDir, sessionID, "meta.json")
		
		var meta map[string]interface{}
		if data, err := os.ReadFile(metaFile); err == nil {
			json.Unmarshal(data, &meta)
		} else {
			// Create basic metadata if file doesn't exist
			info, _ := entry.Info()
			meta = map[string]interface{}{
				"id":            sessionID,
				"created_at":    info.ModTime().Format(time.RFC3339),
				"message_count": 0,
				"last_updated":  info.ModTime().Format(time.RFC3339),
			}
		}
		
		sessions = append(sessions, meta)
	}

	// Sort by last_updated (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		timeI, _ := time.Parse(time.RFC3339, sessions[i]["last_updated"].(string))
		timeJ, _ := time.Parse(time.RFC3339, sessions[j]["last_updated"].(string))
		return timeI.After(timeJ)
	})

	return sessions, nil
}

// LoadSession loads a session by ID
func LoadSession(workspace, sessionID string) (*Session, error) {
	sessionDir := filepath.Join(workspace, ".zen-claw", "sessions", sessionID)
	
	// Load metadata
	metaFile := filepath.Join(sessionDir, "meta.json")
	metaData, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	
	var meta map[string]interface{}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	
	// Load transcript
	transcriptFile := filepath.Join(sessionDir, "transcript.json")
	transcriptData, err := os.ReadFile(transcriptFile)
	if err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}
	
	var transcript []Message
	if err := json.Unmarshal(transcriptData, &transcript); err != nil {
		return nil, fmt.Errorf("unmarshal transcript: %w", err)
	}
	
	// Parse creation time
	createdAt, _ := time.Parse(time.RFC3339, meta["created_at"].(string))
	model, _ := meta["model"].(string)
	
	session := &Session{
		config: Config{
			Workspace: workspace,
			Model:     model,
		},
		id:         sessionID,
		createdAt:  createdAt,
		transcript: transcript,
	}
	
	return session, nil
}

// DeleteSession deletes a session by ID
func DeleteSession(workspace, sessionID string) error {
	sessionDir := filepath.Join(workspace, ".zen-claw", "sessions", sessionID)
	return os.RemoveAll(sessionDir)
}

func (s *Session) ID() string {
	return s.id
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}