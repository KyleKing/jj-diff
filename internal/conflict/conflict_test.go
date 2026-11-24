package conflict_test

import (
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/conflict"
)

// two is what jj 0.47.1 writes for a two-region, two-sided conflict in its snapshot style.
//
//nolint:gochecknoglobals // a fixture, built from lines so no marker starts a line of this file
var two = strings.Join([]string{
	"a",
	"<<<<<<< conflict 1 of 2",
	"+++++++ wwtworlq ec4ab3b0 \"side1\"",
	"B1",
	"------- ymzvyvxs 98d54153 \"base\"",
	"b",
	"+++++++ tvyqvpko e8fcb539 \"side2\"",
	"B2",
	">>>>>>> conflict 1 of 2 ends",
	"c",
	"<<<<<<< conflict 2 of 2",
	"+++++++ wwtworlq ec4ab3b0 \"side1\"",
	"G1",
	"------- ymzvyvxs 98d54153 \"base\"",
	"g",
	"+++++++ tvyqvpko e8fcb539 \"side2\"",
	"G2",
	">>>>>>> conflict 2 of 2 ends",
	"h",
}, "\n") + "\n"

func TestParse_ReadsSidesBasesAndLabels(t *testing.T) {
	t.Parallel()

	f := conflict.Parse(two)

	regions := f.Regions()
	if len(regions) != 2 {
		t.Fatalf("got %d regions, want 2", len(regions))
	}

	r := regions[0]
	if len(r.Sides) != 2 || len(r.Bases) != 1 {
		t.Fatalf("got %d sides and %d bases", len(r.Sides), len(r.Bases))
	}

	if r.Sides[0].Label != `wwtworlq ec4ab3b0 "side1"` || r.Sides[1].Text != "B2\n" || r.Bases[0].Text != "b\n" {
		t.Errorf("region 1 = %+v", r)
	}

	if got := f.Resolve(nil); got != two {
		t.Errorf("an unpicked file does not write back as it was:\n%s", got)
	}
}

func TestResolve_EachPick(t *testing.T) {
	t.Parallel()

	f := conflict.Parse(two)
	_, rest, _ := strings.Cut(two, ">>>>>>> conflict 1 of 2 ends\n")
	second := rest

	tests := []struct {
		picks map[int]conflict.Pick
		name  string
		want  string
	}{
		{
			name: "side 2 of region 1 leaves region 2", want: "a\nB2\n" + second,
			picks: map[int]conflict.Pick{0: {Kind: conflict.TakeSide, Side: 2}},
		},
		{name: "base", want: "a\nb\n" + second, picks: map[int]conflict.Pick{0: {Kind: conflict.TakeBase}}},
		{name: "all", want: "a\nB1\nB2\n" + second, picks: map[int]conflict.Pick{0: {Kind: conflict.TakeAll}}},
		{
			name: "both regions", want: "a\nB1\nc\nG2\nh\n",
			picks: map[int]conflict.Pick{0: {Kind: conflict.TakeSide, Side: 1}, 1: {Kind: conflict.TakeSide, Side: 2}},
		},
		{
			name: "custom", want: "a\nmine\n" + second,
			picks: map[int]conflict.Pick{0: {Kind: conflict.Custom, Text: "mine\n"}},
		},
		{name: "a side it lacks", want: two, picks: map[int]conflict.Pick{0: {Kind: conflict.TakeSide, Side: 3}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := f.Resolve(tt.picks); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestParse_LeavesWhatItCannotReadAsText(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"diff style": "<<<<<<< conflict 1 of 1\n%%%%%%% diff from: a\n\\\\\\\\\\\\\\        to: b\n-x\n+y\n" +
			"+++++++ c\nz\n>>>>>>> conflict 1 of 1 ends\n",
		"unterminated":   "<<<<<<< conflict 1 of 1\n+++++++ a\nx\n",
		"missing a base": "<<<<<<< conflict 1 of 1\n+++++++ a\nx\n+++++++ b\ny\n>>>>>>> conflict 1 of 1 ends\n",
		"short marker":   "<<<<<< not one\n",
	}

	for name, text := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := conflict.Parse(text)
			if len(f.Regions()) != 0 {
				t.Errorf("read a region from %q", text)
			}

			if got := f.Resolve(nil); got != text {
				t.Errorf("wrote back %q", got)
			}
		})
	}
}

// jj lengthens every marker in a region whose text holds a line that reads as a marker.
func TestParse_LongerMarkers(t *testing.T) {
	t.Parallel()

	text := "<<<<<<<<< conflict 1 of 1\n+++++++++ a\n<<<<<<< x\n--------- base\nb\n+++++++++ c\nz\n" +
		">>>>>>>>> conflict 1 of 1 ends\n"

	r := conflict.Parse(text).Regions()
	if len(r) != 1 || r[0].Sides[0].Text != "<<<<<<< x\n" {
		t.Fatalf("got %+v", r)
	}
}

func TestParse_NoTerminatingNewline(t *testing.T) {
	t.Parallel()

	text := "<<<<<<< conflict 1 of 1\n+++++++ a (no terminating newline)\nx\n------- b\ny\n" +
		"+++++++ c (no terminating newline)\nz\n>>>>>>> conflict 1 of 1 ends\n"

	f := conflict.Parse(text)
	if got := f.Resolve(map[int]conflict.Pick{0: {Kind: conflict.TakeSide, Side: 1}}); got != "x" {
		t.Errorf("got %q, want x with no newline", got)
	}

	if got := f.Regions()[0].Sides[1].Label; got != "c" {
		t.Errorf("label = %q", got)
	}
}
