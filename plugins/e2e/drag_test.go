//go:build windows

package main

// THE DRAG SCENARIOS, AND WHAT CAN BE CHECKED ABOUT THEM WITHOUT A DESKTOP.
//
// All four ran green on a desktop on 2026-10-27, so the gate is off and the inverse is asserted
// instead: they are ordinary members of a run, filed where a flag typed for something else will
// not sweep them up, and the sentences their verdicts rest on say what they claim to.
//
// What getting them there took is worth keeping, because none of it was in the drag path. The
// first run failed all four on their own precondition — the contract reader knew one guarded
// shape and a drag is another, so a guarded drag read back as legacy. The next two were the
// harness's own latency against a gesture that lasts about 190ms: see
// `TestTheHarnessNoticesAMouseEventInsideAGesture` and `studioRun.stopNow`. A claim can be
// correct, tested, and never reached.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// THE DRAG SCENARIOS RUN ORDINARILY, AND THEY ARE NOT UNDER THE PRESS'S SELECTOR.
//
// The gate half of this test is gone and its going is the point: all four ran green on a desktop
// on 2026-10-27, so they are regression checks now and belong in every default run. What is
// asserted instead is the inverse — re-gating one would take it out of default runs, and the only
// evidence anybody would see of that is a line in a report nobody reads twice. The holds' shape,
// one slice over.
//
// The NAME claim is a separate decision and outlives the gate: one filed under `window/` — where
// two of them would sit naturally, since they are about a window that moved or was resized —
// would be selected by `-only window`, which is the guarded press's regression gate.
func TestTheDragScenariosRunLikeAnyOther(t *testing.T) {
	var found int
	for _, s := range scenarios() {
		if !strings.HasPrefix(s.name, "drag/") {
			continue
		}
		found++
		if s.onlyWhenAsked {
			t.Errorf("%q has run green on a desktop and is gated out of a default "+
				"run again, so the regression it was written to catch is only "+
				"checked when somebody remembers to name it", s.name)
		}
		if s.layer != layerDesktop {
			t.Errorf("%q is labelled %q and it needs a desktop", s.name, s.layer)
		}
		// AND NOT UNDER ANYBODY ELSE'S SELECTOR, asked of EVERY other family
		// rather than of the one that was obviously a risk.
		//
		// `window` was the obvious one and two of these names failed on it: two of
		// the four are about a window that moved or was resized, which is how they
		// want to be named and is exactly what `-only window` means to somebody
		// changing the guarded press. Checking only the obvious collision is how the
		// next one gets through, so this asks about all of them.
		for _, family := range families(t) {
			if family == "drag" {
				continue
			}
			if strings.Contains(s.name, family) {
				t.Errorf("%q contains %q, so `-only %s` — another "+
					"family's regression gate — would select it",
					s.name, family, family)
			}
		}
		if s.about == "" || s.run == nil {
			t.Errorf("%q is not a runnable, described scenario", s.name)
		}
	}
	if found != 4 {
		t.Errorf("%d drag scenario(s) are registered, want 4: the gesture, the moved "+
			"window, the resized window, and the stop", found)
	}
}

// ── AND WHAT THE VERDICTS REST ON ─────────────────────────────────────────────

// aDragLog is a gesture as the fixture would log one: a press, some movement, a release.
func aDragLog(window rect, from, to [2]int, moves int) []fixtureEvent {
	at := func(seq int, what string, x, y int) fixtureEvent {
		return fixtureEvent{Seq: seq, Kind: "mouse", What: what, Control: "Name",
			ScreenX: x, ScreenY: y, Window: window}
	}
	out := []fixtureEvent{at(1, "left_down", from[0], from[1])}
	for i := 1; i <= moves; i++ {
		p := float64(i) / float64(moves+1)
		out = append(out, at(1+i, "move",
			from[0]+int(float64(to[0]-from[0])*p),
			from[1]+int(float64(to[1]-from[1])*p)))
	}
	return append(out, at(2+moves, "left_up", to[0], to[1]))
}

