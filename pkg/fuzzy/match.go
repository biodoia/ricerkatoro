package fuzzy

import (
	"strings"
	"unicode"
)

// Normalize lowercases, trims, and removes extra whitespace from a string.
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Collapse multiple spaces
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// StripPunctuation removes common punctuation from a string.
func StripPunctuation(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// LevenshteinDistance computes the edit distance between two strings.
func LevenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two rows instead of full matrix
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// Similarity returns a 0.0-1.0 similarity score between two strings.
// It normalizes both strings before comparison.
func Similarity(a, b string) float64 {
	a = Normalize(StripPunctuation(a))
	b = Normalize(StripPunctuation(b))

	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	maxLen := max(len(a), len(b))
	dist := LevenshteinDistance(a, b)
	return 1.0 - float64(dist)/float64(maxLen)
}

// ContainsMatch checks if needle is approximately contained in haystack.
func ContainsMatch(haystack, needle string, threshold float64) bool {
	haystack = Normalize(haystack)
	needle = Normalize(needle)

	if strings.Contains(haystack, needle) {
		return true
	}

	return Similarity(haystack, needle) >= threshold
}
