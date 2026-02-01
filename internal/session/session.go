package session

import (
	"fmt"
	"os"
	"path/filepath"
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

	// TODO: Save transcript to file
	return nil
}

func (s *Session) Load() error {
	// TODO: Load transcript from file
	return nil
}

func (s *Session) ID() string {
	return s.id
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}