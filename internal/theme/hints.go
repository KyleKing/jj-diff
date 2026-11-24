package theme

import "github.com/kyleking/aragonite/tui/keyhint"

// HintGap parts one hint from the next, and the footer's facts from its hints, as second-look's do.
const HintGap = "   "

// Hints is the face every row of key hints is drawn in, the footer's and an overlay's alike: bare
// keys in the accent and what they do muted, the shape second-look's and jojo's footers take.
func Hints() keyhint.Styles {
	return keyhint.Styles{Key: Skin.Accent, Text: Skin.Muted, Bare: true, Gap: HintGap}
}

// Legend is the face of a key listing meant to be read, such as the help overlay: the keys in the
// accent and what they do at body brightness.
func Legend() keyhint.Styles {
	return keyhint.Styles{Key: Skin.Accent, Text: Skin.Body, Head: Skin.Title}
}
