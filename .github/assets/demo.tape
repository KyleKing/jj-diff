# VHS documentation
#
# Run with:
#  vhs .github/assets/demo.tape
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>
#   Output <path>.mp4               Create an MP4 output at the given <path>
#   Output <path>.webm              Create a WebM output at the given <path>
#
# Settings:
#   Set FontSize <number>           Set the font size of the terminal
#   Set FontFamily <string>         Set the font family of the terminal
#   Set Height <number>             Set the height of the terminal
#   Set Width <number>              Set the width of the terminal
#   Set LetterSpacing <float>       Set the font letter spacing (tracking)
#   Set LineHeight <float>          Set the font line height
#   Set Theme <string>              Set the theme of the terminal (JSON)
#   Set Padding <number>            Set the padding of the terminal
#   Set Framerate <number>          Set the framerate of the recording
#   Set PlaybackSpeed <float>       Set the playback speed of the recording
#
# Sleep:
#   Sleep <time>                    Sleep for a set amount of <time> in seconds
#
# Type:
#   Type[@<time>] "<characters>"    Type <characters> into the terminal with a
#                                   <time> delay between each character
#
# Keys:
#   Backspace[@<time>] [number]     Press the Backspace key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output

Output .github/assets/demo.gif
Set PlaybackSpeed 1.5
Set TypingSpeed 50ms

Set Theme "Catppuccin Mocha"
Set FontFamily "JetBrains Mono"

Set FontSize 14
Set Width 1400
Set Height 900

Hide
# Store current directory and build jj-diff
Type "export PROJECT_DIR=$(pwd)" Enter Sleep 500ms
Type "make build" Enter Sleep 1s

# Create and navigate to temporary directory
Type "export DEMO_DIR=$(mktemp -d /tmp/jj-diff-demo.XXXXXX)" Enter Sleep 500ms
Type "cd $DEMO_DIR" Enter Sleep 500ms

# Initialize jj repository
Type "jj git init demo-repo" Enter Sleep 500ms
Type "cd demo-repo" Enter Sleep 500ms

# Run setup script to create demo files
Type "bash $PROJECT_DIR/.github/assets/setup-demo.sh" Enter Sleep 1s

Type "clear" Enter
Show

# Show initial state
Type "# Welcome to jj-diff demo! Let's view our changes..." Enter Sleep 2s
Type "jj status" Enter Sleep 2s

# Launch jj-diff in browse mode
Type "# Launch jj-diff to browse the changes" Enter Sleep 1s
Type "$PROJECT_DIR/jj-diff" Enter Sleep 3s

# Navigate through files in file list
Down Sleep 500ms
Down Sleep 500ms
Up Sleep 500ms

# Switch to diff view with Tab
Tab Sleep 1s

# Scroll through diff content
Down Sleep 300ms
Down Sleep 300ms
Down Sleep 300ms
Down Sleep 300ms

# Navigate to next hunk
Type "n" Sleep 1s
Type "n" Sleep 1s

# Switch back to file list
Tab Sleep 500ms
Down Sleep 500ms

# Switch to diff view for next file
Tab Sleep 1s
Down Sleep 300ms
Down Sleep 300ms

# Try search functionality
Type "/" Sleep 500ms
Type "Email" Sleep 1s
Enter Sleep 1s

# Show help overlay
Type "?" Sleep 3s
Type "?" Sleep 500ms

# Show final state
Sleep 1s

# Quit
Type "q" Sleep 1s

Hide
# Cleanup
Type "cd /tmp" Enter Sleep 500ms
Type "rm -rf $DEMO_DIR" Enter Sleep 500ms
Show
