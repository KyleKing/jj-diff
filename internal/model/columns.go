package model

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
)

// maxColumns is the most columns a split takes, one per digit key.
const maxColumns = 9

// columnPage is how many hunks ctrl+d and ctrl+u move the cursor in a column.
const columnPage = 5

// changeKey names one changed line of the diff.
type changeKey struct {
	path string
	hunk int
	line int
}

// item is one hunk as a column shows it: the hunk, with only the changed lines that column owns.
type item struct {
	file int
	hunk int
}

// columnsSnapshot is what undo puts back in the columns.
type columnsSnapshot struct {
	owner map[changeKey]int
	descs []string
}

// splitColumns is the split columns mode: every changed line belongs to one column, column 0 being
// what stays in the source and each other column a new commit, parent-most first. A line no map
// entry names is in column 0.
type splitColumns struct {
	owner map[changeKey]int
	sel   *SelectionState
	draft string
	// descs is each column's description; the source's is never written.
	descs []string
	// cursor is each column's hunk cursor, an index into its items.
	cursor []int
	undo   []columnsSnapshot
	redo   []columnsSnapshot
	focus  int
	// line is the visual cursor, a line index in the hunk at the cursor.
	line       int
	anchor     int
	visual     bool
	describing bool
	parallel   bool
}

func newColumns() *splitColumns {
	return &splitColumns{
		owner:  map[changeKey]int{},
		descs:  []string{"", ""},
		cursor: []int{0, 0},
		sel:    NewSelectionState(),
	}
}

func (c *splitColumns) count() int { return len(c.descs) }

func (c *splitColumns) ownerOf(k changeKey) int { return c.owner[k] }

// addColumn appends an empty new-commit column.
func (c *splitColumns) addColumn() {
	c.descs = append(c.descs, "")
	c.cursor = append(c.cursor, 0)
}

func (c *splitColumns) snapshot() columnsSnapshot {
	return columnsSnapshot{owner: maps.Clone(c.owner), descs: slices.Clone(c.descs)}
}

// edit records the columns before change, so undo can return to them.
func (c *splitColumns) edit(change func()) {
	c.undo = append(c.undo, c.snapshot())
	c.redo = nil

	change()
}

func (c *splitColumns) restore(s columnsSnapshot) {
	c.owner, c.descs = s.owner, s.descs

	for len(c.cursor) < len(c.descs) {
		c.cursor = append(c.cursor, 0)
	}

	c.cursor = c.cursor[:len(c.descs)]
	c.focus = min(c.focus, len(c.descs)-1)
	c.sel = NewSelectionState()
	c.visual = false
}

// changed is every changed line of the diff in order.
func changed(changes []diff.FileChange, each func(k changeKey, l diff.Line)) {
	for _, f := range changes {
		for h, hunk := range f.Hunks {
			for i, l := range hunk.Lines {
				if l.Type != diff.LineContext {
					each(changeKey{path: f.Path, hunk: h, line: i}, l)
				}
			}
		}
	}
}

// items is what column col shows: every hunk with a changed line it owns, in diff order.
func (m *Model) items(col int) []item {
	var out []item

	for fi, f := range m.changes {
		for h, hunk := range f.Hunks {
			for i, l := range hunk.Lines {
				if l.Type != diff.LineContext && m.cols.ownerOf(changeKey{f.Path, h, i}) == col {
					out = append(out, item{file: fi, hunk: h})

					break
				}
			}
		}
	}

	return out
}

// columnCounts is how many hunks, added lines, and removed lines column col holds.
func (m *Model) columnCounts(col int) (int, int, int) {
	hunks, added, removed := len(m.items(col)), 0, 0

	changed(m.changes, func(k changeKey, l diff.Line) {
		if m.cols.ownerOf(k) != col {
			return
		}

		if l.Type == diff.LineAddition {
			added++
		} else {
			removed++
		}
	})

	return hunks, added, removed
}

// current is the hunk at the focused column's cursor, false when the column is empty.
func (m *Model) current() (item, bool) {
	items := m.items(m.cols.focus)
	if len(items) == 0 {
		return item{}, false
	}

	i := min(max(m.cols.cursor[m.cols.focus], 0), len(items)-1)
	m.cols.cursor[m.cols.focus] = i

	return items[i], true
}

