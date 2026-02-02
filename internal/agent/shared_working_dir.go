package agent

import (
	"sync"
)

// SharedWorkingDir provides thread-safe shared working directory for tools
type SharedWorkingDir struct {
	dir string
	mu  sync.RWMutex
}

// NewSharedWorkingDir creates a new shared working directory
func NewSharedWorkingDir() *SharedWorkingDir {
	return &SharedWorkingDir{}
}

// Set sets the working directory
func (s *SharedWorkingDir) Set(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dir = dir
}

// Get gets the working directory
func (s *SharedWorkingDir) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dir
}