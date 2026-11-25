---
name: doneram-fix-bump
description: Finish a dependency bump that doneram opened but could not merge on its own. Use on a doneram/failing-<tool> branch (the bump breaks validation) or a doneram/review-<tool> branch (a major or a changed build provenance needs a decision), or when asked to fix, migrate, or review a doneram update PR.
---

# Finish a doneram bump

doneram's weekly run already applied the version change on this branch. Your
job is the part it cannot do: change the code so the new version works, or
explain why it should not be merged. Do not change the version back, and do
not bump anything else.

## 1. Get the context

```sh
git branch --show-current                    # doneram/failing-<tool> or doneram/review-<tool>
doneram explain <tool> --validate            # the change, where it is pinned, breaking notes, validate output
```

`explain` prints, against the base branch:

- the version change and a compare link,
- every file and line the version is pinned at,
- the release notes' breaking changes that name something this repo uses,
  and for a GitHub Action, the inputs and outputs that changed,
- a provenance warning when the new release was built or published
  differently from the old one,
- the output of the config's `validate` command.

The pull request body has the same validation output; read it when the
branch has a PR (`gh pr view` or the GitHub MCP tools).

## 2. Decide what kind of fix this is

- **Provenance warning.** Stop. Do not edit code. Report what changed (lost
  attestation, different source repository, different publisher) and
  recommend that a person verify the release before merging. A hijacked
  release looks like an ordinary bump.
- **Breaking API change.** Read the release notes or migration guide behind
  the compare link, then update each call site the notes name. Search the
  repo for every use, not just the first.
- **Configuration format change.** (a linter's or hook manager's major).
  Migrate the config file with the tool's own migration command when it has
  one; otherwise follow its upgrade guide.
- **Something else fails.** (a test, a type check). Fix the cause in the
  code. Never skip, disable, or loosen a test or lint rule to get green.

## 3. Prove it

Run the validate command `explain` printed, and repeat until it passes. Then
run `doneram explain <tool> --validate` once more and confirm it says
"passes".

## 4. Hand back

Commit on this same branch with a conventional message
(`fix: adapt to <tool> <version>`), and summarize:

- what broke and why, citing the release note line,
- what you changed,
- the final validate result,
- anything a reviewer should still check by hand.

If you could not make validation pass, say so plainly with the remaining
failure. Do not report success that the validate output does not show.
