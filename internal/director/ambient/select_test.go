package ambient_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What "learn what I just did" is allowed to conclude from a trail.
//
// These drive the PURE selection function over a hand-built buffer. Nothing here touches a store,
// a session or a desktop: the whole point of separating selection from promotion is that the
// question "which part of the past did they mean" can be asked deterministically, in a fixture,
// with the boundaries and the refusals visible. See ADR-094.

const app = "settings"

// walk records one leg of somebody's afternoon.
func walk(b *ambient.Buffer, from, to string, at time.Time, acts ...ambient.Act) {
	b.Walked(ambient.Step{From: from, To: to, Application: app, By: ambient.ByHuman,
		Did: acts, At: at})
}

// press is an action on a control that could be named.
func press(label string) ambient.Act {
	return ambient.Act{Kind: ambient.Activate,
		Target: ambient.Target{Role: "button", Label: label}}
}

// anonymous is an action on a control whose name was withheld — the ordinary passive case for a
// role outside the plaintext allowlist.
func anonymous() ambient.Act {
	return ambient.Act{Kind: ambient.Activate, Target: ambient.Target{Role: "listitem"}}
}

func now() time.Time { return time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC) }

// THE ONE THAT MATTERS: Home, click Bluetooth, Bluetooth, click Mouse, Mouse.
//
// The whole product target of this roadmap, as a fixture. Somebody used their computer normally
// while Marco watched, and what comes back is the walk they took and what they pressed on the way
// — with no question asked, no repeat demonstration and nothing rehearsed.
func TestTheWalkSomebodyJustTookIsWhatComesBack(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	walk(b, "subj_bt", "subj_mouse", at.Add(4*time.Second), press("Mouse"))

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Selected {
		t.Fatalf("outcome %q (%s); the clean case must be selected", res.Outcome, res.Why)
	}
	d := res.Demonstration
	if d.From != "subj_home" || d.To != "subj_mouse" {
		t.Errorf("the walk runs %s -> %s, want subj_home -> subj_mouse", d.From, d.To)
	}
	if len(d.Steps) != 2 {
		t.Fatalf("%d step(s), want 2: %+v", len(d.Steps), d.Steps)
	}
	if got := d.Steps[0].Did[0].Target.Label; got != "Bluetooth & devices" {
		t.Errorf("the first step activated %q, want Bluetooth & devices", got)
	}
	if got := d.Steps[1].Did[0].Target.Label; got != "Mouse" {
		t.Errorf("the second step activated %q, want Mouse", got)
	}
}

// AN UNRECOGNISED SCREEN CROSSED ON THE WAY IS NOT AN ENDPOINT.
//
// Home, click Bluetooth, two frames part-way through arriving, Bluetooth. That is ONE semantic
// transition. Recording it as three edges through two screens nobody could describe would put
// junk in the topology and produce a play that waits for a spinner.
//
// The bridging is done by the observer, which records nothing for a reading it cannot place — see
// TestALoadingScreenIsNotSomewhereYouWent. This is the other half: what selection sees afterwards
// is one step, and the count of what was crossed rides on it.
func TestAnUnknownScreenInTheMiddleIsOneTransition(t *testing.T) {
	b := ambient.New()
	at := now()
	b.Walked(ambient.Step{From: "subj_home", To: "subj_bt", Application: app,
		By: ambient.ByHuman, Did: []ambient.Act{press("Bluetooth & devices")},
		Bridged: 2, Settled: 3, At: at})

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Selected {
		t.Fatalf("outcome %q (%s)", res.Outcome, res.Why)
	}
	if n := len(res.Demonstration.Steps); n != 1 {
		t.Fatalf("%d step(s), want 1", n)
	}
	if got := res.Demonstration.Steps[0].Bridged; got != 2 {
		t.Errorf("the step says %d screen(s) were crossed, want 2. A leg assembled across "+
			"frames nobody could place is still that leg, and a reader weighing it is "+
			"entitled to know how much of the middle was never seen.", got)
	}
}

