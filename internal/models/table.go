package models

import (
	"sync"
	"time"
)

// ItemStatus represents the state of a research item.
type ItemStatus string

const (
	StatusPending    ItemStatus = "pending"
	StatusSearching  ItemStatus = "searching"
	StatusSearched   ItemStatus = "searched"
	StatusValidated  ItemStatus = "validated"
	StatusConflict   ItemStatus = "conflict"
	StatusDone       ItemStatus = "done"
)

// ResearchItem represents a single row in the research table.
type ResearchItem struct {
	ID             string                       `json:"id"`
	InputFields    map[string]string            `json:"input_fields"`
	ValidateFields []string                     `json:"validate_fields"`
	Status         ItemStatus                   `json:"status"`
	Results        map[string][]ProviderResult  `json:"results"`        // provider_name → results
	Consensus      map[string]ConsensusField    `json:"consensus"`      // field → consensus data
	RetryCount     int                          `json:"retry_count"`
	CreatedAt      time.Time                    `json:"created_at"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

// ProviderResult wraps search results from a specific provider.
type ProviderResult struct {
	Provider  string         `json:"provider"`
	Results   []SearchResult `json:"results"`
	Error     string         `json:"error,omitempty"`
	Duration  time.Duration  `json:"duration"`
	Timestamp time.Time      `json:"timestamp"`
}

// ResearchTable holds the full table state.
type ResearchTable struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Items          []*ResearchItem `json:"items"`
	SearchFields   []string        `json:"search_fields"`   // fields used to build search queries
	ValidateFields []string        `json:"validate_fields"` // fields to validate via search
	CreatedAt      time.Time       `json:"created_at"`

	mu sync.RWMutex
}

// NewResearchTable creates a new empty table.
func NewResearchTable(id, name string, searchFields, validateFields []string) *ResearchTable {
	return &ResearchTable{
		ID:             id,
		Name:           name,
		Items:          make([]*ResearchItem, 0),
		SearchFields:   searchFields,
		ValidateFields: validateFields,
		CreatedAt:      time.Now(),
	}
}

// AddItem adds a research item to the table.
func (t *ResearchTable) AddItem(item *ResearchItem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Items = append(t.Items, item)
}

// GetItem returns an item by ID.
func (t *ResearchTable) GetItem(id string) *ResearchItem {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, item := range t.Items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

// StatusSummary returns counts of items per status.
func (t *ResearchTable) StatusSummary() map[ItemStatus]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	summary := make(map[ItemStatus]int)
	for _, item := range t.Items {
		summary[item.Status]++
	}
	return summary
}
