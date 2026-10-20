package main

// THE FIXTURES, AND WHICH OF THEM ARE PRODUCTION.
//
// Production, unchanged, in every test in this package:
//
//	workflowrun.Runner       the coordinator: grounding, authorisation, the pointer and hover
//	                         probes, the press, the outcome window, the verdict
//	workflowrun's verifier   verify.go, deciding verified / contradicted / unconfirmed
//	marcoexec.Executor       the one path to an effect, lowering each operation to legal Marco
//	driver.RunFileWithHosts  the runtime, reading the play off disk and dispatching its sentences
//	demonstrated.*           the bridge under test
//
// Simulated, and each one stands exactly where a real thing stands:
//
//	dpNotebook                 the MarcoRunner. Writes the program down and performs nothing, so
//	                         NO TEST IN THIS PACKAGE EMITS DESKTOP INPUT.
//	dpAWalkingGrounder         the person naming which dpSaved screen they are on
//	dpAnswersWhereItWasAsked   the person agreeing to each press, recording what it was asked
//	theLooks                 the observation loop. In production this closure is the capture,
//	                         propose and observe body of performworkflow.go; here it is a
//	                         scripted sequence that responds to how many presses have gone out.
//
// The fixture shapes are deliberately the ones in `workflowrun/twostep_test.go`, because that is
// the acceptance [[ADR-242-the-second-action-waits-for-the-first-ones-answer]] was closed with
// and this bridge has to hold the same rule from the other side of the act boundary.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var (
	// dpLooked is the fixture clock's origin. A test starting anywhere else is handed a
	// grant that expired before it was asked for.
	dpLooked = time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	dpWindow = directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500}
)

// dpFirstOutcome and dpSecondOutcome are what each interaction's demonstration read afterwards.
func dpFirstOutcome() []string {
	return []string{"bloom", "foliage", "presets", "shadows", "textures"}
}
func dpSecondOutcome() []string {
	return []string{"ambient", "master", "music", "speech", "voice"}
}

// dpATwoTapRoutine is two presses across three screens, each interaction with an outcome of its
// own — the case this bridge exists for, and the case that used to advance on an arrival alone.
func dpATwoTapRoutine() workflowstore.Artifact {
	control := func(label string, y float64) workflowstore.Control {
		return workflowstore.Control{Label: label,
			Readings: []observe.Region{
				{X: 0.410, Y: y, Width: 0.072, Height: 0.028}},
			Containers: []observe.Region{
				{X: 0.027, Y: y - 0.011, Width: 0.946, Height: 0.050}}}
	}
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics live",
		SavedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Screens: []workflowstore.Screen{
			{Label: "screen_a"}, {Label: "screen_b"}, {Label: "screen_c"},
		},
		Controls: []workflowstore.Control{
			control("control_1", 0.241), control("control_2", 0.291),
		},
		Steps: []workflowstore.Step{
			{
				Position: 0, From: "screen_a", To: "screen_b",
				Control: "control_1", Cause: observe.CauseClick, Seen: 2,
				Outcome: dpFirstOutcome(),
			},
			{
				Position: 1, From: "screen_b", To: "screen_c",
				Control: "control_2", Cause: observe.CauseClick, Seen: 2,
				Outcome: dpSecondOutcome(),
			},
		},
	}
}

// dpTheNames are the names a person gave the two interactions, which is what the play says.
func dpTheNames() []demonstrated.Named {
	return []demonstrated.Named{
		{Position: 0, Name: "the graphics tab"},
		{Position: 1, Name: "apply changes"},
	}
}

// dpNotebook is a MarcoRunner that writes the program down and performs nothing.
type dpNotebook struct {
	programs []string
}

func (n *dpNotebook) Run(ctx context.Context, name, program string) (
	directorapi.MarcoResult, error) {

	n.programs = append(n.programs, program)
	return directorapi.MarcoResult{Output: "[dryrun] OS's Click\n"}, nil
}

// clicks is how many of the programs written down actually press something.
//
// An action is a move and then a press, so counting PROGRAMS answers a different question from
// the one these tests ask.
func (n *dpNotebook) clicks() int {
	c := 0
	for _, p := range n.programs {
		if strings.Contains(p, "OS's Click") {
			c++
		}
	}
	return c
}

// dpAWalkingGrounder names each live screen as the dpSaved screen a test says it is.
//
// Stands where `promptGrounder` stands, and establishes nothing by itself: a label per observed
// screen id, which is the correspondence a person makes.
type dpAWalkingGrounder struct {
	says  map[observe.VisualScreenID]string
	times int
}

