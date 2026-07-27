package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type allOrNothing struct {
	keep bool
}

func (s allOrNothing) IsHunkSelected(string, int) bool      { return s.keep }
func (s allOrNothing) HasPartialSelection(string, int) bool { return false }
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

			writeTree(t, left, "main.go", leftMain)
			writeTree(t, left, "NOTES.md", leftDoc)
			writeTree(t, right, "main.go", rightMain)
			writeTree(t, right, "NOTES.md", rightDoc)
			writeTree(t, right, "extra.txt", "temp\n")

			patch, err := CompareDirectories(left, right)
			if err != nil {
				t.Fatalf("CompareDirectories: %v", err)
			}

			applier := NewApplier(left, right)
			if err := applier.ApplySelections(Parse(patch), allOrNothing{keep: keep}); err != nil {
				t.Fatalf("ApplySelections: %v", err)
			}

			wantMain, wantDoc := leftMain, leftDoc
			if keep {
				wantMain, wantDoc = rightMain, rightDoc
			}

			for name, want := range map[string]string{"main.go": wantMain, "NOTES.md": wantDoc} {
				//nolint:gosec // G304: the path is built from the test's own temp directory.
				got, err := os.ReadFile(filepath.Join(right, name))
				if err != nil {
					t.Fatalf("reading %s: %v", name, err)
				}

				if string(got) != want {
					t.Errorf("%s mismatch\n got: %q\nwant: %q", name, got, want)
				}
			}

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

	patch, err := CompareDirectories(left, right)
	if err != nil {
		t.Fatalf("CompareDirectories: %v", err)
	}

	for _, line := range strings.Split(patch, "\n") {
		if line == "" || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "@@") ||
			strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") {
			continue
		}

		body := line[1:]
		if body == "" || strings.HasPrefix(body, "u") || strings.HasPrefix(body, "ADD") {
			continue
		}

		if !strings.HasPrefix(body, "package") && !strings.HasPrefix(body, "import") &&
			!strings.HasPrefix(body, "func") && !strings.HasPrefix(body, "}") &&
			!strings.HasPrefix(body, "\t") {
			t.Errorf("patch line is a mid-line fragment: %q", line)
		}
	}
}
