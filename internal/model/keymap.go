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
	actionApply   = "apply"
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
	keys.ActionShell, keys.ActionInspect, keys.ActionYank, keys.ActionLabelJump,
	keys.ActionJumpBack, keys.ActionJumpForward, keys.ActionOpenFile, keys.ActionDefinition,
	keys.ActionReferences, keys.ActionCalls,
	keys.ActionSideOld, keys.ActionSideNew, keys.ActionRead, keys.ActionOnlyUnread, keys.ActionRounds,
	keys.ActionOutline, keys.ActionCosmetic, keys.ActionMoreContext, keys.ActionLessContext,
	keys.ActionViewWrap, keys.ActionViewGrammar, keys.ActionViewImages, keys.ActionViewCycle,
	// Visual mode only means something where lines can be selected, so jj-diff binds it itself.
	keys.ActionVisual,
}

// supportedObjects are the ] and [ objects a single revision's diff has, and the conflict viewer's.
func supportedObjects(mode OperatingMode) []string {
	if mode == ModeResolve {
		return []string{keys.ObjConflict, keys.ObjFile}
	}

	return []string{keys.ObjHunk, keys.ObjFile}
}

// columnsUnsupported is the shared vocabulary the split columns do not act on: they have one pane,
// no search, and draw hunks without folds or view options. Each is taken off the Split screen so its
// legend lists only what works there.
//
//nolint:gochecknoglobals // a fixed table, read only.
var columnsUnsupported = []string{
	keys.ActionSwitchPane, keys.ActionRepeat, keys.ActionSearch, keys.ActionFilter, keys.ActionWhitespace,
	keys.ActionScrollDown, keys.ActionScrollUp, keys.ActionReload,
	keys.ActionFoldToggle, keys.ActionFoldInvert, keys.ActionFoldOpenAll, keys.ActionFoldAll,
	keys.ActionFrameMiddle, keys.ActionFrameTop, keys.ActionFrameBottom,
	keys.ActionViewSideBySide, keys.ActionViewNumbers, keys.ActionViewRefine,
}

// offColumns takes the bindings columnsUnsupported names off the Split screen, leaving them
// everywhere else they applied.
func offColumns(bindings []keys.Binding) []keys.Binding {
	for i, b := range bindings {
		if !slices.Contains(columnsUnsupported, b.Action) {
			continue
		}

		if b.Contexts == 0 {
			b.Contexts = ^keys.Context(0)
		}

		bindings[i].Contexts = b.Contexts &^ keys.Split
	}

	return bindings
}

// resolveUnsupported is what the conflict viewer leaves out beyond unsupported: it has no selection,
// no folds, and no second pane.
//
//nolint:gochecknoglobals // a fixed table, read only.
var resolveUnsupported = []string{
	keys.ActionRepeat, keys.ActionSwitchPane, keys.ActionFilter, keys.ActionWhitespace,
	keys.ActionFoldToggle, keys.ActionFoldInvert, keys.ActionFoldOpenAll, keys.ActionFoldAll,
	keys.ActionFrameMiddle, keys.ActionFrameTop, keys.ActionFrameBottom, keys.ActionViewNumbers,
	keys.ActionViewRefine, keys.ActionSearch,
}

// pickerFlags are the squash flags the destination picker offers. -m and -i are left out: the
// selection is the interactive part, and a message prompt is not built yet.
//
//nolint:gochecknoglobals // a fixed table, read only.
var pickerFlags = []string{"t", "o", "A", "B", "u", "k"}

// sharedBindings is the slice of aragonite's table jj-diff uses in mode.
func sharedBindings(mode OperatingMode) []keys.Binding {
	drop := slices.Clone(unsupported)

	for _, o := range keys.Objects() {
		if !slices.Contains(supportedObjects(mode), o.Noun) {
			drop = append(drop, keys.NextAction(o.Noun), keys.PrevAction(o.Noun))
		}
	}

	if mode == ModeBrowse {
		drop = append(drop, keys.ActionUndo, keys.ActionRedo, keys.ActionRepeat)
	}

	if mode == ModeDiffEditor {
		drop = append(drop, keys.ActionRepeat)
	}

	if mode == ModeResolve {
		drop = append(drop, resolveUnsupported...)

		return slices.Concat(keys.Without(slices.Concat(keys.Global(), keys.DiffView()), drop...),
			keys.ConflictPicks())
	}

	shared := keys.Without(slices.Concat(keys.Global(), keys.DiffView()), drop...)

	if mode == ModeInteractive {
		// The columns' -o, -A, and -B place the result for jojo's picker, which jj-diff has not got.
		shared = append(offColumns(shared), splitFlag())
		shared = append(shared, keys.SplitColumns()...)

		for _, f := range keys.FlagBindings(keys.VerbSquash) {
			if slices.Contains(pickerFlags, f.Keys[0][len("- "):]) {
				shared = append(shared, f)
			}
		}
	}

	return shared
}

