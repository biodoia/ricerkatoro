package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
)

const tavilyBaseURL = "https://api.tavily.com/search"

// TavilyProvider implements SearchProvider for the Tavily API.
type TavilyProvider struct {
	apiKey string
	client *http.Client
}

// NewTavilyProvider creates a new Tavily search provider.
func NewTavilyProvider(apiKey string) *TavilyProvider {
	return &TavilyProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *TavilyProvider) Name() string { return "tavily" }

func (t *TavilyProvider) IsAvailable() bool { return t.apiKey != "" }

type tavilyRequest struct {
	APIKey      string   `json:"api_key"`
	Query       string   `json:"query"`
	SearchDepth string   `json:"search_depth"`
	MaxResults  int      `json:"max_results"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

type tavilyResponse struct {
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (t *TavilyProvider) Search(ctx context.Context, query string, opts models.SearchOpts) ([]models.SearchResult, error) {
	if !t.IsAvailable() {
		return nil, fmt.Errorf("tavily: API key not configured")
	}

	depth := opts.SearchDepth
	if depth == "" {
		depth = "basic"
	}
	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 5
	}

	reqBody := tavilyRequest{
		APIKey:         t.apiKey,
		Query:          query,
		SearchDepth:    depth,
		MaxResults:     maxResults,
		IncludeDomains: opts.IncludeDomains,
		ExcludeDomains: opts.ExcludeDomains,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tavily: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilyBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tavily: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tavily: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var tavilyResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return nil, fmt.Errorf("tavily: decode response: %w", err)
	}

	results := make([]models.SearchResult, 0, len(tavilyResp.Results))
	for _, r := range tavilyResp.Results {
		results = append(results, models.SearchResult{
			Provider:  "tavily",
			Query:     query,
			Fields:    make(map[string]string),
			URL:       r.URL,
			Title:     r.Title,
			Snippet:   r.Content,
			Score:     r.Score,
			Timestamp: time.Now(),
		})
	}
	return results, nil
}
