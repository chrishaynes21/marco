package resolveroute

// WHAT THE REQUEST ALREADY SAID ABOUT A ROUTINE'S ARGUMENTS.
//
// A proposal and nothing more: every test here is about what gets SHOWN beside a routine before
// anybody presses anything. Nothing in this file authorises a run, and the two rules that keep it
// that way are that a proposal travels only with an identity, and that a secret is never one.
//
// See [[Extracting-argument-values-proposal-2026-10-30]].

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// recordingExtractor says what it was asked and answers with the spans it was told to.
type recordingExtractor struct {
	asked   int
	input   string
	names   []string
	spans   []resolver.Span
	ok      bool
	routine string
}

func (e *recordingExtractor) extract(_ context.Context, input, routine string,
	names []string) ([]resolver.Span, bool) {

	e.asked++
	e.input, e.names, e.routine = input, names, routine
	return e.spans, e.ok
}

// takingText is one routine that declares arguments, and the resolver that will select it.
func takingText(t *testing.T, e *recordingExtractor, takes ...routes.Argument) (Resolver,
	[]Candidate) {

	t.Helper()
	c := aCandidate("ark-graphics", "arkascended", true)
	c.Takes, c.SourceAt = takes, "sha256:abc"
	r := Resolver{
		Reg: routes.Registry{Dir: t.TempDir()},
		Advisor: &recordingAdvisor{ok: true, offers: dispatch.Decision{
			Intent: dispatch.IntentRun, Route: "ark-graphics",
			Source: dispatch.SourceAdvisor}},
	}
	if e != nil {
		r.Extract = e.extract
	}
	return r, []Candidate{c}
}

func text(name string) routes.Argument   { return routes.Argument{Name: name} }
func secret(name string) routes.Argument { return routes.Argument{Name: name, Secret: true} }

const request = "set ark graphics to epic"

