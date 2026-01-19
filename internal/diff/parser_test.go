package diff

import (
	"testing"
)

func TestParse_EmptyDiff(t *testing.T) {
	result := Parse("")
	if len(result) != 0 {
		t.Errorf("Expected 0 files, got %d", len(result))
	}
}

func TestParse_SimpleDiff(t *testing.T) {
	input := `diff --git a/file.txt b/file.txt
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,4 @@
 line 1
+line 2
 line 3
 line 4
`

	result := Parse(input)

	if len(result) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(result))
	}

	file := result[0]
	if file.Path != "file.txt" {
		t.Errorf("Expected path 'file.txt', got '%s'", file.Path)
	}

	if file.ChangeType != ChangeTypeModified {
		t.Errorf("Expected ChangeTypeModified, got %v", file.ChangeType)
	}

	if len(file.Hunks) != 1 {
		t.Fatalf("Expected 1 hunk, got %d", len(file.Hunks))
	}

	hunk := file.Hunks[0]
	if hunk.OldStart != 1 || hunk.OldLines != 3 {
		t.Errorf("Expected old range 1,3 got %d,%d", hunk.OldStart, hunk.OldLines)
	}

	if hunk.NewStart != 1 || hunk.NewLines != 4 {
		t.Errorf("Expected new range 1,4 got %d,%d", hunk.NewStart, hunk.NewLines)
	}

	if len(hunk.Lines) != 4 {
		t.Fatalf("Expected 4 lines, got %d", len(hunk.Lines))
	}

	expectedTypes := []LineType{LineContext, LineAddition, LineContext, LineContext}
	for i, expected := range expectedTypes {
		if hunk.Lines[i].Type != expected {
			t.Errorf("Line %d: expected type %v, got %v", i, expected, hunk.Lines[i].Type)
		}
	}
}

func TestParse_NewFile(t *testing.T) {
	input := `diff --git a/newfile.txt b/newfile.txt
new file mode 100644
--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,2 @@
+new content
+line 2
`

	result := Parse(input)

	if len(result) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(result))
	}

	file := result[0]
	if file.ChangeType != ChangeTypeAdded {
		t.Errorf("Expected ChangeTypeAdded, got %v", file.ChangeType)
	}
}

func TestParse_DeletedFile(t *testing.T) {
	input := `diff --git a/oldfile.txt b/oldfile.txt
deleted file mode 100644
--- a/oldfile.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-old content
-line 2
`

	result := Parse(input)

	if len(result) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(result))
	}

	file := result[0]
	if file.ChangeType != ChangeTypeDeleted {
		t.Errorf("Expected ChangeTypeDeleted, got %v", file.ChangeType)
	}
}

func TestParse_MultipleFiles(t *testing.T) {
	input := `diff --git a/file1.txt b/file1.txt
--- a/file1.txt
+++ b/file1.txt
@@ -1 +1,2 @@
 line 1
+line 2
diff --git a/file2.txt b/file2.txt
--- a/file2.txt
+++ b/file2.txt
@@ -1 +1,2 @@
 content
+more content
`

	result := Parse(input)

	if len(result) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(result))
	}

	if result[0].Path != "file1.txt" {
		t.Errorf("Expected first file 'file1.txt', got '%s'", result[0].Path)
	}

	if result[1].Path != "file2.txt" {
		t.Errorf("Expected second file 'file2.txt', got '%s'", result[1].Path)
	}
}

func TestFileChange_AddedLines(t *testing.T) {
	fc := FileChange{
		Hunks: []Hunk{
			{
				Lines: []Line{
					{Type: LineContext},
					{Type: LineAddition},
					{Type: LineAddition},
					{Type: LineDeletion},
				},
			},
		},
	}

	added := fc.AddedLines()
	if added != 2 {
		t.Errorf("Expected 2 added lines, got %d", added)
	}
}

func TestFileChange_DeletedLines(t *testing.T) {
	fc := FileChange{
		Hunks: []Hunk{
			{
				Lines: []Line{
					{Type: LineContext},
					{Type: LineAddition},
					{Type: LineDeletion},
					{Type: LineDeletion},
				},
			},
		},
	}

	deleted := fc.DeletedLines()
	if deleted != 2 {
		t.Errorf("Expected 2 deleted lines, got %d", deleted)
	}
}

func TestFileChange_TotalLines(t *testing.T) {
	fc := FileChange{
		Hunks: []Hunk{
			{Lines: []Line{{}, {}, {}}},
			{Lines: []Line{{}, {}}},
		},
	}

	total := fc.TotalLines()
	if total != 5 {
		t.Errorf("Expected 5 total lines, got %d", total)
	}
}
