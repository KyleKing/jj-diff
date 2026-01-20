package diff

import (
	"github.com/sergi/go-diff/diffmatchpatch"
)

type SpanType int

const (
	SpanEqual SpanType = iota
	SpanAdded
	SpanDeleted
)

type IntraLineSpan struct {
	Start int
	End   int
	Type  SpanType
	Text  string
}

type WordDiffResult struct {
	OldSpans []IntraLineSpan
	NewSpans []IntraLineSpan
}

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

			for j := 0; j < maxPairs; j++ {
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

type LinePair struct {
	OldLineIdx int
	NewLineIdx int
	OldLine    *Line
	NewLine    *Line
}

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
