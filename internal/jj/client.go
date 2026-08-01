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
	"slices"
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
// dropped, so an unparsable output yields an empty slice rather than an error.
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

// MoveChanges applies a patch onto destination and squashes it there. Nothing the patch does not carry
// is touched: the caller's working copy is never read, written, or moved by the sequence, because every
// write happens in a throwaway workspace. The temp file holding the patch is removed even on failure,
// and a failure to remove it is joined onto the returned error.
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

// moveChangesWithPatch pins the destination before any command runs, then builds the patch into a
// commit from a scratch workspace. A failure anywhere rolls the repository back to the operation
// recorded up front, and a rollback that itself failed is reported alongside the cause.
func (c *Client) moveChangesWithPatch(patchFile, destination string) error {
	destID, err := c.resolveChangeID(destination)
	if err != nil {
		return fmt.Errorf("failed to resolve destination %q: %w", destination, err)
	}

	opID, err := c.getCurrentOperationID()
	if err != nil {
		return fmt.Errorf("failed to get operation ID for rollback: %w", err)
	}

	if err := c.applyPatchInScratchWorkspace(patchFile, destID); err != nil {
		return c.restoreOperationAfter(opID, err)
	}

	return nil
}

// Sentinel errors the move path returns on its own rather than wrapping one from jj or git.
var (
	errRevsetNoMatch       = errors.New("revset matched no revision")
	errNewCommitNotFound   = errors.New("jj created a commit that could not be found afterwards")
	errPatchChangedNothing = errors.New("the patch applied cleanly but changed nothing, so there is nothing to move")
)

// scratchWorkspacePrefix names both the temp directory and the jj workspace, so a leaked workspace is
// identifiable in jj workspace list.
const scratchWorkspacePrefix = "jj-diff-scratch"

// applyPatchInScratchWorkspace builds the patch into a commit on destID from a workspace of its own.
// No command here names @, so the caller's working copy is untouched whether the run succeeds or
// fails. The workspace is forgotten and its directory removed on every return path, including a panic,
// though a panic discards the cleanup's own error along with the return value.
func (c *Client) applyPatchInScratchWorkspace(patchFile, destID string) (err error) {
	root, err := os.MkdirTemp("", scratchWorkspacePrefix+"-*")
	if err != nil {
		return fmt.Errorf("failed to create scratch workspace directory: %w", err)
	}

	// jj creates the workspace directory itself and refuses an existing one, so it goes under root
	// rather than being root.
	name := filepath.Base(root)
	dir := filepath.Join(root, "workspace")

	if _, addErr := c.executeJJ("workspace", "add", "--name", name, dir); addErr != nil {
		addErr = fmt.Errorf("failed to create scratch workspace: %w", addErr)
		if rmErr := os.RemoveAll(root); rmErr != nil {
			addErr = errors.Join(addErr, fmt.Errorf("failed to remove %s: %w", root, rmErr))
		}

		return addErr
	}

	defer func() {
		err = errors.Join(err, c.removeScratchWorkspace(name, root))
	}()

	scratch := &Client{baseDir: dir}

	if _, err := scratch.executeJJ("new", destID); err != nil {
		return fmt.Errorf("failed to create scratch commit on %s: %w", destID, err)
	}

	if err := applyPatchFile(dir, root, patchFile); err != nil {
		return err
	}

	changed, err := scratch.Diff("@")
	if err != nil {
		return fmt.Errorf("failed to read the scratch commit: %w", err)
	}

	if strings.TrimSpace(changed) == "" {
		return errPatchChangedNothing
	}

	if _, err := scratch.executeJJ("squash", "--into", destID); err != nil {
		return fmt.Errorf("failed to squash changes into %s: %w", destID, err)
	}

	return nil
}

// removeScratchWorkspace drops the workspace from the repository and deletes its directory, reporting
// both failures rather than the first, because either one left behind is a leak the caller should see.
func (c *Client) removeScratchWorkspace(name, root string) error {
	var errs error

	if _, err := c.executeJJ("workspace", "forget", name); err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to forget scratch workspace %s: %w", name, err))
	}

	if err := os.RemoveAll(root); err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to remove %s: %w", root, err))
	}

	return errs
}

// applyPatchFile applies patchFile inside dir. Git resolves a patch's paths against the nearest
// enclosing repository rather than against the working directory, so an unrelated repository above
// ceiling would turn the apply into a silent no-op. The ceiling stops that search.
func applyPatchFile(dir, ceiling, patchFile string) error {
	if resolved, err := filepath.EvalSymlinks(ceiling); err == nil {
		ceiling = resolved
	}

	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.Command("git", "apply", patchFile)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+ceiling)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply patch: %w: %s", err, output)
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

// createNewCommit adds an empty described commit as a child of the working copy and returns its change
// ID. Because jj does not report the ID it created and --no-edit leaves @ where it was, the ID is
// found by diffing the working copy's children across the call rather than by reading @ afterwards.
func (c *Client) createNewCommit(description string) (string, error) {
	before, err := c.changeIDs("@+")
	if err != nil {
		return "", fmt.Errorf("failed to list existing children: %w", err)
	}

	if _, err := c.executeJJ("new", "-m", description, "--no-edit"); err != nil {
		return "", fmt.Errorf("failed to create new commit: %w", err)
	}

	after, err := c.changeIDs("@+")
	if err != nil {
		return "", fmt.Errorf("failed to list children after creating the commit: %w", err)
	}

	for _, changeID := range after {
		if !slices.Contains(before, changeID) {
			return changeID, nil
		}
	}

	return "", errNewCommitNotFound
}

func (c *Client) changeIDs(revset string) ([]string, error) {
	output, err := c.executeJJ("log", "-r", revset, "--no-graph", "-T", `change_id ++ "\n"`)
	if err != nil {
		return nil, err
	}

	return strings.Fields(output), nil
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
