package jj

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// patchFileMode is the permission a temporary patch file is written with. It holds the user's
// changes, so it stays readable only by them.
const patchFileMode = 0o600

// editorTool is the merge-tools name the write paths define on the command line for this program,
// distinct from any jj-diff entry a user configured, so their settings cannot change what runs.
const editorTool = "jj-diff-selection"

// The global flags a write passes jj to run this program as its editor.
const (
	flagConfig = "--config"
	flagTool   = "--tool"
)

// FlagApplyPatch and FlagUnapplyPatch are the flags this program takes when jj runs it as the diff
// editor for a write. The two trees follow them, and the program rewrites the right one from the
// patches without drawing anything.
const (
	FlagApplyPatch   = "apply-patch"
	FlagUnapplyPatch = "unapply-patch"
)

// Sentinel errors the write paths return on their own rather than wrapping one from jj.
var (
	errRevsetNoMatch = errors.New("revset matched no revision")
	errNoGroups      = errors.New("no groups to split out")
	errEmptyPatch    = errors.New("nothing is selected")
)

// Placement says where squash puts the selection. Into, the zero value, squashes into the
// destination; the others create a new commit placed relative to it, as jj squash -o, -A, and -B do.
type Placement string

// The placements jj squash offers.
const (
	Into         Placement = "--into"
	Onto         Placement = "--onto"
	InsertAfter  Placement = "--insert-after"
	InsertBefore Placement = "--insert-before"
)

// MoveOptions are the choices a squash picker offers beyond the destination.
type MoveOptions struct {
	// Placement is where the selection lands relative to the destination, Into when empty.
	Placement Placement
	// KeepEmptied keeps the source when the move empties it, as jj squash --keep-emptied does.
	KeepEmptied bool
	// UseDestinationMessage keeps the destination's description when the source is abandoned, so
	// jj never needs an editor to combine the two.
	UseDestinationMessage bool
}

// Outcome is what a write reports beyond succeeding: jj's own word that it left a conflict behind,
// which a move into a revision lacking the file does.
type Outcome struct {
	// Conflicts is jj's notice naming the revisions that now hold conflicts, empty when none do.
	Conflicts string
	// Undo is the command that reverses the write, empty when one jj undo does.
	Undo string
}

// MoveChanges moves the changes the patch selects out of source and into destination as one jj
// operation: jj squash --from source --into destination, with this program as the diff editor
// applying patch. The selection leaves source and lands in destination whatever their relationship,
// so a sibling or descendant destination is a true move rather than a copy, and a destination lacking
// a file the selection touches takes a conflict jj records rather than an error.
//
// The patch is a git diff of source against its parents, as GeneratePatch writes it. Nothing it
// does not select is touched; the working copy changes only when it is source or destination. A
// failure restores the operation recorded before the command, so the repository is as it was.
func (c *Client) MoveChanges(patch, source, destination string, opts MoveOptions) (Outcome, error) {
	placement := opts.Placement
	if placement == "" {
		placement = Into
	}

	args := []string{"squash", "--from", source, string(placement), destination}
	if opts.KeepEmptied {
		args = append(args, "--keep-emptied")
	}

	if opts.UseDestinationMessage {
		args = append(args, "--use-destination-message")
	}

	return c.write(args, selection{apply: patch})
}

// Restore discards the changes patch selects from revision, as jj restore --changes-in revision -i
// does, leaving every other change in it alone. It is one operation, restored on failure.
func (c *Client) Restore(patch, revision string) (Outcome, error) {
	// jj restore shows the revision on the left and its parents on the right, so the right side
	// that discards the selection is the revision with the selection taken back out.
	return c.write([]string{"restore", "--changes-in", revision}, selection{unapply: patch})
}

// SplitGroup is one commit a split carves out of the source. Through is a git diff of the source
// against its parents selecting this group and, for a chain, every group before it, so each patch
// applies to the original parent exactly however close the groups' hunks sit.
type SplitGroup struct {
	Through     string
	Description string
}

// Split carves groups out of source as new commits, the first group parent-most, and leaves what no
// group selected in source, which keeps its change ID and description. A chain stacks them between
// source's parents and source; parallel makes each a sibling of source on its parents. Each group is
// one jj split, and a failure part way restores the operation recorded before the first, so the
// whole split is undone as one.
//
// For a chain, group i's Through must include every group before it, because jj shows each split the
// commit the last one made as its left side.
func (c *Client) Split(source string, groups []SplitGroup, parallel bool) (Outcome, error) {
	if len(groups) == 0 {
		return Outcome{}, errNoGroups
	}

	sourceID, err := c.resolveChangeID(source)
	if err != nil {
		return Outcome{}, fmt.Errorf("failed to resolve %q: %w", source, err)
	}

	opID, err := c.getCurrentOperationID()
	if err != nil {
		return Outcome{}, fmt.Errorf("failed to get operation ID for rollback: %w", err)
	}

	placement := []string{"--insert-before", sourceID}
	if parallel {
		placement = []string{"--onto", sourceID + "-"}
	}

	var outcome Outcome

	for i, g := range groups {
		sel := selection{apply: g.Through}
		if i > 0 && !parallel {
			sel.unapply = groups[i-1].Through
		}

		args := append([]string{"split", "-r", sourceID, "-m", g.Description}, placement...)

		got, err := c.runSelection(args, sel)
		if err != nil {
			return Outcome{}, c.restoreOperationAfter(opID, fmt.Errorf("splitting out group %d: %w", i+1, err))
		}

		outcome.Conflicts = strings.TrimSpace(outcome.Conflicts + "\n" + got.Conflicts)
	}

	// Each group is its own operation, so one jj undo would take back only the last.
	if len(groups) > 1 {
		outcome.Undo = "jj op restore " + shortOp(opID)
	}

	return outcome, nil
}

