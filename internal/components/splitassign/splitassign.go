package splitassign

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/theme"
)

type SplitTag rune

type DestinationType int

const (
	DestExistingRevision DestinationType = iota
	DestNewCommit
)

type DestinationSpec struct {
	Type        DestinationType
	ChangeID    string
	Description string
}

type Model struct {
	tags          []SplitTag
	revisions     []jj.RevisionEntry
	selectedTag   int
	selectedRev   int
	destinations  map[SplitTag]*DestinationSpec
	visible       bool
	focusOnTags   bool
}

func New() Model {
	return Model{
		tags:         []SplitTag{},
		revisions:    []jj.RevisionEntry{},
		selectedTag:  0,
		selectedRev:  0,
		destinations: make(map[SplitTag]*DestinationSpec),
		visible:      false,
		focusOnTags:  true,
	}
}

func (m *Model) SetTags(tags []SplitTag) {
	m.tags = tags
	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i] < m.tags[j]
	})
	if m.selectedTag >= len(m.tags) {
		m.selectedTag = 0
	}
}

func (m *Model) SetRevisions(revisions []jj.RevisionEntry) {
	m.revisions = revisions
	if m.selectedRev >= len(revisions) {
		m.selectedRev = 0
	}
}

func (m *Model) Show() {
	m.visible = true
	m.focusOnTags = true
}

func (m *Model) Hide() {
	m.visible = false
}

func (m *Model) IsVisible() bool {
	return m.visible
}

func (m *Model) ToggleFocus() {
	m.focusOnTags = !m.focusOnTags
}

func (m *Model) MoveUp() {
	if m.focusOnTags {
		if m.selectedTag > 0 {
			m.selectedTag--
		}
	} else {
		if m.selectedRev > 0 {
			m.selectedRev--
		}
	}
}

func (m *Model) MoveDown() {
	if m.focusOnTags {
		if m.selectedTag < len(m.tags)-1 {
			m.selectedTag++
		}
	} else {
		if m.selectedRev < len(m.revisions)-1 {
			m.selectedRev++
		}
	}
}

func (m *Model) AssignRevisionToCurrentTag() {
	if m.selectedTag >= 0 && m.selectedTag < len(m.tags) &&
		m.selectedRev >= 0 && m.selectedRev < len(m.revisions) {
		tag := m.tags[m.selectedTag]
		rev := m.revisions[m.selectedRev]
		m.destinations[tag] = &DestinationSpec{
			Type:        DestExistingRevision,
			ChangeID:    rev.ChangeID,
			Description: rev.Description,
		}
	}
}

func (m *Model) AssignNewCommitToCurrentTag(description string) {
	if m.selectedTag >= 0 && m.selectedTag < len(m.tags) {
		tag := m.tags[m.selectedTag]
		m.destinations[tag] = &DestinationSpec{
			Type:        DestNewCommit,
			Description: description,
		}
	}
}

func (m *Model) AssignNewCommitToTag(tag SplitTag, description string) {
	m.destinations[tag] = &DestinationSpec{
		Type:        DestNewCommit,
		Description: description,
	}
}

func (m Model) GetDestinations() map[SplitTag]*DestinationSpec {
	return m.destinations
}

func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	maxHeight := height - 6
	if maxHeight < 10 {
		maxHeight = 10
	}

	modalWidth := width - 20
	if modalWidth < 60 {
		modalWidth = 60
	}
	if modalWidth > 100 {
		modalWidth = 100
	}

	var lines []string
	lines = append(lines, styleHeader("Assign Destinations to Tags", modalWidth))
	lines = append(lines, "")

	leftWidth := modalWidth / 2
	rightWidth := modalWidth - leftWidth - 3

	lines = append(lines, m.renderSplitView(leftWidth, rightWidth, maxHeight))

	lines = append(lines, "")
	lines = append(lines, styleFooter("Tab: Switch Panel | Enter: Assign | N: New Commit | Esc: Cancel", modalWidth))

	content := strings.Join(lines, "\n")
	return renderModal(content, width, height)
}

func (m Model) renderSplitView(leftWidth, rightWidth, maxHeight int) string {
	tagLines := m.renderTagList(leftWidth, maxHeight)
	revLines := m.renderRevisionList(rightWidth, maxHeight)

	maxLines := len(tagLines)
	if len(revLines) > maxLines {
		maxLines = len(revLines)
	}

	for len(tagLines) < maxLines {
		tagLines = append(tagLines, strings.Repeat(" ", leftWidth))
	}
	for len(revLines) < maxLines {
		revLines = append(revLines, strings.Repeat(" ", rightWidth))
	}

	var combined []string
	for i := 0; i < maxLines; i++ {
		combined = append(combined, fmt.Sprintf("%s │ %s", tagLines[i], revLines[i]))
	}

	return strings.Join(combined, "\n")
}

func (m Model) renderTagList(width, maxHeight int) []string {
	var lines []string

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Primary)
	if m.focusOnTags {
		headerStyle = headerStyle.Background(theme.MutedBg)
	}
	lines = append(lines, headerStyle.Render(truncateOrPad("Tags", width)))

	for i, tag := range m.tags {
		isSelected := i == m.selectedTag
		dest := m.destinations[tag]
		var line string
		if dest != nil {
			if dest.Type == DestNewCommit {
				line = fmt.Sprintf("[%s] → NEW: %s", string(tag), truncate(dest.Description, width-15))
			} else {
				line = fmt.Sprintf("[%s] → %s", string(tag), truncate(dest.ChangeID, width-10))
			}
		} else {
			line = fmt.Sprintf("[%s] (unassigned)", string(tag))
		}

		if isSelected && m.focusOnTags {
			lines = append(lines, styleSelected(truncateOrPad(line, width)))
		} else {
			lines = append(lines, truncateOrPad(line, width))
		}
	}

	return lines
}

func (m Model) renderRevisionList(width, maxHeight int) []string {
	var lines []string

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Primary)
	if !m.focusOnTags {
		headerStyle = headerStyle.Background(theme.MutedBg)
	}
	lines = append(lines, headerStyle.Render(truncateOrPad("Revisions", width)))

	for i, rev := range m.revisions {
		isSelected := i == m.selectedRev
		desc := truncate(rev.Description, width-15)
		line := fmt.Sprintf("%-10s %s", rev.ChangeID, desc)

		if isSelected && !m.focusOnTags {
			lines = append(lines, styleSelected(truncateOrPad(line, width)))
		} else {
			lines = append(lines, truncateOrPad(line, width))
		}
	}

	return lines
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
	if len(text) > width {
		if width > 3 {
			return text[:width-3] + "..."
		}
		return text[:width]
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
