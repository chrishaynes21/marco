package main

// THE BRIDGE ACCEPTANCE: A SAVED PLAY, ITS OWN BYTES, AND WHAT IT IS ALLOWED TO REACH.
//
// Every test here saves a complete two-action `.marco` play and its binding record, reloads both,
// preflights them, and runs THE FILE through the production runtime. Nothing regenerates a
// snippet per effect and nothing calls `PerformWorkflow(name)`: the order and the continuation
// are in the dpSaved source, which is the claim the whole increment rests on.
//
// Each one asserts the thing an acceptance has to assert and a unit test usually does not:
// whether the second interaction was ever ASKED ABOUT. Counting final presses cannot tell a run
// that stopped from a run that asked, was refused, and stopped — and those are different
// products.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// dpAStore is where a dpSaved routine is found, standing where `demo.Store` stands.
type dpAStore map[string]workflowstore.Artifact

func (s dpAStore) Sequence(name string) (workflowstore.Artifact, bool) {
	a, ok := s[name]
	return a, ok
}

// dpSaved is a play and its bindings on disk, as a person's machine would have them.
type dpSaved struct {
	dir   string
	play  string
	src   string
	binds demonstrated.Bindings
}

// dpSave composes a play from a routine and writes BOTH files, through the production path.
func dpSave(t *testing.T, art workflowstore.Artifact, picked []demonstrated.Named) dpSaved {
	t.Helper()
	src, binds, err := demonstrated.Compose(art, "an act", picked, dpLooked, marcoChecks{})
	if err != nil {
		t.Fatalf("composing the play: %v", err)
	}
	dir := t.TempDir()
	play := filepath.Join(dir, "ark-graphics-live.marco")
	if err := os.WriteFile(play, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := binds.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ark-graphics-live.scenes.json"),
		raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dpSaved{dir: dir, play: play, src: src, binds: binds}
}

// dpReload reads both files back off disk, the way a run does.
func (s dpSaved) dpReload(t *testing.T) (string, demonstrated.Bindings) {
	t.Helper()
	src, err := os.ReadFile(s.play)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, "ark-graphics-live.scenes.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := demonstrated.UnmarshalBindings(raw)
	if err != nil {
		t.Fatalf("reloading the bindings: %v", err)
	}
	return string(src), b
}

const dpTheRun = "r_1792448000_1"

// dpPerform is one whole run of a dpSaved play: dpReload, preflight, drive the production coordinator.
func dpPerform(t *testing.T, s dpSaved, art workflowstore.Artifact, st *dpStage,
	now *time.Time, ctx context.Context) (*demonstrated.Account, *dpAnswersWhereItWasAsked,
	*dpAWalkingGrounder, *dpNotebook, *workflowrun.Runner, error) {

	t.Helper()
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatalf("the saved pair did not preflight: %v", err)
	}
	run, bind, grant, ground, book := dpARun(t, art, now)
	st.book = book
	st.now = now
	sup := demonstrated.NewSupervised(run, bind, st.look, 120)
	h, rerr := performDemonstratedPlay(ctx, dpTheRun, plan, sup, nil, io.Discard)
	return h, grant, ground, book, run, rerr
}

// dpAWorkingStage is the world in which both interactions do what they were demonstrated to do.
func dpAWorkingStage() *dpStage {
	return dpAWalk(dpPanelWords(dpFirstOutcome()...), dpPanelWords(dpSecondOutcome()...))
}

// ── 1. SAVE, RELOAD, COMPILE ──────────────────────────────────────────────────

