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
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
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
	if why := a.Done(); why != "" {
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

	if why := a.Play("the audio page", false, ""); why == "" {
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

func pressed(reads, on []string, outcome int) Interaction {
	return Interaction{Pressable: true, Verifiable: outcome > 0, Reads: reads, On: on,
		OutcomeWords: outcome}
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
			aDemonstrationLike(pressed(words, words, 0)), nil, true,
			"could not confirm what changed", "five have to agree"},
		{"a click on a screen nobody read",
			aDemonstrationLike(pressed(words, nil, 61)), nil, true,
			"read no words on the screen", "resting on that screen BEFORE"},
		{"all of it",
			aDemonstrationLike(pressed(words, words, 61)), nil, false,
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
	if why := a.Done(); why != "" {
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
