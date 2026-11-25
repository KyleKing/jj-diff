package model

import (
	"context"
	"fmt"
	"maps"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/conflict"
	"github.com/kyleking/jj-diff/internal/jj"
)

// conflictFile is one conflicted file the viewer holds: what jj materialized, and its regions.
type conflictFile struct {
	path   string
	kind   string
	text   string
	parsed conflict.File
}

// picks is every file's choices, by file index and then region index.
type picks []map[int]conflict.Pick

func (p picks) clone() picks {
	out := make(picks, len(p))
	for i, m := range p {
		out[i] = maps.Clone(m)
	}

	return out
}

// resolveState is the conflict viewer: the files, what has been picked, and where the cursor is.
type resolveState struct {
	files  []conflictFile
	picks  picks
	undo   []picks
	redo   []picks
	file   int
	region int
	// scroll is how far the region view is scrolled down from the region's first row.
	scroll int
	// stacked draws the sides one above another instead of side by side.
	stacked bool
}

type conflictsLoadedMsg struct {
	err   error
	files []jj.ConflictedFile
}

func (m *Model) loadConflicts() tea.Cmd {
	client, revision := m.client, m.revision

	return func() tea.Msg {
		files, err := client.Conflicts(revision)

		return conflictsLoadedMsg{files: files, err: err}
	}
}

// conflictsLoaded takes a fresh read of the revision's conflicts, which starts the picks over. After
// a write in a session launched for one, a read that finds none left quits.
func (m *Model) conflictsLoaded(msg conflictsLoadedMsg) tea.Cmd {
	leave := m.leaveIfResolved
	m.leaveIfResolved = false

	if msg.err != nil {
		m.err = msg.err

		return nil
	}

	if leave && len(msg.files) == 0 {
		return tea.Quit
	}

	if leave {
		m.notice += " · " + plural(len(msg.files), "file") + " still conflicted, q returns"
	}

	res := &resolveState{}

	if m.res != nil {
		res.stacked = m.res.stacked
	}

	for _, f := range msg.files {
		file := conflictFile{path: f.Path, kind: f.Kind, text: f.Text, parsed: conflict.Parse(f.Text)}
		res.files = append(res.files, file)
		res.picks = append(res.picks, map[int]conflict.Pick{})
	}

	m.res = res

	return nil
}

// regions is the regions of the file at the cursor.
func (r *resolveState) regions() []*conflict.Region {
	if r.file >= len(r.files) {
		return nil
	}

	return r.files[r.file].parsed.Regions()
}

// currentRegion is the region at the cursor, false when the file has none the viewer can read.
func (r *resolveState) currentRegion() (*conflict.Region, bool) {
	regions := r.regions()
	if r.region < 0 || r.region >= len(regions) {
		return nil, false
	}

	return regions[r.region], true
}

// unresolved counts the regions with no pick, across every file.
func (r *resolveState) unresolved() (int, int) {
	left, total := 0, 0

	for i, f := range r.files {
		for j := range f.parsed.Regions() {
			total++

			if r.picks[i][j].Kind == conflict.Unresolved {
				left++
			}
		}
	}

	return left, total
}

// resolveAction runs one action in the conflict viewer.
func (m *Model) resolveAction(action string, pressed []string) tea.Cmd {
	if cmd, ok := m.resolveGeneral(action); ok {
		return cmd
	}

	if cmd, ok := m.resolveHistory(action); ok {
		return cmd
	}

	r := m.res
	if r == nil || len(r.files) == 0 {
		return nil
	}

	if step, ok := m.motion.Resolve(action); ok {
		m.resolveStep(step)

		return nil
	}

	if m.pickAction(action, pressed) {
		return nil
	}

	switch action {
	case keys.ActionEditRegion:
		return m.editRegion()
	case keys.ActionMergeTool:
		return m.mergeTool()
	case keys.ActionUndo:
		m.pickHistory(&r.undo, &r.redo, "undo")
	case keys.ActionRedo:
		m.pickHistory(&r.redo, &r.undo, "redo")
	case keys.ActionViewSideBySide:
		r.stacked = !r.stacked
	case actionConfirm, actionApply:
		return m.writeResolution()
	default:
		m.resolveScroll(action)
	}

	return nil
}

// resolveHistory is u and ctrl+r once the picks have no history left: the session's writes, which
// outlive the conflicts they resolved, so a revision left with none still takes u.
func (m *Model) resolveHistory(action string) (tea.Cmd, bool) {
	redo := action == keys.ActionRedo
	if (action != keys.ActionUndo && !redo) || m.selectionHistory(redo) > 0 {
		return nil, false
	}

	return m.rewind(redo), true
}

// resolveGeneral runs the actions that work with no conflicts loaded, false for any other.
func (m *Model) resolveGeneral(action string) (tea.Cmd, bool) {
	switch action {
	case keys.ActionQuit:
		return m.quit(), true
	case keys.ActionCancel:
		return m.cancel(), true
	case keys.ActionReload:
		return m.loadConflicts(), true
	case keys.ActionBack:
		// The conflict viewer is always launched into (--resolve), so its top level has nothing to
		// back out to but the shell.
		return m.quit(), true
	}

	return nil, false
}

