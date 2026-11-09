package orchestrator

// SIMPLIFYING A ROUTINE IS THE REVIEW FLOW WITH THE OTHER SET OF RULES.
//
// Two choices, one surface, one replacement mechanism. What is held here is the part a review
// cannot be trusted without: that the two choices really do produce different routines, that both
// of them read the mode off the saved FILE rather than off the environment, that the statement
// survives into what is written, and that a command which rewrites a file will not pick between
// namesakes.
//
// The mode is the one with a measured cost behind it. `simplify` depends on it as much as the
// comment does: with no anchor to wait on, the recorded gaps are the only waiting a routine has,
// so a routine whose file states `coordinates` must keep them. Simplifying under the environment
// capped every gap to 50ms and put nothing in its place — a menu pressed before it had drawn.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]] and
// [[ADR-254-what-a-recording-captures-is-stated]].

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aTypedDemonstration is five keys at a typist's pace, a long deliberate pause, and a press.
//
// The gaps are the point. 140ms between letters is typing rhythm either way; the 3 second gap
// before the press is a person waiting for something, and nothing about folding typing may touch
// it. The keys are varied so `simplify` folds them into text rather than leaving them as presses.
func aTypedDemonstration() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "editor", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "m", Down: true, T: at(100)},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(240)},
		{Kind: recorder.EvKey, KeyName: "r", Down: true, T: at(380)},
		{Kind: recorder.EvKey, KeyName: "c", Down: true, T: at(920)},
		{Kind: recorder.EvKey, KeyName: "o", Down: true, T: at(1060)},
		// THE DELIBERATE PAUSE, outside the typing.
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "editor", PID: gamePID, T: at(4060)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "editor", PID: gamePID, T: at(4065)},
	}
}

// aTypedRoutine saves that demonstration as a routine stating the given mode.
func aTypedRoutine(t *testing.T, mode RecordedAs) (Deps, routes.Route) {
	t.Helper()
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("type marco", aTypedDemonstration(), "editor", "a test", mode)
	if dr == nil || len(dr.Steps) == 0 {
		t.Fatal("the typed demonstration produced no draft")
	}
	rt, err := d.Save(dr, ScopeApp, "editor")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	return d, rt
}

// THE TWO CHOICES PRODUCE DIFFERENT ROUTINES, WHICH IS WHAT MAKES THE CHOICE A CHOICE.
//
// Without this the surface offers two buttons that do the same thing, and every assertion about
// which one a review shows is an assertion about a label.
func TestTheTwoRebuildsProduceDifferentRoutines(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)

	current, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing the current rebuild: %v", err)
	}
	folded, err := d.Prepare(rt, RebuildSimplifyTyping)
	if err != nil {
		t.Fatalf("preparing the simplified rebuild: %v", err)
	}
	if current.Now == folded.Now {
		t.Fatalf("both choices rebuilt the same play, so the choice is a label:"+
			"\n%s", current.Now)
	}
	// AND THE DIFFERENCE IS THE ONE THAT WAS ASKED FOR: the demonstration has a 540ms
	// gap in the middle of the typing, which is typing rhythm to the aggressive rule and
	// a deliberate pause to the ordinary one. So the simplified rebuild types the whole
	// word in one step and the current one does not.
	if !strings.Contains(folded.Now, `do OS's Type with "marco"`) {
		t.Errorf("the simplified rebuild did not fold the typing into one step:\n%s",
			folded.Now)
	}
	if strings.Contains(current.Now, `do OS's Type with "marco"`) {
		t.Errorf("the current rebuild folded across the pause too, so the two "+
			"choices differ in something other than what they are named for:\n%s",
			current.Now)
	}
	// AND THEY ARE TWO REVIEWS. The kind is part of the review's identity, so one
	// cannot be replaced with under the other's token.
	if current.Token == folded.Token {
		t.Errorf("the two rebuilds share the token %q", current.Token)
	}
	if current.Rebuild != RebuildCurrent || folded.Rebuild != RebuildSimplifyTyping {
		t.Errorf("the preparations report %q and %q", current.Rebuild, folded.Rebuild)
	}
	// AND THE ONE THAT CHANGES TIMING SAYS SO, in words, before anybody chooses it.
	if folded.Rebuild.Caution() == "" {
		t.Error("the simplified rebuild carries no caution, so `simplify` reads as a " +
			"tidy-up that changes nothing")
	}
	if current.Rebuild.Caution() != "" {
		t.Errorf("the ordinary rebuild carries a caution it has nothing to warn "+
			"about: %q", current.Rebuild.Caution())
	}
	// WHAT IT TAKES OUT, WHAT THAT BUYS, AND WHAT IT COSTS. The third is the one worth
	// naming here: this pass shortens a routine by pressing sooner, and nothing in
	// Marco knows when a screen will catch up, so the person has to be told before they
	// choose rather than by running it.
	for _, says := range []string{"quarter of a second", "before a menu has appeared",
		"Nothing else changes"} {
		if !strings.Contains(folded.Rebuild.Caution(), says) {
			t.Errorf("the caution does not say %q: %q", says,
				folded.Rebuild.Caution())
		}
	}
}

