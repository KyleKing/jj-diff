package filelist_test

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/hermetic"
	"github.com/kyleking/aragonite/tui/framecheck"

	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// The theme is process-global, so it is pinned once here. Plain unicode icons draw no file glyph,
// which keeps the expected rows free of Nerd Font code points; the one test that wants the glyph
// switches the set itself and runs before the parallel tests resume.
func TestMain(m *testing.M) {
	for key, value := range map[string]string{"CATPPUCCIN_THEME": "macchiato", "ARAGONITE_ICONS": "unicode"} {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}

	theme.Init()
	hermetic.Main(m, hermetic.App("jj-diff", "jojo"))
}

func changedFile(path string, additions, deletions int) diff.FileChange {
	lines := make([]diff.Line, 0, additions+deletions)
	for range additions {
		lines = append(lines, diff.Line{Type: diff.LineAddition})
	}

	for range deletions {
		lines = append(lines, diff.Line{Type: diff.LineDeletion})
	}

	return diff.FileChange{
		Path:       path,
		ChangeType: diff.ChangeTypeModified,
		Hunks:      []diff.Hunk{{Lines: lines}},
	}
}

func expandedRows(t *testing.T, width int, files ...diff.FileChange) []string {
	t.Helper()

	m := filelist.New()
	m.SetExpanded(true)
	m.SetFiles(files)

	view := m.View(width, m.ContentHeight(), true)
	rows := strings.Split(view, "\n")

	for _, row := range rows {
		if got := lipgloss.Width(row); got != width {
			t.Errorf("row %q is %d cells, want %d", ansi.Strip(row), got, width)
		}
	}

	return strings.Split(ansi.Strip(view), "\n")
}

// The stats are two columns sized to the widest count and set against the pane's right edge, so the
// numbers line up whatever the path length is, and a count of zero leaves its cell blank.
func TestExpandedListAlignsStatsAndOmitsZeroCounts(t *testing.T) {
	t.Parallel()

	rows := expandedRows(t, 40, changedFile("a.go", 120, 0), changedFile("internal/long/path.go", 3, 45))
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want a title and one row per file: %q", len(rows), rows)
	}

	if !strings.HasPrefix(rows[0], "  ── Files 1/2 ──") {
		t.Errorf("title %q, want a labeled rule after the margin", rows[0])
	}

	for _, want := range []string{"  M a.go", "+120", "  M internal/long/path.go", "+3 -45"} {
		if !strings.Contains(strings.Join(rows, "\n"), want) {
			t.Errorf("list = %q, want it to contain %q", rows, want)
		}
	}

	if strings.Contains(strings.Join(rows, "\n"), "-0") {
		t.Errorf("list = %q, want no -0", rows)
	}

	// The add column ends in one place on both rows, and the delete column a cell before the edge.
	addEnd := strings.Index(rows[1], "+120") + len("+120")
	if got := strings.Index(rows[2], "+3") + len("+3"); got != addEnd {
		t.Errorf("add columns end at %d and %d:\n%s\n%s", addEnd, got, rows[1], rows[2])
	}

	if got := strings.Index(rows[2], "-45") + len("-45"); got != 40-1 {
		t.Errorf("delete column ends at %d, want one cell inside the edge: %q", got, rows[2])
	}
}

// A path too long for the row gives up the front of its directory, and never the counts: the
// counts used to be the part cut, so a narrow pane lost the deletions off the selection bar.
func TestExpandedListCutsTheDirectoryBeforeTheCounts(t *testing.T) {
	t.Parallel()

	rows := expandedRows(t, 30, changedFile("internal/components/statusbar/statusbar.go", 5, 4))

	if !strings.HasSuffix(rows[1], "+5 -4 ") {
		t.Errorf("row %q, want the counts against the right edge", rows[1])
	}

	if !strings.Contains(rows[1], "…") || !strings.Contains(rows[1], "statusbar.go") {
		t.Errorf("row %q, want the directory cut and the name kept", rows[1])
	}
}

