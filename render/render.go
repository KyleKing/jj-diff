// Package render turns unified diff text into styled terminal lines. It is the read-only half of
// jj-diff's diff rendering, published for other tools to draw a diff the same way this one does:
// the same parser, the same word-level intra-line highlighting, and the same syntax highlighting.
// Hunk and line selection stay inside jj-diff, since only an editor needs them.
//
// # Drawing plain first
//
// Syntax colors cost a lexer pass over every file a diff touches: tens of milliseconds for a
// small change and over a second for a large one. Lines and Document.Rows lex on the first draw
// of a file and block until it is done, which is right for a one-shot render and wrong for a
// screen that must answer the next key. Such a screen draws with Peek, which never lexes, and
// starts the lexing in the background with Start:
//
//	doc := render.New(text, opts) // opts.Syntax set
//	rows, colored := doc.Peek(0, doc.Len())
//	if !colored {
//		done := doc.Start()
//		cmd = func() tea.Msg { <-done; return coloredMsg{doc} }
//	}
//
// and, when coloredMsg arrives, draws the same rows again with Peek (or Rows), now in color and
// without lexing. The colored rows hold the same text in the same places as the plain ones, so a
// cursor or scroll offset kept across the swap stays on the same line.
//
// Only the lexing runs in the background. A Document is not safe for concurrent use, so the
// command waits on Start's channel and the drawing stays on the update loop.
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
	"github.com/kyleking/aragonite/tui/pen"
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
	started  chan struct{}
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

// Start lexes every file of the diff in the background and returns a channel closed once all of
// their colors are ready, after which Peek draws in color and Rows never blocks on the lexer.
// Calling it again answers the same channel. Without Options.Syntax there is
// nothing to lex, and the channel is already closed. The lexing is safe to run while the caller
// draws; waiting on the channel is the only part of the Document a command should touch.
func (d *Document) Start() <-chan struct{} {
	if d.started != nil {
		return d.started
	}

	done := make(chan struct{})
	d.started = done

	h := d.renderer.highlighter
	if h == nil {
		close(done)

		return done
	}

	files := d.drawnFiles()
	waits := make([]<-chan struct{}, 0, len(files))

	for _, file := range files {
		waits = append(waits, h.Start(file))
	}

	go func() {
		for _, wait := range waits {
			<-wait
		}

		close(done)
	}()

	return done
}

// drawnFiles is the index of each file section with a hunk drawn line by line, skipping a
// collapsed generated file or a folded hunk, whose lines never reach the screen and so are never
// worth a lexer pass. It names sections rather than paths, because one diff can hold the same path
// twice (a `git log -p` across commits) and each section is lexed on its own.
func (d *Document) drawnFiles() []int {
	var files []int

	seen := map[int]bool{}

	for i := range d.segments {
		seg := &d.segments[i]
		if seg.hunk < 0 || seg.note != "" || seen[seg.file] {
			continue
		}

		seen[seg.file] = true
		files = append(files, seg.file)
	}

	return files
}

// Peek renders rows [from, to) like Rows, except that it never lexes: a line whose file Start has
// not finished lexing is drawn plain, in its line face, rather than waited for. The bool reports
// whether every row drawn had its colors, so a caller that sees false redraws once Start's channel
// closes. Without Options.Syntax every row is as colored as it will get, and it is true.
func (d *Document) Peek(from, to int) ([]string, bool) {
	r := d.renderer
	r.peek, r.uncolored = true, false

	defer func() { r.peek = false }()

	rows := d.Rows(from, to)

	return rows, !r.uncolored
}

