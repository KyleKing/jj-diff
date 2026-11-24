//nolint:testpackage // white-box: the quoting and the notice parsing are unexported.
package jj

import (
	"strings"
	"testing"
)

func TestTomlString_QuotesWhatTOMLRefusesRaw(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`/tmp/jj-diff`:   `"/tmp/jj-diff"`,
		`C:\bin\jj-diff`: `"C:\\bin\\jj-diff"`,
		`say "hi"`:       `"say \"hi\""`,
		"tab\there":      `"tab\u0009here"`,
		"$left":          `"$left"`,
	}

	for in, want := range tests {
		if got := tomlString(in); got != want {
			t.Errorf("tomlString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestConflictNotice_ReadsJJsLineAndTheRevisionsUnderIt(t *testing.T) {
	t.Parallel()

	stderr := "Working copy  (@) now at: yswrwnzm 71615543 src\n" +
		"New conflicts appeared in 1 commits:\n" +
		"  zmkwyook 1259aff9 (conflict) sib\n" +
		"Hint: To resolve the conflicts, start by creating a commit on top of\n"

	want := "New conflicts appeared in 1 commits: zmkwyook 1259aff9 (conflict) sib"
	if got := conflictNotice(stderr); got != want {
		t.Errorf("conflictNotice() = %q, want %q", got, want)
	}

	if got := conflictNotice("Working copy  (@) now at: x\n"); got != "" {
		t.Errorf("a clean write reported %q", got)
	}
}

func TestEchoed_DropsTheEditorPlumbing(t *testing.T) {
	t.Parallel()

	got := Echoed([]string{
		"jj", "squash", "--from", "@", "--into", "rnzx", "-u", "--tool", editorTool,
		"--config", "merge-tools.x.program=y", "--config", "ui.editor=false",
	})

	if want := "jj squash --from @ --into rnzx -u"; strings.Join(got, " ") != want {
		t.Errorf("Echoed = %q, want %q", got, want)
	}

	resolve := Echoed([]string{"jj", "resolve", "-r", "@", "--tool", ":editor", "--config", "ui.editor=[]"})
	if want := "jj resolve -r @ --tool :editor"; strings.Join(resolve, " ") != want {
		t.Errorf("Echoed keeps the resolve tool: got %q, want %q", resolve, want)
	}
}

func TestIsWrite(t *testing.T) {
	t.Parallel()

	for args, want := range map[string]bool{
		"squash --from @": true, "split -r @": true, "restore --changes-in @": true, "undo": true,
		"op restore abc": true, "resolve -r @": true, "resolve --list -r @": false, "diff -r @": false,
		"op log": false, "log -r @": false,
	} {
		if got := isWrite(strings.Fields(args)); got != want {
			t.Errorf("isWrite(%s) = %v, want %v", args, got, want)
		}
	}
}
