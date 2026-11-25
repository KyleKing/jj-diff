// Command jj-diff is a terminal diff viewer for Jujutsu. It runs in three modes: browse,
// interactive (which can move selected hunks to another revision), and diff-editor (which jj
// invokes with a left and a right directory). A fourth, with --apply-patch or --unapply-patch, draws
// nothing: it is how jj-diff's own writes have jj run it back as the diff editor, with the selection
// already chosen.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/keyusage"
	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/launch"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/model"
	"github.com/kyleking/jj-diff/internal/reads"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Sentinel errors main returns on its own rather than wrapping one from a package it calls.
var (
	errMissingDir     = errors.New("directory does not exist")
	errScmRecordUnimp = errors.New("scm-record compatibility mode is not implemented")
	errBadArgs        = errors.New(
		"invalid arguments: use 'jj-diff' for revision mode or 'jj-diff LEFT RIGHT' for diff-editor mode",
	)
)

var (
	version = "dev"     //nolint:gochecknoglobals // set by goreleaser ldflags
	commit  = "none"    //nolint:gochecknoglobals // set by goreleaser ldflags
	date    = "unknown" //nolint:gochecknoglobals // set by goreleaser ldflags
)

// diffEditorArgCount is the positional argument count that means jj invoked the program as a diff
// editor, passing the left and right directories.
const diffEditorArgCount = 2

// appName is what the coach's ledger and the usage log are filed under: $XDG_STATE_HOME/jj-diff.
const appName = "jj-diff"

type flags struct {
	resolveWith    string
	applyPatch     string
	unapplyPatch   string
	revision       string
	scmInput       string
	destination    string
	tabWidth       int
	version        bool
	resolve        bool
	split          bool
	browse         bool
	interactive    bool
	showWhitespace bool
	sideBySide     bool
	wordDiff       bool
}

func parseFlags() flags {
	var f flags

	flag.BoolVar(&f.version, "version", false, "Show program version")
	flag.BoolVar(&f.version, "v", false, "Show program version (shorthand)")
	flag.StringVar(&f.revision, "r", "@", "Revision to view/edit")
	flag.StringVar(&f.revision, "revision", "@", "Revision to view/edit")
	flag.BoolVar(&f.browse, "browse", false, "Force browse mode (read-only)")
	flag.BoolVar(&f.interactive, "interactive", false, "Force interactive mode")
	flag.BoolVar(&f.interactive, "i", false, "Force interactive mode (shorthand)")
	flag.StringVar(
		&f.scmInput,
		"scm-input",
		"",
		"Path to scm-record input file (compatibility mode)",
	)
	flag.StringVar(&f.destination, "destination", "", "Pre-set destination revision")
	flag.StringVar(&f.destination, "d", "", "Pre-set destination revision (shorthand)")
	flag.BoolVar(&f.showWhitespace, "show-whitespace", false, "Visualize whitespace characters")
	flag.BoolVar(&f.sideBySide, "side-by-side", false, "Side-by-side diff view")
	flag.BoolVar(&f.sideBySide, "s", false, "Side-by-side diff view (shorthand)")
	flag.BoolVar(&f.wordDiff, "word-diff", false, "Enable word-level highlighting")
	flag.StringVar(&f.applyPatch, jj.FlagApplyPatch, "",
		"With LEFT RIGHT: make RIGHT the LEFT tree plus this patch, drawing nothing (jj-diff's own writes)")
	flag.StringVar(&f.unapplyPatch, jj.FlagUnapplyPatch, "",
		"With LEFT RIGHT: take this patch back out of the LEFT tree first (jj-diff's own writes)")
	flag.BoolVar(&f.resolve, "resolve", false, "Open the conflict viewer on the revision's conflicted files")
	flag.BoolVar(&f.resolve, "conflicts", false, "Same as --resolve")
	flag.BoolVar(&f.split, "split", false, "Open the revision in split columns (implies -i)")
	flag.StringVar(&f.resolveWith, jj.FlagResolveWith, "",
		"With FILE: write the planned resolution over FILE, drawing nothing (jj-diff's own resolve)")
	flag.IntVar(
		&f.tabWidth,
		"tab-width",
		0,
		"Tab display width (default: 4, 0 uses config/default)",
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [LEFT RIGHT]\n\n", os.Args[0])
		fmt.Fprintf(
			os.Stderr,
			"A TUI for interactive diff viewing and manipulation in Jujutsu (jj)\n\n",
		)
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  jj-diff              # Browse working copy changes\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -r @-        # Browse parent's changes\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -i           # Interactive mode (squash, split, or discard hunks)\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -i -d @-     # c squashes the selection into the parent\n")
		fmt.Fprintf(os.Stderr, "  jj-diff --split -r X # Split X into commits, one column each\n")
		fmt.Fprintf(os.Stderr, "  jj-diff --resolve -r X  # Pick sides for X's conflicts\n")
		fmt.Fprintf(
			os.Stderr,
			"  jj-diff LEFT RIGHT   # Diff-editor mode (for jj split, diffedit)\n",
		)
	}

	flag.Parse()

	return f
}

