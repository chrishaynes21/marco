//go:build windows

package main

// SCENARIO: A SEEDED ROUTINE, PLAYED THROUGH THE REAL STUDIO, ASSERTED AGAINST THE APPLICATION.
//
// # The spine
//
//	a window to press things in          plugins/fixture, started by this harness
//	a routine in the library             seeded through the production generator
//	the real Studio, on this run's store the actual binary, the actual page
//	Scenes → the card → Play             pressed through UI Automation, not called
//	`marco do`, spawned by Studio        real input, through the real executor
//	what the application received        the fixture's own log, which Marco cannot read
//	what Marco says it did               the run record, written by the child
//
// Nine things in that list can fail independently and five of them have failed in this product's
// history. None of them is reachable from a unit test.
//
// # What makes the verdict mean something
//
// Two accounts, never crossed. The fixture says a left button went down on the control called
// Save, at a point four pixels from its centre, while its window was at a particular place. Marco
// says it performed a routine. A run where both are true is the only one this calls passed —
// and the case that matters most is the one where Marco is confident and the application received
// nothing, which no single channel can see.
//
// # The order of the questions is the design
//
// The first disagreement is the finding; everything after it is consequence. So they are asked
// from the act outwards: was anything received at all, was it on the right control, in the right
// place, exactly once, did the application change, and only then what Marco says about it. A
// report that led with "Marco said performed and save_presses is 0" would be stating the
// consequence of the press not happening and calling it the defect.

import (
	"fmt"
	"strings"
	"time"
)

func scenarios() []scenario {
	return []scenario{
		{
			name:  "play/seeded routine presses the application",
			layer: layerDesktop,
			about: "a seeded routine played from Studio's own window, asserted " +
				"against what the fixture received",
			run: playASeededRoutine,
		},
	}
}

func playASeededRoutine(w *world) result {
	started := time.Now()

	f, err := startFixture(w.b, w.art, false)
	if err != nil {
		return ending(started, err)
	}
	defer f.close()

	seed, err := seedPressOf(f, w.art, "fixture save", "Save")
	if err != nil {
		return ending(started, err)
	}

	s, err := startStudio(w.b, w.art, w.store)
	if err != nil {
		return ending(started, err)
	}
	defer s.close()

	// THE WINDOW HAS TO OFFER THE ROUTINE BEFORE ANYTHING CAN PLAY IT.
	//
	// Through the rail, like a person: the routines live on the Scenes surface and the
	// others are not in the accessibility tree at all while they are hidden — which is
	// correct, and is why a harness that went looking for the Play button first would
	// report that Studio has no such control.
	if err := s.surface("scenes", "capBtn"); err != nil {
		return fail(started, err.Error())
	}
	if err := s.chooseRoutine(routeIdentity(seed), seed.name); err != nil {
		s.keepRunView()
		w.art.picture("failure.png", 0, 0, w.machine.ScreenW, w.machine.ScreenH)
		return fail(started, err.Error())
	}
	// AND THE BUTTON HAS TO BE OFFERED, NOT JUST PRESENT. A Play button on screen and
	// disabled is the window refusing, and pressing it would be a call that succeeds and
	// presses nothing.
	if _, err := s.waitFor(10*time.Second, "routinePlayBtn", enabled); err != nil {
		s.keepRunView()
		return fail(started, fmt.Sprintf("Studio will not offer to play the seeded "+
			"routine: %v", err))
	}

	// WHERE BOTH ACCOUNTS STAND BEFORE ANYTHING HAPPENS.
	//
	// Read now rather than after, because a count that was never zero proves nothing about
	// a run — and a fixture restored from a previous session's state file once produced a
	// phantom press for exactly that reason.
	before, err := f.read()
	if err != nil {
		return fail(started, err.Error())
	}
	if before.SavePresses != 0 {
		return fail(started, fmt.Sprintf("the fixture had already received %d press(es) "+
			"of Save before this run started, so nothing it reports afterwards is "+
			"about the run", before.SavePresses))
	}
	seq := f.lastSeq()
	recordsBefore := countRecords(w.store)

	// AND STUDIO HAS TO BE THE WINDOW IN FRONT.
	//
	// Not for the harness's benefit: `PlayRoutine` raises the application the routine is
	// for, and Windows refuses the foreground to a process that does not already hold it.
	// Studio holds it legitimately. If it does not, this stops — input that could land in
	// somebody else's window is input this run may not send.
	if err := s.front(); err != nil {
		return skip(started, err.Error())
	}

	w.art.step("pressing Play")
	if err := s.press("routinePlayBtn"); err != nil {
		return fail(started, err.Error())
	}

	// ── WHAT THE APPLICATION RECEIVED ────────────────────────────────────────
	got, arrived := f.waitForPress(45*time.Second, seq)
	if !arrived {
		s.keepRunView()
		w.art.picture("failure.png", 0, 0, w.machine.ScreenW, w.machine.ScreenH)
		w.art.copyIn("fixture-state-after.json", f.state)
		rec, recErr := waitForRecord(5*time.Second, w.store, recordsBefore)
		said := "and Marco wrote no finished run record either"
		if recErr == nil {
			said = fmt.Sprintf("while Marco wrote %s", rec)
		}
		return fail(started, fmt.Sprintf("the application received NOTHING after Play "+
			"was pressed, %s", said))
	}
	w.art.step("the fixture received %s", got)

	// ── AND WHAT MARCO SAYS IT DID ───────────────────────────────────────────
	rec, err := waitForRecord(45*time.Second, w.store, recordsBefore)
	if err != nil {
		s.keepRunView()
		return fail(started, fmt.Sprintf("the application received %s, and Marco wrote "+
			"no finished run record: %v", got, err))
	}
	w.art.step("Marco recorded %s", rec)
	w.art.copyIn("run-record.json", rec.file)
	after, err := f.read()
	if err != nil {
		return fail(started, err.Error())
	}
	w.art.copyIn("fixture-state-after.json", f.state)
	presses, err := f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}

	// THE QUESTIONS, FROM THE ACT OUTWARDS.
	save := before.Controls["Save"]
	why := firstOf(
		onTheWrongControl(got, "Save"),
		outsideTheControl(got, before, save),
		notExactlyOne(presses),
		theApplicationDidNotChange(before, after),
		marcoSaidSomethingElse(rec, seed),
	)
	if why != "" {
		s.keepRunView()
		w.art.picture("failure.png", 0, 0, w.machine.ScreenW, w.machine.ScreenH)
		return fail(started, why,
			fmt.Sprintf("the fixture received %s", got),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("%q pressed %s at %d,%d — %d,%d into the control", seed.name,
			got.Control, got.ScreenX, got.ScreenY, got.OnControlX, got.OnControlY),
		fmt.Sprintf("the application's own count went %d → %d", before.SavePresses,
			after.SavePresses),
		fmt.Sprintf("Marco's record: %s", rec),
	)
}

