package ambient_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// When repeated watching may become knowing.
//
// These drive the PURE policy over hand-built summaries. Nothing here touches a store, a session
// or a desktop — the whole reason folding and judging are separate functions is that the rule can
// be stated and attacked without any of that. See ADR-095.

// screen is a structural signature distinctive enough to be established and told apart.
func screen(members int, terms ...observe.InterfaceTerm) observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Members: members, TermsKnown: true,
		Roles: map[string]int{"button": members, "panel": 1}, Terms: terms,
	}
}

// known is an endpoint Marco already recognises.
func known(id string) observe.WatchedEnd { return observe.WatchedEnd{Subject: id} }

// seenOnce is the summary one clean crossing produces.
func seenOnce(at time.Time) observe.WatchedEdge {
	return ambient.Fold(observe.WatchedEdge{
		Application: app, From: known("subj_home"), To: known("subj_bt"),
		Kind: string(ambient.Activate), Target: "Bluetooth & devices", Role: "button",
	}, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman, At: at,
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)
}

// learning is the policy with ambient learning switched on and everything else default.
var learning = ambient.Policy{Enabled: true}

// ── A: one demonstration is not a habit ───────────────────────────────────────

// ONE CLEAN TRAVERSAL IS ALREADY KNOWLEDGE.
//
// # The correction this test exists to hold
//
// It used to say the opposite: two independent demonstrations, on the reasoning that "one is not
// evidence of a habit". That is right about habits and this is not a habit store — it is a graph
// of what the interface IS. Somebody went from A to B by pressing X and arrived; the relationship
// is a fact about the world, and waiting for a repetition is waiting for them to prove that a door
// they just walked through is still a door.
//
// What repetition buys is CONFIDENCE, which lives in the evidence. What it must not be is the
// admission ticket for the relationship's existence.
//
// Raising the default back to two must fail this.
func TestOneCleanTraversalIsAlreadyKnowledge(t *testing.T) {
	e := seenOnce(now())
	if e.Seen != 1 {
		t.Fatalf("the fixture was traversed %d time(s), want 1", e.Seen)
	}
	j := ambient.Judge(e, learning, now())
	if j.Verdict != ambient.Promote || j.Why != ambient.WhyEnough {
		t.Fatalf("verdict %q/%q, want promote/%q — a clean traversal is a fact about the "+
			"interface, not a habit somebody has to prove", j.Verdict, j.Why, ambient.WhyEnough)
	}
}

// AND A DEPLOYMENT MAY STILL ASK FOR CORROBORATION.
//
// The default is one; the policy has the knob. Kept configurable so a deployment that wants a
// second traversal before admission can have it, and so the refusal it produces stays reachable
// and tested rather than being a word nothing can say.
func TestAPolicyMayStillAskForCorroboration(t *testing.T) {
	cautious := ambient.Policy{Enabled: true, Traversals: 2}
	e := seenOnce(now())

	j := ambient.Judge(e, cautious, now())
	if j.Verdict != ambient.Wait || j.Why != ambient.WhyTooFew || j.Short != 1 {
		t.Fatalf("verdict %q/%q short %d, want wait/%q short 1",
			j.Verdict, j.Why, j.Short, ambient.WhyTooFew)
	}
	e.Seen++
	if j := ambient.Judge(e, cautious, now()); j.Verdict != ambient.Promote {
		t.Errorf("a second traversal did not satisfy a policy asking for two: %+v", j)
	}
}

// ── B: one action is one demonstration, however many times it is sampled ──────

