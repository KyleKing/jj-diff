package diff

import (
	"fmt"
	"strings"
)

// Section is the text git writes after a hunk's second @@: the enclosing function or type, or
// nothing when git found none. A hunk that creates or deletes a whole file has nothing around it,
// and jj writes the file's first line there, so that hunk has no section.
func (h *Hunk) Section() string {
	if (h.OldStart == 0 && h.OldLines == 0) || (h.NewStart == 0 && h.NewLines == 0) {
		return ""
	}

	rest, ok := strings.CutPrefix(h.Header, "@@")
	if !ok {
		return ""
	}

	_, section, ok := strings.Cut(rest, "@@")
	if !ok {
		return ""
	}

	return strings.TrimSpace(section)
}

// Range is the span the hunk covers in the new file, `L13–24`, or in the old one when it leaves
// nothing behind there. A hunk built without its header counts is measured from its lines.
func (h *Hunk) Range() string {
	start, count := h.NewStart, h.NewLines
	if count == 0 && h.OldLines > 0 {
		start, count = h.OldStart, h.OldLines
	}

	if start == 0 && count == 0 {
		start, count = measuredRange(h.Lines)
	}

	switch {
	case count <= 1:
		return fmt.Sprintf("L%d", start)
	default:
		return fmt.Sprintf("L%d–%d", start, start+count-1)
	}
}

func measuredRange(lines []Line) (int, int) {
	first, last := 0, 0

	for _, line := range lines {
		number := line.NewLineNum
		if line.Type == LineDeletion {
			number = line.OldLineNum
		}

		if first == 0 || number < first {
			first = number
		}

		last = max(last, number)
	}

	return first, max(last-first+1, 0)
}
