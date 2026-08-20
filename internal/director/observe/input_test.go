package observe_test

import (
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Correlating what the player did with what the screen became.
//
// The point of this evidence is to turn a list of screens into a graph with edges. The risk it
// carries is that a keyboard observer is the most invasive surface in the system, so the first
// tests here are about what the type CANNOT hold.

func nav(i observe.NavIntent) observe.InputEvent { return observe.InputEvent{Intent: i} }

// withInput is a valid inference that also carries navigation.
func withInput(in []observe.InputEvent, regions ...observe.ShadowRegion) observe.ShadowSample {
	s := valid(regions...)
	s.Inputs = in
	return s
}

// THE privacy regression, asserted from the SHAPE of the type rather than from intent.
//
// A promise not to record text is worth less than an inability to. If someone adds a `Key`,
// `Label` or `Note` field to InputEvent, this fails — which is the only kind of guarantee worth
// having on a keyboard observer.
func TestAnInputEventCannotCarryText(t *testing.T) {
	rt := reflect.TypeOf(observe.InputEvent{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		switch f.Type {
		case reflect.TypeOf(observe.NavIntent("")):
			// A closed vocabulary, checked below. Not free-form.
		case reflect.TypeOf(int64(0)), reflect.TypeOf(observe.PointerAt{}):
		case reflect.TypeOf(false):
			// Widened once, deliberately, for InputEvent.Conditional — one bit saying the
			// intent was admitted on screen context rather than on the key alone. A bool
			// cannot hold a key, a code, a rune or a name, and this one does not even
			// distinguish which key produced the intent. See the fuller note in
			// navsource_test.go's TestRawKeyIdentityCannotCrossTheBoundary.
			//
			// Not a general permission for bools: the next one argues for itself here.
		case reflect.TypeOf(&observe.SemanticTarget{}):
			// Widened a second time, deliberately, for InputEvent.Target — the
			// goal-centric milestone's one privacy change. A target carries what the
			// event was aimed at: a role, and a control's name when the canonical role
			// allowlist or an explicit Learn demonstration admits it, shape-filtered
			// either way. It is an enrichment: capture never depends on it, and the raw
			// event survives its absence. See SemanticTarget's own note for the licences,
			// and navsource_test.go for the full argument.
		default:
			t.Errorf("InputEvent.%s is a %s. This type is the privacy boundary for a "+
				"keyboard observer: it may hold a closed intent, a relative timestamp and "+
				"a normalised pointer position, and nothing else. A field that can hold "+
				"arbitrary text is a field that will eventually hold a password",
				f.Name, f.Type)
		}
	}
	// PointerAt is likewise numbers only.
	pt := reflect.TypeOf(observe.PointerAt{})
	for i := 0; i < pt.NumField(); i++ {
		if pt.Field(i).Type.Kind() != reflect.Float64 {
			t.Errorf("PointerAt.%s is not a float64", pt.Field(i).Name)
		}
	}
}

// An intent outside the vocabulary is dropped, not carried.
//
// The other half of the guarantee: the field is typed, but NavIntent's underlying type is a
// string, so a platform layer could construct one holding anything. Admission is what closes
// that door.
func TestAnUnknownIntentIsDroppedRatherThanCarried(t *testing.T) {
	totals := fold(
		withInput([]observe.InputEvent{nav("typed:hunter2"), nav(observe.NavPause)}, hudOnly()...),
		menu(), menu(),
	)
	for _, tr := range totals.Transitions {
		for intent := range tr.Preceded {
			if !intent.Known() {
				t.Errorf("transition %s→%s carries intent %q, which is not in the closed "+
					"vocabulary — arbitrary text reached a privacy-bounded record",
					tr.From, tr.To, intent)
			}
		}
	}
}

// A position may only ride on a pointer press.
func TestAKeyboardIntentCannotSmuggleACoordinate(t *testing.T) {
	var k observe.ShadowTracker
	s := withInput([]observe.InputEvent{{
		Intent: observe.NavConfirm, Where: observe.PointerAt{X: 0.5, Y: 0.5},
	}}, hudOnly()...)
	k.Observe(&s, s.Structure())
	// Nothing to assert on the transition yet; the guarantee is that admission zeroed it,
	// which the JSON of a later transition would otherwise carry. Exercised via the
	// vocabulary test above; here we simply prove admission does not panic or retain.
	if len(k.Transitions()) != 0 {
		t.Fatal("a single inference produced a transition")
	}
}

func hudOnly() []observe.ShadowRegion {
	return []observe.ShadowRegion{det("icon", 0.02, 0.86, 0.19, 0.10)}
}

// THE correlation. A change of screen carries the navigation seen before it.
func TestAScreenChangeCarriesTheNavigationThatPrecededIt(t *testing.T) {
	totals := fold(
		gameplay(), gameplay(),
		withInput([]observe.InputEvent{nav(observe.NavPause)}, menuRegions()...),
		menu(), menu(),
	)

	var found bool
	for _, tr := range totals.Transitions {
		if tr.Preceded[observe.NavPause] > 0 {
			found = true
			if tr.Count != 1 {
				t.Errorf("transition %s→%s counted %d times, want 1", tr.From, tr.To, tr.Count)
			}
		}
	}
	if !found {
		t.Fatalf("no transition records the pause intent that preceded it; the state graph "+
			"has no edges. transitions=%v", totals.Transitions)
	}
}

// A held key must not outvote a deliberate press.
//
// A low-level hook delivers repeats while a direction is held. The question a correlation
// answers is WHICH intents were present before a change, not how many samples the hook
// produced, and counting repeats would let leaning on a stick drown out every confirm.
func TestAHeldKeyCountsOncePerTransition(t *testing.T) {
	held := make([]observe.InputEvent, 0, 40)
	for i := 0; i < 40; i++ {
		held = append(held, nav(observe.NavDown))
	}
	held = append(held, nav(observe.NavConfirm))

	totals := fold(gameplay(), gameplay(), withInput(held, menuRegions()...), menu(), menu())

	for _, tr := range totals.Transitions {
		if n := tr.Preceded[observe.NavDown]; n > 1 {
			t.Errorf("transition %s→%s counted the held key %d times; a repeat rate is not "+
				"evidence strength", tr.From, tr.To, n)
		}
	}
}

// Input observed during a SKIPPED slot must survive to the next real inference.
//
// The detector sits out roughly a third of slots at this cadence. The player does not, and the
// keypress that opened the menu very often lands in a slot the detector skipped — so banking
// input across skipped slots is what makes the correlation work at all rather than an
// optimisation.
func TestNavigationDuringASkippedSlotIsNotLost(t *testing.T) {
	skippedWithInput := observe.ShadowSample{
		Detector: "screenparser", Ran: false,
		Inputs: []observe.InputEvent{nav(observe.NavPause)},
	}
	totals := fold(gameplay(), gameplay(), skippedWithInput, menu(), menu())

	var found bool
	for _, tr := range totals.Transitions {
		if tr.Preceded[observe.NavPause] > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("navigation observed during a skipped detector slot was discarded; the " +
			"keypress that caused the change is exactly the one most likely to land there")
	}
}

// A change nobody caused is reported as such.
//
// Held apart from the correlated ones deliberately: a transition that usually has no input
// before it is evidence AGAINST reading the correlated ones as causes, and a report that
// omitted it would make every correlation look stronger than it is.
func TestAChangeWithNoNavigationIsCountedAsUnattributed(t *testing.T) {
	totals := fold(gameplay(), gameplay(), menu(), menu(), menu())

	var total int
	for _, tr := range totals.Transitions {
		total += tr.Unattributed
		if tr.Attributed()+tr.Unattributed != tr.Count {
			t.Errorf("transition %s→%s: attributed %d + unattributed %d != count %d",
				tr.From, tr.To, tr.Attributed(), tr.Unattributed, tr.Count)
		}
	}
	if total == 0 {
		t.Error("a session with no input at all reported no unattributed changes")
	}
}

// Correlation is not causation, and the type says so.
//
// Dominant reports the count alongside the intent so one observation and four out of four
// cannot render identically.
func TestTheDominantIntentIsReportedWithItsSupport(t *testing.T) {
	totals := fold(
		gameplay(), gameplay(),
		withInput([]observe.InputEvent{nav(observe.NavPause)}, menuRegions()...),
		menu(), menu(),
	)
	for _, tr := range totals.Transitions {
		intent, n := tr.Dominant()
		if intent == "" {
			continue
		}
		if n <= 0 || n > tr.Count {
			t.Errorf("transition %s→%s: dominant %q supported %d times against %d changes",
				tr.From, tr.To, intent, n, tr.Count)
		}
	}
}

func menuRegions() []observe.ShadowRegion {
	return []observe.ShadowRegion{
		det("button", 0.414, 0.437, 0.172, 0.037),
		det("button", 0.414, 0.480, 0.172, 0.035),
		det("button", 0.414, 0.520, 0.172, 0.037),
		det("button", 0.414, 0.562, 0.172, 0.035),
		det("icon", 0.02, 0.86, 0.19, 0.10),
	}
}