func TestATwoActionPlayAndItsBindingsSurviveSaveAndReload(t *testing.T) {
	art := dpATwoTapRoutine()
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	if src != s.src {
		t.Fatal("the play that came back off disk is not the play that was written")
	}
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatalf("the reloaded pair does not preflight: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("the plan binds %d interaction(s): %+v", len(plan.Steps), plan.Steps)
	}
	if plan.Steps["the graphics tab"] != 0 || plan.Steps["apply changes"] != 1 {
		t.Fatalf("the names bind to the wrong interactions: %+v", plan.Steps)
	}
	// AND THE REVISION IS THE ROUTINE'S CONTENT, not its timestamp. A corrected routine
	// keeps its SavedAt — it is the link to the raw demonstration — and moves CorrectedAt,
	// so a revision built from the timestamp would call two different procedures one.
	dpCorrected := art
	dpCorrected.CorrectedAt = art.SavedAt.Add(time.Hour)
	dpCorrected.Steps = dpCorrected.Steps[:1]
	was, _ := demonstrated.RevisionOf(art)
	now, _ := demonstrated.RevisionOf(dpCorrected)
	if was == now {
		t.Fatal("a dpCorrected play has the same revision as the one it replaced")
	}
}

// AND THE GENERATED PLAY IS THE NESTED FORM, which is the only form that gates.
//
// Measured, and the reason this test exists rather than a comment: with the two sentences FLAT,
// a failure of the first does not stop the second — the host is asked for both and the script
// logs success. See TestAFlatPlayWouldHavePressedTheSecondControl, which performs that
// measurement rather than asserting it.
func TestTheGeneratedPlayIsNested(t *testing.T) {
	s := dpSave(t, dpATwoTapRoutine(), dpTheNames())
	first := strings.Index(s.src, `do GraphicsTab's Press...`)
	second := strings.Index(s.src, `do ApplyChanges's Press...`)
	if first < 0 || second < 0 {
		t.Fatalf("the play is not the shape this bridge generates:\n%s", s.src)
	}
	// THE GATE BETWEEN THE TWO, looked for BETWEEN them rather than in the whole file —
	// the header comment contains the words `when ok?` as prose, and the first draft of
	// this assertion matched that and failed a play that was correctly nested.
	gate := strings.Index(s.src[first:second], "when ok?")
	if gate < 0 {
		t.Fatalf("there is no success arm between the two interactions:\n%s", s.src)
	}
	// A SENTENCE AT THE SAME INDENT AS THE FIRST would be the flat form. The second `do`
	// must be deeper than the first.
	if dpIndentOf(s.src, second) <= dpIndentOf(s.src, first) {
		t.Fatalf("the two interactions sit at the same depth, which is the flat form "+
			"that does not gate:\n%s", s.src)
	}
}

func dpIndentOf(src string, at int) int {
	start := strings.LastIndex(src[:at], "\n") + 1
	return at - start
}

// ── 2. THE SAVED ORDER CONTROLS THE INVOCATION ORDER ──────────────────────────

func TestTheSavedSourceOrderControlsInvocationOrder(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	h, _, _, book, _, _ := dpPerform(t, dpSave(t, art, dpTheNames()), art, dpAWorkingStage(),
		&now, context.Background())

	if got := h.Outcome(); got != runref.Verified {
		t.Fatalf("the run ended %q: %+v", got, h.Asked())
	}
	asked := dpNamesOf(h)
	if len(asked) != 2 || asked[0] != "the graphics tab" || asked[1] != "apply changes" {
		t.Fatalf("the play's sentences were not what drove the order: %v", asked)
	}
	if book.clicks() != 2 {
		t.Fatalf("%d press(es) went out, want 2", book.clicks())
	}
}