// A VALUE THE REQUEST ALREADY CARRIES IS PROPOSED BESIDE THE ROUTINE.
func TestARequestsOwnWordsAreProposedForTheArgument(t *testing.T) {
	// "epic" is [20, 24) in the request above.
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs := takingText(t, e, text("quality"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if said.Kind != dispatch.IntentRun || said.ID != "arkascended/focus/ark-graphics" {
		t.Fatalf("the routine was not selected: %+v", said)
	}
	if len(said.Values) != 1 {
		t.Fatalf("the proposals are %+v", said.Values)
	}
	got := said.Values[0]
	if got.Name != "quality" || got.Value != "epic" || got.From != routes.FromExtracted {
		t.Errorf("the proposal is %+v", got)
	}
	// AND THE VERSION THE NAMES WERE READ FROM TRAVELS WITH IT, so a routine saved again
	// before the confirmation is prepared can be refused rather than crossed into.
	if said.SourceAt != "sha256:abc" {
		t.Errorf("the proposal names the revision %q", said.SourceAt)
	}
	// AND WHAT WAS ASKED IS THE REQUEST AND THE NAMES. The request as the policy holds it,
	// trimmed, because the spans index THAT string and a value read out of a different one
	// is a value off by however much whitespace somebody typed.
	if e.input != request {
		t.Errorf("the question carried %q", e.input)
	}
	if strings.Join(e.names, ",") != "quality" {
		t.Errorf("the question asked about %v", e.names)
	}
}

// AND THE PROVENANCE WORD IS THE ONE A SURFACE SHOWS.
//
// Asserted against the literal, because every other test here compares with the constant and
// would read the same if the constant said something else. What a person is told about where a
// value came from is the whole reason the field exists.
func TestTheProvenanceOfAProposedValueIsSaidAsExtracted(t *testing.T) {
	if routes.FromExtracted != "extracted" {
		t.Errorf("a proposed value is reported as %q", routes.FromExtracted)
	}
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs := takingText(t, e, text("quality"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))
	b, err := json.Marshal(said.Values)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `[{"name":"quality","value":"epic","from":"extracted"}]` {
		t.Errorf("the proposal travels as %s", b)
	}
}

// AND AN ANSWER WHOSE EVERY SPAN WAS DROPPED NAMES NO REVISION.
//
// The one path where the resolver said yes and nothing usable came of it. A revision carried with
// no values is a confirmation prepared for a routine nobody proposed anything for.
func TestAnAnswerWithNothingUsableInItNamesNoRevision(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "preset", Start: 20, End: 24}}}
	r, cs := takingText(t, e, text("quality"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if len(said.Values) != 0 {
		t.Errorf("a span for an undeclared name was proposed: %+v", said.Values)
	}
	if said.SourceAt != "" {
		t.Errorf("nothing usable still named the revision %q", said.SourceAt)
	}
}

// A SECRET IS NEVER ASKED ABOUT.
//
// The hardest rule here. A secret argument resolves from the credential store at run time, and
// the reason is that a password must not pass through a routine, a recording or a diagnostic on
// its way to being typed. A request is worse than all three: asking would send somebody's
// password to a model, and answering would carry it back and put it in a field on a card.
func TestASecretIsNeverAskedAboutAndNeverProposed(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "password", Start: 20, End: 24}}}
	r, cs := takingText(t, e, secret("password"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if said.Kind != dispatch.IntentRun {
		t.Fatalf("the routine was not selected: %+v", said)
	}
	// NOTHING WAS ASKED AT ALL, which is the claim: a routine whose only argument is a
	// secret produces no question, so there is no answer to have to refuse.
	if e.asked != 0 {
		t.Errorf("a secret was sent to the resolver: %q about %v", e.input, e.names)
	}
	if len(said.Values) != 0 {
		t.Errorf("a secret was proposed: %+v", said.Values)
	}
}

// AND A SECRET BESIDE A TEXT ARGUMENT IS LEFT OUT OF THE QUESTION, not filtered out of the answer.
func TestASecretIsLeftOutOfAQuestionItSharesWithText(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs := takingText(t, e, text("quality"), secret("password"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if strings.Join(e.names, ",") != "quality" {
		t.Errorf("the question asked about %v", e.names)
	}
	if len(said.Values) != 1 || said.Values[0].Name != "quality" {
		t.Errorf("the proposals are %+v", said.Values)
	}
}

// AND A SPAN FOR A SECRET, ANSWERED ANYWAY, IS NOT A PROPOSAL.
//
// Belt and braces on purpose: `resolver.Extract` already drops a name nobody asked about, and
// this is the layer that knows WHY one was not asked. A plugin answering a name it was not given
// is the shape of a model inventing a field, and this is the field where that would matter most.
func TestASpanNamingASecretIsNotProposed(t *testing.T) {
	e := &recordingExtractor{ok: true, spans: []resolver.Span{
		{Name: "quality", Start: 20, End: 24},
		{Name: "password", Start: 4, End: 7},
	}}
	r, cs := takingText(t, e, text("quality"), secret("password"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	for _, p := range said.Values {
		if p.Name == "password" {
			t.Errorf("a secret came back as a proposal: %+v", p)
		}
	}
	if len(said.Values) != 1 {
		t.Errorf("the proposals are %+v", said.Values)
	}
}

// PROPOSALS ARE IN THE ORDER THE ROUTINE DECLARES, whatever order they were answered in.
func TestProposalsAreInTheOrderTheRoutineDeclares(t *testing.T) {
	e := &recordingExtractor{ok: true, spans: []resolver.Span{
		{Name: "quality", Start: 20, End: 24},
		{Name: "where", Start: 4, End: 7},
	}}
	r, cs := takingText(t, e, text("where"), text("quality"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if len(said.Values) != 2 {
		t.Fatalf("the proposals are %+v", said.Values)
	}
	if said.Values[0].Name != "where" || said.Values[1].Name != "quality" {
		t.Errorf("the proposals arrived as %+v", said.Values)
	}
	if said.Values[0].Value != "ark" || said.Values[1].Value != "epic" {
		t.Errorf("the values are %+v", said.Values)
	}
}

// AN ARGUMENT NOTHING ANSWERED FOR SIMPLY HAS NO PROPOSAL.
//
// It is MISSING, and missing is the case the existing field and the existing refusal already
// cover. Nothing is invented for it and nothing is defaulted.
func TestAnArgumentNothingAnsweredForHasNoProposal(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs := takingText(t, e, text("quality"), text("preset"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if len(said.Values) != 1 || said.Values[0].Name != "quality" {
		t.Errorf("the proposals are %+v", said.Values)
	}
}

// NOTHING IS PROPOSED WHEN THERE IS NOTHING TO PROPOSE, and no question is asked either.
func TestNothingIsAskedWhenThereIsNothingToAskAbout(t *testing.T) {
	// A ROUTINE THAT TAKES NOTHING.
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs := takingText(t, e)
	if said := r.Resolve(context.Background(), request, cs,
		at("arkascended")); len(said.Values) != 0 || said.SourceAt != "" {
		t.Errorf("a routine that takes nothing was given %+v", said.Values)
	}
	if e.asked != 0 {
		t.Errorf("a routine that takes nothing was asked about: %v", e.names)
	}

	// A RESOLVER THAT ANSWERED NOTHING — absent, route-only, timed out, refused: one
	// answer, and the person says what they meant in the fields that are already there.
	declined := &recordingExtractor{ok: false,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs = takingText(t, declined, text("quality"))
	if said := r.Resolve(context.Background(), request, cs,
		at("arkascended")); len(said.Values) != 0 || said.SourceAt != "" {
		t.Errorf("a refusal proposed %+v", said.Values)
	}

	// AND NO EXTRACTOR CONFIGURED AT ALL, which is every machine with no plugin on it.
	r, cs = takingText(t, nil, text("quality"))
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))
	if len(said.Values) != 0 || said.SourceAt != "" {
		t.Errorf("an unconfigured resolver proposed %+v", said.Values)
	}
	// AND THE ROUTINE IS STILL SELECTED. The whole point of a proposal being a proposal:
	// nothing about it is load-bearing for getting to the routine.
	if said.Kind != dispatch.IntentRun || said.ID != "arkascended/focus/ark-graphics" {
		t.Errorf("without an extractor the answer is %+v", said)
	}
}

// AND A VALUE IS NEVER PROPOSED BESIDE A ROUTINE NOBODY PICKED.
//
// The second hard rule. A shared name is a QUESTION — nothing has been selected — and a value
// carried beside it is a value waiting for a routine to attach itself to: whichever routine the
// person then picks would arrive with text that was read for a different one's argument.
func TestAQuestionCarriesNoProposals(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, _ := takingText(t, e, text("quality"))

	// TWO ROUTINES CALLED THE SAME THING, which nothing can have answered exactly, so
	// the ambiguity re-check turns the resolver's run into a question.
	first := aCandidate("ark-graphics", "arkascended", true)
	first.Takes, first.SourceAt = []routes.Argument{text("quality")}, "sha256:abc"
	second := aCandidate("ark-graphics", "steam", true)
	second.Takes, second.SourceAt = []routes.Argument{text("quality")}, "sha256:def"
	said := r.Resolve(context.Background(), "ark graphics",
		[]Candidate{first, second}, at("arkascended"))

	if said.Kind != dispatch.IntentClarify {
		t.Fatalf("a shared name was not a question: %+v", said)
	}
	if len(said.Values) != 0 || said.SourceAt != "" {
		t.Errorf("a question carries %+v at %q", said.Values, said.SourceAt)
	}
}

// AND NEITHER DOES AN ANSWER THAT IS NOT A RUN.
func TestOnlyARunCarriesProposals(t *testing.T) {
	// EACH DECISION CARRIES WHAT ITS KIND REQUIRES, because `dispatch.Decide` VALIDATES
	// one: a chat or a clarification with nothing to say is refused, and so is a learn
	// offer with no name. Without those fields these three cases never reached the arm
	// they are about — they were three rejections falling through to the deterministic
	// one, and the test passed while a mutation that handed a near miss the library's
	// first routine survived it.
	for _, offer := range []dispatch.Decision{
		{Intent: dispatch.IntentLearn, Name: "ark graphics",
			Source: dispatch.SourceAdvisor},
		{Intent: dispatch.IntentChat, Reply: "I can't do that one",
			Source: dispatch.SourceAdvisor},
		{Intent: dispatch.IntentClarify, Route: "ark-graphics",
			Reply: "did you mean ark graphics?", Source: dispatch.SourceAdvisor},
	} {
		e := &recordingExtractor{ok: true,
			spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
		c := aCandidate("ark-graphics", "arkascended", true)
		c.Takes, c.SourceAt = []routes.Argument{text("quality")}, "sha256:abc"
		r := Resolver{
			Reg:     routes.Registry{Dir: t.TempDir()},
			Advisor: &recordingAdvisor{ok: true, offers: offer},
			Extract: e.extract,
		}
		said := r.Resolve(context.Background(), request, []Candidate{c},
			at("arkascended"))
		// AND THE ARM WAS REALLY REACHED, which is the thing this test got wrong
		// once: an answer that is not the kind it was given is a rejection, and a
		// rejection proves nothing about the kind.
		if said.Kind != offer.Intent {
			t.Fatalf("a %q offer was answered as %q, so this case never reached "+
				"the arm it is about", offer.Intent, said.Kind)
		}
		if len(said.Values) != 0 || said.SourceAt != "" {
			t.Errorf("a %q answer carries %+v", offer.Intent, said.Values)
		}
		if e.asked != 0 {
			t.Errorf("a %q answer asked about %v", offer.Intent, e.names)
		}
	}
}

// AND EXTRACTION CANNOT WIDEN ANYTHING.
//
// Everything about which routine answered, how, and what the person is told about the resolver's
// part in it must read the same with the question wired up and with it absent. A proposal is
// something SHOWN beside a decision that was already made without it.
func TestExtractionChangesNothingAboutTheDecision(t *testing.T) {
	plain := func(e *recordingExtractor) Answer {
		r, cs := takingText(t, e, text("quality"))
		return r.Resolve(context.Background(), request, cs, at("arkascended"))
	}
	with := plain(&recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}})
	without := plain(nil)

	with.Values, without.Values = nil, nil
	with.SourceAt, without.SourceAt = "", ""
	// The library is each call's own temporary directory, which is this test's doing and
	// not the decision's.
	with.Library, without.Library = "", ""
	if !reflect.DeepEqual(with, without) {
		t.Errorf("the decision differs:\n with    %+v\n without %+v", with, without)
	}
}

// AND A ROUTINE THAT WOULD REFUSE IS STILL NOT OFFERED, proposals or no proposals.
//
// The eligibility rules are upstream of this and must stay upstream of it: a value read out of a
// request is not a reason to offer a routine that cannot run.
func TestProposalsDoNotMakeAnIneligibleRoutineAnswer(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	r, cs := takingText(t, e, text("quality"))
	cs[0].Runnable = false
	said := r.Resolve(context.Background(), request, cs, at("arkascended"))

	if said.ID != "" || len(said.Values) != 0 {
		t.Errorf("an unrunnable routine answered with %+v", said)
	}
	if e.asked != 0 {
		t.Errorf("an unrunnable routine was asked about: %v", e.names)
	}
}

// THE VALUE QUESTION IS TOLD WHICH ROUTINE WAS CHOSEN, so the words that name it — however they
// were misheard — are not taken for a value (ADR-321). Not passing the name must fail this.
func TestTheValueQuestionNamesTheRoutine(t *testing.T) {
	e := &recordingExtractor{ok: true,
		spans: []resolver.Span{{Name: "quality", Start: 20, End: 24}}}
	c := aCandidate("ark-graphics", "arkascended", true)
	c.Takes, c.SourceAt = []routes.Argument{text("quality")}, "sha256:abc"
	r := Resolver{Reg: routes.Registry{Dir: t.TempDir()},
		Advisor: &recordingAdvisor{ok: true, offers: dispatch.Decision{Intent: dispatch.IntentRun,
			Route: "ark-graphics", Source: dispatch.SourceAdvisor}},
		Extract: e.extract}
	r.Resolve(context.Background(), request, []Candidate{c}, at("arkascended"))
	if e.asked != 1 || e.routine != c.Name {
		t.Errorf("the value question was asked %d time(s) about the routine %q, want %q", e.asked, e.routine, c.Name)
	}
}
