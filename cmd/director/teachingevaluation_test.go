package main

// ONE TEACHING SESSION WITH A MISTAKE IN IT.
//
// [[ADR-234-the-person-does-not-have-to-prove-the-click]]. The owner's definition of done:
//
//	I want a demonstration session in which the user can act naturally, including making a
//	mistake, and Marco: records the actual inputs accurately, preserves their coordinates and
//	context, uses OCR and vision to understand the interaction without overriding raw evidence,
//	avoids unnecessary questions [...] and clearly identifies unsupported steps without
//	discarding the rest of the demonstration.
//
// So this drives the REAL observer, the real correlator, the real draft and the real artifact
// through a session that contains four of the awkward things on the owner's list at once:
//
//	a demonstrated click whose button boundary is known        -> a usable step
//	an OCR label missing on the clicked frame                  -> no text on the control
//	a click Marco could not segment at all                     -> kept, named, not replayable
//	stray keys that were watched and named nothing             -> counted, not promoted
//
// What it measures is the thing the owner asked to be measured: how many questions a completed
// teaching session puts to a person, and whether the parts that did work survived the parts that
// did not.

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aSessionWithAMistake is the whole demonstration, through the production observer.
//
// The geometry is Ark's, measured: [[Experiment-085-the-graphics-tab-is-a-button]] found the
// GRAPHICS tab at 7.2%x2.8% and the tab bar containing it at 94.6%x5.0%, so the clicked control
// here is a real button inside a real panel rather than a shape chosen to make a test pass.
func aSessionWithAMistake(t *testing.T) (*fastobserve.Observer, observe.PointerAt, observe.PointerAt) {
	t.Helper()
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	tabBar := observe.Region{X: 0.027, Y: 0.230, Width: 0.946, Height: 0.050}
	graphics := observe.Region{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}
	menu := []observe.Region{tabBar, graphics}
	// The panel that stands after the tab is chosen. A different layout, so the two
	// screens cannot fold into one.
	panel := []observe.Region{
		{X: 0.05, Y: 0.05, Width: 0.12, Height: 0.04},
		{X: 0.05, Y: 0.12, Width: 0.12, Height: 0.04},
		{X: 0.70, Y: 0.85, Width: 0.22, Height: 0.06},
	}

	onGraphics := observe.PointerAt{X: 0.414, Y: 0.261} // press 3 of the Ark capture
	onNothing := observe.PointerAt{X: 0.880, Y: 0.700}  // bare background: the mistake

	t0 := time.Unix(1700000000, 0).UTC()
	n := 0
	look := func(regions []observe.Region, presses ...observe.PointerAt) {
		n++
		o.Observe(fastobserve.Look{
			At:      t0.Add(time.Duration(n) * 100 * time.Millisecond),
			Window:  fastobserve.WindowRef{Generation: 1},
			Regions: regions, Front: true, Presses: presses,
		})
	}
	settle := func(regions []observe.Region) {
		for i := 0; i < o.SettleLooks()+1; i++ {
			look(regions)
		}
	}

	settle(menu)
	look(menu, onNothing) // THE MISTAKE: a click on nothing Marco can see
	settle(menu)          // and nothing happened, so the menu is still standing
	look(menu, onGraphics)
	settle(panel)
	settle(menu) // back out with Escape, which names no control
	look(menu, onGraphics)
	settle(panel)
	o.Close()
	return o, onGraphics, onNothing
}

// strayKeys is what the input layer saw: the Escape that worked, and a key that named nothing.
//
// Counted rather than interpreted. An ambiguous keypress is neither promoted to a deliberate
// step because it happened nor deleted because its effect was not observed.
func strayKeys() observe.InputStats {
	return observe.InputStats{
		Received: 11, Classified: 6,
		Ignored: map[observe.IgnoreReason]int{
			observe.IgnoreKeyRelease: 3,
		},
	}
}

