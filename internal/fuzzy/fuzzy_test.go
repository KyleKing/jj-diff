package fuzzy

import (
	"testing"
)

func TestScore_ExactMatch(t *testing.T) {
	score, indices := Score("hello", "hello")
	if score == 0 {
		t.Error("Expected exact match to have score > 0")
	}
	if len(indices) != 5 {
		t.Errorf("Expected 5 indices, got %d", len(indices))
	}
}

func TestScore_NoMatch(t *testing.T) {
	score, _ := Score("hello", "xyz")
	if score != 0 {
		t.Errorf("Expected no match (score 0), got score %d", score)
	}
}

func TestScore_PartialMatch(t *testing.T) {
	score, indices := Score("hello world", "hlo")
	if score == 0 {
		t.Error("Expected partial match to have score > 0")
	}
	if len(indices) != 3 {
		t.Errorf("Expected 3 indices, got %d", len(indices))
	}
}

func TestScore_CaseInsensitive(t *testing.T) {
	score1, _ := Score("Hello", "hello")
	score2, _ := Score("hello", "HELLO")

	if score1 == 0 || score2 == 0 {
		t.Error("Expected case-insensitive matches to have score > 0")
	}
}

func TestScore_ConsecutiveBonus(t *testing.T) {
	// "hel" in "hello" should score higher than "hel" in "hxexlxlxo"
	score1, _ := Score("hello", "hel")
	score2, _ := Score("hxexlxlxo", "hel")

	if score1 <= score2 {
		t.Errorf("Expected consecutive match (%d) to score higher than non-consecutive (%d)", score1, score2)
	}
}

func TestScore_WordBoundaryBonus(t *testing.T) {
	// "fc" should match "FileClass" better than "performance"
	score1, _ := Score("FileClass", "fc")
	score2, _ := Score("performance", "fc")

	if score1 <= score2 {
		t.Errorf("Expected word boundary match (%d) to score higher than middle match (%d)", score1, score2)
	}
}

func TestScore_PathMatching(t *testing.T) {
	paths := []string{
		"internal/model/model.go",
		"internal/model/model_test.go",
		"cmd/main.go",
	}

	// "mdl" should match "model" files best
	for _, path := range paths[:2] {
		score, _ := Score(path, "mdl")
		if score == 0 {
			t.Errorf("Expected 'mdl' to match %s", path)
		}
	}

	// "cmd" should match cmd/main.go best
	score1, _ := Score(paths[2], "cmd")
	score2, _ := Score(paths[0], "cmd")

	if score1 <= score2 {
		t.Errorf("Expected 'cmd' to match cmd/main.go (%d) better than model path (%d)", score1, score2)
	}
}

func TestFilter_EmptyQuery(t *testing.T) {
	items := []string{"foo", "bar", "baz"}
	matches := Filter("", items)

	if len(matches) != len(items) {
		t.Errorf("Expected all items with empty query, got %d", len(matches))
	}

	for _, match := range matches {
		if match.Matched {
			t.Error("Expected no items to be marked as matched with empty query")
		}
	}
}

func TestFilter_SortsByScore(t *testing.T) {
	items := []string{
		"internal/components/diffview/diffview.go",
		"internal/diff/parser.go",
		"internal/diff/patch.go",
	}

	matches := Filter("diff", items)

	if len(matches) == 0 {
		t.Fatal("Expected matches for 'diff' query")
	}

	// Verify sorted by score descending
	for i := 0; i < len(matches)-1; i++ {
		if matches[i].Score < matches[i+1].Score {
			t.Errorf("Matches not sorted: match[%d].Score=%d < match[%d].Score=%d",
				i, matches[i].Score, i+1, matches[i+1].Score)
		}
	}
}

func TestFilter_NoMatches(t *testing.T) {
	items := []string{"foo", "bar", "baz"}
	matches := Filter("xyz", items)

	if len(matches) != 0 {
		t.Errorf("Expected no matches for 'xyz', got %d", len(matches))
	}
}

func TestFilterWithData_PreservesData(t *testing.T) {
	items := []string{"foo", "bar"}
	data := []interface{}{42, "hello"}

	matches := FilterWithData("foo", items, data)

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	if matches[0].Original.(int) != 42 {
		t.Errorf("Expected Original to be 42, got %v", matches[0].Original)
	}
}

func TestScore_RealWorldExamples(t *testing.T) {
	testCases := []struct {
		text          string
		query         string
		shouldMatch   bool
		betterMatches []string
	}{
		{
			text:        "internal/model/model.go",
			query:       "mm",
			shouldMatch: true,
		},
		{
			text:        "tests/integration/client_test.go",
			query:       "tic",
			shouldMatch: true,
		},
		{
			text:        "README.md",
			query:       "rm",
			shouldMatch: true,
		},
	}

	for _, tc := range testCases {
		score, _ := Score(tc.text, tc.query)
		if tc.shouldMatch && score == 0 {
			t.Errorf("Expected '%s' to match '%s'", tc.query, tc.text)
		}
		if !tc.shouldMatch && score > 0 {
			t.Errorf("Expected '%s' to NOT match '%s'", tc.query, tc.text)
		}
	}
}
