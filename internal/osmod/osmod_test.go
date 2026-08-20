package osmod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/osmod"
)

// The seeded routes/os.marco must not drift from the canonical embedded surface.
//
// buildGraph prefers a sibling <module>.marco over the built-in, so a stale copy in
// routes/ SHADOWS the real act surface for every route in that tree. That is how a
// capability can be exported, tested, and still be unreachable from the routes people
// actually run — the failure this test exists to catch. It was caught for real:
// routes/os.marco was missing KeyDown, KeyUp, Drag, Roll, EightBall and Restore.
func TestSeededRouteSurfaceMatchesTheCanonicalOne(t *testing.T) {
	seeded, err := os.ReadFile(filepath.Join("..", "..", "routes", "os.marco"))
	if err != nil {
		t.Skipf("no seeded copy to check: %v", err)
	}
	if norm(string(seeded)) != norm(osmod.Source) {
		t.Fatal("routes/os.marco has drifted from internal/osmod/os.marco — it shadows the " +
			"embedded surface for every route under routes/, so capabilities exported here " +
			"are silently unreachable there. Copy the canonical file over it.")
	}
}

func norm(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n") }
