package main

// PRESSING PLAY PERFORMS THE ACT, AND WHAT IT HANDS OVER IS A DECISION.
//
// The surface half of the authorisation contract. The runner's half — what a grant licenses,
// what it refuses, and that a granted run asks nobody anything — is held in
// cmd/director/actgrant_test.go; this is about what the window actually does when somebody
// presses the button:
//
//	it refuses, in front of them, anything it can answer before raising a window
//	it writes the authorisation, naming the Act and the interactions they were shown
//	and it hands the runner that file, with --live only when they asked for it
//
// Driven through the stand-in runner in TestMain, which echoes the arguments it was given — so
// what is asserted is what the child was really asked to do.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aPlayableAct is a store with one demonstration and an Act over its verifiable interaction.
//
// Interaction 1 is the one with an outcome recorded, which is the one a person would pick: the
// other two cannot be checked afterwards and the surface says so.
func aPlayableAct(t *testing.T, name string) *Acts {
	t.Helper()
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", name,
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	// THE DESKTOP'S ANSWER, STOOD IN FOR. Raising a window needs a window; what is
	// under test here is everything else. The production path is held below, by
	// TestAnApplicationThatIsNotRunningIsSaidSoBeforeAnythingStarts.
	a.raising = func(string) string { return "" }
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")
	return a
}

// grantedIn is the authorisation file the child was handed, read back.
func grantedIn(t *testing.T, said string) grant {
	t.Helper()
	fields := strings.Fields(said)
	for i, f := range fields {
		if f != "--granted" || i+1 >= len(fields) {
			continue
		}
		raw, err := os.ReadFile(fields[i+1])
		if err != nil {
			t.Fatalf("the authorisation the child was given cannot be read: %v", err)
		}
		var g grant
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatalf("the authorisation is not readable JSON: %v", err)
		}
		return g
	}
	t.Fatalf("the run was started with no authorisation at all:\n%s", said)
	return grant{}
}

// PLAY HANDS THE RUNNER AN AUTHORISATION FOR THE ACT THAT WAS CHOSEN.
//
// Every field in it is a term the runner checks before it performs anything, so every field is
// asserted here: a surface that wrote the wrong revision, or the interactions in the wrong
// order, would have its runs refused for a reason that is its own fault and reads like the Act's.
func TestPlayHandsTheRunnerAnAuthorisationForTheChosenAct(t *testing.T) {
	a := aPlayableAct(t, "the audio page")

	if err := a.Play("the audio page", false, ""); err != "" {
		t.Fatalf("Play would not start: %s", err)
	}
	said := waitForTranscript(t, a)
	for _, always := range []string{"--play the audio page", "--hotkeys", "--granted"} {
		if !strings.Contains(said, always) {
			t.Errorf("the run was started without %q:\n%s", always, said)
		}
	}
	g := grantedIn(t, said)
	if g.Version != actGrantVersion {
		t.Errorf("the authorisation is version %d", g.Version)
	}
	if g.Act != "the audio page" {
		t.Errorf("the authorisation is for %q", g.Act)
	}
	if g.Application != "ArkAscended.exe" {
		t.Errorf("the authorisation names the application %q", g.Application)
	}
	if g.Revision == "" {
		t.Error("the authorisation names no revision, so an Act composed again " +
			"between the press and the run would be performed anyway")
	}
	if len(g.Interactions) != 1 || g.Interactions[0] != "the audio tab" {
		t.Errorf("the authorisation is over %v", g.Interactions)
	}
	// AND WHAT THE NAME MEANS, which is the part of an Act's identity nothing else here
	// pins. The same name over the same demonstration at a different position is a
	// different control, and a grant that did not say which would be spendable on it.
	if len(g.Bound) != 1 || g.Bound[0].Name != "the audio tab" ||
		g.Bound[0].Position != 1 {
		t.Errorf("the authorisation does not say what its interaction means: %+v",
			g.Bound)
	}
	if g.Source == "" {
		t.Error("the authorisation names no play digest, so it does not pin which " +
			"version of the Act was authorised")
	}
	if !strings.Contains(g.By, "pressed Play") {
		t.Errorf("the authorisation does not say who gave it: %q", g.By)
	}
	if g.At.IsZero() {
		t.Error("the authorisation does not say when it was given")
	}
}

