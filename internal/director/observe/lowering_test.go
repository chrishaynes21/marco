package observe_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What earns the right to be written down.
//
// The gate is the DERIVED verification — an assessment folded with stored rehearsal evidence —
// and not `ProcedureCandidate.Verified`, which is always false and deliberately vestigial. A
// candidate Marco has only watched, however consistent it looked, is not a procedure it knows
// works.

// ── fixtures ──────────────────────────────────────────────────────────────────

// lowerTopology remembers the given subjects, with the start screen named.
//
// The name matters: a route whose starting screen nobody has named cannot be written down, because
// `Screen's Showing with "…"` is executable meaning rather than a label.
func lowerTopology(ids ...string) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	for _, id := range ids {
		r := observe.RememberedSubject{ID: id}
		switch id {
		case "subj_a":
			r.Called = "the pause menu"
		case "subj_b":
			// The DESTINATION needs a name too: a play says where it finishes as well as
			// where it starts, and both are executable meaning.
			r.Called = "controller settings"
		}
		top.Subjects[id] = r
	}
	return top
}

// twoStep is A → B: one confirm, then down-down-confirm.
func twoStep() observe.ProcedureCandidate {
	return observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
		Application:  "testgame", Sequence: 1,
		Start: observe.Checkpoint{Subject: "subj_a", Verdict: observe.MatchSame},
		Steps: []observe.DemonstrationStep{{
			Intents: []observe.NavIntent{observe.NavConfirm},
			Arrived: observe.Checkpoint{Subject: "subj_x", Verdict: observe.MatchSame},
		}, {
			Intents: []observe.NavIntent{observe.NavDown, observe.NavDown, observe.NavConfirm},
			Arrived: observe.Checkpoint{Subject: "subj_b", Verdict: observe.MatchSame},
		}},
		Complete: true, Reason: observe.ReasonArrived, Checkpoints: 3, Events: 4,
	}
}

// rehearsed is the assessment as it stands after a completed attempt, folded.
func rehearsed(t *testing.T, c observe.ProcedureCandidate, top observe.Topology,
	digest string) observe.CandidateAssessment {

	t.Helper()
	a := observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
		observe.Corroboration{Compared: true, Agreement: observe.AgreementSame})
	return a.WithRehearsal(c, digest, top, []observe.RehearsalEvidence{{
		Application: "testgame", Relationship: c.Relationship, Sequence: c.Sequence,
		Evidence: digest, Source: "subj_a", Destination: "subj_b", Completed: true,
	}})
}

func watchedOnly(c observe.ProcedureCandidate, top observe.Topology) observe.CandidateAssessment {
	return observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
		observe.Corroboration{Compared: true, Agreement: observe.AgreementSame})
}

// ── the one case that lowers ──────────────────────────────────────────────────

// A verified procedure hands over its meanings, in order, with repeats intact.
func TestAVerifiedProcedureMayBeWrittenDown(t *testing.T) {
	c := twoStep()
	top := lowerTopology("subj_a", "subj_b", "subj_x")
	j := observe.JudgeLowering(c, rehearsed(t, c, top, "d1"), top, "testgame")

	if !j.Eligible {
		t.Fatalf("refusals %v", j.Refusals)
	}
	// A step is a run of ACTIONS now, because a play can press a control as well as send
	// a meaning. These are all meanings; the click case is in targetsurvival_test.go.
	want := [][]observe.LoweredAction{
		{{Intent: observe.NavConfirm}},
		{{Intent: observe.NavDown}, {Intent: observe.NavDown}, {Intent: observe.NavConfirm}},
	}
	if !reflect.DeepEqual(j.Steps, want) {
		t.Fatalf("steps = %v, want %v", j.Steps, want)
	}
	if j.Presses() != 4 {
		t.Errorf("presses = %d", j.Presses())
	}
	// And nothing about HOW Director came to believe it came along for the ride.
	rt := reflect.TypeOf(observe.LoweringJudgement{})
	for _, forbidden := range []string{"Digest", "Evidence", "Checkpoint", "Subject",
		"Verdict", "Window", "Session", "Confidence", "Track", "State"} {
		if _, ok := rt.FieldByName(forbidden); ok {
			t.Errorf("LoweringJudgement carries %s into the generator; a field that exists "+
				"gets printed", forbidden)
		}
	}
}

