package jj

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct {
	baseDir string
}

type RevisionSpec struct {
	Raw         string
	ChangeID    string
	Description string
	Validated   bool
}

func NewClient(baseDir string) *Client {
	return &Client{baseDir: baseDir}
}

func (c *Client) CheckInstalled() error {
	cmd := exec.Command("jj", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("jj command not found: %w", err)
	}
	return nil
}

func (c *Client) Diff(revision string) (string, error) {
	cmd := exec.Command("jj", "diff", "-r", revision, "--git", "--color=never")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("jj diff failed: %w: %s", err, output)
	}

	return string(output), nil
}

func (c *Client) Status() ([]FileStatus, error) {
	cmd := exec.Command("jj", "status", "--no-pager")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj status failed: %w: %s", err, output)
	}

	return parseStatus(string(output)), nil
}

func (c *Client) ShowRevision(revision string) (*RevisionInfo, error) {
	cmd := exec.Command("jj", "show", "-r", revision, "--no-graph", "--summary")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jj show failed: %w: %s", err, output)
	}

	return parseRevisionInfo(string(output)), nil
}

func (c *Client) Undo() error {
	cmd := exec.Command("jj", "undo")
	cmd.Dir = c.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj undo failed: %w: %s", err, output)
	}

	return nil
}

type FileStatus struct {
	Path       string
	ChangeType ChangeType
}

type ChangeType int

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
		if line == "" || strings.HasPrefix(line, "Working copy") || strings.HasPrefix(line, "Parent commit") {
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

func (c *Client) executeJJ(args ...string) (string, error) {
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
