package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetStatus handles the ricerkatoro_get_status tool.
func (h *Handler) GetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	jobID, _ := args["job_id"].(string)
	tableID, _ := args["table_id"].(string)

	// If job_id is provided, return job status
	if jobID != "" && h.Store != nil {
		job, err := h.Store.GetJob(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("job %s not found: %v", jobID, err)), nil
		}
		out, _ := json.MarshalIndent(job, "", "  ")
		return mcp.NewToolResultText(string(out)), nil
	}

	// Otherwise return table summary
	if tableID == "" {
		// Return all tables summary
		h.mu.RLock()
		summaries := make([]map[string]interface{}, 0)
		for _, t := range h.Tables {
			summaries = append(summaries, map[string]interface{}{
				"table_id": t.ID,
				"name":     t.Name,
				"rows":     len(t.Items),
				"status":   t.StatusSummary(),
			})
		}
		h.mu.RUnlock()
		out, _ := json.MarshalIndent(summaries, "", "  ")
		return mcp.NewToolResultText(string(out)), nil
	}

	table := h.getTable(tableID)
	if table == nil {
		return mcp.NewToolResultError(fmt.Sprintf("table %s not found", tableID)), nil
	}

	summary := map[string]interface{}{
		"table_id":        table.ID,
		"name":            table.Name,
		"total_rows":      len(table.Items),
		"search_fields":   table.SearchFields,
		"validate_fields": table.ValidateFields,
		"status_counts":   table.StatusSummary(),
	}

	// Get latest job if available
	if h.Store != nil {
		if job, err := h.Store.GetLatestJob(tableID); err == nil {
			summary["latest_job"] = job
		}
	}

	out, _ := json.MarshalIndent(summary, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}