var theTestWindow = rect{X: 100, Y: 100, W: 800, H: 600}

// THE SHAPE IS A PRESS, MOVEMENT, AND A RELEASE — AND NOTHING ELSE BETWEEN THEM.
//
// The last part is what the obvious version of this check misses: asserting the first event is a
// press and the last is a release is satisfied by TWO drags in a row, which is a routine doing
// twice what it was asked.
func TestTheGestureShapeIsAskedOfTheWholeLog(t *testing.T) {
	good := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 5)
	if why := theGestureDidNotArrive(good); why != "" {
		t.Errorf("an ordinary drag is reported as not having arrived: %s", why)
	}
	for _, c := range []struct {
		what string
		got  []fixtureEvent
		says string
	}{
		{"nothing at all", nil, "received nothing at all"},
		{"a release with no press", good[len(good)-1:], "begins with its button"},
		{"a press with no release", good[:len(good)-1], "is still down"},
		{"two gestures in a row", append(append([]fixtureEvent{}, good...), good...),
			"more than one gesture"},
	} {
		t.Run(c.what, func(t *testing.T) {
			why := theGestureDidNotArrive(c.got)
			if why == "" {
				t.Fatalf("%s is reported as an arrived drag", c.what)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the sentence is %q and does not say %q", why, c.says)
			}
		})
	}
}

// AND THE TRAVEL IS REQUIRED WITHOUT A COUNT.
//
// Windows coalesces mouse moves, so the number that arrives depends on how busy the machine is —
// a scenario asserting one would be asserting the scheduler and would fail with nothing wrong.
// What cannot be coalesced away is that movement happened at all, and a gesture with none is two
// clicks, which is what a teleport looks like.
func TestTheTravelIsRequiredAndNotCounted(t *testing.T) {
	for _, moves := range []int{1, 3, 24} {
		got := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, moves)
		if why := theTravelWasNotReceived(got); why != "" {
			t.Errorf("a drag whose travel arrived as %d move(s) is reported as "+
				"not having travelled: %s", moves, why)
		}
	}
	// AND NONE AT ALL IS THE ONE THAT FAILS.
	teleport := []fixtureEvent{
		{Seq: 1, Kind: "mouse", What: "left_down", ScreenX: 200, ScreenY: 200},
		{Seq: 2, Kind: "mouse", What: "left_up", ScreenX: 600, ScreenY: 400},
	}
	why := theTravelWasNotReceived(teleport)
	if why == "" {
		t.Fatal("a press and a release at two places is reported as a drag, which " +
			"is the one thing this check exists to tell apart")
	}
	if !strings.Contains(why, "teleport") {
		t.Errorf("the sentence is %q and does not say what arrived instead", why)
	}
	// AND A TELEPORT WITH A MOVE EVENT IN IT IS THE SAME TELEPORT.
	//
	// The case a reader that counted movement would pass, and the reason the claim is
	// about a point between the ends rather than about a number: a replay that jumped
	// straight to the far end sends a move of its own, at the destination, and then lets
	// go there. One move arrived, no travel did.
	for _, c := range []struct {
		what  string
		moved [2]int
	}{
		{"one jump to the destination", [2]int{600, 400}},
		{"one move that never left the press", [2]int{200, 200}},
	} {
		t.Run(c.what, func(t *testing.T) {
			jumped := []fixtureEvent{
				{Seq: 1, Kind: "mouse", What: "left_down", ScreenX: 200,
					ScreenY: 200},
				{Seq: 2, Kind: "mouse", What: "move", ScreenX: c.moved[0],
					ScreenY: c.moved[1]},
				{Seq: 3, Kind: "mouse", What: "left_up", ScreenX: 600,
					ScreenY: 400},
			}
			why := theTravelWasNotReceived(jumped)
			if why == "" {
				t.Fatalf("%s is reported as travel, so a gesture that never "+
					"went between its ends reads as a drag", c.what)
			}
			if !strings.Contains(why, "BETWEEN") {
				t.Errorf("the sentence is %q and does not say what was "+
					"missing", why)
			}
			// AND THE OTHER READING IS NAMED. A log with no intermediate
			// point is also what an application that never ran during the
			// travel produces, and that is a reason to look at the machine
			// rather than at Marco. A sentence that convicted the gesture
			// of teleporting would send somebody to the wrong place.
			if !strings.Contains(why, "never ran") {
				t.Errorf("the sentence is %q and names only one of the two "+
					"things this log can mean", why)
			}
		})
	}
}

