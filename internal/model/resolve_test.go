//nolint:testpackage // white-box: these tests read the conflict viewer's state.
package model

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/jj"
)

// twoRegions is what jj 0.47.1 materializes for a merge whose sides both rewrote lines 2 and 7.
//
//nolint:gochecknoglobals // a fixture, built from lines so no marker starts a line of this file
var twoRegions = strings.Join([]string{
	"a",
	"<<<<<<< conflict 1 of 2",
	"+++++++ wwtworlq ec4ab3b0 \"side1\"",
	"B1",
	"------- ymzvyvxs 98d54153 \"base\"",
	"b",
	"+++++++ tvyqvpko e8fcb539 \"side2\"",
	"B2",
	">>>>>>> conflict 1 of 2 ends",
	"c",
	"d",
	"e",
	"f",
	"<<<<<<< conflict 2 of 2",
	"+++++++ wwtworlq ec4ab3b0 \"side1\"",
	"G1",
	"------- ymzvyvxs 98d54153 \"base\"",
	"g",
	"+++++++ tvyqvpko e8fcb539 \"side2\"",
	"G2",
	">>>>>>> conflict 2 of 2 ends",
	"h",
}, "\n") + "\n"

const oneRegion = "<<<<<<< conflict 1 of 1\n+++++++ wwtworlq ec4ab3b0 \"side1\"\nx1\n" +
	"------- ymzvyvxs 98d54153 \"base\"\nx\n+++++++ tvyqvpko e8fcb539 \"side2\"\nx2\n" +
	">>>>>>> conflict 1 of 1 ends\n"

func inResolve(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeResolve)
	m.revision = "@"

	return Update(t, m, conflictsLoadedMsg{files: []jj.ConflictedFile{
		{Path: "d/e/g.txt", Kind: "2-sided conflict", Text: oneRegion},
		{Path: "f.txt", Kind: "2-sided conflict", Text: twoRegions},
	}})
}

func (m *Model) resolved(file int) string {
	return m.res.files[file].parsed.Resolve(m.res.picks[file])
}

