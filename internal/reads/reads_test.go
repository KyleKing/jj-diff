package reads_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/seen"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/reads"
)

// patch is two files, the first with two hunks, as `jj diff --git` prints them.
const patch = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,4 @@ package a
 package a
+// one
 func A() {}
 func B() {}
@@ -20,3 +21,3 @@ func C() {
 	x := 1
-	y := 2
+	y := 3
 }
diff --git a/b.go b/b.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/b.go
@@ -0,0 +1,2 @@
+package b
+func D() {}
`

// jojoDir is the directory jojo opens for the workspace at root, written out the way
// cmd/jojo/main.go derives it (jj.RepoStore, then StateDir and RepoKey) rather than through the
// function under test, so the two tools cannot drift apart unnoticed.
func jojoDir(t *testing.T, root string) string {
	t.Helper()

	repo := filepath.Join(root, ".jj", "repo")

	info, err := os.Stat(repo)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsDir() {
		data, err := os.ReadFile(repo) //nolint:gosec // G304: a fixture the test wrote.
		if err != nil {
			t.Fatal(err)
		}

		repo = strings.TrimSpace(string(data))
		if !filepath.IsAbs(repo) {
			repo = filepath.Join(root, ".jj", repo)
		}
	}

	resolved, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := seen.StateDir("jojo", "seen", seen.RepoKey(resolved))
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

func mkdir(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
}

// jj-diff started anywhere in any workspace of a repository opens the directory jojo opens at that
// repository's root, or the two would show each other nothing.
func TestOpen_AgreesWithJojo(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	base := t.TempDir()
	main := filepath.Join(base, "main")
	mkdir(t, filepath.Join(main, ".jj", "repo"))
	mkdir(t, filepath.Join(main, "internal", "deep"))

	second := filepath.Join(base, "second")
	mkdir(t, filepath.Join(second, ".jj"))

	if err := os.WriteFile(filepath.Join(second, ".jj", "repo"), []byte("../../main/.jj/repo"), 0o600); err != nil {
		t.Fatal(err)
	}

	want := jojoDir(t, main)

	if got := jojoDir(t, second); got != want {
		t.Fatalf("the fixture's workspaces disagree under jojo's own rule: %s and %s", got, want)
	}

	for _, dir := range []string{main, filepath.Join(main, "internal", "deep"), second} {
		store, err := reads.Open(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}

		if store.Dir() != want {
			t.Errorf("%s: opened %s, jojo opens %s", dir, store.Dir(), want)
		}
	}

	if _, err := reads.Open(t.TempDir()); err == nil {
		t.Error("a directory in no jj repository opened a store")
	}
}

// fixture is a store with marks jojo would have written for the patch's first two hunks, scoped
// by change.
func fixture(t *testing.T, change string, read ...int) (*seen.Store, []diff.FileChange) {
	t.Helper()

	store := seen.Open(t.TempDir())
	live := seen.Parse([]byte(patch))

	ids := make([]seen.ID, 0, len(read))
	for _, i := range read {
		ids = append(ids, live[i].ID)
	}

	if err := store.Mark(change, true, live, ids...); err != nil {
		t.Fatal(err)
	}

	return store, diff.Parse(patch)
}

func TestMarks_AnswersWhatJojoMarked(t *testing.T) {
	t.Parallel()

	writer, changes := fixture(t, "kxyz", 0, 1)

	k := reads.NewMarks(seen.Open(writer.Dir()))
	k.SetHunks(reads.Map(patch, changes))

	if k.Ready() {
		t.Fatal("marks answered before a scope was known")
	}

	k.SetScope("kxyz")

	cases := []struct {
		file, hunk int
		read       bool
	}{{0, 0, true}, {0, 1, true}, {1, 0, false}, {2, 0, false}, {0, 5, false}}

	for _, c := range cases {
		if got := k.Read(c.file, c.hunk); got != c.read {
			t.Errorf("file %d hunk %d: read %t, want %t", c.file, c.hunk, got, c.read)
		}
	}

	if !k.FileRead(0) || k.FileRead(1) {
		t.Errorf("file reads: a.go %t, b.go %t", k.FileRead(0), k.FileRead(1))
	}

	if got := k.Unread(); got != 1 {
		t.Errorf("unread %d, want 1", got)
	}

	k.SetScope("other")

	if got := k.Unread(); got != 3 {
		t.Errorf("another change's scope: unread %d, want every hunk (3)", got)
	}
}

// A mark jojo saves while jj-diff is open shows on jj-diff's next look, found by the file's stamp
// rather than by rereading it every time.
func TestMarks_RefreshPicksUpAnotherProcess(t *testing.T) {
	t.Parallel()

	writer, changes := fixture(t, "kxyz")

	k := reads.NewMarks(seen.Open(writer.Dir()))
	k.SetHunks(reads.Map(patch, changes))
	k.SetScope("kxyz")

	if k.Refresh() {
		t.Error("an unchanged file was reread")
	}

	live := seen.Parse([]byte(patch))

	// A coarse filesystem clock can stamp two saves alike; the size differs here regardless.
	time.Sleep(10 * time.Millisecond)

	if err := writer.Mark("kxyz", true, live, live[2].ID); err != nil {
		t.Fatal(err)
	}

	if !k.Refresh() {
		t.Fatal("a save by another process was not noticed")
	}

	if !k.Read(1, 0) {
		t.Error("the hunk another process marked reads unread")
	}
}

// A file the two parsers disagree on draws no marks rather than another hunk's.
func TestMap_LeavesUnpairedFilesUnnamed(t *testing.T) {
	t.Parallel()

	changes := diff.Parse(patch)
	changes[0].Hunks = changes[0].Hunks[:1]

	h := reads.Map(patch, changes)

	if _, ok := h.ID(0, 0); ok {
		t.Error("a file whose hunk count differs was named")
	}

	if _, ok := h.ID(1, 0); !ok {
		t.Error("an agreeing file was left unnamed")
	}
}

// Without a store nothing is read and nothing is ready, which is the diff editor's case.
func TestMarks_NilShowsNothing(t *testing.T) {
	t.Parallel()

	var k *reads.Marks

	k.SetScope("x")
	k.SetHunks(reads.Map(patch, diff.Parse(patch)))

	if k.On() || k.Ready() || k.Read(0, 0) || k.Unread() != 0 || k.Refresh() {
		t.Error("a nil Marks answered something")
	}
}
