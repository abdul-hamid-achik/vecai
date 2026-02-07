package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// WebSearchTool performs web searches using the Tavily API
type WebSearchTool struct {
	httpClient  *http.Client
	lastRequest time.Time
	minDelay    time.Duration
	mu          sync.Mutex
}

// NewWebSearchTool creates a new WebSearchTool with configured HTTP client
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		minDelay: 1 * time.Second,
	}
}

// NewWebSearchToolWithDelay creates a new WebSearchTool with a custom minimum request delay
func NewWebSearchToolWithDelay(minDelay time.Duration) *WebSearchTool {
	return &WebSearchTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		minDelay: minDelay,
	}
}

// waitForRateLimit enforces minimum delay between API requests
func (t *WebSearchTool) waitForRateLimit() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.lastRequest.IsZero() {
		elapsed := time.Since(t.lastRequest)
		if elapsed < t.minDelay {
			time.Sleep(t.minDelay - elapsed)
		}
	}
	t.lastRequest = time.Now()
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for current information. Returns relevant results with titles, URLs, and content snippets. Use this to find up-to-date information, documentation, tutorials, or answers to questions."
}

func (t *WebSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query string.",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (1-10, default: 5).",
				"default":     5,
				"minimum":     1,
				"maximum":     10,
			},
			"search_depth": map[string]any{
				"type":        "string",
				"description": "Search depth: 'basic' for fast results, 'advanced' for more comprehensive search.",
				"enum":        []string{"basic", "advanced"},
				"default":     "basic",
			},
			"include_domains": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of domains to specifically include in the search (e.g., ['github.com', 'stackoverflow.com']).",
			},
			"exclude_domains": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of domains to exclude from the search.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Permission() PermissionLevel {
	return PermissionExecute // Sends data to external service; requires explicit approval
}

// tavilyRequest represents the request body for Tavily API
type tavilyRequest struct {
	APIKey         string   `json:"api_key"`
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results,omitempty"`
	SearchDepth    string   `json:"search_depth,omitempty"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
	IncludeAnswer  bool     `json:"include_answer"`
}

// tavilyResponse represents the response from Tavily API
type tavilyResponse struct {
	Answer  string         `json:"answer,omitempty"`
	Query   string         `json:"query"`
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (t *WebSearchTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	// Get API key from environment
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("TAVILY_API_KEY environment variable is required for web search")
	}

	// Parse query (required)
	query, ok := input["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Build request
	req := tavilyRequest{
		APIKey:        apiKey,
		Query:         query,
		MaxResults:    5,
		SearchDepth:   "basic",
		IncludeAnswer: true,
	}

	// Parse optional parameters
	if maxResults, ok := input["max_results"].(float64); ok {
		mr := int(maxResults)
		if mr < 1 {
			mr = 1
		} else if mr > 10 {
			mr = 10
		}
		req.MaxResults = mr
	}

	if searchDepth, ok := input["search_depth"].(string); ok {
		if searchDepth == "basic" || searchDepth == "advanced" {
			req.SearchDepth = searchDepth
		}
	}

	if includeDomains, ok := input["include_domains"].([]any); ok {
		for _, d := range includeDomains {
			if domain, ok := d.(string); ok {
				req.IncludeDomains = append(req.IncludeDomains, domain)
			}
		}
	}

	if excludeDomains, ok := input["exclude_domains"].([]any); ok {
		for _, d := range excludeDomains {
			if domain, ok := d.(string); ok {
				req.ExcludeDomains = append(req.ExcludeDomains, domain)
			}
		}
	}

	// Wait for rate limit before making API request
	t.waitForRateLimit()

	// Make API request
	result, err := t.callTavilyAPI(ctx, req)
	if err != nil {
		return "", fmt.Errorf("web search failed: %w", err)
	}

	return formatWebSearchResults(query, result), nil
}

func (t *WebSearchTool) callTavilyAPI(ctx context.Context, req tavilyRequest) (*tavilyResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result tavilyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func formatWebSearchResults(query string, resp *tavilyResponse) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Web Search Results for: \"%s\"\n\n", query))

	// Include AI-generated answer if available
	if resp.Answer != "" {
		sb.WriteString("### Summary\n")
		sb.WriteString(resp.Answer)
		sb.WriteString("\n\n---\n\n")
	}

	if len(resp.Results) == 0 {
		sb.WriteString("No results found.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("### Results (%d found)\n\n", len(resp.Results)))

	for i, r := range resp.Results {
		sb.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("URL: %s\n", r.URL))
		if r.Score > 0 {
			sb.WriteString(fmt.Sprintf("Relevance: %.0f%%\n", r.Score*100))
		}
		sb.WriteString("\n")
		if r.Content != "" {
			// Truncate long content
			content := r.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(content)
			sb.WriteString("\n")
		}
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}
