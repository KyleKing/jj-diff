package diffview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/pen"
	"github.com/kyleking/aragonite/tui/skin"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
	"github.com/kyleking/jj-diff/internal/theme"
)

// The bases a row draws on: a context, added, or removed line's, and the cursor's selection.
const (
	baseContext = iota
	baseAdded
	baseRemoved
	baseCursor
	bases
)

// The line kinds a gutter tells apart, indexing a base's number and sign pens.
const (
	kindContext = iota
	kindAdded
	kindRemoved
	kinds
)

// rowPens is everything one base draws with. Building it costs a few lipgloss renders per pen, so it
// is built once per theme rather than once per row, which is most of what a frame used to cost.
type rowPens struct {
	faces  *highlight.Faces
	number [kinds]pen.Pen
	sign   [kinds]pen.Pen
	marker pen.Pen
}

// pens is every pen the pane draws with, for the theme generation it was built under.
type pens struct {
	row         [bases]rowPens
	inRange     string
	picked      string
	current     string
	separator   string
	wordAdded   pen.Pen
	wordRemoved pen.Pen
	match       pen.Pen
	foldNote    pen.Pen
	subtitle    pen.Pen
	muted       pen.Pen
	accent      pen.Pen
	success     pen.Pen
	cursor      pen.Pen
	generation  uint64
}

func kindOf(kind diff.LineType) int {
	switch kind {
	case diff.LineAddition:
		return kindAdded
	case diff.LineDeletion:
		return kindRemoved
	case diff.LineContext:
	}

	return kindContext
}

func baseOf(kind diff.LineType, isCursor bool) int {
	if isCursor {
		return baseCursor
	}

	switch kind {
	case diff.LineAddition:
		return baseAdded
	case diff.LineDeletion:
		return baseRemoved
	case diff.LineContext:
	}

	return baseContext
}

// pensFor answers the pens for the current theme, rebuilding them when Init ran again since.
func (m *Model) pensFor() *pens {
	gen := theme.Generation()
	if p := m.cache.pens; p != nil && p.generation == gen {
		return p
	}

	// The pane's highlighter always draws in the theme's palette, so the pens outlive any one file.
	p := &pens{generation: gen}
	palette := highlight.ThemePalette()

	for b := range bases {
		base := baseStyle(b)
		r := &p.row[b]
		r.faces = palette.Faces(base)
		r.marker = pen.Of(base.Faint(true))

		for k, kind := range []diff.LineType{diff.LineContext, diff.LineAddition, diff.LineDeletion} {
			quiet := theme.Skin.Muted
			if kind != diff.LineContext {
				quiet = theme.Skin.Subtitle
			}

			r.number[k] = pen.Of(skin.On(base, quiet))
			r.sign[k] = pen.Of(signStyle(kind, &base))
		}
	}

	p.wordAdded, p.wordRemoved = pen.Of(theme.WordDiffAddedStyle), pen.Of(theme.WordDiffDeletedStyle)
	p.match = pen.Of(lipgloss.NewStyle().Background(theme.Accent).Foreground(theme.Palette.Base))
	p.foldNote = pen.Of(foldNoteStyle())
	p.subtitle, p.muted = pen.Of(theme.Skin.Subtitle), pen.Of(theme.Skin.Muted)
	p.accent, p.success, p.cursor = pen.Of(theme.Skin.Accent), pen.Of(theme.Skin.Success), pen.Of(theme.Skin.Cursor)
	p.inRange, p.picked, p.current = p.cursor.Render("┃ "), p.cursor.Render("• "), p.cursor.Render("▸ ")
	p.separator = pen.Of(theme.Skin.Hairline).Render(paneSeparator)

	m.cache.pens = p
	m.invalidate()

	return p
}

func baseStyle(b int) lipgloss.Style {
	switch b {
	case baseCursor:
		return rowBase(diff.LineContext, true)
	case baseAdded:
		return rowBase(diff.LineAddition, false)
	case baseRemoved:
		return rowBase(diff.LineDeletion, false)
	}

	return rowBase(diff.LineContext, false)
}

