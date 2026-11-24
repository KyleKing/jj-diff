// Package splitpreview renders the read-only summary of a pending multi-way split, one row per tag
// with its destination and the amount of work assigned to it.
package splitpreview

import (
	"fmt"
	"strconv"

	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Layout of the centered modal, in terminal cells. The modal shrinks with the terminal until it
// hits minModalWidth and stops growing at maxModalWidth. A summary row is drawn with a two-cell
// indent and a matching gap after it, which is what rowInsetWidth accounts for.
const preferredWidth = 72

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
	ChangeID    string
	Description string
	Type        DestinationType
}

// SplitSummary is one row of the preview. FileCount and HunkCount are what the tag currently holds,
// so they are recomputed by the parent rather than tracked here.
type SplitSummary struct {
	Destination DestinationSpec
	FileCount   int
	HunkCount   int
	Tag         SplitTag
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
func (m Model) View(width, _ int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)

	body := make([]string, 0, len(m.summaries))
	for _, summary := range m.summaries {
		body = append(body, summaryLine(summary))
	}

	if len(body) == 0 {
		body = append(body, theme.Skin.Muted.Render("No tags assigned"))
	}

	return modal.Box("Split", strconv.Itoa(len(m.summaries)), body, []keyhint.Hint{
		{Key: "enter", What: "apply"}, {Key: "e", What: "edit"}, {Key: "esc", What: "cancel"},
	}, inner)
}

// summaryLine reads as one move: the tag, where its changes go, and how much of the diff it takes.
func summaryLine(summary SplitSummary) string {
	tag := theme.Skin.Cursor.Render(string(summary.Tag))

	dest := theme.Skin.Accent.Render("new ") + theme.Skin.Body.Render(summary.Destination.Description)
	if summary.Destination.Type != DestNewCommit {
		dest = theme.Skin.Body.Render(summary.Destination.ChangeID) + " " +
			theme.Skin.Muted.Render(summary.Destination.Description)
	}

	size := theme.Skin.Muted.Render(fmt.Sprintf("%d %s, %d %s",
		summary.FileCount, plural(summary.FileCount, "file"), summary.HunkCount, plural(summary.HunkCount, "hunk")))

	return tag + theme.Skin.Muted.Render(" → ") + dest + "   " + size
}

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}

	return noun + "s"
}
