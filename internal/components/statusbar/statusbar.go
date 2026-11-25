// Package statusbar renders the single-line footer: the mode, the diff's source and destination, and
// the keybinding hints the parent's keymap says work where the user is. It holds no keys of its own,
// so the footer cannot drift from what the keys do.
package statusbar

import (
	"math"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	ellipsis = "…"

	// The footer keeps margin columns off the terminal's edge, in line with the panes above it.
	margin = 2
	// The facts are parted from the hints as one hint is from the next.
	partGap = theme.HintGap
	// Two part one fact from the next.
	factGap = "  "

	sourceShare    = 2
	minSourceWidth = 12
)

// Context is what the footer describes. Destination is omitted from the render when empty. Hints
// are the keys that work here, most useful first: Fit sheds from the end, and a Pin hint stays.
// Notice is a message about the last thing done, which takes the place of the facts while it is up.
type Context struct {
	Destination string
	Mode        string
	Source      string
	Notice      string
	// Lead is the coach's next step or tip, drawn whole after the mode where it fits, in place of the
	// source and destination when only that makes room, and left out otherwise.
	Lead string
	// LeadKey is the key the lead names, whose hint gives its room to the others while the lead is
	// drawn.
	LeadKey string
	// Badge is an open text field's mode (INSERT, NORMAL), drawn just before the hints it names the
	// keys of, and kept while they shed.
	Badge        string
	Hints        []keyhint.Hint
	IsVisualMode bool
	// Alarm draws Notice as a failure rather than news.
	Alarm bool
}

// key is everything the drawn line depends on, comparable so the last line can be reused.
func (c *Context) key() string {
	var b strings.Builder

	b.WriteString(c.Destination + "\x00" + c.Mode + "\x00" + c.Source + "\x00" + c.Notice + "\x00" + c.Lead +
		"\x00" + c.LeadKey + "\x00" + c.Badge)

	if c.IsVisualMode {
		b.WriteString("\x00v")
	}

	if c.Alarm {
		b.WriteString("\x00a")
	}

	for _, h := range c.Hints {
		b.WriteString("\x00" + h.Key + "\x01" + h.What)

		if h.Pin {
			b.WriteString("\x01p")
		}

		if h.Off {
			b.WriteString("\x01o")
		}

		if h.Suggest {
			b.WriteString("\x01s")
		}
	}

	return b.String()
}

// Model renders the footer entirely from the Context passed to each call. It keeps the last line it
// drew, since the footer is redrawn on every key and its context rarely changes between two.
type Model struct {
	last *drawnLine
}

type drawnLine struct {
	line       string
	key        string
	generation uint64
	width      int
}

// New returns the footer renderer.
func New() Model {
	return Model{last: &drawnLine{}}
}

// ViewWithContext renders the footer to exactly width cells: the mode and the revisions it moves
// between as quiet facts, then the keys that work here in the shape every sibling tool's footer
// takes, the key in the accent and what it does muted. Hints that do not fit are shed from the end
// of the list, and the help hint always survives because it is the only route to the rest of the
// keymap, as esc does in a visual selection. The facts give way before either does.
func (m Model) ViewWithContext(width int, ctx *Context) string {
	if width <= 0 {
		return ""
	}

	if m.last == nil {
		return m.draw(width, ctx)
	}

	gen, key := theme.Generation(), ctx.key()
	if m.last.width != width || m.last.key != key || m.last.generation != gen {
		*m.last = drawnLine{line: m.draw(width, ctx), key: key, width: width, generation: gen}
	}

	return m.last.line
}

func (Model) draw(width int, ctx *Context) string {
	lead := strings.Repeat(" ", min(margin, width))
	plain := facts(ctx, width)
	facts := withLead(ctx, plain, width)
	hints := ctx.Hints

	if facts != plain && ctx.LeadKey != "" {
		hints = slices.DeleteFunc(slices.Clone(hints), func(h keyhint.Hint) bool {
			return h.Key == ctx.LeadKey && !h.Pin
		})
	}

	badge := ""
	if ctx.Badge != "" {
		badge = ctx.Badge + " "
	}

	room := width - margin - lipgloss.Width(facts) - len(partGap) - lipgloss.Width(badge)

	if room < lipgloss.Width(keyhint.Pinned(hintStyles(), hints)) {
		facts, room = "", width-margin-lipgloss.Width(badge)
	}

	line := badge + keyhint.Fit(hintStyles(), hints, room)
	if facts != "" {
		line = facts + partGap + line
	}

	return padTo(lead+line, width)
}

