package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	readability "codeberg.org/readeck/go-readability/v2"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WEB TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// WebSearchTool searches the web using Brave Search API
type WebSearchTool struct {
	BaseTool
	apiKey string
	client *http.Client
}

// NewWebSearchTool creates a web search tool
func NewWebSearchTool(apiKey string) *WebSearchTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results (1-10, default 5)",
			},
		},
		"required": []string{"query"},
	}

	return &WebSearchTool{
		BaseTool: NewBaseTool(
			"web_search",
			"Search the web using Brave Search API. Returns titles, URLs, and snippets.",
			params,
		),
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	count := 5
	if c, ok := args["count"].(float64); ok {
		count = int(c)
		if count < 1 {
			count = 1
		}
		if count > 10 {
			count = 10
		}
	}

	// Check API key
	if t.apiKey == "" {
		return map[string]interface{}{
			"error":   "Brave Search API key not configured",
			"hint":    "Set BRAVE_API_KEY environment variable or configure in config.yaml",
			"success": false,
		}, nil
	}

	// Build request
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("failed to create request: %v", err),
			"success": false,
		}, nil
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("search request failed: %v", err),
			"success": false,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return map[string]interface{}{
			"error":   fmt.Sprintf("search failed with status %d: %s", resp.StatusCode, string(body)),
			"success": false,
		}, nil
	}

	// Parse response
	var braveResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("failed to parse response: %v", err),
			"success": false,
		}, nil
	}

	// Format results
	var results []map[string]interface{}
	for _, r := range braveResp.Web.Results {
		results = append(results, map[string]interface{}{
			"title":   r.Title,
			"url":     r.URL,
			"snippet": r.Description,
		})
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
		"success": true,
	}, nil
}

// WebFetchTool fetches and extracts content from URLs
type WebFetchTool struct {
	BaseTool
	client *http.Client
}

// NewWebFetchTool creates a web fetch tool
func NewWebFetchTool() *WebFetchTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to fetch (http/https only)",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Extract mode: 'markdown' (default), 'text', or 'raw'",
			},
			"max_chars": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum characters to return (default 50000)",
			},
		},
		"required": []string{"url"},
	}

	return &WebFetchTool{
		BaseTool: NewBaseTool(
			"web_fetch",
			"Fetch a URL and extract readable content. Converts HTML to markdown/text.",
			params,
		),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	rawURL, ok := args["url"].(string)
	if !ok || rawURL == "" {
		return nil, fmt.Errorf("url parameter is required")
	}

	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return map[string]interface{}{
			"url":     rawURL,
			"error":   fmt.Sprintf("invalid URL: %v", err),
			"success": false,
		}, nil
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return map[string]interface{}{
			"url":     rawURL,
			"error":   "only http/https URLs are supported",
			"success": false,
		}, nil
	}

	// Block private/localhost
	host := strings.ToLower(parsedURL.Hostname())
	if host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "172.") {
		return map[string]interface{}{
			"url":     rawURL,
			"error":   "private/localhost URLs are blocked",
			"success": false,
		}, nil
	}

	mode := "markdown"
	if m, ok := args["mode"].(string); ok && m != "" {
		mode = m
	}

	maxChars := 50000
	if mc, ok := args["max_chars"].(float64); ok {
		maxChars = int(mc)
	}

	// Fetch URL
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return map[string]interface{}{
			"url":     rawURL,
			"error":   fmt.Sprintf("failed to create request: %v", err),
			"success": false,
		}, nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := t.client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"url":     rawURL,
			"error":   fmt.Sprintf("fetch failed: %v", err),
			"success": false,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"url":        rawURL,
			"error":      fmt.Sprintf("HTTP %d", resp.StatusCode),
			"status_code": resp.StatusCode,
			"success":    false,
		}, nil
	}

	// Read body with limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return map[string]interface{}{
			"url":     rawURL,
			"error":   fmt.Sprintf("failed to read response: %v", err),
			"success": false,
		}, nil
	}

	contentType := resp.Header.Get("Content-Type")
	var content string
	var title string

	// Extract content based on content type
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		// Use readability for main content extraction
		article, err := readability.FromReader(strings.NewReader(string(body)), parsedURL)
		if err != nil || article.Node == nil {
			// Fallback to basic extraction
			content = extractTextFromHTML(string(body))
			title = extractTitleFromHTML(string(body))
		} else {
			title = article.Title()
			if mode == "text" {
				var buf strings.Builder
				article.RenderText(&buf)
				content = buf.String()
			} else if mode == "raw" {
				content = string(body)
			} else {
				// markdown mode - render HTML then convert
				var buf strings.Builder
				article.RenderHTML(&buf)
				content = htmlToMarkdown(buf.String())
			}
		}
	} else if strings.Contains(contentType, "text/") || strings.Contains(contentType, "application/json") {
		content = string(body)
	} else {
		return map[string]interface{}{
			"url":          rawURL,
			"error":        fmt.Sprintf("unsupported content type: %s", contentType),
			"content_type": contentType,
			"success":      false,
		}, nil
	}

	// Truncate if needed
	if len(content) > maxChars {
		content = content[:maxChars] + "\n... (truncated)"
	}

	return map[string]interface{}{
		"url":          rawURL,
		"title":        title,
		"content":      content,
		"content_type": contentType,
		"length":       len(content),
		"success":      true,
	}, nil
}

