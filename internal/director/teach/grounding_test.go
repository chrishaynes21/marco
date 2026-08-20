package teach_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// Showing the user the two decisions Teach makes on their behalf.
//
// The property every test here defends: grounding is a PICTURE of a decision that has already been
// made. It cannot make one, unmake one, or change what happens next — so a Director that cannot
// point teaches exactly the same lesson as one that can, and the only difference is what the person
// gets to see.

// recordingGround is a grounding that answers whatever the test tells it to, and remembers what it
// was asked.
type recordingGround struct {
	asked []groundCall
	reply func(state observe.ScreenStateID, role observe.ReferentRole) observe.VisualReferent
}

type groundCall struct {
	state observe.ScreenStateID
	role  observe.ReferentRole
	app   string
}

func (g *recordingGround) Ground(t observe.ShadowTotals, application string,
	state observe.ScreenStateID, role observe.ReferentRole) observe.VisualReferent {

	g.asked = append(g.asked, groundCall{state: state, role: role, app: application})
	if g.reply != nil {
		return g.reply(state, role)
	}
	return observe.VisualReferent{
		Role: role, Application: application, Regions: []observe.Region{
			{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.05}},
		About: "3 controls I recognise this screen by",
	}
}

// pointable is a grounding that always succeeds.
func pointable() *recordingGround { return &recordingGround{} }

// blind is a grounding that can never point, for every reason it might not be able to.
func blind(why observe.ReferentUnavailable) *recordingGround {
	return &recordingGround{reply: func(_ observe.ScreenStateID,
		role observe.ReferentRole) observe.VisualReferent {

		return observe.VisualReferent{Role: role, Unavailable: why}
	}}
}

// THE fail-closed test, and the reason this whole feature is safe to add to a learning flow.
//
// A teach session that cannot point must be IDENTICAL to one that never tried. Not similar:
// identical in phase, in the subject it established, in the route it discovered, in what it wrote
// to the store and in what it asked the user for. Grounding is presentation, and presentation that
// can change a judgement is not presentation.
//
// The mutations this kills: branching on the referent anywhere in the coordinator, gating the
// phase change on a successful grounding, or refusing the session when the grounding fails.
func TestGroundingFailureDoesNotChangeAnythingAboutTheTeachSession(t *testing.T) {
	ungrounded := runToRoute(t, nil)

	for _, why := range []observe.ReferentUnavailable{
		observe.ReferentNothingWatched,
		observe.ReferentNotOnScreen,
		observe.ReferentNotAPart,
		observe.ReferentCoordinatesUnreliable,
		observe.ReferentAnotherApplication,
	} {
		got := runToRoute(t, blind(why))
		switch {
		case got.Phase != ungrounded.Phase:
			t.Errorf("with grounding refused (%s) the session reached %q; a session that "+
				"never tried to point reached %q", why, got.Phase, ungrounded.Phase)
		case got.Refusal != ungrounded.Refusal:
			t.Errorf("with grounding refused (%s) the session refused with %q, want %q",
				why, got.Refusal, ungrounded.Refusal)
		case got.Start != ungrounded.Start:
			t.Errorf("with grounding refused (%s) the start became %q, want %q",
				why, got.Start, ungrounded.Start)
		case got.Route != ungrounded.Route:
			t.Errorf("with grounding refused (%s) the route became %+v, want %+v",
				why, got.Route, ungrounded.Route)
		}
	}

	// And a grounding that DOES point changes nothing either — the same properties, from the
	// other side. A picture that improved the evidence would be evidence.
	shown := runToRoute(t, pointable())
	if shown.Phase != ungrounded.Phase || shown.Start != ungrounded.Start ||
		shown.Route != ungrounded.Route {
		t.Errorf("being able to point changed the session: %q/%q/%+v vs %q/%q/%+v",
			shown.Phase, shown.Start, shown.Route,
			ungrounded.Phase, ungrounded.Start, ungrounded.Route)
	}
}

// Grounding is asked about the screen the decision was made ON, and about nothing else.
//
// The mutation: ground whatever the current state is at display time. Every box still appears, and
// the START highlight silently becomes a confirmation of wherever the user is standing.
func TestEachEndpointIsGroundedOnTheScreenItWasEstablishedOn(t *testing.T) {
	g := pointable()
	s := runToRoute(t, g)

	if len(g.asked) != 2 {
		t.Fatalf("grounding was asked %d time(s), want one for the start and one for the "+
			"destination: %+v", len(g.asked), g.asked)
	}
	if g.asked[0].role != observe.ReferentTeachStart ||
		g.asked[1].role != observe.ReferentTeachDestination {
		t.Fatalf("the roles were %q then %q", g.asked[0].role, g.asked[1].role)
	}
	for i, call := range g.asked {
		if call.state == "" {
			t.Errorf("endpoint %d was grounded with no screen named; the resolver would have "+
				"to pick one, and the only one available is the current one", i)
		}
		if call.app != app {
			t.Errorf("endpoint %d was grounded against %q, want the application being "+
				"taught (%q)", i, call.app, app)
		}
	}
	if s.StartState == "" || s.DestinationState == "" {
		t.Fatalf("the session did not pin its screens: start=%q destination=%q",
			s.StartState, s.DestinationState)
	}
	// The script begins on one screen and ends on another, so a coordinator that grounded both
	// endpoints against the same moment is visible here and nowhere else.
	if s.StartState == s.DestinationState {
		t.Fatalf("both endpoints were pinned to %q. The demonstration began on one screen and "+
			"ended on another, so START and DESTINATION would highlight the same thing and a "+
			"person could not tell them apart", s.StartState)
	}
	if g.asked[0].state != s.StartState || g.asked[1].state != s.DestinationState {
		t.Errorf("grounding was asked about %q then %q; the session pinned %q and %q",
			g.asked[0].state, g.asked[1].state, s.StartState, s.DestinationState)
	}
	if s.StartReferent == nil || s.DestinationReferent == nil {
		t.Fatal("an endpoint kept no referent, so nothing downstream can show it")
	}
}

