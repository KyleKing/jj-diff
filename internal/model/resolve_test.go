//nolint:testpackage // white-box: these tests read the conflict viewer's state.
package model

import (
	"fmt"
	"path/filepath"
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

func TestShortLabel_DropsTheCommitHash(t *testing.T) {
	t.Parallel()

	if got := shortLabel(`wwtworlq ec4ab3b0 "side1"`); got != `wwtworlq "side1"` {
		t.Errorf("got %q", got)
	}

	if got := shortLabel("side #1"); got != "side #1" {
		t.Errorf("got %q", got)
	}
}
