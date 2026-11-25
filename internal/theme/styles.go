// Package theme holds the Catppuccin palette as package-level colors that every component reads
// directly. Init must run before any component builds a style, so the colors are set once at
// startup and treated as immutable afterwards.
package theme

import (
	"image/color"
	"os"
	"sync/atomic"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	aragonite "github.com/kyleking/aragonite/tui/theme"
)

// Exported color variables.
var (
	Primary         color.Color
	Accent          color.Color
	Secondary       color.Color
	Text            color.Color
	Muted           color.Color
	AddedLine       color.Color
	DeletedLine     color.Color
	AddedBg         color.Color
	DeletedBg       color.Color
	WordDiffAddedBg color.Color
	WordDiffDelBg   color.Color
)

// Exported style variables.
var (
	HeaderStyle          lipgloss.Style
	HunkHeaderStyle      lipgloss.Style
	SectionStyle         lipgloss.Style
	KeyStyle             lipgloss.Style
	AdditionStyle        lipgloss.Style
	DeletionStyle        lipgloss.Style
	BorderStyle          lipgloss.Style
	WordDiffAddedStyle   lipgloss.Style
	WordDiffDeletedStyle lipgloss.Style
)

// mutedTintShare keeps line numbers and comments readable without competing with code.
const mutedTintShare = 0.6

// tint returns base moved share of the way toward accent.
func tint(base, accent color.Color, share float64) color.Color {
	const channelMax, byteMax = 0xffff, 0xff

	br, bg, bb, _ := base.RGBA()
	ar, ag, ab, _ := accent.RGBA()

	mix := func(from, to uint32) uint8 {
		return uint8((float64(from)*(1-share) + float64(to)*share) / channelMax * byteMax)
	}

	return color.RGBA{R: mix(br, ar), G: mix(bg, ag), B: mix(bb, ab), A: byteMax}
}

// Profile is the color depth the theme is fitted to: colorprofile.Detect, except that
// COLORTERM=truecolor is honored inside tmux. The program hands the same answer to Bubble Tea so
// the faces and the renderer agree on the depth.
func Profile() colorprofile.Profile { return aragonite.Profile(os.Stdout, os.Environ()) }

// Init initializes all colors and styles for a dark background, or the flavor CATPPUCCIN_THEME
// names, at the terminal's color depth. It never asks the terminal for its background; a program
// that needs it calls InitFor with the answer.
func Init() { InitFor(true) }

// InitFor initializes all colors and styles for a terminal whose background is dark or not, unless
// CATPPUCCIN_THEME names a flavor, at the terminal's color depth.
func InitFor(dark bool) {
	t := For(dark)
	applyTheme(&t, Profile())
}

// applyTheme sets color variables and recomputes all styles, deriving the diff tints so they stay
// distinct once p reduces them to the colors the terminal can show.
func applyTheme(t *Theme, p colorprofile.Profile) {
	Primary = t.Primary
	Accent = t.Accent
	Secondary = t.Secondary
	Text = t.Text
	// The two sides' colors say which side a count or a line is on, so they keep their hue in
	// 256 colors, where the nearest match for Latte's red is a grey.
	AddedLine = aragonite.Hue(t.AddedLine, p)
	DeletedLine = aragonite.Hue(t.DeletedLine, p)

	// Every background comes from the skin, which fits it to p; nothing here paints one of its own.
	applySkin(t, p)

	Muted = tint(Palette.Base, Text, mutedTintShare)

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

	AdditionStyle = lipgloss.NewStyle().
		Foreground(AddedLine)

	DeletionStyle = lipgloss.NewStyle().
		Foreground(DeletedLine)

	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Primary)

	WordDiffAddedStyle = lipgloss.NewStyle().
		Background(WordDiffAddedBg).
		Foreground(Text)

	WordDiffDeletedStyle = lipgloss.NewStyle().
		Background(WordDiffDelBg).
		Foreground(Text)

	generation.Add(1)
}

// generation counts the times the styles were rebuilt, so a component that caches what it drew
// with them knows when that cache went stale.
var generation atomic.Uint64

// Generation is the number of times Init (or a test's theme swap) has rebuilt the styles. A
// component caching rendered rows or pens keys them by it, since Init can run again in tests.
func Generation() uint64 {
	return generation.Load()
}
