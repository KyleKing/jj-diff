package diff_test

import (
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

func TestHunkSectionAndRange(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, header       string
		section, wantRange string
		oldStart, oldLines int
		newStart, newLines int
	}{
		{
			name: "function", header: "@@ -1,10 +1,12 @@ func parse(input string) int {",
			oldStart: 1, oldLines: 10, newStart: 1, newLines: 12,
			section: "func parse(input string) int {", wantRange: "L1–12",
		},
		{
			name: "no function", header: "@@ -4,3 +4,3 @@",
			oldStart: 4, oldLines: 3, newStart: 4, newLines: 3, wantRange: "L4–6",
		},
		{
			name: "new file", header: "@@ -0,0 +1,1 @@ note",
			newStart: 1, newLines: 1, wantRange: "L1",
		},
		{
			name: "deleted file", header: "@@ -1,2 +0,0 @@ gone",
			oldStart: 1, oldLines: 2, wantRange: "L1–2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := diff.Hunk{
				Header: tc.header, OldStart: tc.oldStart, OldLines: tc.oldLines,
				NewStart: tc.newStart, NewLines: tc.newLines,
			}

			if got := h.Section(); got != tc.section {
				t.Errorf("Section() = %q, want %q", got, tc.section)
			}

			if got := h.Range(); got != tc.wantRange {
				t.Errorf("Range() = %q, want %q", got, tc.wantRange)
			}
		})
	}
}
