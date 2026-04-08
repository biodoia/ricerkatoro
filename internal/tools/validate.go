package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// Validate handles the ricerkatoro_validate tool.
func (h *Handler) Validate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	retryConflicts, _ := args["retry_conflicts"].(bool)

	// Build filter
	rowIDSet := make(map[string]bool)
	for _, id := range rowIDs {
		rowIDSet[id] = true
	}

	var validated, conflicted, skipped int

	for _, item := range table.Items {
		if len(rowIDSet) > 0 && !rowIDSet[item.ID] {
			continue
		}

		// Only validate items that have been searched
		if item.Status != models.StatusSearched && item.Status != models.StatusConflict {
			if !retryConflicts || item.Status != models.StatusConflict {
				skipped++
				continue
			}
		}

		if retryConflicts && item.Status == models.StatusConflict {
			// Re-search with refined queries
			if err := h.Dispatcher.ValidateItem(ctx, item, ""); err != nil {
				continue
			}
		} else {
			// Just re-run consensus
			consensus, passed := h.Consensus.ValidateItem(item)
			item.Consensus = consensus
			if passed {
				item.Status = models.StatusValidated
			} else {
				item.Status = models.StatusConflict
			}
		}

		if item.Status == models.StatusValidated {
			validated++
		} else {
			conflicted++
		}

		if h.Store != nil {
			h.Store.UpdateItemStatus(item)
		}
	}

	result := map[string]interface{}{
		"table_id":   tableID,
		"validated":  validated,
		"conflicted": conflicted,
		"skipped":    skipped,
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}