// AND --live ONLY WHEN THEY PRESSED THE BUTTON THAT SAYS SO.
//
// The most dangerous thing this surface can get wrong, asserted again for the new gesture:
// `Play` touches the machine and `Try it without clicking` does not, and both go through one
// path that takes the decision as an argument.
func TestTryingItWithoutClickingDoesNotPassLive(t *testing.T) {
	for _, live := range []bool{false, true} {
		a := aPlayableAct(t, "the audio page")
		if err := a.Play("the audio page", live, ""); err != "" {
			t.Fatalf("live=%v: Play would not start: %s", live, err)
		}
		said := waitForTranscript(t, a)
		if got := strings.Contains(said, "--live"); got != live {
			t.Errorf("live=%v: --live present=%v\n%s", live, got, said)
		}
		// AND READING IS ALWAYS ON. An Act's interactions are verified against what
		// the demonstration saw, and a run that cannot read the screen cannot
		// verify one — which is the difference between an Act and a macro.
		if !strings.Contains(said, "--reading") {
			t.Errorf("live=%v: the run cannot read the screen, so nothing it "+
				"performs can be verified:\n%s", live, said)
		}
		_ = a.Cancel()
	}
}

// AND WHAT PLAY CAN ANSWER NOW, IT ANSWERS NOW.
//
// Before a window is raised and before a child exists. Each of these is a sentence a person can
// act on, and each used to be either a refusal halfway through a run or nothing at all.
func TestPlayRefusesWhatItCanAnswerBeforeStartingAnything(t *testing.T) {
	a := aPlayableAct(t, "the audio page")

	// ── AN ACT THAT IS NOT THERE.
	if err := a.Play("something nobody composed", false, ""); !strings.Contains(err,
		"no Act called") {
		t.Errorf("playing an Act that does not exist said %q", err)
	}
	// ── AND NOTHING WAS STARTED BY EITHER OF THEM.
	if w := a.Watch(); w.Active {
		t.Error("a refused Play started a run anyway")
	}
}

// AND AN ACT WHOSE DEMONSTRATION HAS MOVED UNDER IT IS REFUSED IN ITS OWN WORDS.
//
// The preflight has already asked whether the play still matches its bindings and whether the
// demonstration is still the revision it was pinned to. A person needs that sentence: "this Act
// cannot be performed as it stands" plus the reason is actionable, and a run that started and
// then refused is not.
func TestAnActWhoseDemonstrationMovedIsRefusedWithTheReason(t *testing.T) {
	a, art := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "moved",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	a.raising = func(string) string { return "" }
	// THE DEMONSTRATION, SAVED AGAIN WITH A CHANGE. Its revision moves, and the Act is
	// pinned to the one it was composed from.
	art.Steps[1].Outcome = append(art.Steps[1].Outcome, "balance")
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}

	why := a.Play("moved", false, "")
	if why == "" {
		t.Fatal("an Act pinned to a revision that no longer exists was performed")
	}
	if !strings.Contains(why, "cannot be performed as it stands") {
		t.Errorf("the refusal does not say what is wrong: %q", why)
	}
	if w := a.Watch(); w.Active {
		t.Error("a refused Play started a run anyway")
	}
}

// AND AN ACT COMPOSED AGAIN SINCE IT WAS LISTED IS NOT PERFORMED UNDER THE OLD DECISION.
//
// # The window this closes
//
// Between a person reading an Act in the list and pressing Play on it, the Act can be composed
// again — by them in the panel above, or by `director compose-act` in a terminal. The dangerous
// version of that is the one where nothing they can see has changed: the same names, in the same
// order, over the same demonstration, bound to different interactions. The list said "the audio
// tab"; the Act now presses something else under that name.
//
// So the page hands back the digest it was displaying, and Studio refuses when the Act on disk
// is no longer that one. The person is sent to look at it again, which is the only honest
// instruction: what they read is out of date.
func TestAnActComposedAgainSinceItWasListedIsRefused(t *testing.T) {
	a := aPlayableAct(t, "the audio page")

	why := a.Play("the audio page", false, "a digest from a version that has gone")
	if why == "" {
		t.Fatal("an Act was performed under a decision made about a different " +
			"version of it")
	}
	if !strings.Contains(why, "composed again since it was listed") {
		t.Errorf("the refusal does not say what happened: %q", why)
	}
	if w := a.Watch(); w.Active {
		t.Error("a refused Play started a run anyway")
	}
	// AND THE IDENTITY IT IS SHOWING WORKS, so what is being tested is staleness and
	// not the presence of the argument.
	shown := actCalled(t, a, "the audio page").Identity
	if shown == "" {
		t.Fatal("the Act is listed with no identity, so a page has nothing to " +
			"hand back")
	}
	if err := a.Play("the audio page", false, shown); err != "" {
		t.Fatalf("the Act was refused under the identity it is listed with: %s", err)
	}
}

// actCalled is one Act as the page lists it.
func actCalled(t *testing.T, a *Acts, name string) Act {
	t.Helper()
	for _, c := range a.Saved() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("there is no Act called %q", name)
	return Act{}
}

