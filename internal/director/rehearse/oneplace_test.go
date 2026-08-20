package rehearse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// "Where are we?" has ONE answer, and this package asks for it rather than working it out.
//
// # Why a source rule and not a behavioural one
//
// Because the failure this guards is not a wrong answer — it is a SECOND answer. A rehearsal that
// derived the current place itself agreed with Sight for as long as both were maintained, and the
// day they stopped agreeing the panel and the attempt would describe different screens with no
// way to tell which was right from the outside.
//
// The audit that opened Roadmap 34E found "what place is on stage now" answered in several
// places. They all projected through `SignatureOfState` and resolved through `Recall`, so they
// agreed — which is exactly why nobody noticed, and exactly why the copies survived.
//
// `observe.PlaceNow` is that one answer: it projects the settled evidence and resolves it against
// durable memory, samples nothing, stores nothing, and holds no cache. Its parameter is
// `observe.Recogniser`, which can only recall — a resolver cannot establish anything on the way
// past.
//
// Restoring a local `SignatureOfState` + `Recall` pair must fail this.
func TestThisPackageHasNoCurrentPlaceResolverOfItsOwn(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}
	var usesPlaceNow bool
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		src := string(b)
		if strings.Contains(src, "observe.PlaceNow(") {
			usesPlaceNow = true
		}
		if strings.Contains(src, ".Recall(") {
			t.Errorf("%s recalls a place itself.\nThe current place has one answer, "+
				"observe.PlaceNow, and a second one here would describe a different "+
				"screen from the panel the person is reading.", name)
		}
		if strings.Contains(src, "observe.SignatureOfState(") {
			t.Errorf("%s projects a place signature itself.\nTwo projections of the same "+
				"screen is how the same page becomes two durable subjects.", name)
		}
	}
	if !usesPlaceNow {
		t.Error("nothing in this package asks observe.PlaceNow where it is; either the " +
			"resolver moved or this test is guarding nothing")
	}
}
