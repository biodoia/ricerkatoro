package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// LoadTable handles the ricerkatoro_load_table tool.
func (h *Handler) LoadTable(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	name, _ := args["name"].(string)
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	dataRaw, ok := args["data"]
	if !ok {
		return mcp.NewToolResultError("data is required"), nil
	}

	searchFieldsRaw, _ := args["search_fields"]
	validateFieldsRaw, _ := args["validate_fields"]

	searchFields := toStringSlice(searchFieldsRaw)
	validateFields := toStringSlice(validateFieldsRaw)

	if len(searchFields) == 0 {
		return mcp.NewToolResultError("search_fields is required (at least one field)"), nil
	}
	if len(validateFields) == 0 {
		return mcp.NewToolResultError("validate_fields is required (at least one field)"), nil
	}

	// Parse data array
	dataJSON, _ := json.Marshal(dataRaw)
	var rows []map[string]interface{}
	if err := json.Unmarshal(dataJSON, &rows); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid data format: %v", err)), nil
	}

	tableID := fmt.Sprintf("tbl_%d", time.Now().UnixNano())
	table := models.NewResearchTable(tableID, name, searchFields, validateFields)

	for i, row := range rows {
		inputFields := make(map[string]string)
		for k, v := range row {
			inputFields[k] = fmt.Sprintf("%v", v)
		}

		item := &models.ResearchItem{
			ID:             fmt.Sprintf("row_%d", i),
			InputFields:    inputFields,
			ValidateFields: validateFields,
			Status:         models.StatusPending,
			Results:        make(map[string][]models.ProviderResult),
			Consensus:      make(map[string]models.ConsensusField),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		table.AddItem(item)
	}

	// Persist
	h.setTable(table)
	if h.Store != nil {
		h.Store.SaveTable(table)
		for _, item := range table.Items {
			h.Store.SaveItem(tableID, item)
		}
	}

	result := map[string]interface{}{
		"table_id":        tableID,
		"name":            name,
		"rows_loaded":     len(rows),
		"search_fields":   searchFields,
		"validate_fields": validateFields,
		"status":          "loaded",
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	raw, _ := json.Marshal(v)
	var result []string
	json.Unmarshal(raw, &result)
	return result
}
