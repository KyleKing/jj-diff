package diff

import (
	agdiff "github.com/kyleking/aragonite/diff"
)

// HunkKey names a hunk by its file's index in a FileChange slice and its own index in that file.
type HunkKey struct {
	File int
	Hunk int
}

// Aragonite returns files in aragonite's model, which numbers hunks from 1 across the slice.
func Aragonite(files []FileChange) *agdiff.Diff {
	out := &agdiff.Diff{Files: make([]agdiff.File, len(files))}
	hunk := 0

	for i := range files {
		file := &files[i]
		converted := agdiff.File{OldPath: file.Path, NewPath: file.Path}

		switch file.ChangeType {
		case ChangeTypeAdded:
			converted.OldPath = ""
		case ChangeTypeDeleted:
			converted.NewPath = ""
		case ChangeTypeModified, ChangeTypeRenamed:
		}

		for j := range file.Hunks {
			hunk++
			out.Headers = append(out.Headers, file.Hunks[j].Header)

			for _, line := range file.Hunks[j].Lines {
				numbered := AragoniteLine(line)
				numbered.Hunk = hunk
				converted.Lines = append(converted.Lines, numbered)
			}
		}

		out.Files[i] = converted
	}

	return out
}

// AragoniteLine returns line in aragonite's model, with no hunk number.
func AragoniteLine(line Line) agdiff.Line {
	var converted agdiff.Line

	converted.Text = line.Content

	switch line.Type {
	case LineAddition:
		converted.Kind, converted.New = agdiff.KindAdd, line.NewLineNum
	case LineDeletion:
		converted.Kind, converted.Old = agdiff.KindRemove, line.OldLineNum
	case LineContext:
		converted.Kind, converted.Old, converted.New = agdiff.KindContext, line.OldLineNum, line.NewLineNum
	}

	return converted
}

// MechanicalHunks names why each reindent-only or whitespace-only hunk is mechanical, and omits the rest.
func MechanicalHunks(files []FileChange) map[HunkKey]string {
	reasons := Aragonite(files).MechanicalHunks()
	out := make(map[HunkKey]string, len(reasons))
	hunk := 0

	for i := range files {
		for j := range files[i].Hunks {
			hunk++

			if reason, ok := reasons[agdiff.HunkRef{Path: files[i].Path, Hunk: hunk}]; ok {
				out[HunkKey{File: i, Hunk: j}] = reason
			}
		}
	}

	return out
}
