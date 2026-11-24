// Package jj shells out to the jj binary for the diff, status, and revision data the UI needs, and
// carries the write paths that move, split out, or discard a selection. Every write is one jj command
// that runs this program back as jj's diff editor, so jj itself does the rewriting.
package jj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// statusFieldCount is the smallest number of whitespace-separated fields a jj status line carries: a
// one-letter change type and a path.
const statusFieldCount = 2

// Client runs jj in one repository. Every call shells out and blocks, so callers in the UI wrap them
// in a tea.Cmd. No call is cancellable, so each one runs under a background context.
type Client struct {
	baseDir string
	// editor is the jj-diff binary jj runs as the diff editor for a write, empty for this process.
	editor string
}

// Option configures a Client.
type Option func(*Client)

// WithEditor names the jj-diff binary jj runs to apply a selection, which is this process's own
// executable unless a caller (a test running inside go test) has to name a built one.
func WithEditor(path string) Option {
	return func(c *Client) { c.editor = path }
}

// NewClient runs jj with baseDir as the working directory, so baseDir decides which repository every
// call acts on.
func NewClient(baseDir string, opts ...Option) *Client {
	c := &Client{baseDir: baseDir}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Dir is the directory jj runs in, which decides the repository.
func (c *Client) Dir() string { return c.baseDir }

// jjCommand builds a jj invocation rooted at the client's repository.
func (c *Client) jjCommand(args ...string) *exec.Cmd {
	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.CommandContext(context.Background(), "jj", args...)
	cmd.Dir = c.baseDir

	return cmd
}

// CheckInstalled reports whether a jj binary is on PATH. It looks the binary up rather than running
// it, since a process start is time the first frame waits for, so it says nothing about baseDir
// being a jj repo.
func (*Client) CheckInstalled() error {
	if _, err := exec.LookPath("jj"); err != nil {
		return fmt.Errorf("jj command not found: %w", err)
	}

	return nil
}

// Diff returns the git-format diff for a revset, uncolored. The revset is resolved by jj at call
// time, so a moving revset such as @ follows the working copy.
func (c *Client) Diff(revision string) (string, error) {
	// stdout alone: a warning jj prints on stderr would otherwise be parsed as part of the diff.
	return c.executeJJ("diff", "-r", revision, "--git", "--color=never")
}

// Status lists the working copy's changed files. Lines jj prints that are not file entries are
// dropped, so an unparsable output yields an empty slice rather than an error.
func (c *Client) Status() ([]FileStatus, error) {
	cmd := c.jjCommand("status", "--no-pager")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj status failed: %w: %s", err, output)
	}

	return parseStatus(string(output)), nil
}

// ShowRevision returns one revision's metadata. Fields jj did not print are left empty rather than
// reported, so the result is never nil on success.
func (c *Client) ShowRevision(revision string) (*RevisionInfo, error) {
	cmd := c.jjCommand("show", "-r", revision, "--no-graph", "--summary")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj show failed: %w: %s", err, output)
	}

	return parseRevisionInfo(string(output)), nil
}

// RevisionLabel names a revset by its short change ID and the first line of its description.
func (c *Client) RevisionLabel(revset string) (string, error) {
	const template = `change_id.shortest(8) ++ " " ++ ` +
		`if(description, description.first_line(), "(no description)")`

	// A snapshot never changes a change ID or a description, so the label skips it. That keeps it
	// off the working-copy lock the diff loading beside it at startup takes.
	output, err := c.executeJJ(
		"log", "--ignore-working-copy", "-r", revset, "--no-graph", "--limit", "1", "-T", template,
	)
	if err != nil {
		return "", err
	}

	label := strings.TrimSpace(output)
	if label == "" {
		return "", fmt.Errorf("%q: %w", revset, errRevsetNoMatch)
	}

	return label, nil
}

// Undo reverts the last jj operation in the repository, whether or not this client caused it.
func (c *Client) Undo() error {
	cmd := c.jjCommand("undo")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj undo failed: %w: %s", err, output)
	}

	return nil
}

// resolveChangeID pins a revset to the change ID it names right now. A revset such as @- moves as the
// repository changes underneath it, so anything that outlives a single command has to hold the change
// ID instead of the revset that produced it.
func (c *Client) resolveChangeID(revset string) (string, error) {
	output, err := c.executeJJ("log", "-r", revset, "--no-graph", "--limit", "1", "-T", "change_id")
	if err != nil {
		return "", err
	}

	changeID := strings.TrimSpace(output)
	if changeID == "" {
		return "", fmt.Errorf("%q: %w", revset, errRevsetNoMatch)
	}

	return changeID, nil
}

// restoreOperationAfter reports cause, restoring opID first when an operation landed after it, and
// additionally reports when that restore failed and the repository is therefore left modified. A
// command jj refused writes no operation, and restoring then would only add one.
func (c *Client) restoreOperationAfter(opID string, cause error) error {
	if now, err := c.getCurrentOperationID(); err == nil && now == opID {
		return cause
	}

	if restoreErr := c.restoreOperation(opID); restoreErr != nil {
		return errors.Join(
			cause,
			fmt.Errorf("rollback to operation %s failed, repository may be left modified: %w", opID, restoreErr),
		)
	}

	return cause
}

// defaultRevisionLimit is how many rows GetRevisions lists when the caller names no limit.
const defaultRevisionLimit = 500

// Destinations is the revset a destination picker lists for source: every visible revision but the
// source itself and the root, which no selection can move into.
func Destinations(source string) string {
	return "~(" + source + ") ~ root()"
}

