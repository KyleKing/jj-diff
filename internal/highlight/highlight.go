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
		style, face := p.face(aghighlight.Class(c), base)
		if face != nil {
			style = style.Foreground(face)
		}

		f.class[c] = pen.Of(style)
	}

	return f
}

// Highlighter colors the lines of one set of files. Its lexing is cached per file and safe to
// run in the background, but drawing is not safe for concurrent use, and a changed diff needs a
// new Highlighter.
type Highlighter struct {
	spans   *aghighlight.Cache
	text    map[agdiff.LineRef]string
	palette Palette
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
	converted := diff.Aragonite(files)
	text := make(map[agdiff.LineRef]string)

	for i := range converted.Files {
		for _, line := range converted.Files[i].Lines {
			text[refOf(files[i].Path, line)] = line.Text
		}
	}

	return &Highlighter{spans: aghighlight.NewCache(converted), text: text, palette: p}
}

// Palette is the palette the Highlighter draws in, for a caller building Faces for it.
func (h *Highlighter) Palette() *Palette {
	return &h.palette
}

// Start lexes path in the background and returns a channel closed once its colors are ready.
// Calling it again for the same path answers the same channel.
func (h *Highlighter) Start(path string) <-chan struct{} {
	return h.spans.Start(path)
}

// Paint appends line's content in f to b, lexing its file first if that has not happened.
func (h *Highlighter) Paint(b *strings.Builder, path string, line diff.Line, f *Faces) {
	h.draw(b, path, line, f, true)
}

// Peek appends line's content in f to b without ever lexing: until Start's lexing of path has
// landed it draws the content plain. It reports whether the colors were ready.
func (h *Highlighter) Peek(b *strings.Builder, path string, line diff.Line, f *Faces) bool {
	return h.draw(b, path, line, f, false)
}

func (h *Highlighter) draw(b *strings.Builder, path string, line diff.Line, f *Faces, wait bool) bool {
	content := line.Content
	if content == "" {
		return true
	}

	agLine := diff.AragoniteLine(line)
	if h.text[refOf(path, agLine)] != content {
		f.Base.Write(b, content)

		return true
	}

	var spans []aghighlight.Span

	ready := true
	if wait {
		spans = h.spans.Spans(path, agLine)
	} else {
		spans, ready = h.spans.Peek(path, agLine)
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
