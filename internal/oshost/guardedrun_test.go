package oshost

// WHAT A REFUSAL DOES TO THE ROUTINE AROUND IT.
//
// A press that checks the window it was recorded in can refuse, and the question this file
// answers is what happens to the rest of the routine when it does. Pressing on would be the
// thing the check exists to prevent, one step later: the second press of a sequence means
// nothing once the first did not happen — a menu that never opened has no button to press, and
// the coordinate that button occupied belongs to whatever is there instead.
//
// So a guarded route is generated as a CHAIN: each propagating action in its own capability,
// calling the next only when its own action went through, and `this is ok!` reachable only
// through the last of them. The whole claim is about what does NOT happen, which is why every
// case here counts the input sent rather than reading the output.
//
// # Where the run's answer comes from
//
// Not the output and not an exit code. A root handler is compulsory in Marco, so the failure is
// caught and logged and the program ends tidily — which means a command that read only those
// would report success about a routine that pressed nothing. The host keeps its own refusals
// (`Refused`) and the caller turns a non-empty list into a failed run; `runRoute` in cmd/marco
// is that caller, and director's runner sees the same refusal through the capability status its
// own runner already records.
//
// Everything from the recording to the press is production: the simplifier, the generator, the
// registry, the compiler, the host. The recorder and the desktop are stood in for.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// theSecondWindow is where the second press of the two-press recording was made. Same window,
// and a different offset, so the two presses are told apart by where they land.
const secondRelX, secondRelY = 376, 219

// twoPressesInTheGame is an ordinary recording: the app comes forward, two presses in it, then the
// trip to Marco's window to press Stop.
//
// `lose` is applied to the press named by `which` (1 or 2), which is how a case says WHICH press
// cannot be checked — the difference between "nothing happened" and "the first one happened and
// then it stopped".
func twoPressesInTheGame(which int, lose func(*recorder.RecordedEvent)) []recorder.RecordedEvent {
	press := func(n, x, y, rx, ry int, at int) recorder.RecordedEvent {
		e := recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: true, X: x, Y: y, App: "arkascended", PID: 4812,
			T: when(at), WinRel: true, RelX: rx, RelY: ry, RelTo: "window",
			WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe"}
		if n == which && lose != nil {
			lose(&e)
		}
		return e
	}
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
		press(1, 842, 274, 722, 210, 100),
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: 4812, T: when(105)},
		press(2, 496, 283, secondRelX, secondRelY, 900),
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "arkascended", PID: 4812, T: when(905)},
		// ── AND NOW THEY GO AND PRESS STOP ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: when(3000)},
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", PID: 7001, T: when(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", PID: 7001, T: when(3205)},
	}
}

// firstPress and secondPress are where each press lands in the window the desktop is standing in
// for — its origin plus the offset that press was demonstrated at.
var (
	firstPress  = [2]int{sameWindowMoved.Left + 722, sameWindowMoved.Top + 210}
	secondPress = [2]int{sameWindowMoved.Left + secondRelX, sameWindowMoved.Top + secondRelY}
)

// PREPARATION THAT FAILS PRESSES NOTHING AND THE RUN IS UNSUCCESSFUL.
//
// The application never came forward, so there is no window for any of this to be about. The
// routine must not go on to press coordinates into whatever IS in front — which is what a route
// with the preparation above its body rather than around it would do, because a bare `do` whose
// status nobody reads does not stop anything.
func TestPreparationThatFailsPressesNothing(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, twoPressesInTheGame(0, nil))

	rec, h, out := playedPreparing(t, at, func(string) (uintptr, error) {
		return 0, errTheWindowIsNotThere
	})

	if len(rec.moves) != 0 || len(rec.clicks) != 0 {
		t.Errorf("input was sent although the application never came forward: "+
			"moves=%v clicks=%v\n%s", rec.moves, rec.clicks, out)
	}
	refused := h.Refused()
	if len(refused) == 0 {
		t.Fatalf("the run reports nothing wrong, so a command performing this "+
			"routine would report success about a routine that pressed "+
			"nothing\n%s", out)
	}
	if !strings.Contains(refused[0], "could not be brought to the front") {
		t.Errorf("the run's answer is %q and does not say the application never "+
			"came forward", refused[0])
	}
	// AND EXACTLY ONE, because the routine stopped AT the preparation. Three would
	// mean the preparation was a line above the body rather than around it: every
	// press attempted in turn and refused in turn, which is the routine carrying on
	// past something that did not happen.
	if len(refused) != 1 {
		t.Errorf("%d things were attempted after the application failed to come "+
			"forward: %v", len(refused), refused)
	}
}

