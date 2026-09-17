// Package render turns unified diff text into styled terminal lines. It is the read-only half of
// jj-diff's diff rendering, published for other tools to draw a diff the same way this one does:
// the same parser, the same word-level intra-line highlighting, and the same syntax highlighting.
// Hunk and line selection stay inside jj-diff, since only an editor needs them.
package render

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
	// Gutter is the line-number column, drawn only when Options.LineNumbers is set.
	Gutter color.Color
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
}

// Lines renders diffText as one styled string per terminal row, in file order. Input that parses to
// nothing returns nothing, because a caller has a better empty state than a blank pane.
//
// Input must be in git format, since jj's own default format does not parse.
// Callers run `jj diff --git`.
//
//nolint:gocritic // hugeParam: Options is a value the caller builds once per call.
func Lines(diffText string, opts Options) []string {
	files := diff.Parse(diffText)
	if len(files) == 0 {
		return nil
	}

	r := &renderer{opts: opts}
	if opts.Syntax {
		r.highlighter = highlight.NewWith((&opts.Palette.Syntax).internal())
	}

	var lines []string
	for _, file := range files {
		lines = append(lines, r.file(file)...)
	}

	return lines
}

type renderer struct {
	highlighter *highlight.Highlighter
	opts        Options
}

func (r *renderer) file(file diff.FileChange) []string {
	lines := []string{r.fit(r.styled(file.ChangeType.String()+" "+file.Path, r.opts.Palette.File))}

	for i := range file.Hunks {
		hunk := &file.Hunks[i]
		lines = append(lines, r.fit(r.styled(hunk.Header, r.opts.Palette.Hunk)))

		words := map[int]diff.WordDiffResult{}
		if r.opts.WordDiff {
			words = diff.ComputeHunkWordDiffs(hunk)
		}

		if r.opts.SideBySide {
			lines = append(lines, r.columns(file.Path, hunk.Lines, words)...)

			continue
		}

		for j, line := range hunk.Lines {
			lines = append(lines, r.line(file.Path, line, words[j]))
		}
	}

	return lines
}

// gutterPad is the separator drawn between the two columns.
const gutterPad = " \u2502 "

// columns draws the hunk as two panes. Each pane is fitted on its own, so a long line is cut at its
// own column rather than pushing the other pane off the row.
func (r *renderer) columns(path string, lines []diff.Line, words map[int]diff.WordDiffResult) []string {
	pane := r.paneWidth()
	pairs := diff.PairSides(lines)
	out := make([]string, 0, len(pairs))

	for _, pair := range pairs {
		left := r.pane(path, pair.Left, words[pair.LeftIdx], pane)
		right := r.pane(path, pair.Right, words[pair.RightIdx], pane)
		out = append(out, left+gutterPad+right)
	}

	return out
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

// pane draws one side of a row, padded to width so the separator stays in one column. A nil line is
// the blank opposite an addition or a deletion with no counterpart.
func (r *renderer) pane(path string, line *diff.Line, words diff.WordDiffResult, width int) string {
	if line == nil {
		return strings.Repeat(" ", max(width, 0))
	}

	content := r.content(path, *line, words)
	if r.opts.LineNumbers {
		content = r.styled(sideNumber(*line), r.opts.Palette.Gutter) + content
	}

	styled := r.styled(content, r.face(line.Type))
	if width <= 0 {
		return styled
	}

	if gap := width - lipgloss.Width(styled); gap > 0 {
		return styled + strings.Repeat(" ", gap)
	}

	return ansi.Truncate(styled, width, "")
}

// sideNumber is the one line number that side carries, since a column shows one file's numbering.
func sideNumber(line diff.Line) string {
	n := line.NewLineNum
	if line.Type == diff.LineDeletion {
		n = line.OldLineNum
	}

	return pad(strconv.Itoa(n)) + " "
}

func (r *renderer) line(path string, line diff.Line, words diff.WordDiffResult) string {
	text := line.Type.String() + r.content(path, line, words)
	if r.opts.LineNumbers {
		text = r.styled(gutter(line), r.opts.Palette.Gutter) + text
	}

	return r.fit(r.styled(text, r.face(line.Type)))
}

// content is the line's text with whatever highlighting the options ask for, and no marker.
func (r *renderer) content(path string, line diff.Line, words diff.WordDiffResult) string {
	if spans := sideSpans(line.Type, words); len(spans) > 0 {
		return r.emphasize(line, words)
	}

	if r.highlighter != nil {
		return r.highlighter.HighlightLine(path, line.Content)
	}

	return line.Content
}

// emphasize redraws the line from its word spans, so syntax highlighting is dropped for that line:
// the two cannot both style the same bytes, and which bytes changed is the more useful of the two.
func (r *renderer) emphasize(line diff.Line, words diff.WordDiffResult) string {
	var out strings.Builder

	mark := r.opts.Palette.AddedWord
	if line.Type == diff.LineDeletion {
		mark = r.opts.Palette.RemovedWord
	}

	for _, span := range sideSpans(line.Type, words) {
		if span.Type == diff.SpanEqual || mark == nil {
			out.WriteString(span.Text)

			continue
		}

		out.WriteString(lipgloss.NewStyle().Background(mark).Render(span.Text))
	}

	return out.String()
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

	return ansi.Truncate(line, r.opts.Width, "")
}
