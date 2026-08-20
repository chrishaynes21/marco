package orchestrator_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Every key went out. That is not the same as the play having worked.
//
// Roadmap 28 proved a play refuses to BEGIN on the wrong screen. This proves it refuses to
// SUCCEED on the wrong screen — and that the two failures are different in kind:
//
//	wrong start        zero effects, and the destination is never even asked about
//	wrong destination  the effects have already happened, and the play still fails
//
// A host can send `down` and `confirm` perfectly while a dialog eats them or the menu had one
// more row than the demonstration did. A play that reported success there would be lying about
// the only thing anybody cares about. See [[ADR-032-a-play-says-where-it-ends]].

// travellingStage is a world whose current screen changes when the effects arrive.
//
// The ONLY thing that moves it is the recording host seeing the last navigation of the route, so
// the ordering is structural rather than timed: if the destination were checked before the
// effects, it would see the source and fail. No sleeps, no clocks, no barriers to get wrong.
type travellingStage struct {
	app string
	// names is the user's vocabulary for this application.
	names map[string]string
	// before and after are the durable subjects in front, either side of the route.
	before, after string
	// arrived is flipped by the OS host when the final navigation is emitted.
	arrived *bool
	// outcomeBefore and outcomeAfter are how recognition goes at each point.
	outcomeBefore, outcomeAfter screenhost.Outcome
	// looks counts recognitions, so "it reused the first answer" is detectable.
	looks *int
}

func (s *travellingStage) Application() string { return s.app }

func (s *travellingStage) SubjectNamed(application, name string) (string, bool) {
	if !strings.EqualFold(application, "testgame") {
		return "", false
	}
	id, ok := s.names[strings.ToLower(name)]
	return id, ok
}

// CurrentSubject answers about NOW, and now depends on whether the route has run.
func (s *travellingStage) CurrentSubject(string) (string, screenhost.Outcome) {
	*s.looks++
	if *s.arrived {
		return s.after, s.outcomeAfter
	}
	return s.before, s.outcomeBefore
}

// movingHost is the recording host that also moves the world.
//
// The world changes because the EFFECTS happened, which is the only honest way to write this
// test: a fixture that flipped the screen on a timer would pass whether or not the play checked
// the destination after acting.
type movingHost struct {
	calls   []string
	arrived *bool
	// final is the navigation that completes the route.
	final string
}

func (h *movingHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	call := c.Act + "'s " + c.Action
	if in := c.Input.AsText(); in != "" {
		call += " with " + in
	}
	h.calls = append(h.calls, call)
	if strings.EqualFold(c.Action, "navigate") && c.Input.AsText() == h.final {
		*h.arrived = true
	}
	return "ok", runtime.Absent(), nil
}

func (h *movingHost) pressed() []string {
	var out []string
	for _, c := range h.calls {
		if strings.HasPrefix(c, "OS's") {
			out = append(out, c)
		}
	}
	return out
}

