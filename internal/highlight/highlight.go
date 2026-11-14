// Package highlight applies chroma syntax highlighting to diff content, picking a lexer from the
// file path. Highlighting is best effort: a path with no known lexer renders unstyled.
package highlight

import (
	"image/color"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

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

// Highlighter provides syntax highlighting for code.
type Highlighter struct {
	style   *chroma.Style
	palette Palette
}

// New creates a syntax highlighter drawing in the process-wide theme.
func New() *Highlighter {
	return NewWith(ThemePalette())
}

// NewWith creates a syntax highlighter drawing in p, for a caller outside this program that has its
// own palette and never calls theme.Init.
func NewWith(p Palette) *Highlighter { //nolint:gocritic // a palette is built once, at startup.
	// Use a minimal style that works well with terminal colors
	return &Highlighter{
		style:   styles.Get("monokai"),
		palette: p,
	}
}

// HighlightLine applies syntax highlighting to a single line of code
// filePath is used to detect the language
// Returns the highlighted line with lipgloss styling.
func (h *Highlighter) HighlightLine(filePath, line string) string {
	base := lipgloss.NewStyle()

	return h.HighlightLineOn(filePath, line, &base)
}

// HighlightLineOn draws every token on base, because an outer style does not survive the reset after the first token.
func (h *Highlighter) HighlightLineOn(filePath, line string, base *lipgloss.Style) string {
	if line == "" {
		return line
	}

	// Detect lexer from file extension
	lexer := h.detectLexer(filePath)
	if lexer == nil {
		return line
	}

	// Tokenize the line
	tokens, err := lexer.Tokenise(nil, line+"\n")
	if err != nil {
		return line
	}

	// Build styled output
	var result strings.Builder
	for _, token := range tokens.Tokens() {
		token.Value = strings.TrimSuffix(token.Value, "\n")
		if token.Value == "" {
			continue
		}

		result.WriteString(h.styleToken(token, base))
	}

	return result.String()
}

// Chroma lexer names shared by more than one extension.
const (
	bashLexer       = "bash"
	cLexer          = "c"
	cppLexer        = "cpp"
	javascriptLexer = "javascript"
	typescriptLexer = "typescript"
	yamlLexer       = "yaml"
)

// lexerByExtension names the chroma lexer to fall back to for extensions chroma's own filename and
// extension lookups miss.
var lexerByExtension = map[string]string{
	".bash": bashLexer,
	".c":    cLexer,
	".cc":   cppLexer,
	".cpp":  cppLexer,
	".css":  "css",
	".go":   "go",
	".h":    cLexer,
	".hpp":  cppLexer,
	".html": "html",
	".java": "java",
	".js":   javascriptLexer,
	".json": "json",
	".jsx":  javascriptLexer,
	".md":   "markdown",
	".py":   "python",
	".rb":   "ruby",
	".rs":   "rust",
	".sh":   bashLexer,
	".sql":  "sql",
	".toml": "toml",
	".ts":   typescriptLexer,
	".tsx":  typescriptLexer,
	".yaml": yamlLexer,
	".yml":  yamlLexer,
}

//nolint:ireturn // chroma.Lexer is the interface the lexer registry hands back; there is no concrete type to return.
func (*Highlighter) detectLexer(filePath string) chroma.Lexer {
	if lexer := lexers.Match(filePath); lexer != nil {
		return chroma.Coalesce(lexer)
	}

	ext := filepath.Ext(filePath)
	if lexer := lexers.Get(ext); lexer != nil {
		return chroma.Coalesce(lexer)
	}

	if name, ok := lexerByExtension[ext]; ok {
		return lexers.Get(name)
	}

	return nil
}

func (h *Highlighter) styleToken(token chroma.Token, base *lipgloss.Style) string {
	value := token.Value
	tokenType := token.Type

	// Map chroma token types to lipgloss styles
	// Use subtle colors that don't conflict with diff colors
	style := *base

	var face color.Color

	switch {
	case tokenType.InCategory(chroma.Comment):
		face = h.palette.Comment

	case tokenType.InCategory(chroma.Keyword):
		style, face = style.Bold(true), h.palette.Keyword

	case tokenType.InSubCategory(chroma.LiteralString):
		face = h.palette.String

	case tokenType.InSubCategory(chroma.LiteralNumber):
		face = h.palette.Number

	case tokenType == chroma.Name, tokenType == chroma.NameFunction:
		face = h.palette.Name

	case tokenType == chroma.NameClass, tokenType == chroma.NameBuiltin:
		face = h.palette.Type

	case tokenType.InCategory(chroma.Operator):
		face = h.palette.Operator

	case tokenType == chroma.GenericHeading, tokenType == chroma.GenericSubheading:
		style, face = style.Bold(true), h.palette.Name

	default:
		return style.Render(value)
	}

	if face == nil {
		return style.Render(value)
	}

	return style.Foreground(face).Render(value)
}

// IsEnabled returns whether syntax highlighting is available for a file.
func (h *Highlighter) IsEnabled(filePath string) bool {
	return h.detectLexer(filePath) != nil
}
