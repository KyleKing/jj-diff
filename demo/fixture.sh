#!/bin/sh
# The demo repository's history, written into the jj repository in the current
# directory. The content is invented: a small recipe scaler with no ties to any
# real project. demo/scene.sh runs it; pass "conflicts" to add two rounding
# fixes that disagree and a merge of them bookmarked as "conflicted".
set -eu

scene=${1:-}

cat > recipe.go <<'EOF'
package pantry

import "fmt"

// Ingredient is one line of a recipe card.
type Ingredient struct {
	Name   string
	Amount float64
	Unit   string
}

// Recipe is a list of ingredients written for a number of servings.
type Recipe struct {
	Title    string
	Servings int
	Items    []Ingredient
}

// Scale returns the recipe rewritten for n servings.
func (r Recipe) Scale(n int) Recipe {
	factor := float64(n) / float64(r.Servings)
	out := Recipe{Title: r.Title, Servings: n}
	for _, it := range r.Items {
		it.Amount *= factor
		out.Items = append(out.Items, it)
	}
	return out
}

// String prints the recipe as a card.
func (r Recipe) String() string {
	s := fmt.Sprintf("%s (serves %d)\n", r.Title, r.Servings)
	for _, it := range r.Items {
		s += fmt.Sprintf("  %6.1f %-5s %s\n", it.Amount, it.Unit, it.Name)
	}
	return s
}
EOF

cat > units.go <<'EOF'
package pantry

// perCup is how many grams one cup of an ingredient weighs.
var perCup = map[string]float64{
	"flour": 120,
	"sugar": 200,
	"butter": 227,
}
EOF

cat > README.md <<'EOF'
# pantry

Scale a recipe card to the number of people at the table.
EOF

jj --quiet describe -m "feat: scale a recipe by servings"
jj --quiet new

cat > units.go <<'EOF'
package pantry

import "math"

// perCup is how many grams one cup of an ingredient weighs.
var perCup = map[string]float64{
	"flour":  120,
	"sugar":  200,
	"butter": 227,
	"oats":   90,
}

// Grams converts cups of an ingredient to grams, false when it is unknown.
func Grams(name string, cups float64) (float64, bool) {
	w, ok := perCup[name]
	if !ok {
		return 0, false
	}
	g := cups * w
	return math.Round(g), true
}
EOF

jj --quiet describe -m "feat: convert cups to grams"
jj --quiet bookmark create grams -r @

if [ "$scene" = conflicts ]; then
  jj --quiet new grams -m "fix: round grams to the nearest five"
  sed -i 's|return math.Round(g), true|return math.Round(g/5) * 5, true|' units.go
  sed -i 's|"oats":   90,|"oats":   85,|' units.go
  jj --quiet new grams -m "fix: keep a decimal for small amounts"
  sed -i 's|return math.Round(g), true|return math.Round(g*10) / 10, true|' units.go
  sed -i 's|"oats":   90,|"oats":   92,|' units.go
  jj --quiet new 'grams+' -m "merge the two rounding fixes"
  jj --quiet bookmark create conflicted -r @
  exit 0
fi

jj --quiet new grams

# The working copy mixes three intentions: a fix that belongs in the grams
# commit, a new feature, and a note for the README.
cat > units.go <<'EOF'
package pantry

import "math"

// perCup is how many grams one cup of an ingredient weighs.
var perCup = map[string]float64{
	"flour":  125,
	"sugar":  200,
	"butter": 227,
	"oats":   90,
}

// Grams converts cups of an ingredient to grams, false when it is unknown.
func Grams(name string, cups float64) (float64, bool) {
	w, ok := perCup[name]
	if !ok {
		return 0, false
	}
	g := cups * w
	return math.Round(g), true
}
EOF

cat > recipe.go <<'EOF'
package pantry

import (
	"fmt"
	"sort"
)

// Ingredient is one line of a recipe card.
type Ingredient struct {
	Name   string
	Amount float64
	Unit   string
}

// Recipe is a list of ingredients written for a number of servings.
type Recipe struct {
	Title    string
	Servings int
	Items    []Ingredient
}

// Scale returns the recipe rewritten for n servings.
func (r Recipe) Scale(n int) Recipe {
	if n <= 0 || r.Servings <= 0 {
		return r
	}
	factor := float64(n) / float64(r.Servings)
	out := Recipe{Title: r.Title, Servings: n}
	for _, it := range r.Items {
		it.Amount *= factor
		out.Items = append(out.Items, it)
	}
	return out
}

// String prints the recipe as a card.
func (r Recipe) String() string {
	s := fmt.Sprintf("%s (serves %d)\n", r.Title, r.Servings)
	for _, it := range r.Items {
		s += fmt.Sprintf("  %6.1f %-5s %s\n", it.Amount, it.Unit, it.Name)
	}
	return s
}

// Shopping sums every recipe's ingredients into one list, sorted by name.
func Shopping(recipes ...Recipe) []Ingredient {
	totals := map[string]Ingredient{}
	for _, r := range recipes {
		for _, it := range r.Items {
			t := totals[it.Name]
			t.Name, t.Unit = it.Name, it.Unit
			t.Amount += it.Amount
			totals[it.Name] = t
		}
	}
	list := make([]Ingredient, 0, len(totals))
	for _, it := range totals {
		list = append(list, it)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
EOF

cat > README.md <<'EOF'
# pantry

Scale a recipe card to the number of people at the table.

`Shopping` adds several cards into one list for the week's shop.
EOF

jj --quiet describe -m "feat: a shopping list across recipes"
