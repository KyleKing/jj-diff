package integration_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/tests/integration"
)

func TestMain(m *testing.M) {
	code := m.Run()

	integration.Cleanup()
	os.Exit(code)
}

// writer is a client that has jj run the freshly built jj-diff as its diff editor.
func writer(t *testing.T, repo *integration.TestRepo) *jj.Client {
	t.Helper()

	return jj.NewClient(repo.Dir, jj.WithEditor(integration.Editor(t)))
}

// twoHunks is a file whose change from base to edited is two hunks far enough apart that git keeps
// them separate: line 2 and line 19.
const (
	base   = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\nl16\nl17\nl18\nl19\nl20\n"
	edited = "l1\nTWO\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\nl16\nl17\nl18\nNINETEEN\nl20\n"
	first  = "l1\nTWO\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\nl16\nl17\nl18\nl19\nl20\n"
	second = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\nl16\nl17\nl18\nNINETEEN\nl20\n"
)

// firstHunk picks hunk 0 of f.txt.
var firstHunk = integration.Pick{Hunks: map[string][]int{"f.txt": {0}}} //nolint:gochecknoglobals // a fixture

// move runs MoveChanges and asserts it was exactly one operation.
func move(t *testing.T, repo *integration.TestRepo, pick integration.Pick, source, dest string) jj.Outcome {
	t.Helper()

	patch := repo.Patch(source, pick)
	before := repo.OpID()

	outcome, err := writer(t, repo).MoveChanges(patch, source, dest, jj.MoveOptions{})
	if err != nil {
		t.Fatalf("MoveChanges(%s -> %s) failed: %v", source, dest, err)
	}

	if n := repo.OpsSince(before); n != 1 {
		t.Errorf("the move took %d operations, want 1:\n%s", n, repo.MustRun("op", "log", "-n", "4"))
	}

	return outcome
}

func TestMoveChanges_ToAnAncestor(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)

	move(t, repo, firstHunk, "@", "@-")

	if got := repo.Show("@-", "f.txt"); got != first {
		t.Errorf("@- holds\n%s\nwant only the first hunk moved in", got)
	}

	repo.AssertDiffNotContains("@", "+TWO")
	repo.AssertDiffContains("@", "+NINETEEN")
	repo.AssertFileContent("f.txt", edited)
}

func TestMoveChanges_ToADescendant(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)
	repo.Commit("source")

	move(t, repo, firstHunk, "@-", "@")

	repo.AssertDiffNotContains("@-", "+TWO")
	repo.AssertDiffContains("@-", "+NINETEEN")
	repo.AssertDiffContains("@", "+TWO")
	repo.AssertFileContent("f.txt", edited)
}

// A sibling is where the scratch-workspace move copied instead of moving: the hunk landed in the
// sibling and stayed in the source.
func TestMoveChanges_ToASiblingMovesRatherThanCopies(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)
	repo.MustRun("describe", "-m", "source")
	source := repo.GetChangeID("@")
	repo.MustRun("new", "@-", "-m", "sibling")
	sibling := repo.GetChangeID("@")
	repo.MustRun("new", "root()", "-m", "elsewhere")

	repo.WriteFile("other.txt", "untouched\n")
	repo.OpID() // snapshot, so the commit read next is the one the move must leave alone

	wc := repo.CommitID("@")

	move(t, repo, firstHunk, source, sibling)

	repo.AssertDiffNotContains(source, "+TWO")
	repo.AssertDiffContains(source, "+NINETEEN")

	if got := repo.Show(sibling, "f.txt"); got != first {
		t.Errorf("the sibling holds\n%s\nwant the first hunk", got)
	}

	if repo.CommitID("@") != wc || repo.ReadFile("other.txt") != "untouched\n" {
		t.Error("the working copy changed although the move did not involve it")
	}
}

