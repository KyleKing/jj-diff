// Package highlight colors diff content by the byte-range spans aragonite's highlight package
// lexes, once per file, so drawing a frame costs a lookup per line rather than a lexer pass.
package highlight

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	agdiff "github.com/kyleking/aragonite/diff"
	aghighlight "github.com/kyleking/aragonite/highlight"

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

// Highlighter colors the lines of one set of files. Its lexing is cached per file, so it is not
// safe for concurrent use and a changed diff needs a new Highlighter.
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

// Line draws line's content on base. A line whose content differs from the diff's own under its numbers
// (hidden whitespace changes) draws unhighlighted rather than with the spans of the text it replaced.
func (h *Highlighter) Line(path string, line diff.Line, base *lipgloss.Style) string {
	content := line.Content
	if content == "" {
		return ""
	}

	agLine := diff.AragoniteLine(line)
	if h.text[refOf(path, agLine)] != content {
		return base.Render(content)
	}

	spans := h.spans.Spans(path, agLine)
	if len(spans) == 0 {
		return base.Render(content)
	}

	var out strings.Builder

	at := 0

	for _, span := range spans {
		if span.From > at {
			out.WriteString(base.Render(content[at:span.From]))
		}

		style, face := h.face(span.Class, base)
		if face != nil {
			style = style.Foreground(face)
		}

		out.WriteString(style.Render(content[span.From:span.To]))
		at = span.To
	}

	if at < len(content) {
		out.WriteString(base.Render(content[at:]))
	}

	return out.String()
}

func refOf(path string, line agdiff.Line) agdiff.LineRef {
	return agdiff.LineRef{Path: path, Kind: line.Kind, Old: line.Old, New: line.New}
}

func (h *Highlighter) face(class aghighlight.Class, base *lipgloss.Style) (lipgloss.Style, color.Color) {
	style := *base

	switch class {
	case aghighlight.Comment:
		return style, h.palette.Comment
	case aghighlight.Keyword:
		return style.Bold(true), h.palette.Keyword
	case aghighlight.String, aghighlight.Escape:
		return style, h.palette.String
	case aghighlight.Number, aghighlight.Constant:
		return style, h.palette.Number
	case aghighlight.Name, aghighlight.Function:
		return style, h.palette.Name
	case aghighlight.Type, aghighlight.Namespace:
		return style, h.palette.Type
	case aghighlight.Operator:
		return style, h.palette.Operator
	case aghighlight.Plain, aghighlight.Punctuation, aghighlight.Field:
	}

	return style, nil
}
