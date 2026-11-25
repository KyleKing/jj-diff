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
// and goarch values, the pairs ignore drops, and whether the build sets
// CGO_ENABLED=1.
type releaseMatrix struct {
	ignored      map[releaseTarget]bool
	list         string // the list the current line belongs to
	pending      string // an ignore entry's goos, waiting for its goarch
	goos, goarch []string
	cgo          bool
}

// read takes one trimmed line of .goreleaser.yml. It is a line reader for the
// template's own shape, not YAML: a key ending in a colon opens a list, and
// any line that is not one of its entries closes it.
func (m *releaseMatrix) read(line string) {
	switch line {
	case "env:", "goos:", "goarch:", "ignore:":
		m.list = strings.TrimSuffix(line, ":")
	default:
		if !m.entry(line) {
			m.list = ""
		}
	}
}

// entry records line as an entry of the open list and reports whether it was one.
func (m *releaseMatrix) entry(line string) bool {
	item, isItem := strings.CutPrefix(line, "- ")
	value := func(s, key string) string {
		return strings.Trim(strings.TrimSpace(strings.TrimPrefix(s, key)), `"'`)
	}

	switch {
	case m.list == "ignore" && isItem && strings.HasPrefix(item, "goos:"):
		m.pending = value(item, "goos:")
	case m.list == "ignore" && strings.HasPrefix(line, "goarch:"):
		m.ignored[releaseTarget{m.pending, value(line, "goarch:")}] = true
	case !isItem:
		return false
	case m.list == "env":
		m.cgo = m.cgo || value(item, "") == "CGO_ENABLED=1"
	case m.list == "goos":
		m.goos = append(m.goos, value(item, ""))
	case m.list == "goarch":
		m.goarch = append(m.goarch, value(item, ""))
	default:
		return false
	}

	return true
}

// targets is every goos crossed with every goarch, less the ignore pairs.
func (m *releaseMatrix) targets() []releaseTarget {
	var out []releaseTarget

	for _, goos := range m.goos {
		for _, goarch := range m.goarch {
			if target := (releaseTarget{goos, goarch}); !m.ignored[target] {
				out = append(out, target)
			}
		}
	}

	return out
}

// readReleaseMatrix reads the build matrix out of .goreleaser.yml rather than
// repeating it, so a target added or dropped there is covered without an edit
// here.
func readReleaseMatrix(t *testing.T) releaseMatrix {
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

	return matrix
}

// TestEveryReleaseTargetBuilds compiles the binary for each pair goreleaser
// ships. CI runs on Linux only, so a Unix-only call (an ioctl, a syscall
// constant) would pass every other check and fail only the release workflow,
// after the tag is cut. It skips under -short, since a cold build cache
// compiles the standard library once per target.
//
// The template keeps this file current, so a project-specific build check
// belongs in a file of its own.
func TestEveryReleaseTargetBuilds(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("cross-compiles the binary for every release target")
	}

	matrix := readReleaseMatrix(t)
	if matrix.cgo {
		t.Skip("CGO_ENABLED=1 cannot cross-compile; the release builds each target on its own runner")
	}

	targets := matrix.targets()
	// A reader that silently found nothing would pass.
	if len(targets) == 0 {
		t.Fatal("read no goos/goarch targets from .goreleaser.yml")
	}

	// One target at a time rather than a parallel subtest each: ten parallel
	// cross-compiles starved other packages' timed tests when hk ran ci and
	// verify-released together, and a slower guard costs less than a flaky suite.
	for _, target := range targets {
		//nolint:gosec // the arguments are a constant and the matrix read above
		cmd := exec.CommandContext(t.Context(), "go", "build", "-o", os.DevNull, ".")
		cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH="+target.goarch, "CGO_ENABLED=0")

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("GOOS=%s GOARCH=%s go build: %v\n%s", target.goos, target.goarch, err, out)
		}
	}
}
