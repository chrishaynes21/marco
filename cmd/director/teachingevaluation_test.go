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
	o, _, onNothing := aSessionWithAMistake(t)
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

	// AND NO PRESS COORDINATE REACHED THE SAVED WORKFLOW. The evidence is kept in the
	// session, where it belongs; the artifact stays unable to act on a coordinate, which
	// is ADR-181 and ADR-183 unchanged.
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"0.88", "blind_presses", "\"at\""} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the saved workflow contains %q:\n%s", forbidden, body)
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
