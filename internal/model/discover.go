package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/keyusage"
	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"

	"github.com/kyleking/jj-diff/internal/theme"
)

// The palette's own commands, which no key presses: the coach's band and its tips. Each starts with
// a colon, so a run can never be mistaken for an action in the keymap.
const (
	cmdNext    = ":next"
	cmdTipsOff = ":tips off"
	cmdTipsOn  = ":tips on"
	screenWord = "the screen"
)

// WithLedger gives the coach the ledger it remembers tips and runs in across sessions, which also
// orders the palette's recent section. Without one nothing outlives the session.
func (m *Model) WithLedger(l *coach.Ledger) *Model {
	m.ledger = l
	m.discover()

	return m
}

// WithUsage records keys, actions, and refused presses into r, the opt-in usage log. A nil r
// records nothing.
func (m *Model) WithUsage(r *keyusage.Recorder) *Model {
	m.usage = r

	return m
}

// discover builds the palette and the coach, which the ledger feeds.
func (m *Model) discover() {
	m.pal = palette.New(theme.Skin)
	m.welcome = welcomeFor(m.mode, m.onlyColumns, m.keymap)
	m.welcome.OpenIfDue(m.ledger)
	m.coach = coach.New(coach.Config[coachState]{
		Ledger: m.ledger, Keymap: m.keymap, Rules: coachRules(), Tips: coachTips(), Start: m.built,
		Detectors: coachDetectors(m.keymap),
	})
}

// about is what the palette teaches for each action beyond its binding: the jj command it runs,
// with this session's revisions in it, and the git words a reader may search for it by.
func (m *Model) about() map[string]palette.About {
	src, dest := orDefault(m.revision, "S"), orDefault(m.destination, "<dest>")
	squash := "jj squash --from " + src + " --into " + dest
	split := "jj split -r " + src + " -B " + src
	resolve := "jj resolve -r " + src + " --tool :editor"

	confirm := palette.About{Cmd: squash, Also: words("commit / write / apply")}

	switch {
	case m.mode == ModeDiffEditor:
		confirm = palette.About{
			Cmd: "(writes the kept hunks back for jj)", Also: words("write / save / done"),
			Prose: []string{"jj ran jj-diff as its diff editor; what stays selected is what jj keeps."},
		}
	case m.mode == ModeResolve:
		confirm = palette.About{Cmd: resolve, Also: words("mergetool / write / save")}
	case m.cols != nil:
		confirm = palette.About{Cmd: split, Also: words("write / apply")}
	}

	return map[string]palette.About{
		actionSquash: {
			Cmd: squash, Flags: "-o/-A/-B place a new commit  -u keep dest message  -k keep emptied",
			Also: words("amend / fixup / move / absorb"),
			Prose: []string{
				"Moves the selection, else the hunk at the cursor, out of the revision into the one you pick.",
			},
		},
		actionSplit: {
			Cmd: split, Flags: "-p siblings instead of a chain",
			Also:  words("break up / commit part / carve"),
			Prose: []string{"Opens the split columns: one column per resulting commit, the source on the left."},
		},
		actionRestore: {
			Cmd:   "jj restore --changes-in " + src,
			Also:  words("unstage / discard / revert / checkout / drop"),
			Prose: []string{"Takes the selection out of the revision, asking first (y discards); u brings it back."},
		},
		actionSelect: {Also: words("stage / add / pick")},
		keys.ActionRead: {
			Also:  words("mark / reviewed / viewed / seen / unread"),
			Prose: []string{"Marks the hunk read in the store jojo and second-look share; again unmarks it."},
		},
		actionInvert:  {Also: words("toggle all / flip")},
		actionConfirm: confirm,
		actionApply: map[bool]palette.About{
			true: {Cmd: resolve, Also: words("mergetool / write")}, false: {Cmd: split, Also: words("write / commit")},
		}[m.mode == ModeResolve],
		actionPick:        {Cmd: squash, Also: words("into / destination")},
		keys.ActionReload: {Cmd: "jj diff -r " + src + " --git", Also: words("refresh / status")},
		keys.ActionUndo: map[bool]palette.About{
			true: {
				Also: words("reset / jj undo / revert"),
				Prose: []string{
					"Takes back the last change: a selection edit, else this session's last write, by restoring " +
						"the operation before it. Refused once jj has run anything since.",
				},
			},
			false: {Also: words("reset"), Prose: []string{"Undoes the last change to the selection."}},
		}[m.writesThroughJJ()],
		keys.ActionRedo: map[bool]palette.About{
			true:  {Also: words("jj redo"), Prose: []string{"Puts back what u took back."}},
			false: {},
		}[m.writesThroughJJ()],
		keys.ActionMessages:           {Also: words("log / history / trail / commands / errors")},
		keys.ActionDescribe:           {Cmd: "jj split -m", Also: words("message / reword / commit message")},
		keys.ActionDescribeEditor:     {Also: words("message / reword / editor")},
		keys.ActionMergeTool:          {Cmd: "jj resolve -r " + src, Also: words("mergetool / external")},
		keys.ActionTakeSide:           {Cmd: resolve, Also: words("ours / theirs / keep")},
		keys.ActionTakeAll:            {Also: words("both / union")},
		keys.ActionViewSideBySide:     {Also: words("split view / side by side")},
		keys.ActionWhitespace:         {Also: words("ignore space / -w")},
		keys.FlagAction("split", "p"): {Cmd: split + " --parallel", Also: words("parallel / siblings")},
	}
}

// words splits a list of search words on " / ", since one alias may be two words ("commit part").
func words(list string) []string { return strings.Split(list, " / ") }

