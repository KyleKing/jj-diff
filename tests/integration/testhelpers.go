// Package integration drives internal/jj against a real jj binary in a throwaway repository.
// Every test skips when jj is absent, because the CI runners have none.
package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

// Permissions for the throwaway repository the fixtures build. Everything lives under the test's own
// temp directory, so nothing there is readable by anyone else.
const (
	fixtureDirMode  = 0o750
	fixtureFileMode = 0o600
)

// TestRepo represents a temporary jj repository for testing.
type TestRepo struct {
	t   testing.TB
	Dir string
}

// NewTestRepo creates a new temporary jj repository, skipping the test when no
// jj binary is installed. Every test in this package drives the real CLI, so
// without one there is nothing to assert rather than something to fail.
func NewTestRepo(tb testing.TB) *TestRepo {
	tb.Helper()

	if _, err := exec.LookPath("jj"); err != nil {
		tb.Skip("jj is not installed; skipping the integration suite")
	}

	tmpDir := tb.TempDir()

	// Initialize jj repo
	//nolint:gosec // G204: fixture helper runs the literal jj binary with test arguments.
	cmd := exec.CommandContext(context.Background(), "jj", "git", "init", tmpDir)
	if err := cmd.Run(); err != nil {
		tb.Fatalf("Failed to init jj repo: %v", err)
	}

	// Set test user config
	configPath := filepath.Join(tmpDir, ".jj", "repo", "config.toml")
	configContent := `[user]
name = "Test User"
email = "test@example.com"
`
	if err := os.WriteFile(configPath, []byte(configContent), fixtureFileMode); err != nil {
		tb.Fatalf("Failed to write config: %v", err)
	}

	return &TestRepo{
		Dir: tmpDir,
		t:   tb,
	}
}

// WriteFile writes a file to the repository.
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)

	if err := os.MkdirAll(filepath.Dir(fullPath), fixtureDirMode); err != nil {
		r.t.Fatalf("Failed to create directories: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), fixtureFileMode); err != nil {
		r.t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// ReadFile reads a file from the repository.
func (r *TestRepo) ReadFile(path string) string {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)
	//nolint:gosec // G304: the path is built from the test's own temp directory.
	content, err := os.ReadFile(fullPath)
	if err != nil {
		r.t.Fatalf("Failed to read file %s: %v", path, err)
	}

	return string(content)
}

// Commit creates a new commit with current changes.
func (r *TestRepo) Commit(message string) {
	r.t.Helper()

	//nolint:gosec // G204: fixture helper runs the literal jj binary with test arguments.
	cmd := exec.CommandContext(context.Background(), "jj", "commit", "-m", message)
	cmd.Dir = r.Dir
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("Failed to commit: %v", err)
	}
}

// Run executes a jj command in the repository.
func (r *TestRepo) Run(args ...string) (string, error) {
	r.t.Helper()
	//nolint:gosec // G204: fixture helper runs the literal jj binary with test arguments.
	cmd := exec.CommandContext(context.Background(), "jj", args...)
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()

	return string(output), err
}

// MustRun executes a jj command and fails test on error.
func (r *TestRepo) MustRun(args ...string) string {
	r.t.Helper()
	output, err := r.Run(args...)
	if err != nil {
		r.t.Fatalf("Command 'jj %v' failed: %v\nOutput: %s", args, err, output)
	}

	return output
}

// GetDiff returns the diff for a revision.
func (r *TestRepo) GetDiff(revision string) string {
	r.t.Helper()
	return r.MustRun("diff", "-r", revision, "--git", "--color=never")
}

// GetChangeID returns the change ID for a revision.
func (r *TestRepo) GetChangeID(revision string) string {
	r.t.Helper()
	output := r.MustRun("log", "-r", revision, "--no-graph", "-T", "change_id")

	return strings.TrimSpace(output)
}

// AssertFileContent verifies file contents.
func (r *TestRepo) AssertFileContent(path, expected string) {
	r.t.Helper()
	actual := r.ReadFile(path)
	if actual != expected {
		r.t.Errorf("File %s content mismatch:\nExpected:\n%s\nActual:\n%s", path, expected, actual)
	}
}

// AssertDiffContains verifies diff contains expected string.
func (r *TestRepo) AssertDiffContains(revision, expected string) {
	r.t.Helper()
	got := r.GetDiff(revision)
	if !strings.Contains(got, expected) {
		r.t.Errorf(
			"Diff for %s missing expected content:\n%s\nFull diff:\n%s",
			revision,
			expected,
			got,
		)
	}
}

