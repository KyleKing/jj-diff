# Contributing to jj-diff

## Setup

Prerequisites: Go (see `go.mod`), [mise](https://mise.jdx.dev/), [hk](https://hk.jdx.dev/)

```bash
mise install
hk install --mise
mise run ci
```

## Tasks

| Command | Description |
|---------|-------------|
| `mise run bench` | Run benchmarks |
| `mise run build` | Build binary |
| `mise run ci` | Full CI check (tests + build) |
| `mise run clean` | Clean build artifacts |
| `mise run demo` | Generate VHS demo recordings |
| `mise run format` | Auto-fix lint and formatting |
| `mise run hooks` | Run git hooks |
| `mise run lint` | Run linter |
| `mise run test` | Run tests with coverage |
| `mise tasks` | List all available tasks |

## Code Guidelines

Follow [AGENTS.md](AGENTS.md) for code organization, testing patterns, and error handling.

Linting is configured in `.golangci.toml` with 40+ rules. Run `mise run format` to auto-fix.

## Git Workflow

Conventional commits enforced via [commitizen](https://commitizen-tools.github.io/commitizen/):

```
<type>(<scope>): <subject>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Git hooks run automatically via hk on commit and push.


## Releases

Automated via goreleaser on tag push:

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions builds binaries for Linux, macOS, Windows, and FreeBSD (amd64/arm64).

After release, update the Homebrew formula:

```bash
mise run brew:sha
```

Then update `Formula/jj-diff.rb` with the new version and SHA256 values.


## Troubleshooting

```bash
mise install --force   # Reinstall tools
hk install --mise --force  # Reinstall hooks
go test -v -run TestName ./package  # Debug specific test
```
