package diff

import (
	"github.com/sergi/go-diff/diffmatchpatch"
)

// SpanType marks a run inside a line as unchanged, added, or deleted.
type SpanType int

// Span kinds inside a line. SpanEqual is the zero value, so a span left unset renders without
// intra-line highlighting.
const (
	SpanEqual SpanType = iota
	SpanAdded
	SpanDeleted
)

// IntraLineSpan is a run of one line's content. Start and End are byte offsets into that line's
// Content with End exclusive, so a span's byte width is not its column width.
type IntraLineSpan struct {
	Start int
	End   int
	Type  SpanType
	Text  string
}

// WordDiffResult holds the spans for one deleted and added line pair. Each side's spans are
// contiguous and in order, so concatenating their Text rebuilds that side's line.
type WordDiffResult struct {
	OldSpans []IntraLineSpan
	NewSpans []IntraLineSpan
}

// ComputeWordDiff diffs two single lines into highlight spans. Pass content only, without the diff
// marker, because offsets are relative to the start of the content the renderer draws.
func ComputeWordDiff(oldLine, newLine string) WordDiffResult {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldLine, newLine, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	result := WordDiffResult{
		OldSpans: make([]IntraLineSpan, 0),
		NewSpans: make([]IntraLineSpan, 0),
	}

	oldPos := 0
	newPos := 0

	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			oldLen := len(d.Text)
			newLen := len(d.Text)
			result.OldSpans = append(result.OldSpans, IntraLineSpan{
				Start: oldPos,
				End:   oldPos + oldLen,
				Type:  SpanEqual,
				Text:  d.Text,
			})
			result.NewSpans = append(result.NewSpans, IntraLineSpan{
				Start: newPos,
				End:   newPos + newLen,
				Type:  SpanEqual,
				Text:  d.Text,
			})
			oldPos += oldLen
			newPos += newLen

		case diffmatchpatch.DiffDelete:
			oldLen := len(d.Text)
			result.OldSpans = append(result.OldSpans, IntraLineSpan{
				Start: oldPos,
				End:   oldPos + oldLen,
				Type:  SpanDeleted,
				Text:  d.Text,
			})
			oldPos += oldLen

		case diffmatchpatch.DiffInsert:
			newLen := len(d.Text)
			result.NewSpans = append(result.NewSpans, IntraLineSpan{
				Start: newPos,
				End:   newPos + newLen,
				Type:  SpanAdded,
				Text:  d.Text,
			})
			newPos += newLen
		}
	}

	return result
}

// FindLinePairs pairs each deletion in a hunk with the addition at the same position in the addition
// run that follows it, leaving the surplus on the longer side unpaired. The returned pointers alias
// hunk.Lines, so the hunk must outlive the pairs.
func FindLinePairs(hunk *Hunk) []LinePair {
	var pairs []LinePair
	i := 0
	lines := hunk.Lines

	for i < len(lines) {
		if lines[i].Type == LineDeletion {
			delStart := i
			for i < len(lines) && lines[i].Type == LineDeletion {
				i++
			}
			delEnd := i

			addStart := i
			for i < len(lines) && lines[i].Type == LineAddition {
				i++
			}
			addEnd := i

			delCount := delEnd - delStart
			addCount := addEnd - addStart

			maxPairs := delCount
			if addCount < maxPairs {
				maxPairs = addCount
			}

			for j := range maxPairs {
				pairs = append(pairs, LinePair{
					OldLineIdx: delStart + j,
					NewLineIdx: addStart + j,
					OldLine:    &lines[delStart+j],
					NewLine:    &lines[addStart+j],
				})
			}
		} else {
			i++
		}
	}

	return pairs
}

// LinePair links a deletion to the addition it is compared against. Both indices are positions in
// the hunk's Lines slice, and OldLine and NewLine point into that same slice.
type LinePair struct {
	OldLineIdx int
	NewLineIdx int
	OldLine    *Line
	NewLine    *Line
}

// ComputeHunkWordDiffs keys word diffs by hunk line index, storing one pair's result under both of
// its line indices. A line with no counterpart is absent, so a renderer treats a missing key as
// "no intra-line highlighting" instead of an error.
func ComputeHunkWordDiffs(hunk *Hunk) map[int]WordDiffResult {
	results := make(map[int]WordDiffResult)
	pairs := FindLinePairs(hunk)

	for _, pair := range pairs {
		result := ComputeWordDiff(pair.OldLine.Content, pair.NewLine.Content)
		results[pair.OldLineIdx] = result
		results[pair.NewLineIdx] = result
	}

	return results
}