// A FAST SECOND TRAVERSAL IS A SECOND TRAVERSAL.
//
// # The ceremony this removes
//
// The old rule made two crossings count as one unless a minute had passed between them, so a
// person who did a thing, went back, and did it again fifteen seconds later was credited with one.
// The clock was standing in for a hazard it did not guard: a thousand provider samples of ONE
// action reading as a thousand demonstrations.
//
// That hazard is handled twice, upstream, where it belongs. Duplicate input events are coalesced
// into one act, and a crossing is recorded only when the PLACE CHANGES — so a screen sampled forty
// times produces no traversal at all. To traverse A --X--> B a second time you have to get back to
// A, and getting back to A is itself a recorded step. Semantic re-entry is what makes a second
// traversal second; the clock never was.
//
// Reintroducing a minimum gap must fail this.
func TestAFastSecondTraversalIsASecondTraversal(t *testing.T) {
	at := now()
	e := observe.WatchedEdge{Application: app, From: known("subj_home"), To: known("subj_bt"),
		Kind: string(ambient.Activate), Target: "Bluetooth & devices", Role: "button"}
	for i := 0; i < 6; i++ {
		e = ambient.Fold(e, ambient.Step{
			From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
			At:  at.Add(time.Duration(i) * 5 * time.Second),
			Did: []ambient.Act{press("Bluetooth & devices")},
		}, true)
	}
	if e.Seen != 6 {
		t.Fatalf("%d traversals recorded from six crossings five seconds apart, want 6. "+
			"Somebody who did a thing six times did it six times.", e.Seen)
	}
	// AND IT IS ONE EDGE, however often it was taken.
	if e.Sessions != 1 {
		t.Errorf("%d watching sessions, want 1: they all happened in one", e.Sessions)
	}
}

// AND A DIFFERENT WATCHING SESSION IS PROVENANCE, NOT A THRESHOLD.
//
// "Twice in a minute" and "twice across a restart" are different strengths of the same fact, and
// the second is worth recording: it has survived a different window generation and very often a
// different day. It gates nothing — the edge was already knowledge after the first traversal.
func TestADifferentSessionIsProvenanceRatherThanAThreshold(t *testing.T) {
	at := now()
	e := seenOnce(at)
	if j := ambient.Judge(e, learning, at); j.Verdict != ambient.Promote {
		t.Fatal("the first traversal did not already qualify, so this proves nothing")
	}
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
		At: at.Add(time.Second), Did: []ambient.Act{press("Bluetooth & devices")},
	}, false)

	if e.Sessions != 2 {
		t.Errorf("%d watching sessions after a boundary, want 2", e.Sessions)
	}
	if e.Seen != 2 {
		t.Errorf("%d traversals, want 2", e.Seen)
	}
}

// ── C: the same edge again ────────────────────────────────────────────────────

// THE SAME THING AGAIN, LATER, IS STILL ONE EDGE.
//
// A second clean crossing of a relationship that already qualified: the counts move, the verdict
// does not, and there is never a second edge to have a second verdict about.
func TestTheSameThingAgainLaterIsRemembered(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
		At:  at.Add(2 * time.Second),
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)

	if e.Seen != 2 {
		t.Fatalf("%d traversals, want 2", e.Seen)
	}
	j := ambient.Judge(e, learning, at)
	if j.Verdict != ambient.Promote || j.Why != ambient.WhyEnough {
		t.Fatalf("verdict %q/%q, want promote/%q", j.Verdict, j.Why, ambient.WhyEnough)
	}
}

// AND A DIFFERENT WATCHING SESSION IS ALWAYS A DIFFERENT OCCASION.
//
// The other meaning of "again": next time. A session boundary is a restart of the observer and
// often of the day, and requiring a clock gap on top of it would refuse somebody who did the same
// thing twice a few seconds either side of one.
func TestADifferentSessionIsADifferentOccasion(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
		At: at.Add(time.Second), Did: []ambient.Act{press("Bluetooth & devices")},
	}, false)

	if e.Seen != 2 {
		t.Fatalf("%d occasions across a session boundary, want 2", e.Seen)
	}
}

// ── D: contradiction ──────────────────────────────────────────────────────────