// A STATED COORDINATE MODE SURVIVES BOTH CHOICES, EVEN WITH THE ENVIRONMENT SAYING OTHERWISE.
//
// The defect, measured: `SimplifyRoute` read this process's idea of the mode, so a routine whose
// file states `coordinates`, simplified by a process launched with `$MARCO_CV=on`, had every
// recorded gap capped to 50ms with no anchor to wait on in its place. The 3 second pause this
// routine waits before pressing is exactly what that destroys.
func TestAStatedCoordinateModeSurvivesSimplifyingWithVisionOn(t *testing.T) {
	t.Setenv("MARCO_CV", "on")
	d, rt := aTypedRoutine(t, Coordinates)
	saved, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	// THE ARRANGEMENT HELD: the routine states its mode, and it waits.
	if ModeOf(string(saved)) != Coordinates {
		t.Fatalf("the saved play states mode %q", ModeOf(string(saved)))
	}
	if !strings.Contains(string(saved), "Sleep with 3000") {
		t.Fatalf("the saved play does not carry the deliberate pause, so there is "+
			"nothing here to lose:\n%s", saved)
	}
	// ── THE ORDINARY REBUILD KEEPS IT, WHICH IS THE WHOLE OF ADR-254 ──
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !strings.Contains(p.Now, "Sleep with 3000") {
		t.Errorf("the rebuilt play lost the 3 second pause it waits before "+
			"pressing, so it will press before the screen has caught "+
			"up:\n%s", p.Now)
	}
	// ── AND SIMPLIFYING CAPS IT, BECAUSE THAT IS WHAT IT IS FOR ──
	//
	// NOT A REVERSAL OF THE RULE ABOVE, and the difference is who decided. ADR-254's
	// defect was the ENVIRONMENT capping at save time with nothing on screen to say
	// so — which is still what the case above forbids, under $MARCO_CV=on, for the
	// rebuild a caller gets by saying nothing. This is the choice named "Make it
	// faster", whose caution says it presses sooner and whose result is shown beside
	// the original before anything is written. A simplification that could not touch
	// a wait was a no-op on every recording of clicks, which is most of them.
	folded, err := d.Prepare(rt, RebuildSimplifyTyping)
	if err != nil {
		t.Fatalf("preparing the simplified version: %v", err)
	}
	if strings.Contains(folded.Now, "Sleep with 3000") {
		t.Errorf("simplifying kept the 3 second pause, so it cannot shorten a "+
			"play whose length is its waiting:\n%s", folded.Now)
	}
	if !strings.Contains(folded.Now, "Sleep with 250") {
		t.Errorf("simplifying did not clamp the pause to a quarter of a "+
			"second:\n%s", folded.Now)
	}
	// AND THE STATEMENT ABOUT HOW IT WAS RECORDED SURVIVED INTO BOTH.
	// `SimplifyRoute` wrote through `codegen.Route` and deleted it.
	for _, got := range []Prepared{p, folded} {
		if ModeOf(got.Now) != Coordinates {
			t.Errorf("the rebuilt play states mode %q, so a tidy-up edited "+
				"a claim about somebody's recording out of their "+
				"file:\n%s", ModeOf(got.Now), got.Now)
		}
	}
	// AND THE CONTROL FOR THE FIRST CASE: the same demonstration stating NOTHING, under
	// the same environment and the same ordinary rebuild, IS capped — which is what
	// makes the case above a reading of the file rather than a build that never caps.
	plain, plainRt := aTypedRoutine(t, Unstated)
	q, err := plain.Prepare(plainRt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing the unstated play: %v", err)
	}
	if strings.Contains(q.Now, "Sleep with 3000") {
		t.Errorf("a play stating no mode kept its uncapped pause under "+
			"$MARCO_CV=on, so this build does not cap at all and the case "+
			"above establishes nothing:\n%s", q.Now)
	}
}

