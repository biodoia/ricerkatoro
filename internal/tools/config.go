package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/autoschei/ricerkatoro-mcp/internal/engine"
	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/providers"
	"github.com/autoschei/ricerkatoro-mcp/internal/storage"
	"github.com/mark3labs/mcp-go/mcp"
)

// Config handles the ricerkatoro_config tool.
func (h *Handler) Config(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	// Lock to prevent race conditions during config mutation
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update providers
	if provData, ok := args["providers"]; ok {
		provJSON, _ := json.Marshal(provData)
		var provConfigs []models.ProviderConfig
		if err := json.Unmarshal(provJSON, &provConfigs); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid providers format: %v", err)), nil
		}
		h.Cfg.Providers = provConfigs

		// Rebuild providers list
		var provs []providers.SearchProvider
		for _, pc := range provConfigs {
			if !pc.Enabled || pc.APIKey == "" {
				continue
			}
			switch pc.Name {
			case "tavily":
				provs = append(provs, providers.NewTavilyProvider(pc.APIKey))
			case "brave":
				provs = append(provs, providers.NewBraveProvider(pc.APIKey))
			case "exa":
				provs = append(provs, providers.NewExaProvider(pc.APIKey))
			}
		}
		h.Providers = provs

		// Rebuild dispatcher with new providers
		h.Dispatcher = engine.NewDispatcher(
			provs, h.Consensus, h.Store,
			h.Cfg.MaxConcurrency, h.Cfg.MaxRetries,
		)
	}

	if voyageKey, ok := args["voyage_api_key"].(string); ok && voyageKey != "" {
		h.Cfg.VoyageConfig.APIKey = voyageKey
		h.Voyage = storage.NewVoyageStore(voyageKey, h.Cfg.VoyageConfig.Model)
	}

	if conc, ok := args["concurrency"].(float64); ok && conc > 0 {
		h.Cfg.MaxConcurrency = int(conc)
	}

	if thresh, ok := args["confidence_threshold"].(float64); ok && thresh > 0 {
		h.Cfg.ConfidenceThreshold = thresh
		h.Consensus = engine.NewConsensusEngine(thresh)
	}

	// Build status response
	var activeProviders []string
	for _, p := range h.Providers {
		if p.IsAvailable() {
			activeProviders = append(activeProviders, p.Name())
		}
	}

	result := map[string]interface{}{
		"status":               "configured",
		"active_providers":     activeProviders,
		"concurrency":          h.Cfg.MaxConcurrency,
		"confidence_threshold": h.Cfg.ConfidenceThreshold,
		"voyage_available":     h.Voyage.IsAvailable(),
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// parseArgs extracts arguments as map[string]interface{} from a CallToolRequest.
func parseArgs(req mcp.CallToolRequest) map[string]interface{} {
	if req.Params.Arguments == nil {
		return make(map[string]interface{})
	}
	// Arguments is `any`, try to assert or marshal/unmarshal
	if m, ok := req.Params.Arguments.(map[string]interface{}); ok {
		return m
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	var m map[string]interface{}
	json.Unmarshal(raw, &m)
	if m == nil {
		return make(map[string]interface{})
	}
	return m
}
