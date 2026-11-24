// Package conflict reads a file jj materialized with conflict markers in its snapshot style, where
// each side and each base is written out whole under a label naming the revision it came from, and
// writes the file back with some regions resolved and the rest left as the markers they were.
package conflict

import (
	"regexp"
	"strings"
)

// noNewline is what jj appends to a side's label when that side ends the file without a newline.
const noNewline = "(no terminating newline)"

// opening is a region's first marker. Seven is the shortest jj writes; it writes longer ones when the
// file already holds a line that would read as a marker, and every marker in a region is as long.
//
//nolint:gochecknoglobals // compiled once, read only.
var opening = regexp.MustCompile(`^(<{7,})(?: (.*))?$`)

// Term is one side or base of a region: the label jj gave it, naming the revision it came from, and
// its text with every line's newline.
type Term struct {
	Label string
	Text  string
}

// Lines is the term's text split into lines without their newlines.
func (t Term) Lines() []string {
	if t.Text == "" {
		return nil
	}

	return strings.Split(strings.TrimSuffix(t.Text, "\n"), "\n")
}

// Region is one conflicted region. Sides are what each parent made of it, in jj's order, and Bases
// sit between them, one fewer than the sides. Raw is the region exactly as the file holds it, markers
// included, which is what an unresolved region is written back as.
type Region struct {
	Raw   string
	Sides []Term
	Bases []Term
}

// Segment is a run of the file: plain text, or a region when Region is set.
type Segment struct {
	Region *Region
	Text   string
}

// File is a conflicted file split into its plain runs and its regions, in order.
type File struct {
	Segments []Segment
}

// Regions is the file's regions in order.
func (f File) Regions() []*Region {
	var out []*Region

	for _, s := range f.Segments {
		if s.Region != nil {
			out = append(out, s.Region)
		}
	}

	return out
}

// Parse splits text into plain runs and regions. A region whose markers it cannot read (another
// marker style, or an unterminated one) is kept as plain text, so writing the file back changes
// nothing there.
func Parse(text string) File {
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var (
		f     File
		plain strings.Builder
	)

	flush := func() {
		if plain.Len() > 0 {
			f.Segments = append(f.Segments, Segment{Text: plain.String()})
			plain.Reset()
		}
	}

	for i := 0; i < len(lines); i++ {
		region, end, ok := parseRegion(lines, i)
		if !ok {
			plain.WriteString(lines[i])

			continue
		}

		flush()
		f.Segments = append(f.Segments, Segment{Region: region})
		i = end
	}

	flush()

	return f
}

// parseRegion reads the region opening at lines[start], answering it and the index of its closing
// marker, or false when lines[start] opens none it can read.
func parseRegion(lines []string, start int) (*Region, int, bool) {
	m := opening.FindStringSubmatch(strings.TrimRight(lines[start], "\r\n"))
	if m == nil {
		return nil, 0, false
	}

	n := len(m[1])
	add, remove, closing := strings.Repeat("+", n), strings.Repeat("-", n), strings.Repeat(">", n)
	region := &Region{}

	var term *Term

	for i := start + 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")

		switch {
		case isMarker(line, closing):
			if len(region.Sides) == 0 || len(region.Bases) != len(region.Sides)-1 {
				return nil, 0, false
			}

			region.Raw = strings.Join(lines[start:i+1], "")
			trimNoNewline(region)

			return region, i, true
		case isMarker(line, add):
			region.Sides = append(region.Sides, Term{Label: label(line, n)})
			term = &region.Sides[len(region.Sides)-1]
		case isMarker(line, remove):
			region.Bases = append(region.Bases, Term{Label: label(line, n)})
			term = &region.Bases[len(region.Bases)-1]
		case term == nil || isMarker(line, strings.Repeat("%", n)) || isMarker(line, strings.Repeat(`\`, n)):
			// Text before the first term, or a diff-style section: not a snapshot region.
			return nil, 0, false
		default:
			term.Text += lines[i]
		}
	}

	return nil, 0, false
}

// isMarker reports whether line is marker alone or marker followed by a space and a label.
func isMarker(line, marker string) bool {
	return line == marker || strings.HasPrefix(line, marker+" ")
}

func label(line string, n int) string {
	return strings.TrimSpace(line[n:])
}

// trimNoNewline takes the newline jj had to add after a term that ends the file without one, which
// its label says, back off the term's text.
func trimNoNewline(r *Region) {
	for _, terms := range [][]Term{r.Sides, r.Bases} {
		for i := range terms {
			if strings.Contains(terms[i].Label, noNewline) {
				terms[i].Label = strings.TrimSpace(strings.Replace(terms[i].Label, noNewline, "", 1))
				terms[i].Text = strings.TrimSuffix(terms[i].Text, "\n")
			}
		}
	}
}

// Kind is how a region is resolved.
type Kind int

// The ways to resolve a region. Unresolved, the zero value, leaves its markers in place.
const (
	Unresolved Kind = iota
	TakeSide
	TakeBase
	TakeAll
	Custom
)

// Pick is the choice for one region. Side is 1-based and read only by TakeSide; Text only by Custom.
type Pick struct {
	Text string
	Kind Kind
	Side int
}

// Text is what pick makes of r, false when the pick names a side r lacks.
func (r *Region) Text(p Pick) (string, bool) {
	switch p.Kind {
	case TakeSide:
		if p.Side < 1 || p.Side > len(r.Sides) {
			return "", false
		}

		return r.Sides[p.Side-1].Text, true
	case TakeBase:
		if len(r.Bases) == 0 {
			return "", false
		}

		return r.Bases[0].Text, true
	case TakeAll:
		var b strings.Builder

		for _, s := range r.Sides {
			b.WriteString(s.Text)

			if s.Text != "" && !strings.HasSuffix(s.Text, "\n") {
				b.WriteByte('\n')
			}
		}

		return b.String(), true
	case Custom:
		return p.Text, true
	case Unresolved:
	}

	return r.Raw, true
}

// Resolve writes the file back with each region the picks name, by its index among the regions,
// replaced by what the pick makes of it, and every other region left as its markers.
func (f File) Resolve(picks map[int]Pick) string {
	var (
		b      strings.Builder
		region int
	)

	for _, s := range f.Segments {
		if s.Region == nil {
			b.WriteString(s.Text)

			continue
		}

		text, ok := s.Region.Text(picks[region])
		if !ok {
			text = s.Region.Raw
		}

		b.WriteString(text)

		region++
	}

	return b.String()
}

// Around is the plain text either side of region i, empty where another region or the file's edge
// sits there instead.
func (f File) Around(i int) (string, string) {
	var before, after string

	n := -1

	for j, s := range f.Segments {
		if s.Region == nil {
			continue
		}

		n++
		if n != i {
			continue
		}

		if j > 0 && f.Segments[j-1].Region == nil {
			before = f.Segments[j-1].Text
		}

		if j+1 < len(f.Segments) && f.Segments[j+1].Region == nil {
			after = f.Segments[j+1].Text
		}

		break
	}

	return before, after
}
