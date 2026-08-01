package diff_test

import (
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

func TestComputeWordDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		oldLine        string
		newLine        string
		expectOldSpans int
		expectNewSpans int
	}{
		{
			name:           "identical lines",
			oldLine:        "hello world",
			newLine:        "hello world",
			expectOldSpans: 1,
			expectNewSpans: 1,
		},
		{
			name:           "single word change",
			oldLine:        "hello world",
			newLine:        "hello earth",
			expectOldSpans: 2,
			expectNewSpans: 2,
		},
		{
			name:           "completely different",
			oldLine:        "foo",
			newLine:        "bar",
			expectOldSpans: 1,
			expectNewSpans: 1,
		},
		{
			name:           "empty to content",
			oldLine:        "",
			newLine:        "hello",
			expectOldSpans: 0,
			expectNewSpans: 1,
		},
		{
			name:           "content to empty",
			oldLine:        "hello",
			newLine:        "",
			expectOldSpans: 1,
			expectNewSpans: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := diff.ComputeWordDiff(tt.oldLine, tt.newLine)

			if len(result.OldSpans) != tt.expectOldSpans {
				t.Errorf("Expected %d old spans, got %d", tt.expectOldSpans, len(result.OldSpans))
			}
			if len(result.NewSpans) != tt.expectNewSpans {
				t.Errorf("Expected %d new spans, got %d", tt.expectNewSpans, len(result.NewSpans))
			}
		})
	}
}

func TestComputeWordDiffSpanTypes(t *testing.T) {
	t.Parallel()

	result := diff.ComputeWordDiff("old_function()", "new_function()")

	hasDeleted := false
	hasAdded := false
	hasEqual := false

	for _, span := range result.OldSpans {
		switch span.Type {
		case diff.SpanDeleted:
			hasDeleted = true
		case diff.SpanEqual:
			hasEqual = true
		case diff.SpanAdded:
			t.Error("Old line should carry no added span")
		}
	}

	for _, span := range result.NewSpans {
		if span.Type == diff.SpanAdded {
			hasAdded = true
		}
	}

	if !hasDeleted {
		t.Error("Expected deleted span in old line")
	}
	if !hasAdded {
		t.Error("Expected added span in new line")
	}
	if !hasEqual {
		t.Error("Expected equal span for common parts")
	}
}

func TestFindLinePairs(t *testing.T) {
	t.Parallel()

	hunk := &diff.Hunk{
		Lines: []diff.Line{
			{Type: diff.LineContext, Content: "context"},
			{Type: diff.LineDeletion, Content: "old line"},
			{Type: diff.LineAddition, Content: "new line"},
			{Type: diff.LineContext, Content: "more context"},
		},
	}

	pairs := diff.FindLinePairs(hunk)

	if len(pairs) != 1 {
		t.Fatalf("Expected 1 pair, got %d", len(pairs))
	}

	pair := pairs[0]
	if pair.OldLineIdx != 1 {
		t.Errorf("Expected OldLineIdx=1, got %d", pair.OldLineIdx)
	}
	if pair.NewLineIdx != 2 {
		t.Errorf("Expected NewLineIdx=2, got %d", pair.NewLineIdx)
	}
	if pair.OldLine.Content != "old line" {
		t.Errorf("Expected old content 'old line', got %q", pair.OldLine.Content)
	}
	if pair.NewLine.Content != "new line" {
		t.Errorf("Expected new content 'new line', got %q", pair.NewLine.Content)
	}
}

func TestFindLinePairsMultiplePairs(t *testing.T) {
	t.Parallel()

	hunk := &diff.Hunk{
		Lines: []diff.Line{
			{Type: diff.LineDeletion, Content: "old1"},
			{Type: diff.LineDeletion, Content: "old2"},
			{Type: diff.LineAddition, Content: "new1"},
			{Type: diff.LineAddition, Content: "new2"},
		},
	}

	pairs := diff.FindLinePairs(hunk)

	if len(pairs) != 2 {
		t.Fatalf("Expected 2 pairs, got %d", len(pairs))
	}

	if pairs[0].OldLine.Content != "old1" || pairs[0].NewLine.Content != "new1" {
		t.Error("First pair mismatch")
	}
	if pairs[1].OldLine.Content != "old2" || pairs[1].NewLine.Content != "new2" {
		t.Error("Second pair mismatch")
	}
}

func TestFindLinePairsUnbalanced(t *testing.T) {
	t.Parallel()

	hunk := &diff.Hunk{
		Lines: []diff.Line{
			{Type: diff.LineDeletion, Content: "old1"},
			{Type: diff.LineDeletion, Content: "old2"},
			{Type: diff.LineDeletion, Content: "old3"},
			{Type: diff.LineAddition, Content: "new1"},
		},
	}

	pairs := diff.FindLinePairs(hunk)

	if len(pairs) != 1 {
		t.Fatalf("Expected 1 pair (min of 3 del, 1 add), got %d", len(pairs))
	}
}

func TestComputeHunkWordDiffs(t *testing.T) {
	t.Parallel()

	hunk := &diff.Hunk{
		Lines: []diff.Line{
			{Type: diff.LineContext, Content: "context"},
			{Type: diff.LineDeletion, Content: "hello world"},
			{Type: diff.LineAddition, Content: "hello earth"},
		},
	}

	results := diff.ComputeHunkWordDiffs(hunk)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results (for indices 1 and 2), got %d", len(results))
	}

	if _, ok := results[1]; !ok {
		t.Error("Expected result for line index 1")
	}
	if _, ok := results[2]; !ok {
		t.Error("Expected result for line index 2")
	}
}
