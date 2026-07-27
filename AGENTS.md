# AI Agent Guidelines for jj-diff

Guidelines for AI coding assistants working on this Go project. Project-specific
architecture, patterns, and domain context live in [DESIGN.md](DESIGN.md).

## Package Structure

```
jj-diff/
├── cmd/jj-diff/      # CLI entry point
├── internal/
│   ├── model/        # Core TEA model (orchestrator)
│   ├── jj/           # jj CLI integration
│   ├── diff/         # Parser and patch generator
│   ├── search/       # Search functionality
│   ├── fuzzy/        # Fuzzy matching
│   ├── components/   # UI components
│   ├── config/       # Configuration
│   └── theme/        # Catppuccin themes
└── go.mod
```

- One package, one purpose; short lowercase names with no underscores (`httputil`, not `http_util`)
- Avoid grab-bag packages (`util`, `common`, `misc`)
- `internal/` prevents external imports at the compiler level
- Group related types and their methods in one file named after the primary type (`user.go`, `user_test.go`); keep `main.go` thin

## Code Style

- Favor small, composable, single-responsibility functions and composition over inheritance
- Define interfaces where they are consumed, not where implemented, and keep them to 1-3 methods
- Accept a `context.Context` as the first argument for cancellable or I/O-bound work
- Name with MixedCaps and keep acronyms uppercase (`ServeHTTP`, `userID`, `GetHTTPClient`)
- Use the functional-options pattern for constructors with optional configuration:

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

## Error Handling

- Return errors rather than panicking outside truly unrecoverable states
- Wrap with context: `fmt.Errorf("doing something: %w", err)`
- Inspect with `errors.Is` / `errors.As`; define custom types for domain-specific errors
- Validate at system boundaries and trust internal code (parse, don't validate)

## Comments and Documentation

- Code should be self-explanatory; do not comment what the code plainly does
- Doc-comment exported symbols, describing non-obvious behavior and invariants rather than restating types
- Skip docstrings on self-explanatory private helpers

## Testing

- Prefer table-driven tests with subtests via `t.Run`
- Use the `_test` package suffix for black-box tests and place tests next to the code they cover

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -1, -1, -2},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Add(tt.a, tt.b); got != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

## TUI Testing

For interactive terminal UIs (e.g. Bubble Tea, tview), a plain subprocess pipe won't trigger real
rendering — the program detects a non-tty and falls back to non-interactive behavior. Exploratory
"does it work" testing needs a real PTY:

- Manual/exploratory: run the binary inside `tmux` (`tmux new -d -x 80 -y 24 <cmd>`), drive it with
  `tmux send-keys`, and inspect what actually rendered with `tmux capture-pane -p` (plain text) or
  `-e` (with ANSI/color codes preserved). This is the fastest way to answer "does it look right"
  without writing test code first.
- Scripted/automated: `github.com/creack/pty` to spawn the process attached to a pseudo-terminal, or
  `github.com/Netflix/go-expect` for expect-style send/wait-for-pattern interaction. For Bubble Tea
  specifically, prefer `github.com/charmbracelet/x/exp/teatest`, which drives the `tea.Model` directly
  without a real PTY and supports golden-file output comparison.

Evaluating whether it "looks right": check for correct wrapping/truncation at the target terminal
width, no overlapping or stale content between re-renders, cursor and alt-screen state restored on
every quit path (not just the happy one), and no leftover ANSI escape sequences bleeding into
piped/non-tty output.

Corner cases worth exercising deliberately:
- Terminal resize mid-session (`SIGWINCH`) and minimum supported dimensions
- Every quit path independently (`q`, `ctrl-c`, `esc`) — each must restore terminal state
- Full keyboard navigation: tab/shift-tab focus order, arrow keys, and wraparound at list boundaries
- Empty/zero-item and single-item states, not just the populated case
- Rapid repeated keypresses (double-fire on a debounced action) and paste of multi-byte/unicode input
- Piped stdin/stdout (non-tty) — the program should degrade gracefully, not hang or panic

When exploratory testing turns up a bug, translate it into a regression test before moving on rather
than just fixing it ad hoc:
- Golden-file/snapshot tests (`teatest.RequireEqualOutput`, regenerated with `-update` only when the
  render intentionally changed) lock down the exact rendered frame for a given input sequence
- Scripted PTY/expect tests assert on the specific prompt or output pattern that was missing or wrong
- Name the test after the bug's trigger condition (e.g. `TestResize_MinHeight`,
  `TestQuit_RestoresCursorOnCtrlC`) so a future regression is traceable to what broke
- Prefer parametrizing over terminal size rather than hardcoding one width/height, since layout bugs
  are usually size-dependent

## Anti-Patterns to Avoid

- Naked returns, functions over ~50 lines, and deep nesting (prefer early returns)
- Interface pollution (define interfaces only once a consumer needs them)
- Ignored errors (`_ = doThing()` is almost always wrong)
- Shared global state; pass dependencies explicitly

## Workflow

- Run `mise run ci` (tests + build) before committing; `mise run format` or `hk fix` auto-fixes lint and formatting
- Conventional commits are enforced by commitizen
- Do not stage, commit, or push without explicit instruction
