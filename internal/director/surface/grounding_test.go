package surface

// APPLYING ONE ANSWER LATER, AND REFUSING TO.
//
// The order is the design — observe, evaluate, select by relation, let the stored evidence
// contradict — so the tests are arranged by which of those steps says no, and the last group is
// the things a stored record must never be able to do.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// remembered is a durable store holding one target and whatever has been demonstrated about it.
//
// A fake because the real one opens a file, and `semanticmemory` has its own tests for the
// writing. What this stands in for is narrow on purpose: the two read-only methods `Repertoire`
// declares and nothing else.
type remembered struct {
	target   string
	verdict  observe.MatchVerdict
	surfaces []observe.DemonstratedSurface
	// asked records what was looked up, so a test can assert the signature was built the
	// canonical way rather than assembled here.
	asked []observe.StructureSignature
}

func (r *remembered) ResolveTarget(application string,
	sig observe.StructureSignature) observe.Recollection {

	r.asked = append(r.asked, sig)
	if r.target == "" {
		return observe.Recollection{Verdict: observe.MatchInsufficient}
	}
	v := r.verdict
	if v == "" {
		v = observe.MatchSame
	}
	return observe.Recollection{Verdict: v,
		Subject: observe.RememberedSubject{ID: r.target, Application: application,
			Structure: sig}}
}

func (r *remembered) DemonstratedSurfaces(string) []observe.DemonstratedSurface {
	return r.surfaces
}

// theRowWasDemonstrated is the record a settled press on the Trackmania menu row produces.
func theRowWasDemonstrated(c observe.SurfaceConfirmation) observe.DemonstratedSurface {
	q, ok := ask(nested(), "settings").Question()
	if !ok {
		panic("the nested fixture no longer produces a question")
	}
	a := q.Answer(at(theRow.X+20, theRow.Y+theRow.Height/2))
	d, err := q.Demonstrated(a, Given{Application: "fixture", Target: "subject-1",
		By: "chris", At: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC), Confirmed: c})
	if err != nil {
		panic(err)
	}
	return d
}

// groundIt runs grounding over the nested fixture with whatever memory holds.
func groundIt(mem Repertoire, w directorapi.WorldState) Grounded {
	return NewGrounder(mem).Ground(&w, "fixture", "place-1", "click settings", "click",
		directorapi.ElementQuery{Text: "settings"})
}

// ── WHAT A STORED ANSWER DOES ─────────────────────────────────────────────────

// A STORED DEMONSTRATION GROUNDS THE SAME REQUEST LATER.
//
// The milestone, end to end inside this package: the screen is read again, the candidates are
// rebuilt, and the one thing memory supplies is which of them the press belongs to.
func TestAStoredDemonstrationGroundsTheSameRequestLater(t *testing.T) {
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{
			theRowWasDemonstrated(attributedCrossing(t))}}

	// FIRST, WITHOUT IT: a question and no demonstration, which is where the person was
	// before they answered.
	cold := groundIt(&remembered{}, nested())
	if cold.Shown.Given() {
		t.Fatal("something was grounded with nothing in memory")
	}
	if cold.Question == nil {
		t.Fatal("with nothing in memory there is no question either")
	}

	g := groundIt(mem, nested())
	if !g.Shown.Given() {
		t.Fatalf("a stored demonstration grounded nothing: %s", g.Because)
	}
	// THE IDENTITY IS THE WORDS AND THE SURFACE IS THE ROW, which is the separation the
	// whole design rests on: the request named "SETTINGS" and the press goes elsewhere.
	if g.Shown.ElementID != "word" {
		t.Errorf("it vouched for %s, want the Prop the request named",
			g.Shown.ElementID)
	}
	if g.Shown.Surface != "row" {
		t.Errorf("the press was aimed at %s, want the row the person pressed",
			g.Shown.Surface)
	}
	// AND THE QUESTION IS NOT ASKED AGAIN.
	if g.Question != nil {
		t.Error("a grounded request still asks the person to show it")
	}
	// AND IT SAYS WHO, AND WHAT THEY SHOWED.
	for _, want := range []string{"chris", "15 Oct 2026", "SETTINGS",
		string(observe.RelationContainingRegion)} {
		if !strings.Contains(g.Shown.Because, want) {
			t.Errorf("the reason does not mention %q: %s", want, g.Shown.Because)
		}
	}
	// AND THE LOOKUP WENT THROUGH THE CANONICAL TARGET SIGNATURE, place included.
	if len(mem.asked) == 0 {
		t.Fatal("memory was never asked which target this is")
	}
	sig := mem.asked[0]
	if sig.Subject != observe.SubjectTarget || sig.Place != "place-1" ||
		sig.Label != "SETTINGS" {
		t.Errorf("the lookup signature is not the canonical one: %+v", sig)
	}
	t.Logf("%s", g.Shown.Because)
}