// AND A FIRST PRESS THAT REFUSES MEANS THE SECOND IS NEVER ATTEMPTED.
//
// The case the chain exists for. Nothing is pressed at all, and specifically the second press —
// whose own window would have passed every check — is not reached, because by then the routine
// had already stopped.
func TestAFirstPressThatRefusesNeverReachesTheSecond(t *testing.T) {
	// THE FIRST PRESS CANNOT BE CHECKED: its program was not recorded, so there is
	// nothing to compare the window's owner against.
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere,
		twoPressesInTheGame(1, func(e *recorder.RecordedEvent) { e.Process = "" }))

	rec, h, out := played(t, at)

	if len(rec.moves) != 0 || len(rec.clicks) != 0 {
		t.Errorf("input was sent: moves=%v clicks=%v\n%s", rec.moves, rec.clicks,
			out)
	}
	for _, m := range rec.moves {
		if m == secondPress {
			t.Error("the second press was made after the first was refused, " +
				"which is the routine carrying on past a press that did " +
				"not happen")
		}
	}
	if len(h.Refused()) != 1 {
		t.Errorf("the run reports %d refusal(s) for one refused press: %v",
			len(h.Refused()), h.Refused())
	}
	saysItStopped(t, out)
}

// AND A SECOND PRESS THAT REFUSES KEEPS THE FIRST, AND THE RUN IS STILL UNSUCCESSFUL.
//
// Both halves matter and they pull in opposite directions. The first press HAPPENED — it was
// checked, it went through, the cursor moved and the button went down — and an account that
// hid it would be telling somebody nothing was done to their machine when something was. And
// the run is unsuccessful anyway, because the routine is the whole sequence and it did not
// finish: reporting success for "most of it" is how a half-performed routine gets run again.
func TestASecondPressThatRefusesKeepsTheFirstAndStillFails(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere,
		twoPressesInTheGame(2, func(e *recorder.RecordedEvent) { e.Dpi = 0 }))

	rec, h, out := played(t, at)

	if len(rec.moves) != 1 || rec.moves[0] != firstPress {
		t.Fatalf("the first press landed at %v, want %v — it was checked and it "+
			"went through, so it has to be in the account\n%s", rec.moves,
			firstPress, out)
	}
	if len(h.Refused()) != 1 {
		t.Fatalf("the run reports %d refusal(s): %v", len(h.Refused()),
			h.Refused())
	}
	if !strings.Contains(h.Refused()[0], "what scale it was drawn at") {
		t.Errorf("the run's answer is %q and does not name the measurement the "+
			"second press is missing", h.Refused()[0])
	}
	saysItStopped(t, out)
}

// AND WHEN EVERY PRESS GOES THROUGH, THE ROUTINE COMPLETES — AND CLAIMS NOTHING MORE.
//
// Both presses, in order, at the window's origin plus the offsets they were demonstrated at, and
// a run with nothing refused. What it does NOT say is that the routine worked: nothing here
// looked at the screen afterwards. Playback completed is a claim about input that was sent and
// checked before it was sent; whether the application did what the person wanted is a different
// question, asked by a different thing, and this is deliberately silent about it.
func TestEveryPressGoingThroughCompletesThePlayback(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, twoPressesInTheGame(0, nil))

	rec, h, out := played(t, at)

	want := [][2]int{firstPress, secondPress}
	if len(rec.moves) != 2 || rec.moves[0] != want[0] || rec.moves[1] != want[1] {
		t.Fatalf("pressed at %v, want %v\n%s", rec.moves, want, out)
	}
	if r := h.Refused(); len(r) != 0 {
		t.Errorf("a routine whose every press went through reports %v", r)
	}
	// AND IT SAYS IT FINISHED, which is the other side of `saysItStopped`: the one
	// ending that claims completion is reachable, and only this way.
	if !strings.Contains(out, "ark graphics: done") {
		t.Errorf("a routine whose every press went through does not say it "+
			"finished:\n%s", out)
	}
	// AND NOTHING IN THE ACCOUNT CLAIMS THE SCREEN WAS CHECKED AFTERWARDS.
	for _, word := range []string{"verified", "worked", "confirmed"} {
		if strings.Contains(strings.ToLower(out), word) {
			t.Errorf("the run says %q, and nothing here looked at the screen "+
				"after pressing:\n%s", word, out)
		}
	}
}

