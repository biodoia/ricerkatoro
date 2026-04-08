package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/providers"
	"github.com/autoschei/ricerkatoro-mcp/internal/storage"
)

// Dispatcher manages parallel search execution.
type Dispatcher struct {
	providers       []providers.SearchProvider
	consensus       *ConsensusEngine
	store           *storage.SQLiteStore
	maxConcurrency  int
	maxRetries      int
}

// NewDispatcher creates a new parallel dispatcher.
func NewDispatcher(
	provs []providers.SearchProvider,
	consensus *ConsensusEngine,
	store *storage.SQLiteStore,
	maxConcurrency int,
	maxRetries int,
) *Dispatcher {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	if maxRetries <= 0 {
		maxRetries = 2
	}
	return &Dispatcher{
		providers:      provs,
		consensus:      consensus,
		store:          store,
		maxConcurrency: maxConcurrency,
		maxRetries:     maxRetries,
	}
}

// SearchItem executes search on a single item across all providers.
func (d *Dispatcher) SearchItem(ctx context.Context, item *models.ResearchItem, queryTemplate string) error {
	item.Status = models.StatusSearching
	if item.Results == nil {
		item.Results = make(map[string][]models.ProviderResult)
	}

	query := BuildQuery(item, queryTemplate)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, p := range d.providers {
		if !p.IsAvailable() {
			continue
		}
		wg.Add(1)
		go func(prov providers.SearchProvider) {
			defer wg.Done()
			start := time.Now()
			results, err := prov.Search(ctx, query, models.SearchOpts{
				MaxResults:  5,
				SearchDepth: "basic",
			})
			// Extract structured fields from snippets
			if len(results) > 0 {
				ExtractFields(results, item.ValidateFields)
			}
			pr := models.ProviderResult{
				Provider:  prov.Name(),
				Results:   results,
				Duration:  time.Since(start),
				Timestamp: time.Now(),
			}
			if err != nil {
				pr.Error = err.Error()
				log.Printf("[dispatcher] provider %s error for item %s: %v", prov.Name(), item.ID, err)
			}
			mu.Lock()
			item.Results[prov.Name()] = append(item.Results[prov.Name()], pr)
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	item.Status = models.StatusSearched
	return nil
}

// ValidateItem runs consensus validation on an item, retrying if needed.
func (d *Dispatcher) ValidateItem(ctx context.Context, item *models.ResearchItem, queryTemplate string) error {
	consensus, passed := d.consensus.ValidateItem(item)
	item.Consensus = consensus

	if passed {
		item.Status = models.StatusValidated
		return nil
	}

	if item.RetryCount >= d.maxRetries {
		item.Status = models.StatusConflict
		return nil
	}

	// Retry with refined queries for conflicting fields
	item.RetryCount++
	for field, cf := range consensus {
		if cf.Score < d.consensus.Threshold && len(cf.Conflicts) > 0 {
			var conflictValues []string
			for _, c := range cf.Conflicts {
				conflictValues = append(conflictValues, c.Value)
			}
			refinedQuery := BuildRefinedQuery(item, field, conflictValues)
			if err := d.searchWithQuery(ctx, item, refinedQuery); err != nil {
				log.Printf("[dispatcher] retry search failed for item %s field %s: %v", item.ID, field, err)
			}
		}
	}

	// Re-validate after retry
	consensus, passed = d.consensus.ValidateItem(item)
	item.Consensus = consensus
	if passed {
		item.Status = models.StatusValidated
	} else {
		item.Status = models.StatusConflict
	}
	return nil
}

func (d *Dispatcher) searchWithQuery(ctx context.Context, item *models.ResearchItem, query string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, p := range d.providers {
		if !p.IsAvailable() {
			continue
		}
		wg.Add(1)
		go func(prov providers.SearchProvider) {
			defer wg.Done()
			start := time.Now()
			results, err := prov.Search(ctx, query, models.SearchOpts{
				MaxResults:  5,
				SearchDepth: "advanced",
			})
			// Extract structured fields from retry snippets
			if len(results) > 0 {
				ExtractFields(results, item.ValidateFields)
			}
			pr := models.ProviderResult{
				Provider:  prov.Name(),
				Results:   results,
				Duration:  time.Since(start),
				Timestamp: time.Now(),
			}
			if err != nil {
				pr.Error = err.Error()
			}
			mu.Lock()
			item.Results[prov.Name()] = append(item.Results[prov.Name()], pr)
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return nil
}

// SearchBatch runs parallel search on multiple items.
// Returns a job ID for status tracking.
func (d *Dispatcher) SearchBatch(
	ctx context.Context,
	table *models.ResearchTable,
	queryTemplate string,
	startIdx, endIdx int,
) (*models.JobStatus, error) {
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx <= 0 || endIdx > len(table.Items) {
		endIdx = len(table.Items)
	}
	items := table.Items[startIdx:endIdx]

	job := &models.JobStatus{
		ID:        fmt.Sprintf("job_%d", time.Now().UnixNano()),
		TableID:   table.ID,
		Total:     len(items),
		Status:    "running",
		StartedAt: time.Now(),
	}
	if d.store != nil {
		d.store.SaveJob(job)
	}

	sem := make(chan struct{}, d.maxConcurrency)
	var completed atomic.Int32
	var failed atomic.Int32
	var wg sync.WaitGroup

	for _, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(it *models.ResearchItem) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := d.SearchItem(ctx, it, queryTemplate); err != nil {
				log.Printf("[dispatcher] search failed for item %s: %v", it.ID, err)
				failed.Add(1)
			} else {
				// Run validation
				if err := d.ValidateItem(ctx, it, queryTemplate); err != nil {
					log.Printf("[dispatcher] validation failed for item %s: %v", it.ID, err)
				}
				completed.Add(1)
			}

			// Persist item state
			if d.store != nil {
				d.store.UpdateItemStatus(it)
			}

			// Update job progress
			job.Completed = int(completed.Load())
			job.Failed = int(failed.Load())
			job.InProgress = job.Total - job.Completed - job.Failed
			if d.store != nil {
				d.store.SaveJob(job)
			}
		}(item)
	}

	// Run in background, return job immediately
	go func() {
		wg.Wait()
		now := time.Now()
		job.CompletedAt = &now
		job.Status = "completed"
		job.Completed = int(completed.Load())
		job.Failed = int(failed.Load())
		job.InProgress = 0
		if d.store != nil {
			d.store.SaveJob(job)
		}
	}()

	return job, nil
}