func (d *Document) row(seg *segment, row int) string {
	r := d.renderer
	file := &d.files[seg.file]

	if seg.hunk < 0 {
		if row == 0 {
			return r.fit(r.palettePens().file.draw(file.ChangeType.String() + " " + file.Path))
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
		return r.column(seg.file, seg.pairs[row-1], words)
	}

	return r.line(seg.file, hunk.Lines[row-1], words[row-1])
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
	// pens holds each line kind's pens, on its wash ([kind][1]) and bare ([kind][0]), built the
	// first time a row of that kind draws, since a lipgloss render per run is most of a row's cost.
	pens  [kinds][2]*kindPens
	faces *facePens
	opts  Options
	// peek draws without lexing, for Document.Peek, and uncolored records that a line was drawn
	// without the colors it will have once lexed.
	peek, uncolored bool
}

// The line kinds, indexing a renderer's pens.
const (
	kindContext = iota
	kindAdded
	kindRemoved
	kinds
)

func kindIndex(kind diff.LineType) int {
	switch kind {
	case diff.LineAddition:
		return kindAdded
	case diff.LineDeletion:
		return kindRemoved
	case diff.LineContext:
	}

	return kindContext
}

// kindPens is what one line kind draws with on one base.
type kindPens struct {
	base   pen.Pen
	syntax *highlight.Faces
	mark   pen.Pen
	sign   pen.Pen
	number pen.Pen
	marked bool
}

// facePen draws in one palette color, or leaves text untouched where the palette has none.
type facePen struct {
	pen pen.Pen
	set bool
}

func facePenOf(c color.Color) facePen {
	if c == nil {
		return facePen{}
	}

	return facePen{pen: pen.Of(lipgloss.NewStyle().Foreground(c)), set: true}
}

func (f *facePen) draw(text string) string {
	if !f.set {
		return text
	}

	return f.pen.Render(text)
}

// facePens is a pen for each palette color drawn on its own, and the faint the notes and the
// truncation marker draw in.
type facePens struct {
	marker                      string
	faint                       pen.Pen
	file, hunk, gutter, context facePen
	side                        [kinds]facePen
}

func (r *renderer) palettePens() *facePens {
	if r.faces != nil {
		return r.faces
	}

	p := &r.opts.Palette
	faint := pen.Of(lipgloss.NewStyle().Faint(true))
	r.faces = &facePens{
		file: facePenOf(p.File), hunk: facePenOf(p.Hunk), gutter: facePenOf(p.Gutter), context: facePenOf(p.Context),
		side:   [kinds]facePen{facePenOf(p.Context), facePenOf(p.Added), facePenOf(p.Removed)},
		faint:  faint,
		marker: faint.Render(truncationMarker),
	}

	return r.faces
}

// kindPens answers kind's pens on its wash when onBase is set, or on no style at all.
func (r *renderer) kindPens(kind diff.LineType, onBase bool) *kindPens {
	k, b := kindIndex(kind), 0
	if onBase {
		b = 1
	}

	if got := r.pens[k][b]; got != nil {
		return got
	}

	base := lipgloss.NewStyle()
	if onBase {
		base = r.base(kind)
	}

	kp := &kindPens{base: pen.Of(base)}

	if r.highlighter != nil {
		kp.syntax = r.highlighter.Palette().Faces(base)
	}

	mark := r.opts.Palette.AddedWord
	if kind == diff.LineDeletion {
		mark = r.opts.Palette.RemovedWord
	}

	if mark != nil {
		kp.mark, kp.marked = pen.Of(base.Background(mark)), true
	}

	kp.sign = pen.Of(r.signStyle(kind))
	kp.number = pen.Of(r.numberStyle(kind))
	r.pens[k][b] = kp

	return kp
}

// hunkLabelGap separates a hunk's function from its line range.
const hunkLabelGap = "  "

func (r *renderer) hunkHead(hunk *diff.Hunk) string {
	if !r.opts.HunkLabels {
		return r.fit(r.palettePens().hunk.draw(hunk.Header))
	}

	span := r.palettePens().gutter.draw(hunk.Range())
	if section := hunk.Section(); section != "" {
		span = r.palettePens().hunk.draw(section) + hunkLabelGap + span
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
	f := r.palettePens()

	return r.fit(f.faint.Render(f.context.draw(text)))
}

// gutterPad is the separator drawn between the two columns.
const gutterPad = " \u2502 "

// column draws one row of two panes. Each pane is fitted on its own, so a long line is cut at its
// own column rather than pushing the other pane off the row.
func (r *renderer) column(file int, pair diff.SidePair, words map[int]diff.WordDiffResult) string {
	pane := r.paneWidth()
	left := r.pane(file, pair.Left, false, words[pair.LeftIdx], pane)
	right := r.pane(file, pair.Right, true, words[pair.RightIdx], r.rightWidth(pane))

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
func (r *renderer) pane(file int, line *diff.Line, isRight bool, words diff.WordDiffResult, width int) string {
	if line == nil {
		return strings.Repeat(" ", max(width, 0))
	}

	if r.restyled() {
		return r.washed(r.numbers(sideNumber(*line, isRight), line.Type)+r.sign(line.Type), file, *line, words, width)
	}

	content := r.content(file, *line, words, false)
	if r.opts.LineNumbers {
		content = r.palettePens().gutter.draw(sideNumber(*line, isRight)) + content
	}

	styled := r.palettePens().side[kindIndex(line.Type)].draw(content)
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

func (r *renderer) line(file int, line diff.Line, words diff.WordDiffResult) string {
	if r.restyled() {
		lead := r.numbers(gutter(line), line.Type) + r.sign(line.Type)

		return r.washed(lead, file, line, words, r.opts.Width)
	}

	text := line.Type.String() + r.content(file, line, words, false)
	if r.opts.LineNumbers {
		text = r.palettePens().gutter.draw(gutter(line)) + text
	}

	return r.fit(r.palettePens().side[kindIndex(line.Type)].draw(text))
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
	return r.kindPens(kind, true).sign.Render(r.signGlyph(kind))
}

func (r *renderer) signGlyph(kind diff.LineType) string {
	switch {
	case !r.opts.SignBar:
		return kind.String()
	case kind == diff.LineContext:
		return " "
	}

	return skin.SignBar
}

func (r *renderer) signStyle(kind diff.LineType) lipgloss.Style {
	base := r.base(kind)

	if r.opts.SignBar && kind != diff.LineContext {
		if face := r.face(kind); face != nil {
			base = base.Foreground(face)
		}
	}

	return base
}

// numbers is the line-number gutter on the line's wash, or nothing when the options leave it off.
func (r *renderer) numbers(text string, kind diff.LineType) string {
	if !r.opts.LineNumbers {
		return ""
	}

	return r.kindPens(kind, true).number.Render(text)
}

func (r *renderer) numberStyle(kind diff.LineType) lipgloss.Style {
	style := r.base(kind)
	if r.opts.Palette.Gutter != nil {
		style = style.Foreground(r.opts.Palette.Gutter)
	}

	return style
}

// washSGR is about what the escape sequences around a row's padding run add, which sizes the row.
const washSGR = "\x1b[38;2;255;255;255;48;2;255;255;255m\x1b[m"

// washed draws lead and the line's content on its wash, cut or padded to width so the tint runs
// edge to edge. A zero width leaves the line its natural length.
func (r *renderer) washed(lead string, file int, line diff.Line, words diff.WordDiffResult, width int) string {
	text := lead + r.content(file, line, words, true)

	if width <= 0 {
		return text
	}

	cells := lipgloss.Width(text)
	if cells > width {
		return r.cut(text, width)
	}

	var out strings.Builder

	out.Grow(len(text) + width - cells + len(washSGR))
	out.WriteString(text)

	// A line that fills the width still closes with an empty run on its wash, as it always has.
	if base := r.kindPens(line.Type, true).base; cells == width {
		base.Write(&out, "")
	} else {
		base.Spaces(&out, width-cells)
	}

	return out.String()
}

// content is the line's text with whatever highlighting the options ask for, and no marker. With
// onBase every part of it is drawn on the line's wash, so the wash is not broken by a token's
// reset; without, it is drawn on no style, for an older-style line whose face goes around it.
func (r *renderer) content(file int, line diff.Line, words diff.WordDiffResult, onBase bool) string {
	if spans := sideSpans(line.Type, words); len(spans) > 0 {
		return r.emphasize(line, words, onBase)
	}

	if r.highlighter != nil {
		var out strings.Builder

		faces := r.kindPens(line.Type, onBase).syntax
		if !r.peek {
			r.highlighter.Paint(&out, file, line, faces)
		} else if !r.highlighter.Peek(&out, file, line, faces) {
			r.uncolored = true
		}

		return out.String()
	}

	if r.restyled() {
		return r.kindPens(line.Type, onBase).base.Render(line.Content)
	}

	return line.Content
}

// emphasize redraws the line from its word spans, so syntax highlighting is dropped for that line:
// the two cannot both style the same bytes, and which bytes changed is the more useful of the two.
func (r *renderer) emphasize(line diff.Line, words diff.WordDiffResult, onBase bool) string {
	var out strings.Builder

	kp := r.kindPens(line.Type, onBase)

	for _, span := range sideSpans(line.Type, words) {
		if span.Type == diff.SpanEqual || !kp.marked {
			r.plain(&out, span.Text, kp)

			continue
		}

		kp.mark.Write(&out, span.Text)
	}

	return out.String()
}

// plain draws unmarked text on its base, or leaves it bare for a line drawn the older way, whose
// face is applied around the whole line.
func (r *renderer) plain(out *strings.Builder, text string, kp *kindPens) {
	if !r.restyled() {
		out.WriteString(text)

		return
	}

	kp.base.Write(out, text)
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
func (r *renderer) cut(line string, width int) string {
	return ansi.Truncate(line, width-1, "") + "\x1b[m" + r.palettePens().marker
}

func plural(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}