// AND EVERY POINT HAS TO BE INSIDE THE WINDOW, NOT JUST THE ENDS.
//
// A path that left the window and came back would have dragged across whatever it crossed, and
// the only account of that is the fixture's record of each point.
func TestEveryPointOfTheGestureIsHeldInsideTheWindow(t *testing.T) {
	good := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 5)
	if why := anEndLandedOutside(good, theTestWindow); why != "" {
		t.Errorf("a gesture entirely inside the window is reported as leaving it: %s",
			why)
	}
	// A MIDDLE POINT OUTSIDE IT, with both ends inside — the case a check on the ends
	// alone would call a pass.
	wandered := append([]fixtureEvent{}, good...)
	wandered[3].ScreenX = theTestWindow.X + theTestWindow.W + 40
	why := anEndLandedOutside(wandered, theTestWindow)
	if why == "" {
		t.Fatal("a gesture that left the window partway is reported as having " +
			"stayed inside it, so a drag across the neighbouring app reads as a " +
			"pass")
	}
	if !strings.Contains(why, "outside") {
		t.Errorf("the sentence is %q", why)
	}
}

// AND THE REFUSAL IS NOT ONE EVENT, WHICH IS STRICTER THAN NOT ONE PRESS.
//
// The guard is read before the gesture and again immediately before the button goes down, so an
// implementation that pressed and then thought better of it would leave a DOWN with no up — a
// button held on somebody's desktop by a routine that reported refusing. A reader counting only
// presses would call that a pass; a reader counting only complete gestures would too.
func TestARefusedDragIsAssertedOnNotOneEvent(t *testing.T) {
	if why := anythingAtAllArrived(nil, theTestWindow); why != "" {
		t.Errorf("an empty log is reported as having received something: %s", why)
	}
	lonely := []fixtureEvent{{Seq: 1, Kind: "mouse", What: "left_down", ScreenX: 200,
		ScreenY: 200, Window: theTestWindow}}
	if why := anythingAtAllArrived(lonely, theTestWindow); why == "" {
		t.Error("a mouse-down with no release is reported as a refusal, so a routine " +
			"that pressed and regretted it would read as having sent nothing — " +
			"with the button still down")
	}
}

