//nolint:testpackage // white-box: the quoting and the notice parsing are unexported.
package jj

import "testing"

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