// WHAT MARCO DID IS NOT WHAT YOU DEMONSTRATED.
//
// A play running while watching is on moves the screen too. Offering that back as something the
// person just showed Marco is how a system comes to learn its own behaviour from itself — and the
// person would have no way to tell, because the walk would look exactly like one of theirs.
//
// Its own outcome rather than "nothing recent", because the two sentences are different and only
// one of them is true.
func TestWhatMarcoDidIsNotWhatYouDemonstrated(t *testing.T) {
	b := ambient.New()
	at := now()
	b.Walked(ambient.Step{From: "subj_home", To: "subj_bt", Application: app,
		By: ambient.ByMarco, Did: []ambient.Act{press("Bluetooth & devices")}, At: at})

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.NotYours {
		t.Fatalf("outcome %q, want %q: Marco's own performance was offered back as the "+
			"person's demonstration", res.Outcome, ambient.NotYours)
	}
	if len(res.Demonstration.Steps) != 0 {
		t.Error("a demonstration was returned for a walk nobody demonstrated")
	}
}

// A SCREEN CHANGE WITH NOTHING BEHIND IT CANNOT BE LEARNED.
//
// A timer fired, a notification arrived, a page finished loading on its own. Marco saw where
// somebody ended up and did not see them do anything to get there, so it does not know how to
// make it happen again — and saying so is the answer, not "I learned it".
func TestAChangeNobodyCausedIsNotADemonstration(t *testing.T) {
	b := ambient.New()
	walk(b, "subj_home", "subj_bt", now())

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Insufficient || res.Why != ambient.ShortNoAction {
		t.Fatalf("outcome %q/%q, want insufficient/%q", res.Outcome, res.Why,
			ambient.ShortNoAction)
	}
}

// A PRESS ON SOMETHING MARCO COULD NOT NAME IS NOT ENOUGH.
//
// The commonest real shortfall on a desktop, and the one whose sentence matters most: a press on
// a list item resolves to a role and no name under passive watching, because the plaintext
// allowlist deliberately excludes the roles somebody's own documents and contacts live in. That
// is the privacy boundary working rather than perception failing, and the refusal has to say so
// rather than reading as "I could not see your screen".
func TestAPressMarcoCouldNotNameIsRefusedForTheRightReason(t *testing.T) {
	b := ambient.New()
	walk(b, "subj_home", "subj_bt", now(), anonymous())

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Insufficient || res.Why != ambient.ShortUnnamedTarget {
		t.Fatalf("outcome %q/%q, want insufficient/%q", res.Outcome, res.Why,
			ambient.ShortUnnamedTarget)
	}
}

// A SCREEN MARCO CAN NEITHER RECOGNISE NOR DESCRIBE IS NOT AN ENDPOINT.
func TestAWalkThroughUndescribableScreensIsRefused(t *testing.T) {
	b := ambient.New()
	b.Walked(ambient.Step{From: "subj_home", To: "seen_state_4", Application: app,
		By: ambient.ByHuman, Did: []ambient.Act{press("Bluetooth")}, At: now()})

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Insufficient || res.Why != ambient.ShortUnknownPlace {
		t.Fatalf("outcome %q/%q, want insufficient/%q", res.Outcome, res.Why,
			ambient.ShortUnknownPlace)
	}
}

// A SCREEN MARCO DOES NOT KNOW BUT CAN DESCRIBE IS STILL SOMEWHERE YOU WENT.
//
// The first time anybody uses a program, every screen in it is unknown — which is exactly when
// they most want to show Marco something. A transient endpoint carrying its structural shape is
// sufficient: an explicit Learn holds the licence to establish it from that shape, and does.
func TestAScreenMarcoCanDescribeIsGoodEnoughToLearn(t *testing.T) {
	b := ambient.New()
	shape := &ambient.Shape{Local: "state_4", Signature: observe.StructureSignature{
		Subject: observe.SubjectState, Members: 6, TermsKnown: true,
		Roles: map[string]int{"button": 4}, Terms: []observe.InterfaceTerm{observe.TermSettings},
	}}
	b.Walked(ambient.Step{From: "subj_home", To: "seen_state_4", ToShape: shape,
		Application: app, By: ambient.ByHuman,
		Did: []ambient.Act{press("Bluetooth")}, At: now()})

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Selected {
		t.Fatalf("outcome %q/%q; a describable screen must be learnable", res.Outcome, res.Why)
	}
	if res.Demonstration.Steps[0].ToShape == nil {
		t.Fatal("the shape did not survive selection, so nothing could establish that screen")
	}
}