// journey saves the real generated play and returns everything needed to run it.
func journey(t *testing.T, sc *travellingStage) (orchestrator.Deps, *movingHost, *strings.Builder) {
	t.Helper()
	// The PRODUCTION generator, both claims and all. Not a hand-written fixture.
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute",
		"the pause menu", "controller settings", [][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	for _, want := range []string{
		`do Screen's Showing with "the pause menu"...`,
		`do Screen's Showing with "controller settings"...`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("the generated play does not carry %q:\n%s", want, src)
		}
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if err := reg.SaveWithOrigin(rt, src, routes.Origin{
		Kind: routes.KindLearned, Application: "testgame",
		From: "subj_a", To: "subj_b", Sequence: 1, Evidence: "e1",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}

	host := &movingHost{arrived: sc.arrived, final: "confirm"}
	// THE outcome surface. `d.Do` returns nil for a play that ran and failed — "you said no"
	// and "something went wrong" are different things and only one of them is an exit code.
	// What a PERSON sees is the play's own sentence, so that is what these tests read.
	out := &strings.Builder{}
	d := orchestrator.Deps{
		Reg:   reg,
		Hosts: map[string]runtime.Host{"*": host, "Screen": screenhost.New(sc)},
		In:    strings.NewReader("y\n"),
		Out:   out,
		App:   func() string { return "testgame" },
	}
	d.Authority = orchestrator.AskFirst{Deps: d}
	return d, host, out
}

// succeeded reports whether the play said it finished where it meant to.
//
// The generated script logs `done` on ok and the play's own error otherwise, so this is the
// ordinary Marco outcome rather than anything this milestone invented.
func succeeded(out *strings.Builder) bool { return strings.Contains(out.String(), "done") }

// stageWith builds a world that starts on the pause menu and ends wherever the test says.
func stageWith(after string, out screenhost.Outcome) (*travellingStage, *bool, *int) {
	arrived, looks := false, 0
	return &travellingStage{
		app: "testgame",
		names: map[string]string{
			"the pause menu": "subj_a", "controller settings": "subj_b",
		},
		before: "subj_a", after: after,
		arrived: &arrived, looks: &looks,
		outcomeBefore: screenhost.Recognised, outcomeAfter: out,
	}, &arrived, &looks
}

// ── the headline: arriving, and not arriving ──────────────────────────────────

// The route runs and the play succeeds, because it ended where it said it would.
func TestAPlayThatArrivesWhereItSaidSucceeds(t *testing.T) {
	sc, _, looks := stageWith("subj_b", screenhost.Recognised)
	d, host, out := journey(t, sc)

	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if !succeeded(out) {
		t.Fatalf("the play did not report success: %q", out.String())
	}
	want := []string{`OS's Navigate with down`, `OS's Navigate with confirm`}
	if got := host.pressed(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("the host was asked for %v, want %v", got, want)
	}
	// TWO recognitions: once before the effects, once after. One would mean the answer was
	// reused, and reusing it is the whole failure this milestone exists to prevent.
	if *looks < 2 {
		t.Fatalf("the world was looked at %d time(s); the destination must be observed "+
			"fresh, after the effects", *looks)
	}
}

// Every key went out, the application landed somewhere else, and the play FAILS.
//
// THE headline. Note what is different from a wrong start: the effects already happened. Marco
// does not undo them, does not retry, and does not send anything to recover — it reports that
// this invocation did not end where the play said it would.
func TestAPlayThatDoesNotArriveFails(t *testing.T) {
	sc, _, _ := stageWith("subj_x", screenhost.Recognised) // somewhere else entirely
	d, host, out := journey(t, sc)

	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}

	// The effects DID happen. This is not a wrong-start case.
	want := []string{`OS's Navigate with down`, `OS's Navigate with confirm`}
	if got := host.pressed(); len(got) != len(want) {
		t.Fatalf("the route did not run: %v", got)
	}
	// And the play still failed.
	if succeeded(out) {
		t.Fatal("the play reported success from somewhere it did not expect to be; " +
			"emitting every key is not the same as having arrived")
	}
	// In words a person can act on, naming no internals.
	if !strings.Contains(out.String(), "expected to finish on controller settings") {
		t.Errorf("the failure does not say what it expected: %q", out.String())
	}
	for _, jargon := range []string{"subj_", "fingerprint", "postcondition", "predicate",
		"rehearsal", "digest"} {
		if strings.Contains(out.String(), jargon) {
			t.Errorf("the failure shows %q to the user: %q", jargon, out.String())
		}
	}
	// NOTHING was sent afterwards. No retry, no recovery, no walking back.
	if got := host.pressed(); len(got) != len(want) {
		t.Fatalf("input was sent after the destination check failed: %v", got)
	}
}

// ── every other way of not knowing where it ended up ──────────────────────────

// Silence is never yes, after the effects just as much as before them.
func TestOnlyAPositiveArrivalIsSuccess(t *testing.T) {
	for _, tc := range []struct {
		name    string
		after   string
		outcome screenhost.Outcome
	}{
		{"a different screen", "subj_x", screenhost.Recognised},
		{"an ambiguous screen", "", screenhost.Ambiguous},
		{"a screen nobody can see", "", screenhost.Unobservable},
		{"the recogniser has gone", "", screenhost.Unavailable},
		{"nothing memory knows", "", screenhost.Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, _, _ := stageWith(tc.after, tc.outcome)
			d, host, out := journey(t, sc)

			if err := d.Do("volume"); err != nil {
				t.Fatalf("do: %v", err)
			}
			if len(host.pressed()) != 2 {
				t.Fatalf("the route did not run: %v", host.pressed())
			}
			if succeeded(out) {
				t.Fatalf("%s was treated as arriving; the effect host succeeding is not "+
					"the application having gone anywhere", tc.name)
			}
		})
	}
}