// AND REORDERING THE PLAY REORDERS THE RUN, which is the whole reason `Begin` exists.
//
// The same routine, the same revision, the same two interactions — dpSaved with the names in the
// other order. Nothing about the artifact changed, so a run that still performed interaction 0
// first would be performing the FILE's order rather than the play's, silently.
//
// On the ONE-SCREEN routine, because that is the only world in which the question is about the
// order at all: on the three-screen walk, interaction 1 cannot be performed first because its
// screen is not up, and that refusal is the test below this one.
func TestReorderingTheSavedPlayReordersTheRun(t *testing.T) {
	art := dpATwoTapPanel()
	now := dpLooked
	backwards := []demonstrated.Named{
		{Position: 1, Name: "apply changes"},
		{Position: 0, Name: "the graphics tab"},
	}
	// THE WORLD ANSWERS EACH PRESS WITH THAT INTERACTION'S OWN OUTCOME, in the order the
	// play performs them: interaction 1 first, so its words appear first.
	st := dpAPanel(dpPanelWords(dpSecondOutcome()...), dpPanelWords(dpFirstOutcome()...))
	h, grant, _, book, _, _ := dpPerform(t, dpSave(t, art, backwards), art, st,
		&now, context.Background())

	if got := h.Outcome(); got != runref.Verified {
		t.Fatalf("the reordered play ended %q: %+v", got, h.Asked())
	}
	asked := dpNamesOf(h)
	if len(asked) != 2 || asked[0] != "apply changes" || asked[1] != "the graphics tab" {
		t.Fatalf("the reordered play did not reorder the run: %v", asked)
	}
	if h.Asked()[0].Step != 1 || h.Asked()[1].Step != 0 {
		t.Fatalf("the sentences performed interactions %d then %d",
			h.Asked()[0].Step, h.Asked()[1].Step)
	}
	// AND THE PERSON WAS ASKED ABOUT INTERACTION 1 FIRST, which is the half a press count
	// cannot see: a run that pressed in the new order but asked in the old one would be
	// putting the wrong question to somebody.
	if len(grant.asked) == 0 || grant.asked[0].Step != 1 {
		t.Fatalf("the first authorisation was for interaction %+v", grant.asked)
	}
	if book.clicks() != 2 {
		t.Fatalf("%d press(es) went out, want 2", book.clicks())
	}
}

// AND A REORDERING THAT CANNOT BE PERFORMED REFUSES, rather than performing the original order.
//
// The Architect's condition, in one test: *reordering may legitimately refuse because the game is
// on the wrong screen; it must never silently execute the original order.* On the three-screen
// walk, interaction 1 belongs to a screen that only exists after interaction 0 has happened. A
// play that asks for it first is standing somewhere it cannot act, and the honest answer is
// nothing — no press, and a sentence saying why.
func TestAReorderingThatCannotBePerformedRefusesRatherThanPerformingTheOldOrder(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	backwards := []demonstrated.Named{
		{Position: 1, Name: "apply changes"},
		{Position: 0, Name: "the graphics tab"},
	}
	st := dpAWorkingStage()
	st.limit = 60
	h, grant, _, book, _, _ := dpPerform(t, dpSave(t, art, backwards), art, st,
		&now, context.Background())

	if book.clicks() != 0 {
		t.Fatalf("%d press(es) went out on a screen neither interaction could be "+
			"performed from", book.clicks())
	}
	if len(grant.asked) != 0 {
		t.Fatalf("somebody was asked to authorise %+v", grant.asked)
	}
	if got := h.Outcome(); got == runref.Verified {
		t.Fatalf("a run that pressed nothing reported %q", got)
	}
	// AND THE FIRST SENTENCE WAS THE PLAY'S FIRST SENTENCE. The failure is interaction 1
	// being unperformable here, not interaction 0 being performed instead.
	if len(h.Asked()) == 0 || h.Asked()[0].Step != 1 {
		t.Fatalf("the run did not start with the play's first sentence: %+v", h.Asked())
	}
}

// ── 3. NOTHING FOLLOWS AN UNVERIFIED INTERACTION ──────────────────────────────

func TestTheSecondInteractionAsksForNeitherAuthorisationNorInputUntilTheFirstVerifies(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	// THE OUTCOME IS WITHHELD. The screen changes — the destination arrives — and the
	// words the demonstration read never appear, so interaction 0's verdict can only ever
	// be `unconfirmed`.
	st := dpAWalk(nil, nil)
	st.limit = 120
	h, grant, _, book, run, err := dpPerform(t, dpSave(t, art, dpTheNames()), art, st,
		&now, context.Background())
	// THE ACCOUNT, NOT THE ERROR. The play handles its own failure in its `or?` arm, so
	// the runtime completes successfully — see Host.Outcome.
	_ = err
	if got := h.Outcome(); got != runref.Unconfirmed {
		t.Fatalf("the run ended %q, want unconfirmed: %+v", got, h.Asked())
	}
	if n := dpAskedAbout(grant, 1); n != 0 {
		t.Fatalf("the second interaction was put to the person %d time(s) with the "+
			"first one unverified", n)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out, want exactly 1", book.clicks())
	}
	if len(h.Asked()) != 1 {
		t.Fatalf("the play asked for %d interaction(s): %+v", len(h.Asked()), h.Asked())
	}
	if v := h.Asked()[0].Verdict; v != workflowrun.VerdictUnconfirmed {
		t.Fatalf("interaction 0's verdict is %q, want unconfirmed", v)
	}
	// AND THE RUN'S OWN ACCOUNTING AGREES. One press sent, by the coordinator's count.
	if run.Sent() != 1 {
		t.Fatalf("the coordinator counted %d press(es) sent", run.Sent())
	}
}