func main() {
	f := parseFlags()

	if f.version {
		fmt.Printf("jj-diff %s (commit: %s, built: %s)\n", version, commit, date)
		return
	}

	if runEditorHalf(&f) {
		return
	}

	// The diff starts loading before anything else, because the first frame waits on it and nothing
	// about it needs the theme or the terminal.
	build, err := prepare(&f, flag.Args())
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	cfg := configure(&f)

	// The revision's label is read before the terminal is taken, so a revision that does not exist
	// is reported on the shell's screen.
	if err := build.wait(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// The coach's ledger is always on: it holds tip ids and action names with times, never keys. The
	// usage log is opt-in, and nil when it is off.
	ledger, err := coach.OpenLedger(appName)
	if err != nil {
		log.Printf("coach: starting without its notes: %v", err)
	}

	usage := keyusage.Open(appName)

	var (
		app      *model.Model
		buildErr error
	)

	start := func(dark bool) tea.Model {
		theme.InitFor(dark)
		theme.UseIcons(cfg.Icons)

		m, err := build.make(cfg)
		if err != nil {
			buildErr = err

			return failed{}
		}

		m.WithLedger(ledger).WithUsage(usage)
		app = &m

		return app
	}

	// A theme left to the terminal is asked for inside the program, where the answer cannot eat a key
	// typed as the command was entered.
	var first tea.Model = launch.New(start)
	if theme.Pinned() {
		first = start(true)
	}

	// The renderer draws at the depth the theme was fitted to, which inside tmux is not what
	// Bubble Tea would detect on its own.
	p := tea.NewProgram(first, tea.WithColorProfile(theme.Profile()))
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	if buildErr != nil {
		log.Fatalf("Failed to initialize: %v", buildErr)
	}

	if err := ledger.Save(); err != nil {
		log.Printf("coach: saving its notes: %v", err)
	}

	if err := usage.Close(); err != nil {
		log.Printf("usage: %v", err)
	}

	report(app, build.editor)
}

// configure reads the config file and lays the flags over it.
func configure(f *flags) config.Config {
	cfg := config.LoadConfig()

	if f.showWhitespace {
		cfg.ShowWhitespace = true
	}
	if f.sideBySide {
		cfg.ViewMode = config.ViewModeSideBySide
	}
	if f.wordDiff {
		cfg.WordLevelDiff = true
	}
	if f.tabWidth > 0 {
		cfg.TabWidth = f.tabWidth
	}

	return cfg
}

// report says what the session wrote once the screen is gone, and exits non-zero for a diff editor
// that wrote nothing. A program that ended before its model was built (app nil) wrote nothing.
func report(app *model.Model, editor bool) {
	// The program that launched this one can repeat the outcome: the last line that starts with
	// "jj-diff: " is it.
	if app != nil {
		if outcome := app.Outcome(); outcome != "" {
			fmt.Fprintln(os.Stderr, "jj-diff: "+outcome)
		}
	}

	// A diff editor that quits without writing exits non-zero, which is what tells jj to leave the
	// revision as it was instead of taking the right side it started with.
	aborted := editor
	if app != nil {
		aborted = app.Aborted()
	}

	if aborted {
		fmt.Fprintln(os.Stderr, "jj-diff: quit without writing; the revision is unchanged")
		os.Exit(1)
	}
}

// runEditorHalf runs the half of a write jj hands back to this program, which draws nothing, and
// reports whether there was one.
func runEditorHalf(f *flags) bool {
	switch {
	case f.resolveWith != "":
		if flag.NArg() != 1 {
			log.Fatalf("Failed to resolve: %v", errBadArgs)
		}

		if err := jj.ApplyResolution(f.resolveWith, flag.Arg(0)); err != nil {
			log.Fatalf("Failed to resolve: %v", err)
		}
	case f.applyPatch != "" || f.unapplyPatch != "":
		if flag.NArg() != diffEditorArgCount {
			log.Fatalf("Failed to apply the selection: %v", errBadArgs)
		}

		if err := jj.ApplySelection(flag.Arg(0), flag.Arg(1), f.unapplyPatch, f.applyPatch); err != nil {
			log.Fatalf("Failed to apply the selection: %v", err)
		}
	default:
		return false
	}

	return true
}

// builder finishes a model once the theme and the config are known. The work prepare started in the
// background (the diff, the revision's label) runs while the terminal is set up: wait is the part a
// model cannot be built without, and make builds it once the theme is in place.
type builder struct {
	wait func() error
	make func(cfg config.Config) (model.Model, error)
	// editor is diff-editor mode, which exits non-zero when it writes nothing.
	editor bool
}

func prepare(f *flags, args []string) (*builder, error) {
	switch len(args) {
	case 0:
		return prepareRevisionMode(f)
	case diffEditorArgCount:
		return prepareDiffEditorMode(args[0], args[1])
	default:
		return nil, errBadArgs
	}
}

func prepareRevisionMode(f *flags) (*builder, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	client := jj.NewClient(wd)

	if err := client.CheckInstalled(); err != nil {
		return nil, fmt.Errorf("jj is not installed or not in PATH: %w", err)
	}

	if f.scmInput != "" {
		return nil, fmt.Errorf("%s: %w", f.scmInput, errScmRecordUnimp)
	}

	mode := model.ModeBrowse
	if f.interactive || f.split {
		mode = model.ModeInteractive
	}

	if f.resolve {
		mode = model.ModeResolve
	}

	// The label and the diff are each a jj call the first frame waits on, so both run at once.
	revision := diff.NewRevisionSource(client, f.revision)
	// The conflict viewer reads the conflicts rather than the diff, so it starts no diff.
	var source diff.Source = revision
	if !f.resolve {
		source = diff.Prefetch(revision)
	}

	labeled := make(chan error, 1)

	go func() { labeled <- revision.ResolveLabel() }()

	wait := func() error {
		if err := <-labeled; err != nil {
			return fmt.Errorf("reading the revision: %w", err)
		}

		return nil
	}

	return &builder{wait: wait, make: func(cfg config.Config) (model.Model, error) {
		m, err := model.NewModelWithSource(source, client, f.destination, mode, cfg)
		if err != nil {
			return model.Model{}, fmt.Errorf("building the revision model: %w", err)
		}

		if f.split {
			m.StartInColumns()
		}

		// A launch for one write hands the terminal back once it lands, as jojo expects of both.
		if f.split || f.resolve {
			m.ReturnAfterWrite()
		}

		// jojo's read marks for this repository, shown read-only. A directory with no jj repository
		// or no state directory shows none.
		if store, err := reads.Open(wd); err == nil {
			m.WithReads(store)
		}

		return m, nil
	}}, nil
}

func prepareDiffEditorMode(leftDir, rightDir string) (*builder, error) {
	if _, err := os.Stat(leftDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("left %s: %w", leftDir, errMissingDir)
	}
	if _, err := os.Stat(rightDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("right %s: %w", rightDir, errMissingDir)
	}

	source := diff.Prefetch(diff.NewDirectorySource(leftDir, rightDir))

	build := func(cfg config.Config) (model.Model, error) {
		m, err := model.NewModelWithSource(source, nil, "", model.ModeDiffEditor, cfg)
		if err != nil {
			return model.Model{}, fmt.Errorf("building the diff-editor model: %w", err)
		}

		return m, nil
	}

	return &builder{wait: func() error { return nil }, make: build, editor: true}, nil
}

// failed is the program when its model could not be built: it quits before drawing anything, and
// main reports why once the terminal is back.
type failed struct{}

func (failed) Init() tea.Cmd                         { return tea.Quit }
func (f failed) Update(tea.Msg) (tea.Model, tea.Cmd) { return f, nil }
func (failed) View() tea.View                        { return tea.NewView("") }
