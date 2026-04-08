package tools

import (
	"sync"

	"github.com/autoschei/ricerkatoro-mcp/internal/engine"
	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/providers"
	"github.com/autoschei/ricerkatoro-mcp/internal/storage"
)

// Handler holds shared state for all tool implementations.
type Handler struct {
	Cfg        *models.ServerConfig
	Store      *storage.SQLiteStore
	Voyage     *storage.VoyageStore
	Providers  []providers.SearchProvider
	Dispatcher *engine.Dispatcher
	Consensus  *engine.ConsensusEngine
	Tables     map[string]*models.ResearchTable
	mu         sync.RWMutex
}

// NewHandler creates a new tool handler.
func NewHandler(
	cfg *models.ServerConfig,
	store *storage.SQLiteStore,
	voyage *storage.VoyageStore,
	provs []providers.SearchProvider,
	dispatcher *engine.Dispatcher,
	consensus *engine.ConsensusEngine,
	tables map[string]*models.ResearchTable,
) *Handler {
	return &Handler{
		Cfg:        cfg,
		Store:      store,
		Voyage:     voyage,
		Providers:  provs,
		Dispatcher: dispatcher,
		Consensus:  consensus,
		Tables:     tables,
	}
}

func (h *Handler) getTable(id string) *models.ResearchTable {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Tables[id]
}

func (h *Handler) setTable(t *models.ResearchTable) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Tables[t.ID] = t
}
