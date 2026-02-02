package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Session struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
	WorkingDir  string    `json:"working_dir"`
	Messages    []Message `json:"messages"`
	Stats       SessionStats `json:"stats"`
	Tags        []string  `json:"tags,omitempty"`
}

type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

type SessionStats struct {
	SessionID          string `json:"session_id"`
	MessageCount       int    `json:"message_count"`
	UserMessages       int    `json:"user_messages"`
	AssistantMessages  int    `json:"assistant_messages"`
	ToolMessages       int    `json:"tool_messages"`
	WorkingDir         string `json:"working_dir"`
}

func NewSession(id string) *Session {
	if id == "" {
		id = generateID()
	}
	
	return &Session{
		ID:          id,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		Messages:    []Message{},
		Stats: SessionStats{
			SessionID: id,
		},
	}
}

func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.LastUpdated = time.Now()
	
	// Update stats
	switch msg.Role {
	case "user":
		s.Stats.UserMessages++
	case "assistant":
		s.Stats.AssistantMessages++
	case "tool":
		s.Stats.ToolMessages++
	}
	s.Stats.MessageCount++
}

func (s *Session) GetMessages() []Message {
	return s.Messages
}

func (s *Session) SetWorkingDir(dir string) {
	s.WorkingDir = dir
	s.Stats.WorkingDir = dir
}

func (s *Session) GetStats() SessionStats {
	return s.Stats
}

func (s *Session) Save(filePath string) error {
	// Update last updated timestamp
	s.LastUpdated = time.Now()
	
	// Marshal session to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// ListSessions returns all saved sessions
func ListSessions(workspace string) ([]map[string]interface{}, error) {
	sessionsDir := filepath.Join(workspace, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return []map[string]interface{}{}, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	var sessions []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".json")
		sessionFile := filepath.Join(sessionsDir, entry.Name())
		
		var session Session
		if data, err := os.ReadFile(sessionFile); err == nil {
			if err := json.Unmarshal(data, &session); err == nil {
				// Create summary for listing
				meta := map[string]interface{}{
					"id":            session.ID,
					"created_at":    session.CreatedAt.Format(time.RFC3339),
					"last_updated":  session.LastUpdated.Format(time.RFC3339),
					"message_count": len(session.Messages),
					"working_dir":   session.WorkingDir,
					"tags":          session.Tags,
				}
				sessions = append(sessions, meta)
			}
		}
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
	sessionFile := filepath.Join(workspace, "sessions", sessionID+".json")
	
	// Read session file
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	
	// Parse session
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	
	return &session, nil
}

// DeleteSession deletes a session by ID
func DeleteSession(workspace, sessionID string) error {
	sessionFile := filepath.Join(workspace, "sessions", sessionID+".json")
	return os.Remove(sessionFile)
}

func generateID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}