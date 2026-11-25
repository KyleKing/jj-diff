# One still per screen, rendered once per breakpoint by `mise run demo:stills`,
# which fills in the __NAME__, __WIDTH__, __HEIGHT__, and __FONT__ placeholders.
#
# Every Screenshot follows a Wait on text only that screen draws, so a still never
# shows a half-loaded or wrong screen. A Wait that never matches fails the render.
# Keep a Sleep after each Screenshot: VHS writes the file asynchronously and can exit first.

Output demo/.work/stills-__NAME__.gif

Set Shell "bash"
Set FontFamily "__FONT__"
Set FontSize 15
Set Width __WIDTH__
Set Height __HEIGHT__
Set Padding 18
Set WaitTimeout 20s
Set Framerate 15

Env COLORTERM "truecolor"

Hide
Type "./jj-diff --help" Enter
Show
Wait+Screen /Usage:/
Sleep 300ms
Screenshot docs/images/__NAME__-help.png
Sleep 1s
