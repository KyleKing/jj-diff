package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/seen"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/reads"
	"github.com/kyleking/jj-diff/tests/integration"
)

// Marks jojo saves for a change, read from its main workspace, show in jj-diff started in a
// subdirectory of a second workspace of the same repository: one directory, one scope, one id per
// hunk. The state directory is the test's own, so the runner's marks are never read.
func TestReads_JojoMarksShowInAnotherWorkspace(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	repo := integration.NewTestRepo(t)
	repo.WriteFile("src/a.go", "package a\n\nfunc A() {}\n")
	repo.WriteFile("b.txt", "one\n")
	repo.Commit("base")
	repo.WriteFile("src/a.go", "package a\n\n// A is a.\nfunc A() {}\n")
	repo.WriteFile("b.txt", "two\n")

	// jojo files marks under its log template's change id, written out here rather than through
	// jj.ScopeTemplate so the two cannot drift together.
	change := strings.TrimSpace(repo.MustRun("log", "-r", "@", "--no-graph", "-T", "change_id.shortest(8)"))
	if len(change) < 8 || !strings.HasPrefix(repo.GetChangeID("@"), change) {
		t.Fatalf("jojo's scope %q is not an eight-character prefix of the change id", change)
	}

	gitDiff := repo.GetDiff("@")
	live := seen.Parse([]byte(gitDiff))

	if len(live) != 2 {
		t.Fatalf("the fixture has %d hunks, want 2", len(live))
	}

	// jojo: the store at its repository's root, a mark on the first hunk.
	jojo, err := reads.Open(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := jojo.Mark(change, true, live, live[0].ID); err != nil {
		t.Fatal(err)
	}

	second := filepath.Join(t.TempDir(), "second")
	repo.MustRun("workspace", "add", "--name", "second", "-r", "@", second)

	if err := os.MkdirAll(filepath.Join(second, "src"), 0o750); err != nil {
		t.Fatal(err)
	}

	store, err := reads.Open(filepath.Join(second, "src"))
	if err != nil {
		t.Fatal(err)
	}

	if store.Dir() != jojo.Dir() {
		t.Fatalf("the second workspace opened %s, the first %s", store.Dir(), jojo.Dir())
	}

	client := jj.NewClient(repo.Dir)

	scope, err := client.ScopeID("@")
	if err != nil || scope != change {
		t.Fatalf("scope %q (%v), want the change id %q", scope, err, change)
	}

	text, err := client.Diff(change)
	if err != nil {
		t.Fatal(err)
	}

	changes := diff.Parse(text)
	marks := reads.NewMarks(store)
	marks.SetHunks(reads.Map(text, changes))
	marks.SetScope(scope)

	if got := marks.Unread(); got != 1 {
		t.Errorf("unread %d, want 1", got)
	}

	for f := range changes {
		want := live[0].Path == changes[f].Path
		if got := marks.Read(f, 0); got != want {
			t.Errorf("%s: read %t, want %t", changes[f].Path, got, want)
		}
	}
}
