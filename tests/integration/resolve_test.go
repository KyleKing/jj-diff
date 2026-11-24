package integration_test

import (
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/conflict"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/tests/integration"
)

// conflicted builds a merge of sides off one base, each side rewriting line 2 and line 7 of f.txt
// (and of d/e/g.txt, to have a second, nested file), and answers the merge's change ID.
func conflicted(t *testing.T, sides int) (*integration.TestRepo, string) {
	t.Helper()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", "a\nb\nc\nd\ne\nf\ng\nh\n")
	repo.WriteFile("d/e/g.txt", "x\n")
	repo.Commit("base")
	repo.MustRun("bookmark", "create", "base", "-r", "@-")

	parents := make([]string, 0, sides)

	for i := 1; i <= sides; i++ {
		n := string(rune('0' + i))
		repo.MustRun("new", "base", "-m", "side"+n)
		repo.WriteFile("f.txt", "a\nB"+n+"\nc\nd\ne\nf\nG"+n+"\nh\n")
		repo.WriteFile("d/e/g.txt", "x"+n+"\n")
		repo.MustRun("bookmark", "create", "s"+n, "-r", "@")
		parents = append(parents, "s"+n)
	}

	repo.MustRun(append([]string{"new", "-m", "merge"}, parents...)...)
	rev := repo.GetChangeID("@")

	return repo, rev
}

func conflictsOf(t *testing.T, repo *integration.TestRepo, rev string) map[string]jj.ConflictedFile {
	t.Helper()

	files, err := writer(t, repo).Conflicts(rev)
	if err != nil {
		t.Fatalf("Conflicts: %v", err)
	}

	out := map[string]jj.ConflictedFile{}
	for _, f := range files {
		out[f.Path] = f
	}

	return out
}

// resolve resolves rev with picks per path and asserts it was one operation.
func resolve(t *testing.T, repo *integration.TestRepo, rev string, picks map[string]map[int]conflict.Pick) {
	t.Helper()

	var res []jj.Resolution

	for path, f := range conflictsOf(t, repo, rev) {
		if p, ok := picks[path]; ok {
			resolved := conflict.Parse(f.Text).Resolve(p)
			res = append(res, jj.Resolution{Path: path, Conflicted: f.Text, Resolved: resolved})
		}
	}

	before := repo.OpID()

	if _, err := writer(t, repo).Resolve(rev, res); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if n := repo.OpsSince(before); n != 1 {
		t.Errorf("the resolve took %d operations, want 1:\n%s", n, repo.MustRun("op", "log", "-n", "4"))
	}
}

func side(n int) conflict.Pick { return conflict.Pick{Kind: conflict.TakeSide, Side: n} }

func TestConflicts_ListsFilesWithLabeledSides(t *testing.T) {
	t.Parallel()

	repo, rev := conflicted(t, 2)

	files := conflictsOf(t, repo, rev)
	if len(files) != 2 {
		t.Fatalf("got %d conflicted files, want 2: %v", len(files), files)
	}

	f, ok := files["f.txt"]
	if !ok || f.Kind != "2-sided conflict" {
		t.Fatalf("f.txt = %+v", f)
	}

	regions := conflict.Parse(f.Text).Regions()
	if len(regions) != 2 {
		t.Fatalf("f.txt has %d regions, want 2:\n%s", len(regions), f.Text)
	}

	first := regions[0]
	if !strings.Contains(first.Sides[0].Label, `"side1"`) || !strings.Contains(first.Bases[0].Label, `"base"`) {
		t.Errorf("labels do not name the revisions: %+v", regions[0])
	}
}

func TestConflicts_NoneIsEmpty(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)
	repo.WriteFile("f.txt", "a\n")

	if files, err := writer(t, repo).Conflicts("@"); err != nil || len(files) != 0 {
		t.Errorf("Conflicts = %v, %v; want none", files, err)
	}
}

