package main

// THE BACKEND BEHIND THE REVIEW BUTTON, THROUGH THE REAL SAVE PATH.
//
// The page test next door renders the window against a stub, which is right for what it asks —
// whether the branches in the page take. This asks the other half: whether the `Acts` methods the
// page calls refuse what they must, with a real library, a real save, and a real demonstration
// beside the routine.
//
// Both are needed and neither substitutes for the other. A greyed-out button proves nothing about
// a backend that would have obliged, and a refusing backend proves nothing about a button that
// offers it.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aSavedRoutineInStudio is one routine saved through Studio's own path, with its demonstration.
func aSavedRoutineInStudio(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a, _ := aStudioRecording(t, "a thing")
	if why := a.SaveDraft(string(orchestrator.ScopeAppAnywhere), ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines after one save", len(rs))
	}
	return a, rs[0]
}

// A ROUTINE STUDIO JUST SAVED READS AS THE CURRENT GENERATION, AND SAYS SO.
//
// Through the real listing, so the card's three fields come off a file this build actually wrote
// rather than off a fixture.
func TestTheListingSaysWhichGenerationWroteARoutine(t *testing.T) {
	_, r := aSavedRoutineInStudio(t)

	if r.Generation != string(orchestrator.GenerationCurrent) {
		t.Errorf("a routine Studio just saved is listed as generation %q", r.Generation)
	}
	if r.Declared == "" {
		t.Error("the listing does not carry what the file itself declares")
	}
	if !strings.Contains(r.Wrote, "generation this build writes") {
		t.Errorf("the sentence is %q", r.Wrote)
	}
	if !r.Rebuildable || !r.Offer {
		t.Errorf("a routine with its demonstration beside it is not offerable "+
			"(rebuildable=%v offer=%v)", r.Rebuildable, r.Offer)
	}
	// AND THE CONTRACT IS A SEPARATE FACT, which is the whole point of the stamp being
	// provenance. Both are listed and neither is read off the other.
	if r.Contract == "" {
		t.Error("the listing stopped saying what its presses promise")
	}
}

// AND A ROUTINE WHOSE DEMONSTRATION IS GONE IS LISTED AS NOT OFFERABLE, WITH THE REASON.
func TestARoutineWithNoDemonstrationIsListedAsNotOfferable(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, ok := a.routineWithID(r.ID)
	if !ok {
		t.Fatal("the routine Studio just saved cannot be found by its own identity")
	}
	route := rt.route()
	if err := os.Remove(a.reg().RecordingPath(route)); err != nil {
		t.Fatal(err)
	}
	again := a.Routines()[0]
	if again.Rebuildable || again.Offer {
		t.Errorf("a routine with no demonstration is offered a rebuilt version "+
			"(rebuildable=%v offer=%v)", again.Rebuildable, again.Offer)
	}
	// AND THE SENTENCE SAYS WHY THE BUTTON IS OFF. A current routine with no
	// demonstration is nothing to worry about today, and the greyed control still
	// needs explaining — an unexplained disabled button is a dead end.
	if !strings.Contains(again.Wrote, "no longer stored beside it") {
		t.Errorf("the sentence does not say why there is nothing to rebuild: %q",
			again.Wrote)
	}
	// AND REVIEWING IT IS REFUSED RATHER THAN FAILING LATER.
	v := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if v.Why == "" {
		t.Fatalf("a routine with no demonstration was reviewed anyway: %+v", v)
	}
	if v.Now != "" {
		t.Error("a refused review carries a rebuilt source")
	}
}

