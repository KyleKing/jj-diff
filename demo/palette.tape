# The README's palette recording: the command palette finding jj's verb from a
# git word, and the legend explaining a key when it is pressed.
# `mise run demo` builds ./jj-diff and runs this.

Output docs/images/palette.gif

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
Type "demo/scene.sh palette" Enter
Sleep 6s
Show

Sleep 1500ms
Enter
Sleep 1s

# The palette takes a git word and names the jj command it runs.
Type ":"
Sleep 1500ms
Type "fixup"
Sleep 2500ms
Escape
Sleep 800ms

# The legend lists what is bound here; pressing a key explains it.
Type "?"
Sleep 2s
Type "S"
Sleep 2500ms
Type "R"
Sleep 2500ms
Escape
Sleep 800ms

Type "q"
Sleep 500ms
