# jj-diff

![demo](.github/assets/demo.gif)

Read a Jujutsu diff in the terminal and move parts of it around: pick hunks or
individual lines and send them to another revision, or split one commit into
several focused ones. Drops in as jj's diff editor for `jj split`, `jj diffedit`,
`jj amend -i`, and `jj squash -i`, so it works the same whether you run it from a
plain `jj` command line or from a TUI that shells out to one
([jj-tui](https://github.com/KyleKing/jj-tui), [jjui](https://github.com/idursun/jjui),
[lazyjj](https://github.com/Cretezy/lazyjj)).

## Alternatives

jj-diff is narrow on purpose: hunk and line moves between revisions, and splitting.
For adjacent problems, one of these may fit better:

| Tool | Notes |
| --- | --- |
| [crecord](https://www.mercurial-scm.org/wiki/CrecordExtension) | "allows you to interactively choose among the changes you have made, and commit only those changes you select" (Mercurial) |
| [Delta](https://github.com/dandavison/delta) | "A syntax-highlighting pager for git, diff, grep, rg --json, and blame output" |
| [Difftastic](https://github.com/Wilfred/difftastic) | "a structural diff that understands syntax" |
| [git add -p](https://git-scm.com/docs/git-add) | "interactively choose hunks of patch between the index and the work tree and add them to the index" (built into Git) |
| [gitui](https://github.com/gitui-org/gitui) | "Blazing fast terminal-ui for git written in rust" |
| [jj's built-in diff editor (scm-record)](https://github.com/arxanas/scm-record) | "a UI component to interactively select changes to include in a commit. It's meant to be embedded in source control tooling" |
| [jjui](https://github.com/idursun/jjui) | "A TUI designed for interacting with the Jujutsu version control system" |
| [KDiff3](https://github.com/KDE/kdiff3) | "Utility for comparing and merging files and directories" |
| [lazygit](https://github.com/jesseduffield/lazygit) | "simple terminal UI for git commands" |
| [lazyjj](https://github.com/Cretezy/lazyjj) | "TUI for Jujutsu/jj. Built in Rust with Ratatui. Interacts with jj CLI" |
| [Magit](https://github.com/magit/magit) | "A Git Porcelain inside Emacs" |
| [Meld](https://gitlab.gnome.org/GNOME/meld) | "Meld is a visual diff and merge tool targeted at developers" |
| [tig](https://github.com/jonas/tig) | "Text-mode interface for git" |
| [vimdiff](https://git-scm.com/docs/git-difftool) | Runs any diff as a Vim or Neovim split through `git difftool`, built into Git |
| _[awesome-jj](https://github.com/Necior/awesome-jj)_ | "A curated list of awesome Jujutsu things" |

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
- Overwrite your working copy. Every patch is built in a scratch workspace, so an
  abandoned run cannot cost you unselected changes. Splitting to a new commit is
  the one step that touches your working copy, and it only moves the parent
  pointer, the same way `jj split` does
- Commit, rebase, or edit commit descriptions. It moves existing changes between
  revisions and nothing else
- Browse the repository. There is no change graph, operation log, or bookmark
  management here. That is [jj-tui](https://github.com/KyleKing/jj-tui)'s job, and
  jjui's, and both hand hunk-level work to a diff editor, which is where jj-diff fits
- Resolve merge conflicts
- Edit file contents. You choose which existing changes move, and you do not type
  new ones

Full docs: [./docs](./docs)

## License

MIT
