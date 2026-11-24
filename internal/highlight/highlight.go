// Package highlight colors diff content by the byte-range spans aragonite's highlight package
// lexes, once per file, so drawing a frame costs a lookup per line rather than a lexer pass.
//
// Lexing a large file takes hundreds of milliseconds, so a screen asks for it in the background
// with Start and draws with Peek, which answers plain text until the spans land, rather than
// blocking its first frame. Colors are drawn through pens made once per base style (Faces), not
// through lipgloss's renderer once per token.
package highlight

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	agdiff "github.com/kyleking/aragonite/diff"
	aghighlight "github.com/kyleking/aragonite/highlight"
	"github.com/kyleking/aragonite/tui/pen"
	"github.com/kyleking/aragonite/tui/skin"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Palette is the face each token class renders in. A nil color leaves that class unstyled, which is
// what a caller with no color for it should pass rather than a guess.
type Palette struct {
	Comment  color.Color
	Keyword  color.Color
	String   color.Color
	Number   color.Color
	Name     color.Color
	Type     color.Color
	Operator color.Color
}

// ThemePalette is the palette built from the process-wide theme, which is what the diff editor
// itself renders with.
func ThemePalette() Palette {
	return Palette{
		Comment:  theme.Muted,
		Keyword:  theme.Accent,
		String:   theme.AddedLine,
		Number:   theme.Secondary,
		Name:     theme.Primary,
		Type:     theme.Accent,
		Operator: theme.Text,
	}
}

// classes is one past the highest aragonite class, the size of a Faces table.
const classes = int(aghighlight.Escape) + 1

// Faces is one base style and every token class drawn on it, as pens, so a line costs no style
// work beyond writing escape sequences. Build one per base a screen draws on and keep it.
type Faces struct {
	Base  pen.Pen
	class [classes]pen.Pen
}

// Faces returns the pens for base under p.
//
//nolint:gocritic // hugeParam: a style is a value, and Faces is built once per base.
func (p *Palette) Faces(base lipgloss.Style) *Faces {
	f := &Faces{Base: pen.Of(base)}

	for c := range classes {
		style, face := p.face(aghighlight.Class(c), lipgloss.NewStyle())
		if face != nil {
			style = style.Foreground(face)
		}

		// skin.On drops the token's color where base is reverse video, so a selected row at 16
		// colors is one even bar rather than a white bar with colored blocks in it.
		f.class[c] = pen.Of(skin.On(base, style))
	}

	return f
}

// Highlighter colors the lines of one set of files. Its lexing is cached per file section and
// safe to run in the background, but drawing is not safe for concurrent use, and a changed diff
// needs a new Highlighter.
//
// A section is one entry of the files it was built over, named by its index. One diff can hold the
// same path more than once (`git log -p` across commits), and two such sections share line numbers
// while holding different text, so each is lexed and looked up on its own rather than by path.
type Highlighter struct {
	palette  Palette
	sections []section
}

// section is one file of the diff with its own span cache and the text each of its lines holds.
type section struct {
	spans *aghighlight.Cache
	text  map[agdiff.LineRef]string
	path  string
}

// New returns a Highlighter over files drawing in the process-wide theme.
func New(files ...diff.FileChange) *Highlighter {
	return NewWith(ThemePalette(), files...)
}

// NewWith returns a Highlighter over files drawing in p, for a caller outside this program that has
// its own palette and never calls theme.Init.
//
//nolint:gocritic // hugeParam: a palette is built once, at startup.
func NewWith(p Palette, files ...diff.FileChange) *Highlighter {
	sections := make([]section, len(files))

	for i := range files {
		converted := diff.Aragonite(files[i : i+1])
		text := make(map[agdiff.LineRef]string)

		for _, line := range converted.Files[0].Lines {
			text[refOf(files[i].Path, line)] = line.Text
		}

		sections[i] = section{path: files[i].Path, spans: aghighlight.NewCache(converted), text: text}
	}

	return &Highlighter{sections: sections, palette: p}
}

// Palette is the palette the Highlighter draws in, for a caller building Faces for it.
func (h *Highlighter) Palette() *Palette {
	return &h.palette
}

// Start lexes the file at index file in the background and returns a channel closed once its
// colors are ready. Calling it again for the same file answers the same channel; an index outside
// the files answers a closed channel, as there is nothing to lex.
func (h *Highlighter) Start(file int) <-chan struct{} {
	if file < 0 || file >= len(h.sections) {
		done := make(chan struct{})
		close(done)

		return done
	}

	sec := &h.sections[file]

	return sec.spans.Start(sec.path)
}

// Paint appends line's content in f to b, lexing its file first if that has not happened.
func (h *Highlighter) Paint(b *strings.Builder, file int, line diff.Line, f *Faces) {
	h.draw(b, file, line, f, true)
}

// Peek appends line's content in f to b without ever lexing: until Start's lexing of file has
// landed it draws the content plain. It reports whether the colors were ready.
func (h *Highlighter) Peek(b *strings.Builder, file int, line diff.Line, f *Faces) bool {
	return h.draw(b, file, line, f, false)
}

func (h *Highlighter) draw(b *strings.Builder, file int, line diff.Line, f *Faces, wait bool) bool {
	content := line.Content
	if content == "" {
		return true
	}

	agLine := diff.AragoniteLine(line)
	if file < 0 || file >= len(h.sections) || h.sections[file].text[refOf(h.sections[file].path, agLine)] != content {
		f.Base.Write(b, content)

		return true
	}

	sec := &h.sections[file]

	var spans []aghighlight.Span

	ready := true
	if wait {
		spans = sec.spans.Spans(sec.path, agLine)
	} else {
		spans, ready = sec.spans.Peek(sec.path, agLine)
	}

	at := 0

	for _, span := range spans {
		if span.From > at {
			f.Base.Write(b, content[at:span.From])
		}

		f.class[span.Class].Write(b, content[span.From:span.To])
		at = span.To
	}

	if at < len(content) {
		f.Base.Write(b, content[at:])
	}

	return ready
}

func refOf(path string, line agdiff.Line) agdiff.LineRef {
	return agdiff.LineRef{Path: path, Kind: line.Kind, Old: line.Old, New: line.New}
}

//nolint:gocritic // hugeParam: a style is a value, and this runs once per Faces.
func (p *Palette) face(class aghighlight.Class, base lipgloss.Style) (lipgloss.Style, color.Color) {
	style := base

	switch class {
	case aghighlight.Comment:
		return style, p.Comment
	case aghighlight.Keyword:
		return style.Bold(true), p.Keyword
	case aghighlight.String, aghighlight.Escape:
		return style, p.String
	case aghighlight.Number, aghighlight.Constant:
		return style, p.Number
	case aghighlight.Name, aghighlight.Function:
		return style, p.Name
	case aghighlight.Type, aghighlight.Namespace:
		return style, p.Type
	case aghighlight.Operator:
		return style, p.Operator
	case aghighlight.Plain, aghighlight.Punctuation, aghighlight.Field:
	}

	return style, nil
}
