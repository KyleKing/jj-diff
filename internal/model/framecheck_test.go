package model_test

import (
	"testing"

	"github.com/kyleking/aragonite/tui/framecheck"
)

// TestChromeIsWidthStable holds every string literal in the module to aragonite's glyph policy: no
// glyph a terminal may draw two cells wide (an emoji-capable arrow, a text-default emoji) in a
// slot the layout counts as one.
func TestChromeIsWidthStable(t *testing.T) {
	t.Parallel()

	framecheck.Source(t, "../..")
}
