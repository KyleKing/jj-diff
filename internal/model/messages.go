package model

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/trail"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/palette"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/theme"
)

// messages is the gm list of the jj commands this session ran: the writes, and the reads behind r.
type messages struct {
	sel   int
	reads bool
	// full shows the selected call's whole command line and stderr in place of the list.
	full bool
}

// messagesWidth is the room the list prefers inside its frame.
const messagesWidth = 96

// openMessages opens the command log on the writes, or on everything when nothing has written yet.
func (m *Model) openMessages() {
	m.closeAllModals()
	m.msgs = &messages{reads: len(m.client.Trail().Calls(true)) == 0}
}

// calls is what the list shows now, newest first.
func (m *Model) calls() []trail.Call {
	return m.client.Trail().Calls(!m.msgs.reads)
}

// handleMessagesKey moves through the list, expands a call, copies it as a shell line, or closes.
func (m *Model) handleMessagesKey(key string) tea.Cmd {
	v := m.msgs
	calls := m.calls()

	switch key {
	case "j", keyDown, "ctrl+n":
		v.sel = min(v.sel+1, max(len(calls)-1, 0))
	case "k", "up", "ctrl+p":
		v.sel = max(v.sel-1, 0)
	case "g", "home":
		v.sel = 0
	case "G", "end":
		v.sel = max(len(calls)-1, 0)
	case keyEnter:
		v.full = !v.full && len(calls) > 0
	case "r":
		v.reads, v.sel, v.full = !v.reads, 0, false
	case "Y", "y":
		if v.sel < len(calls) {
			line := calls[v.sel].Shell()
			m.say("copied "+line, false)

			return tea.SetClipboard(line)
		}
	case keyEsc, "q":
		if v.full {
			v.full = false

			return nil
		}

		m.msgs = nil
	case keyCtrlC:
		return m.cancel()
	}

	return nil
}

// messagesView is the command log's box.
func (m *Model) messagesView(width, height int) string {
	v := m.msgs
	calls := m.calls()
	inner := modal.Width(width, messagesWidth)
	rows := modal.BodyRows(height)

	title, scope := "jj commands", "writes"
	if v.reads {
		scope = "all"
	}

	hints := []keyhint.Hint{
		{Key: "enter", What: "stderr"},
		{Key: "Y", What: "copy as shell"},
		{Key: "r", What: map[bool]string{true: "writes only", false: "show reads"}[v.reads]},
		{Key: keyEsc, What: "close"},
	}

	var body []string

	switch {
	case len(calls) == 0:
		body = []string{theme.Skin.Muted.Render("no writes yet this session · r shows the reads")}
	case v.full:
		c := calls[min(v.sel, len(calls)-1)]
		body = append(body, palette.Wrap(c.Shell(), inner)...)
		body = append(body, "", theme.Skin.Muted.Render("exit "+strconv.Itoa(c.Exit)+" · "+c.Took.String()))

		for line := range strings.SplitSeq(strings.TrimRight(c.Stderr, "\n"), "\n") {
			body = append(body, palette.Wrap(line, inner)...)
		}
	default:
		body = m.callRows(calls, inner, rows)
	}

	return modal.Box(title, scope+" · "+strconv.Itoa(len(calls)), body[:min(len(body), rows)], hints, inner)
}

// callRows is the list scrolled so the selected call is in view.
func (m *Model) callRows(calls []trail.Call, width, rows int) []string {
	var (
		out   []string
		start int
	)

	for i := range calls {
		if i == m.msgs.sel {
			start = len(out)
		}

		sel := -1
		if i == m.msgs.sel {
			sel = 0
		}

		out = append(out, trail.Lines(theme.Skin, calls[i:i+1], width, sel)...)
	}

	if start >= rows {
		out = out[start-rows/2:]
	}

	return out
}