// ONE CONTROL THAT LEADS TWO WAYS IS NOT UNDERSTOOD.
//
// The same screen, the same button, arriving somewhere else. A majority is not an answer here:
// what it means is that Marco does not understand the screen — a mode, a state, something it
// cannot see — and promoting the more frequent destination would be a coin toss dressed as
// knowledge.
//
// It is NEVER rather than WAIT, because more of the same evidence deepens the problem rather than
// settling it. Explicit Learn is the way through, because a person saying what they mean is
// information that a repetition is not.
//
// Ignoring contradictions must fail this.
func TestOneControlThatLeadsTwoWaysIsNeverLearned(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
		At:  at.Add(2 * time.Second),
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)
	if ambient.Judge(e, learning, at).Verdict != ambient.Promote {
		t.Fatal("the fixture does not promote without the contradiction, so this proves nothing")
	}

	e = ambient.Contradict(e, at.Add(3*time.Second))
	j := ambient.Judge(e, learning, at)
	if j.Verdict != ambient.Never || j.Why != ambient.WhyContradicted {
		t.Fatalf("verdict %q/%q, want never/%q — a button that leads two ways was learned "+
			"anyway", j.Verdict, j.Why, ambient.WhyContradicted)
	}
	// AND MORE OF THE SAME DOES NOT FIX IT.
	for i := 0; i < 5; i++ {
		e = ambient.Fold(e, ambient.Step{
			From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
			At:  at.Add(time.Duration(4+i) * time.Second),
			Did: []ambient.Act{press("Bluetooth & devices")},
		}, true)
	}
	if ambient.Judge(e, learning, at).Verdict != ambient.Never {
		t.Error("repetition resolved a contradiction it cannot resolve")
	}
}

// ── F/G handled where evidence is collected; H/I structurally ─────────────────

// AN ACTION MARCO CANNOT NAME IS NEVER LEARNED.
//
// A press on a control whose name was withheld — the ordinary passive case for a role outside the
// plaintext allowlist — cannot become a play step however often it recurs: there is no way to say
// what to press. NEVER rather than WAIT, because the name is not going to arrive by repetition.
func TestAnActionMarcoCannotNameIsNeverLearned(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e.Target, e.Role = "", "listitem"
	e.Seen = 9

	j := ambient.Judge(e, learning, at)
	if j.Verdict != ambient.Never || j.Why != ambient.WhyUnnamedTarget {
		t.Fatalf("verdict %q/%q, want never/%q", j.Verdict, j.Why, ambient.WhyUnnamedTarget)
	}
}

// AN ACTION WORD THIS MARCO DOES NOT KNOW IS NEVER LEARNED.
//
// # Why this is reachable rather than defensive
//
// The candidate ledger is DURABLE, so a summary written by a later Marco — one whose action
// vocabulary has grown a drag, a scroll, a typed value — is read by this one. The closed
// vocabulary is checked on the way out of the store as well as on the way in, so an action word
// this version cannot represent is refused rather than lowered into the nearest thing that fits.
//
// A drag flattened into an activation would be a lie about what somebody did.
func TestAnActionWordThisMarcoDoesNotKnowIsNeverLearned(t *testing.T) {
	at := now()
	for _, kind := range []string{"drag", "scroll", "typed", ""} {
		e := seenOnce(at)
		e.Kind, e.Seen = kind, 9
		j := ambient.Judge(e, learning, at)
		if j.Verdict != ambient.Never || j.Why != ambient.WhyUnsupported {
			t.Errorf("%q: verdict %q/%q, want never/%q",
				kind, j.Verdict, j.Why, ambient.WhyUnsupported)
		}
	}
}

// A SCREEN NOTHING COULD ESTABLISH IS NOT AN ENDPOINT — YET.
//
// WAIT rather than NEVER, and the difference matters: memory improves, and a screen Marco cannot
// describe today may be one it recognises tomorrow. Nothing about this judgement is written into
// the record, so the same evidence is re-judged every time it is asked about — the same discipline
// [[ADR-021-a-judgement-is-recomputed-not-recorded]] applies everywhere else here.
func TestAScreenNothingCouldEstablishIsNotAnEndpointYet(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e.Seen = 9
	// Describable at all, and carrying no discriminator — so it could never be recognised
	// again, which is what the place store itself refuses.
	bare := observe.StructureSignature{Subject: observe.SubjectState, Members: 3}
	e.To = observe.WatchedEnd{Shape: &bare}

	j := ambient.Judge(e, learning, at)
	if j.Verdict != ambient.Wait || j.Why != ambient.WhyUnknownPlace {
		t.Fatalf("verdict %q/%q, want wait/%q", j.Verdict, j.Why, ambient.WhyUnknownPlace)
	}

	// And the same evidence, once the screen is describable, is enough.
	sig := screen(6, observe.TermSettings)
	e.To = observe.WatchedEnd{Shape: &sig, Called: "Bluetooth & devices"}
	if j := ambient.Judge(e, learning, at); j.Verdict != ambient.Promote {
		t.Errorf("a describable screen was still refused: %+v", j)
	}
}

