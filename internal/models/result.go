package models

import "time"

// SearchResult represents a normalized result from any search provider.
type SearchResult struct {
	Provider  string            `json:"provider"`
	Query     string            `json:"query"`
	Fields    map[string]string `json:"fields"`     // extracted field values
	URL       string            `json:"url"`
	Title     string            `json:"title"`
	Snippet   string            `json:"snippet"`
	Score     float64           `json:"score"`       // relevance score 0-1
	Timestamp time.Time         `json:"timestamp"`
}

// ConsensusField holds cross-validation data for a single field.
type ConsensusField struct {
	Field     string          `json:"field"`
	Value     string          `json:"value"`      // best consensus value
	Score     float64         `json:"score"`       // agreement score 0.0-1.0
	Sources   []string        `json:"sources"`     // which providers agree
	Conflicts []ConflictEntry `json:"conflicts"`   // disagreeing values
}

// ConflictEntry records a disagreement.
type ConflictEntry struct {
	Provider string `json:"provider"`
	Value    string `json:"value"`
	URL      string `json:"url,omitempty"`
}

// SearchOpts configures a search request.
type SearchOpts struct {
	MaxResults  int      `json:"max_results"`
	SearchDepth string   `json:"search_depth"` // basic | advanced
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

// JobStatus tracks a batch search job.
type JobStatus struct {
	ID          string     `json:"id"`
	TableID     string     `json:"table_id"`
	Total       int        `json:"total"`
	Completed   int        `json:"completed"`
	Failed      int        `json:"failed"`
	InProgress  int        `json:"in_progress"`
	Status      string     `json:"status"` // running | completed | failed
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
