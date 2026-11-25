// Package gorules holds the project's ruleguard checks. gocritic's ruleguard
// checker loads them through .golangci.toml; the file is parsed by gocritic
// and never built, and the dot directory is what keeps `go build` away from it.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// clampedCounts guards the calls that panic on a negative count. Subtraction
// is the shape that slips under zero when its two sides meet, so the count is
// clamped at the call site or computed clamped upstream.
func clampedCounts(m dsl.Matcher) {
	m.Match(
		`strings.Repeat($s, $a-$b)`,
		`strings.Repeat($s, -$a)`,
		`bytes.Repeat($s, $a-$b)`,
		`bytes.Repeat($s, -$a)`,
		`slices.Repeat($s, $a-$b)`,
		`slices.Repeat($s, -$a)`,
		`make([]$t, $a-$b)`,
		`make([]$t, $a-$b, $c)`,
		`make([]$t, $l, $a-$b)`,
		`make(chan $t, $a-$b)`,
	).Report("count can go negative; clamp it with max(0, ...)")
}