// Every file the picks name lands in one operation, a fully picked file is no longer conflicted, and
// one jj undo puts both back.
func TestResolve_OneOperationThatOneUndoReverses(t *testing.T) {
	t.Parallel()

	repo, rev := conflicted(t, 2)
	before := repo.CommitID(rev)

	resolve(t, repo, rev, map[string]map[int]conflict.Pick{
		"f.txt":     {0: side(1), 1: side(2)},
		"d/e/g.txt": {0: {Kind: conflict.TakeAll}},
	})

	if got := repo.Show(rev, "f.txt"); got != "a\nB1\nc\nd\ne\nf\nG2\nh\n" {
		t.Errorf("f.txt = %q", got)
	}

	if got := repo.Show(rev, "d/e/g.txt"); got != "x1\nx2\n" {
		t.Errorf("d/e/g.txt = %q", got)
	}

	if left := conflictsOf(t, repo, rev); len(left) != 0 {
		t.Errorf("still conflicted: %v", left)
	}

	repo.MustRun("undo")

	if repo.CommitID(rev) != before || len(conflictsOf(t, repo, rev)) != 2 {
		t.Error("jj undo did not put the conflicts back")
	}
}

// A file with one region picked keeps the other's markers, which jj reads back as the conflict that
// remains rather than as text.
func TestResolve_PartialKeepsTheRemainingConflict(t *testing.T) {
	t.Parallel()

	repo, rev := conflicted(t, 2)

	resolve(t, repo, rev, map[string]map[int]conflict.Pick{"f.txt": {0: {Kind: conflict.TakeBase}}})

	left := conflictsOf(t, repo, rev)

	f, ok := left["f.txt"]
	if !ok {
		t.Fatalf("f.txt is no longer conflicted: %v", left)
	}

	regions := conflict.Parse(f.Text).Regions()
	if len(regions) != 1 || regions[0].Sides[1].Text != "G2\n" {
		t.Errorf("the remaining conflict is not region 2:\n%s", f.Text)
	}

	if !strings.HasPrefix(f.Text, "a\nb\nc\n") {
		t.Errorf("region 1 did not take the base:\n%s", f.Text)
	}

	if _, ok := left["d/e/g.txt"]; !ok {
		t.Error("a file nothing picked in was resolved")
	}
}

func TestResolve_ThreeSidesTakeTheThird(t *testing.T) {
	t.Parallel()

	repo, rev := conflicted(t, 3)

	resolve(t, repo, rev, map[string]map[int]conflict.Pick{"f.txt": {0: side(3), 1: side(3)}})

	if got := repo.Show(rev, "f.txt"); got != "a\nB3\nc\nd\ne\nf\nG3\nh\n" {
		t.Errorf("f.txt = %q", got)
	}
}

// A revision below the working copy is resolved in place, and the descendant rebases onto it.
func TestResolve_ARevisionBelowTheWorkingCopy(t *testing.T) {
	t.Parallel()

	repo, rev := conflicted(t, 2)
	repo.MustRun("new", "-m", "child")

	resolve(t, repo, rev, map[string]map[int]conflict.Pick{"f.txt": {0: side(2), 1: side(2)}})

	repo.AssertFileContent("f.txt", "a\nB2\nc\nd\ne\nf\nG2\nh\n")
}

// A resolution planned against text the file no longer holds fails, and writes nothing.
func TestResolve_StalePlanWritesNothing(t *testing.T) {
	t.Parallel()

	repo, rev := conflicted(t, 2)
	before := repo.OpID()

	_, err := writer(t, repo).Resolve(rev, []jj.Resolution{
		{Path: "f.txt", Conflicted: "not what jj holds\n", Resolved: "x\n"},
	})
	if err == nil {
		t.Fatal("a stale plan resolved")
	}

	if n := repo.OpsSince(before); n != 0 {
		t.Errorf("a failed resolve left %d operations", n)
	}
}
