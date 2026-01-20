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

// Highlighter provides syntax highlighting for code
type Highlighter struct {
	style *chroma.Style
}

// New creates a new syntax highlighter
func New() *Highlighter {
	// Use a minimal style that works well with terminal colors
	return &Highlighter{
		style: styles.Get("monokai"),
	}
}

// HighlightLine applies syntax highlighting to a single line of code
// filePath is used to detect the language
// Returns the highlighted line with lipgloss styling
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

func (h *Highlighter) detectLexer(filePath string) chroma.Lexer {
	ext := filepath.Ext(filePath)

	// Try to get lexer by filename
	lexer := lexers.Match(filePath)
	if lexer != nil {
		return chroma.Coalesce(lexer)
	}

	// Try to get lexer by extension
	lexer = lexers.Get(ext)
	if lexer != nil {
		return chroma.Coalesce(lexer)
	}

	// Special cases for common extensions
	switch ext {
	case ".go":
		return lexers.Get("go")
	case ".js", ".jsx":
		return lexers.Get("javascript")
	case ".ts", ".tsx":
		return lexers.Get("typescript")
	case ".py":
		return lexers.Get("python")
	case ".rs":
		return lexers.Get("rust")
	case ".c", ".h":
		return lexers.Get("c")
	case ".cpp", ".hpp", ".cc":
		return lexers.Get("cpp")
	case ".java":
		return lexers.Get("java")
	case ".rb":
		return lexers.Get("ruby")
	case ".sh", ".bash":
		return lexers.Get("bash")
	case ".yaml", ".yml":
		return lexers.Get("yaml")
	case ".json":
		return lexers.Get("json")
	case ".toml":
		return lexers.Get("toml")
	case ".md":
		return lexers.Get("markdown")
	case ".html":
		return lexers.Get("html")
	case ".css":
		return lexers.Get("css")
	case ".sql":
		return lexers.Get("sql")
	}

	return nil
}

func (h *Highlighter) styleToken(token chroma.Token) string {
	value := token.Value
	tokenType := token.Type

	// Map chroma token types to lipgloss styles
	// Use subtle colors that don't conflict with diff colors
	style := lipgloss.NewStyle()

	switch {
	case tokenType == chroma.Comment, tokenType == chroma.CommentSingle, tokenType == chroma.CommentMultiline:
		// Comments: muted/soft color
		style = style.Foreground(theme.SoftMutedBg)

	case tokenType == chroma.Keyword, tokenType == chroma.KeywordNamespace, tokenType == chroma.KeywordType:
		// Keywords: accent color (but not too bright)
		style = style.Foreground(theme.Accent).Bold(true)

	case tokenType == chroma.String, tokenType == chroma.LiteralString, tokenType == chroma.LiteralStringDouble:
		// Strings: subtle green (different from diff additions)
		style = style.Foreground(lipgloss.Color("#a6e3a1"))

	case tokenType == chroma.Number, tokenType == chroma.LiteralNumber:
		// Numbers: subtle orange
		style = style.Foreground(lipgloss.Color("#fab387"))

	case tokenType == chroma.Name, tokenType == chroma.NameFunction:
		// Function names: subtle blue
		style = style.Foreground(lipgloss.Color("#89b4fa"))

	case tokenType == chroma.NameClass, tokenType == chroma.NameBuiltin:
		// Class names: subtle yellow
		style = style.Foreground(lipgloss.Color("#f9e2af"))

	case tokenType == chroma.Operator:
		// Operators: text color
		style = style.Foreground(theme.Text)

	default:
		// Default: normal text color
		return value
	}

	return style.Render(value)
}

// IsEnabled returns whether syntax highlighting is available for a file
func (h *Highlighter) IsEnabled(filePath string) bool {
	return h.detectLexer(filePath) != nil
}