// hintStyles is the footer face second-look and jojo share: bare keys in the accent, words muted.
func hintStyles() keyhint.Styles { return theme.Hints() }

// leadShare is the part of the footer the hints keep however long the lead is: the lead is drawn
// whole or not at all, and never at the cost of the map a reader orients by.
const leadShare = 3

// withLead is the facts with the coach's lead after them: after the mode and the revisions where it
// fits whole, after the mode alone where only that makes room, and left out otherwise.
func withLead(ctx *Context, facts string, width int) string {
	if ctx.Lead == "" || ctx.Notice != "" {
		return facts
	}

	room := width - margin - len(partGap) - lipgloss.Width(keyhint.Pinned(hintStyles(), ctx.Hints)) - width/leadShare
	lead := lipgloss.Width(ctx.Lead)

	if lipgloss.Width(facts)+len(factGap)+lead <= room {
		return facts + factGap + ctx.Lead
	}

	mode := theme.Skin.Subtitle.Render(ctx.Mode)
	if lipgloss.Width(mode)+len(factGap)+lead <= room {
		return mode + factGap + ctx.Lead
	}

	// On a narrow footer the next step is worth more than the mode's name, except in visual mode,
	// whose marker is the only sign keys extend a range.
	if !ctx.IsVisualMode && lead <= room {
		return ctx.Lead
	}

	return facts
}

// facts is the mode, then the source and destination, each a step quieter than the one before, or
// the notice in their place while one is up.
func facts(ctx *Context, width int) string {
	if ctx.Notice != "" {
		face := theme.Skin.Accent
		if ctx.Alarm {
			face = theme.Skin.Error
		}

		// A notice is what the reader is looking at, and a write's names its whole jj command, so it
		// takes all but the pinned hints' room.
		room := width - margin - len(partGap) - lipgloss.Width(keyhint.Pinned(hintStyles(), ctx.Hints))

		return face.Render(ellipsize(ctx.Notice, max(room, minSourceWidth)))
	}

	mode := theme.Skin.Subtitle.Render(ctx.Mode)
	if ctx.IsVisualMode {
		mode += " " + theme.Skin.Cursor.Render("VISUAL")
	}

	parts := []string{mode}
	if ctx.Source != "" {
		// The revision's name gives way to the keys a reader acts with: it keeps at most its share,
		// less whatever the leading hints need to be drawn beside it.
		room := width - margin - lipgloss.Width(mode) - len(factGap) - len(partGap) - leadingHints(ctx.Hints)
		source := ellipsize(ctx.Source, max(min(width/sourceShare-margin, room), minSourceWidth))
		parts = append(parts, theme.Skin.Muted.Render(source))
	}

	if ctx.Destination != "" {
		parts = append(parts, theme.Skin.Accent.Render("→ ")+theme.Skin.Muted.Render(ctx.Destination))
	}

	return strings.Join(parts, factGap)
}

// keptHints is how many of the footer's most useful hints the revision's name makes room for.
const keptHints = 5

// leadingHints is the width the keptHints most useful hints and the pinned one take drawn together.
func leadingHints(hints []keyhint.Hint) int {
	byUse := slices.Clone(hints)
	slices.SortStableFunc(byUse, func(a, b keyhint.Hint) int { return b.Priority - a.Priority })

	keep := make([]keyhint.Hint, 0, keptHints+1)
	for i, h := range byUse {
		if i < keptHints || h.Pin {
			keep = append(keep, h)
		}
	}

	return lipgloss.Width(keyhint.Fit(hintStyles(), keep, math.MaxInt32))
}

// ellipsize cuts text to at most width display cells, ending in an ellipsis when it cut.
func ellipsize(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}

	return ansi.Truncate(text, width-1, "") + ellipsis
}

// padTo fits a styled line to exactly width cells, measuring in cells so a wide glyph cannot push
// the footer onto a second row.
func padTo(line string, width int) string {
	if used := lipgloss.Width(line); used > width {
		line = ansi.Truncate(line, width, "")
	}

	return line + strings.Repeat(" ", max(width-lipgloss.Width(line), 0))
}