// AND PREPARING A SIMPLIFIED VERSION WRITES NOTHING EITHER.
//
// The same claim as the other choice's, said about this one, because "nothing was written" is a
// property of the flow and a second preparation path would have been the obvious place to lose it.
func TestPreparingASimplifiedVersionWritesNothing(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	before := theWholeLibrary(t, d)

	p, err := d.Prepare(rt, RebuildSimplifyTyping)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !p.Changes() {
		t.Fatal("the simplified rebuild is identical to the saved play, so a " +
			"preparation that wrote its own result would be invisible here")
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("preparing a simplified version changed the library: %s", why)
	}
	// AND REPLACING WITH IT WRITES EXACTLY WHAT WAS REVIEWED.
	if err := d.Replace(p); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != p.Now {
		t.Errorf("the file holds something other than the simplified version that was "+
			"reviewed:\n--- on disk ---\n%s\n--- reviewed ---\n%s", raw, p.Now)
	}
}

// AND A REBUILD THIS BUILD CANNOT NAME IS NOT QUIETLY THE GENTLER ONE.
//
// A caller asking for rules nobody here can name is a caller about to show somebody a review
// labelled as something it is not. Defaulting would be the worst of the two answers: the review
// would be of the ordinary rebuild under the simplified one's name, with its timing caution
// attached to a routine whose timing did not change.
func TestARebuildThisBuildCannotNameIsRefused(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	for _, kind := range []Rebuild{"", "aggressive", "Simplify Typing Pauses"} {
		if _, err := d.Prepare(rt, kind); err == nil {
			t.Errorf("a preparation under %q was accepted", kind)
		}
	}
	// AND THE TWO IT CAN NAME ARE BOTH ACCEPTED, or the refusal above is satisfied by a
	// function that refuses everything.
	for _, kind := range Offered() {
		if _, err := d.Prepare(rt, kind); err != nil {
			t.Errorf("a preparation under %q was refused: %v", kind, err)
		}
	}
}

// ── THE COMMAND WILL NOT CHOOSE BETWEEN NAMESAKES ─────────────────────────────

// SIMPLIFYING A NAME TWO ROUTINES ANSWER TO IS REFUSED, AND NEITHER IS TOUCHED.
//
// `Registry.Resolve` is built for PERFORMING: it tries the foreground application's routes, then
// ANY other application's focus route with that slug. Reaching into another application is right
// for "do the thing I named", because a focus route brings its application forward. It is wrong
// for "rewrite the thing I named": `marco simplify` already resolved exactly rather than by fuzzy
// match, because a guess that rewrites a file destroys somebody's work — and that guard was about
// the MATCHING, not the SELECTION. An exact slug shared by two routines is still two routines.
func TestSimplifyingANamesakeIsRefusedAndNothingIsRewritten(t *testing.T) {
	d, first := aTypedRoutine(t, Coordinates)
	// THE SAME NAME, SAVED FOR ANOTHER APPLICATION — the shape `Resolve` would reach
	// into. Focus-scoped, which is the case it reaches for.
	dr := d.DraftOf("type marco", aTypedDemonstration(), "discord", "a test", Coordinates)
	second, err := d.Save(dr, ScopeAppAnywhere, "discord")
	if err != nil {
		t.Fatalf("saving the namesake: %v", err)
	}
	before := theWholeLibrary(t, d)
	out := &bytes.Buffer{}
	d.Out = out
	d.In = strings.NewReader("y\n") // a yes, so a refusal cannot be mistaken for a no

	err = d.SimplifyRoute("type marco")
	if err == nil {
		t.Fatal("a command that rewrites a file chose between two plays of one name")
	}
	for _, says := range []string{"2 plays", "will not choose"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}
	// AND IT SAYS WHERE EACH ONE IS, which is the only thing that lets somebody pick.
	for _, where := range []string{"editor", "discord"} {
		if !strings.Contains(err.Error(), where) {
			t.Errorf("the refusal does not name %q: %v", where, err)
		}
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("the refused command changed the library: %s", why)
	}
	// AND WITH ONE OF THEM GONE IT PROCEEDS, so the refusal is about the ambiguity and
	// not about the name. The line it prints names the place, because a name is what the
	// person typed and a routine is a place.
	if err := d.Reg.Delete(second); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	d.In = strings.NewReader("y\n")
	if err := d.SimplifyRoute("type marco"); err != nil {
		t.Fatalf("simplifying the only play of that name: %v", err)
	}
	if !strings.Contains(out.String(), "editor, only while it is in front") {
		t.Errorf("the command did not say which play it was about before asking:"+
			"\n%s", out.String())
	}
	raw, _ := os.ReadFile(d.Reg.Path(first))
	if !strings.Contains(string(raw), `do OS's Type with "marco"`) {
		t.Errorf("the simplified play was not written:\n%s", raw)
	}
}

