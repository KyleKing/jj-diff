package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/overlay"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/statusbar"
	"github.com/kyleking/jj-diff/internal/theme"
)

// View wraps the rendered content in the alternate screen buffer, which the program stays in for
// its whole run.
func (m *Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	// A terminal coming back into focus is when jojo may have marked hunks meanwhile, which the
	// next message's stat then picks up.
	view.ReportFocus = m.marks.On()

	return view
}

// render draws the two panels, or the overlay that is up. It returns the empty string until the
// first tea.WindowSizeMsg arrives, because every width is derived from m.width.
func (m *Model) render() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	// Bubble Tea renders once before the first tea.WindowSizeMsg arrives. Panel
	// widths are derived from m.width, so laying out at zero produces garbage.
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if m.mode == ModeResolve {
		frame := *m

		return frame.finish(frame.renderResolve())
	}

	if m.cols != nil && len(m.changes) > 0 {
		frame := *m

		return frame.finish(frame.renderColumns())
	}

	if len(m.changes) == 0 {
		return m.finish(m.renderEmpty())
	}

	// The push* calls below install per-frame render state on the panel components, and the
	// callbacks they install close over the model. Drawing from a copy keeps that state out of the
	// model Update owns, so a frame can never leave stale callbacks behind for the next one.
	frame := *m

	fileListExpanded := frame.focusedPanel == PanelFileList
	frame.fileList.SetExpanded(fileListExpanded)

	fileListHeight := frame.fileListHeight()

	frame.pushSelectionState()
	frame.pushSearchState()
	frame.pushReads()

	fileListView := frame.fileList.View(frame.width, fileListHeight, fileListExpanded)
	diffViewHeight := frame.screenHeight() - fileListHeight - chromeHeight
	diffViewView := frame.renderDiffView(diffViewHeight)

	// The collapsed list is the diff's heading on a band, which already sets it apart, so the row
	// under it is left as space. The expanded list ends in a hairline, quieter than anything it
	// separates.
	border := strings.Repeat(" ", frame.width)
	if fileListExpanded {
		border = strings.Repeat(" ", min(paneMargin, frame.width)) +
			theme.Skin.Hairline.Render(strings.Repeat("\u2500", max(frame.width-paneMargin-1, 0))) + " "
	}

	panels := fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		fileListView,
		border,
		diffViewView,
		frame.renderStatusBar(),
	)

	return frame.finish(panels)
}

// finish lays everything that floats over a screen on it, bottom to top: the coach's band, the
// modal that is up, the which-key panel, and last the palette over every row but the footer.
//
// The palette leaves the modal out rather than covering part of it: a strip over the bottom rows
// would cut the box and leave its top edge standing above the strip, a frame with no sides. The
// palette lists the modal's own keys and the footer still names it, and closing the palette brings
// the box back.
func (m *Model) finish(screen string) string {
	if !m.pal.IsOpen() {
		return m.withWelcome(m.whichKey(composite(m.withBand(screen), m.overlayView(), m.width, m.height)))
	}

	frame := m.whichKey(m.withBand(screen))

	lines := strings.Split(frame, "\n")
	last := len(lines) - 1

	return strings.Join(append(m.pal.Over(lines[:last], m.width), lines[last]), "\n")
}

// fileListHeight is the rows the file list takes: one collapsed, or as many as it has files up to a
// third of the terminal while it has focus.
func (m *Model) fileListHeight() int {
	if m.focusedPanel != PanelFileList {
		return collapsedFileListHeight
	}

	return max(min(m.fileList.ContentHeight(), m.screenHeight()/maxFileListFraction), minFileListHeight)
}

// diffHeight is the rows the diff pane draws in, which paging and framing measure against.
func (m *Model) diffHeight() int {
	return max(m.screenHeight()-m.fileListHeight()-chromeHeight, 1)
}

// whichKeyMargin is the columns left of the which-key panel at its widest.
const whichKeyMargin = 4

// whichKey floats the keys that can follow a waiting prefix over the bottom right of the frame, the
// way mini.which-key does, so a chord never has to be remembered whole.
func (m *Model) whichKey(frame string) string {
	hints := m.matcher.Panel(m.context())
	if len(hints) == 0 {
		return frame
	}

	head := keyhint.Hint{Head: true, What: strings.Join(m.matcher.Pending(), "") + "…"}
	styles := keyhint.PanelStyles{
		Frame: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Skin.Edge.GetForeground()).Padding(0, 1),
		Styles: theme.Legend(),
		Count:  theme.Skin.Muted,
		On:     theme.Skin.Accent,
	}

	// The panel sizes to its widest hint, and may take all but a margin, so a flag's description
	// is never cut to fit half the screen.
	box := keyhint.Panel(styles, append([]keyhint.Hint{head}, hints...), max(1, m.width-whichKeyMargin))

	// The panel sits above the footer, which keeps saying where the user is and how to get out.
	lines := strings.Split(frame, "\n")
	last := len(lines) - 1

	return strings.Join(append(overlay.BottomRight(lines[:last], box, m.width), lines[last]), "\n")
}

