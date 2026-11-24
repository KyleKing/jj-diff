package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/empty"
	"github.com/kyleking/aragonite/tui/glyph"

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
	// A pane's body sits a border and a space in from either side.
	titleInset = 4
	// The top edge around a title: "╭─ " before it, " " and at least one "─" and "╮" after, the
	// same shape as aragonite's overlay.Box.
	titleEdge = 6
)

// pane is one labeled box of text: a side, the base, or the result.
type pane struct {
	title string
	// note is the revision's description. It follows the title in the top edge where the two fit
	// whole, and otherwise heads the box on rows of its own, so it is never cut to a few letters.
	note  string
	lines []string
	// on draws the border in the accent, for the side the region takes.
	on bool
}

// renderResolve draws the conflict viewer: the files, the region at the cursor with a pane per side
// and the base, and what the region becomes.
func (m *Model) renderResolve() string {
	r := m.res
	inner := max(m.width-2*paneMargin, 1)
	bodyHeight := max(m.screenHeight()-resolveChrome-1, 1)

	if r == nil || len(r.files) == 0 {
		e := m.noConflicts()
		if r == nil {
			e = empty.Empty{Title: "Reading the conflicts", Why: "jj resolve --list is running."}
		}

		card := empty.Card(theme.Skin, e, m.width, max(m.screenHeight()-1, 1))

		return strings.Join(append(card, m.renderStatusBar()), "\n")
	}

	left, total := r.unresolved()
	title := theme.Skin.Title.Render("Resolve") + "  " + theme.Skin.Muted.Render(fmt.Sprintf(
		"jj resolve -r %s · %d of %d regions unpicked", m.revision, left, total,
	))
	hairline := theme.Skin.Hairline.Render(strings.Repeat("─", inner))
	rows := []string{fit(title, inner), m.fileTabs(inner), hairline}

	body := m.regionRows(inner)
	body = body[min(r.scroll, max(len(body)-bodyHeight, 0)):]
	rows = append(rows, body[:min(len(body), bodyHeight)]...)

	for len(rows) < m.screenHeight()-1 {
		rows = append(rows, "")
	}

	margin := strings.Repeat(" ", min(paneMargin, m.width))
	for i := range rows {
		rows[i] = fit(margin+rows[i], m.width)
	}

	return strings.Join(append(rows[:m.screenHeight()-1], m.renderStatusBar()), "\n")
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
		id, desc := splitLabel(s.Label)
		out = append(out, pane{
			title: fmt.Sprintf("%d %s", i+1, id),
			note:  glyph.Settle(desc),
			lines: s.Lines(),
			on:    (p.Kind == conflict.TakeSide && p.Side == i+1) || p.Kind == conflict.TakeAll,
		})

		if i == 0 && len(region.Bases) > 0 {
			id, desc := splitLabel(region.Bases[0].Label)
			out = append(out, pane{
				title: "0 base " + id, note: glyph.Settle(desc), lines: region.Bases[0].Lines(),
				on: p.Kind == conflict.TakeBase,
			})
		}
	}

	return out
}

// splitLabel parts jj's side label (change ID, commit ID, then the quoted description) into the
// change ID and the description, dropping the commit hash, since the change ID already names the
// revision and the description is what a reader recognizes. A label in any other shape is all ID.
func splitLabel(label string) (string, string) {
	f := strings.Fields(label)
	const fields = 3 // change ID, commit ID, description

	if len(f) < fields || !isHex(f[1]) {
		return label, ""
	}

	desc := strings.Join(f[2:], " ")
	if unquoted, err := strconv.Unquote(desc); err == nil {
		desc = unquoted
	}

	return f[0], desc
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

	widths := paneWidths(panes, width-(n-1)*paneGap)
	boxes := make([][]string, n)
	height := 0

	for i, p := range panes {
		boxes[i] = box(p, widths[i])
		height = max(height, len(boxes[i]))
	}

	rows := make([]string, height)
	for i := range height {
		parts := make([]string, n)

		for j, b := range boxes {
			parts[j] = strings.Repeat(" ", widths[j])
			if i < len(b) {
				parts[j] = b[i]
			}
		}

		rows[i] = strings.Join(parts, strings.Repeat(" ", paneGap))
	}

	return rows
}

// paneWidths shares avail among panes side by side by what each needs to show its title and lines
// whole: a pane needing less than an even share gets what it needs, the others split the rest
// evenly, and room left once every pane has its need is spread evenly so the row fills avail.
func paneWidths(panes []pane, avail int) []int {
	n := len(panes)
	needs := make([]int, n)
	order := make([]int, n)

	for i, p := range panes {
		needs[i], order[i] = paneNeed(p), i
	}

	sort.SliceStable(order, func(a, b int) bool { return needs[order[a]] < needs[order[b]] })

	widths := make([]int, n)
	left := avail

	for k, i := range order {
		share := left / (n - k)
		if needs[i] > share {
			// Every pane from here on needs more than an even share of what is left, so they split it.
			for j, rest := range order[k:] {
				widths[rest] = left / (n - k)
				if j == n-k-1 {
					widths[rest] += left % (n - k)
				}
			}

			return widths
		}

		widths[i] = needs[i]
		left -= needs[i]
	}

	for i := range widths {
		widths[i] += left / n
	}

	widths[n-1] += left % n

	return widths
}

// paneNeed is the width that draws p's title, its note beside it, and its widest line uncut.
func paneNeed(p pane) int {
	need := lipgloss.Width(fullTitle(p)) + titleEdge

	for _, l := range p.lines {
		need = max(need, lipgloss.Width(expand(l))+titleInset)
	}

	return max(need, minPaneWidth)
}

func fullTitle(p pane) string {
	if p.note == "" {
		return p.title
	}

	return p.title + ` "` + p.note + `"`
}

// noteRows is the most rows a note that does not fit in the top edge wraps to before it is cut.
const noteRows = 2

// box draws a pane in a rounded border width cells wide, its title in the top edge, with its note
// beside the title when both fit and wrapped under it when they do not.
func box(p pane, width int) []string {
	edge := theme.Skin.Edge
	if p.on {
		edge = theme.Skin.Accent
	}

	inner := max(width-titleInset, 1)
	room := max(width-titleEdge, 0)

	title := fullTitle(p)

	var lines []string

	if lipgloss.Width(title) > room {
		title = ansi.Truncate(p.title, room, "…")
		lines = wrapNote(p.note, inner)
	}

	top := "╭─ " + title + " " + strings.Repeat("─", max(width-lipgloss.Width(title)-titleEdge+1, 0)) + "╮"

	rows := []string{edge.Render(top)}

	if len(p.lines) == 0 {
		lines = append(lines, theme.Skin.Muted.Render("(empty)"))
	}

	lines = append(lines, p.lines...)

	for _, l := range lines {
		rows = append(rows, edge.Render("│")+" "+fit(expand(l), inner)+" "+edge.Render("│"))
	}

	return append(rows, edge.Render("╰"+strings.Repeat("─", max(width-borders, 0))+"╯"))
}

// wrapNote breaks a note into at most noteRows rows of width cells, cutting the last with an
// ellipsis when the note runs longer.
func wrapNote(note string, width int) []string {
	if note == "" {
		return nil
	}

	rows := strings.Split(ansi.Wrap(note, width, ""), "\n")
	if len(rows) > noteRows {
		rows = rows[:noteRows]
		rows[noteRows-1] = ansi.Truncate(rows[noteRows-1]+" …", width, "…")
	}

	for i := range rows {
		rows[i] = theme.Skin.Muted.Render(strings.TrimRight(rows[i], " "))
	}

	return rows
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
