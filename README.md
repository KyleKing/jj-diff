# jj-diff

![demo](.github/assets/demo.gif)

Read a Jujutsu diff in the terminal and move parts of it around: pick hunks or
individual lines and send them to another revision, or split one commit into
several focused ones. Drops in as jj's diff editor for `jj split`, `jj diffedit`,
`jj amend -i`, and `jj squash -i`.

## Install

```bash
# Homebrew
brew install --cask kyleking/tap/jj-diff
# Go
go install github.com/kyleking/jj-diff/cmd/jj-diff@latest
# from source
mise install && hk install --mise && mise run build
```

## Quick start

Move a few lines from the working copy into the previous commit:

```bash
jj-diff --interactive
```

Press `d` and pick `@-` as the destination, `n` to reach the hunk, `v` for
line-level selection, `space` to confirm the lines, and `a` to apply. Press `?`
at any point for the keymap.

## What it does not do

- Work on a plain git repository. It shells out to `jj`, so a repo needs jj 0.9.0+
- Read, write, or move your working copy. Every edit lands through a scratch
  workspace, so an abandoned run cannot cost you unselected changes
- Commit, rebase, or edit commit descriptions. It moves existing changes between
  revisions and nothing else
- Resolve merge conflicts
- Edit file contents. You choose which existing changes move, and you do not type
  new ones

Full docs: [./docs](./docs)

## License

MIT
