package main_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"github.com/kyleking/aragonite/hermetic"
)

// These drive the built binary on a pty with a terminal emulator reading what it writes, because a
// resize is a conversation with a terminal that a model test never has: the program and the
// terminal each hold a size, and a frame written for one can reach the other.

// size is a terminal's width and height in cells.
type size struct{ cols, rows int }

func (sz size) winsize() *pty.Winsize {
	//nolint:gosec // G115: a terminal is far smaller than a uint16.
	return &pty.Winsize{Cols: uint16(sz.cols), Rows: uint16(sz.rows)}
}

// emulate is a terminal of sz. It answers queries on a pipe nobody else reads, so the pipe is
// drained, and margins past the bottom row are clamped to it as xterm does, since the library takes
// them as given and panics on the next scroll.
func emulate(sz size) *vt.SafeEmulator {
	term := vt.NewSafeEmulator(sz.cols, sz.rows)

	term.RegisterCsiHandler('r', func(params xansi.Params) bool {
		if height := term.Emulator.Height(); len(params) > 1 && params[1].Param(0) > height {
			params[1] = xansi.Param(height)
		}

		return false
	})

	go io.Copy(io.Discard, term) //nolint:errcheck // it ends with the test binary

	return term
}

// answers is what a terminal replies to the capabilities Bubble Tea asks about. A bare pty answers
// none, so the program would wait out a timeout on each before drawing.
//
//nolint:gochecknoglobals // a table of constants, kept beside the loop that reads it
var answers = []struct{ query, reply string }{
	{"\x1b]11;?", "\x1b]11;rgb:2424/2727/3a3a\a"},
	{"\x1b[c", "\x1b[?62;22c"},
	{"\x1b[?2026$p", "\x1b[?2026;2$y"},
	{"\x1b[?2027$p", "\x1b[?2027;2$y"},
}

// screen is jj-diff running on a pty, with a terminal emulator fed everything it writes.
type screen struct {
	t       *testing.T
	pty     *os.File
	cmd     *exec.Cmd
	term    *vt.SafeEmulator
	drained chan struct{}
	mu      sync.Mutex
}

func (s *screen) drain() {
	defer close(s.drained)

	chunk := make([]byte, 4096)

	for {
		n, err := s.pty.Read(chunk)
		s.feed(chunk[:n])

		for _, a := range answers {
			if strings.Contains(string(chunk[:n]), a.query) {
				_, _ = s.pty.WriteString(a.reply) //nolint:errcheck // the process is gone once this fails
			}
		}

		if err != nil {
			return
		}
	}
}

func (s *screen) feed(chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	defer func() { _ = recover() }() //nolint:errcheck // a chunk the emulator cannot take is redrawn

	_, _ = s.term.Write(chunk) //nolint:errcheck // the emulator only fails once closed
}

// frame is what the terminal shows now, one string per row.
func (s *screen) frame() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return strings.Split(s.term.String(), "\n")
}

func (s *screen) awaitFrame(what string, want func([]string) bool) {
	s.t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if want(s.frame()) {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	s.t.Fatalf("waiting for %s; the terminal showed:\n%s", what, strings.Join(s.frame(), "\n"))
}

func (s *screen) press(keys string) {
	s.t.Helper()

	if _, err := s.pty.WriteString(keys); err != nil {
		s.t.Fatalf("pressing %q: %v", keys, err)
	}
}

// resize changes the terminal the way a window does. The emulator is swapped before the pty changes
// size, so a repaint can never land in the discarded one. The stale argument is what reaches the new
// terminal ahead of the repaint: the tail of a frame the program wrote for the old size.
func (s *screen) resize(sz size, stale string) {
	s.t.Helper()

	s.mu.Lock()
	s.term = emulate(sz)
	_, _ = s.term.WriteString(stale) //nolint:errcheck // the emulator only fails once closed
	s.mu.Unlock()

	if err := pty.Setsize(s.pty, sz.winsize()); err != nil {
		s.t.Fatalf("resizing to %dx%d: %v", sz.cols, sz.rows, err)
	}
}

func (s *screen) quit() {
	s.t.Helper()
	s.press("q")

	done := make(chan error, 1)

	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			s.t.Errorf("jj-diff exited with %v", err)
		}
	case <-time.After(10 * time.Second):
		s.t.Fatal("jj-diff did not quit on q")
	}

	<-s.drained
}

// binary is jj-diff built once for every test that drives it, by TestMain.
var binary string //nolint:gochecknoglobals // set once by TestMain, before any test runs

// TestMain builds the binary and runs the tests in a home of their own, failing the run if one
// reaches the real one.
func TestMain(m *testing.M) {
	hermetic.Main(runner(func() int { return buildAndRun(m) }), hermetic.App("jj-diff", "jojo"))
}

// runner is a function hermetic.Main can drive, so the build runs inside the isolated home too.
type runner func() int

func (r runner) Run() int { return r() }

