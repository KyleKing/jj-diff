// Package render turns unified diff text into styled terminal lines. It is the read-only half of
// jj-diff's diff rendering, published for other tools to draw a diff the same way this one does:
// the same parser, the same word-level intra-line highlighting, and the same syntax highlighting.
// Hunk and line selection stay inside jj-diff, since only an editor needs them.
package render

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/generated"
	"github.com/kyleking/aragonite/tui/skin"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
)

// Palette is every face a rendered diff draws in. A nil color leaves that part unstyled, so a caller
// with no color for one of these passes nothing rather than a guess.
type Palette struct {
	// File heads the block for one path.
	File color.Color
	// Hunk is the @@ line naming the range.
	Hunk color.Color
	// Added, Removed, and Context are the line faces, applied to the whole line.
	Added   color.Color
	Removed color.Color
	Context color.Color
	// AddedWord and RemovedWord are the backgrounds marking what changed inside a line, drawn only
	// when Options.WordDiff is set.
	AddedWord   color.Color
	RemovedWord color.Color
	// Gutter is the line-number column, drawn only when Options.LineNumbers is set, and the line
	// range Options.HunkLabels puts beside a hunk's function.
	Gutter color.Color
	// AddedWash and RemovedWash tint a changed line's whole row, out to Width when it is set. A
	// washed line's text keeps Context's face, since the tint already says which side it is on.
	AddedWash   color.Color
	RemovedWash color.Color
	// Syntax is the token faces. A zero Syntax renders code unhighlighted.
	Syntax SyntaxPalette
}

// SyntaxPalette is the face each class of code token renders in. A nil color leaves that class
// unstyled.
type SyntaxPalette struct {
	Comment  color.Color
	Keyword  color.Color
	String   color.Color
	Number   color.Color
	Name     color.Color
	Type     color.Color
	Operator color.Color
}

func (p *SyntaxPalette) internal() highlight.Palette {
	return highlight.Palette(*p)
}

// Options is what one call renders. The zero value is a plain, full-width, unhighlighted diff.
type Options struct {
	Palette Palette
	// Width truncates each line to that many display cells. Zero leaves lines whole.
	Width int
	// Syntax highlights the code inside each line. It costs a lexer pass per line, so a caller
	// drawing a large diff into a small pane can leave it off.
	Syntax bool
	// WordDiff marks what changed inside a modified line, pairing each deletion with the addition
	// that replaced it.
	WordDiff bool
	// LineNumbers puts the old and new line numbers in a gutter.
	LineNumbers bool
	// SideBySide draws the old and the new content in two columns, pairing each run of deletions
	// with the additions that replaced it. It needs Width, since the columns are cut from it.
	SideBySide bool
	// CollapseGenerated draws a generated file, such as a lockfile, as one line with its size.
	CollapseGenerated bool
	// FoldMechanical draws a reindent-only or whitespace-only hunk as one line, after a pass over every file.
	FoldMechanical bool
	// SignBar marks a changed line's side with a ▎ bar in Added or Removed instead of a + or -.
	// Leave it off under NO_COLOR, where a bar cannot say which side a line is on.
	SignBar bool
	// HunkLabels heads each hunk with the function git found around it and the lines it covers,
	// `func parse()  L13–24`, rather than the raw @@ line.
	HunkLabels bool
}

// Lines renders diffText as one styled string per terminal row, in file order. Input that parses to
// nothing returns nothing, because a caller has a better empty state than a blank pane.
//
// Input must be in git format, since jj's own default format does not parse.
// Callers run `jj diff --git`.
//
//nolint:gocritic // hugeParam: Options is a value the caller builds once per call.
func Lines(diffText string, opts Options) []string {
	doc := New(diffText, opts)

	return doc.Rows(0, doc.Len())
}

// Document is a parsed diff whose rows render on demand, so a window of a large diff costs only that window.
type Document struct {
	renderer *renderer
	files    []diff.FileChange
	segments []segment
	rows     int
}

const generatedRows = 2

type segment struct {
	note  string
	pairs []diff.SidePair
	file  int
	hunk  int
	start int
	rows  int
}

