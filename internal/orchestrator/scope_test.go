package orchestrator

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// WHERE a demonstrated play works — and the arm of that question nothing was testing.
//
// `chooseScope` offers three answers and `saveTaught` acts on all three, but every recorded
// fixture in this package answered "c". So `scopeFocus` — two lines in `chooseScope` and three in
// `saveTaught` — had no coverage at all, in the middle of the behaviour people like most: say the
// name from anywhere, and the app comes to the front by itself.
//
// FOCUS is the only scope that does two things rather than one. It decides where the play may be
// invoked from (anywhere) AND that the play switches windows first, and it is the only scope that
// KEEPS the demonstration's leading Activate — the other two strip it precisely so they cannot
// steal focus. Getting the wrong one of those wrong is silent: a focus play that lost its Activate
// still saves, still resolves, still runs, and simply fires its keys into whatever the person was
// looking at instead of the app they named.

// demonstrationInApp is a recorded demonstration that BEGINS by switching to app.
//
// The leading app switch is what `firstActivateApp` reads to decide which application a play
// belongs to, and what `saveTaught` keeps or strips depending on the scope. It has to be a real
// recorded event rather than a hand-built step so the whole pipeline — simplify, codegen, save —
// is the one the product runs.
func demonstrationInApp(app string) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: app, T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 100, Y: 200, T: at(400)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 100, Y: 200, T: at(405)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(900)}, // stop gesture
	}
}

// demonstrationAlreadyInApp is a demonstration by somebody who was ALREADY in the app.
//
// No leading app switch, because the recorder never saw one — the person was looking at the
// window when they started. This is the ordinary case, and it is the one that separates the two
// halves of focus: with no Activate among the steps, the only thing that can make a focus play
// switch windows is `saveTaught` passing the app to codegen. The fixture with a leading switch
// would still show an Activate whether or not it did.
func demonstrationAlreadyInApp() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 100, Y: 200, T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 100, Y: 200, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(900)}, // stop gesture
	}
}

// learnWithScope demonstrates one play and answers the scope prompt with answer.
func learnWithScope(t *testing.T, answer string) (routes.Registry, *bytes.Buffer) {
	t.Helper()
	return learnEventsWithScope(t, demonstrationInApp("notepad"), answer)
}

// learnEventsWithScope is learnWithScope over a given demonstration.
func learnEventsWithScope(t *testing.T, events []recorder.RecordedEvent, answer string) (
	routes.Registry, *bytes.Buffer) {

	t.Helper()
	reg := routes.Registry{Dir: t.TempDir()}
	out := &bytes.Buffer{}
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader("y\n" + answer + "\n"), // save? yes; where? answer
		Out:     out,
		App:     func() string { return "notepad" },
		StopKey: "esc",
	}
	if err := d.Learn("open the file"); err != nil {
		t.Fatalf("Learn: %v\n%s", err, out.String())
	}
	return reg, out
}

// A play saved with FOCUS lives in the focus scope and switches to its app before acting.
//
// Both halves matter and they fail differently. Losing the focus SCOPE makes the play invisible
// from anywhere else, which a person notices at once. Losing the leading ACTIVATE leaves a play
// that still saves and still runs and quietly types into the wrong window, which is the half that
// had no test.
func TestFocusScopeSavesUnderFocusAndActivatesItsApp(t *testing.T) {
	reg, out := learnWithScope(t, "f")

	focus := routes.Route{App: "notepad", Focus: true, Slug: "open-the-file"}
	if !reg.Has(focus) {
		t.Fatalf("a play saved with [f]ocus is not in the focus scope; output:\n%s", out.String())
	}
	// And it is NOT the context play. The two scopes are different directories, and a focus
	// answer that quietly landed in context would still resolve while the app is in front —
	// which is exactly how this arm could stay broken without anybody noticing.
	if reg.Has(routes.Route{App: "notepad", Slug: "open-the-file"}) {
		t.Error("a [f]ocus answer also wrote a context play")
	}

	src, err := os.ReadFile(reg.Path(focus))
	if err != nil {
		t.Fatalf("reading the saved play: %v", err)
	}
	if !strings.Contains(string(src), `Activate with "notepad"`) {
		t.Fatalf("a focus play does not bring its app to the front, so it will act on "+
			"whatever the person was looking at:\n%s", src)
	}

	// The person is told what they chose, in their words, naming no scope machinery.
	if !strings.Contains(out.String(), "in notepad from anywhere") {
		t.Errorf("the confirmation does not describe a focus play:\n%s", out.String())
	}
}

