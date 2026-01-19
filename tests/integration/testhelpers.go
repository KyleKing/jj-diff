package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepo represents a temporary jj repository for testing
type TestRepo struct {
	Dir     string
	t       *testing.T
	cleanup func()
}

// NewTestRepo creates a new temporary jj repository
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "jj-diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize jj repo
	cmd := exec.Command("jj", "git", "init", tmpDir)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init jj repo: %v", err)
	}

	// Set test user config
	configPath := filepath.Join(tmpDir, ".jj", "repo", "config.toml")
	configContent := `[user]
name = "Test User"
email = "test@example.com"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to write config: %v", err)
	}

	repo := &TestRepo{
		Dir: tmpDir,
		t:   t,
		cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}

	t.Cleanup(repo.cleanup)
	return repo
}

// WriteFile writes a file to the repository
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		r.t.Fatalf("Failed to create directories: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		r.t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// ReadFile reads a file from the repository
func (r *TestRepo) ReadFile(path string) string {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		r.t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

// Commit creates a new commit with current changes
func (r *TestRepo) Commit(message string) {
	r.t.Helper()

	cmd := exec.Command("jj", "commit", "-m", message)
	cmd.Dir = r.Dir
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("Failed to commit: %v", err)
	}
}

// Run executes a jj command in the repository
func (r *TestRepo) Run(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// MustRun executes a jj command and fails test on error
func (r *TestRepo) MustRun(args ...string) string {
	r.t.Helper()
	output, err := r.Run(args...)
	if err != nil {
		r.t.Fatalf("Command 'jj %v' failed: %v\nOutput: %s", args, err, output)
	}
	return output
}

// GetDiff returns the diff for a revision
func (r *TestRepo) GetDiff(revision string) string {
	r.t.Helper()
	return r.MustRun("diff", "-r", revision, "--git", "--color=never")
}

// GetChangeID returns the change ID for a revision
func (r *TestRepo) GetChangeID(revision string) string {
	r.t.Helper()
	output := r.MustRun("log", "-r", revision, "--no-graph", "-T", "change_id")
	return strings.TrimSpace(output)
}

// AssertFileContent verifies file contents
func (r *TestRepo) AssertFileContent(path, expected string) {
	r.t.Helper()
	actual := r.ReadFile(path)
	if actual != expected {
		r.t.Errorf("File %s content mismatch:\nExpected:\n%s\nActual:\n%s", path, expected, actual)
	}
}

// AssertDiffContains verifies diff contains expected string
func (r *TestRepo) AssertDiffContains(revision, expected string) {
	r.t.Helper()
	diff := r.GetDiff(revision)
	if !strings.Contains(diff, expected) {
		r.t.Errorf("Diff for %s missing expected content:\n%s\nFull diff:\n%s", revision, expected, diff)
	}
}

// AssertDiffNotContains verifies diff does not contain string
func (r *TestRepo) AssertDiffNotContains(revision, unexpected string) {
	r.t.Helper()
	diff := r.GetDiff(revision)
	if strings.Contains(diff, unexpected) {
		r.t.Errorf("Diff for %s contains unexpected content:\n%s\nFull diff:\n%s", revision, unexpected, diff)
	}
}

// AssertDiffEmpty verifies revision has no changes
func (r *TestRepo) AssertDiffEmpty(revision string) {
	r.t.Helper()
	diff := r.GetDiff(revision)
	if strings.TrimSpace(diff) != "" {
		r.t.Errorf("Expected empty diff for %s, got:\n%s", revision, diff)
	}
}
