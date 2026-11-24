package jj

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// ApplySelection runs this program's half of a write, run by jj as the diff editor with the two trees
// it materialized. It makes right the left tree with the unapply patch taken back out and then the
// apply patch put in, either patch path being empty to skip it. Whatever right holds is what jj
// records, so right ends up exactly the selection whatever diff jj started it as.
//
// The two trees hold only the files that differ, so replacing right wholesale touches nothing the
// command was not already rewriting.
//
// For a split chain both patches are given, and a file an earlier group took whole is one the two
// trees no longer hold, since jj sees no change in it. Such a file is dropped from both patches when
// they agree on it, which is what this group not touching it means.
func ApplySelection(left, right, unapplyPatch, applyPatch string) error {
	if unapplyPatch != "" && applyPatch != "" {
		if err := dropUntouched(left, right, unapplyPatch, applyPatch); err != nil {
			return err
		}
	}

	if err := replaceTree(left, right); err != nil {
		return fmt.Errorf("resetting %s to %s: %w", right, left, err)
	}

	if unapplyPatch != "" {
		if err := gitApply(right, unapplyPatch, true); err != nil {
			return err
		}
	}

	if applyPatch != "" {
		if err := gitApply(right, applyPatch, false); err != nil {
			return err
		}
	}

	return nil
}

// replaceTree empties dst and copies src into it, keeping file modes and symlinks.
func replaceTree(src, dst string) error {
	entries, err := os.ReadDir(dst)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dst, err)
	}

	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return fmt.Errorf("clearing %s: %w", dst, err)
		}
	}

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, relErr := filepath.Rel(src, path)
		if relErr != nil || rel == "." {
			return relErr //nolint:wrapcheck // wrapped once, where WalkDir returns it
		}

		return copyEntry(path, filepath.Join(dst, rel), d)
	})
	if err != nil {
		return fmt.Errorf("copying %s: %w", src, err)
	}

	return nil
}

func copyEntry(from, to string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("reading %s: %w", from, err)
	}

	switch {
	case d.IsDir():
		if err := os.MkdirAll(to, info.Mode().Perm()); err != nil {
			return fmt.Errorf("creating %s: %w", to, err)
		}

		return nil
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(from)
		if err != nil {
			return fmt.Errorf("reading link %s: %w", from, err)
		}

		if err := os.Symlink(target, to); err != nil {
			return fmt.Errorf("linking %s: %w", to, err)
		}

		return nil
	}

	return copyFile(from, to, info.Mode().Perm())
}

func copyFile(from, to string, mode fs.FileMode) (err error) {
	in, err := os.Open(from) //nolint:gosec // G304: a path inside the tree jj handed this program.
	if err != nil {
		return fmt.Errorf("opening %s: %w", from, err)
	}

	defer func() {
		if cerr := in.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing %s: %w", from, cerr)
		}
	}()

	//nolint:gosec // G302,G304: the mode is the file's own, inside the tree jj handed this program.
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", to, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %s: %w", from, err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", to, err)
	}

	return nil
}

// gitApply applies patchFile inside dir, in reverse when reverse is set. Git resolves a patch's paths
// against the nearest enclosing repository rather than the working directory, so a repository above
// dir would turn the apply into a silent no-op; the ceiling stops that search at dir's parent.
func gitApply(dir, patchFile string, reverse bool) error {
	// A chain step whose patch dropUntouched emptied has nothing to apply.
	if info, err := os.Stat(patchFile); err == nil && info.Size() == 0 {
		return nil
	}

	ceiling := filepath.Dir(dir)
	if resolved, err := filepath.EvalSymlinks(ceiling); err == nil {
		ceiling = resolved
	}

	args := []string{"apply"}
	if reverse {
		args = append(args, "--reverse")
	}

	//nolint:gosec // G204: the binary is a literal; only the arguments vary and no shell is involved.
	cmd := exec.CommandContext(context.Background(), "git", append(args, patchFile)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+ceiling)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply the selection: %w: %s", err, output)
	}

	return nil
}

// dropUntouched rewrites both patch files without the files neither tree holds and both patches
// change alike.
func dropUntouched(left, right, unapplyPatch, applyPatch string) error {
	texts := make([]string, 0, 2)

	for _, path := range []string{unapplyPatch, applyPatch} {
		b, err := os.ReadFile(path) //nolint:gosec // G304: a patch this program wrote.
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		texts = append(texts, string(b))
	}

	held := func(path string) bool {
		for _, dir := range []string{left, right} {
			if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(path))); err == nil {
				return true
			}
		}

		return false
	}

	unapply, apply := patchSections(texts[0]), patchSections(texts[1])

	keep := func(sections, other []patchSection) string {
		var b strings.Builder

		for _, s := range sections {
			if !held(s.path) && slices.ContainsFunc(other, func(o patchSection) bool { return o == s }) {
				continue
			}

			b.WriteString(s.text)
		}

		return b.String()
	}

	for i, out := range []string{keep(unapply, apply), keep(apply, unapply)} {
		path := []string{unapplyPatch, applyPatch}[i]
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil { //nolint:mnd // private to the user
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	return nil
}

// patchSection is one file's part of a git diff, from its diff --git line to the next.
type patchSection struct {
	path string
	text string
}

// patchSections splits a git diff by file, naming each by its new path, or its old one when the file
// is deleted.
func patchSections(patch string) []patchSection {
	var out []patchSection

	for _, part := range strings.SplitAfter(patch, "\n") {
		if strings.HasPrefix(part, "diff --git ") || len(out) == 0 {
			out = append(out, patchSection{path: sectionPath(part)})
		}

		out[len(out)-1].text += part
	}

	return out
}

// sectionPath is the path a diff --git line names: the b/ side, which a rename and an addition carry.
func sectionPath(line string) string {
	_, b, ok := strings.Cut(strings.TrimSuffix(line, "\n"), " b/")
	if !ok {
		return ""
	}

	return b
}
