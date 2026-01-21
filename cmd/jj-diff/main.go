package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/model"
	"github.com/kyleking/jj-diff/internal/theme"
)

const version = "0.1.0"

type flags struct {
	version        bool
	revision       string
	browse         bool
	interactive    bool
	scmInput       string
	destination    string
	showWhitespace bool
	sideBySide     bool
	wordDiff       bool
	tabWidth       int
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
	flag.StringVar(&f.scmInput, "scm-input", "", "Path to scm-record input file (compatibility mode)")
	flag.StringVar(&f.destination, "destination", "", "Pre-set destination revision")
	flag.StringVar(&f.destination, "d", "", "Pre-set destination revision (shorthand)")
	flag.BoolVar(&f.showWhitespace, "show-whitespace", false, "Visualize whitespace characters")
	flag.BoolVar(&f.sideBySide, "side-by-side", false, "Side-by-side diff view")
	flag.BoolVar(&f.sideBySide, "s", false, "Side-by-side diff view (shorthand)")
	flag.BoolVar(&f.wordDiff, "word-diff", false, "Enable word-level highlighting")
	flag.IntVar(&f.tabWidth, "tab-width", 0, "Tab display width (default: 4, 0 uses config/default)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [LEFT RIGHT]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A TUI for interactive diff viewing and manipulation in Jujutsu (jj)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  jj-diff              # Browse working copy changes\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -r @-        # Browse parent's changes\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -i           # Interactive mode (move changes)\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -i -d @-     # Move changes to parent\n")
		fmt.Fprintf(os.Stderr, "  jj-diff LEFT RIGHT   # Diff-editor mode (for jj split, diffedit)\n")
	}

	flag.Parse()
	return f
}

func main() {
	f := parseFlags()

	if f.version {
		fmt.Printf("jj-diff %s\n", version)
		return
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

	var initialModel tea.Model
	var err error

	args := flag.Args()
	if len(args) == 2 {
		initialModel, err = initDiffEditorMode(args[0], args[1], cfg)
	} else if len(args) == 0 {
		initialModel, err = initRevisionMode(f, cfg)
	} else {
		log.Fatalf("Invalid arguments. Use 'jj-diff' for revision mode or 'jj-diff LEFT RIGHT' for diff-editor mode.")
	}

	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	p := tea.NewProgram(initialModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func initRevisionMode(f flags, cfg config.Config) (tea.Model, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	client := jj.NewClient(wd)

	if err := client.CheckInstalled(); err != nil {
		return nil, fmt.Errorf("jj is not installed or not in PATH: %w", err)
	}

	if f.scmInput != "" {
		return nil, fmt.Errorf("scm-record compatibility mode not yet implemented")
	}

	mode := model.ModeBrowse
	if f.interactive {
		mode = model.ModeInteractive
	}

	source := diff.NewRevisionSource(client, f.revision)
	return model.NewModelWithSource(source, client, f.destination, mode, cfg)
}

func initDiffEditorMode(leftDir, rightDir string, cfg config.Config) (tea.Model, error) {
	if _, err := os.Stat(leftDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("left directory does not exist: %s", leftDir)
	}
	if _, err := os.Stat(rightDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("right directory does not exist: %s", rightDir)
	}

	source := diff.NewDirectorySource(leftDir, rightDir)
	return model.NewModelWithSource(source, nil, "", model.ModeDiffEditor, cfg)
}
