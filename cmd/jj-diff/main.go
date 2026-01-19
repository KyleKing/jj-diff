package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/model"
)

const version = "0.1.0"

type flags struct {
	version     bool
	revision    string
	browse      bool
	interactive bool
	scmInput    string
	destination string
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

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A TUI for interactive diff viewing and manipulation in Jujutsu (jj)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  jj-diff              # Browse working copy changes\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -r @-        # Browse parent's changes\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -i           # Interactive mode (move changes)\n")
		fmt.Fprintf(os.Stderr, "  jj-diff -i -d @-     # Move changes to parent\n")
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

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	client := jj.NewClient(wd)

	if err := client.CheckInstalled(); err != nil {
		log.Fatalf("jj is not installed or not in PATH: %v", err)
	}

	var initialModel tea.Model

	if f.scmInput != "" {
		log.Fatal("scm-record compatibility mode not yet implemented")
	}

	mode := model.ModeBrowse
	if f.interactive {
		mode = model.ModeInteractive
	}

	initialModel, err = model.NewModel(client, f.revision, f.destination, mode)
	if err != nil {
		log.Fatalf("Failed to initialize model: %v", err)
	}

	p := tea.NewProgram(initialModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