// AssertDiffNotContains verifies diff does not contain string.
func (r *TestRepo) AssertDiffNotContains(revision, unexpected string) {
	r.t.Helper()
	got := r.GetDiff(revision)
	if strings.Contains(got, unexpected) {
		r.t.Errorf(
			"Diff for %s contains unexpected content:\n%s\nFull diff:\n%s",
			revision,
			unexpected,
			got,
		)
	}
}

// AssertDiffEmpty verifies revision has no changes.
func (r *TestRepo) AssertDiffEmpty(revision string) {
	r.t.Helper()
	got := r.GetDiff(revision)
	if strings.TrimSpace(got) != "" {
		r.t.Errorf("Expected empty diff for %s, got:\n%s", revision, got)
	}
}

// The jj-diff binary the write paths have jj run as their diff editor, built once per test binary.
//
//nolint:gochecknoglobals // one build shared by every test, removed by Cleanup.
var editor struct {
	err  error
	dir  string
	path string
	once sync.Once
}

// Editor builds cmd/jj-diff once and answers its path, because a write runs jj, and jj runs jj-diff
// back as the diff editor, which inside go test is not the test binary.
func Editor(tb testing.TB) string {
	tb.Helper()

	editor.once.Do(func() {
		_, here, _, _ := runtime.Caller(0)

		//nolint:usetesting // shared by every test in the binary, so it outlives any one test's TempDir
		editor.dir, editor.err = os.MkdirTemp("", "jj-diff-editor-*")
		if editor.err != nil {
			return
		}

		editor.path = filepath.Join(editor.dir, "jj-diff")

		//nolint:gosec // G204: builds this module's own command into a temp directory.
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", editor.path,
			filepath.Join(filepath.Dir(here), "..", "..", "cmd", "jj-diff"))

		if out, err := cmd.CombinedOutput(); err != nil {
			editor.err = &buildError{out: string(out), err: err}
		}
	})

	if editor.err != nil {
		tb.Fatalf("building jj-diff: %v", editor.err)
	}

	return editor.path
}

type buildError struct {
	err error
	out string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.out }

// Cleanup removes the binary Editor built, for TestMain to call once the tests have run.
func Cleanup() {
	if editor.dir != "" {
		_ = os.RemoveAll(editor.dir) //nolint:errcheck // a leftover temp directory is not a test failure
	}
}

// OpID is the repository's current operation, snapshotting the working copy first so that a
// snapshot never counts as an operation a write made.
func (r *TestRepo) OpID() string {
	r.t.Helper()

	return strings.TrimSpace(r.MustRun("op", "log", "--no-graph", "--limit", "1", "-T", "id"))
}

// OpsSince counts the operations after opID.
func (r *TestRepo) OpsSince(opID string) int {
	r.t.Helper()

	out := r.MustRun("op", "log", "--ignore-working-copy", "--no-graph", "-T", `id ++ "\n"`)

	for i, id := range strings.Fields(out) {
		if id == opID {
			return i
		}
	}

	r.t.Fatalf("operation %s is not in the log", opID)

	return -1
}

// Show is a file's content in a revision, empty when the revision lacks it.
func (r *TestRepo) Show(revision, path string) string {
	r.t.Helper()

	out, err := r.Run("file", "show", "-r", revision, path)
	if err != nil {
		return ""
	}

	return out
}

// CommitID is the commit a revision names now, which changes whenever the revision is rewritten.
func (r *TestRepo) CommitID(revision string) string {
	r.t.Helper()

	return strings.TrimSpace(r.MustRun("log", "--ignore-working-copy", "-r", revision, "--no-graph", "-T", "commit_id"))
}

// Pick is a selection for diff.GeneratePatch: whole hunks by file and index, or single lines.
type Pick struct {
	Hunks map[string][]int
	Lines map[string]map[int][]int
}

// IsHunkSelected implements diff.SelectionState.
func (p Pick) IsHunkSelected(path string, hunk int) bool {
	for _, h := range p.Hunks[path] {
		if h == hunk {
			return true
		}
	}

	return false
}

// HasPartialSelection implements diff.SelectionState.
func (p Pick) HasPartialSelection(path string, hunk int) bool {
	return len(p.Lines[path][hunk]) > 0
}

// IsLineSelected implements diff.SelectionState.
func (p Pick) IsLineSelected(path string, hunk, line int) bool {
	if p.IsHunkSelected(path, hunk) {
		return true
	}

	for _, l := range p.Lines[path][hunk] {
		if l == line {
			return true
		}
	}

	return false
}

// Patch is the git patch selecting pick out of revision's diff.
func (r *TestRepo) Patch(revision string, pick Pick) string {
	r.t.Helper()

	return diff.GeneratePatch(diff.Parse(r.GetDiff(revision)), pick)
}