// GetRevisions lists revset's revisions in graph order, newest first, for a destination picker. A
// limit that is not positive lists defaultRevisionLimit rows.
func (c *Client) GetRevisions(revset string, limit int) ([]RevisionEntry, error) {
	if limit <= 0 {
		limit = defaultRevisionLimit
	}

	// jj emits one record per revision with no separator between them, so the
	// template has to terminate each record itself for the parser to find the
	// three-line boundaries.
	template := `separate("\n",` +
		`change_id.shortest() ++ "\t" ++ change_id.short(8) ++ "\t" ++ if(current_working_copy, "@") ++ ` +
		`"\t" ++ if(immutable, "!"),` +
		`if(description, description.first_line(), "(no description)")) ++ "\n---\n"`

	output, err := c.executeJJ(
		"log", "-r", revset, "--no-graph", "--limit", strconv.Itoa(limit), "--template", template,
	)
	if err != nil {
		return nil, err
	}

	return parseRevisionEntries(output), nil
}

// RevisionEntry is one row of the destination picker. ChangeID is jj's shortest unique prefix, so it
// is only valid against the repository it came from. Short is the eight-character form a person
// reads, which starts with ChangeID unless the prefix is longer than eight.
type RevisionEntry struct {
	ChangeID    string
	Short       string
	Description string
	// WorkingCopy marks the revision @ points at.
	WorkingCopy bool
	// Immutable marks a revision jj refuses to rewrite, which no squash can go into.
	Immutable bool
}

// FileStatus is one entry from jj status.
type FileStatus struct {
	Path       string
	ChangeType ChangeType
}

// ChangeType is what jj status says happened to a file. String returns the one-letter status jj
// prints.
type ChangeType int

// Change kinds a status line can report. ChangeTypeModified is the zero value and the fallback for a
// status letter that is not recognized.
const (
	ChangeTypeModified ChangeType = iota
	ChangeTypeAdded
	ChangeTypeDeleted
	ChangeTypeRenamed
)

func (ct ChangeType) String() string {
	switch ct {
	case ChangeTypeModified:
		return "M"
	case ChangeTypeAdded:
		return "A"
	case ChangeTypeDeleted:
		return "D"
	case ChangeTypeRenamed:
		return "R"
	default:
		return "?"
	}
}

// RevisionInfo is one revision's metadata as jj show prints it. Any field jj omitted is empty.
type RevisionInfo struct {
	ChangeID    string
	Description string
	Author      string
	Date        string
}

func parseStatus(output string) []FileStatus {
	var files []FileStatus
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Working copy") ||
			strings.HasPrefix(line, "Parent commit") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < statusFieldCount {
			continue
		}

		status := parts[0]
		path := strings.Join(parts[1:], " ")

		var changeType ChangeType
		switch status {
		case "M":
			changeType = ChangeTypeModified
		case "A":
			changeType = ChangeTypeAdded
		case "D":
			changeType = ChangeTypeDeleted
		case "R":
			changeType = ChangeTypeRenamed
		default:
			continue
		}

		files = append(files, FileStatus{
			Path:       path,
			ChangeType: changeType,
		})
	}

	return files
}

func parseRevisionInfo(output string) *RevisionInfo {
	info := &RevisionInfo{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Change ID:"):
			info.ChangeID = strings.TrimSpace(strings.TrimPrefix(line, "Change ID:"))
		case strings.HasPrefix(line, "Author:"):
			info.Author = strings.TrimSpace(strings.TrimPrefix(line, "Author:"))
		case strings.HasPrefix(line, "Date:"):
			info.Date = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		case info.Description == "" && line != "" && !strings.Contains(line, ":"):
			info.Description = line
		}
	}

	return info
}

func parseRevisionEntries(output string) []RevisionEntry {
	var entries []RevisionEntry
	lines := strings.Split(output, "\n")

	for i := 0; i < len(lines); i += 3 {
		if i+2 >= len(lines) {
			break
		}

		ids := strings.Split(strings.TrimSpace(lines[i]), "\t")
		changeID := ids[0]
		description := strings.TrimSpace(lines[i+1])
		separator := strings.TrimSpace(lines[i+2])

		if separator != "---" {
			continue
		}

		if changeID == "" {
			continue
		}

		entry := RevisionEntry{ChangeID: changeID, Short: changeID, Description: description}
		if len(ids) > 1 && strings.HasPrefix(ids[1], changeID) {
			entry.Short = ids[1]
		}

		const markField, immutableField = 2, 3
		entry.WorkingCopy = len(ids) > markField && ids[markField] == "@"
		entry.Immutable = len(ids) > immutableField && ids[immutableField] == "!"
		entries = append(entries, entry)
	}

	return entries
}

func (c *Client) getCurrentOperationID() (string, error) {
	output, err := c.executeJJ("op", "log", "--no-graph", "--limit", "1", "-T", "id")
	if err != nil {
		return "", fmt.Errorf("failed to get current operation ID: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func (c *Client) restoreOperation(opID string) error {
	if _, err := c.executeJJ("op", "restore", opID); err != nil {
		return fmt.Errorf("failed to restore operation: %w", err)
	}

	return nil
}

// executeJJStderr runs jj and answers what it printed on stderr, which is where it reports what a
// write did.
func (c *Client) executeJJStderr(args ...string) (string, error) {
	cmd := c.jjCommand(args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("jj %s failed: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return stderr.String(), nil
}

func (c *Client) executeJJ(args ...string) (string, error) {
	cmd := c.jjCommand(args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("jj %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// ChangeID is the full change ID a revset names now, the first when it names several.
func (c *Client) ChangeID(revset string) (string, error) {
	return c.resolveChangeID(revset)
}
