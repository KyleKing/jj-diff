package diff

import (
	agdiff "github.com/kyleking/aragonite/diff"
)

// SidePair is one row of a two-column diff. Left or Right is nil where that side has no line, which
// is how an added line sits opposite blank space. LeftIdx and RightIdx index the hunk's Lines and
// are -1 for the nil side, so a caller can still reach anything keyed by line index.
type SidePair struct {
	Left     *Line
	Right    *Line
	LeftIdx  int
	RightIdx int
}

// PairSides groups a hunk's lines into two-column rows. A run of deletions and the run of additions
// that follows it are laid out by aragonite's AlignRun, so a modified line lands opposite what
// replaced it by content rather than by position: a line inserted above the edit, or a stray `}`,
// no longer pushes every row after it out of step. Context appears on both sides.
func PairSides(lines []Line) []SidePair {
	var pairs []SidePair

	i := 0
	for i < len(lines) {
		line := &lines[i]

		if line.Type == LineContext {
			pairs = append(pairs, SidePair{Left: line, Right: line, LeftIdx: i, RightIdx: i})
			i++

			continue
		}

		deletions := runEnd(lines, i, LineDeletion)
		additions := runEnd(lines, deletions, LineAddition)
		pairs = append(pairs, alignRun(lines, i, deletions, additions)...)
		i = additions
	}

	return pairs
}

func runEnd(lines []Line, start int, kind LineType) int {
	end := start
	for end < len(lines) && lines[end].Type == kind {
		end++
	}

	return end
}

// alignRun lays the deletions in [from, mid) against the additions in [mid, to). A row AlignRun
// leaves blank on one side carries -1 there, which is the SidePair convention too.
func alignRun(lines []Line, from, mid, to int) []SidePair {
	gone := make([]string, 0, mid-from)
	for j := from; j < mid; j++ {
		gone = append(gone, lines[j].Content)
	}

	came := make([]string, 0, to-mid)
	for j := mid; j < to; j++ {
		came = append(came, lines[j].Content)
	}

	rows := agdiff.AlignRun(gone, came)
	pairs := make([]SidePair, 0, len(rows))

	for _, row := range rows {
		pair := SidePair{LeftIdx: -1, RightIdx: -1}

		if row[0] >= 0 {
			pair.Left, pair.LeftIdx = &lines[from+row[0]], from+row[0]
		}

		if row[1] >= 0 {
			pair.Right, pair.RightIdx = &lines[mid+row[1]], mid+row[1]
		}

		pairs = append(pairs, pair)
	}

	return pairs
}