// AND THE STOP IS ORDERED AGAINST THE RELEASE BY THE LOG, NOT BY A VERDICT.
//
// The one way the stopped-drag scenario can be accidentally green is a gesture that had already
// finished: the run is stopped between steps, records `cancelled`, and every assertion about
// where the button came up is true of a drag nothing interrupted. The first cut read that off the
// outcome word and the existence of movement, and neither orders anything — an outcome says what
// Marco concluded, not when the application was told. So the stop is bracketed in the fixture's
// own counter, and the three readings of that bracket are the three answers this can give.
//
// `mid` is a press at #1, movement at #2–#4 and a release at #5 throughout, so the bracket is the
// only thing that changes between the cases.
func TestTheStopIsOrderedAgainstTheReleaseByTheFixturesLog(t *testing.T) {
	mid := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	if got := mid[len(mid)-1].Seq; got != 5 {
		t.Fatalf("the log this test reasons about ends at #%d, not #5", got)
	}
	// THE RELEASE AFTER THE REQUEST: a cancellation was exercised.
	if why := theDragHadAlreadyFinished(mid, stopBracket{before: 4, after: 4}); why != "" {
		t.Errorf("a release logged after the stop was asked for is reported as "+
			"unexercised: %s", why)
	}
	// AND NO RELEASE AT ALL IS NOT AN UNEXERCISED SCENARIO.
	//
	// It is a button held down on somebody's desktop, which `theButtonWasNotReleased`
	// reports as the finding it is. A NOT RUN here would file the worst outcome this
	// scenario can produce as "nothing happened" — the same mistake as excusing an
	// ignored stop, one claim over.
	if why := theDragHadAlreadyFinished(mid[:4], stopBracket{before: 4, after: 4}); why != "" {
		t.Errorf("a gesture whose button never came up is reported as unexercised "+
			"(%s), so a held button would be filed as NOT RUN", why)
	}
	if why := theButtonWasNotReleased(mid[:4]); why == "" {
		t.Error("and nothing else reports it either")
	}
	for _, c := range []struct {
		what string
		got  []fixtureEvent
		br   stopBracket
		says string
	}{
		{"the release was already logged", mid, stopBracket{before: 5, after: 5},
			"no cancellation was exercised"},
		// THE AMBIGUITY, which is neither a pass nor a failure. The release
		// arrived while the request was being made, and nothing orders them.
		{"the release landed inside the bracket", mid,
			stopBracket{before: 4, after: 6}, "nothing here orders the two"},
		{"the press was not logged before the stop", mid,
			stopBracket{before: 0, after: 0}, "not demonstrably down"},
		{"nothing arrived at all", nil, stopBracket{before: 0, after: 0},
			"no gesture in flight"},
		{"the log could not be read", mid, stopBracket{before: 4, after: 0},
			"could not be read"},
	} {
		t.Run(c.what, func(t *testing.T) {
			why := theDragHadAlreadyFinished(c.got, c.br)
			if why == "" {
				t.Fatalf("%s is reported as an exercised cancellation", c.what)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the sentence is %q and does not say %q", why, c.says)
			}
		})
	}
}

// AND THE ORDER THE CLAIMS ARE ASKED IN IS ITSELF A CLAIM.
//
// Every piece above has its own test and the composition had none, and the composition is where
// the defect was: a log with a press and nothing else has no movement in it, so the scope
// exemption answered first and filed it as NOT RUN, and the missing release was never asked
// about. A left button held down on somebody's desktop is the worst thing this scenario can find
// and it was the one thing it could not report — a defect behind an exemption, which is the third
// time this slice has found that shape.
//
// So this asks the composed decision with both controls: the exemptions must not swallow the held
// button, and they must still exempt what they are for.
func TestTheHeldButtonIsReportedBeforeAnyExemption(t *testing.T) {
	seed := seededRoutine{at: [][2]int{{200, 200}, {600, 400}}}
	mid := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	// letGo is the clean arrangement: a stopped drag that released partway along.
	letGo := append(append([]fixtureEvent{}, mid[:len(mid)-1]...),
		fixtureEvent{Seq: 9, Kind: "mouse", What: "left_up", ScreenX: 400,
			ScreenY: 300, Window: theTestWindow})
	cancelled := runRecord{Outcome: "cancelled", Because: "Stop was pressed"}
	for _, c := range []struct {
		what   string
		got    []fixtureEvent
		br     stopBracket
		rec    runRecord
		says   string
		notRun bool
	}{
		// THE CASE THE ORDER WAS WRONG FOR.
		{"a press and nothing else", mid[:1], stopBracket{before: 1, after: 1},
			cancelled, "still held", false},
		// AND THE SAME THING ONE EXEMPTION OVER: an ordering this cannot
		// settle is no reason to leave a held button unreported either.
		{"a press the bracket cannot order", mid[:1], stopBracket{before: 0, after: 0},
			cancelled, "still held", false},
		// AND THE EXEMPTIONS STILL EXEMPT WHAT THEY ARE FOR.
		{"a press and a release with no travel",
			aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 0),
			stopBracket{before: 1, after: 1}, cancelled, "no movement at all", true},
		{"a drag that had already finished", letGo,
			stopBracket{before: 9, after: 9}, cancelled,
			"no cancellation was exercised", true},
		{"nothing at all", nil, stopBracket{before: 0, after: 0}, cancelled,
			"no gesture in flight", true},
		// AND THE VERDICTS ARE STILL REACHED.
		{"a release at the destination", mid, stopBracket{before: 4, after: 4},
			cancelled, "destination", false},
		{"a stop that was ignored", letGo, stopBracket{before: 4, after: 4},
			runRecord{Outcome: "performed", Because: "it worked"},
			"carried on", false},
	} {
		t.Run(c.what, func(t *testing.T) {
			why, notRun := theStoppedDragsVerdict(c.got, c.br, c.rec, seed)
			if why == "" {
				t.Fatalf("%s is reported as a stopped drag that let go where it "+
					"got to", c.what)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the sentence is %q and does not say %q", why, c.says)
			}
			if notRun != c.notRun {
				was, want := "a failure", "NOT RUN"
				if !c.notRun {
					was, want = want, was
				}
				t.Errorf("%s is reported as %s and it is %s: %s", c.what, was,
					want, why)
			}
		})
	}
	// AND THE CLEAN ARRANGEMENT HAS TO PASS, or every case above is satisfied by a
	// decision that disagrees with everything.
	if why, _ := theStoppedDragsVerdict(letGo, stopBracket{before: 4, after: 4},
		cancelled, seed); why != "" {
		t.Errorf("a stop that let go partway along its travel is reported as "+
			"wrong: %s", why)
	}
	// AND A LOG WITH NO PRESS IN IT IS NOT A HELD BUTTON, which is what lets the held
	// button be asked first: nothing went down, so nothing is down.
	if why := theButtonWasNotReleased(nil); why != "" {
		t.Errorf("an empty log is reported as a button left down: %s", why)
	}
}

