package diffview

import (
	"github.com/kyleking/jj-diff/internal/diff"
)

// RenderContext is everything a ViewMode needs for one render pass. The callbacks are invoked per
// visible hunk or line, so they must be cheap, and a nil callback means that decoration is off.
type RenderContext struct {
	IsSelected      func(hunkIdx int) bool
	WordDiffCache   *WordDiffCache
	IsLineSelected  func(hunkIdx, lineIdx int) bool
	GetMatches      func(hunkIdx, lineIdx int) []MatchRange
	SelectedHunk    int
	LineCursor      int
	TabWidth        int
	VisualAnchor    int
	Width           int
	Height          int
	IsVisualMode    bool
	IsSearching     bool
	Focused         bool
	ShowLineNumbers bool
	ShowWhitespace  bool
	WordLevelDiff   bool
}

// ViewMode is one diff layout. Render must pad its output to ctx.Width by ctx.Height, because the
// caller composites it against a fixed pane.
type ViewMode interface {
	Render(file *diff.FileChange, ctx RenderContext) string
	SupportsSelection() bool
}
