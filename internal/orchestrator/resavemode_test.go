package orchestrator

// A REPAIR MUST NOT EDIT WHAT A ROUTINE SAYS ABOUT ITS OWN RECORDING.
//
// `Resave` exists because the guarded contract arrived before the rule about scopes did: a
// routine saved in between states the window each press must be checked against, under a scope
// that brings no window forward, so every press refuses. The repair rebuilds it from the
// demonstration stored beside it, under the scope its presses need, without anybody
// demonstrating anything again.
//
// It rebuilt it stating NOTHING about how it was recorded, and that is two defects in one line.
//
// The first is a claim edited out of somebody's file: a routine that said it was recorded as
// coordinates came back out of the repair saying nothing, and "nothing" is not a smaller version
// of "coordinates" — it is the honest answer for a file that never said, which this one did.
//
// The second is the one that presses a menu before it has drawn. The stated mode is also what
// keeps the recorded gaps: with the sensing flags inherited ON, simplification caps every pause
// to 50ms because the anchor poll was supposed to do the real waiting — and a coordinate routine
// has no anchor to wait on. So a Studio launched with `$MARCO_CV=on` rebuilt a working routine
// with its pauses taken out and nothing put in their place. The repair button is in that window.
//
// Held through the production path on purpose: the routine is listed by `Unrunnable` and repaired
// by `Resave`, which is what Studio's button calls. A test that built a draft and saved it by hand
// would pass with the repair still reading the mode from the wrong place.
//
// See [[ADR-254-what-a-recording-captures-is-stated]] and
// [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/simplify"
)

// savedUnrepaired saves one demonstration in the state a repair is about, and hands back a Deps
// built the way the repair button builds one.
//
// Two `Deps` over one library deliberately. The first records and saves, stating a mode as Studio
// does; the second is what `Acts.Resave` constructs — a registry, somewhere to write, and nothing
// said about any mode. The gap between them is where the defect lived.
func savedUnrepaired(t *testing.T, mode RecordedAs) (Deps, routes.Route) {
	t.Helper()
	_, dr := recordedWith(t, mode)
	dir := t.TempDir()
	saving := Deps{Reg: routes.Registry{Dir: dir}, Out: &strings.Builder{}, Mode: mode}
	// THE CURRENT-FOREGROUND SCOPE is what leaves it unable to press anything: guarded presses,
	// and no application to bind a window of. Since S9 "Only in" binds the window in front at
	// admission, so this is the one shape Unrunnable still lists. See Unrunnable.
	rt, err := saving.Save(dr, ScopeEverywhere, "")
	if err != nil {
		t.Fatalf("saving the play this repair is about: %v", err)
	}
	return Deps{Reg: routes.Registry{Dir: dir}, Out: &strings.Builder{}}, rt
}

// theOneToRepair finds it the way the button does, through the production listing.
func theOneToRepair(t *testing.T, d Deps, rt routes.Route) Unrunnable {
	t.Helper()
	listed := d.Unrunnable()
	for _, u := range listed {
		if u.Route != rt {
			continue
		}
		if !u.Resavable {
			t.Fatalf("%q has no demonstration stored beside it, so this is a test "+
				"of a refusal and not of a repair", u.Name)
		}
		return u
	}
	t.Fatalf("the play just saved is not listed as one that cannot press anything: "+
		"%+v", listed)
	return Unrunnable{}
}

// sourceOf is the saved Marco, which is the only durable account of a routine.
func sourceOf(t *testing.T, d Deps, rt routes.Route) string {
	t.Helper()
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatalf("reading %s: %v", d.Reg.Path(rt), err)
	}
	return string(raw)
}

var sleeps = regexp.MustCompile(`Sleep with (\d+)`)

// longestPause is the biggest wait the generated routine carries, in milliseconds.
//
// Read out of the source rather than off the steps, because the source is what runs: a repair
// that kept the pauses in a draft and then generated a file without them would pass an assertion
// about the draft.
func longestPause(t *testing.T, src string) int {
	t.Helper()
	most := 0
	for _, m := range sleeps.FindAllStringSubmatch(src, -1) {
		ms, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("%q is not a number of milliseconds", m[1])
		}
		if ms > most {
			most = ms
		}
	}
	return most
}

