package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"

	"github.com/kyleking/jj-diff/internal/theme"
)

// welcomeFor is the first-run card for the mode jj-diff opened in: the keys a first session in it
// starts with, labeled from the keymap so a rebind changes the card.
func welcomeFor(mode OperatingMode, km *keys.Keymap) *coach.Welcome {
	w := &coach.Welcome{Title: "Welcome to jj-diff"}

	switch mode {
	case ModeDiffEditor:
		w.Lead = "jj asked which hunks to keep. Select them, then confirm to hand the selection back."
		w.Starters = coach.StartersFrom(km, actionSelect, keys.NextAction(keys.ObjHunk), actionInvert,
			actionConfirm)
	case ModeResolve:
		w.Lead = "The conflicts in this revision, one side per column. Take a side, then apply."
		w.Starters = coach.StartersFrom(km, keys.NextAction(keys.ObjConflict), keys.ActionTakeSide,
			keys.ActionTakeAll, actionApply)
	case ModeInteractive:
		w.Lead = "Select hunks or lines, then squash them into another change or split them out."
		w.Starters = coach.StartersFrom(km, actionSelect, actionSquash, actionSplit,
			keys.NextAction(keys.ObjHunk))
	case ModeBrowse:
		w.Lead = "Read a revision hunk by hunk. -i opens it to select, squash, and split."
		w.Starters = coach.StartersFrom(km, keys.NextAction(keys.ObjHunk), keys.NextAction(keys.ObjFile),
			keys.NextAction("unread"), keys.ActionOnlyUnread, keys.ActionSwitchPane)
	}

	w.HelpKeysFrom(km)

	return w
}

// welcomeKey answers a key while the card is up: any key takes it down and the dismissal is saved
// at once, since a short first session never reaches the idle lull the ledger is otherwise written
// in. It reports whether the key goes on to act, which every key but esc does.
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