// ── the refusal matrix ────────────────────────────────────────────────────────

func TestTheLoweringRefusalMatrix(t *testing.T) {
	full := lowerTopology("subj_a", "subj_b", "subj_x")

	cases := []struct {
		name string
		// build returns the candidate, the assessment and the topology to judge against.
		build func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology)
		app  string
		want observe.LoweringRefusal
	}{{
		// WATCHED AND NEVER TRIED IS NO LONGER A REFUSAL — see
		// [[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] and
		// CandidateAssessment.CleanlyObserved. What still refuses is an attempt that
		// did not arrive, which is the case below and the newer evidence.
		name: "a demonstration whose navigation says nothing",
		build: func(*testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			c.Steps = nil
			return c, watchedOnly(c, full), full
		},
		app: "testgame", want: observe.RefusalNothingToDo,
	}, {
		name: "a rehearsal that did not complete",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			a := watchedOnly(c, full).WithRehearsal(c, "d1", full,
				[]observe.RehearsalEvidence{{
					Application: "testgame", Relationship: c.Relationship, Sequence: 1,
					Evidence: "d1", Source: "subj_a", Destination: "subj_b",
					Completed: false,
				}})
			return c, a, full
		},
		app: "testgame", want: observe.RefusalNotVerified,
	}, {
		name: "a rehearsal of a demonstration that has since been revised",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			// The evidence says d1; the candidate now digests to d2.
			a := watchedOnly(c, full).WithRehearsal(c, "d2", full,
				[]observe.RehearsalEvidence{{
					Application: "testgame", Relationship: c.Relationship, Sequence: 1,
					Evidence: "d1", Source: "subj_a", Destination: "subj_b",
					Completed: true,
				}})
			return c, a, full
		},
		app: "testgame", want: observe.RefusalNotVerified,
	}, {
		// A REHEARSAL OF ANOTHER DEMONSTRATION DOES NOT LET THE ROUTE THROUGH.
		//
		// Verification is per LINEAGE — candidate 1 having been rehearsed says nothing
		// about candidate 2's evidence — and after
		// [[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] that alone
		// would no longer refuse anything, because candidate 2 is cleanly observed and a
		// cleanly observed route may now be written down.
		//
		// So what refuses is that a rehearsal of this ROUTE is on record and has not
		// produced a verification. Something about the route stopped adding up, and the
		// other demonstration of it must not quietly lower instead. See
		// CandidateAssessment.Rehearsed.
		name: "a rehearsal of another demonstration of the same route",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			a := watchedOnly(c, full).WithRehearsal(c, "d1", full,
				[]observe.RehearsalEvidence{{
					Application: "testgame", Relationship: c.Relationship, Sequence: 2,
					Evidence: "d1", Source: "subj_a", Destination: "subj_b",
					Completed: true,
				}})
			if a.Verified {
				t.Fatal("a rehearsal of another lineage marked this one verified")
			}
			return c, a, full
		},
		app: "testgame", want: observe.RefusalNotVerified,
	}, {
		name: "a screen memory no longer holds",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			gone := lowerTopology("subj_a", "subj_x")
			return c, rehearsed(t, c, full, "d1"), gone
		},
		app: "testgame", want: observe.RefusalEndpointUnknown,
	}, {
		name: "another application",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			return c, rehearsed(t, c, full, "d1"), full
		},
		app: "someothergame", want: observe.RefusalWrongApplication,
	}, {
		name: "a screen the user typed on",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			c.Steps[0].RequiresTextEntry = true
			return c, rehearsed(t, c, full, "d1"), full
		},
		app: "testgame", want: observe.RefusalCannotSayText,
	}, {
		name: "a click with nothing behind it",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			c.Steps[0].Intents = []observe.NavIntent{observe.NavPoint}
			return c, rehearsed(t, c, full, "d1"), full
		},
		app: "testgame", want: observe.RefusalNoTargetToName,
	}, {
		name: "a meaning Marco has no word for",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			c.Steps[0].Intents = []observe.NavIntent{"pirouette"}
			return c, rehearsed(t, c, full, "d1"), full
		},
		app: "testgame", want: observe.RefusalInexpressible,
	}, {
		name: "nothing to do",
		build: func(t *testing.T) (observe.ProcedureCandidate, observe.CandidateAssessment,
			observe.Topology) {
			c := twoStep()
			c.Steps = nil
			return c, rehearsed(t, c, full, "d1"), full
		},
		app: "testgame", want: observe.RefusalNothingToDo,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, a, top := tc.build(t)
			j := observe.JudgeLowering(c, a, top, tc.app)
			if j.Eligible {
				t.Fatalf("%s was written down", tc.name)
			}
			var found bool
			for _, r := range j.Refusals {
				if r == tc.want {
					found = true
				}
			}
			if !found {
				t.Errorf("refusals %v do not include %q", j.Refusals, tc.want)
			}
			// THE invariant every row shares: a refused judgement hands over nothing to
			// write. A generator cannot print half a procedure it was never given.
			if len(j.Steps) != 0 {
				t.Fatalf("a refused judgement handed over %v", j.Steps)
			}
		})
	}
}

