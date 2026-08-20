package observesession_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// What Marco concludes from one watched demonstration, and what it refuses to conclude.
//
// The headline test enters through the production path — a real approved request, a real capture,
// a real session — and never calls `AssessCandidate` itself. The adversarial cases below build
// candidates directly, because what is under test there is the JUDGEMENT rather than the wiring,
// and scripting a session per case would test the sampler twelve times over.

// ── helpers ───────────────────────────────────────────────────────────────────

func reasonsOf(a observe.CandidateAssessment) map[observe.AssessmentReason]bool {
	out := map[observe.AssessmentReason]bool{}
	for _, r := range a.Reasons {
		out[r] = true
	}
	return out
}

// knownTopology is a memory that recognises the named subjects and nothing else.
func knownTopology(ids ...string) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	for _, id := range ids {
		top.Subjects[id] = observe.RememberedSubject{ID: id}
	}
	return top
}

func checkpoint(subject string) observe.Checkpoint {
	return observe.Checkpoint{Subject: subject, Verdict: observe.MatchSame}
}

func transient(terms ...observe.InterfaceTerm) observe.Checkpoint {
	return observe.Checkpoint{
		Transient: true, Verdict: observe.MatchDifferent,
		Structure: observe.StructureSignature{
			Subject: observe.SubjectState, Roles: map[string]int{"button": 4},
			Terms: terms, TermsKnown: true,
		},
	}
}

func step(arrived observe.Checkpoint, intents ...observe.NavIntent) observe.DemonstrationStep {
	return observe.DemonstrationStep{Intents: intents, Arrived: arrived}
}

func candidate(steps ...observe.DemonstrationStep) observe.ProcedureCandidate {
	c := observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
		Application:  "testgame",
		Start:        checkpoint("subj_a"),
		Steps:        steps,
		Complete:     true, Reason: observe.ReasonArrived,
		Checkpoints: len(steps) + 1,
	}
	for _, s := range steps {
		c.Events += len(s.Intents)
	}
	return c
}

// cleanCandidate is A → (durable X) → B, reached deliberately.
func cleanCandidate() observe.ProcedureCandidate {
	return candidate(
		step(checkpoint("subj_x"), observe.NavDown, observe.NavDown, observe.NavConfirm),
		step(checkpoint("subj_b"), observe.NavConfirm),
	)
}

func assess(c observe.ProcedureCandidate, top observe.Topology) observe.CandidateAssessment {
	return observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(), observe.Corroboration{})
}

// ── PART 23: the production path ──────────────────────────────────────────────

// THE wiring test: a demonstration captured through the real path is assessed on it.
//
// Nothing here calls AssessCandidate. Deleting the assessment call from Run must fail this.
func TestACompletedDemonstrationIsAssessedOnTheProductionPath(t *testing.T) {
	dir := t.TempDir()
	store, from, to := approvedRun(t, dir)

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration == nil || !res.Demonstration.Complete {
		t.Fatalf("the demonstration did not complete: %+v", res.Demonstration)
	}
	if res.Assessment == nil {
		t.Fatal("a completed demonstration was not judged; Marco watched an example and " +
			"concluded nothing from it")
	}
	a := *res.Assessment
	if a.Relationship.From != from || a.Relationship.To != to {
		t.Errorf("the assessment names %+v, not the demonstrated edge", a.Relationship)
	}
	// The fixture's intermediate screen is one memory does not recognise, so the honest
	// verdict is that something cannot be checked.
	if a.Verdict != observe.CandidateInsufficient {
		t.Errorf("verdict = %q; the demonstration crosses a screen Marco cannot recognise "+
			"and the verdict should say so. Reasons: %v", a.Verdict, a.Reasons)
	}
	if !reasonsOf(a)[observe.ReasonTransientCheckpoint] {
		t.Errorf("reasons %v do not name the unrecognisable checkpoint", a.Reasons)
	}
	if !reasonsOf(a)[observe.ReasonSingleDemonstration] {
		t.Error("the assessment does not record that this rests on one example")
	}
	// Coverage is a LIST, so a reader learns which part is the problem.
	if len(a.Checkpoints) != len(res.Demonstration.Steps)+1 {
		t.Fatalf("%d checkpoint verdict(s) for %d checkpoint(s)",
			len(a.Checkpoints), len(res.Demonstration.Steps)+1)
	}
	if !a.Checkpoints[0].Verifiable {
		t.Error("the start is a remembered subject and was not marked verifiable")
	}
	if a.Checkpoints[1].Verifiable {
		t.Error("the unrecognisable intermediate screen was marked verifiable")
	}
	if !a.Checkpoints[len(a.Checkpoints)-1].Verifiable {
		t.Error("the destination is a remembered subject and was not marked verifiable")
	}
	// AUTHORITY. The strongest possible outcome here still grants nothing.
	if a.Verified || res.Demonstration.Verified {
		t.Fatal("an assessment marked a watched example as verified")
	}
	joined := strings.Join(observe.DescribeAssessment(a), "\n")
	if !strings.Contains(joined, "verified: no") {
		t.Errorf("the description does not disclaim verification:\n%s", joined)
	}
	for _, claim := range []string{"I know how", "I can do", "learned the procedure"} {
		if strings.Contains(joined, claim) {
			t.Errorf("the description claims %q:\n%s", claim, joined)
		}
	}
}

