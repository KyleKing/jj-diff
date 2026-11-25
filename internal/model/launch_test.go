//nolint:testpackage // white-box: these tests deliver the model's own load messages and read its held keys.
package model

import (
	"errors"
	"testing"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/jj"
)

// errJJExploded is a diff read that failed.
var errJJExploded = errors.New("jj exploded")

// launching is a model whose first diff read has not come back, as every session starts.
func launching(t *testing.T) *Model {
	t.Helper()

	m, err := NewModel(jj.NewClient(t.TempDir()), "@", "", ModeBrowse, config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	return &m
}

// TestLaunch_KeyBeforeTheDiffActsOnceItLands pins a key pressed before the first diff read is back:
// it waits for the diff rather than moving a cursor over no files, and keys keep their order. A
// read that fails releases them too, since the failure is what they may be asking about.
func TestLaunch_KeyBeforeTheDiffActsOnceItLands(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		landed any
		name   string
		keys   []string
		want   int
	}{
		{diffLoadedMsg{changes: TestChanges()}, "one j", []string{"j"}, 1},
		{diffLoadedMsg{changes: TestChanges()}, "in order", []string{"j", "j", "k"}, 1},
		{errMsg{errJJExploded}, "a failed read", []string{"j"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := launching(t)
			for _, key := range tc.keys {
				m = Update(t, m, Key(key))
			}

			if m.selectedFile != 0 || m.early.Ready() {
				t.Fatalf("a key acted before the diff: file %d, ready %t", m.selectedFile, m.early.Ready())
			}

			m = Update(t, m, tc.landed)

			if !m.early.Ready() {
				t.Error("keys are still held after the first load")
			}

			Assert(t, m).HasSelectedFile(tc.want)

			// A later load presses nothing again.
			m = Update(t, m, diffLoadedMsg{changes: TestChanges()})
			Assert(t, m).HasSelectedFile(tc.want)
		})
	}
}

// TestLaunch_CtrlCIsNeverHeld pins the way out of a jj that never answers: ctrl+c acts at once.
func TestLaunch_CtrlCIsNeverHeld(t *testing.T) {
	t.Parallel()

	m := launching(t)

	if _, cmd := m.Update(CtrlKey('c')); !quits(cmd) {
		t.Error("ctrl+c before the diff did not quit")
	}
}
