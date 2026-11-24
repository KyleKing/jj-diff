package model

import (
	"slices"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
)

// Actions jj-diff adds to the shared vocabulary. The shared ones keep aragonite's names, so a rebind
// written for jojo or second-look means the same thing here.
const (
	actionSelect  = "select"
	actionInvert  = "invert"
	actionSquash  = "squash"
	actionSplit   = "split"
	actionRestore = "restore"
	actionConfirm = "confirm"
	actionOpen    = "open"
	actionPick    = "pick"
)

// Legend groups jj-diff files its own verbs under.
const (
	groupSelect = "Select"
	groupWrite  = "Write"
	groupPicker = "Destination"
)

// unsupported is the shared vocabulary jj-diff has no concept for yet. Each is left out of the
// keymap rather than drawn dead, and TestKeymap_AgreesWithShared keeps their keys free.
//
//nolint:gochecknoglobals // a fixed table, read only.
var unsupported = []string{
	keys.ActionPalette, keys.ActionShell, keys.ActionInspect, keys.ActionYank, keys.ActionLabelJump,
	keys.ActionJumpBack, keys.ActionJumpForward, keys.ActionOpenFile, keys.ActionDefinition,
	keys.ActionReferences, keys.ActionCalls, keys.ActionMessages,
	keys.ActionSideOld, keys.ActionSideNew, keys.ActionRead, keys.ActionOnlyUnread, keys.ActionRounds,
	keys.ActionOutline, keys.ActionCosmetic, keys.ActionMoreContext, keys.ActionLessContext,
	keys.ActionViewWrap, keys.ActionViewGrammar, keys.ActionViewImages, keys.ActionViewCycle,
	// Visual mode only means something where lines can be selected, so jj-diff binds it itself.
	keys.ActionVisual,
}

// supportedObjects are the ] and [ objects a single revision's diff has.
//
//nolint:gochecknoglobals // a fixed table, read only.
var supportedObjects = []string{keys.ObjHunk, keys.ObjFile}

// pickerFlags are the squash flags the destination picker offers. -m and -i are left out: the
// selection is the interactive part, and a message prompt is not built yet.
//
//nolint:gochecknoglobals // a fixed table, read only.
var pickerFlags = []string{"t", "o", "A", "B", "u", "k"}

// sharedBindings is the slice of aragonite's table jj-diff uses in mode.
func sharedBindings(mode OperatingMode) []keys.Binding {
	drop := slices.Clone(unsupported)

	for _, o := range keys.Objects() {
		if !slices.Contains(supportedObjects, o.Noun) {
			drop = append(drop, keys.NextAction(o.Noun), keys.PrevAction(o.Noun))
		}
	}

	if mode == ModeBrowse {
		drop = append(drop, keys.ActionUndo, keys.ActionRedo, keys.ActionRepeat)
	}

	if mode == ModeDiffEditor {
		drop = append(drop, keys.ActionRepeat)
	}

	shared := keys.Without(slices.Concat(keys.Global(), keys.DiffView()), drop...)

	if mode == ModeInteractive {
		for _, f := range keys.FlagBindings(keys.VerbSquash) {
			if slices.Contains(pickerFlags, f.Keys[0][len("- "):]) {
				shared = append(shared, f)
			}
		}
	}

	return shared
}

// toolBindings is what jj-diff adds in mode: the verbs that select and write.
func toolBindings(mode OperatingMode) []keys.Binding {
	out := []keys.Binding{
		{
			Action: actionOpen, Keys: []string{keyEnter}, What: "show the file's diff", Short: "diff",
			Group: keys.GroupMove, Contexts: keys.List,
		},
	}

	if mode == ModeBrowse {
		return out
	}

	out = append(out,
		keys.Binding{
			Action: actionSelect, Keys: []string{"space"}, What: "select the hunk, or in visual the lines",
			Short: "select", Group: groupSelect, Contexts: keys.Select,
		},
		keys.Binding{
			Action: keys.ActionVisual, Keys: []string{"v", "V"}, What: "select a range of lines",
			Short: "lines", Group: groupSelect, Contexts: keys.Select,
		},
		keys.Binding{
			Action: actionInvert, Keys: []string{"~"}, What: "invert the selection in the file",
			Short: "invert", Group: groupSelect, Contexts: keys.Select,
		},
	)

	if mode == ModeDiffEditor {
		return append(out, keys.Binding{
			Action: actionConfirm, Keys: []string{"c"}, What: "confirm: write the selection and return to jj",
			Short: "confirm", Group: groupWrite, Contexts: keys.Select,
		})
	}

	return append(out,
		keys.Binding{
			Action: actionSquash, Keys: []string{"s"}, What: "squash the selection into…", Short: "squash",
			Group: groupWrite, Contexts: keys.Select,
		},
		keys.Binding{
			Action: actionSplit, Keys: []string{"S"}, What: "split the selection out to a new commit",
			Short: "split", Group: groupWrite, Contexts: keys.Select,
		},
		keys.Binding{
			Action: actionRestore, Keys: []string{"R"}, What: "discard the selection from the revision",
			Short: "discard", Group: groupWrite, Contexts: keys.Select,
		},
		keys.Binding{
			Action: actionConfirm, Keys: []string{"c"}, What: "confirm: squash into the -d destination",
			Short: "confirm", Group: groupWrite, Contexts: keys.Select,
		},
		keys.Binding{
			Action: actionPick, Keys: []string{keyEnter}, What: "apply into the highlighted revision", Short: "apply",
			Group: groupPicker, Contexts: keys.Picker,
		},
	)
}

