# The README's conflict recording: a merge of two rounding fixes that disagree,
# each region drawn as a pane per side with the revision it came from.
# `mise run demo` builds ./jj-diff and runs this.

Output docs/images/conflicts.gif

Set Shell "bash"
Set FontSize 15
Set Width 1180
Set Height 640
Set Padding 18
Set Theme "Catppuccin Mocha"
Set Framerate 8
Set TypingSpeed 90ms

Env COLORTERM "truecolor"
Env ARAGONITE_ICONS "unicode"

Hide
Type "demo/scene.sh conflicts" Enter
Sleep 6s
Show

# Two regions in units.go, each side labeled with its change and message.
Sleep 3s

# Take side 2 for the first, step to the next, and take side 1 there.
Type "2"
Sleep 2s
Type "]x"
Sleep 2s
Type "1"
Sleep 2s

# Confirm writes the picks through jj resolve, one operation jj undo reverses.
Type "c"
Sleep 3s

Type "q"
Sleep 500ms
