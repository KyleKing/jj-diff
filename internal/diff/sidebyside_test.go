package diff_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

// hunkLines builds a hunk's lines from unified-diff text, one line per string with its marker.
func hunkLines(text ...string) []diff.Line {
	lines := make([]diff.Line, 0, len(text))

	for _, t := range text {
		kind := diff.LineContext

		switch t[0] {
		case '+':
			kind = diff.LineAddition
		case '-':
			kind = diff.LineDeletion
		}

		lines = append(lines, diff.Line{Type: kind, Content: t[1:]})
	}

	return lines
}

// rowsOf spells each pair as "left | right", with "." for a blank side.
func rowsOf(pairs []diff.SidePair) []string {
	side := func(l *diff.Line) string {
		if l == nil {
			return "."
		}

		return strings.TrimSpace(l.Content)
	}

	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, side(p.Left)+" | "+side(p.Right))
	}

	return out
}

func TestPairSides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []diff.Line
		want  []string
	}{
		{
			name:  "context sits on both sides",
			lines: hunkLines(" a", " b"),
			want:  []string{"a | a", "b | b"},
		},
		{
			name:  "a replacement sits opposite what it replaced",
			lines: hunkLines(" ctx", "-const limit = 10", "+const limit = 20", " end"),
			want:  []string{"ctx | ctx", "const limit = 10 | const limit = 20", "end | end"},
		},
		{
			name:  "a pure addition sits opposite a blank",
			lines: hunkLines(" ctx", "+added", " end"),
			want:  []string{"ctx | ctx", ". | added", "end | end"},
		},
		{
			name:  "a pure deletion sits opposite a blank",
			lines: hunkLines("-gone", " end"),
			want:  []string{"gone | .", "end | end"},
		},
		{
			// Positional zipping put the inserted line opposite the old return and every row after
			// it one out of step. Aligned by content, the edited line stays with its replacement.
			name: "a line inserted above an edit does not shift the edit",
			lines: hunkLines(
				"-\treturn total + count",
				"+\tlog.Println(total)",
				"+\treturn total + count + 1",
			),
			want: []string{". | log.Println(total)", "return total + count | return total + count + 1"},
		},
		{
			// A lone brace shares nothing but itself, so positionally it dragged the next real
			// edit out of line with what it replaced.
			name: "a stray closing brace does not anchor the alignment",
			lines: hunkLines(
				"-}",
				"-func greet(name string) string {",
				"+func greet(name, greeting string) string {",
			),
			want: []string{"} | .", "func greet(name string) string { | func greet(name, greeting string) string {"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := rowsOf(diff.PairSides(tt.lines))
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("rows:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Every line lands in exactly one row, on its own side, in order, and its index names it.
func TestPairSidesKeepsEveryLineOnceInOrder(t *testing.T) {
	t.Parallel()

	lines := hunkLines(" a", "-b", "-}", "-c", "+c2", "+}", "+d", " e", "+f", "-g")
	pairs := diff.PairSides(lines)

	var left, right []int

	for _, p := range pairs {
		if p.Left != nil {
			if &lines[p.LeftIdx] != p.Left {
				t.Errorf("LeftIdx %d does not name its line", p.LeftIdx)
			}

			left = append(left, p.LeftIdx)
		} else if p.LeftIdx != -1 {
			t.Errorf("blank left side carries index %d", p.LeftIdx)
		}

		if p.Right != nil && p.Right.Type != diff.LineContext {
			if &lines[p.RightIdx] != p.Right {
				t.Errorf("RightIdx %d does not name its line", p.RightIdx)
			}

			right = append(right, p.RightIdx)
		} else if p.Right == nil && p.RightIdx != -1 {
			t.Errorf("blank right side carries index %d", p.RightIdx)
		}
	}

	wantLeft := []int{0, 1, 2, 3, 7, 9}
	if fmt.Sprint(left) != fmt.Sprint(wantLeft) {
		t.Errorf("left indexes %v, want %v", left, wantLeft)
	}

	wantRight := []int{4, 5, 6, 8}
	if fmt.Sprint(right) != fmt.Sprint(wantRight) {
		t.Errorf("right indexes %v, want %v", right, wantRight)
	}
}