// A ROUTINE FROM A LATER BUILD IS NOT OFFERED A REVIEW, AND IS LEFT EXACTLY AS IT IS.
//
// Two claims in one, because the second is what makes the first acceptable: withholding the offer
// is the whole of what happens. The routine stays listed, stays runnable, and its source is
// untouched.
func TestARoutineFromALaterBuildIsNotOfferedAReviewAndIsUntouched(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, _ := a.routineWithID(r.ID)
	route := rt.route()
	path := a.reg().Path(route)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ahead := strings.Replace(string(raw), "// Generation: "+r.Declared+".",
		"// Generation: 97.", 1)
	if ahead == string(raw) {
		t.Fatalf("the routine does not state its generation where this expects:\n%s",
			raw)
	}
	if err := os.WriteFile(path, []byte(ahead), 0o644); err != nil {
		t.Fatal(err)
	}
	later := a.Routines()[0]
	if later.Generation != string(orchestrator.GenerationUnknown) {
		t.Errorf("a routine stating generation 97 is listed as %q", later.Generation)
	}
	if later.Declared != "97" {
		t.Errorf("the listing declares %q and the file says 97 — which is the only "+
			"fact in it that identifies the routine", later.Declared)
	}
	if later.Offer {
		t.Error("a routine whose generation this build cannot explain is offered a " +
			"rebuilt version")
	}
	// AND IT IS STILL A ROUTINE. Nothing is refused over a comment: a build that would
	// not list or run a forward-dated file would make every one of them unusable.
	if !later.Runnable {
		t.Errorf("a routine from a later build is listed as unable to run: %q",
			later.Why)
	}
	// AND REVIEWING IT IS DECLINED WITH THE CARD'S OWN SENTENCE, so the greyed button
	// and the refusal agree.
	v := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if v.Why != later.Wrote {
		t.Errorf("the refusal says %q and the card says %q, so a person pressing "+
			"past the greyed button is told something else", v.Why, later.Wrote)
	}
	if v.Now != "" || v.Replaceable {
		t.Errorf("a declined review carries a version to replace with: %+v", v)
	}
	if now, _ := os.ReadFile(path); string(now) != ahead {
		t.Errorf("the routine was rewritten by being looked at:\n%s", now)
	}
}

// NOTHING IS REPLACED WITHOUT A REVIEW IN HAND.
//
// The flow's one load-bearing rule at this layer: `ReplaceWithRebuilt` does not build anything.
// A version that was never on the screen is a version nobody approved, and a method that
// helpfully built one would turn the review into decoration.
func TestNothingIsReplacedWithoutAReviewInHand(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, _ := a.routineWithID(r.ID)
	route := rt.route()
	was, err := os.ReadFile(a.reg().Path(route))
	if err != nil {
		t.Fatal(err)
	}
	// A TOKEN NOTHING PREPARED, which is what a page holds when it has no review: the
	// routine's own identity is the obvious wrong thing to send, so it is the one sent.
	why := a.ReplaceWithRebuilt(r.ID)
	if why == "" {
		t.Fatal("a routine was replaced without anybody reviewing a version of it")
	}
	if !strings.Contains(why, "review it again") {
		t.Errorf("the refusal is %q and does not say what to do", why)
	}
	if now, _ := os.ReadFile(a.reg().Path(route)); string(now) != string(was) {
		t.Error("the refused replacement wrote to the routine anyway")
	}
	// AND WITH ONE IN HAND IT GOES THROUGH, or the rule above is satisfied by a method
	// that refuses everything. The routine is current, so the replacement is its own
	// source — which is exactly what makes it checkable: byte for byte.
	v := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if v.Why != "" {
		t.Fatalf("reviewing refused: %s", v.Why)
	}
	if v.Token == "" {
		t.Fatal("the review carries no token, so nothing can say which review a " +
			"replacement is of")
	}
	if why := a.ReplaceWithRebuilt(v.Token); why != "" {
		t.Fatalf("replacing after a review refused: %s", why)
	}
	if now, _ := os.ReadFile(a.reg().Path(route)); string(now) != v.Now {
		t.Errorf("the file holds something other than the version that was reviewed:"+
			"\n--- on disk ---\n%s\n--- reviewed ---\n%s", now, v.Now)
	}
	// AND THE REVIEW IS SPENT. A second press of a button whose version has already
	// been written is not a second replacement.
	if why := a.ReplaceWithRebuilt(v.Token); why == "" {
		t.Error("the same review replaced the routine twice")
	}
}