// aTwoTabAct composes one Act over two of the demonstration's interactions, in that order.
func aTwoTabAct(t *testing.T, picked ...string) *Acts {
	t.Helper()
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "both tabs", picked); r.Error != "" {
		t.Fatal(r.Error)
	}
	a.raising = func(string) string { return "" }
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")
	return a
}

// composedAgain replaces an Act in place, the way a person would have to.
//
// `demonstrated.Save` refuses to write over a play that already exists — "give this one a
// different name rather than replacing it" — so changing an Act means removing the pair and
// composing again. Two commands, and nothing stops anybody running them between being listed
// and pressing Play.
func composedAgain(t *testing.T, a *Acts, picked ...string) {
	t.Helper()
	rt := routes.Route{Slug: routes.Slug("both tabs")}
	if err := os.Remove(a.reg().Path(rt)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(demonstrated.BindingsPath(a.reg(), rt)); err != nil {
		t.Fatal(err)
	}
	if r := a.Compose("ark graphics live", "both tabs", picked); r.Error != "" {
		t.Fatal(r.Error)
	}
}

// THE RECOMPOSE A DIGEST CANNOT SEE IS THE ONE THAT HAD TO BE CAUGHT.
//
// # Why the digest was not enough, although it looked like it was
//
// The check above was over the play digest. Compose an Act over one demonstration with the same
// names in the same order bound to DIFFERENT interactions and the generated Marco is
// byte-identical, with the same bindings digest — so the page handed back a term that already
// agreed, Studio compared two equal strings, and the run proceeded under a decision somebody made
// about other controls. The one change a person cannot see was the one change nothing checked.
//
// The premise is asserted here rather than argued. If a recompose did move the digest, the first
// three assertions would say so and the whole identity would be unnecessary.
func TestTheSameNamesMovedToOtherInteractionsIsRefused(t *testing.T) {
	a := aTwoTabAct(t, "0=the graphics tab", "1=the audio tab")
	was := actCalled(t, a, "both tabs")

	composedAgain(t, a, "1=the graphics tab", "0=the audio tab")
	now := actCalled(t, a, "both tabs")

	if now.Source != was.Source {
		t.Fatalf("the premise is gone: the digest moved with the positions, "+
			"%q then %q", was.Source, now.Source)
	}
	if now.Revision != was.Revision ||
		strings.Join(now.Interactions, "|") != strings.Join(was.Interactions, "|") {
		t.Fatalf("the premise is gone: something an older term checked moved too, "+
			"%v then %v", was, now)
	}
	if fmt.Sprint(now.Bound) == fmt.Sprint(was.Bound) {
		t.Fatalf("nothing was recomposed: the names still mean %v", now.Bound)
	}
	// AND THE TERM THE PAGE HANDS BACK DID MOVE.
	if now.Identity == was.Identity {
		t.Fatalf("the identity is blind to the recompose as well: %q", now.Identity)
	}

	why := a.Play("both tabs", false, was.Identity)
	if why == "" {
		t.Fatal("an Act was performed with every one of its interactions pointing " +
			"somewhere else, under a decision made about the old ones")
	}
	if !strings.Contains(why, "composed again since it was listed") {
		t.Errorf("the refusal does not say what happened: %q", why)
	}
	if w := a.Watch(); w.Active {
		t.Error("a refused Play started a run anyway")
	}
}

// AND AN ACT REPLACED AFTER PLAY WAS PRESSED IS NOT THE ONE AUTHORISED.
//
// # The second window
//
// `Play` read the Act and checked it, and then the launch read it AGAIN to write the
// authorisation. Two reads with an interval between them, and the authorisation came out of the
// second one — so an Act replaced in that interval was caught by neither check. It became the
// authorised version: the grant named the new positions, the runner preflighted the new play, the
// two agreed with each other, and Marco would press controls nobody had read about.
//
// What travels now is the snapshot `Play` checked. The interval still exists — it is a
// filesystem, and two commands can replace an Act in it — but what comes out of it is an
// authorisation for the version somebody saw, which the runner then refuses against the play on
// disk (cmd/director, TestAnActReplacedBetweenPlayAndStartupIsRefused). Refused is the point;
// authorised was the defect.
func TestAnActReplacedBetweenPlayAndTheAuthorisationIsNotAuthorised(t *testing.T) {
	a := aTwoTabAct(t, "0=the graphics tab", "1=the audio tab")
	was := actCalled(t, a, "both tabs")
	// INSIDE THE INTERVAL. `afterReserve` runs between claiming the run slot and
	// writing the authorisation, which is the gap this is about; see acts.go.
	a.afterReserve = func() {
		composedAgain(t, a, "1=the graphics tab", "0=the audio tab")
	}

	if err := a.Play("both tabs", false, was.Identity); err != "" {
		t.Fatalf("Play would not start: %s", err)
	}
	g := grantedIn(t, waitForTranscript(t, a))

	if fmt.Sprint(g.Bound) != fmt.Sprint(was.Bound) {
		t.Errorf("the authorisation is over %v, and what was chosen was %v — the "+
			"Act was read again after the decision was made, so the "+
			"replacement became the authorised version", g.Bound, was.Bound)
	}
	if g.Source != was.Source || g.Revision != was.Revision {
		t.Errorf("the authorisation names play %q revision %q, and what was chosen "+
			"was %q / %q", g.Source, g.Revision, was.Source, was.Revision)
	}
	// AND THE RUN IS AIMED BY THE SNAPSHOT TOO, not by a further read of the store.
	if g.Application != was.Application {
		t.Errorf("the authorisation names the application %q, and the Act names %q",
			g.Application, was.Application)
	}
}

// ── BRINGING THE APPLICATION FORWARD

// ── BRINGING THE APPLICATION FORWARD ────────────────────────────────────────

// aWindow is one candidate as the window system would report it.
func aWindow(app string, h uintptr, w, ht int, minimized bool) windowref.Candidate {
	return windowref.Candidate{
		ID: directorapi.WindowID("hwnd:" + strconv.Itoa(int(h))), Handle: h,
		// THE PROCESS, because that is what "it came forward" means: an
		// application brought forward need not leave the same window in front.
		ProcessID:   uint32(h) * 100,
		Application: app, Bounds: directorapi.Rect{Width: w, Height: ht},
		Minimized: minimized, Visible: !minimized, OnScreen: !minimized,
	}
}

// A MINIMIZED APPLICATION IS RESTORED, NOT REFUSED.
//
// # The defect
//
// The first version skipped minimized windows while choosing, and then told the person the
// application "does not seem to be running, or its window is minimised — bring it up and press
// Play again". Two things wrong with that: it is the surface asking for something it can do
// itself, and `raiseWindow` already restores before it activates — so that branch was
// unreachable for exactly the windows it was written for.
//
// The restore itself is a Windows syscall and no Go test can observe it. What is held here is
// that the DECISION reaches it, with the handle of the window that is down.
func TestAMinimizedApplicationIsRestoredRatherThanRefused(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	lifted := []uintptr{}
	pids := []uint32{}
	a.lifting = func(h uintptr, pid uint32) (bool, bool) {
		lifted = append(lifted, h)
		pids = append(pids, pid)
		return true, true
	}
	// ONE WINDOW, MINIMIZED. Its bounds are what Windows reports for an iconic window:
	// a size it will restore to, at a position off every monitor.
	a.listing = func() []windowref.Candidate {
		return []windowref.Candidate{aWindow("arkascended", 77, 1920, 1080, true)}
	}

	if why := a.bringForward("arkascended"); why != "" {
		t.Fatalf("a running application was refused for being minimised: %s", why)
	}
	if len(lifted) != 1 || lifted[0] != 77 {
		t.Fatalf("the minimised window was not the one raised: %v", lifted)
	}
	// AND THE PROCESS WENT WITH IT, which is what the foreground is then checked
	// against: the window raised and the window in front afterwards are not always
	// the same one.
	if len(pids) != 1 || pids[0] != 7700 {
		t.Fatalf("the raise was asked about process %v, and the window belongs "+
			"to 7700", pids)
	}
}

// AND AN APPLICATION WINDOWS WILL NOT BRING FORWARD IS SAID, NOT STARTED.
//
// # The run this is for
//
// Studio raised the window and ignored the answer: `_ = a.lift(...)`, on the stated grounds
// that the runner's own guards decide. They do decide, and this is what deciding looked like to
// the owner — Play, nothing visible, thirty seconds, "refused — 0 of 2 interactions verified".
// The runner had paused correctly on every look, saying it could not see the screen the Act
// starts on, while the game sat behind the window they were watching. Their words: "play just
// starts a new director, it doesn't try and do anything because it doesn't foreground ark".
//
// So a raise that did not take is answered here, where a sentence can say what to do about it,
// and no run is started — it could only wait for its whole deadline. A platform that cannot
// tell where the foreground is says so instead (`checked` false), and the run proceeds as
// before, because "nobody could say" is not "it is not there".
func TestAnApplicationWindowsWillNotBringForwardIsSaidBeforeAnythingStarts(t *testing.T) {
	for _, c := range []struct {
		name            string
		leading, known  bool
		refusedContains string
		starts          bool
	}{
		{"it came forward", true, true, "", true},
		{"it did not, and this machine can tell", false, true,
			"click ArkAscended.exe's own window", false},
		{"nobody here can tell where the foreground is", false, false, "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := aPlayableAct(t, "the audio page")
			// THE PRODUCTION CHOOSING, with the desktop's two answers
			// stood in for: which windows there are, and what the raise
			// achieved.
			a.raising = nil
			a.listing = func() []windowref.Candidate {
				return []windowref.Candidate{
					aWindow("ArkAscended.exe", 5, 1920, 1080,
						false)}
			}
			a.lifting = func(uintptr, uint32) (bool, bool) {
				return c.leading, c.known
			}

			why := a.Play("the audio page", false, "")
			if c.refusedContains == "" {
				if why != "" {
					t.Fatalf("Play was refused: %s", why)
				}
			} else if !strings.Contains(why, c.refusedContains) {
				t.Fatalf("the refusal does not say what to do: %q", why)
			}
			if !c.starts {
				if w := a.Watch(); w.Active {
					t.Error("a run was started for a window that is " +
						"not in front, which can only wait")
				}
				return
			}
			if said := waitForTranscript(t, a); !strings.Contains(said, "--play") {
				t.Errorf("no run was started:\n%s", said)
			}
		})
	}
}

