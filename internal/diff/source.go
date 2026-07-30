package diff

import (
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
	return s.Client.Diff(s.Revision)
}

// GetSourceLabel returns the revset the caller asked for, unresolved, which is what the header
// shows.
func (s *RevisionSource) GetSourceLabel() string {
	return s.Revision
}

// SupportsRevisions reports true, so interactive mode opens the revision picker.
func (s *RevisionSource) SupportsRevisions() bool {
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
func (s *DirectorySource) GetSourceLabel() string {
	return "diff-editor"
}

// SupportsRevisions reports false, because a diff editor sees two trees and has no revset to resolve
// against.
func (s *DirectorySource) SupportsRevisions() bool {
	return false
}