// New parses diffText once and counts its rows without rendering them. Input that parses to nothing has Len zero.
//
//nolint:gocritic // hugeParam: Options is a value the caller builds once per call.
func New(diffText string, opts Options) *Document {
	files := diff.Parse(diffText)
	doc := &Document{files: files, renderer: &renderer{opts: opts, words: map[wordKey]map[int]diff.WordDiffResult{}}}

	if len(files) == 0 {
		return doc
	}

	if opts.Syntax {
		doc.renderer.highlighter = highlight.NewWith((&opts.Palette.Syntax).internal(), files...)
	}

	var mechanical map[diff.HunkKey]string
	if opts.FoldMechanical {
		mechanical = diff.MechanicalHunks(files)
	}

	set := generated.New(nil)

	for i := range files {
		if opts.CollapseGenerated && len(files[i].Hunks) > 0 && set.Match(files[i].Path) {
			doc.add(segment{file: i, hunk: -1, rows: generatedRows})

			continue
		}

		doc.add(segment{file: i, hunk: -1, rows: 1})

		for j := range files[i].Hunks {
			doc.add(doc.hunkSegment(i, j, mechanical[diff.HunkKey{File: i, Hunk: j}]))
		}
	}

	return doc
}

func (d *Document) hunkSegment(file, hunk int, reason string) segment {
	lines := d.files[file].Hunks[hunk].Lines
	seg := segment{file: file, hunk: hunk, note: reason}

	switch {
	case reason != "":
		seg.rows = 2
	case d.renderer.opts.SideBySide:
		seg.pairs = diff.PairSides(lines)
		seg.rows = 1 + len(seg.pairs)
	default:
		seg.rows = 1 + len(lines)
	}

	return seg
}

func (d *Document) add(seg segment) {
	seg.start = d.rows
	d.rows += seg.rows
	d.segments = append(d.segments, seg)
}

// Len is the number of rows the whole diff renders to.
func (d *Document) Len() int {
	return d.rows
}

// Rows renders rows [from, to), clamped to the document, exactly as Lines draws them.
func (d *Document) Rows(from, to int) []string {
	from, to = max(from, 0), min(to, d.rows)
	if from >= to {
		return nil
	}

	out := make([]string, 0, to-from)
	first := sort.Search(len(d.segments), func(i int) bool {
		return d.segments[i].start+d.segments[i].rows > from
	})

	for i := first; i < len(d.segments) && d.segments[i].start < to; i++ {
		seg := &d.segments[i]

		for row := max(from-seg.start, 0); row < seg.rows && seg.start+row < to; row++ {
			out = append(out, d.row(seg, row))
		}
	}

	return out
}

func (d *Document) row(seg *segment, row int) string {
	r := d.renderer
	file := &d.files[seg.file]

	if seg.hunk < 0 {
		if row == 0 {
			return r.fit(r.styled(file.ChangeType.String()+" "+file.Path, r.opts.Palette.File))
		}

		return r.note(fmt.Sprintf(
			"  ▸ generated: +%d -%d in %d hunk%s", file.AddedLines(), file.DeletedLines(), len(file.Hunks),
			plural(len(file.Hunks)),
		))
	}

	hunk := &file.Hunks[seg.hunk]
	if row == 0 {
		return r.hunkHead(hunk)
	}

	if seg.note != "" {
		return r.note(fmt.Sprintf("  ▸ %s: %d changed lines", seg.note, changedLines(hunk)))
	}

	words := r.wordsFor(seg.file, seg.hunk, hunk)
	if r.opts.SideBySide {
		return r.column(file.Path, seg.pairs[row-1], words)
	}

	return r.line(file.Path, hunk.Lines[row-1], words[row-1])
}

func changedLines(hunk *diff.Hunk) int {
	n := 0

	for _, line := range hunk.Lines {
		if line.Type != diff.LineContext {
			n++
		}
	}

	return n
}

type wordKey struct{ file, hunk int }

type renderer struct {
	highlighter *highlight.Highlighter
	words       map[wordKey]map[int]diff.WordDiffResult
	opts        Options
}

// hunkLabelGap separates a hunk's function from its line range.
const hunkLabelGap = "  "

func (r *renderer) hunkHead(hunk *diff.Hunk) string {
	if !r.opts.HunkLabels {
		return r.fit(r.styled(hunk.Header, r.opts.Palette.Hunk))
	}

	span := r.styled(hunk.Range(), r.opts.Palette.Gutter)
	if section := hunk.Section(); section != "" {
		span = r.styled(section, r.opts.Palette.Hunk) + hunkLabelGap + span
	}

	return r.fit(span)
}

func (r *renderer) wordsFor(file, hunk int, h *diff.Hunk) map[int]diff.WordDiffResult {
	if !r.opts.WordDiff {
		return nil
	}

	key := wordKey{file: file, hunk: hunk}
	if words, ok := r.words[key]; ok {
		return words
	}

	words := diff.ComputeHunkWordDiffs(h)
	r.words[key] = words

	return words
}

func (r *renderer) note(text string) string {
	return r.fit(lipgloss.NewStyle().Faint(true).Render(r.styled(text, r.opts.Palette.Context)))
}

// gutterPad is the separator drawn between the two columns.
const gutterPad = " \u2502 "

