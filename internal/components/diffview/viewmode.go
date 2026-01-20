package diffview

import (
	"github.com/kyleking/jj-diff/internal/diff"
)

type RenderContext struct {
	Width           int
	Height          int
	SelectedHunk    int
	LineCursor      int
	IsVisualMode    bool
	VisualAnchor    int
	ShowWhitespace  bool
	ShowLineNumbers bool
	TabWidth        int
	WordLevelDiff   bool
	IsSearching     bool
	IsSelected      func(hunkIdx int) bool
	IsLineSelected  func(hunkIdx, lineIdx int) bool
	GetMatches      func(hunkIdx, lineIdx int) []MatchRange
	WordDiffCache   *WordDiffCache
}

type ViewMode interface {
	Render(file *diff.FileChange, ctx RenderContext) string
	SupportsSelection() bool
}