func (g *dpAWalkingGrounder) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	label, ok := g.says[at.Screen]
	if !ok {
		return nil
	}
	g.times++
	err := s.Confirm(label, at.Screen, at.Epoch, at.Generation, "SIMULATED fixture", at.At)
	if err != nil && strings.Contains(err.Error(), "already") {
		return nil
	}
	return err
}

// dpAnswersWhereItWasAsked is a granter that stamps its answer with the observation it was asked
// in, which is what a well-behaved one does — and records every question, because whether the
// second interaction was ASKED ABOUT is the assertion these tests exist for.
type dpAnswersWhereItWasAsked struct {
	asked []workflowrun.Intent
}

func (g *dpAnswersWhereItWasAsked) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	g.asked = append(g.asked, in)
	return workflowrun.Authorisation{
		Step: in.Step, By: "SIMULATED fixture",
		Epoch: in.Epoch, Generation: in.Generation, Window: in.Window,
		Expires: dpLooked.Add(time.Hour),
	}, true
}

// dpAskedAbout is how many times an interaction was put to the granter.
func dpAskedAbout(g *dpAnswersWhereItWasAsked, step int) int {
	n := 0
	for _, in := range g.asked {
		if in.Step == step {
			n++
		}
	}
	return n
}

// dpPanelWords is a reading of the panel below the tab bar, which is where an outcome may be read.
func dpPanelWords(words ...string) []observe.ReadWord {
	out := make([]observe.ReadWord, 0, len(words))
	for i, w := range words {
		out = append(out, observe.ReadWord{Text: w, Where: observe.Region{
			X: 0.10, Y: 0.35 + float64(i)*0.03, Width: 0.20, Height: 0.025}})
	}
	return out
}

// dpBeforeWords is a reading of the screen an interaction is standing ON, before anything is
// pressed.
//
// Needed, and not scenery: an interaction with an outcome expectation will not be acted on until
// the screen it stands on has been read, because a verdict needs both sides of the action.
func dpBeforeWords() []observe.ReadWord {
	return dpPanelWords("brightness", "contrast", "gamma")
}

// dpTheWalk is one observation: which live screen, which control is findable, what was read.
func dpTheWalk(screen observe.VisualScreenID, epoch uint64, control string,
	at time.Time, words []observe.ReadWord) workflowrun.Sighting {

	y := 0.241
	if control == "control_2" {
		y = 0.291
	}
	return workflowrun.Sighting{
		Screen: screen, Epoch: epoch, Generation: 1,
		Window: "hwnd:1", Bounds: dpWindow, Front: true, At: at,
		Outcomes: []observe.Reacquisition{{
			Target: observe.VisualTargetID(control), Screen: screen,
			Result:   observe.ReacquireMatched,
			Where:    observe.Region{X: 0.410, Y: y, Width: 0.072, Height: 0.028},
			Controls: 1, Because: "a fixture outcome",
		}},
		Words: words,
	}
}

// dpStage is the scripted observation loop, standing where the live one does.
//
// It answers each look from HOW MANY PRESSES HAVE ACTUALLY GONE OUT, which is what makes it a
// world rather than a tape: the runner decides when to press, and what it then sees is the
// consequence. So a run that refuses to press stays on the screen it refused on, exactly as a
// person's computer would.
//
// Each slice is indexed by that press count. Past the end, the last entry stands.
type dpStage struct {
	book *dpNotebook
	now  *time.Time
	// screen, control and words are what the world looks like after N presses.
	screen  []observe.VisualScreenID
	control []string
	words   [][]observe.ReadWord
	// both offers EVERY control on every look, which is what a one-screen panel does.
	both bool
	// seen counts looks handed out, and limit bounds them; zero is unbounded.
	seen, limit int
	// watch is called with the press count on every look, so a test can make something
	// happen at the moment an action has gone out — a cancellation, in particular, which
	// has to land after one press and before the next.
	watch func(clicks int)
}

func (s *dpStage) look() (workflowrun.Sighting, bool) {
	if s.limit > 0 && s.seen >= s.limit {
		return workflowrun.Sighting{}, false
	}
	s.seen++
	*s.now = (*s.now).Add(100 * time.Millisecond)
	n := s.book.clicks()
	if s.watch != nil {
		s.watch(n)
	}
	at := func(i int) int {
		if i >= len(s.screen) {
			return len(s.screen) - 1
		}
		return i
	}
	i := at(n)
	// THE EPOCH MOVES WITH THE SCREEN. A grant is stamped with the observation it was
	// asked in, and a run whose epoch never changed would be a world that never moved.
	if s.both {
		return dpTheWholePanel(s.screen[i], uint64(5+i), *s.now, s.words[i]), true
	}
	return dpTheWalk(s.screen[i], uint64(5+i), s.control[i], *s.now, s.words[i]), true
}

