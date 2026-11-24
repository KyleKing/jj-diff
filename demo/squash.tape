# The README's squash recording: a working copy that mixes a fix with new
# work, and the fix's one hunk moved into the commit it belongs to.
# `mise run demo` builds ./jj-diff and runs this. demo/scene.sh builds a
# throwaway jj repository from invented content, with a home of its own.

Output docs/images/squash.gif

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
Type "demo/scene.sh squash" Enter
Sleep 6s
Show

# Three files changed: one line in units.go belongs to the grams commit.
Sleep 2s
Type "j"
Sleep 600ms
Type "j"
Sleep 800ms
Enter
Sleep 1500ms

# Select its hunk, then squash: the picker lists the targets in jj log order.
Space
Sleep 1200ms
Type "s"
Sleep 2s
Enter
Sleep 3s

# The footer names the jj command it ran, and jj undo takes it back.
Sleep 1s
Type "q"
Sleep 500ms
