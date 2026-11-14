package diffview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Column layout of the two-column view, in terminal cells. The " │ " between the panes costs
// paneSeparatorWidth.
const (
	paneCount          = 2
	paneSeparatorWidth = 3
)

// SideBySideView renders old and new content in two columns. It is stateless, so one instance can
// serve every file.
type SideBySideView struct{}

// NewSideBySideView returns the two-column renderer.
func NewSideBySideView() *SideBySideView {
	return &SideBySideView{}
}

// SupportsSelection reports false, because the two-column layout draws no selection markers.
func (*SideBySideView) SupportsSelection() bool {
	return false
}

// Render draws the file into ctx.Width by ctx.Height, splitting the width between the two columns
// and padding out when the content is shorter. A nil file renders the empty-state placeholder.
func (*SideBySideView) Render(file *diff.FileChange, ctx *RenderContext) string {
	if file == nil {
		return padToSize("No file selected", ctx.Width, ctx.Height)
	}

	paneWidth := (ctx.Width - paneSeparatorWidth) / paneCount
	leftHeader := truncateOrPad("OLD", paneWidth)
	rightHeader := truncateOrPad("NEW", paneWidth)
	headerStyle := lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true)
	lines := []string{headerStyle.Render(leftHeader) + " │ " + headerStyle.Render(rightHeader)}

	for hunkIdx, hunk := range file.Hunks {
		lines = append(lines, renderSideBySideHunkHeader(
			hunk.Header,
			ctx.Width,
			hunkIdx == ctx.SelectedHunk,
		))

		hunkLines := hunk.Lines
		if ctx.ShowWhitespace {
			hunkLines = diff.ProcessHunkHideWhitespace(hunk.Lines)
		}

		for _, pair := range diff.PairSides(hunkLines) {
			if len(lines) >= ctx.Height {
				break
			}
			lines = append(lines, renderPairedLine(pair, paneWidth, file.Path, ctx))
		}
	}

	for len(lines) < ctx.Height {
		lines = append(lines, strings.Repeat(" ", max(ctx.Width, 0)))
	}

	return strings.Join(lines, "\n")
}

func renderPairedLine(pair diff.SidePair, paneWidth int, path string, ctx *RenderContext) string {
	left := renderSinglePane(pair.Left, false, paneWidth, path, ctx)
	right := renderSinglePane(pair.Right, true, paneWidth, path, ctx)

	return left + " │ " + right
}

// renderSinglePane numbers the left pane by the old file and the right by the new.
func renderSinglePane(
	line *diff.Line,
	isRight bool,
	paneWidth int,
	path string,
	ctx *RenderContext,
) string {
	if line == nil {
		return strings.Repeat(" ", max(paneWidth, 0))
	}

	base := rowBase(line.Type, false)

	head := markerStyle(line.Type, &base).Render(line.Type.String() + " ")
	if ctx.ShowLineNumbers {
		number := line.OldLineNum
		if isRight {
			number = line.NewLineNum
		}

		head = base.Foreground(theme.Muted).Render(fmt.Sprintf("%4d ", number)) + head
	}

	content := fitStyled(
		styleCode(ctx.Highlighter, path, line.Content, &base),
		paneWidth-lipgloss.Width(head),
		&base,
	)

	return padRow(head+content, paneWidth, &base)
}

func renderSideBySideHunkHeader(text string, width int, isCurrent bool) string {
	style := lipgloss.NewStyle().Foreground(theme.Accent)
	if isCurrent {
		style = theme.SelectedFocusedStyle
	}

	return style.Render(truncateOrPad(text, width))
}
