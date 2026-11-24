//nolint:testpackage // white-box: these tests load marks through the model's own messages.
package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/seen"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/reads"
)

// readsPatch is three files as `jj diff --git` prints them: two hunks, one, and one.
const readsPatch = `diff --git a/internal/app/app.go b/internal/app/app.go
index 1111111..2222222 100644
--- a/internal/app/app.go
+++ b/internal/app/app.go
@@ -1,3 +1,4 @@ package app
 package app
+// Run starts the app.
 func Run() {}
 func stop() {}
@@ -20,3 +21,3 @@ func Serve() {
 	x := 1
-	y := 2
+	y := 3
 }
diff --git a/internal/app/config.go b/internal/app/config.go
index 3333333..4444444 100644
--- a/internal/app/config.go
+++ b/internal/app/config.go
@@ -5,3 +5,3 @@ type Config struct {
 	Name string
-	Port int
+	Port uint16
 }
diff --git a/README.md b/README.md
new file mode 100644
index 0000000..5555555
--- /dev/null
+++ b/README.md
@@ -0,0 +1,2 @@
+# app
+It runs.
`

// readsScope is the change id the fixture's marks are filed under.
const readsScope = "kxyzkxyzkxyzkxyzkxyzkxyzkxyzkxyz"

// markReads saves marks for the patch's hunks at indexes read, as jojo would, into dir.
func markReads(t *testing.T, dir string, read ...int) {
	t.Helper()

	live := seen.Parse([]byte(readsPatch))

	ids := make([]seen.ID, 0, len(read))
	for _, i := range read {
		ids = append(ids, live[i].ID)
	}

	if err := seen.Open(dir).Mark(readsScope, true, live, ids...); err != nil {
		t.Fatal(err)
	}
}

// readsModel is a revision in mode whose first file jojo has read whole, loaded through the same
// two messages a real session gets. The store lives in a temp dir, so nothing reads the runner's
// own state.
func readsModel(t *testing.T, mode OperatingMode) (*Model, string) {
	t.Helper()

	return readsModelWith(t, mode, 0, 1)
}

// readsModelWith is readsModel with the patch's hunks at indexes read marked read instead.
func readsModelWith(t *testing.T, mode OperatingMode, read ...int) (*Model, string) {
	t.Helper()

	dir := t.TempDir()
	markReads(t, dir, read...)

	m := NewTestModel(t, mode).WithReads(seen.Open(dir))
	changes := diff.Parse(readsPatch)
	m = Update(t, m, diffLoadedMsg{changes: changes, hunks: reads.Map(readsPatch, changes), named: true})
	m = Update(t, m, scopeMsg{scope: readsScope})

	return m, dir
}

func readsFiles(t *testing.T) *Model {
	t.Helper()

	m, _ := readsModel(t, ModeBrowse)

	return m
}

func readsDiff(t *testing.T) *Model {
	t.Helper()

	m, _ := readsModel(t, ModeBrowse)
	m.focusedPanel = PanelDiffView

	return m
}

// readsStepped is interactive mode after ]u, which skips the read file for the next unread hunk.
func readsStepped(t *testing.T) *Model {
	t.Helper()

	m, _ := readsModel(t, ModeInteractive)

	return Press(t, m, "]", "u")
}

// readsOnlyUnreadFiles is U pressed on the file list: the file read whole leaves it, and the cursor
// moves to the first file with something unread while the list keeps focus.
func readsOnlyUnreadFiles(t *testing.T) *Model {
	t.Helper()

	m, _ := readsModel(t, ModeBrowse)

	return Press(t, m, "U")
}

// readsOnlyUnreadDiff is U pressed on a read hunk whose file has an unread one: the cursor steps to
// the unread hunk and the read one shrinks to its header.
func readsOnlyUnreadDiff(t *testing.T) *Model {
	t.Helper()

	m, _ := readsModelWith(t, ModeBrowse, 0)
	m.focusedPanel = PanelDiffView

	return Press(t, m, "U")
}

// readsSelectedRead is -i with a read hunk selected: its header carries both marks, the read cell
// leading the section and the selection's check after the range.
func readsSelectedRead(t *testing.T) *Model {
	t.Helper()

	m, _ := readsModelWith(t, ModeInteractive, 0)
	m.focusedPanel = PanelDiffView

	return Press(t, m, "space")
}

