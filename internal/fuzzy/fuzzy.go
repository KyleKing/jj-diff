package fuzzy

import (
	"strings"
	"unicode"
)

type Match struct {
	Text     string
	Score    int
	Matched  bool
	Indices  []int
	Original interface{}
}

// Score calculates fuzzy match score for a query against text
// Higher score is better. Returns 0 if no match.
//
// Algorithm:
// - Consecutive character matches get bonus points
// - Case-sensitive matches get bonus points
// - Matches at word boundaries get bonus points
// - Earlier matches get higher scores
func Score(text, query string) (int, []int) {
	if query == "" {
		return 0, nil
	}

	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)

	// Quick check: all query chars must exist in text
	if !containsAllChars(textLower, queryLower) {
		return 0, nil
	}

	score := 0
	indices := []int{}
	textIdx := 0
	consecutiveBonus := 0

	for _, qChar := range queryLower {
		// Find next occurrence of query character
		found := false
		for textIdx < len(textLower) {
			if rune(textLower[textIdx]) == qChar {
				found = true
				indices = append(indices, textIdx)

				// Base score for match
				baseScore := 100

				// Bonus for consecutive matches (higher bonus as streak continues)
				if len(indices) > 1 && indices[len(indices)-1] == indices[len(indices)-2]+1 {
					consecutiveBonus += 25
					baseScore += consecutiveBonus
				} else {
					consecutiveBonus = 0
				}

				// Bonus for case match
				if textIdx < len(text) && rune(text[textIdx]) == qChar {
					baseScore += 10
				}

				// Bonus for word boundary (after /, _, -, or space)
				if textIdx > 0 {
					prevChar := text[textIdx-1]
					if prevChar == '/' || prevChar == '_' || prevChar == '-' || unicode.IsSpace(rune(prevChar)) {
						baseScore += 50
					}
				} else {
					// Start of string bonus
					baseScore += 40
				}

				// Penalty for later matches (prefer matches near start)
				penalty := textIdx / 3
				baseScore -= penalty

				score += baseScore
				textIdx++
				break
			}
			textIdx++
		}

		if !found {
			return 0, nil
		}
	}

	return score, indices
}

func containsAllChars(text, query string) bool {
	textRunes := []rune(text)
	queryRunes := []rune(query)

	textIdx := 0
	for _, qChar := range queryRunes {
		found := false
		for textIdx < len(textRunes) {
			if textRunes[textIdx] == qChar {
				found = true
				textIdx++
				break
			}
			textIdx++
		}
		if !found {
			return false
		}
	}
	return true
}

// Filter returns matches sorted by score (highest first)
func Filter(query string, items []string) []Match {
	if query == "" {
		matches := make([]Match, len(items))
		for i, item := range items {
			matches[i] = Match{
				Text:     item,
				Score:    0,
				Matched:  false,
				Original: item,
			}
		}
		return matches
	}

	matches := []Match{}
	for _, item := range items {
		score, indices := Score(item, query)
		if score > 0 {
			matches = append(matches, Match{
				Text:     item,
				Score:    score,
				Matched:  true,
				Indices:  indices,
				Original: item,
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

// FilterWithData is like Filter but preserves arbitrary data with each item
func FilterWithData(query string, items []string, data []interface{}) []Match {
	if len(items) != len(data) {
		return nil
	}

	if query == "" {
		matches := make([]Match, len(items))
		for i, item := range items {
			matches[i] = Match{
				Text:     item,
				Score:    0,
				Matched:  false,
				Original: data[i],
			}
		}
		return matches
	}

	matches := []Match{}
	for i, item := range items {
		score, indices := Score(item, query)
		if score > 0 {
			matches = append(matches, Match{
				Text:     item,
				Score:    score,
				Matched:  true,
				Indices:  indices,
				Original: data[i],
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}
