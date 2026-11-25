package main_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestSplitLaunch_ExitsZeroWithTheOutcomeOnStderr is jojo's S: jj-diff --split -r @ on a pty, one
// hunk moved to a new column, enter. The program gives the terminal back by itself, exits 0, and
// leaves the outcome as its last stderr line, which is the contract jojo's launcher reads.
func TestSplitLaunch_ExitsZeroWithTheOutcomeOnStderr(t *testing.T) {
	t.Parallel()

	for _, sz := range []size{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", sz.cols, sz.rows), func(t *testing.T) {
			t.Parallel()

			env := isolated(t)
			dir := changedRepo(t, env, 2)

			var stderr bytes.Buffer

			// #nosec G204 -- the binary TestMain built
			cmd := exec.CommandContext(t.Context(), binary, "--split", "-r", "@")
			cmd.Dir, cmd.Env, cmd.Stderr = dir, env, &stderr

			f, err := pty.StartWithSize(cmd, sz.winsize())
			if err != nil {
				t.Fatalf("starting jj-diff on a pty: %v", err)
			}

			s := &screen{t: t, pty: f, cmd: cmd, term: emulate(sz), drained: make(chan struct{})}

			go s.drain()

			t.Cleanup(func() { _ = f.Close() }) //nolint:errcheck // the process is gone by then

			s.awaitFrame("the split columns", func(rows []string) bool {
				return strings.Contains(strings.Join(rows, "\n"), "chain 2 → 1")
			})
			s.press(">")
			s.awaitFrame("a hunk in column 2", func(rows []string) bool {
				return strings.Contains(strings.Join(rows, "\n"), "1 hunk")
			})
			s.press("\r")

			done := make(chan error, 1)

			go func() { done <- cmd.Wait() }()

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("jj-diff --split exited with %v; stderr:\n%s", err, stderr.String())
				}
			case <-time.After(15 * time.Second):
				t.Fatalf("jj-diff --split stayed open after the split applied; the terminal showed:\n%s",
					strings.Join(s.frame(), "\n"))
			}

			<-s.drained

			lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			if last := lines[len(lines)-1]; !strings.HasPrefix(last, "jj-diff: split ") {
				t.Errorf("the last stderr line is %q, want the outcome starting \"jj-diff: split \"", last)
			}

			log := exec.CommandContext(t.Context(), "jj", "log", "--no-graph", "-r", "::@ ~ root()", "-T", `"x\n"`)
			log.Dir, log.Env = dir, env

			out, err := log.CombinedOutput()
			if err != nil {
				t.Fatalf("jj log: %v\n%s", err, out)
			}

			if n := strings.Count(string(out), "x"); n != 2 {
				t.Errorf("@ has %d commits above the root after the split, want 2:\n%s", n, out)
			}
		})
	}
}
