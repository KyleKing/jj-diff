package diff

import (
	"fmt"
	"sync"

	"github.com/kyleking/jj-diff/internal/jj"
)

// Source is where a diff comes from, either a jj revision or a pair of directories. GetDiff blocks
// on external work, so the UI calls it from a tea.Cmd rather than from Update.
type Source interface {
	GetDiff() (string, error)
	GetSourceLabel() string
	SupportsRevisions() bool
}

// RevisionSource generates diffs from jj revisions.
type RevisionSource struct {
	Client   *jj.Client
	Revision string
	label    string
}

// NewRevisionSource reads diffs from a jj revision through client. The revision is resolved on each
// GetDiff call, so a revset such as @ follows the working copy as it moves.
func NewRevisionSource(client *jj.Client, revision string) *RevisionSource {
	return &RevisionSource{
		Client:   client,
		Revision: revision,
	}
}

// GetDiff shells out to jj for the revision's diff, so it blocks and belongs in a tea.Cmd rather
// than in Update.
func (s *RevisionSource) GetDiff() (string, error) {
	text, err := s.Client.Diff(s.Revision)
	if err != nil {
		return "", fmt.Errorf("reading the diff for %s: %w", s.Revision, err)
	}

	return text, nil
}

// ResolveLabel shells out, so call it at startup rather than from the UI loop.
func (s *RevisionSource) ResolveLabel() error {
	label, err := s.Client.RevisionLabel(s.Revision)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", s.Revision, err)
	}

	s.label = label

	return nil
}

// GetSourceLabel returns the resolved label, or the revset when ResolveLabel was never called.
func (s *RevisionSource) GetSourceLabel() string {
	if s.label != "" {
		return s.label
	}

	return s.Revision
}

// SupportsRevisions reports true, so interactive mode opens the revision picker.
func (*RevisionSource) SupportsRevisions() bool {
	return true
}

// DirectorySource generates diffs by comparing two directories.
// Used for diff-editor mode where jj passes $left and $right directories.
type DirectorySource struct {
	LeftPath  string
	RightPath string
}

// NewDirectorySource compares two directory trees, which is how jj invokes a diff editor. Pass
// jj's $left and $right in that order, because the diff reads left as the old side.
func NewDirectorySource(leftPath, rightPath string) *DirectorySource {
	return &DirectorySource{
		LeftPath:  leftPath,
		RightPath: rightPath,
	}
}

// GetDiff walks both trees and diffs every file that differs, so it blocks for as long as the trees
// are large and belongs in a tea.Cmd rather than in Update.
func (s *DirectorySource) GetDiff() (string, error) {
	return CompareDirectories(s.LeftPath, s.RightPath)
}

// GetSourceLabel returns a fixed label, because the directories jj passes are temporary paths that
// mean nothing to the user.
func (*DirectorySource) GetSourceLabel() string {
	return "diff-editor"
}

// SupportsRevisions reports false, because a diff editor sees two trees and has no revset to resolve
// against.
func (*DirectorySource) SupportsRevisions() bool {
	return false
}

// Prefetch starts src's GetDiff in the background and returns a Source whose first GetDiff answers
// with it, so the external work behind the first frame (a jj call, a directory walk) overlaps the
// rest of startup rather than following it. Later GetDiff calls read afresh from src.
func Prefetch(src Source) *Prefetched {
	p := &Prefetched{Source: src, first: make(chan fetched, 1)}

	go func() {
		text, err := src.GetDiff()
		p.first <- fetched{text: text, err: err}
	}()

	return p
}

type fetched struct {
	err  error
	text string
}

// Prefetched is a Source whose first diff was read in the background, from Prefetch.
type Prefetched struct {
	Source

	first chan fetched
	mu    sync.Mutex
	taken bool
}

// GetDiff answers the prefetched diff the first time and asks the wrapped source after.
func (p *Prefetched) GetDiff() (string, error) {
	p.mu.Lock()
	first := !p.taken
	p.taken = true
	p.mu.Unlock()

	if !first {
		text, err := p.Source.GetDiff()
		if err != nil {
			return "", fmt.Errorf("reading the diff again: %w", err)
		}

		return text, nil
	}

	got := <-p.first

	return got.text, got.err
}

// DirectoryOf finds the DirectorySource behind src, through a Prefetch, for the diff editor, which
// writes its selection back into those directories.
func DirectoryOf(src Source) (*DirectorySource, bool) {
	if p, ok := src.(*Prefetched); ok {
		src = p.Source
	}

	dirs, ok := src.(*DirectorySource)

	return dirs, ok
}
