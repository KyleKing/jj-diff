package integration_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/tests/integration"
)

// TestMoveChanges_CoreWorkflow tests the basic MoveChanges workflow
// This is the primary integration test for the core functionality.
func TestMoveChanges_CoreWorkflow(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	// Setup: Create initial commit
	repo.WriteFile("file1.txt", "line 1\nline 2\nline 3\n")
	repo.Commit("Initial commit")

	// Create changes in working copy
	repo.WriteFile("file1.txt", "line 1\nline 2\nNEW LINE\nline 3\n")

	// Get patch for working copy changes
	patch := repo.GetDiff("@")
	if patch == "" {
		t.Fatal("Expected non-empty diff for working copy")
	}

	// Verify patch contains our change
	if !strings.Contains(patch, "+NEW LINE") {
		t.Error("Patch missing expected addition")
	}

	client := jj.NewClient(repo.Dir)

	// Execute: Move changes from @ to @-
	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify: Changes now appear in @-
	repo.AssertDiffContains("@-", "+NEW LINE")

	// Verify: Working copy @ should be empty or nearly empty
	currentDiff := repo.GetDiff("@")
	// The diff might not be completely empty due to jj internals, but shouldn't have our change
	if strings.Contains(currentDiff, "+NEW LINE") {
		t.Error("Working copy still contains moved change")
	}
}

// TestMoveChanges_RollbackOnError tests automatic rollback when patch fails
// This verifies error handling and state restoration.
func TestMoveChanges_RollbackOnError(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	// Setup: Create initial commit
	repo.WriteFile("file1.txt", "line 1\nline 2\n")
	repo.Commit("Initial commit")

	// Get original working copy state
	originalWC := repo.GetChangeID("@")
	originalDiff := repo.GetDiff("@-")

	client := jj.NewClient(repo.Dir)

	// Execute: Try to apply invalid patch
	invalidPatch := `diff --git a/file1.txt b/file1.txt
--- a/file1.txt
+++ b/file1.txt
@@ -99,1 +99,2 @@
 this line doesn't exist
+invalid change
`

	err := client.MoveChanges(invalidPatch, "@", "@-")

	// Verify: Operation should fail
	if err == nil {
		t.Fatal("Expected MoveChanges to fail with invalid patch")
	}

	// Verify: Working copy is unchanged
	currentWC := repo.GetChangeID("@")
	if currentWC != originalWC {
		t.Errorf(
			"Working copy changed after failed operation:\nExpected: %s\nActual: %s",
			originalWC,
			currentWC,
		)
	}

	// Verify: Destination is unchanged
	currentDiff := repo.GetDiff("@-")
	if currentDiff != originalDiff {
		t.Error("Destination changed after failed operation (rollback incomplete)")
	}

	// Verify: No orphaned temporary commits
	logOutput := repo.MustRun("log", "--limit", "5", "--no-graph")
	if strings.Count(logOutput, "Initial commit") != 1 {
		t.Error("Found unexpected commits after rollback")
	}
}

// TestMoveChanges_WorkingCopyPreservation tests that operations complete successfully
// This verifies the workflow completes and repository state is consistent.
func TestMoveChanges_WorkingCopyPreservation(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	// Setup: Create simple commit
	repo.WriteFile("file1.txt", "original content\n")
	repo.Commit("Initial commit")

	// Create change in working copy
	repo.WriteFile("file1.txt", "original content\nworking copy addition\n")

	// Create patch
	patch := repo.GetDiff("@")

	client := jj.NewClient(repo.Dir)

	// Execute: Move changes to @-
	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify: Changes were moved to destination
	destDiff := repo.GetDiff("@-")
	if !strings.Contains(destDiff, "+working copy addition") {
		t.Error("Expected changes not found in destination")
	}

	// Verify: Working copy exists and is valid
	wcChangeID := repo.GetChangeID("@")
	if wcChangeID == "" {
		t.Error("Working copy has no change ID after operation")
	}

	// Verify: Repository has expected structure
	historyOutput := repo.MustRun("log", "--limit", "3", "--no-graph", "-T", "description")
	if !strings.Contains(historyOutput, "Initial commit") {
		t.Error("Repository history missing expected commit")
	}

	// Verify: No orphaned commits or corruption
	statusOutput := repo.MustRun("status")
	if strings.Contains(statusOutput, "error") || strings.Contains(statusOutput, "corrupted") {
		t.Errorf("Repository appears corrupted: %s", statusOutput)
	}
}