// AND THE SCENARIO HAS TO TAKE THAT EVIDENCE, IN THAT ORDER.
//
// The claims above can be asked of a log by a test; whether the scenario ASKS them, and whether
// it has the evidence when it does, is a property of a function body only a desktop runs. So it
// is read the way the hold scenario's absence claim is read — off the source, as an order of
// calls — because a bracket taken at the wrong moment is not a bracket:
//
//   - the lower bound has to be read AFTER the press has arrived, or it bounds nothing: a
//     reading taken at the top of the scenario is a reading from before the gesture, and every
//     event of the drag would fall above it;
//   - the upper bound has to be read AFTER Stop was pressed, or there is no upper bound at all
//     and an ambiguous ordering reads as a clean one;
//   - and the ordering question has to be asked BEFORE any verdict, because every verdict this
//     scenario reaches is true of a drag nothing interrupted.
func TestTheStoppedDragScenarioBracketsItsStop(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scenario_drag.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing scenario_drag.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok &&
			d.Name.Name == "aStoppedDragLetsGoWhereItGotTo" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("aStoppedDragLetsGoWhereItGotTo is gone; it is the desktop half of " +
			"the defect that dropped a stopped drag at its destination")
	}
	// named collects the position of every call to one of these, in source order.
	named := map[string][]token.Pos{}
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch f := call.Fun.(type) {
		case *ast.Ident:
			name = f.Name
		case *ast.SelectorExpr:
			name = f.Sel.Name
		}
		switch name {
		case "lastSeq", "waitForMouse", "theStoppedDragsVerdict", "skip", "fail":
			named[name] = append(named[name], call.Pos())
		case "stopNow":
			// THE STOP, which this scenario raises rather than presses:
			// a button press costs longer than the gesture it would be
			// interrupting. See studioRun.stopNow.
			named["stop"] = append(named["stop"], call.Pos())
		}
		return true
	})
	first := func(what string) token.Pos {
		if len(named[what]) == 0 {
			return token.NoPos
		}
		return named[what][0]
	}
	stop, down := first("stop"), first("waitForMouse")
	if !stop.IsValid() || !down.IsValid() {
		t.Fatal("the scenario no longer waits for the button to go down and then " +
			"presses Stop, which is the whole arrangement")
	}
	var lower, upper bool
	for _, at := range named["lastSeq"] {
		if at > down && at < stop {
			lower = true
		}
		if at > stop {
			upper = true
		}
	}
	if !lower {
		t.Error("the fixture's log is not read between the press arriving and Stop " +
			"being pressed, so the stop's lower bound is a reading from before " +
			"the gesture and every event of the drag falls above it")
	}
	if !upper {
		t.Error("the fixture's log is not read after Stop was pressed, so there is " +
			"no upper bound on the request and a release that arrived while it " +
			"was being made reads as one that arrived after it")
	}
	verdict := first("theStoppedDragsVerdict")
	if !verdict.IsValid() {
		t.Fatal("the scenario no longer asks `theStoppedDragsVerdict`, which is where " +
			"the order of its claims lives — including asking the stop's place in " +
			"the log before any verdict, since every verdict it reaches is true " +
			"of a drag nothing interrupted")
	}
	if verdict < stop {
		t.Error("the verdict is reached before Stop has been pressed, so the bracket " +
			"it is handed has no upper bound yet")
	}
	// AND BOTH DISPOSITIONS HAVE TO BE DOWNSTREAM OF IT, because the one thing the
	// composed decision cannot do from where it sits is report itself: a scenario that
	// took the sentence and called only `fail` would turn every NOT RUN into a failure,
	// and one that called only `skip` would turn a held button into nothing happened.
	var skipped, failed bool
	for _, at := range named["skip"] {
		skipped = skipped || at > verdict
	}
	for _, at := range named["fail"] {
		failed = failed || at > verdict
	}
	if !skipped {
		t.Error("nothing after the verdict reports NOT RUN, so a run that exercised " +
			"no cancellation is reported as a result about Marco")
	}
	if !failed {
		t.Error("nothing after the verdict reports a failure, so a drag that got it " +
			"wrong is reported as unexercised")
	}
}

