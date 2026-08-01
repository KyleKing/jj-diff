package diffview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Column layout of the two-column view, in terminal cells. The " │ " between the panes costs
// paneSeparatorWidth, and each pane reserves paneContentPadding beside its line number.
const (
	paneContentPadding = 2
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

		for _, pair := range pairLines(hunkLines) {
			if len(lines) >= ctx.Height {
				break
			}
			lines = append(lines, renderPairedLine(pair, paneWidth, ctx))
		}
	}

	for len(lines) < ctx.Height {
		lines = append(lines, strings.Repeat(" ", max(ctx.Width, 0)))
	}

	return strings.Join(lines, "\n")
}

type linePair struct {
	Left  *diff.Line
	Right *diff.Line
}

func pairLines(lines []diff.Line) []linePair {
	var pairs []linePair

	i := 0
	for i < len(lines) {
		line := &lines[i]

		switch line.Type {
		case diff.LineContext:
			pairs = append(pairs, linePair{Left: line, Right: line})
			i++

		case diff.LineDeletion:
			delEnd := runEnd(lines, i, diff.LineDeletion)
			addEnd := runEnd(lines, delEnd, diff.LineAddition)
			pairs = append(pairs, pairRuns(lines[i:delEnd], lines[delEnd:addEnd])...)
			i = addEnd

		case diff.LineAddition:
			pairs = append(pairs, linePair{Left: nil, Right: line})
			i++
		}
	}

	return pairs
}

func runEnd(lines []diff.Line, start int, lineType diff.LineType) int {
	end := start
	for end < len(lines) && lines[end].Type == lineType {
		end++
	}

	return end
}

func pairRuns(deletions, additions []diff.Line) []linePair {
	maxCount := max(len(deletions), len(additions))
	pairs := make([]linePair, 0, maxCount)

	for j := range maxCount {
		pair := linePair{}
		if j < len(deletions) {
			pair.Left = &deletions[j]
		}
		if j < len(additions) {
			pair.Right = &additions[j]
		}
		pairs = append(pairs, pair)
	}

	return pairs
}

func renderPairedLine(pair linePair, paneWidth int, ctx *RenderContext) string {
	leftContent := renderSinglePane(pair.Left, paneWidth, ctx, false)
	rightContent := renderSinglePane(pair.Right, paneWidth, ctx, true)

	return leftContent + " │ " + rightContent
}

func renderSinglePane(
	line *diff.Line,
	paneWidth int,
	ctx *RenderContext,
	isRight bool,
) string {
	if line == nil {
		return strings.Repeat(" ", max(paneWidth, 0))
	}

	lineNumStr := ""
	if ctx.ShowLineNumbers {
		if isRight || line.Type == diff.LineContext {
			lineNumStr = fmt.Sprintf("%4d ", line.NewLineNum)
		} else {
			lineNumStr = fmt.Sprintf("%4d ", line.OldLineNum)
		}
	}

	content := line.Content
	maxContentWidth := max(paneWidth-len(lineNumStr)-paneContentPadding, 0)
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	text := lineNumStr + content

	style := lipgloss.NewStyle()
	switch line.Type {
	case diff.LineContext:
	case diff.LineAddition:
		style = style.Foreground(theme.AddedLine)
	case diff.LineDeletion:
		style = style.Foreground(theme.DeletedLine)
	}

	return style.Render(truncateOrPad(text, paneWidth))
}

func renderSideBySideHunkHeader(text string, width int, isCurrent bool) string {
	style := lipgloss.NewStyle().Foreground(theme.Accent)
	if isCurrent {
		style = style.Background(theme.MutedBg)
	}

	return style.Render(truncateOrPad(text, width))
}