// TWO WAYS TO THE SAME PLACE IS A QUESTION.
//
// Somebody opens Mouse from Home through Bluetooth, then opens it again from Home through
// Devices. Both are recent, both are theirs, both end where they are standing. Picking the more
// recent one silently would be choosing arbitrary history — and it would be wrong about half the
// time, because the one somebody means is the one they consider the right way, not the one they
// happened to do last.
func TestTwoWaysToTheSamePlaceIsAQuestion(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	walk(b, "subj_bt", "subj_mouse", at.Add(2*time.Second), press("Mouse"))
	walk(b, "subj_mouse", "subj_home", at.Add(4*time.Second), press("Back"))
	walk(b, "subj_home", "subj_devices", at.Add(6*time.Second), press("Devices"))
	walk(b, "subj_devices", "subj_mouse", at.Add(8*time.Second), press("Mouse"))

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Ambiguous {
		t.Fatalf("outcome %q, want ambiguous. Two routes to one destination in the same "+
			"stretch of activity is a case Marco cannot choose between, and choosing "+
			"anyway is choosing arbitrary history.", res.Outcome)
	}
	if len(res.Alternative.Steps) == 0 {
		t.Error("the ambiguity was reported with no alternative, so the question a person " +
			"is asked cannot name the other reading")
	}
}

// THE SELECTED WALK NEVER GOES ANYWHERE TWICE.
//
// Home, open Bluetooth, go back to Home, open Display. Taking all three legs would learn a walk
// that visits Home twice and passes through a screen it has already left — two intentions spliced
// into one. What is kept is the longest recent stretch that is a SIMPLE walk, which here is
// Bluetooth, back to Home, on to Display.
//
// Not "only the last leg", deliberately. Every leg of that stretch is a real thing the person did
// and every one of them becomes reusable route evidence; the goal is where they stopped, and
// planning finds the shortest way there from wherever somebody is standing later. Cutting to one
// leg would throw away evidence for no gain.
func TestARoundTripIsNotOneDemonstration(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	walk(b, "subj_bt", "subj_home", at.Add(2*time.Second), press("Back"))
	walk(b, "subj_home", "subj_display", at.Add(4*time.Second), press("Display"))

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Selected {
		t.Fatalf("outcome %q/%q", res.Outcome, res.Why)
	}
	d := res.Demonstration
	if len(d.Steps) != 2 {
		t.Fatalf("%d step(s), want 2 — the walk was not cut where it would have visited "+
			"Home a second time: %+v", len(d.Steps), d.Steps)
	}
	if d.From != "subj_bt" || d.To != "subj_display" {
		t.Errorf("the walk runs %s -> %s, want subj_bt -> subj_display", d.From, d.To)
	}
	seen := map[string]bool{d.Steps[0].From: true}
	for _, s := range d.Steps {
		if seen[s.To] {
			t.Errorf("the selected walk visits %s twice: %+v", s.To, d.Steps)
		}
		seen[s.To] = true
	}
}

// A DIFFERENT PROGRAM ENDS THE DEMONSTRATION.
//
// Plays are scoped to one application. A walk that crossed two would be learned as a thing Marco
// cannot represent, and the honest response is to take only the part that is inside one program.
func TestADemonstrationStopsAtTheEdgeOfItsProgram(t *testing.T) {
	b := ambient.New()
	at := now()
	b.Walked(ambient.Step{From: "subj_inbox", To: "subj_draft", Application: "mail",
		By: ambient.ByHuman, Did: []ambient.Act{press("New")}, At: at})
	walk(b, "subj_home", "subj_bt", at.Add(2*time.Second), press("Bluetooth & devices"))

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{})
	if res.Outcome != ambient.Selected {
		t.Fatalf("outcome %q/%q", res.Outcome, res.Why)
	}
	if n := len(res.Demonstration.Steps); n != 1 {
		t.Fatalf("%d step(s), want 1 — the mail leg was folded into a settings walk", n)
	}
	if res.Demonstration.Application != app {
		t.Errorf("the walk is about %q, want %q", res.Demonstration.Application, app)
	}
}

// A LONG PAUSE SEPARATES TWO INTENTIONS.
//
// Somebody did something, went and made a coffee, came back and did something else. "What I just
// did" is the second one. The pause is measured BETWEEN two things they did and never from the
// present — see the note on ambient.Request for why measuring from now would refuse somebody who
// was interrupted before typing the command.
func TestACoffeeBreakSeparatesTwoThingsYouDid(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	walk(b, "subj_bt", "subj_mouse", at.Add(2*ambient.RecentGap), press("Mouse"))

	res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if res.Outcome != ambient.Selected {
		t.Fatalf("outcome %q/%q", res.Outcome, res.Why)
	}
	if n := len(res.Demonstration.Steps); n != 1 {
		t.Fatalf("%d step(s), want 1: a pause of %v was read as part of one demonstration",
			n, 2*ambient.RecentGap)
	}
}

