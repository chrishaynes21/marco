package orchestrator

// WHAT A SAVED ROUTINE STATES ABOUT THE GENERATION THAT WROTE IT.
//
// Four readings, one authority, and one thing the stamp deliberately does NOT say. The readings
// are held here because the damage from merging any two of them is a sentence said to a person
// about a file of theirs: a newer build's routine called current is this build claiming to have
// written it, and an unstated one called "written before versioning" is a claim about when
// somebody made a file that the file does not support.
//
// The history is walked against SYNTHETIC generations. There is one real generation, so there is
// nothing yet to walk — and a mechanism with no live input is one nobody can tell is wired. It
// has to work the first time it matters, which will be on somebody's library.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// THE GENERATION A SAVE WRITES IS THE GENERATION THIS READS BACK.
//
// `codegen` writes the line and this package reads it, and the two cannot share a constant
// because `codegen` may not import this one. So the pair is held by test, the same discipline
// `measuredTerms` and the stated mode are under.
func TestTheGenerationASaveWritesIsTheGenerationThisReads(t *testing.T) {
	_, dr := recordedWith(t, Coordinates)
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	got := GenerationOf(src)
	if got.Reading != GenerationCurrent {
		t.Errorf("a routine this build just wrote reads back as %q (%s)", got.Reading,
			got.Why)
	}
	if got.Generation != codegen.Generation {
		t.Errorf("the saved routine reads back as generation %d and codegen writes %d",
			got.Generation, codegen.Generation)
	}
	// AND IT SAYS SO IN WORDS A PERSON READS, not only in a word a parser finds. The
	// file is the durable account of a routine and somebody opens it.
	if !strings.Contains(src, fmt.Sprintf("Generation: %d", codegen.Generation)) {
		t.Errorf("the saved source does not state its generation anywhere a person "+
			"reading the file would see it:\n%s", src)
	}
}

