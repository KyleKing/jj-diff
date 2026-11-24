# The README's split recording: one commit's hunks laid out as columns, one
# per commit the split makes, moved and described, then applied as jj split.
# `mise run demo` builds ./jj-diff and runs this.

Output docs/images/split.gif

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
Type "demo/scene.sh split" Enter
Sleep 6s
Show

# Column 1 stays in the revision; every other column is a new commit.
Sleep 2500ms

# The README hunk goes to column 2, the units.go fix to a new column 3.
Type ">"
Sleep 1500ms
Type "]f"
Sleep 600ms
Type "]f"
Sleep 800ms
Type "3"
Sleep 1500ms

# Describe the new commits inline.
Type "l"
Sleep 500ms
Type "d"
Sleep 500ms
Type "docs: the shopping list"
Sleep 600ms
Enter
Sleep 800ms
Type "l"
Sleep 500ms
Type "d"
Sleep 500ms
Type "fix: weigh flour at 125 g a cup"
Sleep 600ms
Enter
Sleep 1500ms

# Apply: one jj split per new column.
Enter
Sleep 3s

Type "q"
Sleep 500ms
