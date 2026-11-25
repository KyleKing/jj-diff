package integration_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/model"
	"github.com/kyleking/jj-diff/tests/integration"
)

// interactive is jj-diff -i over rev in a real repository, writing through the freshly built
// binary, with its first diff loaded and the diff pane focused on the first hunk.
func interactive(t *testing.T, repo *integration.TestRepo, rev string) *model.Model {
	t.Helper()

	client := writer(t, repo)

	built, err := model.NewModelWithSource(diff.NewRevisionSource(client, rev), client, "", model.ModeInteractive,
		config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	return press(t, pump(t, &built, built.Init()), "tab")
}

// footer is the status bar's row as drawn, without its colors.
func footer(m *model.Model) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")

	return lines[len(lines)-1]
}

// twoHunkWorkingCopy is a repository whose working copy changes f.txt in two hunks.
func twoHunkWorkingCopy(t *testing.T) *integration.TestRepo {
	t.Helper()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)

	return repo
}

// After R discards a hunk, u restores the operation before the discard, the footer having offered
// exactly that, and ctrl+r puts the discard back: u means jj undo for the session's own writes, as
// it does in jojo, rather than "nothing to undo".
func TestUndo_AfterDiscardRestoresTheLines(t *testing.T) {
	t.Parallel()

	repo := twoHunkWorkingCopy(t)
	before := repo.OpID()

	m := press(t, interactive(t, repo, "@"), "space", "R", "y")
	repo.AssertFileContent("f.txt", second)

	if got := footer(m); !strings.Contains(got, "u undoes it") {
		t.Errorf("the footer after the discard does not offer u: %q", got)
	}

	m = press(t, m, "u")
	repo.AssertFileContent("f.txt", edited)

	if got := footer(m); !strings.Contains(got, "undid: discarded 1 hunk") {
		t.Errorf("the footer after u does not say what it undid: %q", got)
	}

	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "TWO") || !strings.Contains(got, "NINETEEN") {
		t.Errorf("after u the diff does not show both hunks again:\n%s", got)
	}

	m = press(t, m, "ctrl+r")
	repo.AssertFileContent("f.txt", second)

	press(t, m, "u")
	repo.AssertFileContent("f.txt", edited)

	// discard, undo, redo, undo: four operations, each one jj command.
	if n := repo.OpsSince(before); n != 4 {
		t.Errorf("discard, u, ctrl+r, u made %d operations, want 4:\n%s", n, repo.MustRun("op", "log", "-n", "6"))
	}
}

// An edit to the working copy after the discard is an operation of its own (jj snapshots it), so
// restoring the operation before the discard would throw that edit away. So u refuses, and says so.
func TestUndo_RefusesWhenTheWorkingCopyMovedSince(t *testing.T) {
	t.Parallel()

	repo := twoHunkWorkingCopy(t)

	m := press(t, interactive(t, repo, "@"), "space", "R", "y")

	const later = "written after the discard\n"

	repo.WriteFile("g.txt", later)

	m = press(t, m, "u")

	repo.AssertFileContent("f.txt", second)
	repo.AssertFileContent("g.txt", later)

	if got := footer(m); !strings.Contains(got, "can't undo the discard") {
		t.Errorf("the footer does not refuse the undo: %q", got)
	}

	// The refusal is final for this session's history: every record is stale.
	if got := footer(press(t, m, "u")); !strings.Contains(got, "nothing to undo") {
		t.Errorf("a second u does not say nothing is left: %q", got)
	}
}

// u after a split takes back every group's operation at once, since each group is its own jj split.
func TestUndo_AfterSplitRestoresTheSource(t *testing.T) {
	t.Parallel()

	repo := twoHunkWorkingCopy(t)
	repo.Commit("two hunks")

	source := repo.GetChangeID("@-")
	m := interactive(t, repo, source)

	// S opens the columns with the hunk at the cursor in column 2; enter applies.
	m = press(t, m, "S", "enter")
	if n := strings.Count(repo.MustRun("log", "-r", "::@ ~ root()", "--no-graph", "-T", `"x\n"`), "x"); n != 4 {
		t.Fatalf("after the split the stack has %d commits, want 4 (base, group, source, @)", n)
	}

	press(t, m, "u")

	if n := strings.Count(repo.MustRun("log", "-r", "::@ ~ root()", "--no-graph", "-T", `"x\n"`), "x"); n != 3 {
		t.Errorf("after u the stack has %d commits, want the 3 before the split", n)
	}

	if got := repo.Show(source, "f.txt"); got != edited {
		t.Errorf("after u the source holds %q, want both hunks back", got)
	}
}

// R asks before it throws lines away, naming how many, and any key but y keeps them: the key is spent
// on the answer rather than replayed, so a stray j neither discards nor moves.
func TestDiscard_AsksAndAnyKeyButYKeeps(t *testing.T) {
	t.Parallel()

	repo := twoHunkWorkingCopy(t)
	before := repo.OpID()

	m := press(t, interactive(t, repo, "@"), "space", "R")

	if got := footer(m); !strings.Contains(got, "discard 2 lines from 1 hunk?") ||
		!strings.Contains(got, "y discard") || !strings.Contains(got, "n keep") {
		t.Errorf("R does not ask with the count and the answers: %q", got)
	}

	for _, key := range []string{"j", "n", "esc", "R"} {
		m = press(t, m, key)
		if got := footer(m); !strings.Contains(got, "kept: nothing was discarded") {
			t.Errorf("%s after R did not keep the lines: %q", key, got)
		}

		// Asked again for the next key in the table.
		m = press(t, m, "R")
	}

	press(t, m, "esc")

	if n := repo.OpsSince(before); n != 0 {
		t.Errorf("answering anything but y wrote %d operations", n)
	}

	repo.AssertFileContent("f.txt", edited)
}

// The repeat key after a discard asks again, never discarding on one key.
func TestDiscard_RepeatAsksAgain(t *testing.T) {
	t.Parallel()

	repo := twoHunkWorkingCopy(t)

	m := press(t, interactive(t, repo, "@"), "R", "y")
	repo.AssertFileContent("f.txt", second)

	m = press(t, m, ".")
	if got := footer(m); !strings.Contains(got, "discard 2 lines from 1 hunk?") {
		t.Errorf(". after a discard does not ask: %q", got)
	}

	repo.AssertFileContent("f.txt", second)
	press(t, m, "y")
	repo.AssertFileContent("f.txt", base)
}

// In the file list, space selects the whole file and R discards all of it; u brings it back.
func TestDiscard_FromTheFileListTakesTheWholeFile(t *testing.T) {
	t.Parallel()

	repo := twoHunkWorkingCopy(t)

	// tab back to the list from the diff pane interactive opens on.
	m := press(t, interactive(t, repo, "@"), "tab", "space", "R")
	if got := footer(m); !strings.Contains(got, "discard 4 lines from 2 hunks?") {
		t.Fatalf("R on a file selected from the list does not ask about both hunks: %q", got)
	}

	m = press(t, m, "y")
	repo.AssertFileContent("f.txt", base)

	press(t, m, "u")
	repo.AssertFileContent("f.txt", edited)
}