// openColumns starts the split columns with sel, when there is one, in a new commit's column.
func (m *Model) openColumns(sel *SelectionState) {
	m.cols = newColumns()
	m.isVisualMode = false
	m.matcher.Reset()

	if sel == nil || sel.IsEmpty() {
		return
	}

	for _, k := range m.linesOf(sel, 0) {
		m.cols.owner[k] = 1
	}

	m.cols.focus = 1
}

// linesOf is the changed lines sel picks that column col owns: every one in a hunk picked whole, and
// the picked ones in a hunk picked by line.
func (m *Model) linesOf(sel *SelectionState, col int) []changeKey {
	var out []changeKey

	changed(m.changes, func(k changeKey, _ diff.Line) {
		if m.cols.ownerOf(k) == col && sel.IsLineSelected(k.path, k.hunk, k.line) {
			out = append(out, k)
		}
	})

	return out
}

// columnTarget is what a move acts on: the selection in the focused column, else the hunk at its
// cursor.
func (m *Model) columnTarget() []changeKey {
	if !m.cols.sel.IsEmpty() {
		return m.linesOf(m.cols.sel, m.cols.focus)
	}

	it, ok := m.current()
	if !ok {
		return nil
	}

	return m.linesOf(Only(m.changes[it.file].Path, it.hunk), m.cols.focus)
}

// moveTo moves the target to column to, appending a column when to is one past the last.
func (m *Model) moveTo(to int) {
	c := m.cols

	switch {
	case to < 0 || to == c.focus:
		return
	case to > c.count() || to >= maxColumns:
		m.say(fmt.Sprintf("there is no column %d: %s", to+1, plural(c.count(), "column")), true)

		return
	}

	lines := m.columnTarget()
	if len(lines) == 0 {
		m.say(errNothingSelected.Error(), true)

		return
	}

	c.edit(func() {
		if to == c.count() {
			c.addColumn()
		}

		for _, k := range lines {
			if to == 0 {
				delete(c.owner, k)
			} else {
				c.owner[k] = to
			}
		}
	})

	c.sel = NewSelectionState()
	c.visual = false
	m.say(fmt.Sprintf("moved %s to column %d", plural(len(lines), "line"), to+1), false)
}

// columnsAction runs one action in the split columns, false when it is not theirs.
func (m *Model) columnsAction(action string, pressed []string) (tea.Cmd, bool) {
	c := m.cols

	switch action {
	case keys.ActionColumnLeft:
		m.focusColumn(c.focus - 1)
	case keys.ActionColumnRight:
		m.focusColumn(c.focus + 1)
	case keys.ActionMoveLeft:
		m.moveTo(c.focus - 1)
	case keys.ActionMoveRight:
		m.moveTo(c.focus + 1)
	case keys.ActionSendTo:
		if len(pressed) > 0 {
			m.moveTo(int(pressed[len(pressed)-1][0] - '1'))
		}
	case keys.ActionDescribe:
		m.describeColumn()
	case keys.ActionDescribeEditor:
		return m.describeColumnInEditor(), true
	case keys.FlagAction(keys.VerbSplit, "p"):
		c.parallel = !c.parallel
	default:
		return m.columnsEdit(action)
	}

	return nil, true
}

// columnsEdit runs the selection, history, and leaving actions in the split columns.
func (m *Model) columnsEdit(action string) (tea.Cmd, bool) {
	c := m.cols

	switch action {
	case actionSelect:
		m.selectInColumn()
	case keys.ActionVisual:
		m.visualInColumn()
	case actionInvert:
		m.invertInColumn()
	case actionConfirm, actionApply:
		return m.applyColumns(), true
	case keys.ActionUndo:
		m.columnsHistory(&c.undo, &c.redo, "undo")
	case keys.ActionRedo:
		m.columnsHistory(&c.redo, &c.undo, "redo")
	case keys.ActionBack:
		m.columnsBack()
	case keys.ActionQuit:
		m.leaveColumns()
	default:
		return nil, m.columnsMotion(action)
	}

	return nil, true
}