// ── WHAT REFUSES, ONE STEP AT A TIME ──────────────────────────────────────────

// AN UNCONFIRMED DEMONSTRATION DOES NOT AIM, AND THE QUESTION STANDS.
func TestAnUnconfirmedDemonstrationDoesNotAim(t *testing.T) {
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{
			theRowWasDemonstrated(pressOnly(t))}}
	g := groundIt(mem, nested())

	if g.Shown.Given() {
		t.Error("a press nothing confirmed was used to aim")
	}
	if g.Question == nil {
		t.Error("and the question was not asked again")
	}
	if !strings.Contains(g.Because, "observed") {
		t.Errorf("the reason does not say what was missing: %s", g.Because)
	}
}

// A TARGET MEMORY DOES NOT RECOGNISE EXACTLY IS NOT GROUNDED.
//
// `candidate` means memory thinks this MIGHT be a target it knows. Spending a saved aiming
// decision on a maybe is how it gets spent on the wrong control.
func TestOnlyAnExactRecollectionGrounds(t *testing.T) {
	for _, v := range []observe.MatchVerdict{
		observe.MatchCandidate, observe.MatchDifferent, observe.MatchInsufficient,
	} {
		mem := &remembered{target: "subject-1", verdict: v,
			surfaces: []observe.DemonstratedSurface{
				theRowWasDemonstrated(attributedCrossing(t))}}
		g := groundIt(mem, nested())
		if g.Shown.Given() {
			t.Errorf("a %q recollection grounded a press", v)
		}
		if g.Question == nil {
			t.Errorf("a %q recollection left no question either", v)
		}
	}
}

// A DEMONSTRATION ABOUT ANOTHER TARGET DOES NOT APPLY.
func TestADemonstrationAboutAnotherTargetDoesNotApply(t *testing.T) {
	other := theRowWasDemonstrated(attributedCrossing(t))
	other.Target = "subject-9"
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{other}}
	g := groundIt(mem, nested())

	if g.Shown.Given() {
		t.Error("a demonstration about another target grounded this one")
	}
	if !strings.Contains(g.Because, "nobody has demonstrated") {
		t.Errorf("the reason is not that nothing applies: %s", g.Because)
	}
}

// AND TWO RECORDS FOR ONE TARGET IS A CONTRADICTION, NOT A PREFERENCE.
func TestTwoDemonstrationsForOneTargetGroundNothing(t *testing.T) {
	one := theRowWasDemonstrated(attributedCrossing(t))
	two := one
	two.Relation = observe.RelationIdentityRegion
	two.Depth = 0
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{one, two}}
	g := groundIt(mem, nested())

	if g.Shown.Given() {
		t.Error("two contradicting records picked one")
	}
	if !strings.Contains(g.Because, "nothing here chooses") {
		t.Errorf("the reason does not name the contradiction: %s", g.Because)
	}
}

// A RELATION THAT NAMES NO SURFACE ON THIS SCREEN MISSES.
//
// Not "falls back to the nearest" — the nearest surface would be Marco deciding which one the
// person probably meant, which is the thing the question exists to avoid asking Marco.
func TestARelationThatNamesNoSurfaceNowMisses(t *testing.T) {
	deep := theRowWasDemonstrated(attributedCrossing(t))
	deep.Depth = 4
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{deep}}
	g := groundIt(mem, nested())

	if g.Shown.Given() {
		t.Errorf("a relation at depth 4 grounded on a screen with two surfaces: %s",
			g.Shown.Surface)
	}
	if !strings.Contains(g.Because, "no such surface") {
		t.Errorf("the reason does not say the surface is not there: %s", g.Because)
	}
	if g.Question == nil {
		t.Error("and the question was not asked again")
	}
}

