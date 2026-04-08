package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// SearchRow handles the ricerkatoro_search_row tool.
func (h *Handler) SearchRow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	tableID, _ := args["table_id"].(string)
	rowID, _ := args["row_id"].(string)
	queryTemplate, _ := args["query_template"].(string)

	if tableID == "" || rowID == "" {
		return mcp.NewToolResultError("table_id and row_id are required"), nil
	}

	table := h.getTable(tableID)
	if table == nil {
		return mcp.NewToolResultError(fmt.Sprintf("table %s not found", tableID)), nil
	}

	item := table.GetItem(rowID)
	if item == nil {
		return mcp.NewToolResultError(fmt.Sprintf("row %s not found in table %s", rowID, tableID)), nil
	}

	// Execute search
	if err := h.Dispatcher.SearchItem(ctx, item, queryTemplate); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Run validation
	if err := h.Dispatcher.ValidateItem(ctx, item, queryTemplate); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation failed: %v", err)), nil
	}

	// Persist
	if h.Store != nil {
		h.Store.UpdateItemStatus(item)
	}

	result := map[string]interface{}{
		"row_id":    item.ID,
		"status":    item.Status,
		"consensus": item.Consensus,
		"providers_searched": len(item.Results),
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// SearchBatch handles the ricerkatoro_search_batch tool.
func (h *Handler) SearchBatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	tableID, _ := args["table_id"].(string)
	if tableID == "" {
		return mcp.NewToolResultError("table_id is required"), nil
	}

	queryTemplate, _ := args["query_template"].(string)
	startIdx := int(getFloat(args, "start", 0))
	endIdx := int(getFloat(args, "end", 0))

	table := h.getTable(tableID)
	if table == nil {
		return mcp.NewToolResultError(fmt.Sprintf("table %s not found", tableID)), nil
	}

	job, err := h.Dispatcher.SearchBatch(ctx, table, queryTemplate, startIdx, endIdx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("batch search failed: %v", err)), nil
	}

	result := map[string]interface{}{
		"job_id":    job.ID,
		"table_id":  tableID,
		"total":     job.Total,
		"status":    job.Status,
		"message":   "Batch search started. Use ricerkatoro_get_status to track progress.",
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func getFloat(args map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := args[key].(float64); ok {
		return v
	}
	return defaultVal
}