// THE SESSION WORKS, THE MISTAKE IS KEPT, AND NOTHING IS ASKED.
func TestATeachingSessionWithAMistakeAsksNothing(t *testing.T) {
	o, onGraphics, onNothing := aSessionWithAMistake(t)
	reg := o.Registry()
	ev := o.Clicks()

	// ── the demonstrated control survived the mistake ──
	if len(reg) != 1 {
		for _, r := range reg {
			t.Logf("  %s", controlLine(r, reg))
		}
		t.Fatalf("%d control(s); one button pressed on two visits is one identity, and a "+
			"click on bare background elsewhere must not change that", len(reg))
	}
	if reg[0].Presses != 2 {
		t.Errorf("%s records %d press(es), want 2", reg[0].ID, reg[0].Presses)
	}

	// ── and the mistake is kept as the act it was ──
	if len(ev.BlindPresses) != 1 {
		t.Fatalf("%d blind press(es) kept of %d counted; the person pressed somewhere "+
			"and that is the strongest evidence in the session about what they did",
			len(ev.BlindPresses), ev.Blind)
	}
	if ev.BlindPresses[0].At != onNothing {
		t.Errorf("the kept press is at %v and was made at %v",
			ev.BlindPresses[0].At, onNothing)
	}
	if ev.BlindPresses[0].Screen == "" {
		t.Error("the kept press has no screen, so it is a coordinate rather than an act")
	}

	// ── the draft offers the usable step and names the unusable part ──
	draft := workflow.Learn("ark graphics", o.Walk(), ev, reg)
	clicks := 0
	for _, st := range draft.Steps {
		if st.Cause == observe.CauseClick {
			clicks++
		}
	}
	if clicks == 0 {
		t.Fatalf("the draft has no clicked step; %d step(s) in total, and the "+
			"demonstration pressed a known button twice", len(draft.Steps))
	}

	// ── AND THE QUESTION COUNT, which is what the owner asked to be measured ──
	arr := map[observe.VisualScreenID]observe.VisualArrangement{}
	for _, id := range o.Screens() {
		arr[id.ID] = o.Arrangement(id.ID)
	}
	a, err := workflowstore.Save(draft, arr, time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if qs := a.Questions(); len(qs) != 0 {
		for _, q := range qs {
			t.Logf("  asked about %s: %s", q.Control, q.Because)
		}
		t.Errorf("%d question(s) from a session whose control was pressed twice; a "+
			"control settled by a second press has nothing left to ask about", len(qs))
	}
	// AND THE THINGS A PERSON IS SHOWN TO SETTLE ARE COUNTED, because that is the other
	// half of the owner's measure and it is not zero. Three demonstrated steps press one
	// unread control twice and cross one automatic transition, which is TWO facts: the
	// control nobody read, and the screen that changed by itself. It was three before the
	// merge, with the control's fact printed once per step that pressed it.
	if un := draft.Unresolved(); len(un) != 2 {
		for _, u := range un {
			t.Logf("  %s", u)
		}
		t.Errorf("%d unresolved line(s) for two facts", len(un))
	}

	// ── AND WHICH COORDINATES REACHED THE SAVED WORKFLOW, AND WHICH DID NOT ──
	//
	// This guard used to forbid every `"at"` in the artifact, and the rule behind it has
	// since been narrowed ON PURPOSE rather than deleted:
	// [[ADR-248-replaying-a-demonstrated-layout]] authorised one point per INTERPRETED
	// STEP, because a replay of a click cannot be built out of a rectangle a segmenter
	// drew around it. What it did not authorise is any of the rest, so what this asserts
	// now is the boundary rather than the absence.
	//
	// The demonstrated press IS here. The MISTAKE — the click on bare background that
	// Marco could not segment and that never became a step — is NOT, and neither is the
	// tally of presses like it. That distinction is the whole safety argument: the raw
	// record beside this file keeps everything the person did, and the executable
	// artifact keeps only what was interpreted.
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"0.88", "blind_presses", "observations"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the saved workflow contains %q, which belongs to the raw "+
				"record and not to the executable artifact:\n%s",
				forbidden, body)
		}
	}
	if !strings.Contains(string(body), "\"at\"") {
		t.Errorf("the saved workflow carries no demonstrated click at all, so no "+
			"Act over it could be performed on its layout:\n%s", body)
	}
	// AND IT IS THE PRESS THAT WAS INTERPRETED, asserted by its value: a point here
	// that was not one of the demonstrated ones would be something computed.
	for i, st := range a.Steps {
		if st.Control == "" {
			if st.At != nil {
				t.Errorf("step %d changed the screen by %s and carries a "+
					"click at %+v", i, st.Cause, *st.At)
			}
			continue
		}
		if st.At == nil {
			t.Errorf("step %d presses %s and records no click", i, st.Control)
			continue
		}
		if *st.At != onGraphics {
			t.Errorf("step %d records a click at %+v; the person pressed %+v",
				i, *st.At, onGraphics)
		}
	}
}