func buildAndRun(m *testing.M) int {
	dir, err := os.MkdirTemp("", "jj-diff-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	defer os.RemoveAll(dir) //nolint:errcheck // a temp dir left behind is harmless

	binary = filepath.Join(dir, "jj-diff")

	//nolint:gosec // G204: builds this package with the go on PATH.
	build := exec.CommandContext(context.Background(), "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)

		return 1
	}

	return m.Run()
}

// isolated is an environment that reads and writes nothing of the runner's: its own home and
// config, and a jj that knows who is committing.
func isolated(t *testing.T) []string {
	t.Helper()

	home := t.TempDir()

	// The first-run card is recorded as seen, so the screen opens on the diff rather than behind it.
	state := filepath.Join(home, ".local", "state", "jj-diff")
	if err := os.MkdirAll(state, 0o750); err != nil {
		t.Fatal(err)
	}

	welcomed := `{"first":"2026-01-01T00:00:00Z","welcomed":"2026-01-01T00:00:00Z","tips":{},"ran":{}}` + "\n"
	if err := os.WriteFile(filepath.Join(state, "coach.json"), []byte(welcomed), 0o600); err != nil {
		t.Fatal(err)
	}

	return append(os.Environ(),
		"HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
		"JJ_CONFIG="+filepath.Join(home, "jj.toml"), "JJ_USER=Test User", "JJ_EMAIL=test@example.com",
		"TERM=xterm-256color", "COLORTERM=truecolor",
	)
}

// changedRepo is a jj repository whose working copy adds files enough to fill a short terminal, in
// both the file list and the diff under it.
func changedRepo(t *testing.T, env []string, files int) string {
	t.Helper()

	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj is not installed")
	}

	dir := t.TempDir()

	cmd := exec.CommandContext(t.Context(), "jj", "git", "init", ".")
	cmd.Dir, cmd.Env = dir, env

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jj git init: %v\n%s", err, out)
	}

	for i := range files {
		path := filepath.Join(dir, fmt.Sprintf("file%02d.go", i))
		body := fmt.Appendf(nil, "package main\n\nvar v%d = %d\n\nfunc f%d() int { return v%d }\n", i, i, i, i)

		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func open(t *testing.T, sz size, dir string, env []string, args ...string) *screen {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), binary, args...) // #nosec G204 -- the binary TestMain built
	cmd.Dir, cmd.Env = dir, env

	f, err := pty.StartWithSize(cmd, sz.winsize())
	if err != nil {
		t.Fatalf("starting jj-diff on a pty: %v", err)
	}

	s := &screen{t: t, pty: f, cmd: cmd, term: emulate(sz), drained: make(chan struct{})}

	go s.drain()

	t.Cleanup(func() { _ = f.Close() }) //nolint:errcheck // the process is gone by then

	return s
}

// TestResize_GrowAfterAStaleFrameKeepsThePageInPlace pins a grown terminal that received the tail
// of a frame drawn for its old height. The renderer confines a scroll with DECSTBM and restores the
// margins by number, so that tail sets the bottom margin to the old height. Every newline the repaint
// then writes at that row scrolls the page up instead of moving down, and the file list's first rows
// scroll off the top, until something redraws it. Since tmux resets the margins on a resize itself,
// only a terminal that keeps them, as this emulator does, shows the bug.
func TestResize_GrowAfterAStaleFrameKeepsThePageInPlace(t *testing.T) {
	t.Parallel()

	env := isolated(t)
	dir := changedRepo(t, env, 30)

	for _, tc := range []struct {
		name string
		args []string
		from size
		to   size
	}{
		{"browse", nil, size{100, 20}, size{100, 40}},
		{"browse, wider and taller", nil, size{80, 16}, size{140, 44}},
		{"interactive, side by side", []string{"-i", "-s"}, size{120, 20}, size{140, 40}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sc := open(t, tc.from, dir, env, tc.args...)
			sc.awaitFrame("the file list", func(f []string) bool {
				return strings.Contains(strings.Join(f, "\n"), "file01.go")
			})

			// What a scroll inside the old frame ends with: the margins restored as 1 to its height.
			sc.resize(tc.to, xansi.SetTopBottomMargins(1, tc.from.rows))

			sc.awaitFrame("a frame for the new size", func(f []string) bool {
				return len(f) == tc.to.rows && strings.Contains(f[len(f)-1], "help")
			})

			// Give a repaint drawn inside the stale margins time to scroll the page, then read again.
			time.Sleep(300 * time.Millisecond)

			frame := sc.frame()

			if !strings.Contains(frame[0], "Files") {
				t.Errorf("the file list's heading is not the first row after growing to %dx%d:\n%s",
					tc.to.cols, tc.to.rows, strings.Join(frame, "\n"))
			}

			if !strings.Contains(frame[1], "file00.go") {
				t.Errorf("the first file is not right under the heading:\n%s", strings.Join(frame, "\n"))
			}

			sc.quit()
		})
	}
}
