package diff

import (
	"github.com/kyleking/jj-diff/internal/jj"
)

// DiffSource abstracts the source of diff data.
// This enables the same UI to work with both jj revisions and directory comparisons.
type DiffSource interface {
	GetDiff() (string, error)
	GetSourceLabel() string
	SupportsRevisions() bool
}

// RevisionSource generates diffs from jj revisions.
type RevisionSource struct {
	Client   *jj.Client
	Revision string
}

func NewRevisionSource(client *jj.Client, revision string) *RevisionSource {
	return &RevisionSource{
		Client:   client,
		Revision: revision,
	}
}

func (s *RevisionSource) GetDiff() (string, error) {
	return s.Client.Diff(s.Revision)
}

func (s *RevisionSource) GetSourceLabel() string {
	return s.Revision
}

func (s *RevisionSource) SupportsRevisions() bool {
	return true
}

// DirectorySource generates diffs by comparing two directories.
// Used for diff-editor mode where jj passes $left and $right directories.
type DirectorySource struct {
	LeftPath  string
	RightPath string
}

func NewDirectorySource(leftPath, rightPath string) *DirectorySource {
	return &DirectorySource{
		LeftPath:  leftPath,
		RightPath: rightPath,
	}
}

func (s *DirectorySource) GetDiff() (string, error) {
	return CompareDirectories(s.LeftPath, s.RightPath)
}

func (s *DirectorySource) GetSourceLabel() string {
	return "diff-editor"
}

func (s *DirectorySource) SupportsRevisions() bool {
	return false
}
