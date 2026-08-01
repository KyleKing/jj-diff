// Package fuzzy scores candidate strings against a subsequence query and ranks them, backing the
// file and revision pickers.
package fuzzy

import (
	"strings"
	"unicode"
)

// Match is one scored candidate. Indices holds the ascending byte offsets in Text that the query
// matched, and Original carries whatever value the caller paired with the candidate. Matched is
// false on the unranked passthrough Filter returns for an empty query.
type Match struct {
	Original any
	Text     string
	Indices  []int
	Score    int
	Matched  bool
}

// Scoring weights. The consecutive bonus compounds across a run, so characters matched together
// outscore the same characters spread apart, and the position penalty keeps an early match ahead of a
// late one.
const (
	matchScore             = 100
	consecutiveStep        = 25
	caseMatchBonus         = 10
	wordBoundaryBonus      = 50
	startOfTextBonus       = 40
	positionPenaltyDivisor = 3
)

// Score calculates fuzzy match score for a query against text
// Higher score is better. Returns 0 if no match.
//
// Algorithm:
// - Consecutive character matches get bonus points
// - Case-sensitive matches get bonus points
// - Matches at word boundaries get bonus points
// - Earlier matches get higher scores.
//
//nolint:gocritic // unnamedResult asks for names that nonamedreturns, also enabled, rejects.
func Score(text, query string) (int, []int) {
	if query == "" {
		return 0, nil
	}

	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)

	if !containsAllChars(textLower, queryLower) {
		return 0, nil
	}

	score := 0
	indices := []int{}
	next := 0
	consecutiveBonus := 0

	for _, qChar := range queryLower {
		matchIdx := indexOfByteRune(textLower, next, qChar)
		if matchIdx < 0 {
			return 0, nil
		}

		if len(indices) > 0 && matchIdx == indices[len(indices)-1]+1 {
			consecutiveBonus += consecutiveStep
		} else {
			consecutiveBonus = 0
		}

		indices = append(indices, matchIdx)
		score += matchScore + consecutiveBonus + positionScore(text, matchIdx, qChar)
		next = matchIdx + 1
	}

	return score, indices
}

func indexOfByteRune(text string, from int, target rune) int {
	for i := from; i < len(text); i++ {
		if rune(text[i]) == target {
			return i
		}
	}

	return -1
}

func positionScore(text string, idx int, qChar rune) int {
	score := 0
	if idx < len(text) && rune(text[idx]) == qChar {
		score += caseMatchBonus
	}

	switch {
	case idx == 0:
		score += startOfTextBonus
	case isWordBoundary(text[idx-1]):
		score += wordBoundaryBonus
	}

	return score - idx/positionPenaltyDivisor
}

func isWordBoundary(prev byte) bool {
	return prev == '/' || prev == '_' || prev == '-' || unicode.IsSpace(rune(prev))
}

func containsAllChars(text, query string) bool {
	textRunes := []rune(text)

	textIdx := 0
	for _, qChar := range query {
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

// Filter returns matches sorted by score (highest first).
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

// FilterWithData is like Filter but preserves arbitrary data with each item.
func FilterWithData(query string, items []string, data []any) []Match {
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
