package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// GetResults handles the ricerkatoro_get_results tool.
func (h *Handler) GetResults(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	tableID, _ := args["table_id"].(string)
	if tableID == "" {
		return mcp.NewToolResultError("table_id is required"), nil
	}

	table := h.getTable(tableID)
	if table == nil {
		return mcp.NewToolResultError(fmt.Sprintf("table %s not found", tableID)), nil
	}

	rowIDs := toStringSlice(args["row_ids"])
	minConfidence := getFloat(args, "min_confidence", 0)

	// Build row ID set for filtering
	rowIDSet := make(map[string]bool)
	for _, id := range rowIDs {
		rowIDSet[id] = true
	}

	type resultRow struct {
		ID          string                          `json:"id"`
		InputFields map[string]string               `json:"input_fields"`
		Status      models.ItemStatus               `json:"status"`
		Consensus   map[string]models.ConsensusField `json:"consensus"`
	}

	var results []resultRow
	for _, item := range table.Items {
		// Filter by row IDs if specified
		if len(rowIDSet) > 0 && !rowIDSet[item.ID] {
			continue
		}

		// Filter by min confidence
		if minConfidence > 0 {
			passesTreshold := false
			for _, cf := range item.Consensus {
				if cf.Score >= minConfidence {
					passesTreshold = true
					break
				}
			}
			if !passesTreshold && len(item.Consensus) > 0 {
				continue
			}
		}

		results = append(results, resultRow{
			ID:          item.ID,
			InputFields: item.InputFields,
			Status:      item.Status,
			Consensus:   item.Consensus,
		})
	}

	output := map[string]interface{}{
		"table_id":    tableID,
		"total":       len(results),
		"results":     results,
	}

	out, _ := json.MarshalIndent(output, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}
