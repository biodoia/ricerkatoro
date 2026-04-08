package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
)

const braveBaseURL = "https://api.search.brave.com/res/v1/web/search"

// BraveProvider implements SearchProvider for Brave Search API.
type BraveProvider struct {
	apiKey string
	client *http.Client
}

// NewBraveProvider creates a new Brave search provider.
func NewBraveProvider(apiKey string) *BraveProvider {
	return &BraveProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *BraveProvider) Name() string { return "brave" }

func (b *BraveProvider) IsAvailable() bool { return b.apiKey != "" }

type braveResponse struct {
	Web struct {
		Results []braveResult `json:"results"`
	} `json:"web"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func (b *BraveProvider) Search(ctx context.Context, query string, opts models.SearchOpts) ([]models.SearchResult, error) {
	if !b.IsAvailable() {
		return nil, fmt.Errorf("brave: API key not configured")
	}

	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 5
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", strconv.Itoa(maxResults))

	reqURL := braveBaseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("brave: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var braveResp braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return nil, fmt.Errorf("brave: decode response: %w", err)
	}

	results := make([]models.SearchResult, 0, len(braveResp.Web.Results))
	for i, r := range braveResp.Web.Results {
		score := 1.0 - float64(i)*0.1
		if score < 0.1 {
			score = 0.1
		}
		results = append(results, models.SearchResult{
			Provider:  "brave",
			Query:     query,
			Fields:    make(map[string]string),
			URL:       r.URL,
			Title:     r.Title,
			Snippet:   r.Description,
			Score:     score,
			Timestamp: time.Now(),
		})
	}
	return results, nil
}