// AND THE REPORT TELLS THE PERSON WHAT THEY PRESSED THAT MARCO COULD NOT SEE.
//
// As the act they performed, not as a number on a tally. The live run that prompted this said
// `1 landed on nothing the proposer found`, which reads as the person having clicked wrongly when
// what happened is that Marco could not see what they clicked.
func TestTheReportSaysWhatYouPressedThatMarcoCouldNotSee(t *testing.T) {
	o, _, onNothing := aSessionWithAMistake(t)
	reg := o.Registry()
	draft := workflow.Learn("ark graphics", o.Walk(), o.Clicks(), reg)

	var out bytes.Buffer
	reportVisualDemonstration(&out, o, visualResult{in: strayKeys()}, draft, nil)
	got := out.String()

	for _, want := range []string{
		"You pressed these, and Marco saw nothing there to press",
		"Kept as evidence of what you did",
		"None of them can be replayed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not say %q:\n%s", want, got)
		}
	}
	// AND WHERE, so a person can recognise which of their own clicks it was. Shown in
	// percentages of the window, which is how it was recorded.
	if !strings.Contains(got, "88.0%") {
		t.Errorf("the report does not say where the press was made (%.1f%%):\n%s",
			onNothing.X*100, got)
	}
	// AND THE SCREEN IT WAS MADE ON, because a coordinate without one is not an act.
	if !strings.Contains(got, string(reg[0].Screen)) {
		t.Errorf("the report does not name the screen the press was made on:\n%s", got)
	}
	// AND THE DEMONSTRATION IS NOT REPORTED AS A FAILURE. The control that did work is
	// still there, which is the whole point of keeping the two apart.
	if !strings.Contains(got, "Controls, and what each press was taken to be") {
		t.Errorf("a session with one unsegmentable press reports no controls:\n%s", got)
	}
	if strings.Contains(got, "No control was identified") {
		t.Errorf("a session that learned a control says it learned none:\n%s", got)
	}
}

// AND A CLEAN SESSION SAYS NONE OF IT.
//
// Without this the block could print unconditionally, which would tell every person that Marco
// failed to see something whether or not it had.
func TestACleanSessionDoesNotMentionPressesMarcoCouldNotSee(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	menu := twiceMenuRegions()
	at := observe.PointerAt{X: 0.5, Y: 0.425}
	t0 := time.Unix(1700000000, 0).UTC()
	n := 0
	look := func(regions []observe.Region, presses ...observe.PointerAt) {
		n++
		o.Observe(fastobserve.Look{At: t0.Add(time.Duration(n) * 100 * time.Millisecond),
			Window:  fastobserve.WindowRef{Generation: 1},
			Regions: regions, Front: true, Presses: presses})
	}
	for i := 0; i < o.SettleLooks()+1; i++ {
		look(menu)
	}
	look(menu, at)
	for i := 0; i < o.SettleLooks()+1; i++ {
		look(twiceSettingsRegions())
	}
	o.Close()

	if ev := o.Clicks(); len(ev.BlindPresses) != 0 {
		t.Fatalf("%d blind press(es) in a session where every press landed",
			len(ev.BlindPresses))
	}
	var out bytes.Buffer
	reportVisualDemonstration(&out, o, visualResult{}, workflow.Learn("clean", o.Walk(),
		o.Clicks(), o.Registry()), nil)
	if strings.Contains(out.String(), "Marco saw nothing there to press") {
		t.Errorf("a session where every press landed reports one that did not:\n%s",
			out.String())
	}
}