// AND THE ONE THAT IS UP WINS OVER THE ONE THAT IS DOWN.
//
// An executable owns several windows — a launcher, a splash, the main one — and a minimized
// window's reported size is not evidence of anything. So being on screen decides first, and size
// decides between windows in the same state.
func TestTheWindowRaisedIsTheOneSomebodyIsLookingAt(t *testing.T) {
	for _, c := range []struct {
		name  string
		have  []windowref.Candidate
		want  uintptr
		found bool
	}{
		{"the only one", []windowref.Candidate{
			aWindow("arkascended", 1, 800, 600, false)}, 1, true},
		{"the bigger of two that are up", []windowref.Candidate{
			aWindow("arkascended", 1, 400, 300, false),
			aWindow("arkascended", 2, 1920, 1080, false)}, 2, true},
		{"the one that is up, even when it is smaller", []windowref.Candidate{
			aWindow("arkascended", 1, 1920, 1080, true),
			aWindow("arkascended", 2, 400, 300, false)}, 2, true},
		{"a minimized one when that is all there is", []windowref.Candidate{
			aWindow("arkascended", 1, 1920, 1080, true)}, 1, true},
		{"nothing of another application", []windowref.Candidate{
			aWindow("notepad", 1, 800, 600, false)}, 0, false},
		{"nothing with no handle", []windowref.Candidate{
			aWindow("arkascended", 0, 800, 600, false)}, 0, false},
	} {
		got, found := theWindowToRaise(c.have, "ArkAscended")
		if found != c.found {
			t.Errorf("%s: found=%v, want %v", c.name, found, c.found)
			continue
		}
		if found && got.Handle != c.want {
			t.Errorf("%s: raised window %d, want %d", c.name, got.Handle, c.want)
		}
	}
}