// THE SAME AFTERNOON IS NOT LEARNED TWICE.
//
// Somebody learns what they just did, then does something else and learns that too. Without a
// watermark the second Learn would walk back over the first one's evidence and learn the whole
// stretch again, under a second name — two plays for one walk, and the person would have no idea.
//
// A WATERMARK rather than a deletion. The steps stay in the trail: the step after a promoted one
// still needs its predecessor to know where it began.
func TestTheSameAfternoonIsNotLearnedTwice(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	walk(b, "subj_bt", "subj_mouse", at.Add(2*time.Second), press("Mouse"))

	first := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if first.Outcome != ambient.Selected {
		t.Fatalf("the first selection failed: %q/%q", first.Outcome, first.Why)
	}
	b.Promoted(first.Demonstration.Through)

	// And now something else, once.
	walk(b, "subj_mouse", "subj_pointer", at.Add(20*time.Second), press("Pointer"))
	second := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app})
	if second.Outcome != ambient.Selected {
		t.Fatalf("the second selection failed: %q/%q", second.Outcome, second.Why)
	}
	if n := len(second.Demonstration.Steps); n != 1 {
		t.Fatalf("the second learn took %d step(s), want 1. Everything before the watermark "+
			"has already become knowledge and learning it again writes a second play for "+
			"one walk: %+v", n, second.Demonstration.Steps)
	}
	// AND THE TRAIL STILL HOLDS THE WHOLE WALK, because a watermark is not a deletion.
	if n := len(b.Look().Recent); n != 3 {
		t.Errorf("the trail holds %d step(s) after a promotion, want 3", n)
	}
}

// AND A WATERMARK NEVER MOVES BACKWARDS.
//
// Two promotions can finish out of order — nothing serialises them — and the later one arriving
// with a lower number must not re-expose evidence the earlier one already consumed. That would
// make the same walk learnable twice, which is the whole thing the watermark exists to prevent,
// reached from the other side.
//
// The claim was in a comment with nothing holding it: the mutation that makes the watermark take
// any value survived the suite. Measured.
func TestAWatermarkNeverMovesBackwards(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	walk(b, "subj_bt", "subj_mouse", at.Add(time.Second), press("Mouse"))

	b.Promoted(2)
	b.Promoted(1)
	if got := b.Look().Consumed; got != 2 {
		t.Fatalf("the watermark is %d after a promotion through 2 and a late one through 1, "+
			"want 2. Evidence that has already become knowledge came back.", got)
	}
	if res := ambient.SelectDemonstration(b.Look(), ambient.Request{Application: app}); res.Outcome != ambient.Nothing {
		t.Errorf("a walk that was already learned was offered again: %+v", res)
	}
}

// NOTHING RECENT IS ITS OWN ANSWER.
func TestAnEmptyTrailIsNotADemonstration(t *testing.T) {
	res := ambient.SelectDemonstration(ambient.New().Look(), ambient.Request{})
	if res.Outcome != ambient.Nothing {
		t.Fatalf("outcome %q, want %q", res.Outcome, ambient.Nothing)
	}
}

// SELECTION WRITES NOTHING, whatever it concludes.
//
// The property that makes it safe to ask on any phrase: a refusal costs nothing and leaves
// nothing behind. Asserted by asking every question over one buffer and finding it unchanged.
func TestAskingWhatYouJustDidChangesNothing(t *testing.T) {
	b := ambient.New()
	at := now()
	walk(b, "subj_home", "subj_bt", at, press("Bluetooth & devices"))
	before := fmt.Sprintf("%+v", b.Look())

	for _, req := range []ambient.Request{{}, {Application: app}, {Application: "mail"}} {
		ambient.SelectDemonstration(b.Look(), req)
	}
	if after := fmt.Sprintf("%+v", b.Look()); after != before {
		t.Errorf("asking changed the evidence:\n before %s\n after  %s", before, after)
	}
}

// THE TRAIL STAYS BOUNDED however much somebody does.
//
// 36A's claim, restated over the richer step: an afternoon of clicking is still sixty-four steps.
func TestTheTrailIsStillBoundedWithActionsOnIt(t *testing.T) {
	b := ambient.New()
	at := now()
	for i := 0; i < ambient.MaxMoves*4; i++ {
		walk(b, fmt.Sprintf("subj_%d", i), fmt.Sprintf("subj_%d", i+1),
			at.Add(time.Duration(i)*time.Second), press(fmt.Sprintf("thing %d", i)))
	}
	if _, _, recent := b.Size(); recent > ambient.MaxMoves {
		t.Fatalf("the trail grew to %d, past its bound of %d", recent, ambient.MaxMoves)
	}
}

