// Package reads keeps which hunks of a revision have been read, in the store jojo and second-look
// share. It opens that store, names jj-diff's hunks by the same content hash, answers per hunk, per
// file, and for the whole revision, and marks a hunk read or unread there (Toggle), so a mark made
// in any of the three tools shows in the other two.
package reads

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kyleking/aragonite/seen"

	"github.com/kyleking/jj-diff/internal/diff"
)

// App is the state directory the marks live under. It is jojo's, not jj-diff's, because every tool
// that marks must name one directory for a repository.
const App = seen.JJApp

// Open is the store jojo keeps for the jj repository dir is in, whichever workspace or
// subdirectory dir is: aragonite's seen.OpenJJ resolves every one of them to the repository's own
// store under jojo's state directory (seen.JJApp), which is where jojo and second-look file marks.
func Open(dir string) (*seen.Store, error) {
	store, err := seen.OpenJJ(dir)
	if err != nil {
		return nil, fmt.Errorf("finding the repository for read marks: %w", err)
	}

	return store, nil
}

// Hunks is one diff's hunks as the store names them, laid over jj-diff's own parse of it.
type Hunks struct {
	// ids is [file][hunk] in the order of the changes Map was given, empty for a file whose hunks
	// could not be paired, which then draws no marks rather than wrong ones.
	ids [][]seen.ID
	// live is every hunk of the diff as the store parsed it, which a save keeps marks for.
	live []seen.Ref
}

// Map names every hunk of changes, which jj-diff parsed from gitDiff, by the id jojo gives it.
// Hunks pair with a file by path and in order, and only where the counts agree: a file the two
// parsers read differently is left unnamed.
func Map(gitDiff string, changes []diff.FileChange) Hunks {
	byPath := map[string][]seen.ID{}
	live := seen.Parse([]byte(gitDiff))

	for _, r := range live {
		byPath[r.Path] = append(byPath[r.Path], r.ID)
	}

	out := Hunks{ids: make([][]seen.ID, len(changes)), live: live}

	for i := range changes {
		if ids := byPath[changes[i].Path]; len(ids) == len(changes[i].Hunks) {
			out.ids[i] = ids
		}
	}

	return out
}

// ID is the id of hunk h of file f, and false where it has none.
func (x Hunks) ID(f, h int) (seen.ID, bool) {
	if f < 0 || f >= len(x.ids) || h < 0 || h >= len(x.ids[f]) {
		return "", false
	}

	return x.ids[f][h], true
}

// Marks is one revision's read state: the store, the change id it is scoped by, and the diff's
// hunks. The zero value, and one missing any of the three, answers that nothing is read.
type Marks struct {
	store *seen.Store
	set   *seen.Set
	stamp stamp
	scope string
	path  string
	hunks Hunks
}

// stamp is what Refresh compares to notice another process saved: a save renames a new file into
// place, which changes its modification time or its size.
type stamp struct {
	mod  time.Time
	size int64
	ok   bool
}

// NewMarks reads marks from store, nil for none.
func NewMarks(store *seen.Store) *Marks { return &Marks{store: store} }

// Store is the store the marks are read from, nil for none.
func (k *Marks) Store() *seen.Store {
	if k == nil {
		return nil
	}

	return k.store
}

// On reports whether there is a store to read from, which is what binds the unread motions.
func (k *Marks) On() bool { return k != nil && k.store != nil }

// Ready reports whether marks can be answered: a store, a scope, and a diff to name.
func (k *Marks) Ready() bool { return k.On() && k.scope != "" && k.hunks.ids != nil }

// SetScope scopes the marks by a change id and reads them.
func (k *Marks) SetScope(scope string) {
	if !k.On() || scope == k.scope {
		return
	}

	k.scope, k.path, k.set, k.stamp = scope, "", nil, stamp{}

	if path, err := k.store.Path(scope); err == nil {
		k.path = path
	}

	k.store.Reload(scope)
	k.Refresh()
}

// SetHunks names the hunks of a freshly loaded diff.
func (k *Marks) SetHunks(h Hunks) {
	if k.On() {
		k.hunks = h
	}
}

// Refresh rereads the scope's file when it changed on disk since the last read, and reports
// whether it did. It costs one stat when nothing changed, so it can run on every message.
func (k *Marks) Refresh() bool {
	if !k.On() || k.path == "" {
		return false
	}

	now := statOf(k.path)
	if k.set != nil && now == k.stamp {
		return false
	}

	k.stamp = now
	k.store.Reload(k.scope)

	set, err := k.store.Set(k.scope)
	if err != nil {
		set = nil
	}

	k.set = set

	return true
}

func statOf(path string) stamp {
	info, err := os.Stat(path)
	if err != nil {
		return stamp{}
	}

	return stamp{mod: info.ModTime(), size: info.Size(), ok: true}
}

// Read reports whether hunk h of file f has been read.
func (k *Marks) Read(f, h int) bool {
	if !k.Ready() || k.set == nil {
		return false
	}

	id, ok := k.hunks.ID(f, h)

	return ok && k.set.Has(id)
}

// File is how many of file f's hunks have been read, of how many it has a name for.
func (k *Marks) File(f int) (int, int) {
	if !k.Ready() {
		return 0, 0
	}

	total := len(k.hunks.ids[f])
	read := 0

	for h := range total {
		if k.Read(f, h) {
			read++
		}
	}

	return read, total
}

// FileRead reports whether every hunk of file f has been read, false for a file with none.
func (k *Marks) FileRead(f int) bool {
	read, total := k.File(f)

	return total > 0 && read == total
}

// Unread is how many hunks of the whole diff have not been read.
func (k *Marks) Unread() int {
	if !k.Ready() {
		return 0
	}

	n := 0

	for f := range k.hunks.ids {
		read, total := k.File(f)
		n += total - read
	}

	return n
}

// ErrUnnamed refuses a mark on a hunk the store has no name for: one in a file the two parsers read
// differently, or a diff still loading.
var ErrUnnamed = errors.New("this hunk has no read-mark name")

// Toggle marks hunk h of file f read, or unread when it already is, and saves the scope at once so
// jojo and second-look see it before the next keystroke. It reports what the hunk is now. The save
// keeps marks only for hunks this revision's diff still carries, which is the whole diff Map named,
// never a filtered view of it. The caller's next Refresh picks up the file the save rewrote.
func (k *Marks) Toggle(f, h int) (bool, error) {
	id, ok := k.hunks.ID(f, h)
	if !k.Ready() || !ok {
		return false, ErrUnnamed
	}

	read := !k.Read(f, h)

	if err := k.store.Mark(k.scope, read, k.hunks.live, id); err != nil {
		return !read, fmt.Errorf("saving the read mark: %w", err)
	}

	return read, nil
}