// rowKey names one drawn row: which line, how wide, and every state that changes how it looks.
type rowKey struct {
	hunk  int
	at    int
	width int
	state uint8
}

// headerKey names one drawn hunk header, which shows the cursor, the selection mark, and the tags.
type headerKey struct {
	tags     string
	hunk     int
	width    int
	current  bool
	selected bool
}

// The state bits of a rowKey.
const (
	stateCurrent uint8 = 1 << iota
	stateInRange
	stateSelected
	statePair
)

// maxCachedRows bounds the row cache. Rows are a screen's worth at a time, so this holds dozens of
// screens before it is dropped and refilled.
const maxCachedRows = 4096

// drawCache holds what View drew, so a frame where only the cursor moved redraws one or two rows.
// It is held by pointer so copies of a Model share one cache, which only View touches.
type drawCache struct {
	pens      *pens
	rows      map[rowKey]string
	headers   map[headerKey]string
	trackHits *SearchHit
	track     []string
	trackKey  trackKey
}

func (m *Model) cachedRow(key rowKey) (string, bool) {
	row, ok := m.cache.rows[key]

	return row, ok
}

func (m *Model) keepRow(key rowKey, row string) {
	if m.cache.rows == nil || len(m.cache.rows) >= maxCachedRows {
		m.cache.rows = make(map[rowKey]string, maxCachedRows/8)
	}

	m.cache.rows[key] = row
}

// invalidate drops every drawn row, for a change to what the rows show.
func (m *Model) invalidate() {
	if m.cache != nil {
		m.cache.rows = nil
		m.cache.headers = nil
		m.cache.track = nil
	}
}

// writeNumber appends the gutter's number as "%4d " would, without fmt.
func writeNumber(b *strings.Builder, p *pen.Pen, number int) {
	var buf [24]byte

	digits := strconv.AppendInt(buf[:0], int64(number), 10)
	text := make([]byte, 0, len(digits)+numberWidth)

	for range numberWidth - 1 - len(digits) {
		text = append(text, ' ')
	}

	text = append(text, digits...)
	text = append(text, ' ')

	p.Write(b, string(text))
}

// plainWidth is the cells content takes once its tabs are drawn as lipgloss draws them, or -1 where
// it holds a byte that the measuring of styled text would read differently (an escape, a control
// character), so the caller measures what it drew instead.
func plainWidth(content string) int {
	ascii := true
	tabs := 0

	for i := range len(content) {
		c := content[i]

		switch {
		case c == '\t':
			tabs++
		case c < ' ' || c == 0x7f:
			return -1
		case c >= utf8.RuneSelf:
			ascii = false
		}
	}

	if ascii {
		return len(content) + tabs*(tabCells-1)
	}

	return ansi.StringWidth(strings.ReplaceAll(content, "\t", "    "))
}

// bytesPerCell sizes a row's builder: a cell's text plus its share of the escape sequences around
// the runs that color it, so a row rarely grows its buffer.
const bytesPerCell = 3

// tabCells is how many cells lipgloss draws a tab as.
const tabCells = 4

// HighlightedMsg says the syntax colors of a file the pane loaded have landed, so the next frame
// draws them. A pane that has since loaded another file ignores it.
type HighlightedMsg struct {
	Path string
}

// Highlighted is closed once the current file's syntax colors are ready, and nil with no file.
// Until then the pane draws the file's code plain.
func (m *Model) Highlighted() <-chan struct{} {
	return m.lexed
}

// HighlightCmd waits for the current file's colors and answers HighlightedMsg, so the program
// redraws when they land. It answers nil when there is nothing to wait for or a command for this
// file was already handed out, so a caller can ask after every update.
func (m *Model) HighlightCmd() tea.Cmd {
	if m.fileChange == nil || m.lexed == nil || m.announced {
		return nil
	}

	m.announced = true

	lexed, path := m.lexed, m.fileChange.Path

	select {
	case <-lexed:
		return nil
	default:
	}

	return func() tea.Msg {
		<-lexed

		return HighlightedMsg{Path: path}
	}
}