// AND EVERY ENTRANCE TO THE GENERATOR STAMPS IT, which is where this differs from the mode.
//
// A mode is a fact about somebody's recording and a caller that does not know it must not invent
// one. A generation is a fact about the GENERATOR: whatever called it, this generator wrote the
// file. So the dozen callers that reach `codegen.Route` knowing nothing about a recording still
// produce a routine that says which rules wrote it — a file written by this generator that did
// not say so would be exactly the silence this slice is about.
func TestEveryEntranceToTheGeneratorStampsTheGeneration(t *testing.T) {
	plain, _, err := codegen.Route("ark graphics", "arkascended",
		[]macroir.Step{{Kind: macroir.StepKey, Key: "escape", Count: 1}}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := GenerationOf(plain); got.Reading != GenerationCurrent {
		t.Errorf("codegen.Route wrote a routine that reads as %q", got.Reading)
	}
}

// THE HISTORY IS THE ONE AUTHORITY FOR BOTH HALVES: WHICH GENERATION IS CURRENT, AND WHAT IT DID.
//
// `everyMeasuredTerm` exists because a term added to a shape's list and not to the reporting list
// is read, found absent and never named. A generation added to a counter and not to the history
// would be read, found older, and never explainable — so the current generation IS the last
// entry, and an entry with no words is a build that does not pass its own tests.
func TestTheHistoryIsTheOneAuthorityForTheVersionAndTheWords(t *testing.T) {
	if GenerationNow() != codegen.Generation {
		t.Errorf("this package says the current generation is %d and codegen writes %d, "+
			"so a routine this build wrote would read as something other than "+
			"current", GenerationNow(), codegen.Generation)
	}
	for i, g := range theHistory {
		if g.Generation != i+1 {
			t.Errorf("entry %d is generation %d, so the history is not the "+
				"contiguous 1..n a reading walks", i, g.Generation)
		}
		if len(g.Rules) == 0 {
			t.Errorf("generation %d has no words, so a routine stating the one "+
				"before it would be told it is older and never told what "+
				"changed", g.Generation)
		}
		for j, r := range g.Rules {
			if strings.TrimSpace(r.Says) == "" {
				t.Errorf("generation %d rule %d says nothing", g.Generation, j)
			}
		}
	}
	// AND EVERY GENERATION IN THE HISTORY IS ONE A READING CAN EXPLAIN, which is what
	// makes `explains` more than a formality: a gap would read as unknown and offer
	// nothing, which is safe and is not what anybody wants to ship.
	for n := 1; n <= GenerationNow(); n++ {
		if !theHistory.explains(n) {
			t.Errorf("generation %d is in range and has no account", n)
		}
	}
	if len(ThisGenerationDoes()) == 0 {
		t.Error("this build cannot say what its own generation does")
	}
}

// ── THE FOUR READINGS ─────────────────────────────────────────────────────────

// aStampedRoutine is a minimal saved routine stating whatever a case wants it to state.
func aStampedRoutine(lines ...string) string {
	return "// Auto-generated by `marco learn \"ark graphics\"`. Edit freely.\n" +
		strings.Join(lines, "\n") + "\nuse os.\n\nthe App is a script.\n"
}

// A ROUTINE WITH NO GENERATION LINE READS AS UNSTATED, AND UNSTATED MEANS EXACTLY THAT.
func TestARoutineThatStatesNoGenerationReadsAsUnstated(t *testing.T) {
	got := GenerationOf(aStampedRoutine("// Recorded as: coordinates."))
	if got.Reading != GenerationUnstated {
		t.Errorf("a routine with no generation line reads as %q", got.Reading)
	}
	if got.Generation != 0 || got.Declared != "" {
		t.Errorf("a routine that states nothing read back as %+v", got)
	}
	// AND THE SENTENCE CLAIMS NOTHING ABOUT WHEN THE FILE WAS MADE. "Written before
	// versioning" is a claim the file does not support: a handwritten routine says
	// nothing here, and so does one somebody took the line out of.
	said := GenerationSentence(got, true)
	if !strings.Contains(said, "does not state which generation rules wrote it") {
		t.Errorf("the sentence is %q", said)
	}
	for _, forbidden := range []string{"before", "older", "out of date", "outdated"} {
		if strings.Contains(strings.ToLower(said), forbidden) {
			t.Errorf("the sentence for an unstated routine says %q, which is a "+
				"claim about a file that states nothing: %q", forbidden, said)
		}
	}
}

// A MALFORMED, CONFLICTING OR DUPLICATED DECLARATION IS UNKNOWN — NOT CURRENT AND NOT UNSTATED.
//
// Both wrong answers are damaging and in opposite directions. Current would be this build saying
// it wrote a file it cannot read; unstated would be losing the fact that the file says something,
// which is the one clue to what happened to it. And duplicate counts even when the two agree: a
// file answering one question twice has been edited by something, and which answer the file means
// is not in the file.
func TestAStampThisCannotReadIsUnknownAndNeitherOfTheOthers(t *testing.T) {
	for _, c := range []struct {
		what     string
		lines    []string
		declared string
		says     string
	}{
		{"a word rather than a number", []string{"// Generation: two."}, "two",
			"is not a generation"},
		{"nothing at all after the colon", []string{"// Generation: ."}, "",
			"is not a generation"},
		{"a zero", []string{"// Generation: 0."}, "0", "is not a generation"},
		{"a negative", []string{"// Generation: -1."}, "-1", "is not a generation"},
		{"two different answers", []string{"// Generation: 1.", "// Generation: 7."},
			"1, 7", "states a generation 2 times"},
		{"the same answer twice", []string{"// Generation: 1.", "// Generation: 1."},
			"1, 1", "states a generation 2 times"},
	} {
		t.Run(c.what, func(t *testing.T) {
			got := GenerationOf(aStampedRoutine(c.lines...))
			if got.Reading != GenerationUnknown {
				t.Fatalf("%s reads as %q", c.what, got.Reading)
			}
			if got.Generation != 0 {
				t.Errorf("a reading this build cannot explain carries the "+
					"number %d, which invites arithmetic on it",
					got.Generation)
			}
			if got.Declared != c.declared {
				t.Errorf("the file said %q and the reading preserved %q",
					c.declared, got.Declared)
			}
			if !strings.Contains(got.Why, c.says) {
				t.Errorf("the reason is %q and does not say %q", got.Why, c.says)
			}
			// AND NOTHING IS OFFERED. A reader that cannot say what a
			// generation means cannot say what replacing it would change.
			said := GenerationSentence(got, true)
			if strings.Contains(said, "You can review") {
				t.Errorf("a routine this build cannot explain is offered a "+
					"rebuild anyway: %q", said)
			}
			if !strings.Contains(said, "does not recognise") {
				t.Errorf("the sentence is %q", said)
			}
		})
	}
}

// AN OLDER EXECUTOR MEETING A NEWER STAMP DOES NOT RELABEL IT CURRENT.
//
// The reading an older build gets of a file a newer one wrote, and the one place where getting it
// wrong is silent: `current` would put this build's own generation against somebody else's
// routine in a run record, and the record is what a person cites when they cannot work out why a
// routine behaved differently on two machines.
func TestANewerStampIsNotRelabelledCurrentOrLost(t *testing.T) {
	ahead := GenerationNow() + 3
	got := GenerationOf(aStampedRoutine(fmt.Sprintf("// Generation: %d.", ahead)))

	if got.Reading != GenerationUnknown {
		t.Fatalf("a routine from a later build reads as %q", got.Reading)
	}
	if got.Declared != fmt.Sprint(ahead) {
		t.Errorf("the file states generation %d and the reading preserved %q — which "+
			"is the only fact about it there is", ahead, got.Declared)
	}
	if !strings.Contains(got.Why, "later build") {
		t.Errorf("the reason is %q", got.Why)
	}
	// AND NOTHING IS WALKED FROM IT. A gap cannot be described from a generation this
	// build has no account of.
	if diffs := GenerationDifferencesOf(aStampedRoutine(
		fmt.Sprintf("// Generation: %d.", ahead))); len(diffs) != 0 {
		t.Errorf("this build named %d difference(s) from a generation it cannot "+
			"explain: %+v", len(diffs), diffs)
	}
}

// ── THE GENERATION IS NOT THE CONTRACT ────────────────────────────────────────

// A CURRENT GENERATION AND A LEGACY CONTRACT ARE BOTH TRUE AT ONCE.
//
// The stamp is provenance: which rules wrote the file. The contract is what the file's presses
// promise, read off the shapes. A demonstration with no measured press generates exactly the
// unchecked routine it always did, and today's generator wrote it — so a reader that treated the
// stamp as proof of a guarded routine would call that file checked, which is how somebody comes
// to trust a press that checks nothing.
func TestTheGenerationAndTheContractAreIndependentFacts(t *testing.T) {
	dir := t.TempDir()
	// A ROUTINE WITH NOTHING MEASURED IN IT: a keystroke, which carries no window.
	legacy, _, err := codegen.Route("ark graphics", "",
		[]macroir.Step{{Kind: macroir.StepKey, Key: "escape", Count: 1}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := GenerationOf(legacy); got.Reading != GenerationCurrent {
		t.Errorf("a routine today's generator just wrote reads as %q", got.Reading)
	}
	if c, _ := ContractOf(legacy); c != Legacy {
		t.Errorf("a routine with no measured press reads as contract %q, so this case "+
			"does not establish the two can disagree", c)
	}
	// AND THE OTHER WAY ROUND: a measured press is guarded, written by the same
	// generation. Two facts, two readers, neither derived from the other.
	guardedSrc, _, err := codegen.Route("ark graphics", "arkascended",
		[]macroir.Step{{Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
			WinRel: true, RelX: 722, RelY: 210, RelTo: macroir.RelToWindow,
			WinW: 1920, WinH: 1080, Dpi: 144,
			Process: "arkascended.exe"}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := GenerationOf(guardedSrc); got.Reading != GenerationCurrent {
		t.Errorf("a guarded routine today's generator wrote reads as %q", got.Reading)
	}
	if c, _ := ContractOf(guardedSrc); c != Guarded {
		t.Errorf("the measured routine reads as contract %q", c)
	}
}

// ── THE HISTORY WALK, AGAINST SYNTHETIC GENERATIONS ───────────────────────────

// aHistoryOfThree is three generations, the last two of which changed something describable.
//
// SYNTHETIC ON PURPOSE. There is one real generation, so there is nothing real to walk — and
// stamping a real old file with a generation nobody wrote into it would be the retroactive
// placement this design exists to refuse. A made-up future is the honest way to exercise the
// walk, and the walk has to be right the first time it matters.
func aHistoryOfThree() history {
	return history{
		{Generation: 1, Rules: []GeneratorRule{{Says: "the first rules"}}},
		{Generation: 2, Rules: []GeneratorRule{
			{Says: "a typed text stops what follows it",
				Bears: func(src string) bool {
					return strings.Contains(src, "OS's Type")
				}},
		}},
		{Generation: 3, Rules: []GeneratorRule{
			{Says: "a drag states both of its ends",
				Bears: func(src string) bool {
					return strings.Contains(src, "is a Drag with")
				}},
			{Says: "something no saved file can show either way"},
		}},
	}
}

// THE DIFFERENCES WALKED ARE THE ONES AFTER THE GENERATION THE FILE STATES.
func TestTheDifferencesAreTheGenerationsAfterTheOneStated(t *testing.T) {
	h := aHistoryOfThree()
	for _, c := range []struct {
		states int
		want   []string
	}{
		{1, []string{"a typed text stops what follows it",
			"a drag states both of its ends",
			"something no saved file can show either way"}},
		{2, []string{"a drag states both of its ends",
			"something no saved file can show either way"}},
	} {
		t.Run(fmt.Sprint("stating ", c.states), func(t *testing.T) {
			src := aStampedRoutine(fmt.Sprintf("// Generation: %d.", c.states))
			if got := h.read(src); got.Reading != GenerationOlder {
				t.Fatalf("a routine stating %d of 3 reads as %q", c.states,
					got.Reading)
			}
			diffs := h.differences(src)
			var said []string
			for _, d := range diffs {
				said = append(said, d.Says)
			}
			if strings.Join(said, "|") != strings.Join(c.want, "|") {
				t.Errorf("the differences named are %v, want %v", said, c.want)
			}
			// AND EACH ONE IS ATTRIBUTED, so a person can see which change
			// introduced what rather than reading an undated list.
			for _, d := range diffs {
				if d.Generation <= c.states {
					t.Errorf("%q is attributed to generation %d, which "+
						"this routine states it already had", d.Says,
						d.Generation)
				}
			}
		})
	}
	// AND A ROUTINE STATING THE LAST GENERATION IS CURRENT, WITH NOTHING TO WALK.
	src := aStampedRoutine("// Generation: 3.")
	if got := h.read(src); got.Reading != GenerationCurrent {
		t.Errorf("a routine stating the newest generation reads as %q", got.Reading)
	}
	if diffs := h.differences(src); len(diffs) != 0 {
		t.Errorf("a current routine was written without %d thing(s)", len(diffs))
	}
	// AND AN UNSTATED ROUTINE IS NOT WALKED FROM A GUESS. Walking from generation 1
	// because that is the likeliest answer is the inference this whole slice refuses.
	if diffs := h.differences(aStampedRoutine()); len(diffs) != 0 {
		t.Errorf("a routine that states no generation was told what it was written "+
			"without: %+v", diffs)
	}
}

// OLDER IS NOT AFFECTED. A DIFFERENCE IS CLAIMED AGAINST A ROUTINE ONLY WHERE ITS CONTENTS SHOW IT.
//
// A generation difference is a fact about two GENERATORS. Whether it changes anything for one
// routine is a fact about that routine — so a reader that announced every difference as a defect
// would be telling somebody to replace a file over a rule it never exercises, and the next time
// it said so about a real one they would have learnt to ignore it.
func TestOlderDoesNotMeanAffectedUnlessTheContentsShowIt(t *testing.T) {
	h := aHistoryOfThree()
	// A ROUTINE WITH A TYPED TEXT IN IT, stating generation 1: the typing difference
	// bears on it and the drag difference does not.
	typed := aStampedRoutine("// Generation: 1.") +
		"\nthis's Run does...\n    do OS's Type with \"hello\".\n"
	diffs := h.differences(typed)
	bearing := map[string]bool{}
	for _, d := range diffs {
		bearing[d.Says] = d.Bears
	}
	if !bearing["a typed text stops what follows it"] {
		t.Error("a routine that types text is not told the typing difference bears " +
			"on it, so the one claim worth making is the one not made")
	}
	if bearing["a drag states both of its ends"] {
		t.Error("a routine with no drag in it is told the drag difference bears on it")
	}
	if bearing["something no saved file can show either way"] {
		t.Error("a difference nothing can establish is reported as established")
	}
	if !Bears(diffs) {
		t.Error("a routine with an affected content reports nothing bearing on it")
	}
	// AND THE CONTROL: the same older generation, a routine none of the differences
	// touches. Still older, still offered a comparison, and NOT claimed to need one.
	plain := aStampedRoutine("// Generation: 1.") +
		"\nthis's Run does...\n    do OS's Key with \"escape\".\n"
	if got := h.read(plain); got.Reading != GenerationOlder {
		t.Fatalf("the control reads as %q, so it is not the same case", got.Reading)
	}
	quiet := h.differences(plain)
	if len(quiet) == 0 {
		t.Fatal("the control was told of no differences at all, so this does not " +
			"establish that differences are listed without being claimed")
	}
	if Bears(quiet) {
		t.Errorf("a routine none of the differences touches is claimed to need an "+
			"update: %+v", quiet)
	}
}

// AND THE SENTENCE FOR A ROUTINE WITH NO DEMONSTRATION EXPLAINS WITHOUT OFFERING.
//
// The explanation is owed either way; the offer is only real when there is something to rebuild
// from. A card that offered a rebuild and then failed would be the authorization dead-end this
// product has already shipped once.
func TestARoutineWithNoDemonstrationIsExplainedAndNotOffered(t *testing.T) {
	for _, s := range []StatedGeneration{
		{Reading: GenerationUnstated},
		{Reading: GenerationOlder, Generation: 1, Declared: "1"},
	} {
		said := GenerationSentence(s, false)
		if strings.Contains(said, "You can review") {
			t.Errorf("a routine with no stored demonstration is offered a rebuild: "+
				"%q", said)
		}
		if !strings.Contains(said, "no longer stored beside it") {
			t.Errorf("the sentence does not say why there is nothing to rebuild "+
				"from: %q", said)
		}
		if !strings.Contains(said, "recording it again") {
			t.Errorf("the sentence leaves a person with nothing to do: %q", said)
		}
		// AND THE EXPLANATION SURVIVES, which is the half that is owed regardless.
		if withOne := GenerationSentence(s, true); !strings.Contains(withOne,
			"You can review") {
			t.Errorf("the same routine with its demonstration still there is not "+
				"offered one: %q", withOne)
		}
	}
}
