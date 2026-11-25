package main_test

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// stamp is the date and time log.Printf leads a line with by default.
var stamp = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

// An error is the program's own line, named for it, never a log record with a timestamp, and the
// usage names the program as it is typed rather than the absolute path it ran from.
func TestErrors_ReadAsTheProgramsOwnLine(t *testing.T) {
	t.Parallel()

	// #nosec G204 -- the binary TestMain built
	cmd := exec.CommandContext(t.Context(), binary, "-r", "@")
	cmd.Dir = t.TempDir()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		t.Fatal("jj-diff outside a repository succeeded")
	}

	line := strings.SplitN(stderr.String(), "\n", 2)[0]
	if stamp.MatchString(line) || !strings.HasPrefix(line, "jj-diff: ") {
		t.Errorf("the error line is %q, want it led by \"jj-diff: \" and no timestamp", line)
	}

	// #nosec G204 -- the binary TestMain built
	help := exec.CommandContext(t.Context(), binary, "-h")
	stderr.Reset()
	help.Stderr = &stderr
	_ = help.Run() //nolint:errcheck // -h exits non-zero by flag's convention; its text is the subject.

	if first := strings.SplitN(stderr.String(), "\n", 2)[0]; first != "Usage: jj-diff [options] [LEFT RIGHT]" {
		t.Errorf("usage line = %q, want the program's base name", first)
	}
}
