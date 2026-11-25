package model

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/seen"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/reads"
)

// Read marks are shared with jojo and second-look: browse's `␣` marks the hunk under the cursor read
// in the same store, and every mode that views a revision shows them. `-i` and the split columns
// keep `␣` on select and never mark.

// scopeMsg is the change id the revision names, which its marks are filed under, and what moving
// marks an older jojo filed under a prefix of it answered.
type scopeMsg struct {
	err     error
	migrate error
	scope   string
}

// WithReads shows the marks in store on a revision viewed in browse or interactive mode, and binds
// ]u and [u to step between unread hunks. A nil store, the diff editor (two directories, no change
// to scope marks by), and the conflict viewer show none.
func (m *Model) WithReads(store *seen.Store) *Model {
	if store == nil || m.revision == "" || m.client == nil || !showsReads(m.mode) {
		return m
	}

	m.marks = reads.NewMarks(store)
	m.keymap = newKeymap(m.mode, true)
	m.matcher = keys.NewMatcher(m.keymap)
	m.discover()

	return m
}

// loadScope reads the change id the revision names now, which a revset such as @ can move between
// loads, and moves the marks an older jojo filed under a prefix of it into the full id's file
// before they are read.
func (m *Model) loadScope() tea.Cmd {
	client, revision, store := m.client, m.revision, m.marks.Store()

	return func() tea.Msg {
		scope, err := client.ScopeID(revision)
		if err != nil {
			return scopeMsg{err: err}
		}

		_, migrate := store.Migrate(scope)

		return scopeMsg{scope: scope, migrate: migrate}
	}
}

// syncReads rereads the marks when another process saved them since the last look, and hands the
// file list and the diff pane what to draw. It costs a stat when nothing changed, so it runs on
// every message rather than on a timer.
func (m *Model) syncReads() {
	if !m.marks.On() {
		return
	}

	m.marks.Refresh()

	if !m.marks.Ready() {
		m.fileList.SetReads(nil)
		m.fileList.SetOnlyUnread(false)
		m.diffView.SetHidden(nil)

		return
	}

	files := make([]filelist.FileReads, len(m.changes))
	for i := range m.changes {
		files[i].Read, files[i].Total = m.marks.File(i)
	}

	m.fileList.SetReads(files)
	m.fileList.SetOnlyUnread(m.onlyUnread)
	m.diffView.SetHidden(m.hiddenHunks())
}

// hiddenHunks is what U hides of the current file: every read hunk but the one under the cursor,
// which stays drawn whole so a reader who steps onto a read hunk can still read it. Nil with U off.
func (m *Model) hiddenHunks() []bool {
	if !m.onlyUnread || m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return nil
	}

	hidden := make([]bool, len(m.changes[m.selectedFile].Hunks))
	for h := range hidden {
		hidden[h] = h != m.selectedHunk && m.marks.Read(m.selectedFile, h)
	}

	return hidden
}

// toggleOnlyUnread is U: only what is unread, or everything again. Turned on over something read, it
// steps to the next unread hunk, so the screen opens on what is left to read.
func (m *Model) toggleOnlyUnread() {
	m.onlyUnread = !m.onlyUnread

	if !m.onlyUnread {
		m.say("showing every hunk", false)

		return
	}

	if m.selectedFile < len(m.changes) && m.marks.Read(m.selectedFile, m.selectedHunk) {
		panel := m.focusedPanel
		m.stepUnread(false)
		m.focusedPanel = panel
	}

	m.say("showing only unread hunks · U shows every hunk", false)
}

// onlyUnreadWord is U's footer word: what pressing it would do to the read hunks.
func (m *Model) onlyUnreadWord() string {
	if m.onlyUnread {
		return "show read"
	}

	return "hide read"
}

// onlyUnreadGate refuses U until the marks are loaded, and turning it on once nothing is unread.
// Turning it off is always allowed.
func (m *Model) onlyUnreadGate() keys.Availability {
	switch {
	case m.onlyUnread:
		return keys.Allow()
	case !m.marks.Ready():
		return keys.Refuse("read marks are still loading")
	case m.marks.Unread() == 0:
		return keys.Refuse("every hunk is read")
	}

	return keys.Allow()
}

// fileShown reports whether file i is in the list: always, except a file read whole while U is on.
func (m *Model) fileShown(i int) bool {
	return !m.onlyUnread || !m.marks.Ready() || !m.marks.FileRead(i)
}

// pushReads hands the diff pane which of the current file's hunks are read.
func (m *Model) pushReads() {
	if !m.marks.Ready() {
		m.diffView.SetReadState(nil)

		return
	}

	marks, file := m.marks, m.selectedFile

	m.diffView.SetReadState(func(hunkIdx int) bool { return marks.Read(file, hunkIdx) })
}

// stepUnread moves to the next unread hunk after the cursor in patch order, or the previous one
// before it, wrapping past either end, and says so when every hunk is read.
func (m *Model) stepUnread(back bool) {
	type place struct{ file, hunk int }

	var all []place

	at := -1

	for f := range m.changes {
		for h := range m.changes[f].Hunks {
			if f == m.selectedFile && h == m.selectedHunk {
				at = len(all)
			}

			all = append(all, place{f, h})
		}
	}

	n := len(all)
	if at < 0 && back {
		at = n
	}

	for i := 1; i <= n; i++ {
		k := ((at+i)%n + n) % n
		if back {
			k = ((at-i)%n + n) % n
		}

		if p := all[k]; !m.marks.Read(p.file, p.hunk) {
			if p.file != m.selectedFile {
				m.jumpToFile(p.file)
			}

			m.focusedPanel = PanelDiffView
			m.selectedHunk, m.lineCursor, m.isVisualMode = p.hunk, 0, false
			m.diffView.Show(p.hunk, m.diffHeight())

			return
		}
	}

	m.say("every hunk is read", false)
}

// unreadGate refuses the unread motions until the marks are loaded, and once nothing is unread.
func (m *Model) unreadGate() keys.Availability {
	switch {
	case !m.marks.Ready():
		return keys.Refuse("read marks are still loading")
	case m.marks.Unread() == 0:
		return keys.Refuse("every hunk is read")
	}

	return keys.Allow()
}

// markRead is browse's space: the hunk under the cursor goes read, or unread when it already is,
// written to the shared store at once.
func (m *Model) markRead() {
	read, err := m.marks.Toggle(m.selectedFile, m.selectedHunk)
	if err != nil {
		m.say(err.Error(), true)

		return
	}

	m.syncReads()
	m.pushReads()

	word := "unread"
	if read {
		word = "read"
	}

	m.say(fmt.Sprintf("hunk %d marked %s", m.selectedHunk+1, word), false)
}

// readGate refuses space where no mark can be kept: no store or change id (the diff editor sees two
// directories, and a directory outside a jj repository has no store), marks still loading, or no
// hunk under the cursor.
func (m *Model) readGate() keys.Availability {
	switch {
	case !m.marks.On():
		return keys.Refuse("read marks need a revision in a jj repository")
	case !m.marks.Ready():
		return keys.Refuse("read marks are still loading")
	case m.selectedFile < 0 || m.selectedFile >= len(m.changes) || len(m.changes[m.selectedFile].Hunks) == 0:
		return keys.Refuse("no hunk under the cursor")
	}

	return keys.Allow()
}