// ── 4. CONTRADICTED EVIDENCE BLOCKS WHAT FOLLOWS ──────────────────────────────

func TestAContradictedFirstInteractionNeverReachesTheSecond(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	// THE SCREEN SAYS SOMETHING ELSE. Not nothing — a panel of words that is not the one
	// the demonstration read, which is evidence AGAINST the action rather than an absence
	// of evidence for it.
	st := dpAWalk(dpPanelWords("language", "subtitles", "volume", "audio", "mixer"), nil)
	st.limit = 120
	h, grant, _, book, _, err := dpPerform(t, dpSave(t, art, dpTheNames()), art, st,
		&now, context.Background())
	_ = err
	if got := h.Outcome(); got != runref.Contradicted {
		t.Fatalf("the run ended %q, want contradicted: %+v", got, h.Asked())
	}
	if n := dpAskedAbout(grant, 1); n != 0 {
		t.Fatalf("the second interaction was put to the person %d time(s)", n)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out, want exactly 1", book.clicks())
	}
	if len(h.Asked()) != 1 || h.Asked()[0].Verdict != workflowrun.VerdictContradicted {
		t.Fatalf("the account of the run is %+v", h.Asked())
	}
	// AND THE SENTENCE SAYS WHICH, because "it did not work" has several meanings and
	// they call for different responses. Read off the ACCOUNT, not off the runtime's
	// error: the play handled its own failure, so there is no runtime error to read.
	if !strings.Contains(h.Asked()[0].Refused, "contradicted") {
		t.Fatalf("the record does not say the screen disagreed: %+v", h.Asked()[0])
	}
	if !strings.Contains(h.Asked()[0].Refused, "what appeared instead") {
		t.Fatalf("the record does not say what the screen said: %+v", h.Asked()[0])
	}
}

// ── 5. CANCELLATION BLOCKS WHAT FOLLOWS AND KEEPS THE ACCOUNTING ──────────────

func TestCancellationBlocksTheNextInteractionAndPreservesThePriorAccounting(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	ctx, cancel := context.WithCancel(context.Background())
	// CANCELLED THE MOMENT THE FIRST PRESS HAS GONE OUT. Not before — the point is that
	// prior input is still accounted for, which needs there to be prior input.
	st := dpAWorkingStage()
	st.limit = 120
	dpCancelWhenPressed(st, cancel)

	h, grant, _, book, run, err := dpPerform(t, dpSave(t, art, dpTheNames()), art, st, &now, ctx)
	_ = err
	if got := h.Outcome(); got != runref.Cancelled {
		t.Fatalf("the run ended %q, want cancelled: %+v", got, h.Asked())
	}
	if n := dpAskedAbout(grant, 1); n != 0 {
		t.Fatalf("the second interaction was put to the person %d time(s) after the "+
			"run was cancelled", n)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out, want exactly 1", book.clicks())
	}
	// THE ACCOUNTING SURVIVES THE CANCELLATION. A run that forgot the press it had
	// already sent would report a cancelled run as one that never acted, which is the
	// one thing a person needs to know afterwards.
	if run.Sent() != 1 {
		t.Fatalf("after cancellation the run accounts for %d press(es), want 1",
			run.Sent())
	}
	// AND THE ACCOUNT HOLDS NO VERIFIED SECOND INTERACTION, which is the invariant across
	// BOTH orderings a cancellation can take — and both happen, roughly one run in six:
	//
	//   - the cancel lands inside the first interaction, so `Press` returns cancelled,
	//     the play takes its `or?` arm, and the second sentence is never reached;
	//   - the first interaction verifies first and the cancel lands between the
	//     sentences, so the second sentence IS reached and refused — or the runtime's
	//     own frame-tree abort gets there first and the host is never called at all.
	//
	// Asserting one of those was the first draft of this test, and the intermittence it
	// produced is what uncovered the `Account.Cut` defect: in the third ordering every
	// Ask said verified, and the run reported `verified` about half a routine. What must
	// be true in all three is that nothing was verified after the stop.
	for _, a := range h.Asked() {
		if a.Step == 1 && a.Verified() {
			t.Fatalf("the second interaction was verified after the run was "+
				"cancelled: %+v", a)
		}
	}
	if len(h.Asked()) == 0 {
		t.Fatal("the run is recorded as having asked for nothing at all")
	}
}

