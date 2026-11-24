package main_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseTarget is one GOOS/GOARCH pair goreleaser builds.
type releaseTarget struct{ goos, goarch string }

// releaseMatrix is the build section of .goreleaser.yml as lists: the goos
// and goarch values and the pairs ignore drops.
type releaseMatrix struct {
	ignored      map[releaseTarget]bool
	list         string // the list the current line belongs to
	pending      string // an ignore entry's goos, waiting for its goarch
	goos, goarch []string
}

// read takes one trimmed line of .goreleaser.yml. It is a line reader for the
// template's own shape, not YAML: a key ending in a colon opens a list, and
// any other line closes it.
func (m *releaseMatrix) read(line string) {
	value := func(prefix string) string {
		return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"'`)
	}

	switch {
	case line == "goos:" || line == "goarch:" || line == "ignore:":
		m.list = strings.TrimSuffix(line, ":")
	case m.list == "ignore" && strings.HasPrefix(line, "- goos:"):
		m.pending = value("- goos:")
	case m.list == "ignore" && strings.HasPrefix(line, "goarch:"):
		m.ignored[releaseTarget{m.pending, value("goarch:")}] = true
	case m.list == "goos" && strings.HasPrefix(line, "- "):
		m.goos = append(m.goos, value("- "))
	case m.list == "goarch" && strings.HasPrefix(line, "- "):
		m.goarch = append(m.goarch, value("- "))
	default:
		m.list = ""
	}
}

// releaseTargets reads the build matrix out of .goreleaser.yml: every goos
// crossed with every goarch, less the ignore pairs. It reads the file rather
// than repeating the list, so a target the template adds is covered without
// an edit here.
func releaseTargets(t *testing.T) []releaseTarget {
	t.Helper()

	file, err := os.Open(filepath.Join("..", "..", ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("opening .goreleaser.yml: %v", err)
	}
	defer file.Close() //nolint:errcheck // a read-only file

	matrix := releaseMatrix{ignored: map[releaseTarget]bool{}}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		matrix.read(strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("reading .goreleaser.yml: %v", err)
	}

	var targets []releaseTarget

	for _, goos := range matrix.goos {
		for _, goarch := range matrix.goarch {
			if target := (releaseTarget{goos, goarch}); !matrix.ignored[target] {
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// TestEveryReleaseTargetBuilds compiles the binary for each pair goreleaser
// ships. CI runs on Linux only, so a Unix-only call (an ioctl, a syscall
// constant) would pass every check here and fail the release workflow, as it
// once did in second-look.
func TestEveryReleaseTargetBuilds(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("cross-compiles the binary for every release target")
	}

	targets := releaseTargets(t)

	seen := map[string]bool{}
	for _, target := range targets {
		seen[target.goos] = true
	}

	// A reader that silently found nothing would pass, so insist on the
	// operating systems a Unix-only call breaks.
	for _, want := range []string{"linux", "darwin", "windows"} {
		if !seen[want] {
			t.Fatalf("read no %s target from .goreleaser.yml, got %v", want, targets)
		}
	}

	// One target at a time, in this test rather than a parallel subtest each: ten parallel
	// cross-compiles starved timed tests (jojo's picker previews, the pty resize test here) when hk ran ci
	// and verify-released together, and a slower guard costs nothing a flaky suite does not.
	for _, target := range targets {
		//nolint:gosec // the arguments are a constant and the matrix read above
		cmd := exec.CommandContext(t.Context(), "go", "build", "-o", os.DevNull, ".")
		cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH="+target.goarch, "CGO_ENABLED=0")

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("GOOS=%s GOARCH=%s go build: %v\n%s", target.goos, target.goarch, err, out)
		}
	}
}
