package tools

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// Export handles the ricerkatoro_export tool.
func (h *Handler) Export(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	tableID, _ := args["table_id"].(string)
	format, _ := args["format"].(string)
	includeSources, _ := args["include_sources"].(bool)
	includeScores, _ := args["include_scores"].(bool)

	if tableID == "" || format == "" {
		return mcp.NewToolResultError("table_id and format are required"), nil
	}

	table := h.getTable(tableID)
	if table == nil {
		return mcp.NewToolResultError(fmt.Sprintf("table %s not found", tableID)), nil
	}

	switch format {
	case "json":
		return h.doExportJSON(table, includeSources, includeScores)
	case "csv":
		return h.doExportCSV(table, includeSources, includeScores)
	case "markdown":
		return h.doExportMarkdown(table, includeSources, includeScores)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported format: %s (use json, csv, or markdown)", format)), nil
	}
}

type exportRow struct {
	ID      string             `json:"id"`
	Status  string             `json:"status"`
	Input   map[string]string  `json:"input"`
	Results map[string]string  `json:"results"`
	Scores  map[string]float64 `json:"scores,omitempty"`
	Sources map[string][]string `json:"sources,omitempty"`
}

func buildRows(table *models.ResearchTable, includeSources, includeScores bool) []exportRow {
	rows := make([]exportRow, 0, len(table.Items))
	for _, item := range table.Items {
		row := exportRow{
			ID:      item.ID,
			Status:  string(item.Status),
			Input:   item.InputFields,
			Results: make(map[string]string),
		}
		if includeScores {
			row.Scores = make(map[string]float64)
		}
		if includeSources {
			row.Sources = make(map[string][]string)
		}
		for field, cf := range item.Consensus {
			row.Results[field] = cf.Value
			if includeScores {
				row.Scores[field] = cf.Score
			}
			if includeSources {
				row.Sources[field] = cf.Sources
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func (h *Handler) doExportJSON(table *models.ResearchTable, includeSources, includeScores bool) (*mcp.CallToolResult, error) {
	rows := buildRows(table, includeSources, includeScores)
	out, _ := json.MarshalIndent(rows, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func (h *Handler) doExportCSV(table *models.ResearchTable, includeSources, includeScores bool) (*mcp.CallToolResult, error) {
	rows := buildRows(table, includeSources, includeScores)
	if len(rows) == 0 {
		return mcp.NewToolResultText(""), nil
	}

	// Collect all field names
	inputFields := sortedKeys(rows[0].Input)
	resultFields := make(map[string]bool)
	for _, row := range rows {
		for k := range row.Results {
			resultFields[k] = true
		}
	}
	var resFields []string
	for k := range resultFields {
		resFields = append(resFields, k)
	}
	sort.Strings(resFields)

	var buf strings.Builder
	w := csv.NewWriter(&buf)

	// Header
	header := append([]string{"id", "status"}, inputFields...)
	for _, f := range resFields {
		header = append(header, "result_"+f)
		if includeScores {
			header = append(header, "score_"+f)
		}
	}
	w.Write(header)

	// Rows
	for _, row := range rows {
		record := []string{row.ID, row.Status}
		for _, f := range inputFields {
			record = append(record, row.Input[f])
		}
		for _, f := range resFields {
			record = append(record, row.Results[f])
			if includeScores {
				record = append(record, fmt.Sprintf("%.2f", row.Scores[f]))
			}
		}
		w.Write(record)
	}
	w.Flush()

	return mcp.NewToolResultText(buf.String()), nil
}

func (h *Handler) doExportMarkdown(table *models.ResearchTable, includeSources, includeScores bool) (*mcp.CallToolResult, error) {
	rows := buildRows(table, includeSources, includeScores)

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("# Research Results: %s\n\n", table.Name))
	buf.WriteString(fmt.Sprintf("**Table ID:** `%s`  \n", table.ID))
	buf.WriteString(fmt.Sprintf("**Total rows:** %d  \n\n", len(rows)))

	for _, row := range rows {
		buf.WriteString(fmt.Sprintf("## %s [%s]\n\n", row.ID, row.Status))

		buf.WriteString("**Input:**\n")
		for k, v := range row.Input {
			buf.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
		buf.WriteString("\n")

		if len(row.Results) > 0 {
			buf.WriteString("**Results:**\n")
			for field, value := range row.Results {
				line := fmt.Sprintf("- **%s**: %s", field, value)
				if includeScores {
					line += fmt.Sprintf(" (confidence: %.2f)", row.Scores[field])
				}
				buf.WriteString(line + "\n")
				if includeSources && len(row.Sources[field]) > 0 {
					buf.WriteString(fmt.Sprintf("  - Sources: %s\n", strings.Join(row.Sources[field], ", ")))
				}
			}
		}
		buf.WriteString("\n---\n\n")
	}

	return mcp.NewToolResultText(buf.String()), nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