// ── the switch ────────────────────────────────────────────────────────────────

// WATCHING WITHOUT LEARNING REMEMBERS NOTHING, HOWEVER MUCH IT SEES.
//
// The separation this roadmap exists to make expressible. A policy that is switched off has no
// opinion about somebody's evidence at all: it does not say "not seen often enough", because that
// would be a sentence about the wrong thing.
//
// Deleting the Enabled check must fail this.
func TestWatchingWithoutLearningRemembersNothing(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e.Seen = 99

	j := ambient.Judge(e, ambient.Policy{}, at)
	if j.Verdict == ambient.Promote {
		t.Fatal("evidence was learned while learning was switched off")
	}
	if j.Why != ambient.WhyDisabled {
		t.Errorf("the reason is %q; a switched-off policy has no opinion about the evidence "+
			"and saying otherwise is a sentence about the wrong thing", j.Why)
	}
}

// ── after promotion ───────────────────────────────────────────────────────────

// WHAT IS ALREADY KNOWN IS NOT LEARNED AGAIN.
//
// After a candidate becomes durable knowledge, further sightings strengthen the record rather than
// producing a second admission of the same relationship. NEVER, and its own reason, so a
// diagnostic can tell "already yours" from "not yet".
func TestWhatIsAlreadyKnownIsNotLearnedAgain(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e.Seen, e.Promoted = 9, at

	j := ambient.Judge(e, learning, at.Add(time.Hour))
	if j.Verdict != ambient.Never || j.Why != ambient.WhyAlready {
		t.Fatalf("verdict %q/%q, want never/%q", j.Verdict, j.Why, ambient.WhyAlready)
	}
}

// AND FURTHER SIGHTINGS STRENGTHEN THE RECORD RATHER THAN STARTING A SECOND ONE.
func TestSightingsAfterPromotionStrengthenTheSameRecord(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e.Promoted = at
	before := e.Seen

	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
		At: at.Add(time.Hour), Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)
	if e.Seen != before+1 {
		t.Errorf("traversed %d time(s) after another one, want %d", e.Seen, before+1)
	}
	if e.Promoted.IsZero() {
		t.Error("a further sighting erased the record of when this became knowledge, so " +
			"nothing can explain where the knowledge came from")
	}
}

// ── recency, when a policy asks for it ────────────────────────────────────────

// EVIDENCE THE POLICY CALLS STALE IS NOT ACTED ON.
//
// Off by default — the first policy has no recency bound, because a thing somebody does twice a
// year is still a thing they do, and the durable store's own semantics decide validity after
// admission. Reachable through configuration, and tested through it, so the field is a rule
// rather than a decoration.
func TestEvidenceThePolicyCallsStaleIsNotActedOn(t *testing.T) {
	at := now()
	e := seenOnce(at)
	e.Seen = 2

	fussy := ambient.Policy{Enabled: true, Freshness: 24 * time.Hour}
	if j := ambient.Judge(e, fussy, at.Add(48*time.Hour)); j.Why != ambient.WhyStale {
		t.Fatalf("reason %q, want %q", j.Why, ambient.WhyStale)
	}
	if j := ambient.Judge(e, fussy, at.Add(time.Hour)); j.Verdict != ambient.Promote {
		t.Errorf("fresh evidence was called stale: %+v", j)
	}
	// AND THE DEFAULT POLICY HAS NO SUCH BOUND.
	if j := ambient.Judge(e, learning, at.Add(365*24*time.Hour)); j.Verdict != ambient.Promote {
		t.Errorf("the default policy applied a recency bound it does not have: %+v", j)
	}
}

// ── identity ──────────────────────────────────────────────────────────────────