// dpCancelWhenPressed cancels the run as soon as one press has been written down.
func dpCancelWhenPressed(s *dpStage, cancel func()) {
	s.watch = func(clicks int) {
		if clicks >= 1 {
			cancel()
		}
	}
}

// ── 6. INVALID SOURCE OR BAD METADATA REFUSES BEFORE ANY INPUT ────────────────

func TestInvalidSourceOrMissingMetadataRefusesBeforeAnyInput(t *testing.T) {
	art := dpATwoTapRoutine()
	good := dpSave(t, art, dpTheNames())
	src, binds := good.dpReload(t)

	for _, c := range []struct {
		name string
		src  string
		b    demonstrated.Bindings
		in   demonstrated.Routines
		says string
	}{
		{
			name: "no bindings at all",
			src:  src, b: demonstrated.Bindings{Version: demonstrated.BindingsVersion},
			in: dpAStore{art.Name: art}, says: "no bindings",
		},
		{
			name: "a record this build cannot vouch for",
			src:  src, b: demonstrated.Bindings{Version: 99, Scenes: binds.Scenes},
			in: dpAStore{art.Name: art}, says: "version 99",
		},
		{
			name: "the play has been edited since the bindings were saved",
			src:  src + "\n// somebody added a line\n", b: binds,
			in: dpAStore{art.Name: art}, says: "changed since its bindings were saved",
		},
		{
			name: "source that does not compile",
			src:  "the Nonsense is a\n", b: binds,
			in: dpAStore{art.Name: art}, says: "does not compile",
		},
		{
			name: "the play is not on this machine",
			src:  src, b: binds, in: dpAStore{}, says: "not a saved play",
		},
		{
			name: "the play has changed since the play was bound",
			src:  src, b: binds, in: dpAStore{art.Name: dpCorrected(art)},
			says: "has changed since this play was saved",
		},
		{
			name: "a binding names an interaction the play does not have",
			src:  src, b: dpWithPosition(binds, "apply changes", 7),
			in: dpAStore{art.Name: art}, says: "which has 2",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			// A MISMATCHED POSITION ALSO CHANGES THE DIGEST CASE: rebuild the
			// source digest where the test altered the record rather than the play.
			b := c.b
			if c.name == "a binding names an interaction the play does not have" {
				b.Source = binds.Source
			}
			_, err := demonstrated.Preflight(c.src, b, c.in, marcoChecks{})
			if err == nil {
				t.Fatalf("this was accepted and it must not be")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say why: %v", err)
			}
		})
	}
	// AND NOTHING WAS PERFORMED IN ANY OF THEM: preflight is reached before a coordinator
	// exists, so there is no press to count. The assertion that matters is that
	// `PerformFile` cannot be called without a Plan, and a Plan comes only from Preflight.
}

func dpCorrected(a workflowstore.Artifact) workflowstore.Artifact {
	a.Notes = append(a.Notes, "a person corrected this play")
	return a
}

func dpWithPosition(b demonstrated.Bindings, name string, at int) demonstrated.Bindings {
	out := copyBindings(b)
	for i := range out.Scenes {
		if out.Scenes[i].Name == name {
			out.Scenes[i].Position = at
		}
	}
	return out
}