// AND A NO LEAVES IT ALONE, with the preparation already made.
//
// The confirmation is the whole of what `SimplifySaves` skips, so this is the assertion that says
// the prompt still decides anything at all.
func TestSimplifyingAndSayingNoLeavesTheRoutineAlone(t *testing.T) {
	d, _ := aTypedRoutine(t, Coordinates)
	before := theWholeLibrary(t, d)
	out := &bytes.Buffer{}
	d.Out, d.In = out, strings.NewReader("n\n")

	if err := d.SimplifyRoute("type marco"); err != nil {
		t.Fatalf("simplifying: %v", err)
	}
	if !strings.Contains(out.String(), "Left unchanged") {
		t.Errorf("the command did not say it left the play alone:\n%s", out.String())
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("a declined simplify changed the library: %s", why)
	}
	// AND THE CAUTION WAS ON SCREEN BEFORE THE QUESTION, which is the point of it: a
	// person deciding needs to know that folding the typing removes the pauses in it.
	said := out.String()
	if !strings.Contains(said, "quarter of a second") {
		t.Errorf("the command asked without saying what simplifying changes:\n%s", said)
	}
	if strings.Index(said, "quarter of a second") > strings.Index(said, "[y]es") {
		t.Errorf("the caution came after the question:\n%s", said)
	}
}

// AND A CALLER THAT SAID IT CANNOT SHOW A PREVIEW STILL GOES THROUGH THE CHECKED REPLACEMENT.
//
// `SimplifySaves` skips the QUESTION and nothing else. It is a statement by whoever built the
// `Deps` — not evidence of who the caller is, which an environment variable cannot establish —
// and an unattended save must still be unable to write over anything the review path protects.
//
// # Shown through a check only `Replace` makes
//
// A conflicting anchor image. Nothing else in this command looks at the images, so an unattended
// save refusing on one is an unattended save that went through `Deps.Replace` — and with it the
// digests, the token and the atomic write, which have their own tests. The first version of this
// tried to change the file mid-flight through `afterClassifying`, which fires INSIDE `Replace`
// after the digests have already been read: a seam in the wrong gap, asserting nothing. It failed
// and said so, which is what it was for.
func TestASaveWithNoPreviewStillGoesThroughTheCheckedReplacement(t *testing.T) {
	d, rt, image := anAnchoredRoutine(t)
	statingNoGeneration(t, d, rt)
	d.SimplifySaves = true
	d.Out = &bytes.Buffer{}
	const byHand = "a picture somebody else put there"
	if err := os.WriteFile(image, []byte(byHand), 0o644); err != nil {
		t.Fatal(err)
	}
	was, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}

	err = d.SimplifyRoute("anchored thing")
	if err == nil {
		t.Fatal("an unattended save wrote over an image it did not put there, so it " +
			"is not going through the checked replacement")
	}
	if !strings.Contains(err.Error(), "not the one the rebuilt version needs") {
		t.Errorf("the refusal is %q", err)
	}
	if raw, _ := os.ReadFile(image); string(raw) != byHand {
		t.Errorf("the refused save overwrote the image: %q", string(raw))
	}
	if raw, _ := os.ReadFile(d.Reg.Path(rt)); string(raw) != string(was) {
		t.Errorf("the refused save wrote the source anyway:\n%s", raw)
	}
	// AND WITHOUT THE INTERFERENCE IT SAVES WITHOUT ASKING, which is what the option is
	// for. No reader is given, so a prompt would read EOF rather than pass.
	d2, rt2 := aTypedRoutine(t, Coordinates)
	d2.SimplifySaves = true
	d2.Out = &bytes.Buffer{}
	if err := d2.SimplifyRoute("type marco"); err != nil {
		t.Fatalf("an unattended simplify was refused: %v", err)
	}
	if raw, _ := os.ReadFile(d2.Reg.Path(rt2)); !strings.Contains(string(raw),
		`do OS's Type with "marco"`) {
		t.Errorf("the unattended save wrote nothing:\n%s", raw)
	}
	// AND IT NEVER ASKED. A prompt skipped is the whole of what the option does, and a
	// caller that cannot answer one must not be given one.
	if strings.Contains(d2.Out.(*bytes.Buffer).String(), "[y]es") {
		t.Errorf("the unattended save asked a question nobody could answer:\n%s",
			d2.Out.(*bytes.Buffer).String())
	}
}
