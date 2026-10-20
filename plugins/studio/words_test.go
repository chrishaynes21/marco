package main

// WHAT A PERSON IS LOOKING AT, IN THINGS THAT ARE ON THEIR OWN SCREEN.
//
// `control_2` and `screen_a` are names this machine gave itself. Shown to somebody choosing what
// to perform they are worse than nothing, because they look like information: the owner, reading
// a list of them — "it's screen_a to screen_b, how tf am I supposed to know what that means".
//
// So what a surface shows is what was READ: on the control, on the screen it starts from, and
// afterwards. And the absence of a reading is itself the thing a person most needs to know,
// because a screen with no words cannot be identified and an Act over it can only wait.

import (
	"strings"
	"testing"
)

// interactionAt is one step of one demonstration, as the surface offers it.
func interactionAt(t *testing.T, a *Acts, demo string, position int) Interaction {
	t.Helper()
	for _, d := range a.Demonstrations() {
		if d.Name != demo {
			continue
		}
		for _, s := range d.Steps {
			if s.Position == position {
				return s
			}
		}
	}
	t.Fatalf("there is no interaction %d in %q", position, demo)
	return Interaction{}
}

// AN INTERACTION IS NAMED BY THE WORDS ON IT AND PLACED BY THE WORDS AROUND IT.
func TestAnInteractionIsDescribedByWhatWasReadOnTheScreen(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)

	// THE ONE THAT PRESSES A CONTROL SOMEBODY CAN RECOGNISE.
	in := interactionAt(t, a, "ark graphics live", 1)
	if got := strings.Join(in.Reads, " "); got != "audio camera cosmetics graphics video" {
		t.Errorf("the control reads %q — the two-character fragments of a reading of "+
			"a game's own interface are noise in a label, and there were five "+
			"longer words to show instead", got)
	}
	// AND IT STARTS ON A SCREEN THAT WAS READ, steadiest words first.
	if got := strings.Join(in.On, " "); got != "audio graphics settings brightness" {
		t.Errorf("the screen it starts on reads %q, and the words it was read to "+
			"carry — in the order they held still — are audio, graphics, "+
			"settings, then brightness", got)
	}

	// THE ONE THAT STARTS SOMEWHERE NOTHING WAS READ. Empty, which is the honest answer
	// and the warning: see TestAnActThatCouldOnlyWaitSaysSoBeforeItIsRun.
	first := interactionAt(t, a, "ark graphics live", 0)
	if len(first.On) != 0 {
		t.Errorf("a screen nobody read words on is described as reading %v", first.On)
	}
	if len(first.Reads) != 0 {
		t.Errorf("a control nobody read words on is described as reading %v",
			first.Reads)
	}
}

// AND A SHORT WORD IS STILL SHOWN WHEN IT IS ALL THERE IS.
//
// The length rule exists to spend a label on words somebody can recognise, not to hide a
// control. "OK" is two characters and is the whole identity of the button it is on; saying
// nothing about that control would be worse than saying "OK".
func TestTheOnlyWordsThereAreGetShown(t *testing.T) {
	for _, c := range []struct {
		name string
		have []string
		want string
	}{
		{"fragments dropped while there is anything better",
			[]string{"od", "ul", "cosmetics", "graphics"}, "cosmetics graphics"},
		{"the only word there is, however short", []string{"ok"}, "ok"},
		{"all of them short", []string{"ok", "no"}, "ok no"},
		{"capped at what a label can carry",
			[]string{"one", "two", "three", "four", "five", "six", "seven"},
			"one two three four five six"},
		{"nothing read", nil, ""},
	} {
		if got := strings.Join(wordsToShow(c.have, controlWordsShown), " "); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// AN ACT THAT COULD ONLY WAIT SAYS SO BEFORE IT IS RUN.
//
// # The trial this is for
//
// The owner composed an Act over an interaction whose starting screen the demonstration had read
// no words on, pressed Play, and watched for thirty seconds. The runner behaved correctly on
// every look — "screen_b was saved without any words, so a reading cannot identify it" — and
// then refused, having pressed nothing. The list had said `ready`.
//
// Ready was true: the Act preflights, and it would perform perfectly against a screen Marco
// could recognise. Both things have to be sayable at once, so this is a second sentence rather
// than a refusal, and it says where the missing thing is — in the demonstration, not the Act.
func TestAnActThatCouldOnlyWaitSaysSoBeforeItIsRun(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "the waiting one",
		[]string{"0=press something"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	if r := a.Compose("ark graphics live", "the one that can start",
		[]string{"1=press the tabs"}); r.Error != "" {
		t.Fatal(r.Error)
	}

	waiting, starts := actCalled(t, a, "the waiting one"), actCalled(t, a, "the one that can start")
	if !waiting.Runnable {
		t.Fatalf("the premise is gone: the Act does not preflight at all (%s)",
			waiting.Why)
	}
	if !strings.Contains(waiting.Waits, "read no words on") {
		t.Errorf("an Act that can only wait is listed as ready with nothing "+
			"else said: %q", waiting.Waits)
	}
	if !strings.Contains(waiting.Waits, "Demonstrate it again") {
		t.Errorf("the sentence does not say what to do about it: %q", waiting.Waits)
	}
	// AND THE ONE THAT CAN START SAYS NOTHING, because a warning on every Act is a
	// warning nobody reads.
	if starts.Waits != "" {
		t.Errorf("an Act whose starting screen was read is warned about anyway: %q",
			starts.Waits)
	}
}

// AND AN ACT IS NOT ONE OF ITS OWN INTERACTIONS.
//
// Both become declarations in the play this writes, so one name for two things is a compile
// error — `graph: 16:1: duplicate declaration: AudioTab`, which is true, and which was the
// answer somebody got for naming their first Act after the thing it presses.
func TestAnActCannotBeCalledWhatOneOfItsInteractionsIsCalled(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)

	r := a.Compose("ark graphics live", "The Audio Tab", []string{"1=the audio tab"})
	if r.Error == "" {
		t.Fatal("an Act was composed with its own interaction's name")
	}
	if !strings.Contains(r.Error, "name of its own") {
		t.Errorf("the refusal does not say what to do: %q", r.Error)
	}
	if len(a.Saved()) != 0 {
		t.Errorf("the refused Act was saved anyway: %v", a.Saved())
	}
}
