package lastattempt

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The question is recognised however it arrives — typed with punctuation, or spoken after the wake
// word — and an ordinary request that happens to start with "why" is not it.
func TestTheQuestionIsAFixedSetOfPhrasings(t *testing.T) {
	for _, q := range []string{"why", "Why didn't that work?", "why didnt that work",
		"marco, why didn’t it work", "What happened?", "what went wrong", "why did nothing happen"} {
		if !Asks(q) {
			t.Errorf("%q is not taken as the question", q)
		}
	}
	for _, q := range []string{"why is the sky blue", "open what happened folder", "cloud pull stone", ""} {
		if Asks(q) {
			t.Errorf("%q is taken as the question", q)
		}
	}
}

// What was supplied is taken out as whole words, ignoring case, longest first; a word that merely
// contains it is left.
func TestScrubTakesOutWhatWasSupplied(t *testing.T) {
	got := Scrub(`typing "Stone Wall" failed: stone? Stones are elsewhere`, []string{"stone", "stone wall", " "})
	want := `typing "` + Withheld + `" failed: ` + Withheld + `? Stones are elsewhere`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// The record is replaced whole and read back as written; no record yet is not an error.
func TestTheRecordIsReplacedWhole(t *testing.T) {
	home := t.TempDir()
	if _, ok, err := Read(home); ok || err != nil {
		t.Fatalf("an empty home read as %v, %v", ok, err)
	}
	for _, a := range []Attempt{{Outcome: "refused", Stage: StageValues}, {Outcome: "performed", Stage: StageRunning, Run: "r1"}} {
		if err := Write(home, a); err != nil {
			t.Fatal(err)
		}
	}
	a, ok, err := Read(home)
	if !ok || err != nil || a.Outcome != "performed" || a.Run != "r1" || a.Version != 1 {
		t.Errorf("read back %+v %v %v", a, ok, err)
	}
	left, _ := os.ReadDir(home)
	if len(left) != 1 {
		t.Errorf("the home holds %d files, want only the record", len(left))
	}
}

// Each outcome reads as what happened, with why and what next when the record has them.
func TestTheAnswerSaysWhatHappenedWhyAndWhatNext(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	at := now.Add(-3 * time.Minute).Format(time.RFC3339)
	for _, c := range []struct {
		a    Attempt
		want []string
	}{
		{Attempt{At: at, Routine: "cloud pull", Outcome: "refused", Stage: StageValues,
			Because: "no value was given for resource", Next: "say it with resource:…"},
			[]string{"“cloud pull” was refused before anything ran (3 minutes ago).",
				"Why: no value was given for resource", "Next: say it with resource:…"}},
		{Attempt{At: at, Outcome: "unavailable", Stage: StageMatching, Because: "no saved routine answers"},
			[]string{"Nothing took your last command (3 minutes ago).", "Why: no saved routine answers"}},
		{Attempt{At: at, Routine: "open social", Outcome: "performed", Stage: StageRunning, Run: "run-7"},
			[]string{"“open social” ran (3 minutes ago).", "did what it was saved to do", "Its run record: run-7"}},
		{Attempt{At: at, Routine: "open social", Outcome: "failed", Stage: StageRunning, Because: "the key was refused"},
			[]string{"“open social” started and failed", "Why: the key was refused"}},
		{Attempt{At: at, Outcome: "refused", Stage: StageDirector}, []string{"Your last command was refused (3 minutes ago)."}},
	} {
		got := strings.Join(c.a.Says(now), "\n")
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%+v says\n%s\nwhich lacks %q", c.a, got, w)
			}
		}
	}
}

// What the recogniser is told to expect is what Asks answers to.
func TestEveryOfferedPhrasingIsAnswered(t *testing.T) {
	ps := Phrasings()
	if len(ps) < len(questions) {
		t.Fatalf("%d phrasings for %d questions", len(ps), len(questions))
	}
	for _, p := range ps {
		if !Asks(p) {
			t.Errorf("%q is offered to the recogniser and not answered", p)
		}
	}
}