// And it activates even when the demonstration never switched apps.
//
// THE case the focus arm exists for, and the one the fixture above cannot see. When the person was
// already in the app there is no Activate among the recorded steps, so the ONLY thing that makes
// the saved play switch windows is `saveTaught` handing the app to codegen for the focus scope. Cut
// that one assignment and the fixture with a leading app switch still shows an Activate — it comes
// from the recording — while this one goes silently wrong.
func TestAFocusPlayActivatesEvenWhenTheDemonstrationDidNot(t *testing.T) {
	reg, out := learnEventsWithScope(t, demonstrationAlreadyInApp(), "f")

	focus := routes.Route{App: "notepad", Focus: true, Slug: "open-the-file"}
	if !reg.Has(focus) {
		t.Fatalf("a play saved with [f]ocus is not in the focus scope; output:\n%s", out.String())
	}
	src, err := os.ReadFile(reg.Path(focus))
	if err != nil {
		t.Fatalf("reading the saved play: %v", err)
	}
	if !strings.Contains(string(src), `do OS's Activate with "notepad".`) {
		t.Fatalf("a focus play recorded from inside its app never brings the app forward, so "+
			"invoking it from anywhere else types into the wrong window:\n%s", src)
	}
	// And it says so where a person reading the file will see it.
	if !strings.Contains(string(src), "brought to the foreground before running") {
		t.Errorf("the saved play does not say it switches windows:\n%s", src)
	}
}

// A CONTEXT play recorded the same way must NOT activate.
//
// The other side of the same assignment. Context means "only while this app is in front", and a
// context play that brought its app forward would be a focus play wearing the wrong name.
func TestAContextPlayNeverActivatesItsApp(t *testing.T) {
	reg, out := learnEventsWithScope(t, demonstrationAlreadyInApp(), "c")

	rt := routes.Route{App: "notepad", Slug: "open-the-file"}
	if !reg.Has(rt) {
		t.Fatalf("a play saved with [c] is not in the context scope; output:\n%s", out.String())
	}
	src, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatalf("reading the saved play: %v", err)
	}
	if strings.Contains(string(src), "Activate") {
		t.Fatalf("a context play switches windows, which is what [f]ocus is for:\n%s", src)
	}
}

// Every word for focus means focus.
//
// `chooseScope` re-prompts on anything it does not recognise, and a fixture that answered with an
// unlisted synonym would hang on a reader that has no more input rather than fail loudly. So each
// accepted spelling is pinned, and a wrong answer is pinned too — because "it re-prompted" and "it
// chose focus" look identical from outside unless somebody checks where the play landed.
func TestChooseScopeAcceptsEveryWordForFocus(t *testing.T) {
	for _, word := range []string{"f", "focus", "switch"} {
		t.Run(word, func(t *testing.T) {
			reg, out := learnWithScope(t, word)
			if !reg.Has(routes.Route{App: "notepad", Focus: true, Slug: "open-the-file"}) {
				t.Fatalf("%q was not understood as focus; output:\n%s", word, out.String())
			}
		})
	}
}

// Context and global strip the leading Activate; focus keeps it. That contrast IS the scope.
//
// Written as one table because the risk is not that a single arm breaks — it is that all three
// stop differing, which no single-arm test can see.
func TestOnlyAFocusPlaySwitchesWindows(t *testing.T) {
	for _, tc := range []struct {
		answer   string
		route    routes.Route
		activate bool
	}{
		{"c", routes.Route{App: "notepad", Slug: "open-the-file"}, false},
		{"f", routes.Route{App: "notepad", Focus: true, Slug: "open-the-file"}, true},
		{"g", routes.Route{Slug: "open-the-file"}, false},
	} {
		t.Run(tc.answer, func(t *testing.T) {
			reg, out := learnWithScope(t, tc.answer)
			if !reg.Has(tc.route) {
				t.Fatalf("answer %q did not save to %+v; output:\n%s",
					tc.answer, tc.route, out.String())
			}
			src, err := os.ReadFile(reg.Path(tc.route))
			if err != nil {
				t.Fatalf("reading the saved play: %v", err)
			}
			got := strings.Contains(string(src), "Activate")
			if got != tc.activate {
				if tc.activate {
					t.Fatalf("a focus play must switch to its app first:\n%s", src)
				}
				t.Fatalf("answer %q produced a play that steals focus:\n%s", tc.answer, src)
			}
		})
	}
}

// With no application in front there is nothing to focus, so nothing is asked.
//
// The early return in `chooseScope`. Offering "only in " and "in  from anywhere" with an empty
// name would be asking a person to choose between two blanks.
func TestWithNoApplicationTheScopeQuestionIsNotAsked(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	out := &bytes.Buffer{}
	d := Deps{
		Reg: reg,
		Rec: &fakeRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 10, Y: 20, T: at(0)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 10, Y: 20, T: at(5)},
			{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
		}},
		In:      strings.NewReader("y\n"), // save? yes — and NOTHING for a scope answer
		Out:     out,
		App:     func() string { return "" },
		StopKey: "esc",
	}
	if err := d.Learn("click there"); err != nil {
		t.Fatalf("Learn: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "Where?") {
		t.Errorf("the scope question was asked with no application to name:\n%s", out.String())
	}
	if !reg.Has(routes.Route{Slug: "click-there"}) {
		t.Fatalf("an app-less demonstration did not become a global play:\n%s", out.String())
	}
}