// ── the starting screen must have a name somebody gave it ─────────────────────

// A route whose starting screen nobody has named cannot be written down.
//
// Not a cosmetic gap. `do Screen's Showing with "…"` is the play's first executable sentence, and
// the string in it is what the host resolves against durable memory. There is no name Director may
// invent that would make that sentence true — so the absence of one is a refusal, in words.
func TestAnUnnamedStartingScreenIsNotWrittenDown(t *testing.T) {
	c := twoStep()
	// Everything else is in place: remembered, rehearsed, corroborated. Only the name is
	// missing, so nothing but the name can be what refuses this.
	unnamed := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a"}, "subj_b": {ID: "subj_b"}, "subj_x": {ID: "subj_x"},
	}}
	j := observe.JudgeLowering(c, rehearsed(t, c, unnamed, "d1"), unnamed, "testgame")

	if j.Eligible {
		t.Fatal("a route starting on a screen nobody has named was written down")
	}
	var found bool
	for _, r := range j.Refusals {
		if r == observe.RefusalScreenUnnamed {
			found = true
		}
	}
	if !found {
		t.Errorf("refusals %v do not say the screen has no name", j.Refusals)
	}
	if len(j.Steps) != 0 {
		t.Fatalf("a refused judgement handed over %v", j.Steps)
	}
	if j.StartsOn != "" {
		t.Errorf("it produced an entry condition anyway: %q", j.StartsOn)
	}
	// And naming it is the whole difference.
	named := lowerTopology("subj_a", "subj_b", "subj_x")
	if !observe.JudgeLowering(c, rehearsed(t, c, named, "d1"), named, "testgame").Eligible {
		t.Fatal("naming the screen did not make the route lowerable")
	}
}

