package main_test

import (
	"testing"

	"github.com/kyleking/aragonite/hermetic"
)

// Every package with tests installs the guard, so a test added to a new package cannot reach the
// real home unnoticed.
func TestEveryTestPackageIsHermetic(t *testing.T) {
	t.Parallel()

	hermetic.RequireEveryPackage(t, "../..")
}
