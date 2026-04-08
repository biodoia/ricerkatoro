package engine

import (
	"regexp"
	"strings"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/pkg/fuzzy"
)

// Known patterns for common field types.
var knownPatterns = map[string]*regexp.Regexp{
	"email":    regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	"phone":    regexp.MustCompile(`(?:\+?\d{1,3}[\s\-]?)?\(?\d{2,4}\)?[\s\-]?\d{3,4}[\s\-]?\d{3,4}`),
	"url":      regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`),
	"website":  regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`),
	"year":     regexp.MustCompile(`\b(19|20)\d{2}\b`),
	"date":     regexp.MustCompile(`\b\d{1,2}[/\-\.]\d{1,2}[/\-\.]\d{2,4}\b`),
	"price":    regexp.MustCompile(`[$€£¥]\s?\d[\d,]*\.?\d*|\d[\d,]*\.?\d*\s?(?:USD|EUR|GBP)`),
	"revenue":  regexp.MustCompile(`[$€£¥]\s?\d[\d,]*\.?\d*\s?(?:[BMKbmk](?:illion|illion)?)?|\d[\d,]*\.?\d*\s?(?:USD|EUR|GBP)`),
	"zip":      regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`),
	"country":  regexp.MustCompile(`(?i)\b(?:United States|USA|UK|United Kingdom|Germany|France|Italy|Spain|Canada|Australia|Japan|China|India|Brazil|Netherlands|Switzerland|Sweden|Norway|Denmark|Finland|Austria|Belgium|Ireland|Portugal|Poland|Czech Republic|Romania|Hungary|Greece|Turkey|Mexico|Argentina|Colombia|Chile|Peru|South Korea|Taiwan|Singapore|Hong Kong|Indonesia|Thailand|Vietnam|Philippines|Malaysia|New Zealand|South Africa|Nigeria|Kenya|Egypt|Israel|UAE|Saudi Arabia)\b`),
	"linkedin": regexp.MustCompile(`(?:https?://)?(?:www\.)?linkedin\.com/(?:in|company)/[a-zA-Z0-9\-_.]+/?`),
	"twitter":  regexp.MustCompile(`(?:https?://)?(?:www\.)?(?:twitter|x)\.com/[a-zA-Z0-9_]+`),
}

// ExtractFields extracts structured field values from search results.
// It uses regex patterns for known field types and heuristic matching for others.
func ExtractFields(results []models.SearchResult, validateFields []string) {
	for i := range results {
		if results[i].Fields == nil {
			results[i].Fields = make(map[string]string)
		}
		for _, field := range validateFields {
			if _, exists := results[i].Fields[field]; exists && results[i].Fields[field] != "" {
				continue // already populated
			}

			text := results[i].Title + " " + results[i].Snippet

			// Try known pattern first
			if val := extractByPattern(field, text); val != "" {
				results[i].Fields[field] = val
				continue
			}

			// Try heuristic: look for "field: value" or "field - value" patterns
			if val := extractByLabel(field, text); val != "" {
				results[i].Fields[field] = val
				continue
			}

			// Try proximity: find the field name and grab adjacent content
			if val := extractByProximity(field, text); val != "" {
				results[i].Fields[field] = val
			}
		}
	}
}

// extractByPattern uses regex for known field types.
func extractByPattern(field, text string) string {
	fieldLower := strings.ToLower(field)

	// Check direct match
	if re, ok := knownPatterns[fieldLower]; ok {
		if match := re.FindString(text); match != "" {
			return strings.TrimSpace(match)
		}
	}

	// Check if field contains a known pattern name
	for name, re := range knownPatterns {
		if strings.Contains(fieldLower, name) {
			if match := re.FindString(text); match != "" {
				return strings.TrimSpace(match)
			}
		}
	}

	return ""
}

// extractByLabel looks for "field_name: value" or "field_name - value" patterns.
var labelPatterns = []string{
	`(?i)%s\s*[:=]\s*([^\n,;]{2,80})`,
	`(?i)%s\s*[-–—]\s*([^\n,;]{2,80})`,
	`(?i)%s\s+is\s+([^\n,;.]{2,80})`,
	`(?i)%s\s+was\s+([^\n,;.]{2,80})`,
}

func extractByLabel(field, text string) string {
	// Normalize field name for regex (replace underscores/hyphens with flexible space)
	fieldPattern := strings.ReplaceAll(field, "_", `[\s_\-]`)
	fieldPattern = strings.ReplaceAll(fieldPattern, "-", `[\s_\-]`)
	fieldPattern = strings.ReplaceAll(fieldPattern, " ", `[\s_\-]`)

	for _, pattern := range labelPatterns {
		re, err := regexp.Compile(strings.ReplaceAll(pattern, "%s", fieldPattern))
		if err != nil {
			continue
		}
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			val := strings.TrimSpace(matches[1])
			if len(val) > 0 && len(val) < 200 {
				return val
			}
		}
	}
	return ""
}

// extractByProximity finds the field name in text and grabs nearby content.
func extractByProximity(field, text string) string {
	fieldLower := strings.ToLower(field)
	textLower := strings.ToLower(text)

	idx := strings.Index(textLower, fieldLower)
	if idx == -1 {
		// Try fuzzy field name variants
		variants := []string{
			strings.ReplaceAll(fieldLower, "_", " "),
			strings.ReplaceAll(fieldLower, "-", " "),
		}
		for _, v := range variants {
			idx = strings.Index(textLower, v)
			if idx != -1 {
				break
			}
		}
	}

	if idx == -1 {
		return ""
	}

	// Grab text after the field name
	start := idx + len(field)
	if start >= len(text) {
		return ""
	}

	// Skip separators
	remaining := text[start:]
	remaining = strings.TrimLeft(remaining, " :=-–—\t")

	// Take until end of sentence/line
	endMarkers := []string{"\n", ". ", ", ", "; ", " - ", " | "}
	end := len(remaining)
	for _, marker := range endMarkers {
		if i := strings.Index(remaining, marker); i > 0 && i < end {
			end = i
		}
	}

	val := strings.TrimSpace(remaining[:end])
	if len(val) > 0 && len(val) < 200 {
		return val
	}
	return ""
}

// ScoreSnippetRelevance scores how relevant a snippet is to the search query fields.
func ScoreSnippetRelevance(snippet string, inputFields map[string]string) float64 {
	if snippet == "" || len(inputFields) == 0 {
		return 0
	}

	matches := 0
	total := 0
	for _, value := range inputFields {
		if value == "" {
			continue
		}
		total++
		if fuzzy.ContainsMatch(snippet, value, 0.7) {
			matches++
		}
	}

	if total == 0 {
		return 0
	}
	return float64(matches) / float64(total)
}
