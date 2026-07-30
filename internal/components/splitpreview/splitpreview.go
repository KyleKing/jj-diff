// Package splitpreview renders the read-only summary of a pending multi-way split, one row per tag
// with its destination and the amount of work assigned to it.
package splitpreview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/theme"
)

// SplitTag is the single character a hunk carries while a multi-way split is being assembled. It is
// declared here rather than shared with the parent model so this component imports nothing from it.
type SplitTag rune

// DestinationType separates a destination that already exists from one that will be created.
type DestinationType int

// Destinations a tag can be sent to. DestExistingRevision is the zero value, so a DestinationSpec
// that was never filled in reads as targeting an existing revision.
const (
	DestExistingRevision DestinationType = iota
	DestNewCommit
)

// DestinationSpec is where one tag's hunks land. ChangeID is empty for DestNewCommit, where
// Description becomes the message of the commit that gets created.
type DestinationSpec struct {
	Type        DestinationType
	ChangeID    string
	Description string
}

// SplitSummary is one row of the preview. FileCount and HunkCount are what the tag currently holds,
// so they are recomputed by the parent rather than tracked here.
type SplitSummary struct {
	Tag         SplitTag
	Destination DestinationSpec
	FileCount   int
	HunkCount   int
}

// Model is the preview modal. It only displays what the parent hands it and has no cursor of its own.
type Model struct {
	summaries []SplitSummary
	visible   bool
}

// New returns a hidden preview with no summaries.
func New() Model {
	return Model{
		summaries: []SplitSummary{},
		visible:   false,
	}
}

// SetSummaries replaces the rows. The parent recomputes them, because the counts go stale as soon as
// a hunk's tag changes.
func (m *Model) SetSummaries(summaries []SplitSummary) {
	m.summaries = summaries
}

// Show opens the preview.
func (m *Model) Show() {
	m.visible = true
}

// Hide closes the preview.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether the preview is open, which is how the parent decides to route keys here.
func (m *Model) IsVisible() bool {
	return m.visible
}

// View renders the preview centered in the given terminal size, returning the empty string while
// hidden.
func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := width - 20
	if modalWidth < 60 {
		modalWidth = 60
	}
	if modalWidth > 100 {
		modalWidth = 100
	}

	var lines []string
	lines = append(lines, styleHeader("Split Preview", modalWidth))
	lines = append(lines, "")

	if len(m.summaries) == 0 {
		lines = append(lines, styleInfo("No tags assigned", modalWidth))
	} else {
		for _, summary := range m.summaries {
			lines = append(lines, m.renderSummaryLine(summary, modalWidth))
		}
	}

	lines = append(lines, "")
	lines = append(lines, styleFooter("Enter: Apply | e: Edit | Esc: Cancel", modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m Model) renderSummaryLine(summary SplitSummary, width int) string {
	var destStr string
	if summary.Destination.Type == DestNewCommit {
		destStr = fmt.Sprintf("NEW: %s", truncate(summary.Destination.Description, 30))
	} else {
		destStr = fmt.Sprintf(
			"%s (%s)",
			summary.Destination.ChangeID,
			truncate(summary.Destination.Description, 20),
		)
	}

	line := fmt.Sprintf("[%s] → %s | %d files, %d hunks",
		string(summary.Tag),
		destStr,
		summary.FileCount,
		summary.HunkCount,
	)

	return "  " + truncateOrPad(line, width-4)
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func styleFooter(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.SoftMutedBg).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func styleInfo(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.SoftMutedBg).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func truncate(text string, width int) string {
	if len(text) > width {
		if width > 3 {
			return text[:width-3] + "..."
		}

		return text[:width]
	}

	return text
}

func truncateOrPad(text string, width int) string {
	visibleLen := len(text)
	if visibleLen > width {
		if width > 3 {
			return text[:width-3] + "..."
		}

		return text[:width]
	}

	return text + strings.Repeat(" ", width-visibleLen)
}

func renderModal(content string, termWidth, termHeight int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 2)

	modal := borderStyle.Render(content)

	overlay := lipgloss.Place(
		termWidth,
		termHeight,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)

	return overlay
}