// AND AN IGNORED STOP IS A FINDING, NOT AN EXCUSE.
//
// The defect in the first cut, and the reason the outcome word was taken out of the ordering: it
// read `performed` as "the stop was too late" and reported NOT RUN. A run told to stop with a
// gesture demonstrably in flight that finished the gesture anyway is the most serious thing this
// scenario could find, and it was the one result that could not fail.
func TestAnIgnoredStopIsReportedRatherThanExcused(t *testing.T) {
	mid := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	if why := theDragHadAlreadyFinished(mid, stopBracket{before: 4, after: 4}); why != "" {
		t.Fatalf("the ordering is read as unexercised, so nothing downstream is "+
			"asked: %s", why)
	}
	why := theStopWasIgnored(runRecord{Outcome: "performed", Because: "it worked"})
	if why == "" {
		t.Fatal("a run that finished its gesture after being told to stop is " +
			"reported as having stopped")
	}
	if !strings.Contains(why, "carried on") {
		t.Errorf("the sentence is %q and does not say what the run did", why)
	}
	if other := theStopWasIgnored(runRecord{Outcome: "cancelled"}); other != "" {
		t.Errorf("a run that did stop is reported as having ignored it: %s", other)
	}
}

// AND A TRAVEL THAT NEVER STARTED IS NOT RUN FOR A DIFFERENT REASON.
//
// Split from the ordering above because the two were one check and are two facts. A gesture with
// no received movement is a real cancellation — the button was down — but "it let go where it got
// to" is not a claim about a pointer that never got anywhere, so it is out of scope rather than
// too late, and the sentence says which.
func TestAStoppedDragThatNeverTravelledIsOutOfScope(t *testing.T) {
	mid := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	if why := theTravelNeverStarted(mid); why != "" {
		t.Errorf("a gesture that travelled is reported as not having: %s", why)
	}
	why := theTravelNeverStarted(mid[:1])
	if why == "" {
		t.Fatal("a stop that landed before the first step of the travel is reported " +
			"as having exercised where the pointer got to")
	}
	for _, want := range []string{"cancellation is real", "never got anywhere"} {
		if !strings.Contains(why, want) {
			t.Errorf("the sentence is %q and does not say %q — a reader has to be "+
				"able to tell this from a stop that was ignored", why, want)
		}
	}
}