// A destination without the file takes jj's conflict, where the scratch-workspace move died on a
// fatal screen.
func TestMoveChanges_IntoADestinationLackingTheFile(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)
	repo.WriteFile("new.txt", "fresh\n")
	repo.MustRun("describe", "-m", "source")
	source := repo.GetChangeID("@")
	repo.MustRun("new", "root()", "-m", "bare")
	bare := repo.GetChangeID("@")

	added := move(t, repo, integration.Pick{Hunks: map[string][]int{"new.txt": {0}}}, source, bare)
	if added.Conflicts != "" {
		t.Errorf("moving a whole new file reported conflicts: %s", added.Conflicts)
	}

	if got := repo.Show(bare, "new.txt"); got != "fresh\n" {
		t.Errorf("new.txt in the destination = %q", got)
	}

	repo.AssertDiffNotContains(source, "new.txt")

	edit := move(t, repo, firstHunk, source, bare)
	if !strings.Contains(edit.Conflicts, "conflict") {
		t.Errorf("an edit to a file the destination lacks reported no conflict: %+v", edit)
	}

	repo.AssertDiffNotContains(source, "+TWO")
}

func TestMoveChanges_PartialLines(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", "a\nb\nc\nd\n")
	repo.Commit("base")
	repo.WriteFile("f.txt", "a\nB\nC\nd\n")

	// One hunk: a, -b, -c, +B, +C, d. Lines 2 and 4 are -c and +C.
	hunk := diff.Parse(repo.GetDiff("@"))[0].Hunks[0]
	if hunk.Lines[2].Content != "c" || hunk.Lines[4].Content != "C" {
		t.Fatalf("unexpected hunk shape: %+v", hunk.Lines)
	}

	move(t, repo, integration.Pick{Lines: map[string]map[int][]int{"f.txt": {0: {2, 4}}}}, "@", "@-")

	if got := repo.Show("@-", "f.txt"); got != "a\nb\nC\nd\n" {
		t.Errorf("@- holds %q, want only c->C moved", got)
	}

	if got := repo.Show("@", "f.txt"); got != "a\nB\nC\nd\n" {
		t.Errorf("@ holds %q, want the file as edited", got)
	}

	repo.AssertDiffContains("@", "+B")
	repo.AssertDiffNotContains("@", "+C")
}

func TestMoveChanges_OneUndoRestoresEverything(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)
	repo.MustRun("describe", "-m", "source")
	repo.MustRun("new", "@-", "-m", "sibling")
	sibling := repo.GetChangeID("@")
	repo.MustRun("edit", "@-+ & description(substring:source)")

	graph := func() string {
		return repo.MustRun("log", "--ignore-working-copy", "-r", "all()", "--no-graph", "-T", `commit_id ++ "\n"`)
	}

	before := graph()

	move(t, repo, firstHunk, "@", sibling)
	repo.MustRun("undo")

	if after := graph(); after != before {
		t.Errorf("one undo left the graph at\n%s\nwant\n%s", after, before)
	}

	if ws := repo.MustRun("workspace", "list"); strings.Count(strings.TrimSpace(ws), "\n") != 0 {
		t.Errorf("undo brought back a workspace:\n%s", ws)
	}

	repo.AssertFileContent("f.txt", edited)
}

func TestMoveChanges_FailureWritesNothing(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)

	before := repo.OpID()
	wc := repo.CommitID("@")

	invalid := "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,1 +1,1 @@\n-nothing like this\n+x\n"
	if _, err := writer(t, repo).MoveChanges(invalid, "@", "@-", jj.MoveOptions{}); err == nil {
		t.Fatal("a patch that does not apply moved")
	}

	if n := repo.OpsSince(before); n != 0 {
		t.Errorf("a failed move left %d operations", n)
	}

	if repo.CommitID("@") != wc {
		t.Error("a failed move rewrote the working copy")
	}

	repo.AssertFileContent("f.txt", edited)
}

