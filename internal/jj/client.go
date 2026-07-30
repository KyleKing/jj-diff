// Package jj shells out to the jj binary for the diff, status, and revision data the UI needs, and
// carries the one write path that moves selected changes into another revision.
package jj

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client runs jj in one repository. Every call shells out and blocks, so callers in the UI wrap them
// in a tea.Cmd.
type Client struct {
	baseDir string
}

// NewClient runs jj with baseDir as the working directory, so baseDir decides which repository every
// call acts on.
func NewClient(baseDir string) *Client {
	return &Client{baseDir: baseDir}
}

// CheckInstalled reports whether a jj binary is on PATH. It runs outside the repository, so it says
// nothing about baseDir being a jj repo.
func (c *Client) CheckInstalled() error {
	cmd := exec.Command("jj", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("jj command not found: %w", err)
	}

	return nil
}

// Diff returns the git-format diff for a revset, uncolored. The revset is resolved by jj at call
// time, so a moving revset such as @ follows the working copy.
func (c *Client) Diff(revision string) (string, error) {
	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.Command("jj", "diff", "-r", revision, "--git", "--color=never")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("jj diff failed: %w: %s", err, output)
	}

	return string(output), nil
}

// Status lists the working copy's changed files. Lines jj prints that are not file entries are
// dropped, so an unparseable output yields an empty slice rather than an error.
func (c *Client) Status() ([]FileStatus, error) {
	cmd := exec.Command("jj", "status", "--no-pager")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj status failed: %w: %s", err, output)
	}

	return parseStatus(string(output)), nil
}

// ShowRevision returns one revision's metadata. Fields jj did not print are left empty rather than
// reported, so the result is never nil on success.
func (c *Client) ShowRevision(revision string) (*RevisionInfo, error) {
	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.Command("jj", "show", "-r", revision, "--no-graph", "--summary")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj show failed: %w: %s", err, output)
	}

	return parseRevisionInfo(string(output)), nil
}

// Undo reverts the last jj operation in the repository, whether or not this client caused it.
func (c *Client) Undo() error {
	cmd := exec.Command("jj", "undo")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj undo failed: %w: %s", err, output)
	}

	return nil
}

// MoveChanges applies a patch onto destination and squashes it there. It resets the working copy to
// the destination as part of the sequence, so changes the patch does not carry are lost; see the
// first section of FINDINGS.md. The temp file holding the patch is removed even on failure, and a
// failure to remove it is joined onto the returned error.
func (c *Client) MoveChanges(patch, source, destination string) (err error) {
	tmpDir, err := os.MkdirTemp("", "jj-diff-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove temp dir %s: %w", tmpDir, rmErr))
		}
	}()

	patchFile := filepath.Join(tmpDir, "changes.patch")
	if err := os.WriteFile(patchFile, []byte(patch), 0o600); err != nil {
		return fmt.Errorf("failed to write patch: %w", err)
	}

	return c.moveChangesWithPatch(patchFile, destination)
}

// moveChangesWithPatch restores the original working copy before returning, so
// the returned error may carry a second, joined error from that restore.
func (c *Client) moveChangesWithPatch(patchFile, destination string) (err error) {
	destID, err := c.resolveChangeID(destination)
	if err != nil {
		return fmt.Errorf("failed to resolve destination %q: %w", destination, err)
	}

	currentWC, err := c.getCurrentWorkingCopy()
	if err != nil {
		return fmt.Errorf("failed to get current working copy: %w", err)
	}

	defer func() {
		// On the success path the squash consumes the working copy commit, so
		// currentWC no longer resolves and there is nothing to restore. Only a
		// bail-out leaves the caller sitting somewhere it did not start.
		if err == nil {
			return
		}

		if restoreErr := c.restoreWorkingCopy(currentWC); restoreErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("failed to restore working copy %s: %w", currentWC, restoreErr),
			)
		}
	}()

	if _, err := c.executeJJ("new", destID, "--no-edit"); err != nil {
		return fmt.Errorf("failed to create new commit: %w", err)
	}

	// Restore working copy to destination state so patch applies cleanly
	if _, err := c.executeJJ("restore", "--from", destID); err != nil {
		return c.undoAfter(fmt.Errorf("failed to restore working copy: %w", err))
	}

	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.Command("git", "apply", patchFile)
	cmd.Dir = c.baseDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return c.undoAfter(fmt.Errorf("failed to apply patch: %w: %s", err, output))
	}

	if _, err := c.executeJJ("squash", "--into", destID); err != nil {
		return c.undoAfter(fmt.Errorf("failed to squash changes: %w", err))
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
		return "", fmt.Errorf("revset %q matched no revision", revset)
	}

	return changeID, nil
}

func (c *Client) getCurrentWorkingCopy() (string, error) {
	return c.resolveChangeID("@")
}

func (c *Client) restoreWorkingCopy(changeID string) error {
	_, err := c.executeJJ("edit", changeID)
	return err
}

// undoAfter reports cause, and additionally reports when the rollback itself
// failed and the repository is therefore left modified.
func (c *Client) undoAfter(cause error) error {
	if undoErr := c.Undo(); undoErr != nil {
		return errors.Join(
			cause,
			fmt.Errorf("rollback failed, repository may be left modified: %w", undoErr),
		)
	}

	return cause
}

