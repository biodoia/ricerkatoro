package engine

import (
	"fmt"
	"strings"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
)

// BuildQuery generates a search query from a template and item fields.
// Template placeholders use {{field_name}} syntax.
// If no template is provided, a default query is built from input fields + validate fields.
func BuildQuery(item *models.ResearchItem, template string) string {
	if template == "" {
		return buildDefaultQuery(item)
	}
	query := template
	for field, value := range item.InputFields {
		query = strings.ReplaceAll(query, "{{"+field+"}}", value)
	}
	return query
}

func buildDefaultQuery(item *models.ResearchItem) string {
	var parts []string
	for _, value := range item.InputFields {
		if value != "" {
			parts = append(parts, value)
		}
	}
	if len(item.ValidateFields) > 0 {
		parts = append(parts, strings.Join(item.ValidateFields, " "))
	}
	return strings.Join(parts, " ")
}

// BuildRefinedQuery creates a more specific query after a conflict.
func BuildRefinedQuery(item *models.ResearchItem, field string, conflictValues []string) string {
	var parts []string
	for _, value := range item.InputFields {
		if value != "" {
			parts = append(parts, value)
		}
	}
	parts = append(parts, fmt.Sprintf("exact %s", field))
	if len(conflictValues) > 0 {
		parts = append(parts, fmt.Sprintf(`"%s" OR "%s"`, conflictValues[0], strings.Join(conflictValues[1:], `" OR "`)))
	}
	return strings.Join(parts, " ")
}
