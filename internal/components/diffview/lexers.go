package diffview

import (
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
	"github.com/kyleking/jj-diff/internal/theme"
)

// keptLexers bounds how many files' colors the pane keeps: a few screens of a long file list, so
// walking down it and back lexes each file once.
const keptLexers = 64

// lexKey names one file of one parsed diff. A file's hunks are the parse's own backing array, which a
// reload replaces, so the same path read again never answers stale colors; the theme's generation
// keeps a palette change from drawing in the old one.
type lexKey struct {
	hunks      *diff.Hunk
	path       string
	generation uint64
}

// lexers keeps the highlighters of the files shown before. Lexing is the bulk of what moving to a
// file costs, and it is the same work every time the file comes back.
type lexers struct {
	byFile map[lexKey]*highlight.Highlighter
	order  []lexKey
}

// highlighterFor is file's highlighter, the one kept from an earlier visit where there is one.
func (l *lexers) highlighterFor(file diff.FileChange) *highlight.Highlighter {
	if len(file.Hunks) == 0 {
		return highlight.New(file)
	}

	key := lexKey{hunks: &file.Hunks[0], path: file.Path, generation: theme.Generation()}
	if h, ok := l.byFile[key]; ok {
		return h
	}

	if l.byFile == nil {
		l.byFile = map[lexKey]*highlight.Highlighter{}
	}

	if len(l.order) >= keptLexers {
		delete(l.byFile, l.order[0])
		l.order = l.order[1:]
	}

	h := highlight.New(file)
	l.byFile[key] = h
	l.order = append(l.order, key)

	return h
}
