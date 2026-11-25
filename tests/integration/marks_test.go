package integration_test

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/seen"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/model"
	"github.com/kyleking/jj-diff/internal/reads"
	"github.com/kyleking/jj-diff/tests/integration"
)

// browse is jj-diff over rev in a real repository, as main builds it, with the shared marks and its
// first diff and scope loaded.
func browse(t *testing.T, repo *integration.TestRepo, rev string) *model.Model {
	t.Helper()

	client := jj.NewClient(repo.Dir)

	store, err := reads.Open(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	built, err := model.NewModelWithSource(diff.NewRevisionSource(client, rev), client, "", model.ModeBrowse,
		config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	m := built.WithReads(store)

	return pump(t, m, m.Init())
}

// pump runs cmd and feeds what it returns back through Update until nothing is left, as the runtime
// would; a batch or a sequence runs each of its commands.
func pump(t *testing.T, m *model.Model, cmd tea.Cmd) *model.Model {
	t.Helper()

	queue := []tea.Cmd{cmd}

	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]

		for _, msg := range run(next) {
			if _, ok := msg.(tea.QuitMsg); ok {
				continue
			}

			out, more := m.Update(msg)

			got, ok := out.(*model.Model)
			if !ok {
				t.Fatalf("Update returned %T", out)
			}

			m = got
			queue = append(queue, more)
		}
	}

	return m
}

func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, run(c)...)
		}

		return out
	}

	// tea.Sequence's message is unexported; it is a slice of commands run in order.
	if v := reflect.ValueOf(msg); v.IsValid() && v.Kind() == reflect.Slice &&
		v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
		var out []tea.Msg
		for i := range v.Len() {
			c, _ := v.Index(i).Interface().(tea.Cmd) //nolint:errcheck // the element type was checked above.
			out = append(out, run(c)...)
		}

		return out
	}

	if msg == nil {
		return nil
	}

	return []tea.Msg{msg}
}

func press(t *testing.T, m *model.Model, keys ...string) *model.Model {
	t.Helper()

	for _, k := range keys {
		out, cmd := m.Update(model.Key(k))

		got, ok := out.(*model.Model)
		if !ok {
			t.Fatalf("Update returned %T", out)
		}

		m = pump(t, got, cmd)
	}

	return m
}

// Space in browse writes the mark where jojo reads it: the repository's shared store, under the full
// change id, by the hunk's content id. A second session (a reload) opens on it read, and space
// again unmarks it for everyone.
func TestBrowseSpace_MarksTheHunkInJojosStore(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	repo := integration.NewTestRepo(t)
	repo.WriteFile("a.go", "package a\n\nfunc A() {}\n")
	repo.WriteFile("b.txt", "one\n")
	repo.Commit("base")
	repo.WriteFile("a.go", "package a\n\n// A is a.\nfunc A() {}\n")
	repo.WriteFile("b.txt", "two\n")

	change := strings.TrimSpace(repo.MustRun("log", "-r", "@", "--no-graph", "-T", "change_id"))
	live := seen.Parse([]byte(repo.GetDiff("@")))

	if len(live) != 2 {
		t.Fatalf("the fixture has %d hunks, want 2", len(live))
	}

	press(t, browse(t, repo, "@"), "enter", "space")

	// jojo's view of it: its own store for the repository, opened fresh.
	jojo, err := seen.OpenJJ(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	if jojo.Unread(change, string(live[0].ID)) {
		t.Fatalf("jojo's store does not have %s's first hunk read after space", live[0].Path)
	}

	if !jojo.Unread(change, string(live[1].ID)) {
		t.Error("space marked a hunk it was not on")
	}

	// A new session over the same change draws the hunk read: its file's heading counts 1 of 2.
	reloaded := press(t, browse(t, repo, "@"), "enter")
	if view := ansi.Strip(reloaded.View().Content); !strings.Contains(view, "read  +1  1/2") {
		t.Errorf("the reloaded session does not draw the mark:\n%s", view)
	}

	press(t, reloaded, "space")

	again, err := seen.OpenJJ(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	if !again.Unread(change, string(live[0].ID)) {
		t.Error("space on a read hunk after a reload did not unmark it")
	}
}
