package render_test

import (
	"slices"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/render"
)

// lexWait bounds how long a test waits on Start, far above what lexing a sample diff costs.
const lexWait = 10 * time.Second

func awaitLexed(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(lexWait):
		t.Fatal("Start() never closed its channel")
	}
}

func stripped(rows []string) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = ansi.Strip(row)
	}

	return out
}

func TestPeek_DrawsPlainUntilStartLandsThenMatchesRows(t *testing.T) {
	t.Parallel()

	for _, sideBySide := range []bool{false, true} {
		t.Run(map[bool]string{false: "unified", true: "side-by-side"}[sideBySide], func(t *testing.T) {
			t.Parallel()

			opts := benchOptions()
			opts.SideBySide = sideBySide
			text := goDiff(40)

			doc := render.New(text, opts)

			plain, colored := doc.Peek(0, doc.Len())
			if colored {
				t.Fatal("Peek() before Start reported colored rows")
			}

			awaitLexed(t, doc.Start())

			got, colored := doc.Peek(0, doc.Len())
			if !colored {
				t.Fatal("Peek() after Start's channel closed reported plain rows")
			}

			want := render.Lines(text, opts)
			if !slices.Equal(got, want) {
				t.Errorf("Peek() after Start differs from Lines():\n got %q\nwant %q", got, want)
			}

			if slices.Equal(plain, got) {
				t.Error("Peek() before Start drew the colored rows, want them plain")
			}

			// The swap from plain to colored must not move a line, or a kept cursor lands elsewhere.
			if !slices.Equal(stripped(plain), stripped(got)) {
				t.Errorf("plain and colored rows hold different text:\nplain   %q\ncolored %q",
					stripped(plain), stripped(got))
			}
		})
	}
}

func TestStart_WithoutSyntaxIsAlreadyDone(t *testing.T) {
	t.Parallel()

	opts := benchOptions()
	opts.Syntax = false

	for name, text := range map[string]string{"diff": sample, "empty": ""} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := render.New(text, opts)

			select {
			case <-doc.Start():
			default:
				t.Fatal("Start() without syntax returned an open channel")
			}

			if _, colored := doc.Peek(0, doc.Len()); !colored {
				t.Error("Peek() without syntax reported rows still to color")
			}
		})
	}
}

func TestStart_AgainAnswersTheSameChannel(t *testing.T) {
	t.Parallel()

	doc := render.New(sample, benchOptions())

	first, second := doc.Start(), doc.Start()
	if first != second {
		t.Error("Start() twice returned two channels, want one")
	}

	awaitLexed(t, first)
}
