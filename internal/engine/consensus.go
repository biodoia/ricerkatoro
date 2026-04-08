package engine

import (
	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/pkg/fuzzy"
)

// ConsensusEngine performs cross-validation of search results.
type ConsensusEngine struct {
	Threshold float64 // minimum agreement score
}

// NewConsensusEngine creates a new consensus engine.
func NewConsensusEngine(threshold float64) *ConsensusEngine {
	if threshold <= 0 {
		threshold = 0.7
	}
	return &ConsensusEngine{Threshold: threshold}
}

// Validate performs cross-validation on an item's results for a given field.
// It returns the consensus result and whether it passed the threshold.
func (ce *ConsensusEngine) Validate(item *models.ResearchItem, field string) (models.ConsensusField, bool) {
	// Collect all values for this field from all providers
	type valueSource struct {
		Value    string
		Provider string
		URL      string
	}

	var allValues []valueSource
	for providerName, provResult := range item.Results {
		for _, pr := range provResult {
			for _, sr := range pr.Results {
				if val, ok := sr.Fields[field]; ok && val != "" {
					allValues = append(allValues, valueSource{
						Value:    val,
						Provider: providerName,
						URL:      sr.URL,
					})
				}
			}
		}
	}

	if len(allValues) == 0 {
		return models.ConsensusField{
			Field: field,
			Score: 0,
		}, false
	}

	if len(allValues) == 1 {
		return models.ConsensusField{
			Field:   field,
			Value:   allValues[0].Value,
			Score:   0.5, // single source, moderate confidence
			Sources: []string{allValues[0].Provider},
		}, 0.5 >= ce.Threshold
	}

	// Group similar values together
	type cluster struct {
		canonical string
		sources   []string
		urls      []string
		count     int
	}

	var clusters []cluster
	for _, vs := range allValues {
		matched := false
		for i := range clusters {
			if fuzzy.Similarity(clusters[i].canonical, vs.Value) >= 0.8 {
				clusters[i].count++
				clusters[i].sources = append(clusters[i].sources, vs.Provider)
				if vs.URL != "" {
					clusters[i].urls = append(clusters[i].urls, vs.URL)
				}
				matched = true
				break
			}
		}
		if !matched {
			c := cluster{
				canonical: vs.Value,
				sources:   []string{vs.Provider},
				count:     1,
			}
			if vs.URL != "" {
				c.urls = []string{vs.URL}
			}
			clusters = append(clusters, c)
		}
	}

	// Find the largest cluster
	bestIdx := 0
	for i, c := range clusters {
		if c.count > clusters[bestIdx].count {
			bestIdx = i
		}
	}

	best := clusters[bestIdx]
	score := float64(best.count) / float64(len(allValues))

	// Build conflicts
	var conflicts []models.ConflictEntry
	for i, c := range clusters {
		if i != bestIdx {
			url := ""
			if len(c.urls) > 0 {
				url = c.urls[0]
			}
			for _, src := range c.sources {
				conflicts = append(conflicts, models.ConflictEntry{
					Provider: src,
					Value:    c.canonical,
					URL:      url,
				})
			}
		}
	}

	result := models.ConsensusField{
		Field:     field,
		Value:     best.canonical,
		Score:     score,
		Sources:   best.sources,
		Conflicts: conflicts,
	}

	return result, score >= ce.Threshold
}

// ValidateItem performs validation on all validate fields for an item.
func (ce *ConsensusEngine) ValidateItem(item *models.ResearchItem) (map[string]models.ConsensusField, bool) {
	consensus := make(map[string]models.ConsensusField)
	allPassed := true

	for _, field := range item.ValidateFields {
		cf, passed := ce.Validate(item, field)
		consensus[field] = cf
		if !passed {
			allPassed = false
		}
	}

	return consensus, allPassed
}