// TestReads_Golden pins what a revision with read marks draws: the muted glyph on a file read
// whole and on its hunks' headers, and the unread counts in the heading and the Files rule.
func TestReads_Golden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build func(t *testing.T) *Model
		name  string
	}{
		{name: "browse_files", build: readsFiles},
		{name: "browse_diff", build: readsDiff},
		{name: "interactive_stepped", build: readsStepped},
		{name: "only_unread_files", build: readsOnlyUnreadFiles},
		{name: "only_unread_diff", build: readsOnlyUnreadDiff},
		{name: "interactive_selected_read", build: readsSelectedRead},
	}

	for _, c := range cases {
		for _, size := range frameSizes {
			name := fmt.Sprintf("reads_%s_%dx%d", c.name, size.w, size.h)

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := c.build(t)
				m = Update(t, m, tea.WindowSizeMsg{Width: size.w, Height: size.h})

				frameGolden(t, filepath.Join("testdata", name+".golden"), ansi.Strip(m.render())+"\n", size.w)
			})
		}
	}
}

func TestReads_UnreadMotionsStepAcrossFilesAndWrap(t *testing.T) {
	t.Parallel()

	for _, mode := range []OperatingMode{ModeBrowse, ModeInteractive} {
		t.Run(modeName(mode), func(t *testing.T) {
			t.Parallel()

			m, _ := readsModel(t, mode)

			steps := []struct {
				keys       []string
				file, hunk int
			}{
				{[]string{"]", "u"}, 1, 0},
				{[]string{"]", "u"}, 2, 0},
				{[]string{"]", "u"}, 1, 0},
				{[]string{"[", "u"}, 2, 0},
				{[]string{"n"}, 1, 0},
				{[]string{"N"}, 2, 0},
			}

			for i, s := range steps {
				m = Press(t, m, s.keys...)

				if m.selectedFile != s.file || m.selectedHunk != s.hunk || m.focusedPanel != PanelDiffView {
					t.Errorf("step %d %q: at file %d hunk %d, want %d %d on the diff", i, s.keys,
						m.selectedFile, m.selectedHunk, s.file, s.hunk)
				}
			}
		})
	}
}

// Space keeps meaning select in interactive mode: it selects, and marks nothing.
func TestReads_SpaceStillSelects(t *testing.T) {
	t.Parallel()

	m, dir := readsModel(t, ModeInteractive)
	m = Press(t, m, "]", "u", "space")

	if m.selection.IsEmpty() {
		t.Error("space did not select the hunk")
	}

	live := seen.Parse([]byte(readsPatch))
	if !seen.Open(dir).Unread(readsScope, string(live[2].ID)) {
		t.Error("space marked a hunk read; jj-diff only shows marks")
	}
}

func TestReads_EverythingReadRefusesTheMotion(t *testing.T) {
	t.Parallel()

	m, dir := readsModel(t, ModeBrowse)
	markReads(t, dir, 0, 1, 2, 3)

	m = Update(t, m, tea.FocusMsg{})
	m = Press(t, m, "]", "u")

	if !strings.Contains(m.notice, "every hunk is read") {
		t.Errorf("]u with nothing unread said %q", m.notice)
	}
}

// A mark jojo saves while jj-diff is open shows on the next message, such as the terminal coming
// back into focus.
func TestReads_FocusPicksUpMarksSavedElsewhere(t *testing.T) {
	t.Parallel()

	m, dir := readsModel(t, ModeBrowse)

	if m.marks.Read(1, 0) {
		t.Fatal("the fixture already marks config.go")
	}

	time.Sleep(10 * time.Millisecond)
	markReads(t, dir, 0, 1, 2)

	m = Update(t, m, tea.FocusMsg{})

	if !m.marks.Read(1, 0) {
		t.Error("a mark saved by another process did not show after focus")
	}

	if !m.View().ReportFocus {
		t.Error("a revision with marks does not ask the terminal for focus reports")
	}
}

