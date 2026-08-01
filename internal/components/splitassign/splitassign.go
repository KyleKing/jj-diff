// Package splitassign renders the modal that pairs each split tag with a destination, either an
// existing revision or a commit to create. The parent model fills it with tags and revisions, routes
// keys to it while it is visible, and reads the finished pairing back with GetDestinations.
package splitassign

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Layout of the centered modal, in terminal cells. The modal shrinks with the terminal until it
// hits minModalWidth and stops growing at maxModalWidth. The two panels split that width evenly
// minus the " | " separator between them.
const (
	changeIDReserve     = 10
	descriptionReserve  = 15
	ellipsisWidth       = 3
	maxModalWidth       = 100
	minModalWidth       = 60
	modalPaddingX       = 2
	modalPaddingY       = 1
	modalWidthMargin    = 20
	panelCount          = 2
	panelSeparatorWidth = 3
)

// SplitTag is the single character a hunk carries while a multi-way split is being assembled.
// It is declared here rather than shared with the parent model so this component imports nothing
// from it.
type SplitTag rune

// DestinationType separates an assignment that targets a revision that already exists from one that
// creates a commit.
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
	ChangeID    string
	Description string
	Type        DestinationType
}

// Model is the two-panel assignment modal, tags on the left and candidate revisions on the right,
// with one cursor per panel. It owns the destination map for the whole split, so the parent model
// keeps a single instance alive across openings.
type Model struct {
	destinations map[SplitTag]*DestinationSpec
	tags         []SplitTag
	revisions    []jj.RevisionEntry
	selectedTag  int
	selectedRev  int
	visible      bool
	focusOnTags  bool
}

// New returns a hidden modal with no tags, no revisions, and the tag panel focused.
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

// SetTags adopts tags as the tag list and sorts it ascending, which reorders the caller's slice in
// place. The tag cursor rewinds when it would sit past the end. Assignments are untouched, including
// any belonging to a tag that is no longer listed.
func (m *Model) SetTags(tags []SplitTag) {
	m.tags = tags
	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i] < m.tags[j]
	})
	if m.selectedTag >= len(m.tags) {
		m.selectedTag = 0
	}
}

// SetRevisions replaces the candidate revisions and rewinds the revision cursor when it would sit
// past the end. Order is preserved because the caller decides which revisions are worth offering.
func (m *Model) SetRevisions(revisions []jj.RevisionEntry) {
	m.revisions = revisions
	if m.selectedRev >= len(revisions) {
		m.selectedRev = 0
	}
}

// Show opens the modal with the tag panel focused, whatever panel was focused when it last closed.
func (m *Model) Show() {
	m.visible = true
	m.focusOnTags = true
}

// Hide closes the modal, keeping the assignments made so far.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether the modal is open, which is how the parent decides to route keys here.
func (m *Model) IsVisible() bool {
	return m.visible
}

// ToggleFocus swaps the cursor between the tag panel and the revision panel.
func (m *Model) ToggleFocus() {
	m.focusOnTags = !m.focusOnTags
}

// MoveUp moves the focused panel's cursor up one row, stopping at the first.
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

// MoveDown moves the focused panel's cursor down one row, stopping at the last.
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

// AssignRevisionToCurrentTag points the selected tag at the selected revision, replacing any earlier
// assignment. It does nothing when either cursor sits outside its list.
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

// AssignNewCommitToCurrentTag sends the selected tag to a commit to be created with description.
// It does nothing when the tag cursor sits outside the list.
func (m *Model) AssignNewCommitToCurrentTag(description string) {
	if m.selectedTag >= 0 && m.selectedTag < len(m.tags) {
		tag := m.tags[m.selectedTag]
		m.destinations[tag] = &DestinationSpec{
			Type:        DestNewCommit,
			Description: description,
		}
	}
}

// AssignNewCommitToTag sends a named tag to a commit to be created with description, without regard
// to the cursor, and records the tag even when it is not in the modal's tag list.
func (m *Model) AssignNewCommitToTag(tag SplitTag, description string) {
	m.destinations[tag] = &DestinationSpec{
		Type:        DestNewCommit,
		Description: description,
	}
}

// GetDestinations returns the tag-to-destination map by reference, so later assignments are visible
// through a map the caller already holds. A tag the user never assigned is absent.
func (m *Model) GetDestinations() map[SplitTag]*DestinationSpec {
	return m.destinations
}

// View renders the modal centered in the given terminal size, returning the empty string while hidden.
func (m *Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := clamp(width-modalWidthMargin, minModalWidth, maxModalWidth)
	leftWidth := modalWidth / panelCount
	rightWidth := modalWidth - leftWidth - panelSeparatorWidth

	lines := []string{
		styleHeader("Assign Destinations to Tags", modalWidth),
		"",
		m.renderSplitView(leftWidth, rightWidth),
		"",
		styleFooter("Tab: Switch Panel | Enter: Assign | N: New Commit | Esc: Cancel", modalWidth),
	}

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m *Model) renderSplitView(leftWidth, rightWidth int) string {
	tagLines := m.renderTagList(leftWidth)
	revLines := m.renderRevisionList(rightWidth)

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

	combined := make([]string, 0, maxLines)
	for i := range maxLines {
		combined = append(combined, fmt.Sprintf("%s │ %s", tagLines[i], revLines[i]))
	}

	return strings.Join(combined, "\n")
}

func (m *Model) renderTagList(width int) []string {
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
				line = fmt.Sprintf(
					"[%s] → NEW: %s",
					string(tag),
					truncate(dest.Description, width-descriptionReserve),
				)
			} else {
				line = fmt.Sprintf("[%s] → %s", string(tag), truncate(dest.ChangeID, width-changeIDReserve))
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

func (m *Model) renderRevisionList(width int) []string {
	var lines []string

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Primary)
	if !m.focusOnTags {
		headerStyle = headerStyle.Background(theme.MutedBg)
	}
	lines = append(lines, headerStyle.Render(truncateOrPad("Revisions", width)))

	for i, rev := range m.revisions {
		isSelected := i == m.selectedRev
		desc := truncate(rev.Description, width-descriptionReserve)
		line := fmt.Sprintf("%-10s %s", rev.ChangeID, desc)

		if isSelected && !m.focusOnTags {
			lines = append(lines, styleSelected(truncateOrPad(line, width)))
		} else {
			lines = append(lines, truncateOrPad(line, width))
		}
	}

	return lines
}

func clamp(value, lower, upper int) int {
	return min(max(value, lower), upper)
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
		if width > ellipsisWidth {
			return text[:width-ellipsisWidth] + "..."
		}

		return text[:width]
	}

	return text
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		if width > ellipsisWidth {
			return text[:width-ellipsisWidth] + "..."
		}

		return text[:width]
	}

	return text + strings.Repeat(" ", width-len(text))
}

func renderModal(content string, termWidth, termHeight int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(modalPaddingY, modalPaddingX)

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