// ── 7. REPEATING ONE REVISION GROUNDS AND AUTHORISES AGAIN ────────────────────

func TestRepeatingTheSameSavedRevisionGroundsAndAuthorisesAgain(t *testing.T) {
	art := dpATwoTapRoutine()
	s := dpSave(t, art, dpTheNames())

	first := dpLooked
	h1, g1, ground1, book1, _, err1 := dpPerform(t, s, art, dpAWorkingStage(), &first,
		context.Background())
	if err1 != nil {
		t.Fatalf("the first run did not finish: %v", err1)
	}
	if got := h1.Outcome(); got != runref.Verified {
		t.Fatalf("the first run ended %q: %+v", got, h1.Asked())
	}
	second := dpLooked
	h2, g2, ground2, book2, _, err2 := dpPerform(t, s, art, dpAWorkingStage(), &second,
		context.Background())
	if err2 != nil {
		t.Fatalf("the second run did not finish: %v", err2)
	}
	if got := h2.Outcome(); got != runref.Verified {
		t.Fatalf("the second run ended %q: %+v", got, h2.Asked())
	}

	// NOTHING PERSISTED IS PERMISSION. The same file, the same revision, and the person
	// was asked again about each interaction — twice over the two runs, not once.
	for step := 0; step < 2; step++ {
		if a, b := dpAskedAbout(g1, step), dpAskedAbout(g2, step); a == 0 || b == 0 {
			t.Fatalf("interaction %d was authorised %d time(s) then %d; a saved play "+
				"does not carry permission", step, a, b)
		}
	}
	if ground1.times == 0 || ground2.times == 0 {
		t.Fatalf("grounding happened %d then %d times; each run grounds afresh",
			ground1.times, ground2.times)
	}
	if book1.clicks() != 2 || book2.clicks() != 2 {
		t.Fatalf("%d then %d press(es)", book1.clicks(), book2.clicks())
	}
}

// ── 8. ONE CANONICAL RUN REFERENCE ────────────────────────────────────────────

func TestEveryRecordOfOneRunCarriesOneReference(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	h, _, _, _, _, _ := dpPerform(t, dpSave(t, art, dpTheNames()), art, dpAWorkingStage(), &now,
		context.Background())
	asks := h.Asked()
	if len(asks) != 2 {
		t.Fatalf("the account of the run is %+v", asks)
	}
	for _, a := range asks {
		if a.Run != dpTheRun {
			t.Fatalf("%q is recorded under %q, not the run's own reference %q",
				a.Name, a.Run, dpTheRun)
		}
	}
	// AND IT IS THE SHARED FORMAT, the one both binaries already parse, rather than a
	// string this package invented.
	if _, ok := runref.Parse(runref.Cite(dpTheRun, "nowhere.json")); !ok {
		t.Fatalf("%q is not a reference the run-reference format recognises", dpTheRun)
	}
}

// dpNamesOf is which interactions the play asked for, in order.
func dpNamesOf(h *demonstrated.Account) []string {
	var out []string
	for _, a := range h.Asked() {
		out = append(out, a.Name)
	}
	return out
}