// AND CANCELLING LETS GO OF THE VERSION, SO A LATER PRESS CANNOT WRITE IT.
//
// Cancel has nothing to undo — preparation wrote nothing — so what it does is let go. If it did
// not, a person who dismissed a review could still have it written by a stale button press, which
// is a replacement they explicitly declined.
func TestCancellingAReviewLetsGoOfTheVersion(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	v := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if v.Why != "" {
		t.Fatalf("reviewing refused: %s", v.Why)
	}
	a.CancelReview(v.Token)
	why := a.ReplaceWithRebuilt(v.Token)
	if why == "" {
		t.Fatal("a review that was cancelled was still written, so declining a " +
			"replacement does not decline it")
	}
	if !strings.Contains(why, "review it again") {
		t.Errorf("the refusal is %q", why)
	}
}

// AND A REVIEW IS OF THE ROUTINE THAT WAS ASKED ABOUT, NOT WHICHEVER ANSWERED FIRST.
//
// Three routines saved for one application under one scope, so the only thing telling them apart
// is the identity. The version of this that looked a routine up by name answered with whichever
// row came first — and here that would mean reviewing one routine and replacing another.
func TestAReviewIsOfTheRoutineThatWasAskedAbout(t *testing.T) {
	a, rs := aLibraryOfThreeRoutines(t)
	want := rs[2]

	v := a.ReviewRebuilt(want.ID, string(orchestrator.RebuildCurrent))
	if v.Why != "" {
		t.Fatalf("reviewing %q refused: %s", want.Name, v.Why)
	}
	if v.ID != want.ID || v.Name != want.Name {
		t.Errorf("asked to review %q (%s) and got %q (%s)", want.Name, want.ID,
			v.Name, v.ID)
	}
	// AND THE SOURCE IN IT IS THAT ROUTINE'S. The names are in the generated source,
	// so a review of the wrong routine shows the wrong actor.
	if !strings.Contains(v.Was, "CThing") {
		t.Errorf("the review shows a source that is not the chosen routine's:\n%s",
			v.Was)
	}
	if why := a.ReplaceWithRebuilt(v.Token); why != "" {
		t.Fatalf("replacing refused: %s", why)
	}
	// AND THE OTHER TWO ARE UNTOUCHED.
	for _, other := range rs[:2] {
		rt, _ := a.routineWithID(other.ID)
		raw, err := os.ReadFile(a.reg().Path(rt.route()))
		if err != nil {
			t.Fatalf("%q is no longer where it was: %v", other.Name, err)
		}
		if string(raw) != other.Source {
			t.Errorf("%q was rewritten by a review of another routine", other.Name)
		}
	}
}

// AND A REVIEW IS ADDRESSED BY IDENTITY, WHICH IS THE ONLY WAY TO REACH ONE OF TWO SAME-NAMED
// ROUTINES.
//
// One name saved for two applications is two routines, and this is where getting it wrong is
// worst: a lookup that took the first row answering to a name would show one routine's source,
// have somebody approve it, and replace the OTHER one's file — with a success message naming the
// routine they were looking at. Every part of that is correct except the subject.
func TestAReviewOfTwoSameNamedRoutinesIsOfTheOneAddressed(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	a.capture = nil
	aSecondRoutineCalled(t, a, "ark graphics", "discord", "app-anywhere")

	const second = "discord/focus/ark-graphics"
	var first Routine
	for _, r := range a.Routines() {
		if r.ID == "arkascended/focus/ark-graphics" {
			first = r
		}
	}
	if first.ID == "" {
		t.Fatal("the first routine is not in the listing, so this is not the case " +
			"it says it is")
	}
	// AND A NAME IS NOT AN ADDRESS. Asked for by the name both of them answer to, the
	// review has to REFUSE rather than choose one — because choosing is how a person
	// comes to approve one routine's source and have another one's file replaced, with a
	// message naming the one they were looking at. A lookup widened to accept a name is
	// invisible until something passes one, which is why something here does.
	for _, loose := range []string{"ark graphics", "ark-graphics", "Ark Graphics"} {
		v := a.ReviewRebuilt(loose, string(orchestrator.RebuildCurrent))
		if v.Why == "" {
			t.Errorf("a review asked for by %q answered with %q rather than "+
				"refusing an address that names two routines", loose, v.ID)
		}
		if why := a.ReplaceWithRebuilt(loose); why == "" {
			t.Errorf("a replacement asked for by %q went through", loose)
		}
	}
	v := a.ReviewRebuilt(second, string(orchestrator.RebuildCurrent))
	if v.Why != "" {
		t.Fatalf("reviewing %s refused: %s", second, v.Why)
	}
	if v.ID != second {
		t.Errorf("asked to review %s and got %s", second, v.ID)
	}
	if why := a.ReplaceWithRebuilt(v.Token); why != "" {
		t.Fatalf("replacing %s refused: %s", second, why)
	}
	// AND THE OTHER ROUTINE OF THAT NAME IS UNTOUCHED, read off its own file.
	raw, err := os.ReadFile(a.reg().Path(first.route()))
	if err != nil {
		t.Fatalf("the first routine is no longer where it was: %v", err)
	}
	if string(raw) != first.Source {
		t.Errorf("replacing %s rewrote %s, which shares its name:\n%s", second,
			first.ID, raw)
	}
}