// splitFlag is -p, the one split flag the columns offer, on the columns alone: the squash picker's
// own flags hold that prefix there.
func splitFlag() keys.Binding {
	for _, f := range keys.FlagBindings(keys.VerbSplit) {
		if f.Action == keys.FlagAction(keys.VerbSplit, "p") {
			f.Contexts = keys.Split

			return f
		}
	}

	panic("aragonite has no split -p flag")
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

	if mode == ModeResolve {
		return []keys.Binding{
			{
				Action: actionConfirm, Keys: []string{"c"}, What: "confirm: write the picks (one jj operation)",
				Short: actionConfirm, Group: groupWrite, Contexts: keys.Conflict,
			},
			{
				Action: actionApply, Keys: []string{keyEnter}, What: "write the picks", Short: "write",
				Group: groupWrite, Contexts: keys.Conflict,
			},
		}
	}

	out = append(out,
		keys.Binding{
			Action: actionSelect, Keys: []string{"space"}, What: "select the hunk, or in visual the lines",
			Short: "select", Group: groupSelect, Contexts: keys.Select | keys.Split,
		},
		keys.Binding{
			Action: keys.ActionVisual, Keys: []string{"v", "V"}, What: "select a range of lines",
			Short: "lines", Group: groupSelect, Contexts: keys.Select | keys.Split,
		},
		keys.Binding{
			Action: actionInvert, Keys: []string{"~"}, What: "invert the selection in the file",
			Short: "invert", Group: groupSelect, Contexts: keys.Select | keys.Split,
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
			Action: actionConfirm, Keys: []string{"c"}, What: "confirm: squash into -d, or apply the split",
			Short: "confirm", Group: groupWrite, Contexts: keys.Select | keys.Split,
		},
		keys.Binding{
			Action: actionApply, Keys: []string{keyEnter}, What: "apply the split", Short: "apply",
			Group: groupWrite, Contexts: keys.Split,
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

// hint is action's footer hint worded as word (its own Short when empty), nil when unbound, and
// drawn Off with the gate's reason where the action can do nothing.
func (m *Model) hint(action, word string) []keyhint.Hint {
	hints := m.keymap.HintsAt(m.gate, action)
	if len(hints) == 0 {
		return nil
	}

	if word != "" {
		hints[0].What = word
	}

	return hints
}

// labeled is a hint keyed by every sequence of the action, for a range of digits ("1–9").
func (m *Model) labeled(action, word string) []keyhint.Hint {
	b, ok := m.keymap.Binding(action)
	if !ok || len(b.Keys) == 0 {
		return nil
	}

	h := b.Hint()
	h.Key = b.Labels()

	if a := m.gate(action); !a.OK {
		h.Off, h.Why = true, a.Why
	}

	if word != "" {
		h.What = word
	}

	return []keyhint.Hint{h}
}

// pair is one hint for two actions that go together, keyed "h/l".
func (m *Model) pair(a, b, word string) []keyhint.Hint {
	x, _ := m.keymap.Binding(a)
	y, _ := m.keymap.Binding(b)

	return []keyhint.Hint{{Key: x.Label() + "/" + y.Label(), What: word}}
}

// columnsHints is the split columns' footer.
func (m *Model) columnsHints() []keyhint.Hint {
	if m.cols.visual {
		return slices.Concat(m.pair(keys.ActionLineDown, keys.ActionLineUp, "extend"),
			m.hint(actionSelect, "confirm"), m.hint(keys.ActionBack, "cancel"))
	}

	return slices.Concat(m.pair(keys.ActionColumnLeft, keys.ActionColumnRight, "column"),
		m.hint(actionSelect, ""), m.pair(keys.ActionMoveLeft, keys.ActionMoveRight, "move"),
		m.labeled(keys.ActionSendTo, ""), m.hint(actionApply, ""), m.hint(keys.ActionDescribe, ""),
		m.hint(keys.FlagAction(keys.VerbSplit, "p"), "parallel"), m.hint(keys.ActionHelp, ""))
}

// conflictHints is the conflict viewer's footer.
func (m *Model) conflictHints() []keyhint.Hint {
	return slices.Concat(m.labeled(keys.ActionTakeSide, ""), m.hint(keys.ActionTakeBase, ""),
		m.hint(keys.ActionTakeAll, ""), m.hint(actionConfirm, ""), m.hint(keys.NextAction(keys.ObjConflict), ""),
		m.hint(keys.ActionUndo, ""), m.hint(keys.ActionHelp, ""))
}

// footerHints is what the footer offers on a screen, most useful first: Fit sheds from the end,
// and the last one (help, or the way out of a mode) is pinned.
func (m *Model) footerHints() []keyhint.Hint {
	lines := func(word string) []keyhint.Hint { return m.pair(keys.ActionLineDown, keys.ActionLineUp, word) }
	hint := m.hint

	var hints []keyhint.Hint

	switch ctx := m.context(); {
	case ctx == keys.Conflict:
		hints = m.conflictHints()
	case ctx == keys.Split:
		hints = m.columnsHints()
	case m.isVisualMode:
		hints = slices.Concat(lines("extend"), hint(actionSelect, "confirm"), hint(keys.ActionBack, "cancel"))
	case ctx == keys.Picker:
		hints = slices.Concat(lines("move"),
			hint(actionPick, ""), []keyhint.Hint{{Key: "-", What: "flags"}}, hint(keys.ActionSearch, "filter"),
			hint(keys.ActionBack, "cancel"))
	case ctx == keys.List:
		hints = slices.Concat(lines("nav"),
			hint(actionOpen, ""), hint(keys.ActionFilter, ""), hint(keys.ActionSearch, ""), hint(keys.ActionHelp, ""))
	case m.mode == ModeDiffEditor:
		hints = slices.Concat(hint(actionSelect, "keep/drop"), hint(keys.ActionVisual, ""),
			hint(actionConfirm, ""), hint(keys.ActionUndo, ""), hint(keys.NextAction(keys.ObjHunk), ""),
			hint(keys.ActionHelp, ""))
	case m.mode == ModeInteractive:
		hints = slices.Concat(hint(actionSelect, ""), hint(keys.ActionVisual, ""), hint(actionSquash, ""),
			hint(actionSplit, ""), hint(actionRestore, ""), hint(keys.ActionUndo, ""), hint(keys.ActionHelp, ""))
	default:
		hints = slices.Concat(lines("scroll"),
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
	case m.mode == ModeResolve:
		return keys.Conflict
	case m.cols != nil:
		return keys.Split
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