// column draws one row of two panes. Each pane is fitted on its own, so a long line is cut at its
// own column rather than pushing the other pane off the row.
func (r *renderer) column(path string, pair diff.SidePair, words map[int]diff.WordDiffResult) string {
	pane := r.paneWidth()
	left := r.pane(path, pair.Left, false, words[pair.LeftIdx], pane)
	right := r.pane(path, pair.Right, true, words[pair.RightIdx], r.rightWidth(pane))

	return left + gutterPad + right
}

// paneWidth splits the frame between the two columns. A frame too narrow to split still gets one
// cell per pane, which renders as two empty columns rather than a panic.
func (r *renderer) paneWidth() int {
	const panes = 2

	if r.opts.Width <= 0 {
		return 0
	}

	return max(1, (r.opts.Width-lipgloss.Width(gutterPad))/panes)
}

// rightWidth gives the right pane the cell an odd frame leaves over, so a row fills the frame.
func (r *renderer) rightWidth(left int) int {
	if r.opts.Width <= 0 {
		return left
	}

	return max(left, r.opts.Width-lipgloss.Width(gutterPad)-left)
}

// pane draws one side of a row, padded to width so the separator stays in one column. A nil line is
// the blank opposite an addition or a deletion with no counterpart.
func (r *renderer) pane(path string, line *diff.Line, isRight bool, words diff.WordDiffResult, width int) string {
	if line == nil {
		return strings.Repeat(" ", max(width, 0))
	}

	if r.restyled() {
		return r.washed(r.numbers(sideNumber(*line, isRight), line.Type)+r.sign(line.Type), path, *line, words, width)
	}

	content := r.content(path, *line, words, lipgloss.Style{})
	if r.opts.LineNumbers {
		content = r.styled(sideNumber(*line, isRight), r.opts.Palette.Gutter) + content
	}

	styled := r.styled(content, r.face(line.Type))
	if width <= 0 {
		return styled
	}

	if gap := width - lipgloss.Width(styled); gap > 0 {
		return styled + strings.Repeat(" ", gap)
	}

	cut := r.cut(styled, width)

	return cut + strings.Repeat(" ", max(width-lipgloss.Width(cut), 0))
}

// sideNumber is the one line number that side carries, since a column shows one file's numbering.
func sideNumber(line diff.Line, isRight bool) string {
	n := line.OldLineNum
	if isRight {
		n = line.NewLineNum
	}

	return pad(strconv.Itoa(n)) + " "
}

func (r *renderer) line(path string, line diff.Line, words diff.WordDiffResult) string {
	if r.restyled() {
		lead := r.numbers(gutter(line), line.Type) + r.sign(line.Type)

		return r.washed(lead, path, line, words, r.opts.Width)
	}

	text := line.Type.String() + r.content(path, line, words, lipgloss.Style{})
	if r.opts.LineNumbers {
		text = r.styled(gutter(line), r.opts.Palette.Gutter) + text
	}

	return r.fit(r.styled(text, r.face(line.Type)))
}

// restyled says whether a changed line is drawn the newer way, washed or signed, rather than as
// a + or - line in its side's color.
func (r *renderer) restyled() bool {
	p := &r.opts.Palette

	return r.opts.SignBar || p.AddedWash != nil || p.RemovedWash != nil
}

// base is the face every part of a restyled line is drawn on: the wash behind it, and the text in
// Context where there is a wash to say the side and in the side's own color where there is not.
func (r *renderer) base(kind diff.LineType) lipgloss.Style {
	style := lipgloss.NewStyle()

	wash := r.wash(kind)
	if wash != nil {
		style = style.Background(wash)
	}

	face := r.face(kind)
	if wash != nil {
		face = r.opts.Palette.Context
	}

	if face != nil {
		style = style.Foreground(face)
	}

	return style
}

func (r *renderer) wash(kind diff.LineType) color.Color {
	switch kind {
	case diff.LineAddition:
		return r.opts.Palette.AddedWash
	case diff.LineDeletion:
		return r.opts.Palette.RemovedWash
	case diff.LineContext:
		return nil
	}

	return nil
}

// sign is the marker column: the bar in the side's color, or the +, -, or space a plain diff has.
func (r *renderer) sign(kind diff.LineType) string {
	base := r.base(kind)

	if !r.opts.SignBar {
		return base.Render(kind.String())
	}

	if kind == diff.LineContext {
		return base.Render(" ")
	}

	if face := r.face(kind); face != nil {
		base = base.Foreground(face)
	}

	return base.Render(skin.SignBar)
}