// TestMoveChanges_LeavesUnselectedChangesAlone is the regression test for the data-loss bug: the
// sequence used to reset the working copy to the destination, which deleted every change the patch
// did not carry. Everything the patch leaves out has to survive on disk, in the working copy, and
// under the same change ID.
func TestMoveChanges_LeavesUnselectedChangesAlone(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	repo.WriteFile("main.go", "line1\nline2\nline3\nline4\nline5\n")
	repo.WriteFile("other.txt", "a\nb\nc\n")
	repo.Commit("Initial commit")

	repo.WriteFile("main.go", "line1\nCHANGED2\nline3\nline4\nline5\n")
	repo.WriteFile("other.txt", "a\nBBB\nc\n")
	repo.WriteFile("extra.txt", "extra content\n")

	originalWC := repo.GetChangeID("@")
	commitCount := strings.Count(repo.MustRun("log", "--no-graph", "-T", "\"x\\n\"", "-r", "all()"), "x")

	// Only the main.go hunk, which is what selecting one hunk in the UI produces.
	patch := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 line1
-line2
+CHANGED2
 line3
 line4
 line5
`

	client := jj.NewClient(repo.Dir)
	if err := client.MoveChanges(patch, "@", "@-"); err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	repo.AssertFileContent("main.go", "line1\nCHANGED2\nline3\nline4\nline5\n")
	repo.AssertFileContent("other.txt", "a\nBBB\nc\n")
	repo.AssertFileContent("extra.txt", "extra content\n")

	repo.AssertDiffContains("@", "+BBB")
	repo.AssertDiffContains("@", "extra.txt")
	repo.AssertDiffNotContains("@", "+CHANGED2")
	repo.AssertDiffContains("@-", "+CHANGED2")

	if currentWC := repo.GetChangeID("@"); currentWC != originalWC {
		t.Errorf("working copy moved:\nExpected: %s\nActual:   %s", originalWC, currentWC)
	}

	after := strings.Count(repo.MustRun("log", "--no-graph", "-T", "\"x\\n\"", "-r", "all()"), "x")
	if after != commitCount {
		t.Errorf("expected %d revisions after the move, got %d (a scratch commit was left behind)", commitCount, after)
	}
}

// TestMoveChanges_ResolvesDestinationBeforeMoving guards the second bug in the same path. The
// destination used to be handed to jj as a revset and re-resolved by each command, so a relative
// revset such as @- could name a different commit part way through, and once the work moved into a
// scratch workspace it would have resolved against that workspace's own @ instead.
func TestMoveChanges_ResolvesDestinationBeforeMoving(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	repo.WriteFile("file1.txt", "line 1\n")
	repo.Commit("first")
	repo.WriteFile("file2.txt", "line 2\n")
	repo.Commit("second")

	intendedDest := repo.GetChangeID("@-")

	repo.WriteFile("file1.txt", "line 1\nADDED\n")
	patch := repo.GetDiff("@")

	client := jj.NewClient(repo.Dir)
	if err := client.MoveChanges(patch, "@", "@-"); err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	repo.AssertDiffContains(intendedDest, "+ADDED")

	if landed := repo.GetChangeID("@-"); landed != intendedDest {
		t.Errorf("destination drifted:\nExpected: %s\nActual:   %s", intendedDest, landed)
	}
}

// TestMoveChanges_CleansUpScratchWorkspaceOnFailure covers the failure path. A leaked workspace stays
// in jj workspace list forever and its directory persists, so the cleanup has to run even when the
// operation it was created for did not finish.
func TestMoveChanges_CleansUpScratchWorkspaceOnFailure(t *testing.T) {
	// The scratch directories are created under the process temp directory, so this test needs one
	// of its own to look in and cannot run alongside another test that creates them.
	scratchRoot := t.TempDir()
	t.Setenv("TMPDIR", scratchRoot)

	repo := integration.NewTestRepo(t)

	repo.WriteFile("file1.txt", "line 1\nline 2\n")
	repo.Commit("Initial commit")
	repo.WriteFile("file1.txt", "line 1\nline 2\nline 3\n")

	invalidPatch := `diff --git a/file1.txt b/file1.txt
--- a/file1.txt
+++ b/file1.txt
@@ -99,1 +99,2 @@
 this line doesn't exist
+invalid change
`

	client := jj.NewClient(repo.Dir)
	if err := client.MoveChanges(invalidPatch, "@", "@-"); err == nil {
		t.Fatal("expected MoveChanges to fail with an invalid patch")
	}

	workspaces := repo.MustRun("workspace", "list")
	if strings.Contains(workspaces, "jj-diff-scratch") {
		t.Errorf("scratch workspace leaked into jj workspace list:\n%s", workspaces)
	}

	leaked, err := filepath.Glob(filepath.Join(scratchRoot, "jj-diff-scratch-*"))
	if err != nil {
		t.Fatalf("failed to look for leaked scratch directories: %v", err)
	}

	if len(leaked) > 0 {
		t.Errorf("scratch directories left on disk: %v", leaked)
	}

	repo.AssertFileContent("file1.txt", "line 1\nline 2\nline 3\n")
}

// TestApplySplit_FailedPlanLeavesTheWorkingCopyIntact covers the split path's safety property. The
// new-commit destination cannot apply its patch today (see FINDINGS.md), so what this asserts is that
// the failure is reported and rolled back rather than taking the working copy with it.
func TestApplySplit_FailedPlanLeavesTheWorkingCopyIntact(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	repo.WriteFile("a.txt", "a1\n")
	repo.WriteFile("b.txt", "b1\n")
	repo.Commit("Initial commit")

	repo.WriteFile("a.txt", "a1\nA-ADDED\n")
	repo.WriteFile("b.txt", "b1\nB-ADDED\n")

	originalWC := repo.GetChangeID("@")

	patchA := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1,2 @@
 a1
+A-ADDED
`

	client := jj.NewClient(repo.Dir)
	plans := []jj.SplitPlan{{
		Tag:   'a',
		Patch: patchA,
		Destination: jj.SplitDestination{
			Type:        jj.SplitDestNewCommit,
			Description: "split: a only",
		},
	}}

	if err := client.ApplySplit(plans, "@"); err == nil {
		t.Fatal("expected ApplySplit to report the failed plan")
	}

	repo.AssertFileContent("a.txt", "a1\nA-ADDED\n")
	repo.AssertFileContent("b.txt", "b1\nB-ADDED\n")

	if currentWC := repo.GetChangeID("@"); currentWC != originalWC {
		t.Errorf("working copy moved:\nExpected: %s\nActual:   %s", originalWC, currentWC)
	}

	workspaces := repo.MustRun("workspace", "list")
	if strings.Contains(workspaces, "jj-diff-scratch") {
		t.Errorf("scratch workspace leaked into jj workspace list:\n%s", workspaces)
	}
}

// TestGetRevisions_ParsesRealLogOutput guards the jj log template: an escaped
// backslash there produces one unbroken line and silently yields no revisions,
// which empties the destination picker without any error surfacing.
func TestGetRevisions_ParsesRealLogOutput(t *testing.T) {
	t.Parallel()

	repo := integration.NewTestRepo(t)

	repo.WriteFile("file1.txt", "line 1\n")
	repo.Commit("feat: first")
	repo.WriteFile("file2.txt", "line 2\n")
	repo.Commit("feat: second")

	client := jj.NewClient(repo.Dir)

	revisions, err := client.GetRevisions(20)
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
