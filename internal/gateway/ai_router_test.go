package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/neves/zen-claw/internal/ai"
	"github.com/neves/zen-claw/internal/config"
)

// mockProvider implements ai.Provider for testing
type mockProvider struct {
	name          string
	response      *ai.ChatResponse
	err           error
	callCount     int
	supportsTools bool
}

func (m *mockProvider) Name() string        { return m.name }
func (m *mockProvider) SupportsTools() bool { return m.supportsTools }
func (m *mockProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	m.callCount++
	return m.response, m.err
}
func (m *mockProvider) ChatStream(ctx context.Context, req ai.ChatRequest, cb ai.StreamCallback) (*ai.ChatResponse, error) {
	m.callCount++
	return m.response, m.err
}

func TestNewAIRouter(t *testing.T) {
	cfg := config.NewDefaultConfig()

	router := NewAIRouter(cfg)

	if router == nil {
		t.Fatal("NewAIRouter returned nil")
	}
	if router.cache == nil {
		t.Error("cache should be initialized")
	}
	if router.circuits == nil {
		t.Error("circuits should be initialized")
	}
}

func TestAIRouterGetAvailableProviders(t *testing.T) {
	cfg := config.NewDefaultConfig()
	router := NewAIRouter(cfg)

	// Add a mock provider directly for testing
	router.providers["test"] = &mockProvider{name: "test"}

	providers := router.GetAvailableProviders()

	found := false
	for _, p := range providers {
		if p == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected test provider in available providers")
	}
}

func TestAIRouterCacheStats(t *testing.T) {
	cfg := config.NewDefaultConfig()
	router := NewAIRouter(cfg)

	hits, misses, size, hitRate := router.GetCacheStats()

	// Initial state should be zeros
	if hits != 0 || misses != 0 || size != 0 {
		t.Errorf("expected zero stats initially, got hits=%d misses=%d size=%d", hits, misses, size)
	}
	if hitRate < 0 || hitRate > 1 {
		t.Errorf("hit rate should be 0-1, got %f", hitRate)
	}
}

func TestAIRouterCircuitStats(t *testing.T) {
	cfg := config.NewDefaultConfig()
	router := NewAIRouter(cfg)

	stats := router.GetCircuitStats()

	// Should return a map (possibly empty)
	if stats == nil {
		t.Error("expected non-nil circuit stats")
	}
}

func TestSemanticCache(t *testing.T) {
	sc := NewSemanticCache(1*time.Hour, 100)

	t.Run("stores and retrieves similar queries", func(t *testing.T) {
		// Use a query with enough keywords to be cached
		query := "how to implement kubernetes operator pattern in golang"
		response := "Here is how to implement operator pattern..."
		sc.Set(query, response)

		// Same query should return the cached response
		got, found := sc.Get(query)
		if !found {
			t.Fatal("expected to retrieve cached response")
		}
		if got != response {
			t.Errorf("content = %q, want %q", got, response)
		}
	})

	t.Run("returns empty for missing", func(t *testing.T) {
		got, found := sc.Get("completely different unique query xyz")
		if found && got != "" {
			t.Error("expected empty for non-matching query")
		}
	})

	t.Run("short queries not cached", func(t *testing.T) {
		sc.Set("hi", "hello")
		_, found := sc.Get("hi")
		if found {
			t.Error("short queries should not be cached")
		}
	})
}

func TestRequestDeduplicator(t *testing.T) {
	dedup := NewRequestDeduplicator(5 * time.Second)

	req := ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "test message"}},
	}

	t.Run("first request is not duplicate", func(t *testing.T) {
		ir, isDup := dedup.CheckDuplicate(req)
		if isDup {
			t.Error("first request should not be duplicate")
		}
		if ir == nil {
			t.Error("should return inflight request for first request")
		}
	})

	t.Run("second request is duplicate", func(t *testing.T) {
		ir, isDup := dedup.CheckDuplicate(req)
		if !isDup {
			t.Error("second request should be duplicate")
		}
		if ir == nil {
			t.Error("should return inflight request for duplicate")
		}
	})
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input    string
		minWords int // minimum expected keywords
	}{
		{"how to implement kubernetes operator", 3},
		{"the a an is", 0}, // all stopwords
		{"", 0},
		{"hello world foo bar baz", 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			keywords := extractKeywords(tt.input)
			if len(keywords) < tt.minWords {
				t.Errorf("extractKeywords(%q) = %v, want at least %d words", tt.input, keywords, tt.minWords)
			}
		})
	}
}

func TestKeywordOverlap(t *testing.T) {
	a := []string{"kubernetes", "operator", "golang"}
	b := []string{"kubernetes", "controller", "golang"}

	overlap := keywordOverlap(a, b)
	if overlap != 2 {
		t.Errorf("keywordOverlap = %d, want 2", overlap)
	}

	// No overlap
	c := []string{"python", "django"}
	overlap2 := keywordOverlap(a, c)
	if overlap2 != 0 {
		t.Errorf("keywordOverlap = %d, want 0", overlap2)
	}
}
