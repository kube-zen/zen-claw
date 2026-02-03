package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/neves/zen-claw/internal/ai"
)

// RequestDeduplicator prevents duplicate requests within a time window
type RequestDeduplicator struct {
	mu       sync.RWMutex
	inflight map[string]*inflightRequest
	window   time.Duration
}

type inflightRequest struct {
	hash      string
	started   time.Time
	done      chan struct{}
	response  *ai.ChatResponse
	err       error
	completed bool
}

// NewRequestDeduplicator creates a new deduplicator
func NewRequestDeduplicator(window time.Duration) *RequestDeduplicator {
	if window == 0 {
		window = 5 * time.Second
	}
	d := &RequestDeduplicator{
		inflight: make(map[string]*inflightRequest),
		window:   window,
	}
	go d.cleanup()
	return d
}

// CheckDuplicate returns (existing, isdup) if a similar request is in flight
// If isdup is true, wait on existing.done channel for result
func (d *RequestDeduplicator) CheckDuplicate(req ai.ChatRequest) (*inflightRequest, bool) {
	hash := d.hashRequest(req)

	d.mu.Lock()
	defer d.mu.Unlock()

	if existing, ok := d.inflight[hash]; ok {
		if !existing.completed && time.Since(existing.started) < d.window {
			return existing, true
		}
	}

	// Register this request
	ir := &inflightRequest{
		hash:    hash,
		started: time.Now(),
		done:    make(chan struct{}),
	}
	d.inflight[hash] = ir

	return ir, false
}

// Complete marks a request as complete with result
func (d *RequestDeduplicator) Complete(ir *inflightRequest, resp *ai.ChatResponse, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ir.response = resp
	ir.err = err
	ir.completed = true
	close(ir.done)
}

func (d *RequestDeduplicator) hashRequest(req ai.ChatRequest) string {
	// Hash based on messages content (not tools - those vary)
	var sb strings.Builder
	for _, msg := range req.Messages {
		sb.WriteString(msg.Role)
		sb.WriteString(":")
		sb.WriteString(msg.Content)
		sb.WriteString("|")
	}
	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes
}

func (d *RequestDeduplicator) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for hash, ir := range d.inflight {
			if ir.completed && now.Sub(ir.started) > 5*time.Minute {
				delete(d.inflight, hash)
			}
		}
		d.mu.Unlock()
	}
}

// SemanticCache caches responses for semantically similar queries
type SemanticCache struct {
	mu      sync.RWMutex
	entries map[string]*semanticEntry
	ttl     time.Duration
	maxSize int
}

type semanticEntry struct {
	keywords  []string
	response  string
	timestamp time.Time
	hits      int
}

// NewSemanticCache creates a semantic similarity cache
func NewSemanticCache(ttl time.Duration, maxSize int) *SemanticCache {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	if maxSize == 0 {
		maxSize = 500
	}
	c := &SemanticCache{
		entries: make(map[string]*semanticEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go c.cleanup()
	return c
}

// Get returns cached response if query is semantically similar
func (c *SemanticCache) Get(query string) (string, bool) {
	keywords := extractKeywords(query)
	if len(keywords) < 2 {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Find best matching entry
	var bestMatch *semanticEntry
	bestScore := 0

	for _, entry := range c.entries {
		if time.Since(entry.timestamp) > c.ttl {
			continue
		}
		score := keywordOverlap(keywords, entry.keywords)
		if score > bestScore && score >= 3 { // Minimum 3 keyword overlap
			bestScore = score
			bestMatch = entry
		}
	}

	if bestMatch != nil {
		bestMatch.hits++
		return bestMatch.response, true
	}

	return "", false
}

// Set stores a response with its query keywords
func (c *SemanticCache) Set(query, response string) {
	keywords := extractKeywords(query)
	if len(keywords) < 2 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	key := strings.Join(keywords, "_")
	c.entries[key] = &semanticEntry{
		keywords:  keywords,
		response:  response,
		timestamp: time.Now(),
		hits:      0,
	}
}

func (c *SemanticCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func (c *SemanticCache) cleanup() {
	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.Sub(entry.timestamp) > c.ttl {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// extractKeywords extracts meaningful keywords from text
func extractKeywords(text string) []string {
	// Normalize
	text = strings.ToLower(text)

	// Remove common punctuation
	text = strings.Map(func(r rune) rune {
		if r == '?' || r == '!' || r == '.' || r == ',' || r == ':' || r == ';' {
			return ' '
		}
		return r
	}, text)

	// Split into words
	words := strings.Fields(text)

	// Filter stopwords and short words
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "can": true,
		"this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "which": true, "who": true,
		"when": true, "where": true, "why": true, "how": true,
		"all": true, "each": true, "every": true, "both": true,
		"few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "not": true,
		"only": true, "own": true, "same": true, "so": true,
		"than": true, "too": true, "very": true, "just": true,
		"and": true, "but": true, "or": true, "if": true, "then": true,
		"else": true, "for": true, "of": true, "to": true, "from": true,
		"in": true, "on": true, "at": true, "by": true, "with": true,
		"about": true, "into": true, "through": true, "during": true,
		"before": true, "after": true, "above": true, "below": true,
		"between": true, "under": true, "again": true, "further": true,
		"once": true, "here": true, "there": true,
		"me": true, "my": true, "your": true, "his": true, "her": true,
		"its": true, "our": true, "their": true, "please": true,
	}

	var keywords []string
	seen := make(map[string]bool)

	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if stopwords[word] {
			continue
		}
		if seen[word] {
			continue
		}
		seen[word] = true
		keywords = append(keywords, word)
	}

	return keywords
}

// keywordOverlap counts matching keywords between two sets
func keywordOverlap(a, b []string) int {
	bSet := make(map[string]bool)
	for _, w := range b {
		bSet[w] = true
	}

	count := 0
	for _, w := range a {
		if bSet[w] {
			count++
		}
	}
	return count
}