// AN ORDER IS NEVER REISSUED, even after the tail has forgotten the step that had it.
//
// The watermark is expressed in these numbers. If the sequence restarted when the tail wrapped,
// a promotion at 40 would suppress forty later steps that have nothing to do with it.
func TestForgottenStepsDoNotGiveTheirNumbersBack(t *testing.T) {
	b := ambient.New()
	at := now()
	for i := 0; i < ambient.MaxMoves*2; i++ {
		walk(b, fmt.Sprintf("subj_%d", i), fmt.Sprintf("subj_%d", i+1),
			at.Add(time.Duration(i)*time.Second))
	}
	recent := b.Look().Recent
	if got := recent[len(recent)-1].Order; got != ambient.MaxMoves*2 {
		t.Errorf("the newest step is number %d after %d steps; the sequence restarted, and "+
			"a promotion watermark means nothing if it does",
			got, ambient.MaxMoves*2)
	}
}

// AN INVENTED ACTION WORD DOES NOT GET IN.
//
// The vocabulary is closed and checked at the door, so a caller that wanted to put arbitrary text
// into a privacy-bounded record would have to add a constant to the closed list — a reviewable
// act rather than a string appearing in a buffer.
func TestAnInventedActionWordIsDropped(t *testing.T) {
	b := ambient.New()
	b.Walked(ambient.Step{From: "subj_home", To: "subj_bt", Application: app,
		By: ambient.ByHuman, At: now(),
		Did: []ambient.Act{{Kind: "typed the password", Target: ambient.Target{Label: "hunter2"}},
			press("Bluetooth")}})

	recent := b.Look().Recent
	if n := len(recent[0].Did); n != 1 {
		t.Fatalf("%d act(s) kept, want 1: %+v", n, recent[0].Did)
	}
	if recent[0].Did[0].Kind != ambient.Activate {
		t.Errorf("the wrong act survived: %+v", recent[0].Did[0])
	}
}

// A TRANSIENT SHAPE CARRIES NO WORDS ANYBODY COULD READ.
//
// A screen's signature is role counts, members, normalised geometry and closed interface
// vocabulary. The type it lives in ALSO has a Label, a Kind and a Place — those belong to a
// target's identity, they are empty for a screen, and a change that started filling them in for
// screens would put text off somebody's display into a transient buffer.
func TestATransientShapeCarriesNoWordsAnybodyCouldRead(t *testing.T) {
	sig := observe.StructureSignature{
		Subject: observe.SubjectState, Members: 6, TermsKnown: true,
		Roles: map[string]int{"button": 4},
		Terms: []observe.InterfaceTerm{observe.TermSettings},
	}
	s := (&ambient.Shape{Signature: sig, Local: "state_4"}).Clone()
	if s.Signature.Label != "" || s.Signature.Kind != "" || s.Signature.Place != "" {
		t.Errorf("a screen's shape carries a target's identity fields: %+v", s.Signature)
	}
	if s.Signature.Subject != observe.SubjectState {
		t.Errorf("a screen's shape is a %q, want %q", s.Signature.Subject, observe.SubjectState)
	}
}

// AND THE BUFFER STILL HOLDS NO WORDS EXCEPT THE ONE IT IS ALLOWED TO.
//
// 36A's claim, narrowed rather than dropped. The one kind of text that crosses now is the name of
// the ONE control a person's own action landed on; everything else in the type is ids, counts,
// times, closed vocabulary and structure. A title, a line of screen text, a coordinate or a
// screenshot appearing here would be a privacy change, and this is what would catch it.
func TestTheBufferStillHoldsNothingItShouldNot(t *testing.T) {
	b := ambient.New()
	b.Saw(app, "subj_home", now())
	walk(b, "subj_home", "subj_bt", now(), press("Bluetooth & devices"))

	rendered := fmt.Sprintf("%+v", b.Look())
	for _, leak := range []string{"Title", "Screenshot", "Pixels", "Text:", "Keys", "Typed"} {
		if contains(rendered, leak) {
			t.Errorf("the buffer carries %q: %s", leak, rendered)
		}
	}
	// The one exception, asserted as an exception so it cannot quietly become the rule: a
	// label rides on an ACT's target and nowhere else.
	if !contains(rendered, "Bluetooth & devices") {
		t.Error("the control's name did not survive, so nothing could learn what was pressed")
	}
}
