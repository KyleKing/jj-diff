package diffview_test

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/hermetic"
	"github.com/kyleking/aragonite/tui/framecheck"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func TestMain(m *testing.M) {
	for key, value := range map[string]string{
		"CATPPUCCIN_THEME": "macchiato",
		"CLICOLOR_FORCE":   "1",
		"COLORTERM":        "truecolor",
	} {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	theme.Init()
	flag.Parse()
	hermetic.Main(m, hermetic.App("jj-diff", "jojo"))
}

// mixedDiff is tab-indented edits, a reindent-only hunk, a lockfile, and removed lines that begin "-- ".
func mixedDiff() []diff.FileChange {
	return diff.Parse(strings.Join([]string{
		"diff --git a/go.sum b/go.sum",
		"--- a/go.sum",
		"+++ b/go.sum",
		"@@ -1,2 +1,6 @@",
		" github.com/a/b v1.0.0 h1:aaa=",
		" github.com/a/b v1.0.0/go.mod h1:bbb=",
		"+github.com/c/d v2.1.0 h1:ccc=",
		"+github.com/c/d v2.1.0/go.mod h1:ddd=",
		"+github.com/e/f v0.3.0 h1:eee=",
		"+github.com/e/f v0.3.0/go.mod h1:fff=",
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -5,11 +5,11 @@ func greet(name string) string {",
		" \t\"strings\"",
		" )",
		" ",
		"-const limit = 10",
		"+const limit = 20",
		" ",
		" // greet builds the greeting for name.",
		" func greet(name string) string {",
		"-\treturn \"hello, \" + name",
		"+\treturn \"hello, \" + name + \"!\"",
		" }",
		" ",
		" func main() {",
		"diff --git a/reindent.go b/reindent.go",
		"--- a/reindent.go",
		"+++ b/reindent.go",
		"@@ -1,7 +1,7 @@ func work(items []string) int {",
		" package main",
		" ",
		" func work(items []string) int {",
		"-\ttotal := 0",
		"-\tcount := len(items)",
		"-\tlog.Println(total, count)",
		"-\treturn total + count",
		"+\t\ttotal := 0",
		"+\t\tcount := len(items)",
		"+\t\tlog.Println(total, count)",
		"+\t\treturn total + count",
		" }",
		"diff --git a/schema.sql b/schema.sql",
		"--- a/schema.sql",
		"+++ b/schema.sql",
		"@@ -1,4 +1,4 @@",
		"--- users table",
		"+-- the users table",
		" CREATE TABLE users (id INTEGER PRIMARY KEY);",
		"--- orders table",
		"+-- the orders table",
		" CREATE TABLE orders (id INTEGER PRIMARY KEY);",
		"",
	}, "\n"))
}

func fileNamed(t *testing.T, name string) diff.FileChange {
	t.Helper()

	for _, file := range mixedDiff() {
		if file.Path == name {
			return file
		}
	}

	t.Fatalf("no file %q in the fixture", name)

	return diff.FileChange{}
}

var changedRow = regexp.MustCompile(`^\s*\d+ [▎+−] `)

// checkRows requires every row to fill the pane and a changed row's wash to reach its last cell,
// short of the overview track where the pane draws one.
func checkRows(t *testing.T, view string, width, height int, unified bool) {
	t.Helper()

	rows := strings.Split(view, "\n")
	if len(rows) != height {
		t.Errorf("view has %d rows, want %d", len(rows), height)
	}

	for _, row := range rows {
		if got := lipgloss.Width(row); got != width {
			t.Errorf("row %q is %d cells wide, want %d", ansi.Strip(row), got, width)
		}

		if unified && changedRow.MatchString(ansi.Strip(row)) && !tintedToEnd.MatchString(row) {
			t.Errorf("changed row %q does not carry its tint to the last cell: %q", ansi.Strip(row), row)
		}
	}
}

var tintedToEnd = regexp.MustCompile(`\x1b\[[0-9;]*48;[0-9;]+m +\x1b\[m(\x1b\[[0-9;]*m[│┃▍•]\x1b\[m)?$`)

func compareGolden(t *testing.T, name, got string) {
	t.Helper()

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
	}

	root, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("opening testdata (run with -update to create it): %v", err)
	}

	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})

	file := name + ".golden"

	if *update {
		if err := root.WriteFile(file, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	want, err := root.ReadFile(file)
	if err != nil {
		t.Fatalf("reading golden (run with -update to create it): %v", err)
	}

	if string(want) != got {
		t.Errorf("view differs from testdata/%s (run with -update and review the diff)\n--- got\n%s", file, got)
	}
}

func TestViewGolden(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		setup  func(*diffview.Model)
		name   string
		file   string
		mode   config.ViewModeType
		scroll int
	}{
		{name: "unified", file: "main.go", mode: config.ViewModeUnified},
		{name: "side_by_side", file: "main.go", mode: config.ViewModeSideBySide},
		{name: "side_by_side_scrolled", file: "main.go", mode: config.ViewModeSideBySide, scroll: 6},
		{name: "mechanical_hunk_folded", file: "reindent.go", mode: config.ViewModeUnified},
		{
			name: "mechanical_hunk_expanded", file: "reindent.go", mode: config.ViewModeUnified,
			setup: func(m *diffview.Model) { m.ToggleFold(0) },
		},
		{name: "generated_file_collapsed", file: "go.sum", mode: config.ViewModeUnified},
		{
			name: "generated_file_expanded", file: "go.sum", mode: config.ViewModeUnified,
			setup: func(m *diffview.Model) { m.ToggleFold(0) },
		},
		{name: "header_lookalike_lines", file: "schema.sql", mode: config.ViewModeUnified},
	}

	for _, size := range [][2]int{{80, 24}, {160, 45}} {
		for _, sc := range scenarios {
			t.Run(fmt.Sprintf("%s_%dx%d", sc.name, size[0], size[1]), func(t *testing.T) {
				t.Parallel()

				cfg := config.DefaultConfig()
				cfg.ViewMode = sc.mode
				m := diffview.New(cfg)
				m.SetFileChange(fileNamed(t, sc.file))

				if sc.setup != nil {
					sc.setup(&m)
				}

				m.Scroll(sc.scroll)

				view := m.View(size[0], size[1], true)
				checkRows(t, view, size[0], size[1], sc.mode == config.ViewModeUnified)
				framecheck.Require(t, t.Name(), view, framecheck.Width(size[0]))
				compareGolden(t, fmt.Sprintf("%s_%dx%d", sc.name, size[0], size[1]), ansi.Strip(view))
			})
		}
	}
}

