package providers

import (
	"context"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
)

// SearchProvider is the interface all search providers must implement.
type SearchProvider interface {
	// Name returns the provider identifier.
	Name() string
	// Search executes a search query and returns normalized results.
	Search(ctx context.Context, query string, opts models.SearchOpts) ([]models.SearchResult, error)
	// IsAvailable returns true if the provider is configured and ready.
	IsAvailable() bool
}