// A destination name that exists only in ANOTHER application is not this play's destination.
func TestADestinationNamedInAnotherApplicationDoesNotCount(t *testing.T) {
	sc, _, _ := stageWith("subj_b", screenhost.Recognised)
	// The name resolves nowhere in the application that is in front.
	delete(sc.names, "controller settings")
	d, host, out := journey(t, sc)

	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if succeeded(out) {
		t.Fatal("a play succeeded on a destination this application has no name for")
	}
	if len(host.pressed()) != 2 {
		t.Fatalf("the route did not run: %v", host.pressed())
	}
}

// ── the two failures are different in kind ────────────────────────────────────

// A wrong START still sends nothing, and never reaches the destination check.
//
// Roadmap 28's guarantee, re-proved against the postconditioned play: adding a claim about the
// end must not weaken the claim about the beginning.
func TestAWrongStartNeverReachesTheDestinationCheck(t *testing.T) {
	sc, arrived, looks := stageWith("subj_b", screenhost.Recognised)
	sc.before = "subj_x" // the wrong screen
	d, host, out := journey(t, sc)

	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if succeeded(out) {
		t.Fatal("a play on the wrong screen reported success")
	}
	if got := host.pressed(); len(got) != 0 {
		t.Fatalf("a key was pressed on the wrong screen: %v", got)
	}
	if *arrived {
		t.Fatal("the route ran")
	}
	// Looked at ONCE. The destination question is never even asked, because there is no path
	// from a failed entry condition to the body.
	if *looks != 1 {
		t.Errorf("the world was looked at %d times; a refused start asks one question", *looks)
	}
}

// ── the user owns the file ────────────────────────────────────────────────────

// Removing the destination check removes the destination check.
//
// Provenance records what Director wrote. It enforces nothing — a user who deletes the final
// condition owns a play that no longer makes that claim, and Marco stops saying it verified the
// file. Exactly the Roadmap 25 policy, applied to the new half.
func TestRemovingTheDestinationCheckRemovesIt(t *testing.T) {
	sc, _, _ := stageWith("subj_x", screenhost.Recognised) // would FAIL the postcondition
	d, host, out := journey(t, sc)

	// The user edits the final condition out, keeping the entry guard.
	unguarded, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if strings.Contains(unguarded, "controller settings") {
		t.Fatal("the fixture still names the destination")
	}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if err := d.Reg.Save(rt, unguarded); err != nil {
		t.Fatalf("saving: %v", err)
	}

	if err := d.Do("volume"); err != nil {
		t.Fatalf("do: %v", err)
	}
	if !succeeded(out) {
		t.Fatalf("the sidecar went on enforcing a condition the source no longer states: %q",
			out.String())
	}
	if len(host.pressed()) != 2 {
		t.Fatalf("the route did not run: %v", host.pressed())
	}
	// And Marco stops claiming it verified this file.
	if _, state := d.Reg.Origin(rt); state.Verified() {
		t.Error("an edited play still claims to be the artifact Director verified")
	}
}