// ── THE RAW DEMONSTRATION, THROUGH THE PATH A PERSON ACTUALLY TEACHES THROUGH ──
//
// The owner's second finding on the same report, and the one they called the biggest
// architectural gap:
//
//	Marco remembers what it thinks the user taught it better than it remembers exactly what
//	the user did. That needs to change before we invest much more in interpretation.
//
// So this drives the real observer, the real correlator, the real artifact AND the real store,
// and asserts the two records exist side by side with the right things in each: every press
// with its coordinate in the raw record, and no press coordinate in the executable one.

// THE RECORD KEEPS EVERY PRESS, INCLUDING THE ONE MARCO UNDERSTOOD.
func TestTheRawRecordKeepsEveryPressTheSessionSaw(t *testing.T) {
	o, onGraphics, onNothing := aSessionWithAMistake(t)
	ev := o.Clicks()

	// THREE PRESSES WERE MADE: the button twice and the background once.
	if ev.Presses != 3 {
		t.Fatalf("%d press(es) reached the correlator, and three were made", ev.Presses)
	}
	// AND THE RECORD OF THE ACT HAS ALL THREE, which is the property that was missing:
	// one landed on nothing, two resolved to a control, and only the first kind was kept.
	if len(ev.Observations) != ev.Presses {
		t.Fatalf("%d observation(s) for %d press(es); the raw record and the count "+
			"cannot disagree", len(ev.Observations), ev.Presses)
	}
	var onButton, onBackground int
	for _, p := range ev.Observations {
		switch p.At {
		case onGraphics:
			onButton++
			if p.Target == "" {
				t.Errorf("the press on the GRAPHICS tab resolved to no control")
			}
		case onNothing:
			onBackground++
			if p.Landed() {
				t.Errorf("the press on bare background landed on %q", p.Target)
			}
		default:
			t.Errorf("a press at %v was recorded and none was made there", p.At)
		}
	}
	if onButton != 2 || onBackground != 1 {
		t.Errorf("%d press(es) on the button and %d on the background, want 2 and 1",
			onButton, onBackground)
	}
	// AND THEY ARE IN THE ORDER THEY WERE MADE. The mistake came first.
	if ev.Observations[0].At != onNothing {
		t.Errorf("the first press recorded is at %v and the first press made was at %v",
			ev.Observations[0].At, onNothing)
	}
}

