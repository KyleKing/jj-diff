package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/conflict"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Layout of the conflict viewer.
const (
	// The title, the file row, and the hairline over the body.
	resolveChrome = 3
	// How much of the file shows above and below a region.
	contextLines = 3
	// The narrowest a side's pane gets before the sides stack instead.
	minPaneWidth = 24
	// The gap between two panes side by side.
	paneGap = 1
	// A border cell either side of a pane.
	borders = 2
	// A pane's title skips the corner and a space at either end.
	titleInset = 4
)

// pane is one labeled box of text: a side, the base, or the result.
type pane struct {
	title string
	lines []string
	// on draws the border in the accent, for the side the region takes.
	on bool
}

// renderResolve draws the conflict viewer: the files, the region at the cursor with a pane per side
// and the base, and what the region becomes.
func (m *Model) renderResolve() string {
	r := m.res
	inner := max(m.width-2*paneMargin, 1)
	bodyHeight := max(m.height-resolveChrome-1, 1)

	var rows []string

	if r == nil || len(r.files) == 0 {
		note := "reading the conflicts…"
		if r != nil {
			note = m.source + " has no conflicts left · q quits"
		}

		rows = []string{theme.Skin.Title.Render("Resolve") + "  " + theme.Skin.Muted.Render(note)}
	} else {
		left, total := r.unresolved()
		title := theme.Skin.Title.Render("Resolve") + "  " + theme.Skin.Muted.Render(fmt.Sprintf(
			"jj resolve -r %s · %d of %d regions unpicked", m.revision, left, total,
		))
		hairline := theme.Skin.Hairline.Render(strings.Repeat("─", inner))
		rows = append(rows, fit(title, inner), m.fileTabs(inner), hairline)

		body := m.regionRows(inner)
		body = body[min(r.scroll, max(len(body)-bodyHeight, 0)):]
		rows = append(rows, body[:min(len(body), bodyHeight)]...)
	}

	for len(rows) < m.height-1 {
		rows = append(rows, "")
	}

	margin := strings.Repeat(" ", min(paneMargin, m.width))
	for i := range rows {
		rows[i] = fit(margin+rows[i], m.width)
	}

	return strings.Join(append(rows[:m.height-1], m.renderStatusBar()), "\n")
}

// fileTabs is the row of conflicted files, each with how many of its regions are picked.
func (m *Model) fileTabs(width int) string {
	r := m.res
	tabs := make([]string, 0, len(r.files))

	for i, f := range r.files {
		label := fmt.Sprintf(" %s %d/%d ", f.path, len(r.picks[i]), len(f.parsed.Regions()))

		style := theme.Skin.Tab
		if i == r.file {
			style = theme.Skin.TabActive
		}

		tabs = append(tabs, style.Render(label))
	}

	return fit(strings.Join(tabs, " "), width)
}

// regionRows is the body: where the region sits, its panes, and what it becomes.
func (m *Model) regionRows(width int) []string {
	r := m.res
	file := r.files[r.file]
	regions := file.parsed.Regions()

	if len(regions) == 0 {
		return []string{
			theme.Skin.Warning.Render(file.path + ": " + file.kind),
			theme.Skin.Muted.Render("jj-diff reads text conflicts only; M hands this file to your merge tool"),
		}
	}

	region := regions[r.region]
	p := r.picks[r.file][r.region]
	before, after := file.parsed.Around(r.region)

	state := theme.Skin.Warning.Render("unpicked")
	if p.Kind != conflict.Unresolved {
		state = theme.Skin.Success.Render("takes " + pickName(p))
	}

	rows := []string{
		fit(theme.Skin.Heading.Render(fmt.Sprintf("conflict %d of %d", r.region+1, len(regions)))+
			theme.Skin.Muted.Render(" in "+file.path+" · "+file.kind+" · ")+state, width),
	}

	for _, l := range lastLines(before, contextLines) {
		rows = append(rows, fit(theme.Skin.Muted.Render("  "+expand(l)), width))
	}

	rows = append(rows, m.paneRows(sidePanes(region, p), width)...)

	result := pane{title: "result", lines: []string{theme.Skin.Muted.Render("unpicked: the markers stay")}}
	if p.Kind != conflict.Unresolved {
		text, _ := region.Text(p)
		result = pane{title: "result · " + pickName(p), lines: conflict.Term{Text: text}.Lines(), on: true}

		if text == "" {
			result.lines = []string{theme.Skin.Muted.Render("(nothing: the region is removed)")}
		}
	}

	rows = append(rows, m.paneRows([]pane{result}, width)...)

	for _, l := range firstLines(after, contextLines) {
		rows = append(rows, fit(theme.Skin.Muted.Render("  "+expand(l)), width))
	}

	return rows
}

