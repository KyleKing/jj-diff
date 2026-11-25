package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shows reports whether text is anywhere on the terminal.
func (s *screen) shows(text string) bool {
	return strings.Contains(strings.Join(s.frame(), "\n"), text)
}

// firstLaunch is env with the coach's ledger removed, so jj-diff opens on its welcome card.
func firstLaunch(t *testing.T, env []string) []string {
	t.Helper()

	// The last of a repeated variable is the one the child sees, and isolated's comes last.
	state := ""

	for _, kv := range env {
		if dir, ok := strings.CutPrefix(kv, "XDG_STATE_HOME="); ok {
			state = dir
		}
	}

	if err := os.Remove(filepath.Join(state, "jj-diff", "coach.json")); err != nil {
		t.Fatal(err)
	}

	return env
}

// TestLaunch_FirstKeyActs pins the first key of a session. A key typed as the command is entered
// reaches jj-diff before the diff has loaded, and before that, while the terminal is being asked for
// its background; neither may drop it. It runs against a terminal that answers capability queries
// and one that answers none, since a silent terminal holds the background question open for its
// whole wait. On a first launch the welcome card is up and the first key's act is to close it,
// which is the card's design, so there it is the second key that moves the cursor.
func TestLaunch_FirstKeyActs(t *testing.T) {
	t.Parallel()

	dir := changedRepo(t, isolated(t), 4)

	for _, sz := range []size{{80, 24}, {120, 40}} {
		for _, silent := range []bool{false, true} {
			for _, early := range []bool{true, false} {
				for _, welcomed := range []bool{true, false} {
					name := fmt.Sprintf("%dx%d/silent=%t/typed-ahead=%t/welcomed=%t",
						sz.cols, sz.rows, silent, early, welcomed)

					t.Run(name, func(t *testing.T) {
						t.Parallel()
						firstKeyActs(t, sz, dir, silent, early, welcomed)
					})
				}
			}
		}
	}
}

func firstKeyActs(t *testing.T, sz size, dir string, silent, early, welcomed bool) {
	t.Helper()

	// A home per run: dismissing the welcome card writes the coach's ledger on the way out.
	env := isolated(t)
	if !welcomed {
		env = firstLaunch(t, env)
	}

	typed := ""
	if early {
		typed = "j"
	}

	sc := start(t, sz, dir, env, silent, typed)

	// The diff pane shows the file under the cursor, which opens on the first.
	onFirst := func([]string) bool { return sc.shows("v0 = 0") && !sc.shows("v1 = 1") }

	if !early {
		if welcomed {
			sc.awaitFrame("the first file's diff", onFirst)
		} else {
			sc.awaitFrame("the welcome card", func([]string) bool { return sc.shows("Welcome to jj-diff") })
		}

		sc.press("j")
	}

	if !welcomed {
		sc.awaitFrame("the first key to close the welcome card", func(f []string) bool {
			return !sc.shows("Welcome to jj-diff") && onFirst(f)
		})
		sc.press("j")
	}

	sc.awaitFrame("j to move the cursor onto the second file", func([]string) bool {
		return sc.shows("v1 = 1") && !sc.shows("v0 = 0")
	})

	sc.quit()
}