// The entry condition is the name the USER gave, read from memory at judgement time.
//
// The adversarial case is a generator that hardcodes a plausible name, or a caller that supplies
// one. Neither can pass this: the expectation is a name chosen here and stored in the topology,
// so the only way to produce it is to read it back.
func TestTheEntryConditionIsTheNameTheUserGave(t *testing.T) {
	const chosen = "the advanced options screen"
	c := twoStep()
	const arriving = "the audio page"
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a", Called: chosen},
		"subj_b": {ID: "subj_b", Called: arriving}, "subj_x": {ID: "subj_x"},
	}}
	j := observe.JudgeLowering(c, rehearsed(t, c, top, "d1"), top, "testgame")
	if !j.Eligible {
		t.Fatalf("refusals %v", j.Refusals)
	}
	if j.StartsOn.String() != chosen {
		t.Fatalf("the play would begin on %q, not on the screen the user named (%q)",
			j.StartsOn, chosen)
	}
	// And the DESTINATION is the verified relationship's own end, named by the user. The two
	// do not swap, and neither is invented.
	if j.EndsOn.String() != arriving {
		t.Fatalf("the play would expect to finish on %q, not on %q", j.EndsOn, arriving)
	}
	// It is the SOURCE screen's name for the entry condition and the DESTINATION's for the
	// postcondition. Swapping the two remembered names must swap both claims, so neither can
	// be a hardcoded string that happens to match.
	swapped := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a", Called: arriving},
		"subj_b": {ID: "subj_b", Called: chosen}, "subj_x": {ID: "subj_x"},
	}}
	k := observe.JudgeLowering(c, rehearsed(t, c, swapped, "d1"), swapped, "testgame")
	if k.StartsOn.String() != arriving || k.EndsOn.String() != chosen {
		t.Fatalf("the claims did not follow the screens: starts on %q, ends on %q",
			k.StartsOn, k.EndsOn)
	}
}

// A route whose destination nobody has named cannot be written down either.
//
// The same rule as the source, for the same reason: `do Screen's Showing with "…"` is executable
// meaning wherever it appears. A play that cannot say where it expects to finish cannot tell the
// difference between working and having sent some keys.
func TestAnUnnamedDestinationIsNotWrittenDown(t *testing.T) {
	c := twoStep()
	// The source is named; only the destination is missing, so nothing else can be what
	// refuses this.
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a", Called: "the pause menu"},
		"subj_b": {ID: "subj_b"}, "subj_x": {ID: "subj_x"},
	}}
	j := observe.JudgeLowering(c, rehearsed(t, c, top, "d1"), top, "testgame")

	if j.Eligible {
		t.Fatal("a route with an unnamed destination was written down")
	}
	var found bool
	for _, r := range j.Refusals {
		if r == observe.RefusalScreenUnnamed {
			found = true
		}
	}
	if !found {
		t.Errorf("refusals %v do not say a screen has no name", j.Refusals)
	}
	if j.EndsOn != "" {
		t.Errorf("it produced a postcondition anyway: %q", j.EndsOn)
	}
	// And it says WHICH screen still needs one — the destination, not the source, which is
	// already named. That list is what the naming question is raised from.
	if len(j.Unnamed) != 1 || j.Unnamed[0] != "subj_b" {
		t.Fatalf("the judgement asks about %v, not the destination", j.Unnamed)
	}
}

// With both ends unnamed, the source is asked about first.
//
// Deterministic priority, and it is the readable order: the first line of the play is the one
// that makes the second question make sense. There is no queue — naming the source and asking
// the judgement again is what surfaces the destination.
func TestTheSourceIsNamedBeforeTheDestination(t *testing.T) {
	c := twoStep()
	none := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a"}, "subj_b": {ID: "subj_b"}, "subj_x": {ID: "subj_x"},
	}}
	j := observe.JudgeLowering(c, rehearsed(t, c, none, "d1"), none, "testgame")
	if len(j.Unnamed) != 2 || j.Unnamed[0] != "subj_a" || j.Unnamed[1] != "subj_b" {
		t.Fatalf("the order to ask in is %v, want source then destination", j.Unnamed)
	}

	// Name the source; the judgement now asks about the destination and nothing else.
	named := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a", Called: "the pause menu"},
		"subj_b": {ID: "subj_b"}, "subj_x": {ID: "subj_x"},
	}}
	k := observe.JudgeLowering(c, rehearsed(t, c, named, "d1"), named, "testgame")
	if len(k.Unnamed) != 1 || k.Unnamed[0] != "subj_b" {
		t.Fatalf("after naming the source the judgement asks about %v", k.Unnamed)
	}
}