// PATIENCE IS SPENT PER INTERACTION, AND IT IS MEASURED HERE RATHER THAN ASSUMED.
//
// A two-interaction run of this fixture consumes 16 observations in total, and the whole run
// completes with a patience of 6 — because the number that matters is what EACH interaction
// consumes, not what the run does. At 5 the first interaction runs out and the run refuses with
// one press sent, which is the other half of the same fact.
//
// # What this does not pin, and how that was established
//
// `Begin` resets `looks`, and a mutation run shows that reset is NOT observable: removing it
// leaves every test in this package green, and so does removing the same reset in `advance`, and
// so does removing BOTH. The two are redundant with each other and with the paths around them.
// Recorded rather than quietly kept — see begin.go, which says the same thing beside the code.
//
// So what this test holds is the per-interaction allowance from the outside, wherever it comes
// from: a run whose interactions each fit inside the patience completes, and one whose first
// interaction does not, stops.
func TestPatienceIsSpentPerInteractionNotPerRun(t *testing.T) {
	art := dpATwoTapPanel()
	now := dpLooked
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		patience int
		want     runref.Outcome
		clicks   int
	}{
		{6, runref.Verified, 2},
		{5, runref.Refused, 1},
	} {
		now = dpLooked
		run, bind, _, _, book := dpARun(t, art, &now)
		run.WithPatience(c.patience)
		st := dpAPanel(dpPanelWords(dpFirstOutcome()...), dpPanelWords(dpSecondOutcome()...))
		st.book, st.now = book, &now
		h, _ := performDemonstratedPlay(context.Background(), dpTheRun, plan,
			demonstrated.NewSupervised(run, bind, st.look, 120), nil, io.Discard)
		if got := h.Outcome(); got != c.want {
			t.Fatalf("patience %d: the run ended %q, want %q: %+v",
				c.patience, got, c.want, h.Asked())
		}
		if book.clicks() != c.clicks {
			t.Fatalf("patience %d: %d press(es), want %d",
				c.patience, book.clicks(), c.clicks)
		}
	}
}

// AN INTERACTION THAT CANNOT BE CHECKED DOES NOT CARRY THE PLAY FORWARD.
//
// Every routine dpSaved before outcomes were recorded is in this state: its steps have no
// `Outcome`, so there is nothing to compare fresh evidence against and the verdict is
// `VerdictUnasked` forever. [[ADR-242-the-second-action-waits-for-the-first-ones-answer]] keeps
// such a step ADVANCING on its arrival, deliberately — gating on a question nobody can answer
// would stop every old routine on the existing `dpPerform-workflow` path.
//
// Through this bridge the answer has to be different, and the difference is the whole meaning of
// `ok`: the play's second sentence sits inside `when ok?`, and `ok` is a VERIFIED interaction.
// An interaction nobody can check is not a verified one, so the play stops with a sentence
// saying so rather than pressing a second control on the strength of a question that was never
// asked.
//
// The cost is stated rather than hidden: a demonstrated routine with no recorded outcomes cannot
// be composed into a two-action play that runs past its first action. The old path still runs it.
//
// A mutation run found this: `Invoke`'s "ok only for verified" check survived, because every
// other test reaches it with a real verdict.
func TestAnInteractionWithNoRecordedOutcomeDoesNotCarryThePlayForward(t *testing.T) {
	art := dpATwoTapRoutine()
	for i := range art.Steps {
		art.Steps[i].Outcome = nil
	}
	now := dpLooked
	st := dpAWalk(nil, nil)
	st.limit = 120
	h, grant, _, book, _, _ := dpPerform(t, dpSave(t, art, dpTheNames()), art, st,
		&now, context.Background())

	if got := h.Outcome(); got != runref.Unconfirmed {
		t.Fatalf("the run ended %q, want unconfirmed: %+v", got, h.Asked())
	}
	if n := dpAskedAbout(grant, 1); n != 0 {
		t.Fatalf("the second interaction was put to the person %d time(s) after an "+
			"interaction that could not be checked", n)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out, want exactly 1", book.clicks())
	}
	if len(h.Asked()) != 1 {
		t.Fatalf("the account of the run is %+v", h.Asked())
	}
	// AND THE SENTENCE SAYS IT COULD NOT BE CHECKED, which is a different fact from its
	// having failed and from its having been contradicted.
	if a := h.Asked()[0]; a.Verdict != workflowrun.VerdictUnasked {
		t.Fatalf("the verdict recorded is %q", a.Verdict)
	}
}