// Moving everything out of a described source into a described destination makes jj want an editor
// to combine the two messages. That has to fail cleanly rather than wait on a terminal, and -u is the
// way through.
func TestMoveChanges_EmptyingADescribedSourceNeedsAMessageChoice(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", second)
	repo.Commit("source")

	all := integration.Pick{Hunks: map[string][]int{"f.txt": {0}}}
	patch := repo.Patch("@-", all)
	before := repo.OpID()
	client := writer(t, repo)

	if _, err := client.MoveChanges(patch, "@-", "@--", jj.MoveOptions{}); err == nil {
		t.Fatal("combining two descriptions did not fail without an editor")
	}

	if n := repo.OpsSince(before); n != 0 {
		t.Errorf("the refused move left %d operations", n)
	}

	if _, err := client.MoveChanges(patch, "@-", "@--", jj.MoveOptions{UseDestinationMessage: true}); err != nil {
		t.Fatalf("with -u: %v", err)
	}

	if got := repo.Show("@-", "f.txt"); got != second {
		t.Errorf("the destination holds %q", got)
	}

	if desc := repo.MustRun("log", "-r", "@-", "--no-graph", "-T", "description"); strings.TrimSpace(desc) != "base" {
		t.Errorf("the destination's description is %q, want base", desc)
	}
}

func TestRestore_DiscardsOnlyTheSelection(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)

	patch := repo.Patch("@", firstHunk)
	before := repo.OpID()

	if _, err := writer(t, repo).Restore(patch, "@"); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if n := repo.OpsSince(before); n != 1 {
		t.Errorf("the restore took %d operations, want 1", n)
	}

	repo.AssertFileContent("f.txt", second)
}

// Two groups whose hunks sit within a few lines of each other: the second split sees the first's
// commit as its left side, which a patch written against the original parent would not apply to
// without the cumulative Through.
func TestSplit_ChainOfAdjacentGroups(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", "a\nb\nc\nd\ne\n")
	repo.Commit("base")
	repo.WriteFile("f.txt", "a\nB\nc\nD\ne\nF\n")
	repo.MustRun("describe", "-m", "source")
	source := repo.GetChangeID("@")

	// One hunk spanning every change: a -b +B c -d +D e +F. Lines: 0 a, 1 -b, 2 +B, 3 c, 4 -d,
	// 5 +D, 6 e, 7 +F.
	one := integration.Pick{Lines: map[string]map[int][]int{"f.txt": {0: {1, 2}}}}
	two := integration.Pick{Lines: map[string]map[int][]int{"f.txt": {0: {1, 2, 4, 5}}}}

	before := repo.OpID()

	_, err := writer(t, repo).Split(source, []jj.SplitGroup{
		{Through: repo.Patch(source, one), Description: "one"},
		{Through: repo.Patch(source, two), Description: "two"},
	}, false)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if n := repo.OpsSince(before); n != 2 {
		t.Errorf("the split took %d operations, want one per group", n)
	}

	chain := repo.MustRun("log", "-r", "::@ ~ root()", "--no-graph", "-T", `description.first_line() ++ "\n"`)
	if got := strings.Fields(chain); strings.Join(got, " ") != "source two one base" {
		t.Errorf("chain = %q, want source two one base", got)
	}

	for rev, want := range map[string]string{
		"description(substring:one)": "a\nB\nc\nd\ne\n",
		"description(substring:two)": "a\nB\nc\nD\ne\n",
		source:                       "a\nB\nc\nD\ne\nF\n",
	} {
		if got := repo.Show(rev, "f.txt"); got != want {
			t.Errorf("%s holds %q, want %q", rev, got, want)
		}
	}

	if repo.GetChangeID("@") != source {
		t.Error("the working copy left the source")
	}
}

func TestSplit_ParallelGroupsAreSiblings(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)
	repo.MustRun("describe", "-m", "source")

	if _, err := writer(t, repo).Split("@", []jj.SplitGroup{
		{Through: repo.Patch("@", firstHunk), Description: "side"},
	}, true); err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if got := repo.Show("description(substring:side)", "f.txt"); got != first {
		t.Errorf("the sibling holds %q", got)
	}

	parent := repo.MustRun("log", "-r", "description(substring:side)-", "--no-graph", "-T", "description")
	parents := strings.TrimSpace(parent)
	if parents != "base" {
		t.Errorf("the sibling sits on %q, want base", parents)
	}

	repo.AssertDiffContains("@", "+NINETEEN")
	repo.AssertDiffNotContains("@", "+TWO")
}