// AND AN APPLICATION THAT IS NOT RUNNING IS SAID SO, BY THE PRODUCTION PATH.
//
// No stand-in: this is `bringForward` itself, against a real desktop that is certainly not
// running this. The sentence matters as much as the refusal — "start it, bring it up, and press
// Play again" is the whole of what a person has to do, and the alternative is a run that pauses
// for its whole budget about a window that was never open.
func TestAnApplicationThatIsNotRunningIsSaidSoBeforeAnythingStarts(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)

	why := a.bringForward("a-program-nobody-has-dot-exe")
	if why == "" {
		t.Fatal("an application that is not running was treated as ready")
	}
	for _, says := range []string{"does not seem to be running", "press Play again"} {
		if !strings.Contains(why, says) {
			t.Errorf("the sentence does not say %q: %s", says, why)
		}
	}
	// AND AN ACT PINNED TO NOTHING IS NOT REFUSED FOR IT. There is no window to
	// raise and nothing to check against; the run grounds against what leads the
	// desktop, as it always did.
	if why := a.bringForward(""); why != "" {
		t.Errorf("an Act with no application recorded was refused: %s", why)
	}
}

// ── AND HOW IT WENT, IN WORDS ────────────────────────────────────────────────

// EVERY OUTCOME IS SAID IN WORDS, AND THE TWO THAT MATTER MOST SAY WHETHER ANYTHING WAS PRESSED.
//
// All eight cases, because the point of this function is that no outcome falls through it. The
// pressed/not-pressed half is the one a person needs first: something on their machine either
// moved or it did not, and `refused` and `contradicted` are the two words that hide it.
func TestEveryOutcomeIsSaidInWords(t *testing.T) {
	for _, c := range []struct {
		outcome runref.Outcome
		says    string
	}{
		{runref.Verified, "It worked"},
		{runref.Contradicted, "did NOT change"},
		{runref.Unconfirmed, "could not tell whether it worked"},
		{runref.Refused, "nothing was pressed"},
		{runref.Cancelled, "You stopped it"},
		{runref.Failed, "could not finish"},
		{runref.Started, "never said how it ended"},
		{runref.Outcome("something new"), "something new"},
	} {
		got := saidOf("the audio tab", c.outcome, "")
		if !strings.Contains(got, c.says) {
			t.Errorf("%q is said as %q, which does not say %q", c.outcome, got,
				c.says)
		}
		if !strings.Contains(got, "the audio tab") &&
			c.outcome != runref.Cancelled {
			t.Errorf("%q is said without naming the Act: %q", c.outcome, got)
		}
	}
	// ── AND A RUN WITH NO OUTCOME YET SAYS NOTHING, rather than inventing a word
	//    for a run that is still going.
	if got := saidOf("the audio tab", "", ""); got != "" {
		t.Errorf("a run with no outcome is described as %q", got)
	}
	// ── AND THE RUN'S OWN REASON IS CARRIED, not paraphrased.
	got := saidOf("the audio tab", runref.Refused,
		"the look taken at the instant of the press did not agree")
	if !strings.Contains(got, "did not agree") {
		t.Errorf("the run's own reason was dropped: %q", got)
	}
}

