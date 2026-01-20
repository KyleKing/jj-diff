package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

type SideBySideView struct{}

func NewSideBySideView() *SideBySideView {
	return &SideBySideView{}
}

func (v *SideBySideView) SupportsSelection() bool {
	return false
}

func (v *SideBySideView) Render(file *diff.FileChange, ctx RenderContext) string {
	if file == nil {
		return padToSize("No file selected", ctx.Width, ctx.Height)
	}

	var lines []string

	header := fmt.Sprintf("%s %s", file.ChangeType.String(), file.Path)
	lines = append(lines, styleHeader(header, ctx.Width))

	paneWidth := (ctx.Width - 3) / 2
	leftHeader := truncateOrPad("OLD", paneWidth)
	rightHeader := truncateOrPad("NEW", paneWidth)
	headerStyle := lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true)
	lines = append(lines, headerStyle.Render(leftHeader)+" │ "+headerStyle.Render(rightHeader))

	for hunkIdx, hunk := range file.Hunks {
		hunkHeader := v.renderSideBySideHunkHeader(hunk.Header, ctx.Width, hunkIdx == ctx.SelectedHunk)
		lines = append(lines, hunkHeader)

		pairedLines := v.pairLines(hunk.Lines)
		for _, pair := range pairedLines {
			if len(lines) >= ctx.Height {
				break
			}
			lines = append(lines, v.renderPairedLine(pair, paneWidth, ctx))
		}
	}

	for len(lines) < ctx.Height {
		lines = append(lines, strings.Repeat(" ", ctx.Width))
	}

	return strings.Join(lines, "\n")
}

type linePair struct {
	Left  *diff.Line
	Right *diff.Line
}

func (v *SideBySideView) pairLines(lines []diff.Line) []linePair {
	var pairs []linePair
	i := 0

	for i < len(lines) {
		line := &lines[i]

		switch line.Type {
		case diff.LineContext:
			pairs = append(pairs, linePair{Left: line, Right: line})
			i++

		case diff.LineDeletion:
			delStart := i
			for i < len(lines) && lines[i].Type == diff.LineDeletion {
				i++
			}
			delEnd := i

			addStart := i
			for i < len(lines) && lines[i].Type == diff.LineAddition {
				i++
			}
			addEnd := i

			delCount := delEnd - delStart
			addCount := addEnd - addStart
			maxCount := delCount
			if addCount > maxCount {
				maxCount = addCount
			}

			for j := 0; j < maxCount; j++ {
				pair := linePair{}
				if delStart+j < delEnd {
					pair.Left = &lines[delStart+j]
				}
				if addStart+j < addEnd {
					pair.Right = &lines[addStart+j]
				}
				pairs = append(pairs, pair)
			}

		case diff.LineAddition:
			pairs = append(pairs, linePair{Left: nil, Right: line})
			i++
		}
	}

	return pairs
}

func (v *SideBySideView) renderPairedLine(pair linePair, paneWidth int, ctx RenderContext) string {
	leftContent := v.renderSinglePane(pair.Left, paneWidth, ctx, false)
	rightContent := v.renderSinglePane(pair.Right, paneWidth, ctx, true)

	return leftContent + " │ " + rightContent
}

func (v *SideBySideView) renderSinglePane(line *diff.Line, paneWidth int, ctx RenderContext, isRight bool) string {
	if line == nil {
		return strings.Repeat(" ", paneWidth)
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
	maxContentWidth := paneWidth - len(lineNumStr) - 2
	if maxContentWidth < 0 {
		maxContentWidth = 0
	}
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	if ctx.ShowWhitespace {
		content = diff.RenderWhitespaceSimple(content, ctx.TabWidth)
	}

	text := lineNumStr + content

	style := lipgloss.NewStyle()
	switch line.Type {
	case diff.LineAddition:
		style = style.Foreground(theme.AddedLine)
	case diff.LineDeletion:
		style = style.Foreground(theme.DeletedLine)
	}

	return style.Render(truncateOrPad(text, paneWidth))
}

func (v *SideBySideView) renderSideBySideHunkHeader(text string, width int, isCurrent bool) string {
	style := lipgloss.NewStyle().Foreground(theme.Accent)
	if isCurrent {
		style = style.Background(theme.MutedBg)
	}
	return style.Render(truncateOrPad(text, width))
}