// ── PART 24 case 12: reassessment as memory improves ──────────────────────────

// The same demonstration is judged better once Marco learns what its middle screen is.
//
// THE reason a verdict is never written into the candidate. The observation is fixed; what Marco
// can make of it is not, and a stored judgement would still say "unverifiable" long after the
// user had answered the question that resolved it.
func TestAnAssessmentImprovesWhenMemoryDoes(t *testing.T) {
	c := candidate(
		step(transient(observe.TermHelp), observe.NavConfirm),
		step(checkpoint("subj_b"), observe.NavConfirm),
	)

	before := assess(c, knownTopology("subj_a", "subj_b"))
	if before.Verdict != observe.CandidateInsufficient ||
		!reasonsOf(before)[observe.ReasonTransientCheckpoint] {
		t.Fatalf("the transient checkpoint was not reported: %+v", before)
	}

	// The user tells Marco what that screen is, so the demonstration now records a subject.
	improved := c
	improved.Steps = append([]observe.DemonstrationStep{}, c.Steps...)
	improved.Steps[0].Arrived = checkpoint("subj_x")
	after := assess(improved, knownTopology("subj_a", "subj_b", "subj_x"))

	if after.Verdict != observe.CandidateConsistent {
		t.Fatalf("the same demonstration was not judged better once its middle screen "+
			"became recognisable: %+v", after)
	}
	if reasonsOf(after)[observe.ReasonTransientCheckpoint] {
		t.Error("the resolved checkpoint is still reported as unverifiable")
	}
	// And still not verified. Better evidence is not experimental reproduction.
	if after.Verified {
		t.Error("a better assessment granted verification")
	}
	if !reasonsOf(after)[observe.ReasonSingleDemonstration] {
		t.Error("the best available verdict forgot that it rests on one example")
	}
}

// ── PART 24 cases 1–7: the adversarial table ──────────────────────────────────

