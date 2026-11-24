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

Press `tab` to reach the diff, `]h` (or `n`) to reach the hunk, `v` for line-level
selection, `space` to confirm the lines, then `s` and `enter` on `@-` to squash them
there. `jj undo` takes it back. Press `?` at any point for the keymap.

## Keys

The keymap is the one jojo and second-look use, shared through aragonite's
`tui/keys`: `]`/`[` plus a letter jumps (`h` hunk, `f` file), `n`/`N` repeat the
last jump or search, `z` folds, and `\` toggles how the diff is drawn. A prefix
shows the keys that can follow it while it waits. The help overlay (`?`) and the
footer are drawn from the same table, so they list exactly what is bound.

| Key | Action | Where |
| --- | --- | --- |
| `j`/`k`, `ctrl+d`/`ctrl+u`, `ctrl+f`/`ctrl+b`, `ctrl+e`/`ctrl+y` | line, half page, page, scroll a line | everywhere |
| `gg` / `G` | top / bottom (first / last file in the list) | everywhere |
| `]h` / `[h` | next / previous hunk | diff |
| `]f` / `[f` | next / previous file | diff |
| `n` / `N` | repeat the last jump or search forward / back | everywhere |
| `/` | search files and diff content | everywhere |
| `f` | filter the file list (fuzzy) | everywhere |
| `tab` | switch between the file list and the diff | everywhere |
| `enter` | show the file's diff | file list |
| `w` | fold whitespace-only changes | diff |
| `za` `zi` `zR` `zM` | fold this / invert folds / open all / fold all | diff |
| `zz` `zt` `zb` | hunk to the middle / top / bottom | diff |
| `\s` `\n` `\r` | side by side, line numbers, word-level refinement | diff |
| `ctrl+l` | reload from jj | everywhere |
| `?` | help for the screen you are on | everywhere |
| `q` | quit (asks write / discard / stay over a pending selection) | everywhere |
| `esc` | back out one level: a waiting prefix, visual mode, the file list | everywhere |
| `ctrl+c` | quit now, writing nothing | everywhere |

Selecting and writing (`-i`, or as jj's diff editor). A verb acts on the selection,
or on the hunk under the cursor when nothing is selected:

| Key | Action | Mode |
| --- | --- | --- |
| `space` | select the hunk, or in visual mode the lines | `-i`, editor |
| `v` / `V` | select a range of lines | `-i`, editor |
| `~` | invert the selection in the file | `-i`, editor |
| `u` / `ctrl+r` | undo / redo a selection edit | `-i`, editor |
| `s` | squash into…: a picker in jj log order, `/` filters, `enter` applies | `-i` |
| `S` | split into columns, the selection (or the hunk) in a new commit's column | `-i` |
| `R` | discard from the revision | `-i` |
| `c` | confirm: squash into the `-d` destination, or write and return to jj | `-i`, editor |
| `.` | repeat the last write on the current selection | `-i` |
| `-t` `-o` `-A` `-B` `-u` `-k` | in the picker: `--into` / new commit onto, after, before / keep the destination's message / keep the emptied source | `-i` |

Every write is a single jj operation (`jj squash`, `jj split`, or `jj restore` with
jj-diff as the diff editor), so `jj undo` reverses it whole. A split into more than
two commits is one `jj split` per new commit, and the footer names the
`jj op restore` that takes the whole split back. In the diff editor, leaving
without `c` exits non-zero and jj leaves the revision as it was.

### Split columns

`S` (or `jj-diff --split -r REV`) shows one column per commit the split makes.
Column 1 is what stays in the revision, which keeps its change ID and description;
every other column is a new commit, parent-most first, so the chain reads
`2 → 3 → … → 1`. A move acts on the selection in the focused column, or on the hunk
at its cursor:

| Key | Action |
| --- | --- |
| `h` / `l` | focus the column to the left / right |
| `j`/`k`, `]h`/`[h`, `]f`/`[f`, `gg`/`G` | move the hunk cursor in the column (lines, in visual mode) |
| `space`, `v`/`V`, `~` | select the hunk, a line range, or invert the column's selection |
| `<` / `>` | move to the column to the left / right; `>` from the last column makes a new one |
| `1`–`9` | send to column N, making it when N is one past the last |
| `d` / `D` | describe the focused column inline / in `$EDITOR` |
| `-p` | toggle `--parallel`: the new commits become siblings of the revision instead of a chain |
| `u` / `ctrl+r` | undo / redo a move or a description |
| `enter` / `c` | apply: `jj split` per non-empty column; empty columns are dropped |
| `esc` / `q` | back out: visual, then the selection, then the columns, writing nothing |

Moving from 0.8: `[`/`]` became `[f`/`]f`, `p` became `N`/`[h`, `g` became `gg`,
`s`/`l`/`W` (side by side, line numbers, word diff) became `\s`/`\n`/`\r`, `z`
became `za`, `F` merged into `f`, `r` became `ctrl+l`, `d` then `a` became `s` then
`enter` (or `c` with `-d`), and the `S`/letter/`D`/`P` multi-split became `S`.

## What it does not do

- Work on a plain git repository. It shells out to `jj`, so a repo needs jj 0.9.0+
- Touch what you did not select. Every write is one jj command with jj-diff as
  its diff editor, so jj does the rewriting, only the selected lines move, and
  the working copy changes only when it is the revision you are writing to or from.
  A failed write restores the operation it started from
- Commit, rebase, or edit existing commit descriptions. It squashes, splits, or
  discards existing changes and nothing else
- Browse the repository. There is no change graph, operation log, or bookmark
  management here. That is [jj-tui](https://github.com/KyleKing/jj-tui)'s job, and
  jjui's, and both hand hunk-level work to a diff editor, which is where jj-diff fits
- Resolve merge conflicts
- Edit file contents. You choose which existing changes move, and you do not type
  new ones

## Fonts

File rows carry a filetype icon from a [Nerd Font](https://www.nerdfonts.com/), which is the
default. Set `ARAGONITE_ICONS=unicode` to use glyphs every monospace font has instead; the setting
is shared with second-look and jojo, and no column moves either way. Under `NO_COLOR` a changed
line's sign bar turns into `+` and `−`.

Full docs: [./docs](./docs)

## License

MIT
