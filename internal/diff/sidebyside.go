package diff

// SidePair is one row of a two-column diff. Left or Right is nil where that side has no line, which
// is how an added line sits opposite blank space. LeftIdx and RightIdx index the hunk's Lines and
// are -1 for the nil side, so a caller can still reach anything keyed by line index.
type SidePair struct {
	Left     *Line
	Right    *Line
	LeftIdx  int
	RightIdx int
}

// PairSides groups a hunk's lines into two-column rows. A run of deletions is matched against the
// run of additions that follows it, so a modified line lands opposite what replaced it rather than
// below it. Context appears on both sides.
func PairSides(lines []Line) []SidePair {
	var pairs []SidePair

	i := 0
	for i < len(lines) {
		line := &lines[i]

		switch line.Type {
		case LineContext:
			pairs = append(pairs, SidePair{Left: line, Right: line, LeftIdx: i, RightIdx: i})
			i++

		case LineDeletion:
			deletions := runEnd(lines, i, LineDeletion)
			additions := runEnd(lines, deletions, LineAddition)
			pairs = append(pairs, pairRuns(lines, i, deletions, additions)...)
			i = additions

		case LineAddition:
			pairs = append(pairs, SidePair{Right: line, RightIdx: i, LeftIdx: -1})
			i++
		}
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

// pairRuns zips the deletions in [from, mid) against the additions in [mid, to), leaving the longer
// run's tail opposite blank space.
func pairRuns(lines []Line, from, mid, to int) []SidePair {
	deletions, additions := mid-from, to-mid
	pairs := make([]SidePair, 0, max(deletions, additions))

	for j := range max(deletions, additions) {
		pair := SidePair{LeftIdx: -1, RightIdx: -1}

		if j < deletions {
			pair.Left, pair.LeftIdx = &lines[from+j], from+j
		}

		if j < additions {
			pair.Right, pair.RightIdx = &lines[mid+j], mid+j
		}

		pairs = append(pairs, pair)
	}

	return pairs
}