// AND THE STORED EVIDENCE CAN CONTRADICT A SURFACE THE RELATION PICKED.
//
// The row is at depth 1 either way, so the relation still selects it. What refuses is the
// evidence: a surface that was 1.2% of the window and is now most of it is not the same thing,
// whatever its nesting says.
func TestStoredEvidenceCanContradictTheSurfaceItPicked(t *testing.T) {
	grown := stage(
		prop("word", "SETTINGS", theWord,
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("row", "", directorapi.Rect{X: 98, Y: 400, Width: 1000, Height: 500},
			directorapi.SourceStructure),
	)
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{
			theRowWasDemonstrated(attributedCrossing(t))}}
	g := groundIt(mem, grown)

	if g.Shown.Given() {
		t.Errorf("a surface that grew to %v was still pressed", g.Shown.Surface)
	}
	if !strings.Contains(g.Because, "disagrees") {
		t.Errorf("the reason does not say the screen disagrees: %s", g.Because)
	}
}

// NO DURABLE MEMORY STILL ASKS THE QUESTION.
//
// The first half of the interaction works with nothing stored at all, which is what makes this
// usable on a cold Director.
func TestWithNoMemoryAtAllTheQuestionIsStillAsked(t *testing.T) {
	w := nested()
	g := NewGrounder(nil).Ground(&w, "fixture", "place-1", "click settings", "click",
		directorapi.ElementQuery{Text: "settings"})
	if g.Shown.Given() {
		t.Error("something was grounded with no memory")
	}
	if g.Question == nil || !g.Question.Open() {
		t.Fatalf("no question without memory: %s", g.Because)
	}
	if !strings.Contains(g.Because, "no durable memory") {
		t.Errorf("the reason does not say memory is absent: %s", g.Because)
	}
}

// AND A SETTLED SURFACE NEEDS NO DEMONSTRATION AND IS NOT GIVEN ONE.
//
// A record must never be able to move a surface the evidence already settled. Memory holding a
// contradicting relation for such a target changes nothing.
func TestASettledSurfaceIsNotRegrounded(t *testing.T) {
	// The caption-only fixture, where exclusivity leaves exactly one survivor.
	settled := stage(
		prop("word", "SETTINGS", theWord, directorapi.SourceOCR),
		prop("row", "", theRow, directorapi.SourceStructure),
	)
	contradicting := theRowWasDemonstrated(attributedCrossing(t))
	contradicting.Relation = observe.RelationIdentityRegion
	contradicting.Depth = 0
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{contradicting}}
	g := groundIt(mem, settled)

	if g.Hypothesis.Decision != DecisionDefensible {
		t.Fatalf("the fixture is not settled: %s — %s", g.Hypothesis.Decision,
			g.Hypothesis.Explanation)
	}
	if g.Shown.Given() {
		t.Error("a stored record was spent on a surface the evidence had settled")
	}
	if g.Question != nil {
		t.Error("and a settled surface was put to the person as a question")
	}
}

// AND GROUNDING DOES NOT CONSULT ITSELF.
//
// The evaluator a grounder holds must resolve without being told about any demonstration. If it
// were the same resolver the pipeline hands a grounder's answer to, asking "is this demonstrated"
// would require resolving, which would require asking.
//
// Asserted structurally: a fresh grounder's resolver is a different object from one that could
// have been given a demonstration, and grounding a Prop that only a demonstration could admit
// still reports the Prop as rejected by the resolver inside it.
func TestGroundingDoesNotConsultItself(t *testing.T) {
	mem := &remembered{target: "subject-1",
		surfaces: []observe.DemonstratedSurface{
			theRowWasDemonstrated(attributedCrossing(t))}}
	g := groundIt(mem, nested())

	if g.Hypothesis.Identity == nil {
		t.Fatal("no identity at all")
	}
	// The identity is still REJECTED by the evaluator's own resolver — "not an interactive
	// control" — because that resolver was told nothing. The whole point of the grounder is
	// to produce something for a DIFFERENT resolver to be told.
	if g.Hypothesis.Identity.Rejected == "" {
		t.Error("the evaluator's resolver accepted a Prop that affords nothing, so it " +
			"has been told about a demonstration and this can recurse")
	}
	if !g.Shown.Given() {
		t.Fatal("and nothing was grounded, so the assertion above is vacuous")
	}
}
