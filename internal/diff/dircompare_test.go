package diff_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

// bigLines returns n numbered lines, enough to spread a file across several read chunks.
func bigLines(n int, last string) string {
	var b strings.Builder
	for i := range n {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", i%40))
		b.WriteString("\n")
	}

	b.WriteString(last)

	return b.String()
}

type pairCase struct {
	left  map[string]string
	right map[string]string
	name  string
	want  string
}

func pairCases() []pairCase {
	big := bigLines(5000, "tail a\n")
	bigSameSize := bigLines(5000, "tail b\n")

	return []pairCase{
		{
			name:  "identical files are skipped",
			left:  map[string]string{"a.txt": "same\n", "dir/b.go": "package b\n"},
			right: map[string]string{"a.txt": "same\n", "dir/b.go": "package b\n"},
			want:  "",
		},
		{
			name:  "identical empty files are skipped",
			left:  map[string]string{"empty": ""},
			right: map[string]string{"empty": ""},
			want:  "",
		},
		{
			name:  "identical file larger than one read chunk is skipped",
			left:  map[string]string{"big.txt": big},
			right: map[string]string{"big.txt": big},
			want:  "",
		},
		{
			name:  "same size different content is still diffed",
			left:  map[string]string{"f.txt": "a\nb\n"},
			right: map[string]string{"f.txt": "a\nc\n"},
			want: "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n" +
				"@@ -1,2 +1,2 @@\n a\n-b\n+c\n",
		},
		{
			name:  "same size difference past the first read chunk is still diffed",
			left:  map[string]string{"big.txt": big},
			right: map[string]string{"big.txt": bigSameSize},
			want: "diff --git a/big.txt b/big.txt\n--- a/big.txt\n+++ b/big.txt\n" +
				"@@ -4998,4 +4998,4 @@\n" +
				" line " + strings.Repeat("x", 4997%40) + "\n" +
				" line " + strings.Repeat("x", 4998%40) + "\n" +
				" line " + strings.Repeat("x", 4999%40) + "\n" +
				"-tail a\n+tail b\n",
		},
		{
			name:  "different size is diffed",
			left:  map[string]string{"g.txt": "one\n"},
			right: map[string]string{"g.txt": "one\ntwo\n"},
			want:  "diff --git a/g.txt b/g.txt\n--- a/g.txt\n+++ b/g.txt\n@@ -1,1 +1,2 @@\n one\n+two\n",
		},
		{
			name:  "missing trailing newline is diffed",
			left:  map[string]string{"n.txt": "a"},
			right: map[string]string{"n.txt": "b"},
			want:  "diff --git a/n.txt b/n.txt\n--- a/n.txt\n+++ b/n.txt\n@@ -1,1 +1,1 @@\n-a\n+b\n",
		},
		{
			name:  "binary content of equal size is diffed as before",
			left:  map[string]string{"bin": "\x00\x01\x02\n"},
			right: map[string]string{"bin": "\x00\x01\x03\n"},
			want: "diff --git a/bin b/bin\n--- a/bin\n+++ b/bin\n" +
				"@@ -1,1 +1,1 @@\n-\x00\x01\x02\n+\x00\x01\x03\n",
		},
		{
			name:  "added and removed files",
			left:  map[string]string{"gone.txt": "x\n", "keep.txt": "k\n"},
			right: map[string]string{"new.txt": "y\n", "keep.txt": "k\n"},
			want: "diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\n" +
				"--- a/gone.txt\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-x\n" +
				"diff --git a/new.txt b/new.txt\nnew file mode 100644\n" +
				"--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1,1 @@\n+y\n",
		},
		{
			name:  "added empty file produces nothing",
			left:  map[string]string{},
			right: map[string]string{"empty.txt": ""},
			want:  "",
		},
	}
}

// writePair creates both trees, including an empty one, so a case with no files on a side still has
// a directory to walk.
func writePair(t *testing.T, tc pairCase) (string, string) {
	t.Helper()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")

	for dir, files := range map[string]map[string]string{left: tc.left, right: tc.right} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}

		for name, content := range files {
			writeTree(t, dir, name, content)
		}
	}

	return left, right
}

// TestCompareDirectories_Pairs pins the exact patch for every kind of file pair, so the size and
// content short-circuit can only ever drop pairs whose bytes are identical.
func TestCompareDirectories_Pairs(t *testing.T) {
	t.Parallel()

	for _, tt := range pairCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			left, right := writePair(t, tt)

			got, err := diff.CompareDirectories(left, right)
			if err != nil {
				t.Fatalf("CompareDirectories: %v", err)
			}

			if got != tt.want {
				t.Errorf("patch mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// TestCompareDirectories_LinkedFiles covers pairs that share their bytes through the filesystem
// rather than through a copy. Content decides, as it did when both sides were read in full.
func TestCompareDirectories_LinkedFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		link func(target, link string) error
		name string
	}{
		{link: os.Link, name: "hard link"},
		{link: os.Symlink, name: "symlink"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			left := filepath.Join(base, "left")
			right := filepath.Join(base, "right")
			writeTree(t, left, "f.txt", "shared\n")

			if err := os.MkdirAll(right, 0o750); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}

			if err := tt.link(filepath.Join(left, "f.txt"), filepath.Join(right, "f.txt")); err != nil {
				t.Skipf("linking unsupported here: %v", err)
			}

			got, err := diff.CompareDirectories(left, right)
			if err != nil {
				t.Fatalf("CompareDirectories: %v", err)
			}

			if got != "" {
				t.Errorf("linked identical file should produce no patch, got %q", got)
			}
		})
	}
}

// TestCompareDirectories_UnreadableFileErrors keeps a pair that cannot be read an error rather than a
// silently skipped file, even when the sizes match.
func TestCompareDirectories_UnreadableFileErrors(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	writeTree(t, left, "f.txt", "x\n")

	if err := os.MkdirAll(right, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.Symlink(filepath.Join(base, "missing"), filepath.Join(right, "f.txt")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	if _, err := diff.CompareDirectories(left, right); err == nil {
		t.Fatal("expected an error for a dangling symlink")
	}
}