// restoreOperationAfter reports cause, and additionally reports when restoring
// opID failed and the repository is therefore left modified.
func (c *Client) restoreOperationAfter(opID string, cause error) error {
	if restoreErr := c.restoreOperation(opID); restoreErr != nil {
		return errors.Join(
			cause,
			fmt.Errorf("rollback to operation %s failed, repository may be left modified: %w", opID, restoreErr),
		)
	}

	return cause
}

// GetRevisions lists recent revisions newest first for the destination picker, defaulting to 20 when
// limit is not positive.
func (c *Client) GetRevisions(limit int) ([]RevisionEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	// jj emits one record per revision with no separator between them, so the
	// template has to terminate each record itself for the parser to find the
	// three-line boundaries.
	template := `separate("\n",` +
		`change_id.shortest(),` +
		`if(description, description.first_line(), "(no description)")) ++ "\n---\n"`

	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.Command("jj", "log",
		"--no-graph",
		"--limit", fmt.Sprintf("%d", limit),
		"--template", template)
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj log failed: %w: %s", err, output)
	}

	return parseRevisionEntries(string(output)), nil
}

// RevisionEntry is one row of the destination picker. ChangeID is jj's shortest unique prefix, so it
// is only valid against the repository it came from.
type RevisionEntry struct {
	ChangeID    string
	Description string
}

// SplitDestinationType separates a split target that already exists from one that will be created.
type SplitDestinationType int

// Targets a split plan can send its patch to. SplitDestExistingRevision is the zero value, so a
// SplitDestination that was never filled in reads as targeting an existing revision.
const (
	SplitDestExistingRevision SplitDestinationType = iota
	SplitDestNewCommit
)

// SplitDestination is where one split plan's patch lands. ChangeID is empty for SplitDestNewCommit,
// where Description becomes the message of the commit that gets created.
type SplitDestination struct {
	Type        SplitDestinationType
	ChangeID    string
	Description string
}

// SplitPlan is one tag's patch and the destination it goes to. Tag is carried through for error
// messages and has no meaning to jj.
type SplitPlan struct {
	Tag         rune
	Patch       string
	Destination SplitDestination
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
		if len(parts) < 2 {
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
		if strings.HasPrefix(line, "Change ID:") {
			info.ChangeID = strings.TrimSpace(strings.TrimPrefix(line, "Change ID:"))
		} else if strings.HasPrefix(line, "Author:") {
			info.Author = strings.TrimSpace(strings.TrimPrefix(line, "Author:"))
		} else if strings.HasPrefix(line, "Date:") {
			info.Date = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		} else if info.Description == "" && line != "" && !strings.Contains(line, ":") {
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

		changeID := strings.TrimSpace(lines[i])
		description := strings.TrimSpace(lines[i+1])
		separator := strings.TrimSpace(lines[i+2])

		if separator != "---" {
			continue
		}

		if changeID == "" {
			continue
		}

		entries = append(entries, RevisionEntry{
			ChangeID:    changeID,
			Description: description,
		})
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

func (c *Client) createNewCommit(description string) (string, error) {
	if _, err := c.executeJJ("new", "-m", description, "--no-edit"); err != nil {
		return "", fmt.Errorf("failed to create new commit: %w", err)
	}

	changeID, err := c.getCurrentWorkingCopy()
	if err != nil {
		return "", fmt.Errorf("failed to get new commit ID: %w", err)
	}

	return changeID, nil
}

func (c *Client) restoreOperation(opID string) error {
	if _, err := c.executeJJ("op", "restore", opID); err != nil {
		return fmt.Errorf("failed to restore operation: %w", err)
	}

	return nil
}

// ApplySplit runs the plans in order, creating a commit first for each plan that needs one. A failure
// part way through restores the operation recorded before the first plan, so the repository goes back
// to where it started rather than keeping the plans that already succeeded.
func (c *Client) ApplySplit(plans []SplitPlan, source string) error {
	if len(plans) == 0 {
		return fmt.Errorf("no split plans provided")
	}

	opID, err := c.getCurrentOperationID()
	if err != nil {
		return fmt.Errorf("failed to get operation ID for rollback: %w", err)
	}

	for i, plan := range plans {
		var destChangeID string

		if plan.Destination.Type == SplitDestNewCommit {
			changeID, err := c.createNewCommit(plan.Destination.Description)
			if err != nil {
				return c.restoreOperationAfter(opID, fmt.Errorf("failed to create new commit for tag %c: %w", plan.Tag, err))
			}
			destChangeID = changeID
		} else {
			destChangeID = plan.Destination.ChangeID
		}

		if err := c.MoveChanges(plan.Patch, source, destChangeID); err != nil {
			return c.restoreOperationAfter(opID, fmt.Errorf("failed to apply patch for tag %c (plan %d): %w", plan.Tag, i+1, err))
		}
	}

	return nil
}

func (c *Client) executeJJ(args ...string) (string, error) {
	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.Command("jj", args...)
	cmd.Dir = c.baseDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("jj %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}
