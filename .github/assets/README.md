# Demo Assets

## VHS Demo

The `demo.tape` file creates an animated demo of jj-diff using [VHS](https://github.com/charmbracelet/vhs).

### Prerequisites

Install VHS:
```bash
# macOS
brew install vhs

# Other platforms: see https://github.com/charmbracelet/vhs#installation
```

### Running the Demo

From the project root:
```bash
vhs .github/assets/demo.tape
```

This will:
1. Build jj-diff in the current directory
2. Create a temporary jj repository
3. Set up demo files with initial content
4. Make changes to demonstrate diff viewing
5. Launch jj-diff and demonstrate:
   - File list navigation
   - Diff viewing with syntax highlighting
   - Search functionality
   - Help overlay
6. Generate `.github/assets/demo.gif`
7. Clean up the temporary directory

### Output

The demo generates:
- `demo.gif` - Animated GIF suitable for README/documentation

### Customization

Edit `demo.tape` to adjust:
- **Playback speed**: `Set PlaybackSpeed 1.5` (line 47)
- **Dimensions**: `Set Width/Height` (lines 54-55)
- **Theme**: `Set Theme "Catppuccin Mocha"` (line 50)
- **Font**: `Set FontFamily "JetBrains Mono"` (line 51)

### Setup Script

The `setup-demo.sh` script creates the demo repository content:
- Initial Go files with basic structure
- Working copy changes showing additions, modifications, deletions
- Configuration file

Run standalone for testing:
```bash
# In a jj repository
bash .github/assets/setup-demo.sh
```
