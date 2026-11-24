package model

import (
	"strings"

	"github.com/kyleking/aragonite/tui/empty"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/theme"
)

// renderEmpty is the screen with no diff to show: a card naming the revision that is empty, why, and
// the keys out, over the footer. Before the first diff arrives it says the diff is on its way.
func (m *Model) renderEmpty() string {
	body := max(m.screenHeight()-1, 1)

	var lines []string

	if !m.loadedOnce {
		reading := empty.Empty{Title: "Reading the diff", Why: "jj diff is running."}
		lines = empty.Card(theme.Skin, reading, m.width, body)
	} else {
		lines = empty.Card(theme.Skin, m.noChanges(), m.width, body)
	}

	return strings.Join(append(lines, m.renderStatusBar()), "\n")
}

// noChanges is the card for a revision whose diff is empty.
func (m *Model) noChanges() empty.Empty {
	out := m.keysOut(keys.ActionReload, keys.ActionQuit)

	if m.mode == ModeDiffEditor {
		return empty.Empty{
			Title: "No changes between the two trees",
			Why:   "jj handed jj-diff two directories with the same content, so there is nothing to keep or drop.",
			Hints: out,
		}
	}

	why := "jj diff -r " + m.revision + " printed nothing: the revision changes no file."
	switch m.revision {
	case "@", "":
		why += " The working copy is often empty right after jj new; jj-diff -r @- reads its parent."
	case "root()":
		why += " The root commit is always empty; jj-diff -r @ reads the working copy."
	default:
		why += " jj-diff -r " + m.revision + "- reads its parent."
	}

	title := "No changes in "
	if m.startColumns {
		title = "Nothing to split in "
	}

	return empty.Empty{Title: title + orDefault(m.source, m.revision), Why: why, Hints: out}
}

// noConflicts is the conflict viewer's card for a revision with nothing left to resolve.
func (m *Model) noConflicts() empty.Empty {
	return empty.Empty{
		Title: "Nothing conflicted in " + orDefault(m.source, m.revision),
		Why: "jj resolve --list -r " + m.revision + " found no conflicted file, so there is nothing to pick. " +
			"jj log -r 'conflicts()' lists revisions that have some.",
		Hints: m.keysOut(keys.ActionReload, keys.ActionQuit),
	}
}

// emptyColumn is the split columns' card for a column holding no change.
func emptyColumn(col int) empty.Empty {
	if col == 0 {
		return empty.Empty{
			Title: "Column 1 is empty",
			Why:   "The source keeps no change; every hunk moves to a new commit.",
			Hints: []keyhint.Hint{{Key: "<", What: "move a hunk back"}},
		}
	}

	return empty.Empty{
		Title: "New commit",
		Why:   "Nothing here yet.",
		Hints: []keyhint.Hint{{Key: ">", What: "move right"}, {Key: "1–9", What: "send to a column"}},
	}
}

// keysOut is the hints for actions, as the keymap binds them.
func (m *Model) keysOut(actions ...string) []keyhint.Hint {
	return m.keymap.HintsAt(nil, actions...)
}
