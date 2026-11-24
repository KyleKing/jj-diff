// Package splitassign renders the modal that pairs each split tag with a destination, either an
// existing revision or a commit to create. The parent model fills it with tags and revisions, routes
// keys to it while it is visible, and reads the finished pairing back with GetDestinations.
package splitassign

import (
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/destpicker"
	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Layout of the centered modal, in terminal cells. The modal shrinks with the terminal until it
// hits minModalWidth and stops growing at maxModalWidth. The two panels split that width evenly
// minus the " | " separator between them.
const (
	panelCount     = 2
	preferredWidth = 84
	// The tag pane is wide enough for a tag, its arrow, and a change id or a short new-commit message.
	tagPaneWidth = 32
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
func (m *Model) View(width, _ int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)
	leftWidth := min(inner/panelCount, tagPaneWidth)
	rightWidth := inner - leftWidth - lipgloss.Width(paneGap)

	return modal.Box("Assign destinations", strconv.Itoa(len(m.tags)), m.panes(leftWidth, rightWidth), []keyhint.Hint{
		{Key: "tab", What: "switch pane"},
		{Key: "enter", What: "assign"},
		{Key: "N", What: "new commit"},
		{Key: "esc", What: "cancel"},
	}, inner)
}

// paneGap parts the tag pane from the revision pane with a hairline.
var paneGap = " " + "│" + " "

func (m *Model) panes(leftWidth, rightWidth int) []string {
	left := m.tagLines(leftWidth)
	right := m.revisionLines(rightWidth)
	rows := max(len(left), len(right))
	gap := theme.Skin.Hairline.Render(paneGap)

	out := make([]string, 0, rows)
	for i := range rows {
		l, r := strings.Repeat(" ", leftWidth), ""
		if i < len(left) {
			l = left[i]
		}

		if i < len(right) {
			r = right[i]
		}

		out = append(out, l+gap+r)
	}

	return out
}

// heading names a pane, at full weight while it has the keyboard and a step back when it does not.
func heading(name string, focused bool, width int) string {
	face := theme.Skin.Muted
	if focused {
		face = theme.Skin.Title
	}

	return modal.Fit(face.Render(name), width)
}

func (m *Model) tagLines(width int) []string {
	lines := make([]string, 0, 1+len(m.tags))
	lines = append(lines, heading("Tags", m.focusOnTags, width))

	for i, tag := range m.tags {
		lines = append(lines, modal.Row(m.tagLine(tag), width, m.focusOnTags && i == m.selectedTag))
	}

	return lines
}

// tagLine is a tag and where its hunks go, or that they go nowhere yet.
func (m *Model) tagLine(tag SplitTag) string {
	head := theme.Skin.Cursor.Render(string(tag)) + theme.Skin.Muted.Render(" → ")

	dest := m.destinations[tag]
	switch {
	case dest == nil:
		return head + theme.Skin.Warning.Render("unassigned")
	case dest.Type == DestNewCommit:
		return head + theme.Skin.Accent.Render("new ") + theme.Skin.Body.Render(dest.Description)
	}

	return head + theme.Skin.Body.Render(dest.ChangeID)
}

func (m *Model) revisionLines(width int) []string {
	lines := make([]string, 0, 1+len(m.revisions))
	lines = append(lines, heading("Revisions", !m.focusOnTags, width))

	for i := range m.revisions {
		lines = append(lines, modal.Row(destpicker.Line(&m.revisions[i]), width, !m.focusOnTags && i == m.selectedRev))
	}

	return lines
}
