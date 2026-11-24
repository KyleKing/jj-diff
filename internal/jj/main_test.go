//nolint:testpackage // the package's tests are internal, and TestMain goes with them
package jj

import (
	"testing"

	"github.com/kyleking/aragonite/hermetic"
)

// TestMain runs the tests in a home of their own, failing the run if one reaches the real one.
func TestMain(m *testing.M) { hermetic.Main(m, hermetic.App("jj-diff", "jojo")) }