// sidePanes is a pane per side, numbered as the digit that takes it, and the base as 0, in jj's
// order with the base between the first two sides.
func sidePanes(region *conflict.Region, p conflict.Pick) []pane {
	var out []pane

	for i, s := range region.Sides {
		out = append(out, pane{
			title: fmt.Sprintf("%d %s", i+1, shortLabel(s.Label)),
			lines: s.Lines(),
			on:    (p.Kind == conflict.TakeSide && p.Side == i+1) || p.Kind == conflict.TakeAll,
		})

		if i == 0 && len(region.Bases) > 0 {
			out = append(out, pane{
				title: "0 base " + shortLabel(region.Bases[0].Label), lines: region.Bases[0].Lines(),
				on: p.Kind == conflict.TakeBase,
			})
		}
	}

	return out
}

// shortLabel drops the commit hash from jj's side label (change ID, commit ID, then the quoted
// description), since the change ID already names the revision and the description is what a reader
// recognizes.
func shortLabel(label string) string {
	f := strings.Fields(label)
	const fields = 3 // change ID, commit ID, description

	if len(f) < fields || !isHex(f[1]) {
		return label
	}

	return f[0] + " " + strings.Join(f[2:], " ")
}

func isHex(s string) bool {
	return strings.Trim(s, "0123456789abcdef") == ""
}

// paneRows lays panes side by side where each gets minPaneWidth, else one above another.
func (m *Model) paneRows(panes []pane, width int) []string {
	n := len(panes)
	if m.res.stacked || n == 1 || (width-(n-1)*paneGap)/n < minPaneWidth {
		rows := make([]string, 0, len(panes)*(borders+1))
		for _, p := range panes {
			rows = append(rows, box(p, width)...)
		}

		return rows
	}

	w := (width - (n-1)*paneGap) / n
	boxes := make([][]string, n)
	height := 0

	for i, p := range panes {
		boxes[i] = box(p, w)
		height = max(height, len(boxes[i]))
	}

	rows := make([]string, height)
	for i := range height {
		parts := make([]string, n)

		for j, b := range boxes {
			parts[j] = strings.Repeat(" ", w)
			if i < len(b) {
				parts[j] = b[i]
			}
		}

		rows[i] = strings.Join(parts, strings.Repeat(" ", paneGap))
	}

	return rows
}

// box draws a pane in a rounded border width cells wide, its title in the top edge.
func box(p pane, width int) []string {
	edge := theme.Skin.Edge
	if p.on {
		edge = theme.Skin.Accent
	}

	inner := max(width-titleInset, 1)
	title := ansi.Truncate(p.title, max(width-titleInset-1, 0), "…")
	top := "╭ " + title + " " + strings.Repeat("─", max(width-lipgloss.Width(title)-titleInset, 0)) + "╮"

	rows := []string{edge.Render(top)}

	lines := p.lines
	if len(lines) == 0 {
		lines = []string{theme.Skin.Muted.Render("(empty)")}
	}

	for _, l := range lines {
		rows = append(rows, edge.Render("│")+" "+fit(expand(l), inner)+" "+edge.Render("│"))
	}

	return append(rows, edge.Render("╰"+strings.Repeat("─", max(width-borders, 0))+"╯"))
}

// pickName says what a pick takes.
func pickName(p conflict.Pick) string {
	switch p.Kind {
	case conflict.TakeSide:
		return fmt.Sprintf("side #%d", p.Side)
	case conflict.TakeBase:
		return "the base"
	case conflict.TakeAll:
		return "every side"
	case conflict.Custom:
		return "your edit"
	case conflict.Unresolved:
	}

	return "nothing"
}

func expand(s string) string { return strings.ReplaceAll(s, "\t", tabSpaces) }

func lastLines(text string, n int) []string {
	lines := conflict.Term{Text: text}.Lines()

	return lines[max(len(lines)-n, 0):]
}

func firstLines(text string, n int) []string {
	lines := conflict.Term{Text: text}.Lines()

	return lines[:min(len(lines), n)]
}