func TestSideBySideDrawsWordMarks(t *testing.T) {
	t.Parallel()

	render := func(wordDiff bool) string {
		cfg := config.DefaultConfig()
		cfg.ViewMode = config.ViewModeSideBySide
		cfg.WordLevelDiff = wordDiff

		m := diffview.New(cfg)
		m.SetFileChange(fileNamed(t, "main.go"))

		return m.View(120, 20, true)
	}

	plain, marked := render(false), render(true)

	if ansi.Strip(plain) != ansi.Strip(marked) {
		t.Fatal("word marks changed the text drawn")
	}

	if plain == marked {
		t.Error("side-by-side view draws no word marks")
	}
}

func TestRevealShowsWhatIsFolded(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ file, want string }{
		{"reindent.go", "log.Println"},
		{"go.sum", "github.com/e/f"},
	} {
		m := diffview.New(config.DefaultConfig())
		m.SetFileChange(fileNamed(t, tc.file))

		if strings.Contains(m.View(120, 30, true), tc.want) {
			t.Fatalf("%s shows %q before it is revealed", tc.file, tc.want)
		}

		m.Reveal(0)

		if !strings.Contains(ansi.Strip(m.View(120, 30, true)), tc.want) {
			t.Errorf("%s does not show %q once revealed", tc.file, tc.want)
		}
	}
}

// tallFile is one hunk of context with an edit near the bottom, taller than any pane the tests draw.
func tallFile() diff.FileChange {
	lines := []string{"diff --git a/tall.go b/tall.go", "--- a/tall.go", "+++ b/tall.go", "@@ -1,60 +1,60 @@"}
	for i := range 60 {
		switch i {
		case 50:
			lines = append(lines, fmt.Sprintf("-line %d", i), fmt.Sprintf("+line %d changed", i))
		default:
			lines = append(lines, fmt.Sprintf(" line %d", i))
		}
	}

	return diff.Parse(strings.Join(append(lines, ""), "\n"))[0]
}