// THE WORD SURVIVES THE REPAIR.
//
// Asserted on the SOURCE and through `ModeOf`, which is the one reader of that line: Studio's
// listing calls it for the card's sentence about how a routine was recorded, and `marco` calls it
// for the mode in every run record, so a file that carries the word carries it into both. The
// companion tests at those two entrances are `plugins/studio`
// `TestSavingAgainKeepsWhatTheRoutineSaysItRecorded` and `cmd/marco`
// `TestTheRecordOfARepairedRoutineSaysWhatItWasRecordedAs`.
func TestARepairKeepsWhatTheRoutineSaysItRecorded(t *testing.T) {
	// THE SENSING FLAGS INHERITED ON, which is the case the defect needed. Studio is open
	// all afternoon under whatever launched it, and the repair button is in that window.
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")

	d, rt := savedUnrepaired(t, Coordinates)
	if was := sourceOf(t, d, rt); ModeOf(was) != Coordinates {
		t.Fatalf("the play this repair is about does not state coordinates, so "+
			"there is nothing here for a repair to lose:\n%s", was)
	}
	now, err := d.Resave(theOneToRepair(t, d, rt))
	if err != nil {
		t.Fatalf("saving it again: %v", err)
	}
	src := sourceOf(t, d, now)
	if got := ModeOf(src); got != Coordinates {
		t.Errorf("a play that stated %q came out of the repair stating %q, so the "+
			"repair edited a claim about how somebody recorded it:\n%s",
			Coordinates, got, src)
	}
	// AND IN WORDS A PERSON READS, not only in a word a parser finds. Somebody opens
	// this file.
	if !strings.Contains(src, modeStated+string(Coordinates)) {
		t.Errorf("the repaired play does not say how it was recorded anywhere a "+
			"person reading the file would see it:\n%s", src)
	}
}

// AND SO DO THE PAUSES THE WORD IS THERE TO KEEP.
//
// The quieter half, and the one that changes what the routine DOES. A repair that carried the
// comment and dropped the gaps would pass every assertion above while handing somebody a routine
// that presses a menu before it has drawn — the defect looking exactly like the fix.
func TestARepairKeepsTheRecordedPausesOfACoordinateRoutine(t *testing.T) {
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")

	d, rt := savedUnrepaired(t, Coordinates)
	before := longestPause(t, sourceOf(t, d, rt))
	capped := simplify.DefaultOptions().MaxWaitMs
	if before <= capped {
		t.Fatalf("the play this repair is about waits %dms at most, which the %dms "+
			"cap would not change — so this cannot tell a repair that kept the "+
			"pauses from one that capped them", before, capped)
	}
	now, err := d.Resave(theOneToRepair(t, d, rt))
	if err != nil {
		t.Fatalf("saving it again: %v", err)
	}
	if after := longestPause(t, sourceOf(t, d, now)); after != before {
		t.Errorf("the play waited %dms before the repair and %dms after it. With "+
			"the sensing flags inherited on, the repair capped the recorded gaps "+
			"of a play that states it was recorded as coordinates — which has "+
			"no anchor to wait on in their place, so the pauses are simply gone",
			before, after)
	}
}

// A ROUTINE THAT SAYS NOTHING STILL SAYS NOTHING AFTERWARDS.
//
// Absent is the honest answer and not a blank to be filled. It cannot be inferred either: this
// demonstration has no pictures in it, which is what a coordinate recording looks like AND what a
// recording made with anchors on against a flat menu that offered nothing to anchor to looks
// like. Those are two different facts about somebody's afternoon and the file is identical, so a
// repair that read the word off the geometry would be telling them which one happened.
func TestARepairStatesNothingWhenTheRoutineStatedNothing(t *testing.T) {
	t.Setenv("MARCO_CV", "off")

	d, rt := savedUnrepaired(t, Unstated)
	now, err := d.Resave(theOneToRepair(t, d, rt))
	if err != nil {
		t.Fatalf("saving it again: %v", err)
	}
	src := sourceOf(t, d, now)
	if got := ModeOf(src); got != Unstated {
		t.Errorf("a play that stated no mode came out of the repair stating %q", got)
	}
	if strings.Contains(src, modeStated) {
		t.Errorf("the repair wrote a mode line into a play that never carried one, "+
			"so it now makes a claim nobody made:\n%s", src)
	}
}