// TWO REVIEWS OF ONE ROUTINE ARE TWO REVIEWS, AND REPLACE WRITES THE ONE IT WAS GIVEN.
//
// The window held one preparation per routine, so the second review overwrote the first — and
// these calls are asynchronous, so the page can be displaying A's two sources while the window
// holds B. Pressing Replace wrote B: every check inside `Deps.Replace` passed, the success named
// the routine the person was looking at, and the bytes were ones they never saw.
//
// Both orders are exercised, because the defect is about which one ARRIVED last rather than which
// was asked for first.
func TestTwoReviewsOfOneRoutineAreTwoReviews(t *testing.T) {
	a, r := aSavedRoutineInStudio(t)
	rt, _ := a.routineWithID(r.ID)
	route := rt.route()
	statingNoGenerationInStudio(t, a, route)

	first := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if first.Why != "" {
		t.Fatalf("the first review refused: %s", first.Why)
	}
	// THE ROUTINE CHANGES BETWEEN THE TWO, which is what makes them different reviews.
	edited := strings.Replace(first.Was, "// Auto-generated",
		"// a person edited this\n// Auto-generated", 1)
	if edited == first.Was {
		t.Fatal("the edit changed nothing, so the two reviews would be identical")
	}
	if err := os.WriteFile(a.reg().Path(route), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	second := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if second.Why != "" {
		t.Fatalf("the second review refused: %s", second.Why)
	}
	if first.Token == "" || second.Token == "" {
		t.Fatal("a review carries no token, so nothing can say which one a " +
			"replacement is of")
	}
	if first.Token == second.Token {
		t.Fatalf("two reviews of one routine built from different sources share the "+
			"token %q", first.Token)
	}
	if first.Was == second.Was {
		t.Fatal("both reviews show the same saved source, so the window is not " +
			"holding two different ones")
	}
	// THE OUT-OF-ORDER PRESS: the page is displaying the FIRST review — its response
	// arrived last — and Replace is handed that review's token. The window still holds
	// it, so it is reached rather than silently resolving to the newer one; and it is
	// then refused, because the routine it was built from is not the routine on disk.
	why := a.ReplaceWithRebuilt(first.Token)
	if why == "" {
		t.Fatal("the earlier review was written over a routine that had changed " +
			"since it was built")
	}
	if !strings.Contains(why, "saved again") {
		t.Errorf("the refusal is %q, which is not about the routine having moved on "+
			"— so the token resolved to some other review", why)
	}
	if raw, _ := os.ReadFile(a.reg().Path(route)); string(raw) != edited {
		t.Errorf("the refused replacement disturbed the routine:\n%s", raw)
	}
	// AND THE REVIEW THE PERSON WOULD SEE IF THEY LOOKED AGAIN GOES THROUGH, writing
	// its own source and not the other one's.
	if why := a.ReplaceWithRebuilt(second.Token); why != "" {
		t.Fatalf("the current review was refused: %s", why)
	}
	raw, err := os.ReadFile(a.reg().Path(route))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != second.Now {
		t.Errorf("the file holds something other than the review that was replaced "+
			"with:\n--- on disk ---\n%s\n--- the second review ---\n%s", raw,
			second.Now)
	}
	// AND THE TWO REVIEWS SHARE THEIR REBUILT SOURCE, BY CONSTRUCTION — both render the
	// same recording, and the edit between them changed the saved source rather than the
	// demonstration. So what tells them apart is what each was built FROM, which is
	// exactly why the token is derived from that too: the discriminating evidence is the
	// refusal above, since only the first review's source digest is stale and a token
	// that had resolved to the second would have gone through.
	if first.Now != second.Now {
		t.Errorf("the two reviews rebuilt different sources, so the refusal above is " +
			"not the only thing telling them apart and this test is weaker " +
			"than it reads")
	}
	// AND CANCELLING ONE DOES NOT THROW AWAY THE OTHER. Keyed by the routine, letting
	// go of one review let go of whichever the window held.
	third := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	fourth := a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
	if third.Token != fourth.Token {
		t.Fatalf("two reviews of an unchanged routine have different tokens (%q, %q), "+
			"so this cannot establish that cancelling one keeps the other",
			third.Token, fourth.Token)
	}
	a.CancelReview("some-review-nobody-made")
	if why := a.ReplaceWithRebuilt(third.Token); why != "" {
		t.Errorf("cancelling a review nobody made threw away one that was open: %s",
			why)
	}
}

// statingNoGenerationInStudio takes the generation line out of a saved routine.
//
// Which is what makes a replacement VISIBLE: a routine this build just wrote rebuilds byte for
// byte, so "the file holds the second review's source rather than the first's" would be true
// however the window behaved. The orchestrator's own tests carry the same note.
func statingNoGenerationInStudio(t *testing.T, a *Acts, rt routes.Route) {
	t.Helper()
	raw, err := os.ReadFile(a.reg().Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	var took int
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "// Generation: ") {
			took++
			continue
		}
		kept = append(kept, line)
	}
	if took == 0 {
		t.Fatal("the saved routine states no generation already, so a rebuild would " +
			"differ in nothing")
	}
	if err := os.WriteFile(a.reg().Path(rt), []byte(strings.Join(kept, "\n")),
		0o644); err != nil {
		t.Fatal(err)
	}
}