func TestTheAssessmentTable(t *testing.T) {
	full := knownTopology("subj_a", "subj_b", "subj_x")

	cases := []struct {
		name    string
		c       observe.ProcedureCandidate
		top     observe.Topology
		verdict observe.CandidateVerdict
		reason  observe.AssessmentReason
	}{{
		name: "clean, durable throughout", c: cleanCandidate(), top: full,
		verdict: observe.CandidateConsistent,
	}, {
		name: "an unrecognisable intermediate screen",
		c: candidate(
			step(transient(observe.TermHelp), observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, verdict: observe.CandidateInsufficient,
		reason: observe.ReasonTransientCheckpoint,
	}, {
		name: "a screen the user typed on",
		c: func() observe.ProcedureCandidate {
			c := cleanCandidate()
			c.Steps[0].RequiresTextEntry = true
			return c
		}(),
		top: full, verdict: observe.CandidateInsufficient,
		reason: observe.ReasonRequiresTextEntry,
	}, {
		name: "a pointer activation with no semantic target",
		c: candidate(
			step(checkpoint("subj_x"), observe.NavPoint),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, verdict: observe.CandidateInsufficient,
		reason: observe.ReasonUnresolvedPointer,
	}, {
		name: "a run too long to read as deliberate",
		c: candidate(
			step(checkpoint("subj_x"), observe.NavDown, observe.NavDown, observe.NavDown,
				observe.NavDown, observe.NavDown, observe.NavDown, observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, verdict: observe.CandidateAmbiguous,
		reason: observe.ReasonAmbiguousRun,
	}, {
		name: "a run that reverses itself",
		c: candidate(
			step(checkpoint("subj_x"), observe.NavDown, observe.NavUp, observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, verdict: observe.CandidateAmbiguous,
		reason: observe.ReasonBacktracking,
	}, {
		name: "a start memory no longer holds", c: cleanCandidate(),
		top:     knownTopology("subj_b", "subj_x"),
		verdict: observe.CandidateInvalid, reason: observe.ReasonStartUnverifiable,
	}, {
		name: "a destination memory no longer holds", c: cleanCandidate(),
		top:     knownTopology("subj_a", "subj_x"),
		verdict: observe.CandidateInvalid, reason: observe.ReasonEndUnverifiable,
	}, {
		name: "a demonstration that never finished",
		c: func() observe.ProcedureCandidate {
			c := cleanCandidate()
			c.Complete, c.Reason = false, observe.ReasonSessionEnded
			return c
		}(),
		top: full, verdict: observe.CandidateInvalid,
		reason: observe.ReasonIncompleteDemonstration,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := assess(tc.c, tc.top)
			if a.Verdict != tc.verdict {
				t.Errorf("verdict = %q, want %q (reasons %v)", a.Verdict, tc.verdict, a.Reasons)
			}
			if tc.reason != "" && !reasonsOf(a)[tc.reason] {
				t.Errorf("reasons %v do not include %q", a.Reasons, tc.reason)
			}
			if a.Verified {
				t.Error("an assessment granted verification")
			}
			// Every reason is in the closed vocabulary and says whether another example
			// would help — the preparation the next milestone needs.
			for _, r := range a.Reasons {
				_ = r.ResolvableByDemonstration()
				if !strings.Contains(string(r), "_") {
					t.Errorf("reason %q does not look like the closed vocabulary", r)
				}
			}
		})
	}
}

// Volume is not strength.
//
// A demonstration close to a capture bound may be missing the end of itself, and more recorded
// navigation is a reason for LESS confidence rather than more.
func TestADemonstrationNearItsBoundIsWeakerNotStronger(t *testing.T) {
	small := observe.CaptureBounds{
		MaxEvents: 8, MaxCheckpoints: 4, MaxRunLength: 8, MaxInferences: 90, MaxRestarts: 2,
	}
	c := cleanCandidate()
	c.Events = 7
	a := observe.AssessCandidate(c, knownTopology("subj_a", "subj_b", "subj_x"), small, observe.Corroboration{})
	if a.Verdict == observe.CandidateConsistent {
		t.Fatalf("a demonstration at 7 of 8 permitted events was judged fully consistent: %+v", a)
	}
	if !reasonsOf(a)[observe.ReasonNearCaptureBound] {
		t.Errorf("reasons %v do not mention the bound", a.Reasons)
	}
}

// Which gaps another demonstration could close, and which it could not.
func TestTheAssessmentSaysWhetherAnotherExampleWouldHelp(t *testing.T) {
	// A long run is somebody hunting; a cleaner example would settle it.
	hunting := assess(candidate(
		step(checkpoint("subj_x"), observe.NavDown, observe.NavDown, observe.NavDown,
			observe.NavDown, observe.NavDown, observe.NavDown, observe.NavConfirm),
		step(checkpoint("subj_b"), observe.NavConfirm)),
		knownTopology("subj_a", "subj_b", "subj_x"))
	if !hunting.NeedsAnotherDemonstration() {
		t.Error("an ambiguous run was not marked as something another example could settle")
	}

	// Text entry is not: watching it again changes neither what Marco may keep nor what it
	// could reproduce.
	typed := cleanCandidate()
	typed.Steps[0].RequiresTextEntry = true
	a := assess(typed, knownTopology("subj_a", "subj_b", "subj_x"))
	if observe.ReasonRequiresTextEntry.ResolvableByDemonstration() {
		t.Error("text entry is marked as something another demonstration would resolve; it " +
			"needs consent and a representation, not repetition")
	}
	if !reasonsOf(a)[observe.ReasonRequiresTextEntry] {
		t.Fatalf("the text-entry boundary was lost: %v", a.Reasons)
	}
}

// ── PARTS 11–13: comparing two demonstrations ─────────────────────────────────

// Two equivalent demonstrations agree; a materially different one does not.
//
// This is the machinery the next milestone needs, proven synthetically so nobody has to perform
// a second demonstration to find out whether it works.
func TestTwoDemonstrationsAreComparedSemantically(t *testing.T) {
	first := cleanCandidate()

	t.Run("identical", func(t *testing.T) {
		if got := observe.CompareCandidates(first, cleanCandidate()); got != observe.AgreementSame {
			t.Errorf("agreement = %q, want same", got)
		}
	})

	t.Run("one extra directional press", func(t *testing.T) {
		// The same move made from one row further away. A comparison that called this
		// different would never find two honest demonstrations compatible.
		second := candidate(
			step(checkpoint("subj_x"), observe.NavDown, observe.NavDown, observe.NavDown,
				observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavConfirm))
		if got := observe.CompareCandidates(first, second); got != observe.AgreementCompatible {
			t.Errorf("agreement = %q, want compatible", got)
		}
	})

	t.Run("a materially different route", func(t *testing.T) {
		second := candidate(
			step(checkpoint("subj_x"), observe.NavLeft, observe.NavBack, observe.NavDown,
				observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavUp, observe.NavConfirm))
		if got := observe.CompareCandidates(first, second); got != observe.AgreementDifferent {
			t.Errorf("agreement = %q, want different — `back` is decisive and does not "+
				"reduce to padding", got)
		}
	})

	t.Run("a different intermediate screen", func(t *testing.T) {
		second := candidate(
			step(checkpoint("subj_y"), observe.NavDown, observe.NavDown, observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavConfirm))
		if got := observe.CompareCandidates(first, second); got != observe.AgreementDifferent {
			t.Errorf("agreement = %q, want different", got)
		}
	})

	t.Run("a different number of legs", func(t *testing.T) {
		second := candidate(step(checkpoint("subj_b"), observe.NavConfirm))
		if got := observe.CompareCandidates(first, second); got != observe.AgreementDifferent {
			t.Errorf("agreement = %q, want different", got)
		}
	})

	t.Run("one of them typed and the other did not", func(t *testing.T) {
		second := cleanCandidate()
		second.Steps[0].RequiresTextEntry = true
		if got := observe.CompareCandidates(first, second); got != observe.AgreementDifferent {
			t.Errorf("agreement = %q, want different", got)
		}
	})

	t.Run("a different relationship", func(t *testing.T) {
		second := cleanCandidate()
		second.Relationship = observe.RelationshipRef{From: "subj_a", To: "subj_c"}
		if got := observe.CompareCandidates(first, second); got != observe.AgreementIncomparable {
			t.Errorf("agreement = %q, want incomparable", got)
		}
	})

	t.Run("one of them never finished", func(t *testing.T) {
		second := cleanCandidate()
		second.Complete = false
		if got := observe.CompareCandidates(first, second); got != observe.AgreementIncomparable {
			t.Errorf("agreement = %q, want incomparable", got)
		}
	})
}

// A transient checkpoint that has since been recognised still matches the transient one.
//
// Part of the tolerance model, and the case that makes reassessment worth anything: two
// demonstrations of the same route, taken either side of the user naming a screen, must not be
// read as two different routes.
func TestATransientCheckpointStillMatchesItsRecognisedSelf(t *testing.T) {
	before := candidate(
		step(transient(observe.TermHelp), observe.NavConfirm),
		step(checkpoint("subj_b"), observe.NavConfirm))
	after := candidate(
		step(transient(observe.TermHelp), observe.NavConfirm),
		step(checkpoint("subj_b"), observe.NavConfirm))
	if got := observe.CompareCandidates(before, after); got != observe.AgreementSame {
		t.Errorf("two demonstrations through the same unrecognised screen compared as %q", got)
	}
	// And a DIFFERENT unrecognised screen is different.
	other := candidate(
		step(transient(observe.TermQuit), observe.NavConfirm),
		step(checkpoint("subj_b"), observe.NavConfirm))
	if got := observe.CompareCandidates(before, other); got != observe.AgreementDifferent {
		t.Errorf("two demonstrations through different unrecognised screens compared as %q", got)
	}
}

// ── PART 16: authority ────────────────────────────────────────────────────────

// Nothing in an assessment can be run, and nothing it produces flips a candidate's verification.
func TestAnAssessmentGrantsNoAuthority(t *testing.T) {
	rt := reflect.TypeOf(observe.CandidateAssessment{})
	for _, forbidden := range []string{
		"Execute", "Run", "Replay", "Perform", "Apply", "Compile", "Invoke", "Promote",
	} {
		if _, ok := rt.MethodByName(forbidden); ok {
			t.Errorf("CandidateAssessment has a %s method", forbidden)
		}
		if _, ok := reflect.PointerTo(rt).MethodByName(forbidden); ok {
			t.Errorf("*CandidateAssessment has a %s method", forbidden)
		}
	}
	// The best possible verdict still leaves the candidate unverified.
	c := cleanCandidate()
	a := assess(c, knownTopology("subj_a", "subj_b", "subj_x"))
	if a.Verdict != observe.CandidateConsistent {
		t.Fatalf("the clean fixture did not reach the best verdict: %+v", a)
	}
	if a.Verified || c.Verified {
		t.Fatal("the best available verdict granted verification")
	}
}

// ── PART 20: privacy ──────────────────────────────────────────────────────────

// An assessment cannot hold captured input.
func TestNoRawInputCanReachAnAssessment(t *testing.T) {
	forbidden := []string{
		"keycode", "scancode", "rawkey", "rune", "character", "deviceid", "vkey",
		"screenshot", "pixels", "image", "frame", "title", "label", "rawtext",
		"username", "account", "path", "filename", "window", "process", "pid",
		"generation", "coordinate",
	}
	seen := map[reflect.Type]bool{}
	var walk func(rt reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
			rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			if rt.Kind() == reflect.Map {
				walk(rt.Key(), path+"[key]")
			}
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			name := strings.ToLower(f.Name)
			for _, bad := range forbidden {
				if strings.Contains(name, bad) {
					t.Errorf("%s.%s (%s) could hold captured input", path, f.Name, f.Type)
				}
			}
			walk(f.Type, path+"."+f.Name)
		}
	}
	walk(reflect.TypeOf(observe.CandidateAssessment{}), "CandidateAssessment")
}

// ── PARTS 17/18/19: recomputation, not storage ────────────────────────────────

// No verdict is written into durable memory, and the stored candidate is judged fresh.
//
// Persistence of the JUDGEMENT would be the bug. What survives a restart is the observation; the
// conclusion is derived from it and from whatever Marco knows at the moment somebody asks.
func TestAStoredDemonstrationIsJudgedFreshlyEveryTime(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)
	if _, res := demonstrate(t, store, happyScript()); res.Assessment == nil {
		t.Fatal("the demonstration was not assessed")
	}

	// A NEW Director over the same file.
	reopened := memoryAt(t, dir)
	kept := reopened.Candidates("testgame")
	if len(kept) != 1 {
		t.Fatalf("%d candidate(s) survived", len(kept))
	}
	// Nothing durable carries a verdict.
	rt := reflect.TypeOf(observe.ProcedureCandidate{})
	for _, forbidden := range []string{"Verdict", "Assessment", "Reasons"} {
		if _, ok := rt.FieldByName(forbidden); ok {
			t.Errorf("ProcedureCandidate carries a %s field; a judgement stored beside the "+
				"observation would still say 'unverifiable' long after the user had "+
				"resolved it", forbidden)
		}
	}
	// And recomputation is available, giving the same answer for the same inputs — which is
	// the replay-parity property, stated over its real dependency.
	first := observesession.AssessStored("testgame", reopened, reopened,
		observe.DefaultCaptureBounds())
	second := observesession.AssessStored("testgame", reopened, reopened,
		observe.DefaultCaptureBounds())
	if len(first) != 1 {
		t.Fatalf("%d assessment(s) recomputed", len(first))
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("recomputing the same candidate against the same memory gave two answers:\n"+
			"%+v\n%+v", first, second)
	}
	if first[0].Verified {
		t.Error("a recomputed assessment granted verification")
	}
}
