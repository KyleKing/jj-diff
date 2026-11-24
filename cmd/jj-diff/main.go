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

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/model"
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

type flags struct {
	applyPatch     string
	unapplyPatch   string
	revision       string
	scmInput       string
	destination    string
	tabWidth       int
	version        bool
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
	flag.BoolVar(&f.split, "split", false, "Open the revision in split columns (implies -i)")
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

	// The diff starts loading before anything else, because detecting the terminal's theme and color
	// depth waits on the terminal and on tmux, and none of that needs the diff.
	build, err := prepare(&f, flag.Args())
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	theme.Init()

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

	initialModel, err := build(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	p := tea.NewProgram(&initialModel)
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	// A diff editor that quits without writing exits non-zero, which is what tells jj to leave the
	// revision as it was instead of taking the right side it started with.
	if initialModel.Aborted() {
		fmt.Fprintln(os.Stderr, "jj-diff: quit without writing; the revision is unchanged")
		os.Exit(1)
	}
}

// runEditorHalf runs the half of a write jj hands back to this program, which draws nothing, and
// reports whether there was one.
func runEditorHalf(f *flags) bool {
	switch {
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
// background (the diff, the revision's label) is waited on here.
type builder func(cfg config.Config) (model.Model, error)

func prepare(f *flags, args []string) (builder, error) {
	switch len(args) {
	case 0:
		return prepareRevisionMode(f)
	case diffEditorArgCount:
		return prepareDiffEditorMode(args[0], args[1])
	default:
		return nil, errBadArgs
	}
}

func prepareRevisionMode(f *flags) (builder, error) {
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

	// The label and the diff are each a jj call the first frame waits on, so both run at once.
	revision := diff.NewRevisionSource(client, f.revision)
	source := diff.Prefetch(revision)

	labeled := make(chan error, 1)

	go func() { labeled <- revision.ResolveLabel() }()

	return func(cfg config.Config) (model.Model, error) {
		if err := <-labeled; err != nil {
			return model.Model{}, fmt.Errorf("reading the revision: %w", err)
		}

		m, err := model.NewModelWithSource(source, client, f.destination, mode, cfg)
		if err != nil {
			return model.Model{}, fmt.Errorf("building the revision model: %w", err)
		}

		if f.split {
			m.StartInColumns()
		}

		return m, nil
	}, nil
}

func prepareDiffEditorMode(leftDir, rightDir string) (builder, error) {
	if _, err := os.Stat(leftDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("left %s: %w", leftDir, errMissingDir)
	}
	if _, err := os.Stat(rightDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("right %s: %w", rightDir, errMissingDir)
	}

	source := diff.Prefetch(diff.NewDirectorySource(leftDir, rightDir))

	return func(cfg config.Config) (model.Model, error) {
		m, err := model.NewModelWithSource(source, nil, "", model.ModeDiffEditor, cfg)
		if err != nil {
			return model.Model{}, fmt.Errorf("building the diff-editor model: %w", err)
		}

		return m, nil
	}, nil
}