// columnsMotion moves within the focused column: j and k step hunks (lines in visual), ]h and [h
// too, ]f and [f files, gg and G the ends.
func (m *Model) columnsMotion(action string) bool {
	c := m.cols

	if step, ok := m.motion.Resolve(action); ok {
		delta := 1
		if step.Back {
			delta = -1
		}

		if step.Object == keys.ObjFile {
			m.columnFile(delta)
		} else {
			m.columnStep(delta)
		}

		return true
	}

	switch action {
	case keys.ActionLineDown:
		m.columnStep(1)
	case keys.ActionLineUp:
		m.columnStep(-1)
	case keys.ActionHalfPageDown, keys.ActionPageDown:
		m.columnStep(columnPage)
	case keys.ActionHalfPageUp, keys.ActionPageUp:
		m.columnStep(-columnPage)
	case keys.ActionTop:
		c.cursor[c.focus] = 0
		c.visual = false
	case keys.ActionBottom:
		c.cursor[c.focus] = len(m.items(c.focus)) - 1
		c.visual = false
	default:
		return false
	}

	return true
}

func (m *Model) focusColumn(col int) {
	if col < 0 || col >= m.cols.count() {
		return
	}

	m.cols.focus = col
	m.cols.sel = NewSelectionState()
	m.cols.visual = false
}

// columnStep moves the visual line cursor, or else the hunk cursor, by delta, stopping at the ends.
func (m *Model) columnStep(delta int) {
	c := m.cols

	if c.visual {
		m.visualStep(delta)

		return
	}

	n := len(m.items(c.focus))
	c.cursor[c.focus] = min(max(c.cursor[c.focus]+delta, 0), max(n-1, 0))
}

// columnFile moves the hunk cursor to the first hunk of the next or previous file in the column.
func (m *Model) columnFile(delta int) {
	c := m.cols
	items := m.items(c.focus)

	it, ok := m.current()
	if !ok {
		return
	}

	i := c.cursor[c.focus]
	for i += delta; i >= 0 && i < len(items); i += delta {
		if items[i].file != it.file && (delta > 0 || i == 0 || items[i-1].file != items[i].file) {
			c.cursor[c.focus] = i
			c.visual = false

			return
		}
	}
}

// shown reports whether a hunk line is drawn in column col: context, or a changed line col owns.
func (m *Model) shown(col int, it item, line int) bool {
	l := m.changes[it.file].Hunks[it.hunk].Lines[line]

	return l.Type == diff.LineContext || m.cols.ownerOf(changeKey{m.changes[it.file].Path, it.hunk, line}) == col
}

func (m *Model) visualStep(delta int) {
	c := m.cols

	it, ok := m.current()
	if !ok {
		return
	}

	lines := m.changes[it.file].Hunks[it.hunk].Lines
	for i := c.line + delta; i >= 0 && i < len(lines); i += delta {
		if m.shown(c.focus, it, i) {
			c.line = i

			return
		}
	}
}

func (m *Model) visualInColumn() {
	c := m.cols

	it, ok := m.current()
	if !ok {
		return
	}

	for i, l := range m.changes[it.file].Hunks[it.hunk].Lines {
		if l.Type != diff.LineContext && m.shown(c.focus, it, i) {
			c.visual, c.line, c.anchor = true, i, i

			return
		}
	}
}

// selectInColumn toggles the hunk at the cursor, or in visual takes the line range.
func (m *Model) selectInColumn() {
	c := m.cols

	it, ok := m.current()
	if !ok {
		return
	}

	path := m.changes[it.file].Path

	if c.visual {
		c.sel.SelectLineRange(path, it.hunk, c.anchor, c.line)
		c.visual = false

		return
	}

	c.sel.ToggleHunk(path, it.hunk)
}

// invertInColumn flips the selection over every hunk in the column.
func (m *Model) invertInColumn() {
	c := m.cols

	for _, it := range m.items(c.focus) {
		c.sel.ToggleHunk(m.changes[it.file].Path, it.hunk)
	}
}

func (m *Model) columnsHistory(from, to *[]columnsSnapshot, word string) {
	c := m.cols
	if len(*from) == 0 {
		m.say("nothing to "+word, false)

		return
	}

	*to = append(*to, c.snapshot())
	c.restore((*from)[len(*from)-1])
	*from = (*from)[:len(*from)-1]
}