// AND THE RUN'S OWN STATE CARRIES THE SENTENCE, not just the word.
//
// Through `Watch`, where the page gets it. The outcome word is kept beside it: this is a sentence
// ADDED for a person, not a replacement for the vocabulary the record uses.
func TestWhatTheWindowShowsSaysWhatHappened(t *testing.T) {
	ended := aStream(t, runstate.Event{Phase: runstate.Ended, Final: true,
		Act: "the audio page", Outcome: runref.Unconfirmed,
		Reason: "no words were read on the panel, so there was no evidence either way"})

	w := ended.Watch()
	if !strings.Contains(w.Said, "could not tell whether it worked") {
		t.Errorf("the window shows %q, which does not say what happened", w.Said)
	}
	if !strings.Contains(w.Said, "an act") {
		t.Errorf("the sentence does not name the Act: %q", w.Said)
	}
	// AND THE RUN'S OWN REASON IS IN IT, from the thing that decided.
	if !strings.Contains(w.Said, "no evidence either way") {
		t.Errorf("the sentence drops the run's own reason: %q", w.Said)
	}
	// AND THE WORD IS STILL THERE.
	if w.Outcome != "unconfirmed" {
		t.Errorf("the outcome word is %q", w.Outcome)
	}
}

// ── AND THE PAGE OFFERS WHAT THIS BUILT ─────────────────────────────────────