// AND A NAME NOTHING IS BOUND TO IS REFUSED, NOT SEARCHED FOR.
//
// Unreachable from production — the play is generated from the bindings and `Preflight` compares
// the digests — so this is defence in depth, reached here with a Plan built by hand. It is worth
// holding because the wrong answer is not an error but a SEARCH: a bridge that fell back to
// finding a control of that name would be the Theater's authority arriving through a door that
// was built to exclude it.
func TestANameNothingIsBoundToIsRefusedWithoutAnyInput(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	run, bind, grant, _, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.book, st.now = book, &now
	// THE PLAN IS MISSING THE FIRST SENTENCE'S NAME.
	plan := demonstrated.Plan{Routine: art, Revision: "by hand", Name: "half-bound",
		Play:  dpSave(t, art, dpTheNames()).src,
		Steps: map[string]int{"apply changes": 1}}
	h, _ := performDemonstratedPlay(context.Background(), dpTheRun, plan,
		demonstrated.NewSupervised(run, bind, st.look, 120), nil, io.Discard)

	if book.clicks() != 0 {
		t.Fatalf("%d press(es) went out for a name nothing is bound to", book.clicks())
	}
	if len(grant.asked) != 0 {
		t.Fatalf("somebody was asked about %+v", grant.asked)
	}
	if got := h.Outcome(); got != runref.Refused {
		t.Fatalf("the run ended %q, want refused: %+v", got, h.Asked())
	}
	if len(h.Asked()) != 1 || !strings.Contains(h.Asked()[0].Refused, "nothing is bound") {
		t.Fatalf("the account of the run is %+v", h.Asked())
	}
}

// A STOPPED RUN ASKS NOBODY ABOUT THE NEXT INTERACTION.
//
// The deterministic half of the cancellation acceptance. Cancelling the context is what a person
// does, and it reaches the coordinator through `Supervisor.Halt` — but it is a RACE by nature:
// the frame's context and the run's stop flag are not cancelled at the same instant, and the
// window between them is exactly where a prompt nobody should see came from. Measured at roughly
// one run in six before the fix, and one in twenty after it is removed again, which is too thin
// to rest an acceptance on.
//
// So this stops the coordinator directly, which is what `Halt` does, and asserts the invariant
// with no timing in it: once the run is stopped, the next interaction is neither grounded nor put
// to anybody, and nothing else is pressed.
func TestAStoppedRunAsksNobodyAboutTheNextInteraction(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	run, bind, grant, _, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.limit = 120
	st.book, st.now = book, &now
	// STOPPED AS SOON AS THE FIRST PRESS HAS GONE OUT, the way a stop signal arrives.
	st.watch = func(clicks int) {
		if clicks >= 1 {
			run.Stop()
		}
	}
	sup := demonstrated.NewSupervised(run, bind, st.look, 120)
	h, _ := performDemonstratedPlay(context.Background(), dpTheRun, plan, sup, nil,
		io.Discard)

	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out after the run was stopped", book.clicks())
	}
	if n := dpAskedAbout(grant, 1); n != 0 {
		t.Fatalf("the second interaction was put to the person %d time(s) after the "+
			"run was stopped", n)
	}
	if run.Sent() != 1 {
		t.Fatalf("a stopped run accounts for %d press(es), want 1", run.Sent())
	}
	if got := h.Outcome(); got != runref.Cancelled {
		t.Fatalf("the run ended %q, want cancelled: %+v", got, h.Asked())
	}
}

// A RUN CANCELLED BEFORE IT STARTED IS CANCELLED, NOT REFUSED.
//
// The deterministic half of `Account.Cut`'s wiring: with the context already cancelled, the
// runtime aborts the frame tree before any sentence is dispatched, so the host is never called
// and the account holds nothing at all. Without the run-level record, that reads as `refused` —
// Marco declining — when what happened is that somebody stopped it. The two send a person to
// different places.
func TestARunCancelledBeforeItStartedIsReportedAsCancelled(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	run, bind, grant, _, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.book, st.now = book, &now
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h, _ := performDemonstratedPlay(ctx, dpTheRun, plan,
		demonstrated.NewSupervised(run, bind, st.look, 120), nil, io.Discard)

	if book.clicks() != 0 {
		t.Fatalf("%d press(es) went out for a run that was cancelled first",
			book.clicks())
	}
	if len(grant.asked) != 0 {
		t.Fatalf("somebody was asked about %+v", grant.asked)
	}
	if got := h.Outcome(); got != runref.Cancelled {
		t.Fatalf("the run ended %q, want cancelled: %+v", got, h.Asked())
	}
}
