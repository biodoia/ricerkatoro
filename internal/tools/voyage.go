package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/storage"
	"github.com/mark3labs/mcp-go/mcp"
)

// VoyageStore handles the ricerkatoro_voyage_store tool.
func (h *Handler) VoyageStore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	tableID, _ := args["table_id"].(string)
	namespace, _ := args["namespace"].(string)

	if tableID == "" {
		return mcp.NewToolResultError("table_id is required"), nil
	}

	if !h.Voyage.IsAvailable() {
		return mcp.NewToolResultError("Voyage AI is not configured. Set voyage_api_key via ricerkatoro_config first."), nil
	}

	table := h.getTable(tableID)
	if table == nil {
		return mcp.NewToolResultError(fmt.Sprintf("table %s not found", tableID)), nil
	}

	rowIDs := toStringSlice(args["row_ids"])
	rowIDSet := make(map[string]bool)
	for _, id := range rowIDs {
		rowIDSet[id] = true
	}

	// Collect items to embed
	var textsToEmbed []string
	var itemRefs []*models.ResearchItem

	for _, item := range table.Items {
		if len(rowIDSet) > 0 && !rowIDSet[item.ID] {
			continue
		}
		if item.Status != models.StatusValidated && item.Status != models.StatusDone {
			continue
		}

		text := buildEmbeddingText(item)
		textsToEmbed = append(textsToEmbed, text)
		itemRefs = append(itemRefs, item)
	}

	if len(textsToEmbed) == 0 {
		return mcp.NewToolResultText(`{"stored": 0, "message": "No validated items to embed"}`), nil
	}

	// Generate embeddings via Voyage AI
	embeddings, err := h.Voyage.Embed(ctx, textsToEmbed, "document")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("embedding failed: %v", err)), nil
	}

	// Persist to SQLite
	stored := 0
	for i, item := range itemRefs {
		if i >= len(embeddings) {
			break
		}

		metadata := make(map[string]string)
		for k, v := range item.InputFields {
			metadata["input_"+k] = v
		}
		for field, cf := range item.Consensus {
			metadata["result_"+field] = cf.Value
			metadata["score_"+field] = fmt.Sprintf("%.2f", cf.Score)
		}
		metadata["table_id"] = tableID
		metadata["status"] = string(item.Status)

		rec := &storage.EmbeddingRecord{
			ID:        fmt.Sprintf("%s_%s", tableID, item.ID),
			Vector:    embeddings[i],
			Text:      textsToEmbed[i],
			Metadata:  metadata,
			Namespace: namespace,
			CreatedAt: time.Now(),
		}

		if h.Store != nil {
			if err := h.Store.SaveEmbedding(rec); err != nil {
				continue
			}
		}
		stored++

		item.Status = models.StatusDone
		if h.Store != nil {
			h.Store.UpdateItemStatus(item)
		}
	}

	result := map[string]interface{}{
		"stored":    stored,
		"namespace": namespace,
		"message":   fmt.Sprintf("Embedded and stored %d items in SQLite", stored),
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// VoyageSearch handles the ricerkatoro_voyage_search tool.
func (h *Handler) VoyageSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	if !h.Voyage.IsAvailable() {
		return mcp.NewToolResultError("Voyage AI is not configured"), nil
	}

	topK := int(getFloat(args, "top_k", 5))
	namespace, _ := args["namespace"].(string)
	minScore := getFloat(args, "min_score", 0)

	// Embed the query
	embeddings, err := h.Voyage.Embed(ctx, []string{query}, "query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query embedding failed: %v", err)), nil
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return mcp.NewToolResultError("empty embedding returned"), nil
	}
	queryVec := embeddings[0]

	// Search persisted embeddings in SQLite
	if h.Store == nil {
		return mcp.NewToolResultError("storage not available"), nil
	}

	scored, err := h.Store.SearchEmbeddings(queryVec, namespace, topK, minScore)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	type searchResult struct {
		ID       string            `json:"id"`
		Score    float64           `json:"score"`
		Text     string            `json:"text"`
		Metadata map[string]string `json:"metadata"`
	}

	results := make([]searchResult, 0, len(scored))
	for _, s := range scored {
		results = append(results, searchResult{
			ID:       s.Record.ID,
			Score:    s.Score,
			Text:     s.Record.Text,
			Metadata: s.Record.Metadata,
		})
	}

	output := map[string]interface{}{
		"query":   query,
		"results": results,
		"total":   len(results),
	}

	out, _ := json.MarshalIndent(output, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func buildEmbeddingText(item *models.ResearchItem) string {
	var parts []string
	for k, v := range item.InputFields {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	for field, cf := range item.Consensus {
		parts = append(parts, fmt.Sprintf("%s: %s (confidence: %.2f)", field, cf.Value, cf.Score))
	}
	return strings.Join(parts, " | ")
}