// AND A WORD THIS BUILD DOES NOT KNOW IS NOT PASSED ON AS ONE.
//
// A newer build's word, or one somebody typed into the file by hand. `ModeOf` reads it as
// unstated rather than echoing it, and a repair is the last place to start carrying an unverified
// claim forward: the file it writes is the one a run record quotes.
func TestARepairDoesNotCarryOnAWordThisBuildDoesNotKnow(t *testing.T) {
	t.Setenv("MARCO_CV", "off")

	d, rt := savedUnrepaired(t, Unstated)
	edited := modeStated + "semantic.\n" + sourceOf(t, d, rt)
	if err := d.Reg.Save(rt, edited); err != nil {
		t.Fatal(err)
	}
	now, err := d.Resave(theOneToRepair(t, d, rt))
	if err != nil {
		t.Fatalf("saving it again: %v", err)
	}
	src := sourceOf(t, d, now)
	if got := ModeOf(src); got != Unstated {
		t.Errorf("a play stating a word this build does not know came out of the "+
			"repair stating %q", got)
	}
	if strings.Contains(src, "semantic") {
		t.Errorf("the repair copied a word nothing verified into the file it wrote, "+
			"where a run record will quote it as something this build read:\n%s", src)
	}
}

// AND THE REPAIR IS STILL A REPAIR, OF EXACTLY THE ROUTINE IT WAS ASKED ABOUT.
//
// Everything above is about a line in a file. This is the behaviour that line travels with, and
// it is asserted beside it because a change that preserved the mode by taking another route's
// identity, or by leaving both copies on disk, would be a worse defect arriving as a fix. Two
// routines of one name, one of which cannot press anything, is worse than the one that could not.
func TestARepairStillRepairsAndWritesTheSameRoutine(t *testing.T) {
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")

	d, rt := savedUnrepaired(t, Coordinates)
	was, lacked := ContractOf(sourceOf(t, d, rt))
	now, err := d.Resave(theOneToRepair(t, d, rt))
	if err != nil {
		t.Fatalf("saving it again: %v", err)
	}
	// THE SAME ROUTINE, under the scope its presses need: the slug is unchanged, and it now
	// brings forward the application it was RECORDED in. The fixture had none in its identity
	// (the one unrunnable shape left since S9), so that application is the demonstration's.
	want := routes.Route{Slug: rt.Slug, App: "arkascended", Focus: true}
	if now != want {
		t.Errorf("the repair wrote %+v, and the play it was asked about is %+v",
			now, want)
	}
	if left := d.Reg.List(); len(left) != 1 {
		t.Errorf("after one repair the library holds %d plays: %+v", len(left), left)
	}
	if still := d.Unrunnable(); len(still) != 0 {
		t.Errorf("the play still cannot press anything after being repaired: %+v",
			still)
	}
	// AND ITS DEMONSTRATION IS STILL BESIDE IT, so it can be repaired or re-simplified
	// again. A repair that consumed the recording would work exactly once.
	if !d.Reg.HasRecording(now) {
		t.Error("the repaired play has no demonstration stored beside it any more")
	}
	// AND ITS PRESSES SAY WHAT THEY SAID. Compared against the routine before the repair
	// rather than against a word written here: this fixture's contract is whatever the
	// generator makes of it, and the claim is that a repair does not degrade it.
	if contract, lacks := ContractOf(sourceOf(t, d, now)); contract != was {
		t.Errorf("the play's presses read as %q (lacking %v) before the repair and "+
			"%q (lacking %v) after it", was, lacked, contract, lacks)
	}
}