// TWO ROUTINES WITH IDENTICAL CONTENTS ARE TWO REVIEWS, AND REPLACING ONE LEAVES THE OTHER ALONE.
//
// Studio is where the slot lives, so this is the finding at the layer it was found in. A token
// derived from the content alone could not tell two such routines apart: the window would hold
// one, be handed the other's token, and write the right bytes to the wrong file with a success
// naming the routine the person was looking at.
//
// # Why the pair is seeded through the registry
//
// Because Studio REFUSES to save a guarded recording in-app, and it is right to: those presses
// check a window and an in-app routine brings none forward, so every one of them would refuse.
// That guard is about saving, not about what a library can contain — a pair like this arrives
// from `marco learn`, from an older build, or from somebody copying a folder — and the review
// flow has to be correct about a library it did not create.
func TestTwoRoutinesWithTheSameContentsAreTwoReviews(t *testing.T) {
	a, saved := aSavedRoutineInStudio(t)
	src, err := os.ReadFile(a.reg().Path(saved.route()))
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := a.reg().LoadRecording(saved.route())
	if !ok {
		t.Fatal("the saved routine has no demonstration beside it")
	}
	// THE SAME BYTES AND THE SAME DEMONSTRATION, IN-APP FOR TWO APPLICATIONS. Two files,
	// identical in every respect but which directory they are in — which is exactly the
	// identity `Routine.ID` carries and exactly what a content digest cannot see.
	ark := routes.Route{Slug: "shared-thing", App: "arkascended"}
	discord := routes.Route{Slug: "shared-thing", App: "discord"}
	for _, rt := range []routes.Route{ark, discord} {
		if err := a.reg().Save(rt, string(src)); err != nil {
			t.Fatal(err)
		}
		if err := a.reg().SaveRecording(rt, rec); err != nil {
			t.Fatal(err)
		}
	}
	var first, second Review
	for _, r := range a.Routines() {
		switch r.ID {
		case routeID(ark):
			first = a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
		case routeID(discord):
			second = a.ReviewRebuilt(r.ID, string(orchestrator.RebuildCurrent))
		}
	}
	if first.Why != "" || second.Why != "" {
		t.Fatalf("a review refused: %q / %q", first.Why, second.Why)
	}
	if first.Token == "" || second.Token == "" {
		t.Fatal("one of the two routines is not in the listing, so this establishes " +
			"nothing")
	}
	if first.Token == second.Token {
		t.Fatalf("two routines share the review token %q, so the window cannot tell "+
			"which file a replacement is for", first.Token)
	}
	// AND THE CASE IS REAL: the two show the same source and would write the same bytes.
	if first.Was != second.Was || first.Now != second.Now {
		t.Error("the two routines differ in what they show or what they would write, " +
			"so a token from the content alone would already have separated them")
	}
	otherWas, err := os.ReadFile(a.reg().Path(discord))
	if err != nil {
		t.Fatal(err)
	}
	if why := a.ReplaceWithRebuilt(first.Token); why != "" {
		t.Fatalf("replacing the first refused: %s", why)
	}
	if raw, _ := os.ReadFile(a.reg().Path(ark)); string(raw) != first.Now {
		t.Errorf("the replaced routine does not hold the version that was reviewed:"+
			"\n%s", raw)
	}
	if raw, _ := os.ReadFile(a.reg().Path(discord)); string(raw) != string(otherWas) {
		t.Errorf("replacing one routine rewrote the other:\n%s", raw)
	}
}

