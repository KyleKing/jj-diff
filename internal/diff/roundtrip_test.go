package diff_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

type allOrNothing struct {
	keep bool
}

func (s allOrNothing) IsHunkSelected(string, int) bool      { return s.keep }
func (allOrNothing) HasPartialSelection(string, int) bool   { return false }
func (s allOrNothing) IsLineSelected(string, int, int) bool { return s.keep }

func writeTree(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

const (
	leftMain  = "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	rightMain = "package main\n\nimport \"fmt\"\n\nfunc main() {\n" +
		"\tfmt.Println(\"hello, world\")\n\tfmt.Println(\"second line\")\n}\n"
	leftDoc  = "# Notes\n\nline one\nline two\nline three\n"
	rightDoc = "# Notes\n\nline one\nline TWO changed\nline three\nline four added\n"
)

func writeRoundTripTrees(t *testing.T, left, right string) {
	t.Helper()

	writeTree(t, left, "main.go", leftMain)
	writeTree(t, left, "NOTES.md", leftDoc)
	writeTree(t, right, "main.go", rightMain)
	writeTree(t, right, "NOTES.md", rightDoc)
	writeTree(t, right, "extra.txt", "temp\n")
}

func assertFileBytes(t *testing.T, path, want string) {
	t.Helper()

	//nolint:gosec // G304: the path is built from the test's own temp directory.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	if string(got) != want {
		t.Errorf("%s mismatch\n got: %q\nwant: %q", path, got, want)
	}
}

// TestApplySelections_RoundTrip is the contract the jj diff editor depends on:
// selecting everything must reproduce the right tree byte for byte, and
// selecting nothing must reproduce the left tree. Anything in between is a
// corrupted commit, which is why both directions are asserted on exact bytes.
func TestApplySelections_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, keep := range []bool{true, false} {
		t.Run(fmt.Sprintf("keep=%v", keep), func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			left := filepath.Join(base, "left")
			right := filepath.Join(base, "right")
			writeRoundTripTrees(t, left, right)

			patch, err := diff.CompareDirectories(left, right)
			if err != nil {
				t.Fatalf("CompareDirectories: %v", err)
			}

			applier := diff.NewApplier(left, right)
			if err := applier.ApplySelections(diff.Parse(patch), allOrNothing{keep: keep}); err != nil {
				t.Fatalf("ApplySelections: %v", err)
			}

			wantMain, wantDoc := leftMain, leftDoc
			if keep {
				wantMain, wantDoc = rightMain, rightDoc
			}

			assertFileBytes(t, filepath.Join(right, "main.go"), wantMain)
			assertFileBytes(t, filepath.Join(right, "NOTES.md"), wantDoc)

			_, err = os.Stat(filepath.Join(right, "extra.txt"))
			if keep && err != nil {
				t.Errorf("extra.txt should survive a full selection: %v", err)
			}

			if !keep && err == nil {
				t.Error("extra.txt should be removed when nothing is selected")
			}
		})
	}
}

// isPatchMetadata reports the patch lines that carry no diff content and therefore no line body to
// inspect.
func isPatchMetadata(line string) bool {
	return line == "" || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "@@") ||
		strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
		strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file")
}

// isWholeLineBody reports whether a diff line body is one of the whole lines the fixture wrote,
// rather than a fragment of one.
func isWholeLineBody(body string) bool {
	if body == "" || strings.HasPrefix(body, "u") || strings.HasPrefix(body, "ADD") {
		return true
	}

	return strings.HasPrefix(body, "package") || strings.HasPrefix(body, "import") ||
		strings.HasPrefix(body, "func") || strings.HasPrefix(body, "}") ||
		strings.HasPrefix(body, "\t")
}

// TestCompareDirectories_LineGranularity guards against the character-mode diff
// that split segments mid-line and produced fragments such as "-p" / "+fmt.P".
func TestCompareDirectories_LineGranularity(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")

	var wantLeft, wantRight strings.Builder
	for i := range 400 {
		line := fmt.Sprintf("u%d\n", i)
		wantLeft.WriteString(line)
		wantRight.WriteString(line)

		if i%50 == 0 {
			fmt.Fprintf(&wantRight, "ADD%d\n", i)
		}
	}

	writeTree(t, left, "big.txt", wantLeft.String())
	writeTree(t, right, "big.txt", wantRight.String())
	writeTree(t, left, "main.go", leftMain)
	writeTree(t, right, "main.go", rightMain)

	patch, err := diff.CompareDirectories(left, right)
	if err != nil {
		t.Fatalf("CompareDirectories: %v", err)
	}

	for _, line := range strings.Split(patch, "\n") {
		if isPatchMetadata(line) {
			continue
		}

		if !isWholeLineBody(line[1:]) {
			t.Errorf("patch line is a mid-line fragment: %q", line)
		}
	}
}
