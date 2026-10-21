package buildid_test

// WHICH BUILD IS RUNNING, WHICH GIT CANNOT ANSWER.
//
// Git identifies the source. It says nothing about the executable somebody double-clicked, and
// that gap cost the owner a day: Studio spawns the `director.exe` beside its own executable, a
// tree held two of each, and the stale one was invisible because every file looked current.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/buildid"
)

// A BUILD SAYS WHAT IT IS, OR SAYS IT CANNOT.
func TestARevisionIsShortOrHonestlyUnknown(t *testing.T) {
	got := buildid.Revision()
	if got == "" {
		t.Fatal("a build reports an empty revision, which reads as a field nobody " +
			"filled rather than as a build nobody stamped")
	}
	if got == "unknown" {
		// LEGITIMATE: a package built from outside its own module is not stamped,
		// which is exactly what the build script used to do to the director.
		return
	}
	short := strings.TrimSuffix(got, "+dirty")
	if len(short) != 7 {
		t.Errorf("the revision is %q; it is read by a person comparing two of them "+
			"in a window, so it is the short form", got)
	}
}

// AND AN UNKNOWN NEVER AGREES WITH ANYTHING, including another unknown.
//
// Treating silence as agreement is how this happened in the first place: a stream with no build
// on it was read as "the same build" and the difference stayed invisible.
func TestABuildThatCannotSayWhatItIsIsNeverVouchedFor(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"fd6b97b", "fd6b97b", true},
		{"fd6b97b", "fd6b97b+dirty", false},
		{"fd6b97b", "aaaaaaa", false},
		{"unknown", "unknown", false},
		{"unknown", "fd6b97b", false},
		{"", "fd6b97b", false},
		{"", "", false},
		{" fd6b97b ", "fd6b97b", true},
	} {
		if got := buildid.Same(c.a, c.b); got != c.same {
			t.Errorf("Same(%q, %q) is %v", c.a, c.b, got)
		}
	}
}

// AND "CANNOT TELL" IS NOT "THEY DIFFER".
//
// `Differ` is not the negation of `Same`. Same answers "can I vouch that these match", which an
// unknown cannot; Differ answers "can I SEE that they do not", which an unknown also cannot.
// Between them sits a pair nothing can say anything about, and conflating that with
// disagreement turns every unstamped binary into an alarm — which is how this was found, by a
// test binary reporting one process as two builds.
func TestCannotTellIsNotADisagreement(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"unknown", "unknown"},
		{"unknown", "fd6b97b"},
		{"fd6b97b", "unknown"},
		{"", ""},
		{"", "fd6b97b"},
	} {
		if buildid.Same(c.a, c.b) {
			t.Errorf("Same(%q, %q) vouches for a build that cannot say what it "+
				"is", c.a, c.b)
		}
		if buildid.Differ(c.a, c.b) {
			t.Errorf("Differ(%q, %q) reports a disagreement nobody observed",
				c.a, c.b)
		}
	}
	if !buildid.Differ("fd6b97b", "aaaaaaa") {
		t.Error("two known and different revisions are not reported as differing")
	}
	if buildid.Differ("fd6b97b", "fd6b97b") {
		t.Error("one revision differs from itself")
	}
}