// AN INSTALLED MARCO SAYS WHAT IT IS MADE OF, PIECE BY PIECE, AND NOTICES A REPLACED FILE.
//
// `Build` is THIS window's revision and a bundle is five binaries — and the one piece that is
// not Go has no revision at all, because the accessibility provider is built by the .NET
// Framework compiler that ships with Windows. So a line showing only this window's revision
// would be a label standing in for an identity.
//
// The hashes are verified against disk rather than trusted, because a manifest is a claim and a
// claim about files anybody can replace is worth exactly what the check behind it is worth.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func TestAnInstalledMarcoSaysWhatItIsMadeOf(t *testing.T) {
	// A TREE IS NOT A BROKEN INSTALLATION. The test binary runs from a temporary
	// directory with no manifest beside it, which is what a development build looks
	// like — said as what it is rather than reported as a fault.
	made, problems := whatThisInstallationIsMadeOf()
	if len(problems) != 0 {
		t.Errorf("a build with no manifest beside it is reported as a problem: %+v",
			problems)
	}
	// THE MEANING, NOT THE OLD SENTENCE: it is a tree, and there is no manifest. The
	// copy pass shortened "not an installed Marco: these binaries were built in a
	// tree, so there is no manifest of what they are" to four words saying the same
	// two things, because this line sits under every page of the window.
	for _, says := range []string{"tree", "manifest"} {
		if !strings.Contains(made, says) {
			t.Errorf("a build with no manifest says %q, which does not "+
				"mention %q", made, says)
		}
	}

	// AND AN INSTALLATION SAYS WHAT IT IS. Written beside the test binary, which is
	// where the real one sits beside studio.exe.
	exe, err := os.Executable()
	if err != nil {
		t.Skip("this platform cannot say where the test binary is")
	}
	dir := filepath.Dir(exe)
	at := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(at); err == nil {
		t.Skip("something already put a manifest beside the test binary")
	}
	// THE MANIFEST DESCRIBES THE TEST BINARY ITSELF, because it is the one file
	// certain to be there and certain not to change under us.
	self := filepath.Base(exe)
	sum, err := hashOf(exe)
	if err != nil {
		t.Fatal(err)
	}
	write := func(files []bundled) {
		raw, err := json.Marshal(bundleManifest{Version: "abc1234-2026", Packed: "now",
			Files: files})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Remove(at) })

	write([]bundled{{File: self, SHA256: sum, Revision: "abc1234"}})
	made, problems = whatThisInstallationIsMadeOf()
	if len(problems) != 0 {
		t.Errorf("an installation whose files are what it says is reported as a "+
			"problem: %+v", problems)
	}
	for _, says := range []string{"Marco abc1234-2026", self, "abc1234"} {
		if !strings.Contains(made, says) {
			t.Errorf("the line does not say %q: %q", says, made)
		}
	}
	// A PIECE WITH NO REVISION IS SAID TO HAVE NONE, not shown as blank. That is the
	// accessibility provider's real state and the reason the manifest carries hashes.
	write([]bundled{{File: self, SHA256: sum}})
	made, _ = whatThisInstallationIsMadeOf()
	if !strings.Contains(made, "identified by its hash") {
		t.Errorf("a piece with no revision is reported as %q", made)
	}
	// AND A FILE THAT IS NOT WHAT THE MANIFEST SAYS IS A PROBLEM.
	write([]bundled{{File: self, SHA256: strings.Repeat("0", 64), Revision: "abc1234"}})
	_, problems = whatThisInstallationIsMadeOf()
	if len(problems) != 1 {
		t.Fatalf("a replaced file is reported as %d problem(s)", len(problems))
	}
	if !strings.Contains(problems[0].Says, "contents have changed") {
		t.Errorf("the problem says %q", problems[0].Says)
	}
	// AND A MANIFEST NAMING A FILE THAT IS NOT THERE IS A DIFFERENT PROBLEM AGAIN.
	write([]bundled{{File: "no-such-thing.exe", SHA256: sum, Revision: "abc1234"}})
	_, problems = whatThisInstallationIsMadeOf()
	if len(problems) != 1 || !strings.Contains(problems[0].Says, "not on disk") {
		t.Errorf("a missing file is reported as %+v", problems)
	}
	// AND THE SPEECH MODEL IS ONE ITEM, each of its files still verified (ADR-319).
	model := filepath.Join(dir, "plugins", "voice", "model")
	if err := os.MkdirAll(filepath.Join(model, "am"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(dir, "plugins")) })
	if err := os.WriteFile(filepath.Join(model, "am", "final.mdl"), []byte("weights"), 0o644); err != nil {
		t.Fatal(err)
	}
	weights, _ := hashOf(filepath.Join(model, "am", "final.mdl"))
	write([]bundled{{File: self, SHA256: sum, Revision: "abc1234"},
		{File: "plugins/voice/model/am/final.mdl", SHA256: weights},
		{File: "plugins/voice/model/conf/model.conf", SHA256: weights}})
	made, problems = whatThisInstallationIsMadeOf()
	if !strings.Contains(made, "the speech model (1 files)") || strings.Contains(made, "final.mdl") {
		t.Errorf("the model reads as %q", made)
	}
	if len(problems) != 1 || !strings.Contains(problems[0].Says, "model.conf") {
		t.Errorf("a model file missing from disk is reported as %+v", problems)
	}
	// AND BACK TO A GOOD ONE, so the two cases below are about the ENCODING and the
	// SHAPE of the manifest rather than about its contents.
	write([]bundled{{File: self, SHA256: sum, Revision: "abc1234"}})
	// AND A BYTE-ORDER MARK IS NOT A BROKEN MANIFEST. The first packed bundle had
	// one — `Set-Content -Encoding utf8` writes it — and reported "reinstall Marco"
	// about a manifest that was correct. Found by running the artifact from an
	// isolated directory, which no unit test could have done.
	raw, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at, append([]byte{0xEF, 0xBB, 0xBF}, raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	withBOM, problems := whatThisInstallationIsMadeOf()
	if len(problems) != 0 {
		t.Errorf("a manifest with a byte-order mark on it is reported as unreadable: "+
			"%+v", problems)
	}
	if !strings.Contains(withBOM, "Marco abc1234-2026") {
		t.Errorf("a manifest with a byte-order mark read as %q", withBOM)
	}
	// AND A MANIFEST THAT CANNOT BE READ SAYS SO RATHER THAN SAYING NOTHING.
	if err := os.WriteFile(at, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	made, problems = whatThisInstallationIsMadeOf()
	if made != "" || len(problems) != 1 ||
		!strings.Contains(problems[0].Says, "cannot be read") {
		t.Errorf("an unreadable manifest gives %q and %+v", made, problems)
	}
}

// A SECOND LIBRARY IS NAMED ONCE, AND NOTHING IS MOVED.
//
// ADR-252 promised this — "the window names any library-shaped directory beside its launch
// location that holds routines and says what to do about it; nothing is adopted or moved" — and
// `marcopaths.Elsewhere` was written for it and NEVER CALLED. A complete mechanism with no
// caller, which is this repository's recorded failure mode.
//
// It matters now because the alpha's installer created the case: until this slice the launcher
// pointed `MARCO_ROUTES` at its own installation directory, so every overlay-era bundle kept its
// routines under `%LOCALAPPDATA%\Marco\routes` — and with installation and data separated, that
// is a library nothing reads any more.
//
// ONCE. It was then named TWICE: a red problem card above Record and a banner over the routine
// list, 100 words between them for one fact. The readiness panel is for what is WRONG, and two
// libraries is not wrong — so the banner keeps the promise and the panel says nothing. This
// tests the case the deleted one tested, a library beside the EXECUTABLE, through the surviving
// path; `TestALibraryLeftBesideTheLaunchDirectoryIsNamed` is the other place it looks.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func TestASecondLibraryIsNamedOnceAndNothingIsMoved(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("this platform cannot say where the test binary is")
	}
	// A LIBRARY BESIDE THIS BINARY, which is where an old bundled install's routines
	// sit relative to `bin\`. One level up from the executable, which is what
	// `Elsewhere` climbs.
	beside := filepath.Join(filepath.Dir(exe), "..", "routes", "global")
	if err := os.MkdirAll(beside, 0o755); err != nil {
		t.Skipf("cannot write beside the test binary: %v", err)
	}
	at := filepath.Join(beside, "an-older-routine.marco")
	if _, err := os.Stat(at); err == nil {
		t.Skip("something is already there")
	}
	if err := os.WriteFile(at, []byte("use os.\n\nthe App is a script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(at) })

	// AND THE LIBRARY IN USE IS SOMEWHERE ELSE, which is the whole case.
	//
	// Through the ENVIRONMENT rather than by setting the field: `marcopaths.libraryIn`
	// excludes the library in use by asking `Routes()`, so a test that assigned
	// `a.routesDir` directly would have the window and the path rule disagreeing about
	// which library is in use — a disagreement production cannot have, since `newActs`
	// takes the field from `Routes()`.
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	// AND NOWHERE ELSE TO FIND ONE. `Library` looks beside the working directory as
	// well as beside the executable, and the working directory of this test is the
	// package — three levels under which sits this repository's own `routes`, when the
	// person running it has one. That would answer instead of the fixture, so the test
	// would read a different count on the author's machine than in a clean checkout.
	t.Chdir(t.TempDir())

	l := a.Library()
	if l.Elsewhere == "" || l.Says == "" {
		t.Fatalf("a second library holding a routine is not named: %+v", l)
	}
	// WHAT IT HAS TO CARRY: that the routines are there, where, that nothing moved,
	// and the one setting that would use them. A notice naming two libraries without
	// saying how to choose would leave somebody stuck.
	for _, says := range []string{"1 routine", "Nothing has been moved", "MARCO_ROUTES"} {
		if !strings.Contains(l.Says, says) {
			t.Errorf("it says %q and does not mention %q", l.Says, says)
		}
	}
	// AND IT IS SAID ONCE. The readiness panel is for what is wrong; two libraries is
	// not wrong, and it used to say so there as well, in a second sentence of its own.
	for _, p := range a.Ready().Problems {
		if strings.Contains(strings.ToLower(p.Says+" "+p.Do), "library") &&
			strings.Contains(p.Says+p.Do, l.Elsewhere) {
			t.Errorf("the other library is reported in the readiness panel as "+
				"well as over the routine list: %+v", p)
		}
	}
	// AND THE OTHER LIBRARY IS STILL THERE, UNTOUCHED, which is the claim that matters.
	if _, err := os.Stat(at); err != nil {
		t.Errorf("the routine in the other library is gone: %v", err)
	}
	// AND WHEN THE LIBRARY IN USE IS THAT ONE, THERE IS NOTHING TO SAY. Without this
	// the notice fires at everybody who has one library in the ordinary place.
	t.Setenv("MARCO_ROUTES", filepath.Join(filepath.Dir(exe), "..", "routes"))
	if l := newActs(t.TempDir()).Library(); l.Elsewhere != "" {
		t.Errorf("the library in use is reported as another library: %+v", l)
	}
}