// AND A LEGACY ROUTE IS STILL ONE BLOCK THAT PRESSES ON.
//
// The other half of keeping this an opt-in, at the level of the generated file. A route whose
// presses asked to be checked against nothing was saved under a contract that says a click is
// attempted and the run carries on — and that contract is not re-read now. If the chain were
// generated for every route, every routine on the owner's machine would stop at its first
// unsuccessful click.
func TestALegacyRouteIsStillOneBlock(t *testing.T) {
	src, _ := savedFrom(t, orchestrator.ScopeAppAnywhere,
		twoPressesInTheGame(0, func(*recorder.RecordedEvent) {}))
	if !strings.Contains(src, "this can Step2.") {
		t.Fatalf("the guarded route is not a chain, so this test proves "+
			"nothing:\n%s", src)
	}

	legacy, _ := savedFrom(t, orchestrator.ScopeAppAnywhere, legacyPresses())
	if strings.Contains(legacy, "this can Step2.") {
		t.Errorf("a route that asked to be checked against nothing was "+
			"generated as a chain:\n%s", legacy)
	}
	// AND ITS PREPARATION IS STILL A BARE SENTENCE.
	//
	// `do OS's Activate with "x".` ignores its own status, which IS the legacy
	// contract: an application that did not come forward does not stop the routine.
	// Wrapping it in arms would stop every routine the owner has whose application was
	// already in front under another name — and a route with one propagating action and
	// no guarded press carries no "Step2" for the check above to notice it by, so this
	// is the assertion that catches a chain generated for everything.
	if !strings.Contains(legacy, "do OS's Activate with \"arkascended\".\n") {
		t.Errorf("the legacy route's preparation is no longer a bare sentence, "+
			"so its application failing to come forward now stops it:\n%s",
			legacy)
	}
	if !strings.Contains(legacy, "this is ok!") {
		t.Errorf("the legacy route lost its ending:\n%s", legacy)
	}
}

// legacyPresses is the same demonstration as recorded by a build that measured no windows.
func legacyPresses() []recorder.RecordedEvent {
	out := twoPressesInTheGame(0, nil)
	for i := range out {
		out[i].RelTo, out[i].WinW, out[i].WinH = "", 0, 0
		out[i].Dpi, out[i].Process = 0, ""
	}
	return out
}

// errTheWindowIsNotThere is what `Activate` is told when the application is not running.
var errTheWindowIsNotThere = errNotThere{}

type errNotThere struct{}

func (errNotThere) Error() string { return "no window named arkascended" }

var _ = winctx.WindowFacts{}

// saysItStopped holds the route's OWN answer, which is the half `Refused` does not cover.
//
// `Refused` is the host's account of what it would not do. This is the routine's: the chain's
// `this is ok!` sits inside the success of its last press, so a refused run reaches the root's
// failure arm and never the success one. A chain whose arms both ended in `this is ok!` would
// stop just the same and report that it worked — which is the unconditional ending the contract
// forbids, and nothing above this would have noticed.
func saysItStopped(t *testing.T, out string) {
	t.Helper()
	if !strings.Contains(out, "stopped there") {
		t.Errorf("the routine does not say it stopped:\n%s", out)
	}
	if strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine reports that it finished, having refused a press:\n%s",
			out)
	}
}
