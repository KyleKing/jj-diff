package filelist

import (
	"slices"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/skin"

	"github.com/kyleking/jj-diff/internal/theme"
)

// ReadGlyph marks a file every hunk of which has been read, and leads a read hunk's header. It is
// the cell jojo's read strip fills (▰ read, ▱ unread), one stable cell wide, and deliberately not a
// check, which is what a selected hunk carries in -i.
const ReadGlyph = "▰"

// FileReads is how much of one file has been read, by hunk.
type FileReads struct {
	Read  int
	Total int
}

// reads is the read state the list draws: a column for whole files read, this file's unread count
// in the collapsed heading, and the revision's in the expanded rule.
type reads struct {
	files []FileReads
	// generation changes whenever files does, so a frame drawn from older counts is not reused.
	generation uint64
	unread     int
	on         bool
}

// SetReads draws read state, one entry per file in list order; nil turns it off. It is cheap to
// call with unchanged counts, which keep the drawn frame.
func (m *Model) SetReads(files []FileReads) {
	on := files != nil

	if on == m.reads.on && slices.Equal(files, m.reads.files) {
		return
	}

	unread := 0
	for _, f := range files {
		unread += f.Total - f.Read
	}

	m.reads = reads{
		files: slices.Clone(files), generation: m.reads.generation + 1, unread: unread, on: on,
	}
}

// SetOnlyUnread leaves the files read whole out of the expanded list, or puts them back. It
// changes only what is drawn: the selection is still an index into every file, and the caller
// keeps it off a file the list leaves out.
func (m *Model) SetOnlyUnread(on bool) { m.onlyUnread = on }

func (r *reads) at(i int) (FileReads, bool) {
	if !r.on || i < 0 || i >= len(r.files) {
		return FileReads{}, false
	}

	return r.files[i], true
}

// fileRead reports whether file i has hunks and every one of them is read.
func (r *reads) fileRead(i int) bool {
	f, ok := r.at(i)

	return ok && f.Total > 0 && f.Read == f.Total
}

// readCell is the read column's width: the glyph and a space.
const readCell = 2

// cell is the read column of row i: the muted glyph for a file read whole, blank otherwise, and
// nothing at all while read state is off, so a list without marks keeps its layout.
func (r *reads) cell(i int, base *lipgloss.Style) string {
	if !r.on {
		return ""
	}

	if !r.fileRead(i) {
		return base.Render(blank(readCell))
	}

	return skin.On(*base, theme.Skin.Muted).Render(ReadGlyph) + base.Render(" ")
}

// heading is what the collapsed heading says of file i before its counts: how many of its hunks are
// unread, or the glyph once all are read.
func (r *reads) heading(i int, band *lipgloss.Style) string {
	f, ok := r.at(i)
	if !ok || f.Total == 0 {
		return ""
	}

	text := strconv.Itoa(f.Total-f.Read) + " unread"
	if f.Read == f.Total {
		text = ReadGlyph + " read"
	}

	return skin.On(*band, theme.Skin.Muted).Render(text) + band.Render(headingGap)
}

// total is what the expanded rule adds after the cursor's place: the revision's unread hunks.
func (r *reads) total() string {
	if !r.on || len(r.files) == 0 {
		return ""
	}

	if r.unread == 0 {
		return " · all read"
	}

	return " · " + strconv.Itoa(r.unread) + " unread"
}

// blank is n spaces.
func blank(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}

	return string(b)
}