// A judgement is recomputed, so eligibility stops being true when the world moves.
func TestLoweringEligibilityIsRecomputedNotRemembered(t *testing.T) {
	c := twoStep()
	top := lowerTopology("subj_a", "subj_b", "subj_x")
	if !observe.JudgeLowering(c, rehearsed(t, c, top, "d1"), top, "testgame").Eligible {
		t.Fatal("the fixture is not eligible")
	}
	// The destination is forgotten. The rehearsal still happened; it is now a fact about a
	// screen nobody can find.
	forgotten := lowerTopology("subj_a", "subj_x")
	if observe.JudgeLowering(c, rehearsed(t, c, forgotten, "d1"),
		forgotten, "testgame").Eligible {
		t.Fatal("a route whose destination memory no longer holds was still lowerable")
	}
}

// The refusals a person reads are the closed vocabulary, and nothing else.
func TestLoweringSaysWhyInWordsNotInScores(t *testing.T) {
	c := twoStep()
	top := lowerTopology("subj_a", "subj_b", "subj_x")
	// A route with a rehearsal on record that did not verify. A merely-watched one is no
	// longer refused at all — see the matrix case above and
	// [[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] — so a fixture
	// built on one would be asserting the wording of a refusal that never happens.
	a := watchedOnly(c, top).WithRehearsal(c, "d1", top, []observe.RehearsalEvidence{{
		Application: "testgame", Relationship: c.Relationship, Sequence: 2,
		Evidence: "d1", Source: "subj_a", Destination: "subj_b", Completed: true,
	}})
	j := observe.JudgeLowering(c, a, top, "testgame")
	lines := strings.Join(observe.DescribeLowering(j), "\n")
	if !strings.Contains(lines, "not_verified") {
		t.Errorf("the description does not say why:\n%s", lines)
	}
	for _, bad := range []string{"0.", "%", "score", "confidence"} {
		if strings.Contains(lines, bad) {
			t.Errorf("the description reaches for %q:\n%s", bad, lines)
		}
	}
}

// A SCREEN MARCO NAMED ITSELF DOES NOT HAVE TO BE NAMED AGAIN.
//
// # The live failure
//
// Both screens of a demonstrated route carried correct inferred names — `Home` and
// `Bluetooth & devices`, grounded in Actor evidence, with no Audience name at all. Lowering read
// `Called`, found it empty, and refused `screen_unnamed`. Marco asked the Audience to name a
// screen it had already named, and the panel offered no way to decline.
//
// `Called` still wins and the record still keeps the two apart. But a play refers to a screen by a
// word a reader understands, and that word does not have to have come from a person.
//
// Deleting the Name() call in JudgeLowering must fail this.
func TestAScreenMarcoNamedItselfNeedsNoAudienceName(t *testing.T) {
	c := twoStep()
	// Nobody has typed anything. Marco worked both names out.
	inferred := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a", Semantic: "Home"},
		"subj_b": {ID: "subj_b", Semantic: "Bluetooth & devices"},
		"subj_x": {ID: "subj_x"},
	}}
	j := observe.JudgeLowering(c, rehearsed(t, c, inferred, "d1"), inferred, "testgame")

	for _, r := range j.Refusals {
		if r == observe.RefusalScreenUnnamed {
			t.Fatalf("a route between two screens Marco had already named was refused %q. "+
				"The Audience is asked to supply a word Marco has, and cannot decline.", r)
		}
	}
	if got := string(j.StartsOn); got != "Home" {
		t.Errorf("the play starts on %q, want Home", got)
	}
	if got := string(j.EndsOn); got != "Bluetooth & devices" {
		t.Errorf("the play ends on %q, want Bluetooth & devices", got)
	}
	// And the Audience's word still wins where there is one.
	authored := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a", Called: "Start", Semantic: "Home"},
		"subj_b": {ID: "subj_b", Semantic: "Bluetooth & devices"},
		"subj_x": {ID: "subj_x"},
	}}
	k := observe.JudgeLowering(c, rehearsed(t, c, authored, "d1"), authored, "testgame")
	if got := string(k.StartsOn); got != "Start" {
		t.Errorf("the play starts on %q; the Audience said Start", got)
	}
}