// pickAction records the pick an action names, false when it names none.
func (m *Model) pickAction(action string, pressed []string) bool {
	switch action {
	case keys.ActionTakeSide:
		if len(pressed) > 0 {
			m.takePick(conflict.Pick{Kind: conflict.TakeSide, Side: int(pressed[len(pressed)-1][0] - '0')})
		}
	case keys.ActionTakeBase:
		m.takePick(conflict.Pick{Kind: conflict.TakeBase})
	case keys.ActionTakeAll:
		m.takePick(conflict.Pick{Kind: conflict.TakeAll})
	case keys.ActionResetRegion:
		m.takePick(conflict.Pick{})
	default:
		return false
	}

	return true
}

// resolveScroll moves the region view, or with gg and G the cursor to the first or last region.
func (m *Model) resolveScroll(action string) {
	r := m.res

	switch action {
	case keys.ActionLineDown, keys.ActionScrollDown:
		r.scroll++
	case keys.ActionLineUp, keys.ActionScrollUp:
		r.scroll = max(r.scroll-1, 0)
	case keys.ActionHalfPageDown, keys.ActionPageDown:
		r.scroll += helpPageStep
	case keys.ActionHalfPageUp, keys.ActionPageUp:
		r.scroll = max(r.scroll-helpPageStep, 0)
	case keys.ActionTop:
		r.file, r.region, r.scroll = 0, 0, 0
	case keys.ActionBottom:
		r.file = len(r.files) - 1
		r.region, r.scroll = max(len(r.regions())-1, 0), 0
	}
}

// resolveStep steps between regions, crossing into the next or previous file at either end, or
// between files.
func (m *Model) resolveStep(step keys.Step) {
	r := m.res
	delta := 1

	if step.Back {
		delta = -1
	}

	r.scroll = 0

	if step.Object == keys.ObjFile {
		r.file = min(max(r.file+delta, 0), len(r.files)-1)
		r.region = 0

		return
	}

	next := r.region + delta

	switch {
	case next >= 0 && next < len(r.regions()):
		r.region = next
	case delta > 0 && r.file+1 < len(r.files):
		r.file, r.region = r.file+1, 0
	case delta < 0 && r.file > 0:
		r.file--
		r.region = max(len(r.regions())-1, 0)
	default:
		m.say("no more conflicts that way", false)
	}
}

// takePick records p for the region at the cursor, refusing a side the region lacks.
func (m *Model) takePick(p conflict.Pick) {
	r := m.res

	region, ok := r.currentRegion()
	if !ok {
		m.say("this file has no region the viewer can read; M hands it to your merge tool", true)

		return
	}

	if _, ok := region.Text(p); !ok {
		m.say("this region has "+plural(len(region.Sides), "side"), true)

		return
	}

	r.undo = append(r.undo, r.picks.clone())
	r.redo, m.unwritten = nil, nil

	if p.Kind == conflict.Unresolved {
		delete(r.picks[r.file], r.region)
	} else {
		r.picks[r.file][r.region] = p
	}
}

func (m *Model) pickHistory(from, to *[]picks, word string) {
	r := m.res
	if len(*from) == 0 {
		m.say("nothing to "+word, false)

		return
	}

	*to = append(*to, r.picks.clone())
	r.picks = (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
}

// editRegion hands what the region now reads as, a side or the markers, to $EDITOR, and takes what
// comes back as the region's text.
func (m *Model) editRegion() tea.Cmd {
	r := m.res

	region, ok := r.currentRegion()
	if !ok {
		return nil
	}

	text, _ := region.Text(r.picks[r.file][r.region])
	file, idx := r.file, r.region

	return editInEditor(fmt.Sprintf("region-%d.txt", idx+1), text, func(m *Model, text string) {
		if m.res != r {
			return
		}

		r.file, r.region = file, idx
		m.takePick(conflict.Pick{Kind: conflict.Custom, Text: text})
	})
}

// mergeTool hands the file at the cursor to the merge tool the user configured, which writes on its
// own, and reads the conflicts again when it returns.
func (m *Model) mergeTool() tea.Cmd {
	path := m.res.files[m.res.file].path

	//nolint:gosec // G204: the literal jj binary, with the revision and the path it listed.
	cmd := exec.CommandContext(context.Background(), "jj", "resolve", "-r", m.revision, "--", jj.Fileset(path))
	cmd.Dir = m.client.Dir()
	run := m.client.Trail().Start(cmd.Args, "", true)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		call := run.Done(err, nil)
		if err != nil {
			return writtenMsg{err: fmt.Errorf("jj resolve %s: %w", path, err)}
		}

		return writtenMsg{summary: "resolved " + path + " in your merge tool", call: &call}
	})
}

// writeResolution writes every file with a pick as one jj resolve. It reads a snapshot taken now.
func (m *Model) writeResolution() tea.Cmd {
	r := m.res

	if m.client == nil || m.revision == "" || r == nil {
		m.say(errNoRevision.Error(), true)

		return nil
	}

	var (
		res     []jj.Resolution
		regions int
	)

	for i, f := range r.files {
		if len(r.picks[i]) == 0 {
			continue
		}

		regions += len(r.picks[i])
		res = append(res, jj.Resolution{Path: f.path, Conflicted: f.text, Resolved: f.parsed.Resolve(r.picks[i])})
	}

	if len(res) == 0 {
		m.say("nothing picked yet: 1–9 takes a side, 0 the base, a all of them", true)

		return nil
	}

	client, revision := m.client, m.revision

	return m.writing(func() tea.Msg {
		outcome, err := client.Resolve(revision, res)

		return writtenMsg{
			err: err, outcome: outcome, call: lastWrite(client),
			summary: fmt.Sprintf("resolved %s in %s", plural(regions, "region"), plural(len(res), "file")),
			verb:    "resolve",
		}
	})
}
