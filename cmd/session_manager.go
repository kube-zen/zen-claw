package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// Enhanced session management functions

// SaveSession saves a session to disk
func (sm *SessionManager) SaveSession(session *Session) error {
    sessionPath := filepath.Join(sm.basePath, session.ID)
    
    // Ensure directory exists
    err := os.MkdirAll(sessionPath, 0755)
    if err != nil {
        return fmt.Errorf("failed to create session directory: %w", err)
    }
    
    // Serialize session to JSON
    data, err := json.MarshalIndent(session, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal session: %w", err)
    }
    
    // Write to session.json file
    sessionFile := filepath.Join(sessionPath, "session.json")
    err = os.WriteFile(sessionFile, data, 0644)
    if err != nil {
        return fmt.Errorf("failed to write session file: %w", err)
    }
    
    return nil
}

// ListSessions lists all saved sessions
func (sm *SessionManager) ListSessions() ([]map[string]interface{}, error) {
    // Check if sessions directory exists
    if _, err := os.Stat(sm.basePath); os.IsNotExist(err) {
        return []map[string]interface{}{}, nil
    }
    
    // Read sessions directory
    entries, err := os.ReadDir(sm.basePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read sessions directory: %w", err)
    }
    
    var sessions []map[string]interface{}
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        // Read session.json file
        sessionFile := filepath.Join(sm.basePath, entry.Name(), "session.json")
        data, err := os.ReadFile(sessionFile)
        if err != nil {
            continue // Skip sessions that can't be read
        }
        
        var session Session
        if err := json.Unmarshal(data, &session); err == nil {
            // Create summary for listing
            meta := map[string]interface{}{
                "id":           session.ID,
                "started_at":   session.StartedAt.Format(time.RFC3339),
                "working_dir":  session.WorkingDir,
                "provider":     session.Provider,
                "model":        session.Model,
            }
            sessions = append(sessions, meta)
        }
    }
    
    // Sort by started_at (newest first)
    sort.Slice(sessions, func(i, j int) bool {
        timeI, _ := time.Parse(time.RFC3339, sessions[i]["started_at"].(string))
        timeJ, _ := time.Parse(time.RFC3339, sessions[j]["started_at"].(string))
        return timeI.After(timeJ)
    })
    
    return sessions, nil
}

// DeleteSession removes a session from disk
func (sm *SessionManager) DeleteSession(sessionID string) error {
    sessionPath := filepath.Join(sm.basePath, sessionID)
    return os.RemoveAll(sessionPath)
}

// CleanupOldSessions removes sessions older than 7 days
func (sm *SessionManager) CleanupOldSessions() error {
    if _, err := os.Stat(sm.basePath); os.IsNotExist(err) {
        return nil
    }
    
    entries, err := os.ReadDir(sm.basePath)
    if err != nil {
        return fmt.Errorf("failed to read sessions directory: %w", err)
    }
    
    // Clean up sessions older than 7 days
    cutoffTime := time.Now().Add(-7 * 24 * time.Hour)
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        sessionPath := filepath.Join(sm.basePath, entry.Name())
        fileInfo, err := entry.Info()
        if err == nil && fileInfo.ModTime().Before(cutoffTime) {
            if err := os.RemoveAll(sessionPath); err != nil {
                fmt.Printf("Warning: failed to remove old session %s: %v\n", entry.Name(), err)
            }
        }
    }
    
    return nil
}

// GetSessionAge calculates the age of a session
func (sm *SessionManager) GetSessionAge(sessionID string) (time.Duration, error) {
    sessionPath := filepath.Join(sm.basePath, sessionID)
    
    // Check if session exists
    if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
        return 0, fmt.Errorf("session %s does not exist", sessionID)
    }
    
    // Read session file to get started_at
    sessionFile := filepath.Join(sessionPath, "session.json")
    data, err := os.ReadFile(sessionFile)
    if err != nil {
        return 0, fmt.Errorf("failed to read session file: %w", err)
    }
    
    var session Session
    if err := json.Unmarshal(data, &session); err != nil {
        return 0, fmt.Errorf("failed to unmarshal session: %w", err)
    }
    
    return time.Since(session.StartedAt), nil
}

// Enhanced session info display
func getSessionInfo(session *Session) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("Session ID: %s\n", session.ID))
    sb.WriteString(fmt.Sprintf("Started: %s\n", session.StartedAt.Format("2006-01-02 15:04:05")))
    sb.WriteString(fmt.Sprintf("Working Directory: %s\n", session.WorkingDir))
    sb.WriteString(fmt.Sprintf("Provider: %s\n", session.Provider))
    sb.WriteString(fmt.Sprintf("Model: %s\n", session.Model))
    
    return sb.String()
}

// Session info function
func getSessionInfoForDisplay(session *Session, workspace string) string {
    age, err := NewSessionManager().GetSessionAge(session.ID)
    if err != nil {
        return fmt.Sprintf("Session ID: %s\nError getting age: %v", session.ID, err)
    }
    
    ageStr := ""
    if age < 24*time.Hour {
        ageStr = fmt.Sprintf("%d hours ago", int(age.Hours()))
    } else {
        ageStr = fmt.Sprintf("%d days ago", int(age.Hours()/24))
    }
    
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("Session ID: %s (%s)\n", session.ID, ageStr))
    sb.WriteString(fmt.Sprintf("Started: %s\n", session.StartedAt.Format("2006-01-02 15:04:05")))
    sb.WriteString(fmt.Sprintf("Working Directory: %s\n", session.WorkingDir))
    sb.WriteString(fmt.Sprintf("Provider: %s\n", session.Provider))
    sb.WriteString(fmt.Sprintf("Model: %s\n", session.Model))
    
    return sb.String()
}

// Session cleanup function
func cleanupOldSessions() {
    sm := NewSessionManager()
    if err := sm.CleanupOldSessions(); err != nil {
        fmt.Printf("Warning: Failed to cleanup old sessions: %v\n", err)
    }
}
