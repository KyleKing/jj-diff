---
name: copier-update-conflicts
description: Resolve the conflicts a copier template update leaves behind (.rej files and conflict markers), and flag local changes worth backporting to the template. Use after `copier update` or on a doneram update branch that moved .copier-answers.yml, or when asked to resolve template update conflicts.
---

# Resolve a copier template update

A `copier update` re-renders the template at its new release and merges it
with this project's edits. Where both changed the same lines it leaves
either inline conflict markers or a `<file>.rej` beside the file. Each one is
a question: should this project take the template's version, keep its own,
or combine them?

## 1. Read the project's own rules

Check `AGENTS.md` (or `CLAUDE.md`) for a section on template updates first. A
template often documents files that always conflict, values copier resets
(a version reset to `0.0.0`), and which side wins; those rules override the
defaults below.

## 2. Find every conflict

```sh
grep -rn '^<<<<<<< \|^>>>>>>> ' --exclude-dir=.git .   # inline markers
find . -name '*.rej' -not -path './.git/*'              # rejected hunks
git diff "$(git merge-base HEAD origin/HEAD)" -- .copier-answers.yml   # which release, which answers
```

Read the template's release notes or its diff between the old and new
`_commit` (the `_src_path` repository) so you know what each template
change was for.

## 3. Resolve each one

For every hunk, decide:

- **Take the template's version.** The template fixed or improved
  something, and the local version is only older. This is the default for
  files the project never meant to customize (CI workflows, shared lint and
  tool config).
- **Keep the local version.** The project changed it on purpose for reasons
  specific to it. Keep it, and make sure the template's intent (a new
  setting, a security fix) is not lost: carry that part over by hand.
- **Combine.** Both sides added something different. Keep both.

Apply each `.rej` hunk by hand (it is a unified diff against the file beside
it), then delete the `.rej`. Remove every conflict marker. Do not delete a
`.rej` without deciding what to do with its hunk.

## 4. Flag backports

A local change that any project generated from this template would want is
a template bug: the next update will conflict on it again in every project.
When you keep a local version because it is better in general, not because
of this project, list it as a backport candidate with the file, the change,
and why it generalizes. Do not edit the template repository from here.

## 5. Prove it

Run the project's validation (`validate` in `.doneram.pkl`, or its CI
command, for example `mise run ci`) and make sure it passes. Confirm no
markers or `.rej` files remain:

```sh
! grep -rln '^<<<<<<< \|^>>>>>>> ' --exclude-dir=.git . && ! find . -name '*.rej' -not -path './.git/*' | grep -q .
```

## 6. Hand back

Commit on the same branch (`chore: resolve <template> <version> update`)
and summarize each resolved conflict (took template, kept local, combined),
the backport candidates, and the validation result.