// shortOp is an operation ID at the length jj prints it.
func shortOp(id string) string {
	const short = 12

	return id[:min(len(id), short)]
}

// selection is what the diff editor does to the right tree: take unapply back out of it, then put
// apply in, both starting from the left tree.
type selection struct {
	apply   string
	unapply string
}

// write runs one jj command with this program as its diff editor, restoring the operation recorded
// before it if it fails. A command jj refuses writes nothing, so the restore is for a failure after
// jj integrated its operation, and is skipped when the operation never moved.
func (c *Client) write(args []string, sel selection) (Outcome, error) {
	opID, err := c.getCurrentOperationID()
	if err != nil {
		return Outcome{}, fmt.Errorf("failed to get operation ID for rollback: %w", err)
	}

	outcome, err := c.runSelection(args, sel)
	if err != nil {
		return Outcome{}, c.restoreOperationAfter(opID, err)
	}

	return outcome, nil
}

// runSelection writes the patches to a private temporary directory, runs args with them as the diff
// editor's instructions, and removes the directory on every path.
func (c *Client) runSelection(args []string, sel selection) (_ Outcome, err error) {
	if strings.TrimSpace(sel.apply) == "" && strings.TrimSpace(sel.unapply) == "" {
		return Outcome{}, errEmptyPatch
	}

	tmpDir, err := os.MkdirTemp("", "jj-diff-selection-*")
	if err != nil {
		return Outcome{}, fmt.Errorf("failed to create temp dir: %w", err)
	}

	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove temp dir %s: %w", tmpDir, rmErr))
		}
	}()

	editArgs := []string{}

	for _, p := range []struct{ flag, body, name string }{
		{FlagUnapplyPatch, sel.unapply, "unapply.patch"},
		{FlagApplyPatch, sel.apply, "apply.patch"},
	} {
		if strings.TrimSpace(p.body) == "" {
			continue
		}

		path := filepath.Join(tmpDir, p.name)
		if err := os.WriteFile(path, []byte(p.body), patchFileMode); err != nil {
			return Outcome{}, fmt.Errorf("failed to write patch: %w", err)
		}

		editArgs = append(editArgs, "--"+p.flag, path)
	}

	tool, err := c.toolArgs(editArgs)
	if err != nil {
		return Outcome{}, err
	}

	stderr, err := c.executeJJStderr(append(args, tool...)...)
	if err != nil {
		return Outcome{}, err
	}

	return Outcome{Conflicts: conflictNotice(stderr)}, nil
}

// toolArgs points jj at this program as the diff editor for one command, defined on the command line
// so no user configuration is read or written. The instructions file is turned off, because the
// program would otherwise read it as a file the revision adds, and the editor is turned off so a
// description jj wants combined fails the command rather than waiting on a terminal it cannot have.
func (c *Client) toolArgs(editArgs []string) ([]string, error) {
	program, err := c.program()
	if err != nil {
		return nil, err
	}

	quoted := make([]string, 0, len(editArgs)+2)
	for _, a := range append(editArgs, "$left", "$right") {
		quoted = append(quoted, tomlString(a))
	}

	return []string{
		flagTool, editorTool,
		flagConfig, "merge-tools." + editorTool + ".program=" + tomlString(program),
		flagConfig, "merge-tools." + editorTool + ".edit-args=[" + strings.Join(quoted, ", ") + "]",
		flagConfig, "ui.diff-instructions=false",
		flagConfig, `ui.editor="false"`,
	}, nil
}

// program is the jj-diff binary jj runs back for a write: the one WithEditor named, else this one.
func (c *Client) program() (string, error) {
	if c.editor != "" {
		return c.editor, nil
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding this program to run as jj's editor: %w", err)
	}

	return self, nil
}

// tomlString quotes s as a TOML basic string, which is what jj reads a --config value as.
func tomlString(s string) string {
	var b strings.Builder

	b.WriteByte('"')

	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}

	b.WriteByte('"')

	return b.String()
}

// conflictNotice is jj's line naming new conflicts and the revisions under it, empty when the
// command left none.
func conflictNotice(stderr string) string {
	lines := strings.Split(stderr, "\n")

	for i, line := range lines {
		if !strings.HasPrefix(line, "New conflicts appeared") {
			continue
		}

		out := []string{line}
		for _, next := range lines[i+1:] {
			if !strings.HasPrefix(next, "  ") {
				break
			}

			out = append(out, strings.TrimSpace(next))
		}

		return strings.Join(out, " ")
	}

	return ""
}

// Echoed is argv as a status line names it: the command as a reader would type it, without the
// --tool and --config pairs that point jj at this program as its editor, which change how jj runs
// it and not what it does.
func Echoed(argv []string) []string {
	out := make([]string, 0, len(argv))

	for i := 0; i < len(argv); i++ {
		switch {
		case argv[i] == flagConfig && i+1 < len(argv):
			i++
		case argv[i] == flagTool && i+1 < len(argv) && argv[i+1] == editorTool:
			i++
		default:
			out = append(out, argv[i])
		}
	}

	return out
}