// typed is what a reader types for a palette command, with and without its colon: the palette
// ranks an entry first when the query is exactly one of its sequences, which is how "next" finds
// :next over "next pane". It names no key, since no key presses these.
func typed(cmd string) []string { return []string{cmd, strings.TrimPrefix(cmd, ":")} }

func orDefault(s, d string) string {
	if s == "" {
		return d
	}

	return s
}

// entries is the screen's actions as the palette lists them, gated and taught, with recency from
// the ledger. The command palette adds its own commands after the keys; the legend lists keys alone.
func (m *Model) entries(mode palette.Mode) []palette.Entry {
	es := palette.FromKeymap(m.keymap, m.context(), m.gate, m.about())
	if mode == palette.Run {
		band := "next steps: show them as a band above the footer"
		if m.band {
			band = "next steps: show them in the footer's status slot"
		}

		es = append(es,
			palette.Entry{
				Action: cmdNext, What: band, Group: screenWord, Also: words("band / coach / what next"),
				Keys:  typed(cmdNext),
				Prose: []string{"The band names each next step, why, and the jj command it runs."},
			},
			palette.Entry{
				Action: cmdTipsOff, What: "stop tips about keys not yet used", Group: screenWord,
				Keys: typed(cmdTipsOff),
			},
			palette.Entry{
				Action: cmdTipsOn, What: "show tips about keys not yet used", Group: screenWord,
				Keys: typed(cmdTipsOn),
			},
			welcomeEntry(),
		)
	}

	return palette.WithSeen(es, m.coach.Ledger(), false)
}

// openPalette opens the command palette (Run) or the legend (Browse) over the screen. The search
// prompt and the file filter give way, as they would to any overlay.
func (m *Model) openPalette(mode palette.Mode) {
	m.searchModal.Hide()
	m.fileList.SetFilterMode(false)
	m.pal.SetLayout(paletteLayout(m.width))
	m.pal.Open(mode, m.entries(mode))
}

// paletteLayout is the drawer with its explainer from 100 columns and the strip below.
func paletteLayout(width int) palette.Layout {
	if width >= palette.DrawerFrom {
		return palette.Drawer
	}

	return palette.Strip
}

// runPalette carries out what the palette ran, by the action's name rather than by replaying a key,
// so a rebind cannot change what a palette run does.
func (m *Model) runPalette(run palette.RunMsg) tea.Cmd {
	switch run.Action {
	case cmdNext:
		m.band = !m.band
		m.say("next steps "+map[bool]string{true: "in a band", false: "in the status line"}[m.band], false)

		return nil
	case cmdTipsOff, cmdTipsOn:
		m.coach.Ledger().SetTipsOff(run.Action == cmdTipsOff)
		m.say("tips "+strings.TrimPrefix(run.Action, ":tips "), false)

		return nil
	case coach.WelcomeAction:
		m.welcome.Open()

		return nil
	case keys.ActionTakeSide, keys.ActionSendTo:
		// A digit names the side or the column, which a palette row cannot.
		m.say(run.Key+" takes the number itself: press 1–9", false)

		return nil
	}

	if m.refused(run.Action) {
		return nil
	}

	m.presetFlags(run.Action, run.Flags)

	cmd := m.dispatch(run.Action)
	m.usage.Action(m.contextName(), run.Action, "")
	m.coach.Observe(coach.Event{At: time.Now(), Action: run.Action, Palette: true})

	if run.From == palette.Run && m.notice == "" && run.Key != "" {
		m.say("ran "+strings.ReplaceAll(run.Action, "-", " ")+" · next time "+run.Key, false)
	}

	return cmd
}

// presetFlags turns on the squash flags typed after the palette's search, "squash -u", so the picker
// opens with them set.
func (m *Model) presetFlags(action string, flags []string) {
	if action != actionSquash && action != actionConfirm {
		return
	}

	for _, f := range flags {
		letter := strings.TrimLeft(f, "-")
		if len(letter) == 1 && !m.flags.On(letter) {
			m.flags.Toggle(letter)
		}
	}
}

// moving reports an action that only moves, which the coach watches for a long way round but never
// counts as a command run.
func moving(action string) bool {
	switch action {
	case keys.ActionLineDown, keys.ActionLineUp, keys.ActionHalfPageDown, keys.ActionHalfPageUp,
		keys.ActionPageDown, keys.ActionPageUp, keys.ActionScrollDown, keys.ActionScrollUp:
		return true
	}

	return false
}

// busy reports anything taking the reader's attention that a tip must not interrupt: a text field,
// an overlay, a picker, or a question.
func (m *Model) busy() bool {
	return m.pal.IsOpen() || m.msgs != nil || m.picker.IsVisible() || m.searchModal.IsVisible() ||
		m.fileList.IsFilterMode() || m.quitPrompt || m.discarding != nil || (m.cols != nil && m.cols.describing) ||
		len(m.matcher.Pending()) > 0
}

// idle asks the coach for a tip once the reader has stopped, saves the ledger in that lull, and arms
// the next ask.
func (m *Model) idle(msg coach.IdleMsg) tea.Cmd {
	m.coach.Tip(msg.At, m.busy())

	if err := m.coach.Ledger().Save(); err != nil {
		m.say("could not save the coach's notes: "+err.Error(), true)
	}

	return m.coach.WakeAt(msg.At)
}

// discoverMsg answers the messages the palette and the coach send back, reporting whether msg was
// one.
func (m *Model) discoverMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case palette.RunMsg:
		return m.runPalette(msg), true
	case palette.RefusedMsg:
		m.usage.Gated(m.contextName(), msg.Action)

		return nil, true
	case coach.IdleMsg:
		return m.idle(msg), true
	case tea.PasteMsg:
		if m.pal.IsOpen() {
			return m.pal.Update(msg), true
		}
	}

	return nil, false
}
