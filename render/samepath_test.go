package render_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/render"
)

// Two commits of `git log -p` touching one file put two sections with the same path into one
// Document, and their lines share line numbers. Each section's colors must come from its own
// lexing, so a short line in the second section is never painted with spans the first section's
// longer line at the same numbers produced.
const samePathTwice = `diff --git a/main.go b/main.go
index 111..222 100644
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 func aVeryLongFunctionNameThatLexesIntoManySpans(argument string) error { return nil }
-const limit = 10
+const limit = 20
diff --git a/main.go b/main.go
index 222..333 100644
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 x
-y
+z
`

func TestDocumentSamePathTwice_ColorsEachSectionFromItsOwnLexing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		sideBySide bool
		peek       bool
	}{
		{name: "unified"},
		{name: "side by side", sideBySide: true},
		{name: "peek after start", peek: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			doc := render.New(samePathTwice, render.Options{Syntax: true, SideBySide: tc.sideBySide, Width: 200})

			var rows []string
			if tc.peek {
				<-doc.Start()

				var ready bool

				rows, ready = doc.Peek(0, doc.Len())
				if !ready {
					t.Errorf("Peek() ready = false after Start's channel closed")
				}
			} else {
				rows = doc.Rows(0, doc.Len())
			}

			joined := ansi.Strip(strings.Join(rows, "\n"))
			want := []string{"aVeryLongFunctionNameThatLexesIntoManySpans", "const limit = 20", " x", "z"}
			for _, want := range want {
				if !strings.Contains(joined, want) {
					t.Errorf("rows = %q, want it to contain %q", joined, want)
				}
			}
		})
	}
}