// newKeymap merges the shared and tool bindings for mode. A collision is a programming error that
// TestKeymap_AgreesWithShared catches, so it panics rather than limping on with a keymap that
// would silently shadow one action with another.
func newKeymap(mode OperatingMode) *keys.Keymap {
	km, err := keys.Merge(sharedBindings(mode), toolBindings(mode), nil)
	if err != nil {
		panic(err)
	}

	return km
}

// footerActions is what the footer offers on a screen, most useful first: Fit sheds from the end,
// and the last one (help, or the way out of a mode) is pinned.
func (m *Model) footerHints() []keyhint.Hint {
	km := m.keymap
	down, _ := km.Binding(keys.ActionLineDown)
	up, _ := km.Binding(keys.ActionLineUp)
	lines := func(word string) keyhint.Hint { return keyhint.Hint{Key: down.Label() + "/" + up.Label(), What: word} }
	hint := func(action, word string) []keyhint.Hint {
		h, ok := km.Hint(action)
		if !ok {
			return nil
		}

		if word != "" {
			h.What = word
		}

		return []keyhint.Hint{h}
	}

	var hints []keyhint.Hint

	switch ctx := m.context(); {
	case m.isVisualMode:
		hints = slices.Concat([]keyhint.Hint{lines("extend")},
			hint(actionSelect, "confirm"), hint(keys.ActionBack, "cancel"))
	case ctx == keys.Picker:
		hints = slices.Concat([]keyhint.Hint{lines("move")},
			hint(actionPick, ""), []keyhint.Hint{{Key: "-", What: "flags"}}, hint(keys.ActionSearch, "filter"),
			hint(keys.ActionBack, "cancel"))
	case ctx == keys.List:
		hints = slices.Concat([]keyhint.Hint{lines("nav")},
			hint(actionOpen, ""), hint(keys.ActionFilter, ""), hint(keys.ActionSearch, ""), hint(keys.ActionHelp, ""))
	case m.mode == ModeDiffEditor:
		hints = slices.Concat(hint(actionSelect, "keep/drop"), hint(keys.ActionVisual, ""),
			hint(actionConfirm, ""), hint(keys.ActionUndo, ""), hint(keys.NextAction(keys.ObjHunk), ""),
			hint(keys.ActionHelp, ""))
	case m.mode == ModeInteractive:
		hints = slices.Concat(hint(actionSelect, ""), hint(keys.ActionVisual, ""), hint(actionSquash, ""),
			hint(actionSplit, ""), hint(actionRestore, ""), hint(keys.ActionUndo, ""), hint(keys.ActionHelp, ""))
	default:
		hints = slices.Concat([]keyhint.Hint{lines("scroll")},
			hint(keys.NextAction(keys.ObjHunk), ""), hint(keys.NextAction(keys.ObjFile), ""),
			hint(keys.ActionWhitespace, ""), hint(keys.ActionViewSideBySide, ""), hint(keys.ActionHelp, ""))
	}

	for i := range hints {
		hints[i].Priority = len(hints) - i
	}

	if len(hints) > 0 {
		hints[len(hints)-1].Pin = true
	}

	return hints
}

// context is the screen the next key lands on, which decides what it means.
func (m *Model) context() keys.Context {
	switch {
	case m.picker.IsVisible():
		return keys.Picker
	case m.focusedPanel == PanelFileList:
		return keys.List
	case m.mode == ModeBrowse:
		return keys.View
	default:
		return keys.Select
	}
}
