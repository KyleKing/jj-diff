package config_test

import (
	"testing"

	"github.com/kyleking/aragonite/hermetic"
)

// TestMain runs the tests in a home of their own, failing the run if one reaches the real one.
func TestMain(m *testing.M) { hermetic.Main(m, hermetic.App("jj-diff", "jojo")) }
