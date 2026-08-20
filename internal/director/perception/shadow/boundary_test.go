package shadow_test

import (
	"go/build"
	"strings"
	"testing"
)

// Shadow perception detects and reports. Nothing else.
//
// The safety tests in the providers package prove shadow EVIDENCE cannot reach belief. This
// proves the shadow code cannot reach an ACTUATOR — a different guarantee, and one that a
// behavioural test cannot give, because the dangerous version of this code is the one someone
// writes next year with a good reason.
//
// Enforced against the import graph, so it fails at the moment the dependency appears rather
// than at the moment it is exercised.
func TestShadowCannotReachAnythingThatActs(t *testing.T) {
	// Packages that can cause a real effect: input, execution, policy mutation.
	forbidden := []string{
		"/internal/oshost",
		"/internal/driver",
		"/internal/recorder",
		"/internal/director/execute",
		"/internal/director/marcoexec",
		"/internal/director/plan",
		"/internal/director/policy",
		"/internal/runtime",
	}

	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}
	for _, imp := range append(pkg.Imports, pkg.TestImports...) {
		for _, bad := range forbidden {
			if strings.Contains("/"+imp, bad) {
				t.Errorf("the shadow package imports %q. Shadow perception may observe and "+
					"report; it may not reach anything that can act. If this dependency is "+
					"genuinely needed, the thing to move is the data, not the boundary", imp)
			}
		}
	}
}
