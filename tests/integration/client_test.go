// Package integration drives internal/jj against a real jj binary in a throwaway repository.
// Every test skips when jj is absent, because the CI runners have none.
package integration

import (
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/jj"
)

// TestMoveChanges_CoreWorkflow tests the basic MoveChanges workflow
// This is the primary integration test for the core functionality.
func TestMoveChanges_CoreWorkflow(t *testing.T) {
	repo := NewTestRepo(t)

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
	repo := NewTestRepo(t)

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
	repo := NewTestRepo(t)

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

// TestGetRevisions_ParsesRealLogOutput guards the jj log template: an escaped
// backslash there produces one unbroken line and silently yields no revisions,
// which empties the destination picker without any error surfacing.
func TestGetRevisions_ParsesRealLogOutput(t *testing.T) {
	repo := NewTestRepo(t)

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
