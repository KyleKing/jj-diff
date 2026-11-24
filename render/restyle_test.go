package render_test

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/render"
)

const sectioned = `diff --git a/main.go b/main.go
index 111..222 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@ func main() {
 	start()
-	limit := 10
+	limit := 20
`

// The wash is #123456, so a row carrying it says so with this background parameter.
const washParam = "48;2;18;52;86"

func restyled() render.Options {
	return render.Options{
		Width: 40,
		Palette: render.Palette{
			Added: lipgloss.Color("2"), Removed: lipgloss.Color("1"), Context: lipgloss.Color("7"),
			AddedWash: lipgloss.Color("#123456"), RemovedWash: lipgloss.Color("#123456"),
		},
		SignBar: true, HunkLabels: true, WordDiff: true,
	}
}

// A hunk reads as the function it sits in and the lines it covers, and as the range alone where
// git found no function, rather than as @@ syntax.
func TestHunkLabelsNameTheFunctionAndRange(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, diff, want string
	}{
		{name: "function", diff: sectioned, want: "func main() {  L1–3"},
		{name: "no function", diff: sample, want: "L1–3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			head := ansi.Strip(render.Lines(tc.diff, restyled())[1])
			if head != tc.want {
				t.Errorf("hunk head = %q, want %q", head, tc.want)
			}
		})
	}
}

// A changed line's side is a bar in its side's color, and the + or - is gone.
func TestSignBarReplacesThePlusAndMinus(t *testing.T) {
	t.Parallel()

	rows := render.Lines(sectioned, restyled())
	plain := make([]string, len(rows))

	for i, row := range rows {
		plain[i] = strings.TrimRight(ansi.Strip(row), " ")
	}

	want := []string{"     start()", "▎    limit := 10", "▎    limit := 20"}
	for i, w := range want {
		if got := plain[2+i]; got != w {
			t.Errorf("row %d = %q, want %q", 2+i, got, w)
		}
	}
}

// The wash runs to the frame's edge on every changed row, padding included, and leaves context
// rows clear, in both layouts.
func TestWashFillsTheChangedRowsEdgeToEdge(t *testing.T) {
	t.Parallel()

	padded := regexp.MustCompile(washParam + `m +\x1b\[m$`)

	for _, sideBySide := range []bool{false, true} {
		opts := restyled()
		opts.SideBySide = sideBySide
		opts.WordDiff = false

		rows := render.Lines(sectioned, opts)
		for _, row := range rows[2:] {
			if got := lipgloss.Width(row); got != opts.Width {
				t.Errorf("side by side %v: row %q is %d cells, want %d", sideBySide, ansi.Strip(row), got, opts.Width)
			}
		}

		if strings.Contains(rows[2], washParam) {
			t.Errorf("side by side %v: the context row %q carries the wash", sideBySide, rows[2])
		}

		if !sideBySide {
			for _, row := range rows[3:] {
				if !padded.MatchString(row) {
					t.Errorf("changed row %q does not end in washed padding", row)
				}
			}
		}
	}
}

// A line too long for the frame is cut with the marker rather than overrunning it.
func TestWashedLinesAreCutAtTheFrame(t *testing.T) {
	t.Parallel()

	opts := restyled()
	opts.Width = 8

	for _, row := range render.Lines(sectioned, opts)[2:] {
		if got := lipgloss.Width(row); got > opts.Width {
			t.Errorf("row %q is %d cells, want at most %d", ansi.Strip(row), got, opts.Width)
		}
	}
}