// dpAWalk is the three-screen world: press the tab, the panel appears; press apply, the next
// panel appears. `after` is what is read once each press has gone out.
func dpAWalk(afterFirst, afterSecond []observe.ReadWord) *dpStage {
	return &dpStage{
		screen:  []observe.VisualScreenID{"now_1", "now_2", "now_3"},
		control: []string{"control_1", "control_2", "control_2"},
		words:   [][]observe.ReadWord{dpBeforeWords(), afterFirst, afterSecond},
	}
}

// dpAPanel is the ONE-screen world, where both interactions are on the screen the run starts on.
//
// Which is what makes a reordering test possible at all: on the three-screen walk, interaction 1
// cannot be performed first because its screen is not up — a refusal that is correct and is its
// own test. Here either order is groundable, so the order performed is the play's and nothing
// else's.
func dpAPanel(after ...[]observe.ReadWord) *dpStage {
	// BOTH CONTROLS FINDABLE ON EVERY LOOK, because they are both on this screen. The
	// first draft offered one at a time, and a reordered play was then refused for a
	// reason the fixture had invented rather than the world having.
	s := &dpStage{
		screen: []observe.VisualScreenID{"now_1"},
		both:   true,
		words:  [][]observe.ReadWord{dpBeforeWords()},
	}
	for _, w := range after {
		s.screen = append(s.screen, "now_1")
		s.words = append(s.words, w)
	}
	return s
}

// dpARun is the production coordinator over that routine, with a clock a test can advance.
func dpARun(t *testing.T, art workflowstore.Artifact, now *time.Time) (
	*workflowrun.Runner, *workflowstore.SessionBinding,
	*dpAnswersWhereItWasAsked, *dpAWalkingGrounder, *dpNotebook) {

	t.Helper()
	s := workflowstore.NewSessionBinding()
	// BOTH INTERACTIONS AGREED TO as belonging in the run, so what is under test is the
	// outcome gate rather than the separate question of whether a step belongs at all.
	for i := range art.Steps {
		if err := s.ConfirmStep(i, "SIMULATED fixture"); err != nil {
			t.Fatal(err)
		}
	}
	grant := &dpAnswersWhereItWasAsked{}
	ground := &dpAWalkingGrounder{says: map[observe.VisualScreenID]string{
		"now_1": "screen_a", "now_2": "screen_b", "now_3": "screen_c",
	}}
	book := &dpNotebook{}
	run := workflowrun.New(art, nil, ground, grant).
		WithExecutor(marcoexec.New(book), false).
		WithClock(func() time.Time { return *now }).
		WithConfirm(func(context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {
			return true, ""
		}).
		WithPatience(60)
	return run, s, grant, ground, book
}

// dpATwoTapPanel is two interactions ON ONE SCREEN, each with an outcome of its own.
//
// The fixture a reordering test needs. On the three-screen walk, performing interaction 1 first
// is refused because its screen is not up — correct, and its own test. Here both interactions
// are groundable from the start, so what decides the order is the play.
func dpATwoTapPanel() workflowstore.Artifact {
	a := dpATwoTapRoutine()
	a.Name = "ark graphics panel"
	a.Screens = []workflowstore.Screen{{Label: "screen_a"}}
	for i := range a.Steps {
		a.Steps[i].From, a.Steps[i].To = "screen_a", "screen_a"
	}
	return a
}

// dpTheWholePanel is one look of a screen where EVERY demonstrated control is findable.
//
// What a settings panel actually is: both controls are on it at once, so which one a run presses
// is decided by the run and not by what the fixture chose to offer.
func dpTheWholePanel(screen observe.VisualScreenID, epoch uint64, at time.Time,
	words []observe.ReadWord) workflowrun.Sighting {

	s := dpTheWalk(screen, epoch, "control_1", at, words)
	s.Outcomes = append(s.Outcomes, observe.Reacquisition{
		Target: "control_2", Screen: screen,
		Result:   observe.ReacquireMatched,
		Where:    observe.Region{X: 0.410, Y: 0.291, Width: 0.072, Height: 0.028},
		Controls: 1, Because: "a fixture outcome",
	})
	return s
}