// routeIdentity is the route's whole identity, in the form the page uses for a card.
//
// The same three scopes `internal/routes` keeps on disk, because that IS what distinguishes two
// routes of one name. Built here rather than read off the page, so that a mismatch between what
// the harness asks for and what the window offers is a failure rather than a thing the harness
// quietly accommodates.
func routeIdentity(s seededRoutine) string {
	scope := "context"
	if s.route.Focus {
		scope = "focus"
	}
	if s.route.App == "" {
		return "global/" + s.route.Slug
	}
	return s.route.App + "/" + scope + "/" + s.route.Slug
}

// ── THE DISAGREEMENTS, EACH IN ITS OWN SENTENCE ───────────────────────────────

func onTheWrongControl(got fixtureEvent, want string) string {
	if got.Control == want {
		return ""
	}
	return fmt.Sprintf("the press landed on %q and the routine was recorded against "+
		"%q — so it arrived, in the right window, at the wrong place", got.Control, want)
}

// outsideTheControl catches a press credited to a control it is not actually inside.
//
// Both channels, deliberately. The control name comes from which window handle Windows delivered
// the message to; the rectangle comes from the fixture's own layout. They agree for an ordinary
// press, and the one case where they would not is the case worth naming: a coordinate that landed
// on an overlapping control, a tooltip, or a window that moved between the reading and the press.
func outsideTheControl(got fixtureEvent, held fixtureState, save control) string {
	if save.W == 0 || save.H == 0 {
		return ""
	}
	x, y := got.ClientX-save.X, got.ClientY-save.Y
	if x >= 0 && y >= 0 && x < save.W && y < save.H {
		return ""
	}
	return fmt.Sprintf("the press was credited to the Save control and landed at %d,%d "+
		"in the window, which is outside Save's own rectangle %d,%d %dx%d — the two "+
		"accounts of where it went disagree", got.ClientX, got.ClientY, save.X, save.Y,
		save.W, save.H)
}

func notExactlyOne(presses []fixtureEvent) string {
	if len(presses) == 1 {
		return ""
	}
	var where []string
	for _, p := range presses {
		where = append(where, p.String())
	}
	return fmt.Sprintf("the routine presses once and the application received %d "+
		"press(es): %s", len(presses), strings.Join(where, "; "))
}

func theApplicationDidNotChange(before, after fixtureState) string {
	if after.SavePresses == before.SavePresses+1 {
		return ""
	}
	return fmt.Sprintf("the application received the press and its own count of Save "+
		"went %d → %d, which is not one more. The press arrived and the application "+
		"did not act on it", before.SavePresses, after.SavePresses)
}

// marcoSaidSomethingElse is MARCO's half, asked last.
//
// Last because it is the account that can be wrong while everything else is right, and a report
// that led with it would be putting Marco's opinion ahead of what happened. Every field here is
// one the record is supposed to carry: which routine, under what contract, with what stated, and
// how many input actions were asked for.
func marcoSaidSomethingElse(rec runRecord, seed seededRoutine) string {
	switch {
	case rec.Outcome != "performed":
		return fmt.Sprintf("the application received the press and acted on it, and "+
			"Marco recorded the run as %q (%s)", rec.Outcome, rec.Because)
	case rec.Routine != seed.route.Slug:
		return fmt.Sprintf("the run record is about %q and the routine played was %q",
			rec.Routine, seed.route.Slug)
	case rec.Application != seed.route.App:
		return fmt.Sprintf("the run record names application %q and the routine is "+
			"saved for %q", rec.Application, seed.route.App)
	case rec.Contract != seed.contract:
		return fmt.Sprintf("the routine was saved as %s and the run record says it "+
			"was performed as %s", seed.contract, rec.Contract)
	case rec.Mode != "coordinates":
		return fmt.Sprintf("the routine states that it was recorded as coordinates "+
			"and the run record says %q", rec.Mode)
	case rec.Build == "":
		return "the run record does not say which marco performed it, which is the " +
			"field a stale binary hides behind"
	case rec.Asked == nil:
		return "the run record does not say how many input actions were asked for, " +
			"so nothing it says about pressing can be corroborated"
	case *rec.Asked == 0:
		return "the run record says Marco asked the machine for 0 input actions, and " +
			"the application received a press — so something other than this run " +
			"pressed it"
	}
	return ""
}
