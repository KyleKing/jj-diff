// Package highlight applies chroma syntax highlighting to diff content, picking a lexer from the
// file path. Highlighting is best effort: a path with no known lexer renders unstyled.
package highlight

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/theme"
)

// Highlighter provides syntax highlighting for code.
type Highlighter struct {
	style *chroma.Style
}

// New creates a new syntax highlighter.
func New() *Highlighter {
	// Use a minimal style that works well with terminal colors
	return &Highlighter{
		style: styles.Get("monokai"),
	}
}

// HighlightLine applies syntax highlighting to a single line of code
// filePath is used to detect the language
// Returns the highlighted line with lipgloss styling.
func (h *Highlighter) HighlightLine(filePath, line string) string {
	if line == "" {
		return line
	}

	// Detect lexer from file extension
	lexer := h.detectLexer(filePath)
	if lexer == nil {
		return line
	}

	// Tokenize the line
	tokens, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}

	// Build styled output
	var result strings.Builder
	for _, token := range tokens.Tokens() {
		result.WriteString(h.styleToken(token))
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

func (*Highlighter) styleToken(token chroma.Token) string {
	value := token.Value
	tokenType := token.Type

	// Map chroma token types to lipgloss styles
	// Use subtle colors that don't conflict with diff colors
	style := lipgloss.NewStyle()

	switch tokenType {
	case chroma.Comment, chroma.CommentSingle, chroma.CommentMultiline:
		// Comments: muted/soft color
		style = style.Foreground(theme.SoftMutedBg)

	case chroma.Keyword, chroma.KeywordNamespace, chroma.KeywordType:
		// Keywords: accent color (but not too bright)
		style = style.Foreground(theme.Accent).Bold(true)

	case chroma.LiteralString, chroma.LiteralStringDouble:
		// Strings: subtle green (different from diff additions)
		style = style.Foreground(lipgloss.Color("#a6e3a1"))

	case chroma.LiteralNumber:
		// Numbers: subtle orange
		style = style.Foreground(lipgloss.Color("#fab387"))

	case chroma.Name, chroma.NameFunction:
		// Function names: subtle blue
		style = style.Foreground(lipgloss.Color("#89b4fa"))

	case chroma.NameClass, chroma.NameBuiltin:
		// Class names: subtle yellow
		style = style.Foreground(lipgloss.Color("#f9e2af"))

	case chroma.Operator:
		// Operators: text color
		style = style.Foreground(theme.Text)

	default:
		// Default: normal text color
		return value
	}

	return style.Render(value)
}

// IsEnabled returns whether syntax highlighting is available for a file.
func (h *Highlighter) IsEnabled(filePath string) bool {
	return h.detectLexer(filePath) != nil
}