// Collapsed, the selected file is the diff pane's heading: its status, path, counts, and place.
func TestCollapsedRowIsTheSelectedFilesHeading(t *testing.T) {
	t.Parallel()

	m := filelist.New()
	m.SetFiles([]diff.FileChange{changedFile("a.go", 1, 0), changedFile("internal/long/path.go", 3, 45)})
	m.SetSelected(1)

	view := m.View(60, 1, false)
	if got := lipgloss.Width(view); got != 60 {
		t.Errorf("heading is %d cells, want 60", got)
	}

	row := ansi.Strip(view)
	for _, want := range []string{"  M internal/long/path.go", "+3 -45  2/2 "} {
		if !strings.Contains(row, want) {
			t.Errorf("heading %q, want it to contain %q", row, want)
		}
	}

	if !strings.HasSuffix(row, "2/2 ") {
		t.Errorf("heading %q, want the place in the list against the right edge", row)
	}
}

// Nerd Font icons are the default, so a file's row carries its type's glyph before the path. It
// swaps the process-wide icon set, so it is not parallel, and puts the plain set back once done:
// cleanups run last-registered first, so t.Setenv restores TestMain's value before theme.Init
// rebuilds from it.
func TestRowsCarryTheFileTypeIcon(t *testing.T) {
	t.Cleanup(theme.Init)
	t.Setenv("ARAGONITE_ICONS", "nerd")
	theme.Init()

	glyph := theme.Icons.File("a.go").Text
	if glyph == "" {
		t.Fatal("the nerd set draws no glyph for a Go file")
	}

	m := filelist.New()
	m.SetExpanded(true)
	m.SetFiles([]diff.FileChange{changedFile("a.go", 1, 0)})

	if row := strings.Split(ansi.Strip(m.View(40, 2, true)), "\n")[1]; !strings.Contains(row, glyph+" a.go") {
		t.Errorf("row %q, want the Go icon before the name", row)
	}
}

// The list keeps the last frame it drew, so after every change of what it shows it must draw what a
// list that never drew before draws in the same state.
func TestKeptFrameMatchesAFreshDraw(t *testing.T) {
	t.Parallel()

	files := []diff.FileChange{changedFile("a/one.go", 3, 1), changedFile("b/two.go", 0, 4), changedFile("c.go", 2, 2)}
	other := []diff.FileChange{changedFile("z/other.go", 1, 0)}

	type state struct {
		query            string
		files            []diff.FileChange
		selected, width  int
		height           int
		expanded, filter bool
		newFiles         bool
	}

	steps := []state{
		{files: files, width: 60, height: 8, newFiles: true},
		{files: files, selected: 1, width: 60, height: 8},
		{files: files, selected: 1, width: 40, height: 8},
		{files: files, selected: 1, width: 40, height: 8, expanded: true},
		{files: files, selected: 2, width: 40, height: 6, expanded: true},
		{files: files, selected: 2, width: 40, height: 6, expanded: true, filter: true, query: "two"},
		{files: other, width: 40, height: 6, expanded: true, newFiles: true},
		{files: other, width: 40, height: 6},
	}

	apply := func(m *filelist.Model, s state) {
		m.SetFiles(s.files)
		m.SetSelected(s.selected)
		m.SetExpanded(s.expanded)
		m.SetFilterMode(s.filter)
		m.SetFilterQuery(s.query)
	}

	kept := filelist.New()

	for i, s := range steps {
		if s.newFiles {
			kept.SetFiles(s.files)
		}

		kept.SetSelected(s.selected)
		kept.SetExpanded(s.expanded)
		kept.SetFilterMode(s.filter)
		kept.SetFilterQuery(s.query)

		fresh := filelist.New()
		apply(&fresh, s)

		if got, want := kept.View(s.width, s.height, true), fresh.View(s.width, s.height, true); got != want {
			t.Errorf("step %d: the kept frame is\n%s\nwant\n%s", i, ansi.Strip(got), ansi.Strip(want))
		}
	}
}