// THE PAGE'S OWN HALF OF THE FLOW.
//
// Source guards, because the rendering is JavaScript and nothing here can run it. Each one is a
// thing that would pass every test above while leaving the person with the surface they were
// handed before: a Run button that asks by key, an outcome word with no sentence, a paused run
// that looks like a failure, and `control_2` as the name of a thing on their screen.
func TestThePageIsTheFlowThatWasAskedFor(t *testing.T) {
	raw, err := os.ReadFile("frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	for _, want := range []struct{ text, why string }{
		{"acts.Picture(routine, w.label)", "the page never asks for a picture of " +
			"what was shown, so review is words alone \u2014 which the project " +
			"says is not an adequate substitute for seeing the target and " +
			"the result"},
		{"paintStepShots(", "an Act\u2019s ordered preview shows no pictures, so the " +
			"only thing identifying its starting screen is a vocabulary read " +
			"off it"},
		{"Pictures are kept", "the card that asks to record says nothing about " +
			"pictures of the person\u2019s screen being kept, so Studio asks the " +
			"recorder for them without telling them"},
		{"keepRecording(", "finishing a recording early offers no way to keep " +
			"recording, so the person has to estimate how long the evidence " +
			"takes to arrive"},
		{"acts.Done(true)", "there is no way to finish a recording anyway, so a " +
			"window that asked for more evidence could refuse forever"},
		{"learnGot", "the recording does not show what the recorder has, so " +
			"whether the evidence arrived is not on screen"},
		{"l.start_known", "the recording never says that the screen it starts " +
			"from has not been read well enough to be recognised — which " +
			"is fixable only while it is still recording, and which a " +
			"captured result does not fix"},
		{"id=\"learnStart\"", "there is nowhere on the recording card for the " +
			"one fault a person can repair while recording, so it is only " +
			"ever mentioned when they try to finish"},
		{"l.missing", "the recording card never paints what the recorder says is " +
			"missing, so the place made for the reading count stays empty"},
		{"id=\"learnWhy\"", "the recording card has nowhere to show how many of " +
			"the readings persistence needs have arrived, so a person waiting " +
			"for a result is estimating OCR — which is the one thing the " +
			"recorder’s own stream exists to stop"},
		// ── WHAT THE POLLS MUST NOT REDRAW ──
		//
		// Both polls run two and a half times a second for as long as the window
		// is open, and both used to rebuild their whole subtree every time. That
		// is invisible for text and not for a PICTURE: measured on the finished
		// review, 66 picture fetches in 4.2 seconds against 6 now; on the run
		// screen, 108 against 9. The owner saw it as "the images blink in the ui".
		{"shownReview", "the finished review is redrawn on every poll, so its " +
			"pictures are cleared and fetched again two and a half times a " +
			"second and the page blinks and jumps"},
		{"box.dataset.showing", "a picture box is emptied and refilled even when " +
			"it is already showing exactly those pictures"},
		{"shots.set(", "pictures are not kept once fetched, so every redraw is a " +
			"round trip to the backend for bytes the page already had"},
		{"shownDemos", "the Scene library is rebuilt on every poll for as long as " +
			"a finished review is open"},
		{"shownRunSteps", "the run's step cards are rebuilt on every poll to move " +
			"the highlight, which throws away their pictures"},
		{"classList.toggle('now'", "the step in hand is marked by rebuilding the " +
			"cards rather than by moving the mark"},
		{"'runShots'", "the run's step list and the Act's preview give their " +
			"picture boxes the SAME element ids, and both are in the document " +
			"at once — so the run repaints the preview's boxes and leaves its " +
			"own empty, which is what it did"},
		{"l.screen_read", "the recording never says that the screen in front has " +
			"not been read yet, so a press made on it has no before side, can " +
			"never be checked, and the person finds that out by waiting"},
		{"id=\"learnRead\"", "there is nowhere on the recording card for the one " +
			"fault that is cheaper to prevent than to report"},
		{"l.unchecked", "the recording does not say how many presses Marco " +
			"measured and could not tell the result of, so a press there is " +
			"nothing left to wait for reads the same as one still being " +
			"checked — which is the thirty seconds the owner spent waiting " +
			"for an answer that had already arrived"},
		{"pausedWhere", "a paused run does not say which screen it is waiting " +
			"for in the words read off it, so the only identity a person is " +
			"shown for it is a label this machine gave itself"},
		{"acts.Play(act, live, seen)", "the page does not perform an Act by " +
			"pressing Play, so the authorisation contract is not reachable " +
			"from it"},
		{"listed.find(a => a.name === act)", "the page does not hand back the " +
			"version of the Act it was showing, so an Act composed again " +
			"between being listed and being performed is performed under a " +
			"decision made about different interactions"},
		{"}).identity", "the page hands back something other than the Act's whole " +
			"identity — the play digest does not move when the same names " +
			"are recomposed onto different interactions, so that term agrees " +
			"in exactly the case it is there to catch"},
		{">Stop<", "the stop control is not called Stop"},
		{"id=\"cancelBtn\"", "Stop is not in the rail, so navigating away from the " +
			"run screen takes away the one control that is live for the whole " +
			"of a run"},
		{"Run with input disabled", "the engineering run is still offered as a " +
			"rehearsal \u2014 its first press never happens, so it cannot reach the " +
			"second interaction and must not be described as trying the Act"},
		{"Not a rehearsal", "the page does not say what a run with input disabled " +
			"cannot prove"},
		{"w.said", "the page shows the outcome word and not what it means"},
		{"'paused'", "the page does not show a paused run, so a run waiting for " +
			"the screen it expects looks like a run doing nothing"},
		{"s.reads", "the page labels an interaction by its machine name rather " +
			"than by the words that are on the person's own screen"},
		{"s.on", "the page does not say which screen an interaction starts on in " +
			"words that are on it, so `screen_a` is what somebody is asked " +
			"to recognise"},
		{"a.waits", "the page lists an Act that can only wait for a screen it " +
			"cannot recognise as plain `ready`"},
		{"Use this one", "what can be performed is not a button of its own, so a " +
			"step that does nothing when pressed looks like one that does"},
		{"no press here", "a step that presses nothing is still shaped like a " +
			"control"},
		{"Teach Marco your first interaction", "an empty library says `0 " +
			"composed` over an empty list and nothing about what to do, which " +
			"is where the owner's first trial stopped"},
		{"A Scene is one interaction", "the page never establishes the two words it " +
			"is built out of, so somebody has to infer what an Act is from a " +
			"list of them"},
		{"nothingChosen", "Play is live with no Act chosen, so pressing it is " +
			"answered by being told off for pressing it"},
		{"s.outcome_words", "the page does not say whether an interaction can be " +
			"checked afterwards, which is the most surprising thing about one"},
		// ── THE ONE-TASK-AT-A-TIME SHAPE ──
		{"rail-history", "there is no rail, so every task is on screen at once and " +
			"the person assembles the workflow in their head"},
		{"w.doing", "the run screen shows the runner's phase instead of what is " +
			"happening to the thing they asked for"},
		{"w.next", "a run that ended says what happened and not what to do about it"},
		{"acts.History()", "there is no run history, so a run is unrecoverable the " +
			"moment the window moves on from it"},
		{"acts.Report(", "there is no way to report a problem with the run " +
			"reference already attached"},
		// ── DELETING A RUN, IN TWO PRESSES ──
		{"ask(subject, false)", "the page cannot ask what a deletion would " +
			"remove, so either there is no way to delete or the first press " +
			"is the one that does it"},
		{"ask(subject, true)", "nothing ever confirms a deletion, so the " +
			"question could be raised and never answered"},
		{"id=\"histAsk\"", "there is nowhere to show what a deletion would " +
			"remove before it removes it"},
		{">Delete all<", "there is no way to clear the run history, so a machine " +
			"with four hundred records has to be cleaned up by hand"},
		{"forget(chosenRun)", "a single run cannot be deleted; only all of them"},
		{"l.mismatched", "the window never says the recorder is a different " +
			"build, so which binary is running is worked out from file " +
			"timestamps — which is what cost the owner a day"},
		{"id=\"build\"", "the window does not say which build it is, so the " +
			"recorder's revision has nothing to be read against"},
		{"paintSceneShots(", "a demonstration's card shows no pictures, so what " +
			"somebody showed Marco is visible only in the review that goes " +
			"away with the window and in an Act's preview once one exists " +
			"— the pictures are on disk and nothing shows them"},
		{"acts.ForgetScene", "a demonstration cannot be removed from the window, " +
			"so being rid of one means finding files by hand"},
		{"acts.ForgetAct", "an Act cannot be removed from the window"},
		{"acts.ForgetEverything(anyway)", "there is no way to put the machine back " +
			"to before any of it — which is what the owner asked for, and is not " +
			"the same as deleting one card at a time"},
		{"id=\"freshBox\"", "starting fresh is not behind anything deliberate, or " +
			"is not there at all"},
		{"askThenDelete(", "each kind of deletion asks in its own way, so the " +
			"promise that the first press removes nothing is made three times " +
			"and can be broken once"},
		{"starts_on", "the Act's detail does not say which screen it has to start " +
			"on, which is the thing a person has to do before pressing Play"},
		{"stepsHTML(", "there is no ordered preview of what an Act will do, so Play " +
			"is pressed on a name"},
		{"w.active ? at : -1", "the run screen does not mark which step is in hand, " +
			"so the preview and the run do not line up"},
		// ── LEARNING, WHICH IS WHAT MARCO IS FOR ──
		{"acts.Learn(", "the window cannot record a demonstration, so the one thing " +
			"Marco is for is the one thing it cannot do"},
		{"acts.Done(false)", "there is no way to tell the recorder you are finished, so a " +
			"recording can only end by running out of seconds"},
		{"acts.StopLearning()", "a recording cannot be abandoned once it has started"},
		{"acts.Applications()", "the application to watch is typed rather than " +
			"chosen from what is running"},
		{"l.learned", "what was recorded is not shown back, so somebody has to go " +
			"and look for it to find out whether it worked"},
		{">That's it<", "the recorder is ended by a clock rather than by the " +
			"person saying they are done"},
		{"buildFromLearned(", "what was just recorded cannot be turned into an Act " +
			"from the review, so the loop breaks between learning something " +
			"and using it and somebody has to go and find it in a list"},
		// THE RECORDING’S SENTENCE COMES FROM THE RECORDER, which is what
		// replaced the fixed paragraph of advice this used to guard. "Rest on the
		// screen you start from" is now said at the moment it applies, by the
		// thing that knows whether anything has been read yet — see
		// learnstate.Doing and its own test.
		{"text($('learnDoing'), l.doing", "the recording does not show the " +
			"recorder\u2019s own account of itself, so what is on screen while " +
			"somebody demonstrates is this window\u2019s guess at it"},
	} {
		if !strings.Contains(page, want.text) {
			t.Errorf("%s (looked for %q)", want.why, want.text)
		}
	}
}
