package jj

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// ApplySelection runs this program's half of a write, run by jj as the diff editor with the two trees
// it materialized. It makes right the left tree with the unapply patch taken back out and then the
// apply patch put in, either patch path being empty to skip it. Whatever right holds is what jj
// records, so right ends up exactly the selection whatever diff jj started it as.
//
// The two trees hold only the files that differ, so replacing right wholesale touches nothing the
// command was not already rewriting.
func ApplySelection(left, right, unapplyPatch, applyPatch string) error {
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