// A CANDIDATE'S HANDLE IS ITS SEMANTIC IDENTITY AND NOTHING ELSE.
//
// Same relationship, two different afternoons, different counts, a different reading of what the
// screen is called: one handle. If any of those reached the id, cross-session evidence would never
// add up — every sighting would be a new candidate and nothing would ever be seen twice.
func TestACandidatesHandleIsItsIdentityAndNothingElse(t *testing.T) {
	sig := screen(6, observe.TermSettings)
	monday := observe.WatchedEnd{Shape: &sig, Called: "Home"}
	tuesday := observe.WatchedEnd{Shape: &sig, Called: "Home page"}
	to := known("subj_bt")

	a := observe.WatchedID(app, monday, to, string(ambient.Activate), "Bluetooth & devices")
	b := observe.WatchedID(app, tuesday, to, string(ambient.Activate), "bluetooth & devices")
	if a != b {
		t.Errorf("two sightings of one relationship got two handles:\n %s\n %s", a, b)
	}
	// And a genuinely different relationship does not collide.
	other := observe.WatchedID(app, monday, known("subj_network"),
		string(ambient.Activate), "Bluetooth & devices")
	if other == a {
		t.Error("two different relationships share a handle")
	}
}

// A RECOGNISED SCREEN REPLACES THE DESCRIPTION IT WAS STANDING IN FOR.
//
// Memory improves between two sightings — an explicit Learn elsewhere, a place established on
// another route — and a candidate that went on carrying the shape it was first seen as would ask a
// promotion to establish a place that already exists.
func TestARecognisedScreenReplacesItsDescription(t *testing.T) {
	at := now()
	sig := screen(6, observe.TermSettings)
	e := ambient.Fold(observe.WatchedEdge{Application: app,
		Kind: string(ambient.Activate), Target: "Bluetooth & devices", Role: "button"},
		ambient.Step{
			From: "seen_state_1", To: "subj_bt", Application: app, By: ambient.ByHuman,
			FromShape: &ambient.Shape{Signature: sig, Called: "Home"}, At: at,
			Did: []ambient.Act{press("Bluetooth & devices")},
		}, true)
	if e.From.Recognised() || e.From.Shape == nil {
		t.Fatalf("the first sighting did not describe the unknown screen: %+v", e.From)
	}

	// The same walk again, and by now Marco knows where it starts.
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman,
		At:  at.Add(2 * time.Second),
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)
	if !e.From.Recognised() || e.From.Subject != "subj_home" {
		t.Fatalf("the candidate still describes a screen Marco now recognises: %+v", e.From)
	}
	if e.From.Shape != nil {
		t.Error("it carries a description beside a durable identity, which is the start of " +
			"a duplicate")
	}
}

// ── eviction ──────────────────────────────────────────────────────────────────

// EVICTION FORGETS THE WEAKEST FIRST, AND IT IS NEVER A COIN TOSS.
//
// A store that evicted by insertion order would forget a candidate one sighting from promotion in
// favour of a thing somebody did once this morning. Every tie breaks on something, so two runs
// over the same evidence forget the same things.
func TestEvictionOrdersCandidatesDeterministically(t *testing.T) {
	at := now()
	mk := func(id string, seen, sessions, contradicted int, promoted bool) observe.WatchedEdge {
		e := observe.WatchedEdge{ID: id, Seen: seen, Sessions: sessions,
			Contradicted: contradicted, Last: at}
		if promoted {
			e.Promoted = at
		}
		return e
	}
	// EVERY FIXTURE LOSES ON THE CRITERION UNDER TEST AND WINS ON ALL THE OTHERS.
	//
	// A pair that agrees on two criteria at once cannot say which one decided, and the
	// comparison passes with the rule under test deleted. Measured: `promoted is strongest`
	// and `more sightings breaks a tie` both did exactly that, and a mutation run that
	// removed the promoted arm and the sessions arm survived this test twice.
	promoted := mk("d", 1, 1, 0, true)
	strong := mk("c", 1, 4, 0, false)
	weak := mk("z", 1, 1, 0, false)
	contradicted := mk("a", 9, 9, 1, false)

	for _, c := range []struct {
		name     string
		weaker   observe.WatchedEdge
		stronger observe.WatchedEdge
	}{
		// The promoted one is behind on sightings, sessions and id, and still strongest:
		// it is the provenance of durable knowledge, and losing it would leave that
		// knowledge unable to say where it came from.
		{"promoted is strongest", strong, promoted},
		// The contradicted one leads on everything countable, and goes first anyway.
		{"contradicted goes early", contradicted, weak},
		{"promoted beats an ordinary tie", weak, promoted},
		// Equal sightings, and the id order runs the OTHER way, so only sessions can
		// decide this.
		{"more sessions breaks a tie", weak, strong},
	} {
		if !c.weaker.WeakerThan(c.stronger) {
			t.Errorf("%s: %q was not weaker than %q", c.name, c.weaker.ID, c.stronger.ID)
		}
		if c.stronger.WeakerThan(c.weaker) {
			t.Errorf("%s: the order is not antisymmetric", c.name)
		}
	}
	// AND A TIE ON EVERYTHING STILL BREAKS, so eviction never depends on slice order.
	x, y := mk("x", 1, 1, 0, false), mk("y", 1, 1, 0, false)
	// Identical in every field the order consults except the handle itself.
	if x.WeakerThan(y) == y.WeakerThan(x) {
		t.Error("two identical candidates cannot be ordered, so which one is forgotten " +
			"depends on how the store happened to be laid out")
	}
}

