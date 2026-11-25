//nolint:testpackage // white-box: these tests set the model's icon resolution and read its legend.
package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/icons"
)

// iconCases are the resolutions the legend's icons row is pinned with: each source, and auto both
// recognizing a terminal and not.
//
//nolint:gochecknoglobals // a fixture
var iconCases = []struct {
	name string
	why  icons.Resolution
}{
	{"wezterm", icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "WezTerm"}},
	{"unknown", icons.Resolution{Mode: icons.Unicode, Source: icons.FromAuto}},
	{"env", icons.Resolution{Mode: icons.None, Source: icons.FromEnv, Detail: "off"}},
	{"setting", icons.Resolution{Mode: icons.Unicode, Source: icons.FromSetting, Detail: "plain"}},
}

// The legend says in the footer's row which icon set the file list draws, why, and how to change it,
// and the footer comes back once it closes.
func TestHelp_NamesTheIconSet(t *testing.T) {
	t.Parallel()

	for _, c := range iconCases {
		for _, size := range frameSizes {
			name := fmt.Sprintf("icons_%s_%dx%d", c.name, size.w, size.h)

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
				m.width, m.height, m.icons = size.w, size.h, c.why

				m = Press(t, m, "?")
				frame := ansi.Strip(m.render())
				lines := strings.Split(frame, "\n")

				if last := lines[len(lines)-1]; !strings.HasPrefix(last, "icons  "+c.why.Mode.String()) {
					t.Errorf("the legend's last row is %q, want the icon set", last)
				}

				frameGolden(t, filepath.Join("testdata", name+".golden"), frame+"\n", size.w)

				m = Press(t, m, "?")
				if strings.Contains(ansi.Strip(m.render()), "icons  ") {
					t.Error("the icons row stayed once the legend closed")
				}
			})
		}
	}
}

// The icons row is exactly the width at any width: the change is said in the longest wording that
// fits and cut after that, and the set itself is cut last.
func TestIconsLine_FitsEveryWidth(t *testing.T) {
	t.Parallel()

	for _, c := range iconCases {
		for _, width := range []int{12, 30, 52, 80, 100, 160} {
			t.Run(fmt.Sprintf("%s/%d", c.name, width), func(t *testing.T) {
				t.Parallel()

				m := NewTestModel(t, ModeBrowse)
				m.width, m.icons = width, c.why

				line := ansi.Strip(m.iconsLine())
				if w := lipgloss.Width(line); w != width {
					t.Errorf("%q is %d wide, want %d", line, w, width)
				}

				if !strings.HasPrefix(line, "icons  ") {
					t.Errorf("%q does not start with the label", line)
				}
			})
		}
	}
}
