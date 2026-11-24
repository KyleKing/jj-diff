package diff_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

var errBoom = errors.New("boom")

// countingSource answers its nth GetDiff with "diff n", so a test can tell a prefetched answer from
// a fresh one.
type countingSource struct {
	err   error
	calls atomic.Int32
}

func (s *countingSource) GetDiff() (string, error) {
	n := s.calls.Add(1)

	return "diff " + string('0'+n), s.err
}

func (*countingSource) GetSourceLabel() string  { return "label" }
func (*countingSource) SupportsRevisions() bool { return true }

func TestPrefetchAnswersTheFirstDiffAndReadsAfreshAfter(t *testing.T) {
	t.Parallel()

	inner := &countingSource{}
	src := diff.Prefetch(inner)

	if got, err := src.GetDiff(); err != nil || got != "diff 1" {
		t.Fatalf("first GetDiff = %q, %v, want the prefetched diff 1", got, err)
	}

	if got, err := src.GetDiff(); err != nil || got != "diff 2" {
		t.Errorf("second GetDiff = %q, %v, want a fresh diff 2", got, err)
	}

	if inner.calls.Load() != 2 {
		t.Errorf("the wrapped source was read %d times, want 2", inner.calls.Load())
	}

	if src.GetSourceLabel() != "label" || !src.SupportsRevisions() {
		t.Error("Prefetch hid the wrapped source's label or revision support")
	}
}

func TestPrefetchHandsBackTheFirstError(t *testing.T) {
	t.Parallel()

	if _, err := diff.Prefetch(&countingSource{err: errBoom}).GetDiff(); !errors.Is(err, errBoom) {
		t.Errorf("GetDiff error = %v, want the prefetch's own error", err)
	}
}

// The diff editor writes its selection back into the directories, so it must still find the
// DirectorySource behind a prefetch.
func TestDirectoryOfFindsTheDirectorySourceBehindAPrefetch(t *testing.T) {
	t.Parallel()

	dirs := diff.NewDirectorySource(t.TempDir(), t.TempDir())

	if got, ok := diff.DirectoryOf(diff.Prefetch(dirs)); !ok || got != dirs {
		t.Errorf("DirectoryOf answered %v, %v, want the DirectorySource", got, ok)
	}

	if got, ok := diff.DirectoryOf(dirs); !ok || got != dirs {
		t.Error("DirectoryOf an unwrapped source did not answer it")
	}

	if _, ok := diff.DirectoryOf(diff.Prefetch(&countingSource{})); ok {
		t.Error("DirectoryOf found a directory behind a source that has none")
	}
}