func TestSplit_FailedGroupRestoresTheWholeSplit(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", base)
	repo.Commit("base")
	repo.WriteFile("f.txt", edited)

	invalid := "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,1 +1,1 @@\n-nothing like this\n+x\n"

	_, err := writer(t, repo).Split("@", []jj.SplitGroup{
		{Through: repo.Patch("@", firstHunk), Description: "fine"},
		{Through: invalid, Description: "broken"},
	}, false)
	if err == nil {
		t.Fatal("a split with a broken group succeeded")
	}

	if log := repo.MustRun("log", "-r", "all()"); strings.Contains(log, "fine") {
		t.Errorf("the first group survived the failed split:\n%s", log)
	}

	repo.AssertFileContent("f.txt", edited)
	repo.AssertDiffContains("@", "+TWO")
}

func TestGetRevisions_ParsesRealLogOutput(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	repo.WriteFile("file1.txt", "line 1\n")
	repo.Commit("feat: first")
	repo.WriteFile("file2.txt", "line 2\n")
	repo.Commit("feat: second")

	client := jj.NewClient(repo.Dir)

	revisions, err := client.GetRevisions(jj.Destinations("@"), 0)
	if err != nil {
		t.Fatalf("GetRevisions failed: %v", err)
	}

	if len(revisions) < 2 {
		t.Fatalf("Expected at least 2 revisions, got %d: %+v", len(revisions), revisions)
	}

	var found bool
	for _, rev := range revisions {
		if rev.ChangeID == "" {
			t.Errorf("Revision has empty change ID: %+v", rev)
		}

		if strings.Contains(rev.ChangeID, "\\n") || strings.Contains(rev.Description, "\\n") {
			t.Errorf("Revision carries a literal \\n escape: %+v", rev)
		}

		if rev.Description == "feat: second" {
			found = true
		}
	}

	if !found {
		t.Errorf("Expected a revision described %q, got %+v", "feat: second", revisions)
	}
}

// A revset such as @- resolves to the change ID and first description line.
func TestResolveLabel_NamesTheRevisionByChangeIDAndDescription(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	repo.WriteFile("file1.txt", "line 1\n")
	repo.Commit("feat(inbox): re-read the inbox\n\nbody that stays out of the header")

	source := diff.NewRevisionSource(jj.NewClient(repo.Dir), "@-")
	if err := source.ResolveLabel(); err != nil {
		t.Fatalf("ResolveLabel failed: %v", err)
	}

	changeID := repo.GetChangeID("@-")
	if want := changeID[:8] + " feat(inbox): re-read the inbox"; source.GetSourceLabel() != want {
		t.Errorf("GetSourceLabel() = %q, want %q", source.GetSourceLabel(), want)
	}
}

// BenchmarkStartupJJCalls is the jj work jj-diff waits on before its first frame: the revision's
// label and its diff, run at once the way main runs them.
func BenchmarkStartupJJCalls(b *testing.B) {
	if _, err := exec.LookPath("jj"); err != nil {
		b.Skip("jj is not installed")
	}

	repo := integration.NewTestRepo(b)
	repo.WriteFile("file1.txt", strings.Repeat("line\n", 500))
	repo.Commit("base")
	repo.WriteFile("file1.txt", strings.Repeat("changed\n", 500))

	client := jj.NewClient(repo.Dir)

	for b.Loop() {
		revision := diff.NewRevisionSource(client, "@")
		source := diff.Prefetch(revision)

		if err := revision.ResolveLabel(); err != nil {
			b.Fatal(err)
		}

		if _, err := source.GetDiff(); err != nil {
			b.Fatal(err)
		}
	}
}