// The diff editor sees two directories and no change id, so it shows no marks and binds no unread
// motion; nor does a revision without a store.
func TestReads_DiffEditorAndNoStoreShowNothing(t *testing.T) {
	t.Parallel()

	store := seen.Open(t.TempDir())

	for _, m := range []*Model{
		NewTestModel(t, ModeDiffEditor).WithReads(store),
		NewTestModel(t, ModeResolve).WithReads(store),
		NewTestModel(t, ModeBrowse).WithReads(nil),
	} {
		if m.marks.On() {
			t.Errorf("%s: marks are on", modeName(m.mode))
		}

		if _, ok := m.keymap.Binding(keys.NextAction(keys.ObjUnread)); ok {
			t.Errorf("%s: ]u is bound", modeName(m.mode))
		}

		if m.View().ReportFocus {
			t.Errorf("%s: asks for focus reports", modeName(m.mode))
		}
	}

	src := diff.NewDirectorySource(t.TempDir(), t.TempDir())

	editor, err := NewModelWithSource(src, nil, "", ModeDiffEditor, NewTestModel(t, ModeBrowse).cfg)
	if err != nil {
		t.Fatal(err)
	}

	if editor.WithReads(store).marks.On() {
		t.Error("a directory source turned marks on")
	}
}

// A read hunk that is also selected carries two marks on one header row, and they must never look
// alike: the read cell, never a check, leads the section, and the selection's ✓ follows the range.
func TestReads_ReadAndSelectedMarksDiffer(t *testing.T) {
	t.Parallel()

	if diffview.ReadGlyph == "✓" || filelist.ReadGlyph == "✓" {
		t.Fatalf("the read glyph is the selection's check")
	}

	m := readsSelectedRead(t)
	m = Update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	var header string

	for line := range strings.SplitSeq(ansi.Strip(m.render()), "\n") {
		if strings.Contains(line, "package app") && strings.Contains(line, "L1") {
			header = line
		}
	}

	if strings.Count(header, diffview.ReadGlyph) != 1 || strings.Count(header, "✓") != 1 {
		t.Errorf("the read, selected hunk's header is %q; want one %s and one ✓", header, diffview.ReadGlyph)
	}

	if strings.Index(header, diffview.ReadGlyph) > strings.Index(header, "✓") {
		t.Errorf("the read cell does not lead the header: %q", header)
	}
}

// U hides what is read and U again shows it: read hunks shrink to their header, the cursor's own
// stays whole, and files read whole leave the list and its motions.
func TestReads_OnlyUnreadHidesAndRestores(t *testing.T) {
	t.Parallel()

	m, _ := readsModel(t, ModeBrowse)
	m = Press(t, m, "U")

	if !m.onlyUnread || m.selectedFile != 1 || m.focusedPanel != PanelFileList {
		t.Fatalf("U left the cursor at file %d (only unread %t, panel %d)", m.selectedFile, m.onlyUnread,
			m.focusedPanel)
	}

	if m = Press(t, m, "k"); m.selectedFile != 1 {
		t.Errorf("k moved onto the file read whole: at %d", m.selectedFile)
	}

	if m = Press(t, m, "g", "g"); m.selectedFile != 1 {
		t.Errorf("gg moved onto the file read whole: at %d", m.selectedFile)
	}

	if m = Press(t, m, "j"); m.selectedFile != 2 {
		t.Errorf("j did not reach README.md: at %d", m.selectedFile)
	}

	m = Press(t, m, "U")

	if m.onlyUnread {
		t.Fatal("U again left only-unread on")
	}

	if m = Press(t, m, "g", "g"); m.selectedFile != 0 {
		t.Errorf("gg with every file shown went to %d", m.selectedFile)
	}
}

func TestReads_OnlyUnreadHidesReadHunksButTheCursors(t *testing.T) {
	t.Parallel()

	m, _ := readsModelWith(t, ModeBrowse, 0)
	m.focusedPanel = PanelDiffView
	m = Press(t, m, "U")

	if m.selectedFile != 0 || m.selectedHunk != 1 {
		t.Fatalf("U on a read hunk left the cursor at %d/%d, want the unread 0/1", m.selectedFile, m.selectedHunk)
	}

	if got := m.hiddenHunks(); len(got) != 2 || !got[0] || got[1] {
		t.Errorf("hidden %v, want the read hunk alone", got)
	}

	m = Press(t, m, "[", "h")

	if got := m.hiddenHunks(); got[0] {
		t.Errorf("the read hunk under the cursor is hidden: %v", got)
	}
}

func TestReads_OnlyUnreadRefusedWithNothingUnread(t *testing.T) {
	t.Parallel()

	m, dir := readsModel(t, ModeBrowse)
	markReads(t, dir, 0, 1, 2, 3)

	m = Update(t, m, tea.FocusMsg{})
	m = Press(t, m, "U")

	if m.onlyUnread || !strings.Contains(m.notice, "every hunk is read") {
		t.Errorf("U with nothing unread: only unread %t, said %q", m.onlyUnread, m.notice)
	}
}