// columnsBack backs out one level: visual, then the selection, then the columns themselves.
func (m *Model) columnsBack() {
	c := m.cols

	switch {
	case c.visual:
		c.visual = false
	case !c.sel.IsEmpty():
		c.sel = NewSelectionState()
	default:
		m.leaveColumns()
	}
}

func (m *Model) leaveColumns() {
	if m.cols != nil && len(m.cols.undo) > 0 {
		m.say("left the split; nothing was written", false)
	}

	m.cols = nil
	m.matcher.Reset()
}

// describeColumn opens the focused column's header for typing its description.
func (m *Model) describeColumn() {
	c := m.cols
	if c.focus == 0 {
		m.say("column 1 is what stays in the source, which keeps its description", false)

		return
	}

	c.describing, c.draft = true, c.descs[c.focus]
}

// handleDescribeKeyPress types into the focused column's description until Enter keeps it or Esc
// drops it.
func (m *Model) handleDescribeKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	c := m.cols

	switch msg.String() {
	case keyEnter:
		c.describing = false
		draft, col := c.draft, c.focus
		c.edit(func() { c.descs[col] = draft })
	case keyEsc:
		c.describing = false
	case keyCtrlC:
		return m.cancel()
	case keyBackspace:
		if r := []rune(c.draft); len(r) > 0 {
			c.draft = string(r[:len(r)-1])
		}
	default:
		c.draft += msg.Key().Text
	}

	return nil
}

// splitGroups turns the columns into Split's groups: every new-commit column that holds anything, in
// order, each patch taking in the ones before it for a chain.
func (m *Model) splitGroups() []jj.SplitGroup {
	c := m.cols
	through := NewSelectionState()

	var groups []jj.SplitGroup

	for col := 1; col < c.count(); col++ {
		sel := NewSelectionState()
		if !c.parallel {
			sel = through
		}

		n := 0

		changed(m.changes, func(k changeKey, _ diff.Line) {
			if c.ownerOf(k) == col {
				sel.ToggleLine(k.path, k.hunk, k.line)
				n++
			}
		})

		if n == 0 {
			continue
		}

		groups = append(groups, jj.SplitGroup{
			Through: diff.GeneratePatch(m.changes, sel), Description: c.descs[col],
		})

		if !c.parallel {
			through = sel.Clone()
		}
	}

	return groups
}

// applyColumns splits the revision into the columns' commits through Client.Split, dropping empty
// columns. It reads a snapshot taken now, as every write does.
func (m *Model) applyColumns() tea.Cmd {
	if m.client == nil || m.revision == "" {
		m.say(errNoRevision.Error(), true)

		return nil
	}

	groups := m.splitGroups()
	if len(groups) == 0 {
		m.say("nothing to split: every change is still in column 1", true)

		return nil
	}

	client, revision, parallel := m.client, m.revision, m.cols.parallel
	m.cols = nil

	return func() tea.Msg {
		outcome, err := client.Split(revision, groups, parallel)

		return writtenMsg{
			err: err, outcome: outcome, call: lastWrite(client),
			summary: fmt.Sprintf("split %s into %s", revision, plural(len(groups)+1, "commit")),
		}
	}
}

// splitCommand is the jj command line the columns will run, and the order the commits land in.
func (m *Model) splitCommand() string {
	c := m.cols
	args := []string{"jj split -r", m.revision}

	if c.parallel {
		args = append(args, "--parallel")
	}

	order := make([]string, 0, c.count())
	for col := 1; col < c.count(); col++ {
		order = append(order, strconv.Itoa(col+1))
	}

	order = append(order, "1")

	if c.parallel {
		return strings.Join(args, " ") + " · siblings " + strings.Join(order, " · ")
	}

	return strings.Join(args, " ") + " · chain " + strings.Join(order, " → ")
}

// describeColumnInEditor writes the focused column's description in $EDITOR.
func (m *Model) describeColumnInEditor() tea.Cmd {
	c := m.cols
	if c.focus == 0 {
		m.describeColumn()

		return nil
	}

	col := c.focus

	return editInEditor("DESCRIPTION", c.descs[col], func(m *Model, text string) {
		if m.cols == nil || col >= m.cols.count() {
			return
		}

		cols := m.cols
		cols.edit(func() { cols.descs[col] = strings.TrimSpace(text) })
	})
}
