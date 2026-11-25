package model

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"

	"github.com/kyleking/jj-diff/internal/theme"
)

// welcomeFor is the first-run card for the screen jj-diff opened on: the keys a first session there
// starts with, labeled from the keymap so a rebind changes the card. A session launched into the
// split columns (columns) is taught the columns' keys, not the interactive diff's it never shows.
func welcomeFor(mode OperatingMode, columns bool, km *keys.Keymap) *coach.Welcome {
	w := &coach.Welcome{Title: "Welcome to jj-diff"}

	switch {
	case columns:
		w.Lead = "One column per commit the split makes, the revision on the left. Move changes " +
			"into the columns, then apply."
		w.Starters = starters(km, actionSelect, keys.ActionMoveRight, keys.ActionSendTo, keys.ActionDescribe,
			actionApply)
	case mode == ModeDiffEditor:
		w.Lead = "jj asked which hunks to keep. Select them, then confirm to hand the selection back."
		w.Starters = starters(km, actionSelect, keys.NextAction(keys.ObjHunk), actionInvert, actionConfirm)
	case mode == ModeResolve:
		w.Lead = "The conflicts in this revision, one side per column. Take a side, then apply."
		w.Starters = starters(km, keys.NextAction(keys.ObjConflict), keys.ActionTakeSide,
			keys.ActionTakeAll, actionApply)
	case mode == ModeInteractive:
		w.Lead = "Select hunks or lines, then squash them into another change or split them out."
		w.Starters = starters(km, actionSelect, actionSquash, actionSplit, keys.NextAction(keys.ObjHunk))
	default:
		w.Lead = "Read a revision hunk by hunk. -i opens it to select, squash, and split."
		w.Starters = starters(km, keys.NextAction(keys.ObjHunk), keys.NextAction(keys.ObjFile),
			keys.NextAction("unread"), keys.ActionOnlyUnread, keys.ActionSwitchPane)
	}

	w.HelpKeysFrom(km)

	return w
}

// starters is the card's lines for actions, each keyed as the legend keys it, so a binding on every
// digit reads "1–9" rather than its first digit.
func starters(km *keys.Keymap, actions ...string) []keyhint.Hint {
	out := coach.StartersFrom(km, actions...)

	for i := range out {
		for _, a := range actions {
			if b, ok := km.Binding(a); ok && b.What == out[i].What && slices.Equal(b.Keys, keys.Digits()) {
				out[i].Key = b.Labels()
			}
		}
	}

	return out
}

// welcomeKey answers a key while the card is up: any key takes it down and the dismissal is saved
// at once, since a short first session never reaches the idle lull the ledger is otherwise written
// in. It reports whether the key goes on to act, which only the card's own help and palette keys do:
// any other key only closes the card, so a first keystroke never writes.
func (m *Model) welcomeKey(key string) bool {
	if !m.welcome.Showing() {
		return true
	}

	pass := m.welcome.Key(m.ledger, key, time.Now())
	if err := m.ledger.Save(); err != nil {
		m.say(fmt.Sprintf("could not save the coach's notes: %v", err), true)
	}

	return pass
}

// withWelcome lays the card over every row but the footer, or where the screen is too small for it,
// says it in the footer instead.
func (m *Model) withWelcome(screen string) string {
	if !m.welcome.Showing() {
		return screen
	}

	lines := strings.Split(screen, "\n")
	last := len(lines) - 1

	if last < 1 {
		return screen
	}

	if !coach.WelcomeFits(m.width, last) {
		lines[last] = " " + m.welcome.Line(theme.Skin, m.width-1)

		return strings.Join(lines, "\n")
	}

	return strings.Join(append(m.welcome.Over(theme.Skin, lines[:last], m.width, last), lines[last]), "\n")
}

// welcomeEntry is :welcome as the palette lists it, typed with or without its colon.
func welcomeEntry() palette.Entry {
	e := coach.WelcomeEntry()
	e.Group, e.Keys = screenWord, typed(coach.WelcomeAction)

	return e
}
