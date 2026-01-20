package diffview

import (
	"github.com/kyleking/jj-diff/internal/diff"
)

type RenderContext struct {
	Focused         bool
	Height          int
	IsLineSelected  func(hunkIdx, lineIdx int) bool
	IsSearching     bool
	IsSelected      func(hunkIdx int) bool
	IsVisualMode    bool
	GetMatches      func(hunkIdx, lineIdx int) []MatchRange
	LineCursor      int
	SelectedHunk    int
	ShowLineNumbers bool
	ShowWhitespace  bool
	TabWidth        int
	VisualAnchor    int
	Width           int
	WordDiffCache   *WordDiffCache
	WordLevelDiff   bool
}

type ViewMode interface {
	Render(file *diff.FileChange, ctx RenderContext) string
	SupportsSelection() bool
}
