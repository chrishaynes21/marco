package resolveroute

// WHICH APPLICATION A REQUEST IS READ AGAINST, AND WHY AGE IS NOT PROOF.
//
// The two cases that matter here are both ones nobody can arrange on a desktop on purpose, which
// is why `Where` is a pure function of three readings: a FRESH observation of an application that
// no longer has the foreground, and a fresh observation of a Marco window. Neither may unlock a
// routine that works only in place, and the first version of this would have let both through on
// the strength of the observation's age.

import (
	"strings"
	"testing"
	"time"
)

func when(sec int) time.Time {
	return time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC).Add(time.Duration(sec) * time.Second)
}

// AND WHAT A PERSON NAMED IS THE AUTHORITATIVE ANSWER.
//
// The only one of the three that is a statement of intent rather than an observation, which is
// why it beats a corroborated foreground rather than agreeing with it: somebody choosing an
// application in Studio has said what they mean, and Studio has the foreground while they do it.
func TestAStatedApplicationBeatsEveryObservation(t *testing.T) {
	where := Where("notepad",
		Seen{App: "arkascended", At: when(0), OK: true},
		Seen{App: "arkascended", OK: true}, when(1))
	if where.Basis != Stated || where.App != "notepad" {
		t.Fatalf("a stated application produced %+v", where)
	}
	if !where.Basis.Admits() {
		t.Error("a stated application does not admit an in-place play")
	}
	if !strings.Contains(where.Says, "you named") {
		t.Errorf("the sentence does not say where it came from: %q", where.Says)
	}
}

// AND A CORROBORATED FOREGROUND IS THE ONLY OBSERVATION THAT ADMITS ANYTHING.
func TestALiveForegroundReadingAdmitsInPlaceRoutines(t *testing.T) {
	where := Where("",
		Seen{App: "arkascended", At: when(0), OK: true},
		Seen{App: "arkascended", OK: true}, when(1))
	if where.Basis != Observed || where.App != "arkascended" {
		t.Fatalf("a corroborated foreground produced %+v", where)
	}
	if !where.Basis.Admits() {
		t.Error("a corroborated foreground does not admit an in-place play")
	}
	if !strings.Contains(where.Says, "in front now") {
		t.Errorf("the sentence does not say it is current: %q", where.Says)
	}
	// AND THE LIVE READING IS WHAT IS CARRIED, not the world model's, when they differ.
	// The reading taken now is the better evidence and there is nothing to merge.
	where = Where("",
		Seen{App: "notepad", At: when(0), OK: true},
		Seen{App: "arkascended", OK: true}, when(1))
	if where.App != "arkascended" {
		t.Errorf("the context is %q, want the application the live reading named",
			where.App)
	}
}

// ── THE TWO THAT AGE WOULD HAVE LET THROUGH ────────────────────────────────────

// A FRESH OBSERVATION OF AN APPLICATION THAT NO LONGER HAS THE FOREGROUND.
//
// Studio takes focus a moment after Marco last saw Ark in front. The observation is seconds old
// and inside any age bound anybody would pick, and it is already wrong — because the window
// system's foreground claim comes through the path that drops Marco's own surfaces, so with
// Studio in front NO observation is produced and the last one simply stands.
//
// It must not unlock an in-place routine, and it must not carry an application either: the field
// anything acts on is filled only when it is known.
func TestAFreshObservationThatIsNoLongerInFrontUnlocksNothing(t *testing.T) {
	where := Where("",
		Seen{App: "arkascended", At: when(10), OK: true}, // a second ago
		Seen{OK: false}, // and a Marco window is in front
		when(11))
	if where.Basis != LastExternal {
		t.Fatalf("a fresh but uncorroborated observation produced basis %q", where.Basis)
	}
	if where.Basis.Admits() {
		t.Error("an observation that could not be corroborated admitted an in-place " +
			"routine")
	}
	if where.App != "" {
		t.Errorf("the context carries %q, and nothing is known to be in front",
			where.App)
	}
	// THE SENTENCE SAYS WHAT WAS LAST SEEN, as prose, where nothing can mistake it for a
	// fact about now — and says how to get a context at all.
	for _, says := range []string{"nothing is known to be in front", "arkascended",
		"1s ago", "name an application"} {
		if !strings.Contains(where.Says, says) {
			t.Errorf("the sentence does not say %q: %q", says, where.Says)
		}
	}
}

// AND A FRESH OBSERVATION OF A MARCO WINDOW.
//
// The accessibility provider takes the bridge's first window with no ownership exclusion, so it
// can report one of Marco's own surfaces as the active application. Nothing here checks the name
// against a list of Marco's windows, and it does not need to: the observation cannot be
// corroborated by the reading that excludes them, so it is not a context, so it names nothing.
func TestAFreshMarcoOwnedObservationUnlocksNothing(t *testing.T) {
	where := Where("",
		Seen{App: "studio", At: when(10), OK: true},
		Seen{OK: false},
		when(10))
	if where.Basis != LastExternal || where.Basis.Admits() {
		t.Fatalf("a Marco-owned observation produced %+v", where)
	}
	if where.App != "" {
		t.Errorf("the context carries %q, which is one of Marco's own surfaces",
			where.App)
	}
}

// AND NO OBSERVATION AT ALL SAYS SO.
func TestNoObservationAtAllIsUnknown(t *testing.T) {
	where := Where("", Seen{OK: false}, Seen{OK: false}, when(0))
	if where.Basis != Unknown || where.Basis.Admits() || where.App != "" {
		t.Fatalf("no reading at all produced %+v", where)
	}
	if !strings.Contains(where.Says, "name an application") {
		t.Errorf("the sentence does not say what to do about it: %q", where.Says)
	}
	// AND AN EMPTY NAME IS NOT A READING. A provider that answered with no application is
	// answering nothing, and `OK` is not what distinguishes that on its own.
	if got := Where("", Seen{App: "   ", OK: true}, Seen{App: "", OK: true}, when(0)); got.Basis != Unknown {
		t.Errorf("a reading with no name in it produced %+v", got)
	}
}

// AND AGE DECORATES THE SENTENCE AND DECIDES NOTHING.
//
// Worth its own case because age was going to be the whole mechanism. A ten-minute-old
// observation and a one-second-old one reach the SAME basis, because what makes either of them
// wrong is the foreground having moved and not the clock.
func TestAgeChangesTheSentenceAndNotTheBasis(t *testing.T) {
	recent := Where("", Seen{App: "arkascended", At: when(600), OK: true},
		Seen{OK: false}, when(601))
	old := Where("", Seen{App: "arkascended", At: when(0), OK: true},
		Seen{OK: false}, when(601))
	if recent.Basis != old.Basis {
		t.Errorf("age decided the basis: %q and %q", recent.Basis, old.Basis)
	}
	if recent.Says == old.Says {
		t.Error("the sentence does not say how old the reading was")
	}
	if !strings.Contains(old.Says, "10m1s ago") {
		t.Errorf("the older sentence does not say its age: %q", old.Says)
	}
	// AND A READING FROM THE FUTURE, OR WITH NO TIME AT ALL, SAYS NOTHING ABOUT AGE
	// rather than a negative number of seconds.
	for _, odd := range []Seen{
		{App: "arkascended", OK: true},
		{App: "arkascended", At: when(900), OK: true},
	} {
		got := Where("", odd, Seen{OK: false}, when(601))
		if strings.Contains(got.Says, "ago") {
			t.Errorf("a reading with no usable time claims an age: %q", got.Says)
		}
	}
}