// ── the summary holds no words anybody could read ─────────────────────────────

// CANDIDATE EVIDENCE IS A SUMMARY, NOT A RECORDING.
//
// The first thing in the ambient path that survives a restart, and therefore the place a privacy
// boundary would be easiest to lose. Counts, times, a semantic identity, a structure and the two
// words the interface put on things. No screenshot, no tree, no trajectory, no typed text, no
// coordinate that means anything off the window it came from.
func TestCandidateEvidenceIsASummaryNotARecording(t *testing.T) {
	sig := screen(6, observe.TermSettings)
	e := observe.WatchedEdge{
		Application: app, From: observe.WatchedEnd{Shape: &sig, Called: "Home"},
		To: known("subj_bt"), Kind: string(ambient.Activate), Target: "Bluetooth & devices",
	}
	rendered := renderDeep(e)
	for _, leak := range []string{
		"Screenshot", "Pixels", "Trajectory", "Keystroke", "Typed", "Clipboard",
		"Password", "Secret", "Title", "Tree",
	} {
		if contains(rendered, leak) {
			t.Errorf("candidate evidence carries %q: %s", leak, rendered)
		}
	}
	// A screen's signature has fields that belong to a TARGET's identity — a label, a kind,
	// a place — and they are empty for a screen. A change that started filling them in would
	// put text off somebody's display into a durable record.
	if e.From.Shape.Label != "" || e.From.Shape.Kind != "" || e.From.Shape.Place != "" {
		t.Errorf("a screen's shape carries a target's identity fields: %+v", e.From.Shape)
	}
}

// renderDeep renders a value including what its pointers point at, so a leak inside a signature
// cannot hide behind an address.
func renderDeep(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return fmt.Sprintf("%+v %s", v, b)
}

// ── what a fold must not lose ─────────────────────────────────────────────────

// NOTHING IS PROMOTED ON NO EVIDENCE AT ALL.
//
// A summary with zero traversals is not a thing anybody watched — it is an empty record, and the
// only way one reaches the policy is a bug upstream or a store written by a different Marco. The
// rule that "one is enough" has to mean one, not "any number including none", and a threshold of
// zero would let an empty row through with nothing behind it.
//
// Setting DefaultTraversals to zero must fail this.
func TestNoTraversalsIsNotOneTraversal(t *testing.T) {
	e := observe.WatchedEdge{
		Application: app, From: known("subj_home"), To: known("subj_bt"),
		Kind: string(ambient.Activate), Target: "Bluetooth & devices", Role: "button",
	}
	if e.Seen != 0 {
		t.Fatalf("the fixture already has %d traversal(s)", e.Seen)
	}
	if j := ambient.Judge(e, learning, now()); j.Verdict != ambient.Wait {
		t.Errorf("a summary nobody has traversed says %q/%q, want wait", j.Verdict, j.Why)
	}
}