func TestResolve_Picks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		keys []string
	}{
		{"side 2", "x2\n", []string{"2"}},
		{"the base", "x\n", []string{"0"}},
		{"every side", "x1\nx2\n", []string{"a"}},
		{"a side it lacks changes nothing", oneRegion, []string{"3"}},
		{"reset", oneRegion, []string{"1", "R"}},
		{"undo", "x1\n", []string{"1", "2", "u"}},
		{"redo", "x2\n", []string{"1", "2", "u", "ctrl+r"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Press(t, inResolve(t), tt.keys...).resolved(0); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ]x steps regions and crosses into the next file at the end of one, and n repeats it.
func TestResolve_ConflictMotionCrossesFiles(t *testing.T) {
	t.Parallel()

	m := Press(t, inResolve(t), "]", "x")
	if m.res.file != 1 || m.res.region != 0 {
		t.Fatalf("]x went to file %d region %d", m.res.file, m.res.region)
	}

	m = Press(t, m, "n", "1")
	got := m.resolved(1)
	if !strings.HasSuffix(got, "f\nG1\nh\n") || !strings.Contains(got, "<<<<<<< conflict 1 of 2") {
		t.Errorf("n did not reach region 2 of f.txt:\n%s", got)
	}

	m = Press(t, m, "[", "f")
	if m.res.file != 0 {
		t.Errorf("[f went to file %d", m.res.file)
	}
}

func TestResolve_WriteNeedsAPick(t *testing.T) {
	t.Parallel()

	m := inResolve(t)
	if cmd := m.dispatch(actionConfirm); cmd != nil || !m.alarm {
		t.Errorf("an empty resolve ran: notice %q", m.notice)
	}
}

func TestResolve_QuitAsksOverPicks(t *testing.T) {
	t.Parallel()

	m := Press(t, inResolve(t), "1", "q")
	if !m.quitPrompt {
		t.Error("q quit over a pick without asking")
	}

	// w writes the picks, which in the conflict viewer is a resolve and never the squash picker.
	m.client = nil

	Press(t, m, "w")

	if m.picker.IsVisible() || !strings.Contains(m.notice, "revision") {
		t.Errorf("w did not try to resolve: %q", m.notice)
	}
}

func TestResolve_Golden(t *testing.T) {
	t.Parallel()

	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
		for _, stacked := range []bool{false, true} {
			name := fmt.Sprintf("resolve_%dx%d", size.w, size.h)
			if stacked {
				name += "_stacked"
			}

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := Press(t, inResolve(t), "]", "x", "2")
				if stacked {
					m = Press(t, m, `\`, "s")
				}

				m.width, m.height = size.w, size.h

				frame := ansi.Strip(m.render())
				if lines := strings.Split(frame, "\n"); len(lines) != size.h {
					t.Errorf("the frame is %d rows, want %d", len(lines), size.h)
				}

				golden(t, filepath.Join("testdata", name+".golden"), frame+"\n")
			})
		}
	}
}

func TestSplitLabel_DropsTheCommitHash(t *testing.T) {
	t.Parallel()

	tests := []struct{ label, id, desc string }{
		{`wwtworlq ec4ab3b0 "side1"`, "wwtworlq", "side1"},
		{`nwytptwx e926c67c "feat: read the port from PORT"`, "nwytptwx", "feat: read the port from PORT"},
		{"side #1", "side #1", ""},
	}

	for _, tt := range tests {
		if id, desc := splitLabel(tt.label); id != tt.id || desc != tt.desc {
			t.Errorf("splitLabel(%q) = %q, %q, want %q, %q", tt.label, id, desc, tt.id, tt.desc)
		}
	}
}

// longLabels is the demo merge: descriptions far longer than a third of an 80- or 100-cell screen.
const longLabels = "package main\n<<<<<<< conflict 1 of 1\n" +
	"+++++++ nwytptwx e926c67c \"feat: read the port from PORT\"\nimport (\n\t\"net/http\"\n\t\"os\"\n)\n" +
	"------- opmvqunm d6c0903d \"feat: serve hello\"\nimport \"net/http\"\n" +
	"+++++++ uklyztnw 8ebf646b \"fix: listen on 9090 and log\"\nimport (\n\t\"log\"\n\t\"net/http\"\n)\n" +
	">>>>>>> conflict 1 of 1 ends\n"

// A side's description is what a reader recognizes it by, so however narrow the panes get it is
// drawn whole, beside the title where it fits and under it where it does not, and never cut to a
// few letters in the top edge.
func TestResolve_PaneTitlesShowTheWholeDescription(t *testing.T) {
	t.Parallel()

	for _, size := range []struct{ w, h int }{{80, 24}, {100, 30}, {140, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			t.Parallel()

			m := NewTestModel(t, ModeResolve)
			m.revision = "@"
			m = Update(t, m, conflictsLoadedMsg{files: []jj.ConflictedFile{
				{Path: "server.go", Kind: "2-sided conflict", Text: longLabels},
			}})
			m.width, m.height = size.w, size.h

			frame := ansi.Strip(m.render())
			panes := paneTexts(frame)

			for i, want := range [][2]string{
				{"1 nwytptwx", "feat: read the port from PORT"},
				{"0 base opmvqunm", "feat: serve hello"},
				{"2 uklyztnw", "fix: listen on 9090 and log"},
			} {
				if i >= len(panes) || !strings.Contains(panes[i], want[0]) || !strings.Contains(panes[i], want[1]) {
					t.Errorf("pane %d does not show %q whole:\n%s", i+1, want, frame)
				}
			}

			for i, line := range strings.Split(frame, "\n") {
				if got := ansi.StringWidth(line); got > size.w {
					t.Errorf("row %d is %d cells, want at most %d", i, got, size.w)
				}
			}
		})
	}
}

// paneTexts reads the side panes out of a frame, one string per pane with its rows joined and its
// borders dropped, so a description wrapped under a title reads as one run of words.
func paneTexts(frame string) []string {
	lines := strings.Split(frame, "\n")

	for top, line := range lines {
		row := []rune(line)
		if !strings.Contains(line, "╭ 1 ") {
			continue
		}

		var out []string

		for left := range row {
			if row[left] != '╭' {
				continue
			}

			right := left + slices.Index(row[left:], '╮')

			var words []string

			for _, l := range lines[top:] {
				r := []rune(l)
				if right >= len(r) || r[left] == '╰' {
					break
				}

				words = append(words, strings.Fields(strings.Trim(string(r[left+1:right]), "─ "))...)
			}

			out = append(out, strings.Join(words, " "))
		}

		return out
	}

	return nil
}