// numbers is the line-number gutter on the line's wash, or nothing when the options leave it off.
func (r *renderer) numbers(text string, kind diff.LineType) string {
	if !r.opts.LineNumbers {
		return ""
	}

	style := r.base(kind)
	if r.opts.Palette.Gutter != nil {
		style = style.Foreground(r.opts.Palette.Gutter)
	}

	return style.Render(text)
}

// washed draws lead and the line's content on its wash, cut or padded to width so the tint runs
// edge to edge. A zero width leaves the line its natural length.
func (r *renderer) washed(lead, path string, line diff.Line, words diff.WordDiffResult, width int) string {
	base := r.base(line.Type)
	text := lead + r.content(path, line, words, base)

	if width <= 0 {
		return text
	}

	if over := lipgloss.Width(text) - width; over > 0 {
		return r.cut(text, width)
	}

	return text + base.Render(strings.Repeat(" ", width-lipgloss.Width(text)))
}

// content is the line's text with whatever highlighting the options ask for, and no marker. Every
// part of it is drawn on base, so a wash behind the line is not broken by a token's reset.
//
//nolint:gocritic // hugeParam: lipgloss.Style is passed by value throughout lipgloss.
func (r *renderer) content(path string, line diff.Line, words diff.WordDiffResult, base lipgloss.Style) string {
	if spans := sideSpans(line.Type, words); len(spans) > 0 {
		return r.emphasize(line, words, base)
	}

	if r.highlighter != nil {
		return r.highlighter.Line(path, line, &base)
	}

	if r.restyled() {
		return base.Render(line.Content)
	}

	return line.Content
}

// emphasize redraws the line from its word spans, so syntax highlighting is dropped for that line:
// the two cannot both style the same bytes, and which bytes changed is the more useful of the two.
//
//nolint:gocritic // hugeParam: lipgloss.Style is passed by value throughout lipgloss.
func (r *renderer) emphasize(line diff.Line, words diff.WordDiffResult, base lipgloss.Style) string {
	var out strings.Builder

	mark := r.opts.Palette.AddedWord
	if line.Type == diff.LineDeletion {
		mark = r.opts.Palette.RemovedWord
	}

	for _, span := range sideSpans(line.Type, words) {
		if span.Type == diff.SpanEqual || mark == nil {
			out.WriteString(r.plain(span.Text, base))

			continue
		}

		out.WriteString(base.Background(mark).Render(span.Text))
	}

	return out.String()
}

// plain draws unmarked text on base, or leaves it bare for a line drawn the older way, whose face
// is applied around the whole line.
//
//nolint:gocritic // hugeParam: lipgloss.Style is passed by value throughout lipgloss.
func (r *renderer) plain(text string, base lipgloss.Style) string {
	if !r.restyled() {
		return text
	}

	return base.Render(text)
}

func sideSpans(kind diff.LineType, words diff.WordDiffResult) []diff.IntraLineSpan {
	switch kind {
	case diff.LineAddition:
		return words.NewSpans
	case diff.LineDeletion:
		return words.OldSpans
	case diff.LineContext:
		return nil
	}

	return nil
}

func (r *renderer) face(kind diff.LineType) color.Color {
	switch kind {
	case diff.LineAddition:
		return r.opts.Palette.Added
	case diff.LineDeletion:
		return r.opts.Palette.Removed
	case diff.LineContext:
		return r.opts.Palette.Context
	}

	return r.opts.Palette.Context
}

// gutterWidth is how wide each of the two line-number columns is, which fits a file of 99999 lines
// before the numbers start pushing the content right.
const gutterWidth = 5

func gutter(line diff.Line) string {
	old, next := strconv.Itoa(line.OldLineNum), strconv.Itoa(line.NewLineNum)

	if line.Type == diff.LineAddition {
		old = ""
	}

	if line.Type == diff.LineDeletion {
		next = ""
	}

	return pad(old) + " " + pad(next) + " "
}

func pad(n string) string {
	if len(n) >= gutterWidth {
		return n
	}

	return strings.Repeat(" ", gutterWidth-len(n)) + n
}

func (*renderer) styled(text string, face color.Color) string {
	if face == nil {
		return text
	}

	return lipgloss.NewStyle().Foreground(face).Render(text)
}

// fit truncates by cells rather than bytes, and leaves an already-styled line whole when it fits,
// since cutting one can land inside an escape sequence.
func (r *renderer) fit(line string) string {
	if r.opts.Width <= 0 || lipgloss.Width(line) <= r.opts.Width {
		return line
	}

	return r.cut(line, r.opts.Width)
}

const truncationMarker = "…"

// cut resets before the marker so the cut token's color does not leak into it.
func (*renderer) cut(line string, width int) string {
	return ansi.Truncate(line, width-1, "") + "\x1b[m" + lipgloss.NewStyle().Faint(true).Render(truncationMarker)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}