// WHEN IT WAS FIRST TAKEN IS KEPT, AND ONLY THE LAST TIME MOVES.
//
// The two together are the only thing that tells "twelve times since Tuesday" from "twelve times
// this afternoon" — the counts cannot, and neither can either timestamp alone. A first-seen that
// moved with every crossing would silently make every relationship look like it was discovered
// moments ago, which is exactly the reading somebody would use to decide whether Marco has known
// something long enough to be trusted about it.
//
// Assigning First unconditionally must fail this.
func TestWhenARelationshipWasFirstTakenIsKept(t *testing.T) {
	at := now()
	e := seenOnce(at)
	if !e.First.Equal(at) {
		t.Fatalf("first sighting is %v, want %v", e.First, at)
	}
	later := at.Add(72 * time.Hour)
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "subj_bt", Application: app, By: ambient.ByHuman, At: later,
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, false)

	if !e.First.Equal(at) {
		t.Errorf("first-seen moved to %v when the relationship was taken again three days "+
			"later. Marco can no longer tell a way somebody has used all week from one "+
			"they found this minute.", e.First)
	}
	if !e.Last.Equal(later) {
		t.Errorf("last-seen is %v, want %v", e.Last, later)
	}
}

// THE FIRST DESCRIPTION OF A SCREEN IS THE ONE THAT IS KEPT.
//
// Two readings of one screen differ in small ways — that is why the canonical matcher has
// tolerances at all. Overwriting the description on every sighting makes the candidate's own
// content move underneath it, and the endpoint the promotion eventually establishes is then
// whichever reading happened to be last rather than the one the evidence accumulated against.
//
// Any of them would do; what must not happen is that it CHANGES.
//
// Overwriting on every sighting must fail this.
func TestTheFirstDescriptionOfAScreenIsTheOneKept(t *testing.T) {
	at := now()
	wide := screen(7, observe.TermSettings, observe.TermAudio)
	narrow := screen(5, observe.TermSettings)

	e := ambient.Fold(observe.WatchedEdge{
		Application: app, Kind: string(ambient.Activate),
		Target: "Bluetooth & devices", Role: "button",
	}, ambient.Step{
		From: "seen_state_1", To: "seen_state_2", Application: app, By: ambient.ByHuman,
		At: at, ToShape: &ambient.Shape{Signature: wide},
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)
	if e.To.Shape == nil || e.To.Shape.Members != wide.Members {
		t.Fatalf("the first sighting did not describe the destination: %+v", e.To)
	}
	e = ambient.Fold(e, ambient.Step{
		From: "seen_state_1", To: "seen_state_2", Application: app, By: ambient.ByHuman,
		At: at.Add(time.Minute), ToShape: &ambient.Shape{Signature: narrow},
		Did: []ambient.Act{press("Bluetooth & devices")},
	}, true)

	if e.To.Shape == nil || e.To.Shape.Members != wide.Members {
		t.Errorf("a second reading replaced the description: %d members, want %d. The "+
			"evidence accumulated against one screen and the promotion would establish "+
			"another.", e.To.Shape.Members, wide.Members)
	}
}

// AND A SCREEN THAT HAS AN IDENTITY NEVER GOES BACK TO BEING A SHAPE.
//
// Memory only improves. Once an end is a durable subject, a later sighting that could only
// describe it must not overwrite the identity with the description — that would ask a promotion to
// establish a place that already exists, and the graph would fork into two copies of one screen
// with the edges divided between them.
//
// Deleting the guard must fail this.
func TestAnIdentifiedScreenNeverGoesBackToBeingAShape(t *testing.T) {
	at := now()
	e := seenOnce(at)
	if e.To.Subject != "subj_bt" {
		t.Fatalf("the fixture's destination is not identified: %+v", e.To)
	}
	e = ambient.Fold(e, ambient.Step{
		From: "subj_home", To: "seen_state_2", Application: app, By: ambient.ByHuman,
		At: at.Add(time.Minute), Did: []ambient.Act{press("Bluetooth & devices")},
		ToShape: &ambient.Shape{Signature: screen(9, observe.TermSettings)},
	}, true)

	if e.To.Subject != "subj_bt" {
		t.Errorf("the destination lost its identity and became %+v. A promotion would now "+
			"establish a second copy of a screen Marco already knows.", e.To)
	}
	if e.To.Shape != nil {
		t.Errorf("an identified end is also carrying a description: %+v", e.To.Shape)
	}
}