// extractTextFromHTML extracts plain text from HTML
func extractTextFromHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}

	// Remove script and style
	doc.Find("script, style, noscript").Remove()

	return strings.TrimSpace(doc.Text())
}

// extractTitleFromHTML extracts title from HTML
func extractTitleFromHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

// htmlToMarkdown converts HTML to simple markdown
func htmlToMarkdown(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}

	var result strings.Builder

	doc.Find("body").Children().Each(func(i int, s *goquery.Selection) {
		convertNodeToMarkdown(s, &result)
	})

	return strings.TrimSpace(result.String())
}

func convertNodeToMarkdown(s *goquery.Selection, result *strings.Builder) {
	nodeName := goquery.NodeName(s)

	switch nodeName {
	case "h1":
		result.WriteString("# " + strings.TrimSpace(s.Text()) + "\n\n")
	case "h2":
		result.WriteString("## " + strings.TrimSpace(s.Text()) + "\n\n")
	case "h3":
		result.WriteString("### " + strings.TrimSpace(s.Text()) + "\n\n")
	case "h4", "h5", "h6":
		result.WriteString("#### " + strings.TrimSpace(s.Text()) + "\n\n")
	case "p":
		text := strings.TrimSpace(s.Text())
		if text != "" {
			result.WriteString(text + "\n\n")
		}
	case "ul":
		s.Children().Each(func(i int, li *goquery.Selection) {
			result.WriteString("- " + strings.TrimSpace(li.Text()) + "\n")
		})
		result.WriteString("\n")
	case "ol":
		s.Children().Each(func(i int, li *goquery.Selection) {
			result.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(li.Text())))
		})
		result.WriteString("\n")
	case "pre", "code":
		result.WriteString("```\n" + s.Text() + "\n```\n\n")
	case "blockquote":
		lines := strings.Split(strings.TrimSpace(s.Text()), "\n")
		for _, line := range lines {
			result.WriteString("> " + line + "\n")
		}
		result.WriteString("\n")
	case "a":
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		if href != "" && text != "" {
			result.WriteString(fmt.Sprintf("[%s](%s)", text, href))
		}
	case "div", "article", "section", "main":
		s.Children().Each(func(i int, child *goquery.Selection) {
			convertNodeToMarkdown(child, result)
		})
	default:
		text := strings.TrimSpace(s.Text())
		if text != "" {
			result.WriteString(text + "\n\n")
		}
	}
}
