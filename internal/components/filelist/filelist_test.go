package filelist_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/diff"
)

func changedFile(path string, additions, deletions int) diff.FileChange {
	lines := make([]diff.Line, 0, additions+deletions)
	for range additions {
		lines = append(lines, diff.Line{Type: diff.LineAddition})
	}

	for range deletions {
		lines = append(lines, diff.Line{Type: diff.LineDeletion})
	}

	return diff.FileChange{
		Path:       path,
		ChangeType: diff.ChangeTypeModified,
		Hunks:      []diff.Hunk{{Lines: lines}},
	}
}

// The stats are two columns sized to the widest count, so the numbers line up whatever the path
// length is, and a count of zero leaves its cell blank.
func TestExpandedListAlignsStatsAndOmitsZeroCounts(t *testing.T) {
	t.Parallel()

	m := filelist.New()
	m.SetExpanded(true)
	m.SetFiles([]diff.FileChange{
		changedFile("a.go", 120, 0),
		changedFile("internal/long/path.go", 3, 45),
	})

	rows := strings.Split(ansi.Strip(m.View(40, m.ContentHeight(), true)), "\n")
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want a title and one row per file: %q", len(rows), rows)
	}

	for _, want := range []string{"M  a.go", "+120", "M  internal/long/path.go", "  +3 -45"} {
		if !strings.Contains(strings.Join(rows, "\n"), want) {
			t.Errorf("list = %q, want it to contain %q", rows, want)
		}
	}

	if strings.Contains(strings.Join(rows, "\n"), "-0") || strings.Contains(rows[1]+rows[2], "Path") {
		t.Errorf("list = %q, want no -0 and no column header", rows)
	}

	if !strings.HasSuffix(strings.TrimRight(rows[1], " "), "+120") {
		t.Errorf("row %q should end at the add column, leaving the delete column blank", rows[1])
	}
}
