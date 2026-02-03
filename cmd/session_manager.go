package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents a Zen Claw session
type Session struct {
	ID          string
	StartedAt   time.Time
	WorkingDir  string
	Provider    string
	Model       string
	Tasks       []Task
	Completed   []Task
	Notes       string
}

// Task represents a single task in a session
type Task struct {
	ID          string
	Title       string
	Description string
	Status      string // "pending", "in-progress", "completed", "failed"
	StartTime   time.Time
	EndTime     time.Time
	Result      string
}

// SessionManager handles session creation and management
type SessionManager struct {
	basePath string
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		basePath: filepath.Join(os.Getenv("HOME"), ".zen", "zen-claw", "sessions"),
	}
}

// CreateSession creates a new session directory
func (sm *SessionManager) CreateSession(sessionID, workingDir, provider, model string) (*Session, error) {
	// Create session directory
	sessionPath := filepath.Join(sm.basePath, sessionID)
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create session object
	session := &Session{
		ID:         sessionID,
		StartedAt:  time.Now(),
		WorkingDir: workingDir,
		Provider:   provider,
		Model:      model,
	}

	// Save session metadata
	err = sm.saveSession(session)
	if err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// LoadSession loads an existing session
func (sm *SessionManager) LoadSession(sessionID string) (*Session, error) {
	sessionPath := filepath.Join(sm.basePath, sessionID)
	
	// Check if session exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("session %s does not exist", sessionID)
	}
	
	// In a real implementation, we'd load from JSON or other format
	// For now, return basic session info
	return &Session{
		ID: sessionID,
	}, nil
}

// SaveSession saves session data to disk
func (sm *SessionManager) saveSession(session *Session) error {
	sessionPath := filepath.Join(sm.basePath, session.ID)
	
	// In a real implementation, we'd serialize to JSON
	// For now, just ensure directory exists
	err := os.MkdirAll(sessionPath, 0755)
	return err
}

// CreateTaskList creates a task list for a session
func (sm *SessionManager) CreateTaskList(session *Session, taskTitle string, tasks []Task) error {
	taskListPath := filepath.Join(sm.basePath, session.ID, "todo.md")
	
	content := fmt.Sprintf("# Session Tasks: %s\n\n", taskTitle)
	content += fmt.Sprintf("Session ID: %s\n", session.ID)
	content += fmt.Sprintf("Created: %s\n\n", session.StartedAt.Format(time.RFC3339))
	content += "## Task List\n\n"
	
	for i, task := range tasks {
		content += fmt.Sprintf("%d. [%s] %s\n", i+1, task.Status, task.Title)
		if task.Description != "" {
			content += fmt.Sprintf("   - %s\n", task.Description)
		}
		content += "\n"
	}
	
	return os.WriteFile(taskListPath, []byte(content), 0644)
}

// UpdateTask updates a task status and result
func (sm *SessionManager) UpdateTask(session *Session, taskID string, status, result string) error {
	// In a real implementation, this would update the task in the session data
	// For now, just log the update
	fmt.Printf("Task %s updated to %s: %s\n", taskID, status, result)
	return nil
}

// GetCurrentTask gets the next pending task
func (sm *SessionManager) GetCurrentTask(session *Session) *Task {
	// Find first pending task
	for _, task := range session.Tasks {
		if task.Status == "pending" {
			return &task
		}
	}
	return nil
}