// AND THE TWO RECORDS LAND IN THE STORE SIDE BY SIDE, each with what belongs in it.
//
// Through `demo.Open` and the real save paths, because the one defect this project keeps
// finding is production code that is never invoked: a record assembled perfectly and written
// by nothing would pass every test above.
func TestTheRoutineAndTheRawDemonstrationAreBothSaved(t *testing.T) {
	o, _, onNothing := aSessionWithAMistake(t)
	reg, ev := o.Registry(), o.Clicks()
	draft := workflow.Learn("ark graphics", o.Walk(), ev, reg)

	arr := map[observe.VisualScreenID]observe.VisualArrangement{}
	for _, sc := range o.Screens() {
		arr[sc.ID] = o.Arrangement(sc.ID)
	}
	a, err := workflowstore.SaveWith(draft, arr, nil,
		time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	// THE PRODUCTION ASSEMBLY, not a fixture: the same call the command makes.
	raw := rawDemonstration(a, "arkascended", "", o, draftOf(o), visualResult{
		in: strayKeys(), looks: 40,
		generations: map[uint64]bool{1: true},
		moments: []workflowstore.Moment{{Look: 7, Screen: string(reg[0].Screen), AtMS: 700,
			Inputs: []observe.InputEvent{
				{Intent: observe.NavBack, AtMS: 700},
				{Intent: observe.NavBack, AtMS: 760, Edge: observe.EdgeRelease},
			}}},
	})
	if err := store.SaveEvidence(raw); err != nil {
		t.Fatal(err)
	}

	// ── READ BACK, AND THE LINK IS CHECKED ──
	saved, ok := store.Sequence("ark graphics")
	if !ok {
		t.Fatal("the play was not saved")
	}
	back, ok := store.Evidence(saved)
	if !ok {
		t.Fatal("the raw demonstration was not saved beside it")
	}
	if len(back.Presses) != 3 {
		t.Errorf("%d press(es) in the record of 3", len(back.Presses))
	}
	// EACH PRESS SAYS WHAT WAS THERE, which is the difference between the one Marco
	// understood and the one it could not segment — and it is the ROUTINE's own label,
	// not a session identity, so it points at a real entry in the file beside it.
	mistake, button := 0, 0
	for _, p := range back.Presses {
		switch p.At {
		case onNothing:
			mistake++
			if p.Screen == "" {
				t.Error("the kept press has no screen, so it is a " +
					"coordinate rather than an act")
			}
			if p.Control != "" || p.Containing != 0 {
				t.Errorf("the press nothing contained reads as %q inside "+
					"%d region(s)", p.Control, p.Containing)
			}
		default:
			button++
			if p.Control != "control_1" {
				t.Errorf("the press on the button reads as %q, and the "+
					"play calls that control %q", p.Control,
					back.Presses[0].Control)
			}
			if p.Containing < 1 {
				t.Errorf("the press on the button says %d region(s) "+
					"contained it", p.Containing)
			}
		}
	}
	if mistake != 1 || button != 2 {
		t.Errorf("%d unsegmented press(es) and %d on the button, want 1 and 2",
			mistake, button)
	}
	// AND THE KEYSTROKE IS A PRESS WITH A DURATION, not a line in an ignore table.
	ks := back.Keystrokes()
	if len(ks) != 1 || ks[0].HeldMS != 60 {
		t.Fatalf("%d keystroke(s) %+v, want one held 60ms", len(ks), ks)
	}

	// ── AND THE EXECUTABLE ARTIFACT CARRIES ONLY THE INTERPRETED CLICK ──
	//
	// The two files are the safety argument, and narrowing the rule did not change it:
	// the raw record holds EVERYTHING the person did, including the press on bare
	// background that Marco could not segment, and the artifact holds the one point per
	// interpreted step that [[ADR-248-replaying-a-demonstrated-layout]] authorised. The
	// mistake is the test: it is in one file and not the other.
	body, err := saved.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"0.88", "observations"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the saved play contains %q, which belongs to the raw "+
				"record and not to the executable artifact:\n%s",
				forbidden, body)
		}
	}
	if !strings.Contains(string(body), "\"at\"") {
		t.Errorf("the saved play carries no demonstrated click:\n%s", body)
	}
	// AND THE RECORD OF AN EARLIER SESSION IS NOT READ AS A LATER ONE'S.
	later := saved
	later.SavedAt = later.SavedAt.Add(time.Hour)
	if _, ok := store.Evidence(later); ok {
		t.Error("evidence of one save was returned for another save of the same name")
	}
}

// AND THE REPORT PRINTS THE RECORD AS ACTS, which is what a person reads it for.
func TestWhatIDidSaysThePressAndTheHoldAndNotACounter(t *testing.T) {
	o, _, _ := aSessionWithAMistake(t)
	a := workflowstore.Artifact{Name: "ark graphics",
		SavedAt: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)}
	raw := rawDemonstration(a, "arkascended", "", o, draftOf(o), visualResult{
		in: strayKeys(), looks: 40,
		generations: map[uint64]bool{1: true},
		moments: []workflowstore.Moment{{Look: 7, AtMS: 700,
			Inputs: []observe.InputEvent{
				{Intent: observe.NavBack, AtMS: 700},
				{Intent: observe.NavBack, AtMS: 1400, Edge: observe.EdgeRelease},
			}}},
	})
	var out bytes.Buffer
	reportRawDemonstration(&out, a, raw)
	got := out.String()

	for _, want := range []string{
		"WHAT YOU DID",
		"arkascended",
		// THE PRESS NOBODY SEGMENTED, said as a boundary problem and not as a bad click.
		"nothing was proposed there",
		// THE HOLD, as a duration.
		"held 700ms",
		// AND WHERE THE MISTAKE WAS MADE.
		"88.0%",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not say %q:\n%s", want, got)
		}
	}
	// AND IT POINTS AT THE OTHER RECORD rather than pretending to be it.
	if !strings.Contains(got, "director sequences") {
		t.Errorf("the raw report does not say where the interpretation is:\n%s", got)
	}
}

