package main

// RECORDING A DEMONSTRATION FROM THE WINDOW.
//
// The thing Marco is for was the one thing the window could not do. These hold the lifecycle it
// now drives — prepare, record, finish, review — and in particular the two halves that are easy
// to get wrong:
//
//	it ends the recorder the RECORDER'S way, with a newline on its stdin, so the artifact and
//	the evidence beside it are written; a kill would throw both away and call it finishing
//
//	it says what came of it from the SAVED ARTIFACT, with one remedy per way a capture can be
//	useless, because "try again" over four different causes is the window guessing

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aRecorderStandingIn points Studio at this test binary and lets it be the recorder.
func aRecorderStandingIn(t *testing.T, a *Acts) {
	t.Helper()
	a.raising = func(string) string { return "" }
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RECORDER", "1")
}

// learningSaid waits for the recorder to be over and returns what came of it.
func learningSaid(t *testing.T, a *Acts) Learning {
	t.Helper()
	for i := 0; i < 300; i++ {
		if l := a.Learning(); l.Over {
			return l
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the recording never finished: %+v", a.Learning())
	return Learning{}
}

// THE RECORDER IS STARTED WITH WHAT IT NEEDS, AND ENDED THE WAY IT ENDS.
func TestRecordingAsksMarcosOwnRecorderAndFinishesItProperly(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	aRecorderStandingIn(t, a)

	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// PREPARING FIRST, and it says what to do with the time.
	l := a.Learning()
	if !l.Active || l.Phase != "preparing" {
		t.Fatalf("a recording that has just started is %+v", l)
	}
	if !strings.Contains(l.Doing, "Switch to ArkAscended.exe") {
		t.Errorf("the preparation does not say what to do: %q", l.Doing)
	}
	if l.Left <= 0 {
		t.Error("the preparation counts down to nothing")
	}

	// AND THE PERSON SAYS WHEN THEY ARE DONE.
	if why := a.Done(false); why != "" {
		t.Fatalf("the recorder could not be told to finish: %s", why)
	}
	got := learningSaid(t, a)
	said := strings.Join(got.Transcript, "\n")
	for _, want := range []string{
		"demonstrate-visual the audio tab opening",
		"--application ArkAscended.exe",
		"--seconds 60",
		// THE PREPARATION IS THE RECORDER'S OWN DELAY, passed explicitly: this
		// window counts down to it, and counting down to a number nobody passed
		// would be counting down to a guess.
		"--delay 5s",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the recorder was not asked for %q:\n%s", want, said)
		}
	}
	// AND IT WAS FINISHED, NOT KILLED. The recorder reports, learns a draft from the
	// walk, saves the artifact and writes the evidence beside it — all of it after the
	// newline. A window that killed the process here would throw that away.
	if !strings.Contains(said, "FINISHED on") {
		t.Errorf("the recorder was not told to finish on its own terms:\n%s", said)
	}
}

// AND NOTHING ELSE RUNS WHILE SOMETHING IS BEING RECORDED.
//
// One screen. A recorder watching it while a runner presses on it would be Marco demonstrating
// to itself, and the artifact would record Marco's own clicks as the person's.
func TestARunAndARecordingDoNotOverlap(t *testing.T) {
	a := aPlayableAct(t, "the audio page")
	aRecorderStandingIn(t, a)
	if why := a.Learn("something else", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// AND THE WINDOW CLOSING ENDS IT, which is also how this test leaves nothing
	// holding the temporary store open. See Close.
	t.Cleanup(a.Close)

	if why := a.Play("the audio page", false, false, ""); why == "" {
		t.Fatal("an Act was performed while a demonstration was being recorded")
	} else if !strings.Contains(why, "recording") {
		t.Errorf("the refusal does not say why: %s", why)
	}
}

// AND A RECORDING IS REFUSED FOR WHAT CAN BE ANSWERED BEFORE ANYTHING IS WATCHED.
func TestRecordingRefusesWhatItCanAnswerFirst(t *testing.T) {
	a := aPlayableAct(t, "the audio page")
	aRecorderStandingIn(t, a)
	for _, c := range []struct {
		name, act, app string
		seconds        int
		says           string
	}{
		{"no name", "", "ArkAscended.exe", 60, "say what to call it"},
		{"no application", "a thing", "", 60, "choose the application"},
		{"a moment", "a thing", "ArkAscended.exe", 1, "between 5 and 300"},
		{"forever", "a thing", "ArkAscended.exe", 4000, "between 5 and 300"},
		// AND NOT UNDER AN ACT'S OWN NAME. Both become declarations in the play
		// that composes them, so this is a compile error two steps later, said
		// here instead.
		{"an Act's name", "the audio page", "ArkAscended.exe", 60,
			"already an Act called"},
	} {
		if why := a.Learn(c.act, c.app, c.seconds); why == "" {
			t.Errorf("%s: it was recorded anyway", c.name)
		} else if !strings.Contains(why, c.says) {
			t.Errorf("%s: the refusal says %q", c.name, why)
		}
		if l := a.Learning(); l.Active {
			t.Fatalf("%s: a refused recording started anyway", c.name)
		}
	}
}

// AND THE APPLICATION IS CHOSEN FROM WHAT IS RUNNING, not typed.
func TestTheApplicationsOfferedAreTheOnesWithAWindow(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	a.listing = func() []windowref.Candidate {
		return []windowref.Candidate{
			aWindow("ArkAscended.exe", 1, 1920, 1080, false),
			// THE SAME APPLICATION TWICE is one entry: a launcher, a splash
			// and the main window are one thing to point at.
			aWindow("ArkAscended.exe", 2, 800, 600, false),
			aWindow("notepad.exe", 3, 600, 400, false),
			// AND NEITHER A MINIMIZED ONE NOR A MESSAGE-ONLY WINDOW. Every
			// process on a desktop owns one of the latter, and a list of forty
			// invisible windows is a list nobody can choose from.
			aWindow("steam.exe", 4, 1200, 800, true),
			aWindow("svchost.exe", 5, 1, 1, false),
		}
	}
	got := strings.Join(a.Applications(), " ")
	if got != "ArkAscended.exe notepad.exe" {
		t.Errorf("the applications offered are %q", got)
	}
}

// ── WHAT CAME OF IT, WITH ONE REMEDY PER CAUSE ──────────────────────────────

// aDemonstrationLike is one as the surface lists it, with the three facts the review reads.
func aDemonstrationLike(steps ...Interaction) *Demonstration {
	return &Demonstration{Name: "a thing", Steps: steps}
}

// pressed is one interaction as the surface lists it.
//
// `on` is the SAMPLE of words shown to a person and `recognisable` is whether the screen has
// enough of them for a run to identify it. Two arguments and not one, because that is the
// distinction the review got wrong: it read the judgement off the length of the sample, and the
// sample is cut to six words.
func pressed(reads, on []string, outcome int, recognisable bool) Interaction {
	return Interaction{Pressable: true, Verifiable: outcome > 0, Reads: reads, On: on,
		OutcomeWords: outcome, Recognisable: recognisable}
}

func TestEveryWayACaptureCanBeUselessHasItsOwnRemedy(t *testing.T) {
	words := []string{"audio", "graphics"}
	for _, c := range []struct {
		name   string
		d      *Demonstration
		waited error
		failed bool
		says   string
		next   string
	}{
		{"nothing was saved at all", nil, nil, true,
			"Nothing was recorded", "with the MOUSE"},
		{"the screen moved on its own",
			aDemonstrationLike(Interaction{Position: 0}), nil, true,
			"nothing in it presses anything", "arrow keys names nothing"},
		{"a click nothing could be read after",
			aDemonstrationLike(pressed(words, words, 0, true)), nil, true,
			"could not confirm what changed", "five have to agree"},
		{"a click on a screen nobody read",
			aDemonstrationLike(pressed(words, nil, 61, false)), nil, true,
			"did not read enough of the screen",
			"resting on that screen BEFORE"},
		// AND A SCREEN READ TOO THINLY IS THE SAME FAULT, which is the one this
		// surface used to pass: two words are words to show somebody and are not
		// enough to tell one screen from another, so the run refuses the Act
		// while the review called it ready.
		{"a click on a screen read too thinly",
			aDemonstrationLike(pressed(words, words, 61, false)), nil, true,
			"did not read enough of the screen",
			"three words to tell one screen from another"},
		{"all of it",
			aDemonstrationLike(pressed(words, words, 61, true)), nil, false,
			"1 press(es), 1 of which", "Build an Act from it"},
	} {
		said, next, failed := learnedOf("a thing", c.d, c.waited)
		if failed != c.failed {
			t.Errorf("%s: failed=%v, want %v (%q)", c.name, failed, c.failed, said)
		}
		if !strings.Contains(said, c.says) {
			t.Errorf("%s: it says %q", c.name, said)
		}
		if !strings.Contains(next, c.next) {
			t.Errorf("%s: what to do is %q", c.name, next)
		}
	}
}

// AND A RECORDER THAT FAILED IS REPORTED AS WHAT IT WAS.
func TestARecorderThatSavedNothingSaysSoAndSaysWhy(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	aRecorderStandingIn(t, a)
	t.Setenv("STUDIO_STANDIN_RECORDER_FAILS", "1")

	if why := a.Learn("nothing doing", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	got := learningSaid(t, a)
	if !got.Failed {
		t.Errorf("a recording that saved nothing is reported as %+v", got)
	}
	if !strings.Contains(got.Said, "Nothing was recorded") {
		t.Errorf("it says %q", got.Said)
	}
	// AND THE RECORDER'S OWN WORDS ARE KEPT, as the evidence rather than as the verdict.
	if !strings.Contains(strings.Join(got.Transcript, "\n"), "nothing was demonstrated") {
		t.Errorf("what the recorder said is not kept: %v", got.Transcript)
	}
}

// AND A RECORDING THAT DID SAVE SOMETHING IS REVIEWED FROM THE ARTIFACT.
//
// Not from the recorder's output: the artifact is the thing that will be performed, and a
// surface that read its verdict out of printed sentences would change what it told somebody the
// moment somebody improved a sentence.
func TestWhatWasLearnedIsReadBackFromTheStore(t *testing.T) {
	a, art := aStoreWithADemonstration(t)
	aRecorderStandingIn(t, a)
	// THE ARTIFACT THE RECORDER WOULD HAVE LEFT, under the name being recorded. The
	// stand-in cannot watch a screen; what is under test is that the review reads the
	// store rather than the child.
	art.Name = "the tabs"
	// ONE PRESS, THE CHECKABLE ONE, which starts on the screen the fixture gives words
	// to. The whole artifact would be reviewed as a demonstration whose FIRST press
	// starts somewhere nothing was read — true of the fixture, and a different case
	// (held above, in TestEveryWayACaptureCanBeUselessHasItsOwnRemedy).
	art.Steps = []workflowstore.Step{art.Steps[1]}
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(workflowstore.Artifact(art)); err != nil {
		t.Fatal(err)
	}

	if why := a.Learn("the tabs", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	if why := a.Done(false); why != "" {
		t.Fatal(why)
	}
	got := learningSaid(t, a)
	if got.Learned == nil {
		t.Fatal("what was learned is not reported at all")
	}
	if got.Learned.Name != "the tabs" || len(got.Learned.Steps) != 1 {
		t.Errorf("what is reported is %+v", got.Learned)
	}
	// THAT PRESS STARTS ON A SCREEN WITH WORDS and can be checked, so this is the
	// ending that is not a failure.
	if got.Failed {
		t.Errorf("a usable demonstration is reported as a failure: %q / %q",
			got.Said, got.Next)
	}
	if !strings.Contains(got.Next, "Build an Act") {
		t.Errorf("what to do next is %q", got.Next)
	}
}

// FINISHING A RECORDING IS CHECKED BEFORE IT IS DONE.
//
// The recorder knows whether it obtained evidence and the person does not: the outcome is words
// read somewhere other than the control, which appeared when it was pressed and stayed — four
// terms over readings on both sides of a press, from a source that answers twice a second. A
// click whose outcome has not been read produces an interaction nothing can check, so the Act
// built from it presses and then reports that it could not confirm anything.
//
// The owner was being told to "wait two seconds" to avoid that, which is an estimate of how long
// seven OCR readings take. This is the same request made by the thing that can measure it.
func TestFinishingEarlySaysWhatIsStillMissing(t *testing.T) {
	for _, c := range []struct {
		what  string
		at    learnstate.Event
		says  string
		stops bool
	}{
		// NOTHING READ ON THE SCREEN THEY STARTED FROM: the Act built from this
		// could never tell it was there, which is exactly how the owner's first
		// live trial ended.
		{"not enough read yet", learnstate.Event{Phase: learnstate.Reading},
			"not enough has been read on the screen you are starting from", true},
		{"no press yet", learnstate.Event{Phase: learnstate.Ready,
			StartKnown: true}, "no press has been seen", true},
		// AND THE REASON IS THE RECORDER'S OWN, not this window's guess at one.
		{"the result is still in flight", learnstate.Event{
			Phase: learnstate.Checking, Presses: 1, Watched: 1, StartKnown: true,
			Missing: "nothing elsewhere on the window appeared and stayed"},
			"nothing elsewhere on the window appeared and stayed", true},
		{"the press landed on nothing", learnstate.Event{
			Phase: learnstate.Blind, Presses: 1, StartKnown: true},
			"landed on nothing Marco could see", true},
		// ── AND THE RESULT THAT IS IN, AND IS NOTHING ──
		//
		// Refused, as `Checking` is, because finishing here saves a press with
		// no check on it. What it may NOT say is "keep recording for a moment":
		// the readings have arrived, so that is a wait with nothing on the other
		// end of it — which is what the window asked for over twenty-nine
		// seconds of the owner’s Rocket League recording.
		{"the result is in and it is nothing", learnstate.Event{
			Phase: learnstate.Unchecked, Presses: 1, Watched: 1,
			Unchecked: 1, StartKnown: true,
			Missing: "nothing elsewhere on the window appeared and stayed"},
			"Waiting will not change that now", true},
		// AND A RECORDING WITH ITS EVIDENCE IN HAND FINISHES WITHOUT A WORD.
		{"the result is captured", learnstate.Event{
			Phase: learnstate.Captured, Presses: 1, Watched: 1, Captured: 1,
			StartKnown: true}, "", false},
		// ── AND THE SCREEN IT STARTS FROM IS ITS OWN QUESTION ──
		//
		// Reported against the first version of this: a captured outcome ended the
		// question, so a recording whose starting screen could never be
		// recognised finished without a word and produced an Act that paused on
		// it. The two faults have two remedies and finishing must name both.
		{"captured, on a screen nothing can name", learnstate.Event{
			Phase: learnstate.Captured, Presses: 1, Watched: 1, Captured: 1},
			"read enough of the screen this starts from", true},
		{"both at once", learnstate.Event{
			Phase: learnstate.Checking, Presses: 1, Watched: 1,
			Missing: "nothing elsewhere on the window appeared and stayed"},
			"Also: Marco has not read enough", true},
	} {
		a, _ := aStoreWithADemonstration(t)
		t.Cleanup(a.Close)
		aRecorderStandingIn(t, a)
		if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
			t.Fatalf("%s: the recorder would not start: %s", c.what, why)
		}
		sayRecording(t, a, c.at)

		why := a.Done(false)
		if c.says == "" {
			if why != "" {
				t.Errorf("%s: finishing was refused with %q", c.what, why)
			}
			continue
		}
		if !strings.Contains(why, c.says) {
			t.Errorf("%s: finishing says %q, which does not mention %q",
				c.what, why, c.says)
		}
		// AND IT OFFERS BOTH ANSWERS, because it is their demonstration and the
		// evidence may never arrive.
		if !strings.Contains(why, "finish anyway") {
			t.Errorf("%s: the refusal does not offer to finish anyway: %q",
				c.what, why)
		}
		// AND SAYING IT AGAIN FINISHES IT. A window that would not stop recording
		// is worse than one that recorded nothing useful.
		if again := a.Done(true); again != "" {
			t.Errorf("%s: finishing anyway was refused: %s", c.what, again)
		}
	}
}

// AND WHAT IS ON SCREEN WHILE IT RECORDS IS THE RECORDER'S ACCOUNT, not this window's clock.
//
// The clock is still the preparation countdown — the one stretch the recorder cannot report on,
// because it has not started looking — and everything after it comes off the stream.
func TestTheRecordingShowsWhatTheRecorderKnows(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aRecorderStandingIn(t, a)
	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}

	// PREPARING IS STILL OURS, and it names the application to switch to.
	if l := a.Learning(); l.Phase != string(learnstate.Preparing) ||
		!strings.Contains(l.Doing, "ArkAscended.exe") {
		t.Fatalf("the preparation reads as %+v", l)
	}

	// AND THEN THE RECORDER'S, with the counts it measured.
	sayRecording(t, a, learnstate.Event{Phase: learnstate.Checking,
		Presses: 2, Watched: 1, Captured: 0,
		Missing: "only a few words appeared and stayed"})
	waitForPhase(t, a, learnstate.Checking)

	l := a.Learning()
	if !strings.Contains(l.Doing, "Checking what changed") {
		t.Errorf("the recording does not say what the recorder is doing: %q", l.Doing)
	}
	if l.Presses != 2 || l.Watched != 1 || l.Captured != 0 {
		t.Errorf("the counts the recorder measured are %d/%d/%d",
			l.Presses, l.Watched, l.Captured)
	}
	if l.Missing != "only a few words appeared and stayed" {
		t.Errorf("the reason the outcome is missing is %q", l.Missing)
	}

	// AND WHETHER THE SCREEN IT STARTS FROM CAN BE RECOGNISED, which the page shows
	// for as long as it cannot: it is fixable only while the recording runs, and no
	// press makes up for it.
	sayRecording(t, a, learnstate.Event{Phase: learnstate.Captured,
		Presses: 1, Watched: 1, Captured: 1, StartKnown: true})
	waitForPhase(t, a, learnstate.Captured)
	if !a.Learning().StartKnown {
		t.Error("the recorder said the starting screen can be recognised and the " +
			"window does not carry it, so the caution would never come down")
	}

	// AND WHETHER THE SCREEN IN FRONT HAS BEEN READ AT ALL, which is what the NEXT
	// press needs and is a weaker question than the one about the starting screen.
	sayRecording(t, a, learnstate.Event{Phase: learnstate.Checking,
		Presses: 1, Watched: 1, StartKnown: true, ScreenRead: false,
		Missing: "2 of the 5 reading(s) persistence needs have arrived"})
	waitForPhase(t, a, learnstate.Checking)
	if a.Learning().ScreenRead {
		t.Error("the recorder said nothing has been read on the screen in front " +
			"and the window does not carry it, so a press made there would " +
			"be unanswerable and nobody would be told until they had waited")
	}
	// AND THE OTHER WAY, which is the half a mutation survives without: a window
	// that hard-coded `false` would pass the assertion above and never take the
	// caution down again.
	sayRecording(t, a, learnstate.Event{Phase: learnstate.Checking,
		Presses: 1, Watched: 1, StartKnown: true, ScreenRead: true,
		Missing: "3 of the 5 reading(s) persistence needs have arrived"})
	waitFor(t, a, func(l Learning) bool { return l.ScreenRead })

	// AND HOW MANY PRESSES THERE IS NOTHING LEFT TO WAIT FOR. Counted beside the
	// captures rather than instead of them, because a recording can have one of each
	// and the page shows both numbers.
	sayRecording(t, a, learnstate.Event{Phase: learnstate.Unchecked,
		Presses: 2, Watched: 2, Captured: 1, Unchecked: 1, StartKnown: true,
		Missing: "nothing elsewhere on the window appeared and stayed"})
	waitForPhase(t, a, learnstate.Unchecked)
	done := a.Learning()
	if done.Unchecked != 1 {
		t.Errorf("%d press(es) reported as measured and empty, and the recorder "+
			"said one", done.Unchecked)
	}
	// AND THE SENTENCE SAYS WAITING IS OVER rather than asking for more of it.
	if !strings.Contains(done.Doing, "waiting will not change") {
		t.Errorf("the sentence for a measured and empty result is %q, and what a "+
			"person needs to know is that waiting is no longer the thing to "+
			"do", done.Doing)
	}
}

// sayRecording writes one state onto the recording's own progress stream, as the recorder would.
//
// The stand-in recorder is a stand-in for the CHILD and does not observe anything, so there is
// nothing for it to report. What is under test here is the window: that it reads the stream
// rather than deciding for itself, and that it refuses to finish over what the stream says.
func sayRecording(t *testing.T, a *Acts, at learnstate.Event) {
	t.Helper()
	a.mu.Lock()
	path := ""
	if a.learn != nil {
		path = a.learn.progress
	}
	a.mu.Unlock()
	if path == "" {
		t.Fatal("the recording was started with nowhere to say what it knows")
	}
	w, err := learnstate.NewWriter(path, "the audio tab opening", "ArkAscended.exe")
	if err != nil {
		t.Fatal(err)
	}
	w.Say(at)
	w.Close()
}

// waitForPhase waits until the window reports a phase, so a test is not racing the clock the
// preparation countdown runs on.
func waitForPhase(t *testing.T, a *Acts, want learnstate.Phase) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if a.Learning().Phase == string(want) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("the window never reported %q; it says %+v", want, a.Learning())
}

// waitFor is waitForPhase for anything else the window reports.
func waitFor(t *testing.T, a *Acts, ok func(Learning) bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if ok(a.Learning()) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("the window never reported it; it says %+v", a.Learning())
}

// A RECORDER THIS WINDOW CANNOT READ IS SAID, NOT WAITED ON.
//
// Studio spawns the `director.exe` beside its own executable, and with two copies of Studio in
// the tree that can be a build from another day. Its stream then carries a version this build
// skips, Studio sees no events, and what it showed was "Reading the starting screen…" — a
// current window narrating a recorder it had never met, for as long as somebody kept recording.
// That cost the owner a day.
func TestARecorderFromAnotherBuildIsSaidRatherThanWaitedOn(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aRecorderStandingIn(t, a)
	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// A STREAM FROM ONE VERSION BACK, which is exactly what a stale sibling writes.
	a.mu.Lock()
	path := a.learn.progress
	a.mu.Unlock()
	if path == "" {
		t.Skip("this recording has no progress stream to stand in for")
	}
	if err := os.WriteFile(path, []byte(
		`{"version":`+strconv.Itoa(learnstate.Version-1)+
			`,"seq":1,"phase":"checking","presses":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, a, func(l Learning) bool {
		return strings.Contains(l.Doing, "different build")
	})
	l := a.Learning()
	if !strings.Contains(l.Doing, "build.cmd") {
		t.Errorf("the window says the build is wrong and not what to do about "+
			"it: %q", l.Doing)
	}
	// AND FINISHING SAYS THE SAME THING, rather than refusing for want of evidence
	// it cannot read either.
	if why := a.Done(false); !strings.Contains(why, "different build") {
		t.Errorf("finishing says %q", why)
	}
}

// AND A RECORDER OF A DIFFERENT BUILD IS NAMED, not inferred from a timestamp.
//
// The protocol version refuses a stream this build cannot parse. This is the other case: the
// shape matches and the behaviour does not, which is what a build one commit apart looks like.
// Git identifies the SOURCE; only the binaries can say what is running, so both say it.
func TestTheWindowSaysWhichBuildIsRecordingBesideItsOwn(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aRecorderStandingIn(t, a)
	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// A STREAM FROM A BUILD THAT IS NOT THIS ONE, in the current shape.
	sayRecording(t, a, learnstate.Event{Phase: learnstate.Ready, StartKnown: true,
		ScreenRead: true})
	waitForPhase(t, a, learnstate.Ready)

	l := a.Learning()
	if l.Build == "" {
		t.Error("the window does not say which build it is, so the recorder's " +
			"revision has nothing to be read against")
	}
	// The stand-in writer stamps its own revision, which IS this build — so these
	// agree, and that is the case that must not be reported as a mismatch.
	if l.Mismatched {
		t.Error("one process reports itself as two different builds")
	}

	// ── AND NOW THE PAIR THIS EXISTS FOR ──
	//
	// Two builds that genuinely differ. A test binary carries no VCS stamp, so
	// both sides have to be put there: the window's own through the substitutable
	// source, and the recorder's by writing the line a recorder would write.
	a.mu.Lock()
	a.revision = func() string { return "aaaaaaa" }
	path := a.learn.progress
	a.mu.Unlock()
	if path == "" {
		t.Skip("this recording has no progress stream")
	}
	line := `{"version":` + strconv.Itoa(learnstate.Version) +
		`,"seq":9,"phase":"ready","build":"bbbbbbb",` +
		`"start_known":true,"screen_read":true}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, a, func(l Learning) bool { return l.Mismatched })
	l = a.Learning()
	if l.Recorder != "bbbbbbb" || l.Build != "aaaaaaa" {
		t.Errorf("the window reports recorder=%q window=%q; both have to be "+
			"shown, because they differ is what somebody acts on and "+
			"the two revisions are what tell them which to rebuild",
			l.Recorder, l.Build)
	}
}
