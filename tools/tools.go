//go:build tools

// Package tools holds the imports go.mod has to keep even though no source
// file names them. gocritic type-checks .golangci/rules.go, so the dsl package
// must resolve as a real dependency.
package tools

import (
	_ "github.com/quasilyte/go-ruleguard/dsl"
)