// ── THE MISTAKE, CORRECTED, WITH THE DEMONSTRATION STILL ON RECORD ──
//
// The owner's fifth scenario and the one the other four exist to make possible:
//
//	An accidental click followed by a correction.
//	The raw evidence must remain unchanged after correction. The learned procedure must
//	reflect the correction.
//
// Through the real store, the real artifact and the real correction path: demonstrate, save
// both records, correct the procedure, and read the raw record back afterwards.
func TestAnAccidentalClickIsCorrectedAndTheDemonstrationSurvivesIt(t *testing.T) {
	o, _, onNothing := aSessionWithAMistake(t)
	reg, ev := o.Registry(), o.Clicks()
	draft := workflow.Learn("ark graphics", o.Walk(), ev, reg)
	arr := map[observe.VisualScreenID]observe.VisualArrangement{}
	for _, sc := range o.Screens() {
		arr[sc.ID] = o.Arrangement(sc.ID)
	}
	a, err := workflowstore.SaveWith(draft, arr, nil,
		time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEvidence(rawDemonstration(a, "arkascended", "", o, draft,
		visualResult{in: strayKeys(), looks: 40,
			generations: map[uint64]bool{1: true}})); err != nil {
		t.Fatal(err)
	}
	before := len(a.Steps)
	if before == 0 {
		t.Fatal("the demonstration produced no step to correct")
	}

	// ── THE CORRECTION, as `director correct --drop last` makes it ──
	corrected, err := a.Dropping(before, "that click was accidental",
		time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(corrected); err != nil {
		t.Fatal(err)
	}

	// ── THE PROCEDURE REFLECTS IT ──
	back, ok := store.Sequence("ark graphics")
	if !ok {
		t.Fatal("the corrected play was not saved")
	}
	if len(back.Steps) != before-1 {
		t.Errorf("%d step(s) after the correction, want %d", len(back.Steps), before-1)
	}
	if !back.Corrected() || back.Corrections[0].Said != "that click was accidental" {
		t.Errorf("the correction is not recorded in the play: %+v", back.Corrections)
	}

	// ── AND THE DEMONSTRATION IS UNCHANGED, which is the half that matters ──
	raw, ok := store.Evidence(back)
	if !ok {
		t.Fatal("the raw demonstration was lost when the procedure was corrected; a " +
			"correction must not falsify the recording")
	}
	if len(raw.Presses) != 3 {
		t.Errorf("%d press(es) in the record after the correction, and three were made",
			len(raw.Presses))
	}
	found := false
	for _, p := range raw.Presses {
		if p.At == onNothing {
			found = true
		}
	}
	if !found {
		t.Error("the press Marco could not segment is gone from the raw record")
	}
	// AND THE REPORT STILL SHOWS IT, which is what the person reads to check.
	var out bytes.Buffer
	reportRawDemonstration(&out, back, raw)
	if !strings.Contains(out.String(), "88.0%") {
		t.Errorf("the raw report no longer says where the mistake was made:\n%s",
			out.String())
	}
}

// AND THE CORRECTION REPORT SAYS WHAT CAME OUT, WHAT IS LEFT, AND WHAT IS KEPT.
func TestTheCorrectionReportSaysWhatIsKeptAsWellAsWhatIsGone(t *testing.T) {
	a := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Steps: []workflowstore.Step{
			{Position: 0, From: "menu", To: "settings", Control: "control_1",
				Cause: observe.CauseClick, Seen: 1},
			{Position: 1, From: "settings", To: "graphics", Cause: observe.CauseUnknown,
				Seen: 1},
		},
	}
	after, err := a.Dropping(2, "the menu did that by itself", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw := workflowstore.Evidence{Name: a.Name, SavedAt: a.SavedAt,
		Presses: []workflowstore.DemonstratedPress{{Look: 4,
			At: observe.PointerAt{X: 0.4, Y: 0.4}, Screen: "screen_a"}},
		Moments: []workflowstore.Moment{{Look: 4, Inputs: []observe.InputEvent{
			{Intent: observe.NavBack, AtMS: 400},
			{Intent: observe.NavBack, AtMS: 460, Edge: observe.EdgeRelease},
		}}}}

	var out bytes.Buffer
	reportCorrection(&out, a, after, 2, true, raw)
	got := out.String()
	for _, want := range []string{
		"CORRECTING",
		// WHAT CAME OUT, and a step with no control is named as what it is rather than
		// as a missing field.
		"the screen changed for another reason",
		// WHAT IS LEFT.
		"control_1",
		// AND WHAT IS KEPT: the demonstration, counted, with where to read it.
		"1 press(es) and 1 keystroke(s)",
		"director what-i-did",
		"the step itself, with your reason",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the correction report does not say %q:\n%s", want, got)
		}
	}
}

// AND A ROUTINE WITH NO RAW RECORD IS STILL CORRECTABLE, without claiming to keep one.
//
// A routine taught before the record existed has none. Refusing to correct it would make the
// new capability a reason the old routines cannot be fixed.
func TestARoutineWithNoRawRecordIsStillCorrectable(t *testing.T) {
	a := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "old tour",
		SavedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		Steps: []workflowstore.Step{
			{Position: 0, From: "a", To: "b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 1},
		},
	}
	after, err := a.Dropping(1, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	reportCorrection(&out, a, after, 1, false, workflowstore.Evidence{})
	got := out.String()
	if strings.Contains(got, "director what-i-did") {
		t.Errorf("a play with no raw record is told to go and read one:\n%s", got)
	}
	if !strings.Contains(got, "performs no step") {
		t.Errorf("a play with nothing left does not say so:\n%s", got)
	}
}

// draftOf is the draft a session would have produced, for the tests that need the label map.
//
// The raw record speaks the ARTIFACT's language — `screen_a`, `control_1` — so building one
// needs the same draft the artifact was built from. Named rather than inlined because three
// tests need it and a different draft would produce different labels.
func draftOf(o *fastobserve.Observer) workflow.Draft {
	return workflow.Learn("ark graphics", o.Walk(), o.Clicks(), o.Registry())
}

// AND A MOMENT WHEN SOMETHING ELSE WAS IN FRONT IS NAMED, NOT DISCARDED.
//
// The owner's list of what a raw demonstration must preserve includes focus changes, and this
// is the form the evidence takes: a window capture is not occlusion-safe, so a look taken while
// something covered the target can hold another application's pixels under the target's own
// name — and an input that arrived on one of those looks may never have reached the application.
// It is kept and reported; the person is told which part of their demonstration Marco cannot
// vouch for.
func TestAMomentBehindAnotherWindowIsNamedAndNotDiscarded(t *testing.T) {
	a := workflowstore.Artifact{Name: "ark graphics",
		SavedAt: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)}
	raw := workflowstore.Evidence{Name: a.Name, SavedAt: a.SavedAt,
		Window: workflowstore.WindowWatched{Application: "arkascended"},
		Looks:  40,
		Moments: []workflowstore.Moment{
			{Look: 7, AtMS: 700, Front: true, Inputs: []observe.InputEvent{
				{Intent: observe.NavBack, AtMS: 700},
			}},
			{Look: 19, AtMS: 1900, Front: false, Inputs: []observe.InputEvent{
				{Intent: observe.NavConfirm, AtMS: 1900},
			}},
		}}

	var out bytes.Buffer
	reportRawDemonstration(&out, a, raw)
	got := out.String()
	for _, want := range []string{
		"Something else was in front for 1 of the 2 moment(s)",
		"look(s) 19",
		"may not have reached",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not say %q:\n%s", want, got)
		}
	}
	// AND THE INPUT IS STILL IN THE RECORD. Naming it is not dropping it.
	if n := len(raw.Inputs()); n != 2 {
		t.Errorf("%d event(s) survived of 2", n)
	}
	// AND A SESSION THAT WAS IN FRONT THROUGHOUT SAYS NONE OF IT.
	raw.Moments[1].Front = true
	var clean bytes.Buffer
	reportRawDemonstration(&clean, a, raw)
	if strings.Contains(clean.String(), "Something else was in front") {
		t.Errorf("a session that held the foreground reports losing it:\n%s",
			clean.String())
	}
}
