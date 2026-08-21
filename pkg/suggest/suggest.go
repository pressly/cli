package suggest

import (
	"slices"
	"strings"
)

const threshold = 0.5

type suggestion struct {
	name  string
	score float64
}

// FindSimilar returns up to maxResults candidates similar to target.
func FindSimilar(target string, candidates []string, maxResults int) []string {
	if target == "" || maxResults <= 0 {
		return []string{}
	}

	suggestions := make([]suggestion, 0, len(candidates))

	for _, name := range candidates {
		score := calculateSimilarity(target, name)
		if score > threshold {
			suggestions = append(suggestions, suggestion{name, score})
		}
	}

	slices.SortFunc(suggestions, func(a, b suggestion) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})

	limit := min(maxResults, len(suggestions))
	result := make([]string, limit)
	for i, suggestion := range suggestions[:limit] {
		result[i] = suggestion.name
	}

	return result
}

func calculateSimilarity(a, b string) float64 {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	if a == b {
		return 1
	}
	if strings.HasPrefix(b, a) {
		return 0.9
	}
	distance := levenshteinDistance(a, b)
	maxLen := float64(max(len(a), len(b)))
	return 1 - float64(distance)/maxLen
}

func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
	}

	for i := 0; i <= len(a); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	return matrix[len(a)][len(b)]
}
