// Package cache implements response caching for AI calls.
// Adapted from zen-brain's Redis cache, simplified to in-memory for zen-claw.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Entry represents a cached response
type Entry struct {
	Response  string
	CreatedAt time.Time
	Hits      int
}

// Cache stores AI responses to avoid duplicate calls
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	ttl     time.Duration
	maxSize int
	enabled bool

	// Stats
	hits   int
	misses int
}

// New creates a new response cache
func New(ttl time.Duration, maxSize int, enabled bool) *Cache {
	if !enabled {
		log.Println("[Cache] DISABLED")
		return &Cache{enabled: false}
	}

	c := &Cache{
		entries: make(map[string]*Entry),
		ttl:     ttl,
		maxSize: maxSize,
		enabled: true,
	}

	// Start cleanup goroutine
	go c.cleanupLoop()

	log.Printf("[Cache] ENABLED: TTL=%v, maxSize=%d", ttl, maxSize)
	return c
}

// Get retrieves a cached response
func (c *Cache) Get(key string) (string, bool) {
	if !c.enabled {
		return "", false
	}

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return "", false
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > c.ttl {
		c.mu.Lock()
		delete(c.entries, key)
		c.misses++
		c.mu.Unlock()
		return "", false
	}

	c.mu.Lock()
	entry.Hits++
	c.hits++
	c.mu.Unlock()

	log.Printf("[Cache] HIT: %s... (hits=%d)", key[:min(16, len(key))], entry.Hits)
	return entry.Response, true
}

// Set stores a response in the cache
func (c *Cache) Set(key string, response string) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &Entry{
		Response:  response,
		CreatedAt: time.Now(),
		Hits:      0,
	}

	log.Printf("[Cache] SET: %s... (size=%d)", key[:min(16, len(key))], len(c.entries))
}

// ComputeKey generates a cache key from prompt and context
func ComputeKey(provider, model, prompt string, context map[string]interface{}) string {
	data := map[string]interface{}{
		"provider": provider,
		"model":    model,
		"prompt":   prompt,
		"context":  context,
	}

	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// ComputeToolKey generates a cache key for tool results
func ComputeToolKey(tool string, args map[string]interface{}) string {
	data := map[string]interface{}{
		"tool": tool,
		"args": args,
	}

	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// Invalidate removes an entry from the cache
func (c *Cache) Invalidate(key string) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// Clear removes all entries
func (c *Cache) Clear() {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	c.entries = make(map[string]*Entry)
	c.mu.Unlock()
	log.Println("[Cache] Cleared")
}

// Stats returns cache statistics
func (c *Cache) Stats() (hits, misses, size int, hitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits = c.hits
	misses = c.misses
	size = len(c.entries)
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return
}

func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		log.Printf("[Cache] Evicted: %s...", oldestKey[:min(16, len(oldestKey))])
	}
}

func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	expired := 0
	for key, entry := range c.entries {
		if time.Since(entry.CreatedAt) > c.ttl {
			delete(c.entries, key)
			expired++
		}
	}

	if expired > 0 {
		log.Printf("[Cache] Cleanup: removed %d expired entries", expired)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
