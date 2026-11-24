package model

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

// editedMsg is text coming back from the user's $EDITOR, for whoever asked: a column's description
// or a conflict region.
type editedMsg struct {
	err error
	// apply takes the text into the model.
	apply func(m *Model, text string)
	text  string
}

// editInEditor hands text to $VISUAL or $EDITOR (vi when neither is set) in a temporary file named
// name, suspending the screen while it runs, and sends the file's text back to apply.
func editInEditor(name, text string, apply func(m *Model, text string)) tea.Cmd {
	dir, err := os.MkdirTemp("", "jj-diff-edit-*")
	if err != nil {
		return func() tea.Msg { return editedMsg{err: fmt.Errorf("creating a file to edit: %w", err)} }
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil { //nolint:mnd // private to the user
		return func() tea.Msg { return editedMsg{err: errors.Join(err, os.RemoveAll(dir))} }
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = "vi"
	}

	// The editor is a command line the user wrote, so a shell splits it, and the file is its argument.
	//nolint:gosec // G204: the user's own editor setting, run on a file this program wrote.
	cmd := exec.CommandContext(context.Background(), "sh", "-c", editor+` "$1"`, "sh", path)

	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		b, readErr := os.ReadFile(path) //nolint:gosec // G304: the file written above.
		rmErr := os.RemoveAll(dir)

		if err := errors.Join(runErr, readErr, rmErr); err != nil {
			return editedMsg{err: fmt.Errorf("editing in %s: %w", editor, err)}
		}

		return editedMsg{text: string(b), apply: apply}
	})
}

// edited takes an editor's text home.
func (m *Model) edited(msg editedMsg) {
	if msg.err != nil {
		m.say(firstLine(msg.err), true)

		return
	}

	if msg.apply != nil {
		msg.apply(m, msg.text)
	}
}
