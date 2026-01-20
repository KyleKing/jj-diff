package theme

import "github.com/charmbracelet/lipgloss"

// Exported color variables
var (
	Primary         lipgloss.Color
	Accent          lipgloss.Color
	Secondary       lipgloss.Color
	Text            lipgloss.Color
	SelectedBg      lipgloss.Color
	MutedBg         lipgloss.Color
	SoftMutedBg     lipgloss.Color
	ModalBg         lipgloss.Color
	AddedLine       lipgloss.Color
	DeletedLine     lipgloss.Color
	WordDiffAddedBg lipgloss.Color
	WordDiffDelBg   lipgloss.Color
)

// Exported style variables
var (
	HeaderStyle            lipgloss.Style
	HunkHeaderStyle        lipgloss.Style
	SectionStyle           lipgloss.Style
	KeyStyle               lipgloss.Style
	FooterStyle            lipgloss.Style
	AdditionStyle          lipgloss.Style
	DeletionStyle          lipgloss.Style
	SelectedFocusedStyle   lipgloss.Style
	SelectedUnfocusedStyle lipgloss.Style
	StatusBarStyle         lipgloss.Style
	BorderStyle            lipgloss.Style
	WordDiffAddedStyle     lipgloss.Style
	WordDiffDeletedStyle   lipgloss.Style
)

// Init detects the appropriate theme and initializes all colors and styles
func Init() {
	t := Detect()
	applyTheme(t)
}

// applyTheme sets color variables and recomputes all styles
func applyTheme(t Theme) {
	Primary = t.Primary
	Accent = t.Accent
	Secondary = t.Secondary
	Text = t.Text
	SelectedBg = t.SelectedBg
	MutedBg = t.MutedBg
	SoftMutedBg = t.SoftMutedBg
	ModalBg = t.ModalBg
	AddedLine = t.AddedLine
	DeletedLine = t.DeletedLine
	WordDiffAddedBg = t.WordDiffAddedBg
	WordDiffDelBg = t.WordDiffDelBg

	HeaderStyle = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	HunkHeaderStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	SectionStyle = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	KeyStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	FooterStyle = lipgloss.NewStyle().
		Background(SoftMutedBg).
		Foreground(Text).
		Padding(0, 1)

	AdditionStyle = lipgloss.NewStyle().
		Foreground(AddedLine)

	DeletionStyle = lipgloss.NewStyle().
		Foreground(DeletedLine)

	SelectedFocusedStyle = lipgloss.NewStyle().
		Background(SelectedBg).
		Foreground(Text).
		Bold(true)

	SelectedUnfocusedStyle = lipgloss.NewStyle().
		Background(MutedBg).
		Foreground(Text)

	StatusBarStyle = lipgloss.NewStyle().
		Background(SoftMutedBg).
		Foreground(Text).
		Padding(0, 1)

	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Primary)

	WordDiffAddedStyle = lipgloss.NewStyle().
		Background(WordDiffAddedBg).
		Foreground(AddedLine)

	WordDiffDeletedStyle = lipgloss.NewStyle().
		Background(WordDiffDelBg).
		Foreground(DeletedLine)
}

// PaneStyle returns a dynamic border style for panes
func PaneStyle(width, height int, focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder())

	if focused {
		return style.BorderForeground(Primary)
	}
	return style.BorderForeground(MutedBg)
}