// A file name is content, and an emoji in it is one a terminal may draw wider than lipgloss
// counts (a heart that is an emoji only with VS16, a Unicode 10+ face). The row settles it, so
// framecheck finds no unstable glyph and the counts stay against the right edge.
func TestExpandedListSettlesEmojiInPaths(t *testing.T) {
	t.Parallel()

	rows := expandedRows(t, 40, changedFile("notes/❤️-\U0001F97A.md", 3, 1), changedFile("a.go", 12, 0))

	framecheck.Require(t, t.Name(), strings.Join(rows, "\n"), framecheck.Width(40))

	for _, row := range rows[1:] {
		if end := strings.TrimRight(row, " "); !strings.HasSuffix(end, "-1") && !strings.HasSuffix(end, "+12") {
			t.Errorf("row %q does not end in its counts", row)
		}
	}
}

// colors is what an SGR stream sets: a foreground color, a background color, or both.
type colors struct{ fg, bg bool }

// sgrColors reads the colors an SGR stream sets, skipping the sub-parameters of an extended color
// so a 256-color index of 31 does not read as red.
func sgrColors(row string) colors {
	var c colors

	for _, m := range sgr.FindAllStringSubmatch(row, -1) {
		params := strings.Split(m[1], ";")
		for i := 0; i < len(params); i++ {
			n, err := strconv.Atoi(params[i])
			if err != nil {
				continue
			}

			switch {
			case n == 38 || n == 48:
				c.fg, c.bg = c.fg || n == 38, c.bg || n == 48
				i += extendedArgs(params, i)
			case n >= 30 && n <= 37, n >= 90 && n <= 97:
				c.fg = true
			case n >= 40 && n <= 47, n >= 100 && n <= 107:
				c.bg = true
			}
		}
	}

	return c
}

// extendedArgs is how many parameters follow an extended color's 38 or 48 at i: an index after 5,
// or three channels after 2.
func extendedArgs(params []string, i int) int {
	if i+1 < len(params) && params[i+1] == "5" {
		return 2
	}

	return 4
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// The selected row is drawn through the skin. Where the terminal has 16 colors the selection is
// reverse video, and a colored cell inside it would reverse into a colored block, so the row
// carries no color at all and reads as one even bar. With more colors the row keeps its cells'
// colors on the tint. Each profile is set through the environment the program reads it from, so
// the cleanups (last registered first) restore it before theme.Init rebuilds.
func TestSelectedRow_OneReverseBarAtSixteenColors(t *testing.T) {
	for _, tc := range []struct {
		name, term, colorterm string
		reverse               bool
	}{
		{name: "16 colors", term: "xterm", reverse: true},
		{name: "256 colors", term: "xterm-256color"},
		{name: "truecolor", term: "xterm-256color", colorterm: "truecolor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(theme.Init)
			t.Setenv("CLICOLOR_FORCE", "1")
			t.Setenv("TERM", tc.term)
			t.Setenv("COLORTERM", tc.colorterm)
			theme.Init()

			for _, width := range []int{30, 60} {
				checkSelectedRow(t, width, tc.reverse)
			}
		})
	}
}

func checkSelectedRow(t *testing.T, width int, reverse bool) {
	t.Helper()

	m := filelist.New()
	m.SetExpanded(true)
	m.SetFiles([]diff.FileChange{changedFile("dir/a.go", 3, 2), changedFile("b.go", 1, 0)})

	row := strings.Split(m.View(width, m.ContentHeight(), true), "\n")[1]
	if !strings.Contains(ansi.Strip(row), "a.go") {
		t.Fatalf("row %q is not the selected file", ansi.Strip(row))
	}

	got := sgrColors(row)

	switch {
	case reverse && (got.fg || got.bg || !strings.HasPrefix(row, "\x1b[7m")):
		t.Errorf("width %d: row %q, want one reverse bar with no colors in it", width, row)
	case !reverse && (!got.fg || !got.bg):
		t.Errorf("width %d: row %q, want its cells' colors on the tint", width, row)
	}
}