// A Director that cannot point at anything still teaches, and says so rather than going quiet.
func TestADirectorWithNoGroundingStillTeachesAndSaysItCannotShowYou(t *testing.T) {
	s := runToRoute(t, nil)

	if s.StartReferent != nil || s.DestinationReferent != nil {
		t.Fatal("a coordinator with no grounding invented a referent")
	}
	lines := s.Grounded()
	if len(lines) != 2 {
		t.Fatalf("%d endpoint line(s), want START and DESTINATION", len(lines))
	}
	for _, l := range lines {
		if !strings.Contains(l.Say, "can't point") {
			t.Errorf("%s reads %q; it should say plainly that Marco can't show you",
				l.Label, l.Say)
		}
		assertNoBackstageLeak(t, l.Say)
	}
}

// Every reason a grounding can fail produces a sentence, and none of them is the question wording.
//
// The shared refusals are phrased for a question — "I know which structure I'm asking about" — and
// a teach session is not asking anything. Reusing them verbatim would read as Marco being confused
// about what it is doing.
func TestASettledEndpointThatCannotBePointedAtSaysWhyInTeachingWords(t *testing.T) {
	for _, why := range []observe.ReferentUnavailable{
		observe.ReferentNothingWatched,
		observe.ReferentNotOnScreen,
		observe.ReferentNotAPart,
		observe.ReferentCoordinatesUnreliable,
		observe.ReferentAnotherApplication,
	} {
		s := runToRoute(t, blind(why))
		lines := s.Grounded()
		if len(lines) == 0 {
			t.Fatalf("%s produced no line at all; the user is left waiting for a highlight "+
				"that is never coming", why)
		}
		for _, l := range lines {
			if !strings.HasPrefix(l.Say, l.Label+" — settled") {
				t.Errorf("%s reads %q; it must say the endpoint IS settled before it says "+
					"it cannot be shown", why, l.Say)
			}
			if strings.Contains(l.Say, "asking about") {
				t.Errorf("%s reads %q, which is the wording for a question. Teach is not "+
					"asking anything", why, l.Say)
			}
			assertNoBackstageLeak(t, l.Say)
		}
	}
}

// A grounded endpoint says what the highlight is, and never that the screen is those controls.
func TestAGroundedEndpointDoesNotClaimTheScreenIsTheControls(t *testing.T) {
	s := runToRoute(t, pointable())

	lines := s.Grounded()
	if len(lines) != 2 {
		t.Fatalf("%d endpoint line(s), want two", len(lines))
	}
	for _, l := range lines {
		if !strings.Contains(l.Say, "recognise this screen by") {
			t.Errorf("%s reads %q; the highlight is what the screen is recognised BY",
				l.Label, l.Say)
		}
		if !strings.Contains(l.Say, "This is what I mean") {
			t.Errorf("%s reads %q; a highlight means \"this is what I mean\"", l.Label, l.Say)
		}
		assertNoBackstageLeak(t, l.Say)
	}
}

// runToRoute drives one teach session as far as a discovered route, with the given grounding.
//
// The SAME script every time, so any difference between the returned sessions is attributable to
// grounding and to nothing else.
func runToRoute(t *testing.T, g teach.Grounding) teach.Session {
	t.Helper()
	m := newMemory()
	route := observe.RelationshipRef{From: startSubject, To: endSubject}

	// The discovery pass ends somewhere OTHER than where it began, which is the only shape in
	// which "which screen was this grounded on" is a question with a wrong answer.
	discovery := placedResult("observe_2")
	discovery.Stats.Shadow.CurrentState = "state_2"
	discovery.Stats.Shadow.States = append(discovery.Stats.Shadow.States,
		observe.ScreenState{ID: "state_2", Episodes: 2})

	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery},
		onPass: func(n int) {
			// The discovery pass makes the edge durable, exactly as the runner's own
			// end-of-session write does.
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := teach.New("open downloads", p, m,
		teach.Bounds{Dwell: time.Second, Watch: 5 * time.Second})
	if g != nil {
		c = c.WithGrounding(g)
	}
	c.Advance(context.Background()) // establish the start
	c.Advance(context.Background()) // discover the route
	return c.Session()
}