// composite centers overlay over base. The panels stay drawn underneath, so you can still read the
// revision you are picking a destination from, but faint and without their colors, so the box the
// keys now go to is the one thing on screen that stands out.
func composite(base, box string, width, height int) string {
	if box == "" {
		return base
	}

	return overlay.Over(base, box, width, height, overlay.Styles{Dim: theme.Skin.Muted})
}

// overlayView returns the modal box that is up, or the empty string when no modal is open. The order
// is the stacking order on screen.
func (m *Model) overlayView() string {
	switch {
	case m.msgs != nil:
		return m.messagesView(m.width, m.height)
	case m.picker.IsVisible():
		return m.picker.View(m.width, m.height)
	}

	return ""
}

// pushSelectionState hands the diff view the callbacks it needs to draw the current selection. Browse
// mode reports nothing selected, so the view still highlights the hunk cursor without marking it.
func (m *Model) pushSelectionState() {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	currentFile := m.changes[m.selectedFile]

	if m.mode != ModeInteractive && m.mode != ModeDiffEditor {
		m.diffView.SetSelection(m.selectedHunk, func(_ int) bool {
			return false
		})

		return
	}

	m.diffView.SetSelection(m.selectedHunk, func(hunkIdx int) bool {
		return m.selection.IsHunkSelected(currentFile.Path, hunkIdx)
	})
	m.diffView.SetVisualState(
		m.lineCursor,
		m.isVisualMode,
		m.visualAnchor,
		func(hunkIdx, lineIdx int) bool {
			return m.selection.IsLineSelected(currentFile.Path, hunkIdx, lineIdx)
		},
	)
}

func (m *Model) pushSearchState() {
	if m.searchState == nil || !m.searchState.IsActive {
		m.fileList.SetSearchState(false, nil)
		m.diffView.SetSearchState(false, nil)
		m.diffView.SetSearchHits(nil)

		return
	}

	m.fileList.SetSearchState(true, m.getFilePathMatches)

	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	currentFile := m.changes[m.selectedFile]
	m.diffView.SetSearchState(true, func(hunkIdx, lineIdx int) []diffview.MatchRange {
		return m.getLineContentMatches(currentFile.Path, hunkIdx, lineIdx)
	})

	m.diffView.SetSearchHits(m.matchIndex().hits[currentFile.Path])
}

// faintSGR is the opening and closing a faint run adds to a dimmed row, which sizes the frame.
const faintSGR = "\x1b[2m\x1b[m"

// renderDiffView dims the whole pane while the file list has focus, so the two panels read as one
// focused and one inactive.
func (m *Model) renderDiffView(height int) string {
	focused := m.focusedPanel == PanelDiffView

	view := m.diffView.View(m.width, height, focused)
	if focused {
		return view
	}

	var out strings.Builder

	out.Grow(len(view) + (strings.Count(view, "\n")+1)*len(faintSGR))

	for i, line := range strings.Split(view, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}

		m.dim.Write(&out, line)
	}

	return out.String()
}

// screenName is what the footer calls the screen: the mode, or the columns while they are up.
func (m *Model) screenName() string {
	if m.cols != nil {
		return "Split"
	}

	return modeName(m.mode)
}

func (m *Model) renderStatusBar() string {
	if m.searchModal.IsVisible() {
		return m.searchModal.Line(m.width)
	}

	ctx := statusbar.Context{
		Destination:  m.destination,
		IsVisualMode: m.isVisualMode || (m.cols != nil && m.cols.visual),
		Mode:         m.screenName(),
		Source:       m.source,
		Notice:       m.notice,
		Alarm:        m.alarm,
		Hints:        m.footerHints(),
	}

	if !m.quitPrompt && m.notice == "" {
		ctx.Lead, ctx.LeadKey, ctx.Hints = m.coachFooter(ctx.Hints)
	}

	if m.quitPrompt {
		question := "quit with a selection pending?"

		switch {
		case m.mode == ModeResolve:
			question = "quit with picks unwritten?"
		case m.cols != nil:
			question = "quit with the split unwritten?"
		}

		ctx.Notice, ctx.Alarm, ctx.Hints = question, true, m.quitPromptHints()
	}

	return m.statusBar.ViewWithContext(m.width, &ctx)
}
