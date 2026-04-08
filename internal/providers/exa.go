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

const exaBaseURL = "https://api.exa.ai/search"

// ExaProvider implements SearchProvider for the Exa API.
type ExaProvider struct {
	apiKey string
	client *http.Client
}

// NewExaProvider creates a new Exa search provider.
func NewExaProvider(apiKey string) *ExaProvider {
	return &ExaProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *ExaProvider) Name() string { return "exa" }

func (e *ExaProvider) IsAvailable() bool { return e.apiKey != "" }

type exaRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
	Contents   struct {
		Text bool `json:"text"`
	} `json:"contents"`
}

type exaResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title string  `json:"title"`
	URL   string  `json:"url"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

func (e *ExaProvider) Search(ctx context.Context, query string, opts models.SearchOpts) ([]models.SearchResult, error) {
	if !e.IsAvailable() {
		return nil, fmt.Errorf("exa: API key not configured")
	}

	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 5
	}

	reqBody := exaRequest{
		Query:      query,
		NumResults: maxResults,
	}
	reqBody.Contents.Text = true

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("exa: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exaBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("exa: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exa: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exa: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var exaResp exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&exaResp); err != nil {
		return nil, fmt.Errorf("exa: decode response: %w", err)
	}

	results := make([]models.SearchResult, 0, len(exaResp.Results))
	for _, r := range exaResp.Results {
		snippet := r.Text
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		results = append(results, models.SearchResult{
			Provider:  "exa",
			Query:     query,
			Fields:    make(map[string]string),
			URL:       r.URL,
			Title:     r.Title,
			Snippet:   snippet,
			Score:     r.Score,
			Timestamp: time.Now(),
		})
	}
	return results, nil
}