// AND THE RELEASE HAS TO BE WHERE IT GOT TO, WHICH IS TWO CLAIMS AND NOT ONE.
//
// Against the seeded ends and not against the last movement received, which would make the check
// a tautology about two adjacent records in the same log — a release always follows the last move,
// wherever either of them is.
//
// The destination is the defect this slice fixed. The second claim is the one the first cut left
// out: "anywhere but the destination" passes a release at a point the pointer never visited, and
// "it let go somewhere else" is not "it let go where it got to". So the release also has to be ON
// the travel.
func TestTheReleaseHasToBeOnTheTravelAndNotAtItsEnd(t *testing.T) {
	seed := seededRoutine{at: [][2]int{{200, 200}, {600, 400}}}
	dropped := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	why := theReleaseWasNotWhereItGotTo(dropped, seed)
	if why == "" {
		t.Fatal("a stopped drag that released at its destination is reported as " +
			"having let go where the pointer got to — which is the first defect " +
			"this slice fixed, read as a pass")
	}
	if !strings.Contains(why, "destination") {
		t.Errorf("the sentence is %q", why)
	}
	// AND LETTING GO PARTWAY IS WHAT A STOP LOOKS LIKE.
	letGo := func(x, y int) []fixtureEvent {
		out := append([]fixtureEvent{}, dropped[:len(dropped)-1]...)
		return append(out, fixtureEvent{Seq: 9, Kind: "mouse", What: "left_up",
			ScreenX: x, ScreenY: y, Window: theTestWindow})
	}
	if why := theReleaseWasNotWhereItGotTo(letGo(400, 300), seed); why != "" {
		t.Errorf("a release partway along the travel is reported as not being on it: "+
			"%s", why)
	}
	// AND SO IS A POINT THE GESTURE'S OWN TRUNCATION PRODUCES. It steps
	// `x1 + int((x2-x1)*t)`, so the points it sends are off the exact line by up to a
	// pixel in each axis, and a band narrower than that would fail a correct run.
	for _, p := range [][2]int{{300, 249}, {300, 251}, {499, 350}} {
		if why := theReleaseWasNotWhereItGotTo(letGo(p[0], p[1]), seed); why != "" {
			t.Errorf("a release at %d,%d — a point the gesture's own arithmetic "+
				"produces — is reported as off the travel: %s", p[0], p[1],
				why)
		}
	}
	// AND A POINT THE PATH NEVER PASSES THROUGH IS THE CASE THE FIRST CUT MISSED.
	for _, c := range []struct {
		what string
		at   [2]int
	}{
		{"a corner of the box the ends make", [2]int{200, 400}},
		{"somewhere else entirely", [2]int{20, 20}},
		{"past the destination along the same line", [2]int{700, 450}},
	} {
		t.Run(c.what, func(t *testing.T) {
			why := theReleaseWasNotWhereItGotTo(letGo(c.at[0], c.at[1]), seed)
			if why == "" {
				t.Fatalf("a release at %s is reported as where the pointer "+
					"got to, and the gesture never goes there", c.what)
			}
			if !strings.Contains(why, "not a point on the travel") {
				t.Errorf("the sentence is %q and does not say what is wrong "+
					"with the place", why)
			}
		})
	}
}

// families is every scenario prefix registered, which is the vocabulary `-only` selects on.
//
// Read off the scenarios rather than listed, so a family added later is asked about without
// anybody having to remember this test — the pump invariant's lesson, one module over: a list
// would be written against the collisions that existed the day it was written.
func families(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	var out []string
	for _, s := range scenarios() {
		name, _, found := strings.Cut(s.name, "/")
		if !found || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) < 4 {
		t.Fatalf("only %d scenario family/families found (%v), so this is checking "+
			"less than it reads as", len(out), out)
	}
	return out
}
