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
	os.Exit(m.Run())
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

var changedRow = regexp.MustCompile(`^\s*\d+ [+-] `)

// checkRows requires every row to fill the pane and a changed row's tint to reach its last cell.
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

var tintedToEnd = regexp.MustCompile(`\x1b\[[0-9;]*48;[0-9;]+m +\x1b\[m$`)

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
