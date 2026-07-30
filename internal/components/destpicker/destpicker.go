// Package destpicker lists candidate revisions so the user can choose where changes move.
// The parent model loads the revisions, routes keys here while the picker is visible, and
// reads the highlighted entry back out on confirmation.
package destpicker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Model is the destination picker. Mutators take a pointer receiver, so a parent holding it
// by value must keep the same field rather than a copy.
type Model struct {
	revisions []jj.RevisionEntry
	selected  int
	visible   bool
}

// New returns a hidden picker with no revisions, so SetRevisions must run before Show.
func New() Model {
	return Model{
		revisions: []jj.RevisionEntry{},
		selected:  0,
		visible:   false,
	}
}

// SetRevisions replaces the candidate list, keeping the cursor where it is when the new list
// is long enough and falling back to the first row otherwise. The slice is retained, not copied.
func (m *Model) SetRevisions(revisions []jj.RevisionEntry) {
	m.revisions = revisions
	if m.selected >= len(revisions) {
		m.selected = 0
	}
}

// Show reveals the picker. While it is visible the parent model routes every key here.
func (m *Model) Show() {
	m.visible = true
}

// Hide takes the picker off screen and leaves the revisions and cursor in place, so a later
// Show resumes on the same row.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether keys belong to the picker rather than the main view.
func (m *Model) IsVisible() bool {
	return m.visible
}

// MoveUp moves the cursor one revision earlier in the list and stops at the first, without wrapping.
func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

// MoveDown moves the cursor one revision later in the list and stops at the last, without wrapping.
func (m *Model) MoveDown() {
	if m.selected < len(m.revisions)-1 {
		m.selected++
	}
}

// GetSelected returns a pointer into the revision slice, or nil when the list is empty. The
// pointer is invalidated by the next SetRevisions, so read what is needed before calling it.
func (m Model) GetSelected() *jj.RevisionEntry {
	if m.selected >= 0 && m.selected < len(m.revisions) {
		return &m.revisions[m.selected]
	}

	return nil
}

// View centers the picker in a terminal of the given cell dimensions, scrolling a window of
// rows that keeps the cursor near the middle. It returns an empty string while hidden.
func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	maxHeight := height - 4
	if maxHeight < 5 {
		maxHeight = 5
	}
	if maxHeight > 20 {
		maxHeight = 20
	}

	modalWidth := width - 20
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalWidth > 80 {
		modalWidth = 80
	}

	var lines []string
	lines = append(lines, styleHeader("Select Destination", modalWidth))
	lines = append(lines, "")

	startIdx := 0
	endIdx := len(m.revisions)

	if len(m.revisions) > maxHeight-3 {
		startIdx = m.selected - (maxHeight-3)/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxHeight - 3
		if endIdx > len(m.revisions) {
			endIdx = len(m.revisions)
			startIdx = endIdx - (maxHeight - 3)
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	for i := startIdx; i < endIdx; i++ {
		rev := m.revisions[i]
		isSelected := i == m.selected
		line := m.renderRevisionLine(rev, isSelected, modalWidth-2)
		lines = append(lines, "  "+line)
	}

	lines = append(lines, "")
	lines = append(lines, styleFooter("Enter: Select | Esc: Cancel | j/k: Navigate", modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m Model) renderRevisionLine(rev jj.RevisionEntry, selected bool, width int) string {
	desc := rev.Description
	if len(desc) > width-15 {
		desc = desc[:width-18] + "..."
	}

	line := fmt.Sprintf("%-10s %s", rev.ChangeID, desc)

	if selected {
		return styleSelected(truncateOrPad(line, width))
	}

	return truncateOrPad(line, width)
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

func styleSelected(text string) string {
	return lipgloss.NewStyle().
		Background(theme.SelectedBg).
		Foreground(theme.Text).
		Render(text)
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		return text[:width-3] + "..."
	}

	return text + strings.Repeat(" ", width-len(text))
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