// lastColumn is the final cell of every row, which is where the overview track draws.
func lastColumn(view string) string {
	var b strings.Builder

	for _, row := range strings.Split(ansi.Strip(view), "\n") {
		r := []rune(row)
		b.WriteRune(r[len(r)-1])
	}

	return b.String()
}

// The track says what waits off screen: the edit below the frame and a search hit above it.
func TestOverviewTrackMarksChangesAndHitsOffScreen(t *testing.T) {
	t.Parallel()

	for _, mode := range []config.ViewModeType{config.ViewModeUnified, config.ViewModeSideBySide} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.ViewMode = mode

			m := diffview.New(cfg)
			m.SetFileChange(tallFile())
			m.Scroll(20)

			view := m.View(80, 12, true)
			checkRows(t, view, 80, 12, false)

			track := lastColumn(view)
			if !strings.Contains(track, "┃") || !strings.Contains(track, "▍") {
				t.Errorf("track %q wants the thumb and a change mark below it", track)
			}

			if strings.Contains(track, "•") {
				t.Errorf("track %q marks a hit before any search", track)
			}

			m.SetSearchHits([]diffview.SearchHit{{Hunk: 0, Line: 2}})

			if track := lastColumn(m.View(80, 12, true)); !strings.Contains(track, "•") {
				t.Errorf("track %q wants the hit above the frame", track)
			}
		})
	}
}

// Content that fits needs no track, so it keeps the column.
func TestOverviewTrackIsAbsentWhenTheFileFits(t *testing.T) {
	t.Parallel()

	m := diffview.New(config.DefaultConfig())
	m.SetFileChange(fileNamed(t, "main.go"))

	if track := lastColumn(m.View(80, 24, true)); strings.ContainsAny(track, "│┃▍") {
		t.Errorf("track %q drawn for a file that fits", track)
	}
}

// foreground matches an SGR sequence that sets a foreground color, basic, bright, or extended.
var foreground = regexp.MustCompile(`\x1b\[(?:\d+;)*(?:3[0-8]|9[0-7])(?:;\d+)*m`)

// The line under the cursor is drawn on the skin's selection. At 16 colors that is reverse video,
// where the line number, the sign, and every syntax token would reverse into colored blocks, so
// the row between the margin and the track carries no color and reads as one even bar. With
// millions of colors the tokens keep their colors on the tint. Each profile is set through the
// environment the theme reads it from; the cleanups (last registered first) restore it before
// theme.Init rebuilds.
func TestCursorLine_OneReverseBarAtSixteenColors(t *testing.T) {
	for _, tc := range []struct {
		name, term, colorterm string
		reverse               bool
	}{
		{name: "16 colors", term: "xterm", reverse: true},
		{name: "truecolor", term: "xterm-256color", colorterm: "truecolor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(theme.Init)
			t.Setenv("TERM", tc.term)
			t.Setenv("COLORTERM", tc.colorterm)
			theme.Init()

			for _, width := range []int{60, 100} {
				m := diffview.New(config.DefaultConfig())
				m.SetFileChange(fileNamed(t, "main.go"))
				m.SetSelection(0, func(int) bool { return false })
				m.SetVisualState(4, true, 4, func(int, int) bool { return false })

				row := cursorRow(t, m.View(width, 12, true))
				colored := foreground.MatchString(row)

				switch {
				case tc.reverse && (colored || !strings.HasPrefix(row, "\x1b[7m")):
					t.Errorf("width %d: cursor row %q, want one reverse bar with no colors in it", width, row)
				case !tc.reverse && !colored:
					t.Errorf("width %d: cursor row %q, want its tokens' colors on the tint", width, row)
				}
			}
		})
	}
}

// cursorRow is the cursor line's body: from its number to the end of its padding, past the
// margin's indicator and short of the overview track.
func cursorRow(t *testing.T, view string) string {
	t.Helper()

	for _, row := range strings.Split(view, "\n") {
		if !strings.Contains(ansi.Strip(row), "const limit = 20") {
			continue
		}

		_, rest, _ := strings.Cut(row, "┃ \x1b[m")
		if at := strings.LastIndex(rest, "\x1b[m"); at >= 0 {
			if track := strings.LastIndex(rest[:at], "\x1b[m"); track >= 0 {
				return rest[:track+len("\x1b[m")]
			}
		}

		return rest
	}

	t.Fatal("no cursor row in the view")

	return ""
}
