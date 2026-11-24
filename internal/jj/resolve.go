package jj

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// FlagResolveWith is the flag this program takes when jj runs it as the editor for jj resolve. It
// names the plan Resolve wrote, and the file to rewrite follows it.
const FlagResolveWith = "resolve-with"

// markerStyle is the conflict marker style every read and write here asks for: each side and base
// written whole under a label naming its revision, which is what the viewer shows and what makes the
// file jj hands the editor byte-identical to the one read.
const markerStyle = "ui.conflict-marker-style=snapshot"

var (
	errNoResolution  = errors.New("nothing is resolved")
	errPlanMismatch  = errors.New("jj handed the editor a file no resolution was planned for")
	errPlanUnapplied = errors.New("jj resolved fewer files than were planned")
)

//nolint:gochecknoglobals // compiled once, read only.
var conflictLine = regexp.MustCompile(`^(.*?)\s+(\d+-sided conflict.*)$`)

// ConflictedFile is one file jj lists as conflicted in a revision, with its text materialized in the
// snapshot marker style.
type ConflictedFile struct {
	Path string
	// Kind is jj's own word for the conflict: "2-sided conflict", "2-sided conflict including 1
	// deletion".
	Kind string
	Text string
}

// Conflicts lists revision's conflicted files with their text, empty when it has none.
func (c *Client) Conflicts(revision string) ([]ConflictedFile, error) {
	out, err := c.output(c.jjCommand("resolve", "--list", "-r", revision))
	if err != nil {
		if strings.Contains(stderrOf(err), "No conflicts found") {
			return nil, nil
		}

		return nil, fmt.Errorf("jj resolve --list failed: %w: %s", err, stderrOf(err))
	}

	var files []ConflictedFile

	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		m := conflictLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		text, err := c.executeJJ("file", "show", "-r", revision, flagConfig, markerStyle, "--", Fileset(m[1]))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", m[1], err)
		}

		files = append(files, ConflictedFile{Path: m[1], Kind: m[2], Text: text})
	}

	return files, nil
}

// stderrOf is what a failed command printed on stderr, which exec keeps on the exit error when the
// caller collected only stdout.
func stderrOf(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return strings.TrimSpace(string(exit.Stderr))
	}

	return ""
}

// Fileset is path as a jj fileset naming exactly that file, relative to the working directory.
func Fileset(path string) string {
	return "file:" + tomlString(path)
}

// Resolution is one file's new text. Conflicted is the text Conflicts read, which is how the editor
// recognizes the file jj hands it; Resolved may keep some regions' markers, which jj reads back as
// the conflict that remains.
type Resolution struct {
	Path       string
	Conflicted string
	Resolved   string
}

// planEntry is one resolution as the editor half reads it.
type planEntry struct {
	Base     string `json:"base"`
	Hash     string `json:"hash"`
	Resolved string `json:"resolved"`
}

// Resolve writes resolutions into revision as one jj operation: jj resolve with its :editor tool,
// whose editor is this program applying the plan. That tool takes any number of sides and keeps
// markers left in the file as the conflict that remains, which a three-way merge tool does neither
// of. A failure restores the operation recorded before it.
func (c *Client) Resolve(revision string, resolutions []Resolution) (_ Outcome, err error) {
	plan := make([]planEntry, 0, len(resolutions))
	paths := make([]string, 0, len(resolutions)+1)
	paths = append(paths, "--")

	for _, r := range resolutions {
		if r.Resolved == r.Conflicted {
			continue
		}

		entry := planEntry{Base: filepath.Base(r.Path), Hash: digest([]byte(r.Conflicted)), Resolved: r.Resolved}
		plan = append(plan, entry)
		paths = append(paths, Fileset(r.Path))
	}

	if len(plan) == 0 {
		return Outcome{}, errNoResolution
	}

	tmpDir, err := os.MkdirTemp("", "jj-diff-resolve-*")
	if err != nil {
		return Outcome{}, fmt.Errorf("failed to create temp dir: %w", err)
	}

	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove temp dir %s: %w", tmpDir, rmErr))
		}
	}()

	planPath := filepath.Join(tmpDir, "plan.json")
	if err := writePlan(planPath, plan); err != nil {
		return Outcome{}, err
	}

	program, err := c.program()
	if err != nil {
		return Outcome{}, err
	}

	editor := "ui.editor=[" + tomlString(program) + ", " + tomlString("--"+FlagResolveWith) + ", " +
		tomlString(planPath) + "]"
	args := []string{"resolve", "-r", revision, flagTool, ":editor", flagConfig, editor, flagConfig, markerStyle}

	opID, err := c.getCurrentOperationID()
	if err != nil {
		return Outcome{}, fmt.Errorf("failed to get operation ID for rollback: %w", err)
	}

	stderr, err := c.executeJJStderr(append(args, paths...)...)
	if err != nil {
		return Outcome{}, c.restoreOperationAfter(opID, err)
	}

	if left, err := readPlan(planPath); err != nil || len(left) > 0 {
		return Outcome{}, c.restoreOperationAfter(opID, errors.Join(errPlanUnapplied, err))
	}

	return Outcome{Conflicts: conflictNotice(stderr)}, nil
}

// ApplyResolution is the editor half of Resolve, which jj runs once per file with the file it
// materialized. It finds the plan entry whose conflicted text hashes as the file does, writes that
// entry's resolution over the file, and drops the entry from the plan, so a later file with the same
// text takes the next one.
func ApplyResolution(planPath, file string) error {
	plan, err := readPlan(planPath)
	if err != nil {
		return err
	}

	text, err := os.ReadFile(file) //nolint:gosec // G304: the file jj asked this program to edit.
	if err != nil {
		return fmt.Errorf("reading %s: %w", file, err)
	}

	hash := digest(text)

	for i, e := range plan {
		if e.Hash != hash || e.Base != filepath.Base(file) {
			continue
		}

		if err := os.WriteFile(file, []byte(e.Resolved), patchFileMode); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}

		return writePlan(planPath, append(plan[:i], plan[i+1:]...))
	}

	return fmt.Errorf("%s: %w", filepath.Base(file), errPlanMismatch)
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:])
}

func writePlan(path string, plan []planEntry) error {
	if plan == nil {
		plan = []planEntry{}
	}

	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("encoding the plan: %w", err)
	}

	if err := os.WriteFile(path, b, patchFileMode); err != nil {
		return fmt.Errorf("writing the plan: %w", err)
	}

	return nil
}

func readPlan(path string) ([]planEntry, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: the plan Resolve wrote.
	if err != nil {
		return nil, fmt.Errorf("reading the plan: %w", err)
	}

	var plan []planEntry
	if err := json.Unmarshal(b, &plan); err != nil {
		return nil, fmt.Errorf("decoding the plan: %w", err)
	}

	return plan, nil
}
