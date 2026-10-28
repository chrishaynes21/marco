package orchestrator

// PREPARING A VERSION WRITES NOTHING, AND REPLACING ONE REFUSES UNLESS THE WORLD IS STILL THE ONE
// THAT WAS REVIEWED.
//
// Four claims, and three of them are about what did NOT happen — which is why each is measured
// against the whole library rather than against the one file the flow is about. A preparation that
// wrote the new source "to a temporary place inside the store", or saved the recording again on
// its way past, would satisfy a test that only looked at the routine.
//
// The fourth is the one a person's trust rests on: what they approved is what is on disk,
// byte for byte, and nothing else moved.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aSavedRoutineWithItsRecording is a routine on disk with its own demonstration beside it, saved
// through the production path.
//
// `Unstated` rather than `Coordinates`, because that is the case this flow is FOR: a routine saved
// before a mode could be stated, whose recording still carries whatever the anchor stack captured.
func aSavedRoutineWithItsRecording(t *testing.T) (Deps, routes.Route) {
	t.Helper()
	_, dr := recordedWith(t, Unstated)
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !d.Reg.HasRecording(rt) {
		t.Fatal("the save left no demonstration beside the routine, so there is " +
			"nothing for a review to rebuild from")
	}
	return d, rt
}

// theWholeLibrary is every file under the store, with its contents digested.
//
// THE WHOLE OF IT, not the one routine: a preparation that wrote anywhere at all has written, and
// "it did not touch the file I was looking at" is a weaker claim than the one being made.
func theWholeLibrary(t *testing.T, d Deps) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(d.Reg.Dir, func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(d.Reg.Dir, path)
		out[rel] = routes.DigestOf(string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("reading the library: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("the library is empty, so nothing here could detect a write")
	}
	return out
}

// differsFrom names what changed between two readings of the library, or "" when nothing did.
func differsFrom(before, after map[string]string) string {
	var said []string
	for path, was := range before {
		switch now, there := after[path]; {
		case !there:
			said = append(said, path+" is gone")
		case now != was:
			said = append(said, path+" was rewritten")
		}
	}
	for path := range after {
		if _, there := before[path]; !there {
			said = append(said, path+" was created")
		}
	}
	sort.Strings(said)
	return strings.Join(said, ", ")
}

// statingNoGeneration takes the generation line out of a saved routine.
//
// WHICH IS WHAT MAKES A NO-WRITE TEST MEAN ANYTHING. A routine this build just wrote rebuilds
// BYTE FOR BYTE, so a `Prepare` that saved its own result would change nothing a digest could
// see — and the test would pass on a preparation that writes. Found by the mutation gate, which
// is what it is for.
//
// With the line gone the rebuild differs by exactly that line, so any write at all is visible.
func statingNoGeneration(t *testing.T, d Deps, rt routes.Route) {
	t.Helper()
	raw, err := os.ReadFile(d.Reg.Path(rt))
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
		t.Fatal("the saved routine states no generation already, so this build is " +
			"not writing one and a rebuild would differ in nothing")
	}
	if err := d.Reg.Save(rt, strings.Join(kept, "\n")); err != nil {
		t.Fatal(err)
	}
}

// PREPARING A VERSION, AND THEN CANCELLING, WRITES NOTHING AT ALL.
//
// Cancelling is not an operation: it is the absence of one, which is the whole reason preparation
// has to write nothing. A flow that staged the new source on disk and tidied up on cancel would
// leave somebody's routine replaced by whatever happened between the two — a crash, a kill, a
// window closed.
func TestPreparingAndCancellingAVersionWritesNothing(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	statingNoGeneration(t, d, rt)
	before := theWholeLibrary(t, d)

	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing a version: %v", err)
	}
	if p.Now == "" {
		t.Fatalf("the preparation produced no version to review (refused: %q), so "+
			"this proves nothing about what a preparation writes", p.Refused)
	}
	// AND THE REBUILT VERSION DIFFERS FROM THE SAVED ONE, which is what makes the
	// digest below able to see a write at all. See statingNoGeneration.
	if !p.Changes() {
		t.Fatal("the rebuilt version is identical to the saved one, so a " +
			"preparation that wrote its own result would be invisible here")
	}
	// AND THE REVIEW HAS SOMETHING TO SHOW: both sources in full, which is what is
	// reviewed. A preparation that returned a summary would have nothing a person could
	// check the replacement against.
	if p.Was == "" || p.Route != rt {
		t.Errorf("the preparation describes %+v rather than the routine it was asked "+
			"about", p.Route)
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("preparing a version for review changed the library: %s", why)
	}
	// CANCELLING: the `Prepared` is dropped and nothing else happens. Said as a test
	// rather than left implicit, because "nothing happens" is a claim about the design
	// and somebody could change it to a staged write without noticing this file.
	p = Prepared{}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("cancelling a review changed the library: %s", why)
	}
}

// A SUCCESSFUL REPLACEMENT WRITES EXACTLY WHAT WAS REVIEWED, AND LEAVES THE ROUTINE WHERE IT WAS.
//
// The claim the whole flow rests on, and it needs the two sources to DIFFER to mean anything: a
// routine this build just wrote rebuilds byte for byte, so `Replace` writing the old source
// instead of the new one would be invisible. The generation line comes out first, so the
// difference is real and known. Found by the mutation gate — the version of this test without it
// passed on a replacement that wrote the source it was meant to be replacing.
func TestAReplacementWritesExactlyTheSourceThatWasReviewed(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	statingNoGeneration(t, d, rt)
	path := d.Reg.Path(rt)
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !p.Changes() {
		t.Fatal("the rebuilt version is identical to the saved one, so this cannot " +
			"tell a replacement from leaving the file alone")
	}
	if err := d.Replace(p); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the routine is not where it was: %v", err)
	}
	if string(raw) != p.Now {
		t.Errorf("the file now holds something other than the version that was "+
			"reviewed:\n--- on disk ---\n%s\n--- reviewed ---\n%s", raw, p.Now)
	}
	if string(raw) == p.Was {
		t.Error("the file holds the source the replacement was supposed to replace")
	}
	// AND IT IS STILL THE SAME ROUTINE, IN THE SAME PLACE. A replacement that rescoped
	// would move the file — `routes/<app>/focus` to `routes/<app>/context` is a routine
	// that stops working from anywhere — and the person reviewed a source, not a move.
	if d.Reg.Path(rt) != path {
		t.Errorf("the routine now lives at %s and was at %s", d.Reg.Path(rt), path)
	}
	if !d.Reg.HasRecording(rt) {
		t.Error("the replacement took the demonstration with it, so the routine can " +
			"never be rebuilt again")
	}
	// AND REPLACING AGAIN IS A NO-OP THAT SAYS SO. The routine has now been written by
	// today's generator from its own recording, so rebuilding it again changes nothing
	// — and offering to replace a file with itself is an offer to do nothing.
	again, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing the replaced routine: %v", err)
	}
	if again.Changes() {
		t.Errorf("a routine this generator just wrote would be changed by being "+
			"rebuilt:\n--- saved ---\n%s\n--- rebuilt ---\n%s", again.Was,
			again.Now)
	}
	if again.WasGeneration.Reading != GenerationCurrent {
		t.Errorf("the replaced routine reads as %q", again.WasGeneration.Reading)
	}
}

// AND THE SCOPE A ROUTINE IS ALREADY UNDER IS READ OFF WHERE IT LIVES, FOR ALL THREE.
//
// Which is what keeps a replacement from rescoping. `Save` CHOOSES a scope; this flow is not
// choosing, and the only way not to choose is to read what is already true.
func TestTheScopeAReplacementUsesIsTheOneTheRoutineAlreadyHas(t *testing.T) {
	for _, c := range []struct {
		what string
		rt   routes.Route
		want Scope
	}{
		{"brings its application forward",
			routes.Route{Slug: "a", App: "ark", Focus: true}, ScopeAppAnywhere},
		{"works only while its application is in front",
			routes.Route{Slug: "a", App: "ark"}, ScopeApp},
		{"is about no application", routes.Route{Slug: "a"}, ScopeEverywhere},
	} {
		if got := ScopeAlreadyOf(c.rt); got != c.want {
			t.Errorf("a routine that %s reads as scope %q, want %q", c.what, got,
				c.want)
		}
	}
}

// A REPLACEMENT REFUSES WHEN THE ROUTINE HAS BEEN SAVED AGAIN SINCE THE REVIEW.
//
// Somebody edits the routine in another window while the review is open. Writing anyway would
// replace a source nobody looked at — invisibly, because the result would look exactly like an
// ordinary successful replacement. This is also the whole of the protection for a hand-edited
// routine: the edit is what the left-hand pane shows, so replacing it is a decision, and
// replacing it WITHOUT that pane being current is not one.
func TestAReplacementAfterTheRoutineChangedRefusesAndLeavesItAlone(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	const byHand = "// a person edited this\nuse os.\n\nthe App is a script.\n"
	if err := d.Reg.Save(rt, byHand); err != nil {
		t.Fatal(err)
	}
	err = d.Replace(p)
	if err == nil {
		t.Fatal("a routine that changed since the review was replaced anyway, so a " +
			"hand edit can be overwritten by a review of the file as it used to be")
	}
	if !strings.Contains(err.Error(), "saved again") {
		t.Errorf("the refusal is %q and does not say the routine was saved again", err)
	}
	raw, _ := os.ReadFile(d.Reg.Path(rt))
	if string(raw) != byHand {
		t.Errorf("the refusal did not leave the routine as it was; it now holds:\n%s",
			raw)
	}
	// AND THE WAY FORWARD IS A FRESH REVIEW, which the refusal says and which works:
	// the comparison is now against the routine as it stands.
	after, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("reviewing the edited routine: %v", err)
	}
	if after.Was != byHand {
		t.Errorf("a fresh review compares against %q rather than the routine as it "+
			"now stands", after.Was)
	}
}

// AND WHEN THE DEMONSTRATION HAS BEEN RECORDED AGAIN SINCE THE REVIEW.
//
// The other half, and the one that is easy to forget: the source is untouched, so a replacement
// that checked only the source would write a rendering of a recording that is no longer there —
// a version of somebody's routine built from a demonstration nobody reviewed.
func TestAReplacementAfterTheRecordingChangedRefusesAndLeavesItAlone(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	was, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	// RECORDED AGAIN: the same routine demonstrated a second time, which is a thing
	// Studio does and which leaves the source alone until a save.
	shorter, err := json.Marshal(aDemonstrationInAGame()[:2])
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Reg.SaveRecording(rt, shorter); err != nil {
		t.Fatal(err)
	}
	err = d.Replace(p)
	if err == nil {
		t.Fatal("a routine whose demonstration changed since the review was replaced " +
			"anyway, from the recording that is no longer there")
	}
	if !strings.Contains(err.Error(), "recorded again") {
		t.Errorf("the refusal is %q and does not say it was recorded again", err)
	}
	raw, _ := os.ReadFile(d.Reg.Path(rt))
	if string(raw) != string(was) {
		t.Errorf("the refusal did not leave the routine as it was; it now holds:\n%s",
			raw)
	}
}

// AND A ROUTINE WHOSE DEMONSTRATION IS GONE IS EXPLAINED AND NOT OFFERED.
//
// Both halves, because they live in different places on purpose. The explanation is owed and is
// `GenerationSentence`'s; the rebuild is impossible and is refused here. An offer that failed
// after somebody took it is the authorization dead-end this product has shipped before.
func TestARoutineWithNoDemonstrationIsExplainedAndCannotBeRebuilt(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	if err := os.Remove(d.Reg.RecordingPath(rt)); err != nil {
		t.Fatal(err)
	}
	// AND ITS GENERATION LINE TAKEN OUT, which is the case this is about: a routine
	// from before the line existed, whose demonstration is also gone. A routine
	// today's generator wrote needs no offer in the first place, so it would not
	// establish that the offer is withheld.
	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "// Generation: ") {
			kept = append(kept, line)
		}
	}
	if err := d.Reg.Save(rt, strings.Join(kept, "\n")); err != nil {
		t.Fatal(err)
	}
	before := theWholeLibrary(t, d)

	_, err = d.Prepare(rt)
	if err == nil {
		t.Fatal("a routine with no demonstration was prepared for review anyway")
	}
	if !strings.Contains(err.Error(), "no longer has the demonstration") {
		t.Errorf("the refusal is %q", err)
	}
	if !strings.Contains(err.Error(), "recording it again") {
		t.Errorf("the refusal leaves a person with nothing to do: %q", err)
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("a refused preparation changed the library: %s", why)
	}
	// AND THE EXPLANATION IS STILL AVAILABLE, off the source, with nothing offered.
	raw, _ := os.ReadFile(d.Reg.Path(rt))
	said := GenerationSentence(GenerationOf(string(raw)), d.Reg.HasRecording(rt))
	if !strings.Contains(said, "does not state which generation") {
		t.Errorf("the routine this is about reads as something else: %q", said)
	}
	if strings.Contains(said, "You can review") {
		t.Errorf("a routine with no demonstration is offered a review: %q", said)
	}
	if !strings.Contains(said, "no longer stored beside it") {
		t.Errorf("the sentence does not explain why there is nothing to rebuild: %q",
			said)
	}
}

// A REBUILD THAT TODAY'S RULES WOULD REFUSE IS A REVIEWABLE ANSWER, NOT AN ERROR.
//
// The third answer to "what would I get if I rebuilt this", and the one a person is most owed:
// today's generator refuses a recording containing a press that cannot be stopped at, which an
// older save wrote happily. So a routine on disk can be one whose own recording will not produce
// a routine any more — and the honest thing is to say so, in the generator's words, beside the
// source it would have replaced.
func TestARebuildTodaysRulesWouldRefuseIsShownAndCannotBeReplacedWith(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	was, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	// A DEMONSTRATION WITH A PICTURE IN IT. Which is what the anchor stack captured
	// before a mode could be stated, and what a press found by matching cannot be: a
	// press nobody can stop at, in a routine whose other press is checked.
	events := aDemonstrationInAGame()
	for i := range events {
		if events[i].Kind == recorder.EvClick && events[i].Down {
			events[i].Image = []byte("a captured patch of the screen")
			break
		}
	}
	raw, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Reg.SaveRecording(rt, raw); err != nil {
		t.Fatal(err)
	}
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("a recording today's rules refuse should be a reviewable answer, and "+
			"preparing it errored: %v", err)
	}
	if p.Refused == "" {
		t.Fatalf("a demonstration with a press that cannot be stopped at rebuilt "+
			"into a routine:\n%s", p.Now)
	}
	if !strings.Contains(p.Refused, "matching a picture") {
		t.Errorf("the refusal is %q and does not name what cannot be wrapped",
			p.Refused)
	}
	// AND THE SAVED SOURCE IS STILL SHOWN BESIDE IT, because the review is a comparison
	// and a person looking at a refusal still needs to see what they have.
	if p.Was != string(was) {
		t.Error("a refused review does not show the routine as it stands")
	}
	if p.Replaceable() || p.Changes() {
		t.Error("a review with nothing to write reports itself replaceable")
	}
	// AND REPLACING WITH IT IS REFUSED, NAMING THE REASON. Not silently ignored: a
	// button that did nothing would read as a save that worked.
	err = d.Replace(p)
	if err == nil {
		t.Fatal("a review that produced no routine was replaced with anyway")
	}
	if !strings.Contains(err.Error(), "matching a picture") {
		t.Errorf("the refusal to replace is %q and does not carry the reason", err)
	}
	if now, _ := os.ReadFile(d.Reg.Path(rt)); string(now) != string(was) {
		t.Errorf("the routine was changed by a replacement that refused:\n%s", now)
	}
}

// AND A REPLACEMENT NEEDS THE ROUTINE IT WAS PREPARED FROM.
//
// An empty `Prepared` is what a caller holds after a cancel, and what a bug would hand over after
// a failed preparation. Writing from one would be writing an empty routine over a working one.
func TestAReplacementFromNothingIsRefused(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	before := theWholeLibrary(t, d)
	if err := d.Replace(Prepared{}); err == nil {
		t.Fatal("an empty review was written to the library")
	}
	// AND ONE NAMING A ROUTINE WITH NO SOURCE TO WRITE IS REFUSED TOO.
	if err := d.Replace(Prepared{Route: rt, Name: "ark graphics"}); err == nil {
		t.Fatal("a review with no rebuilt version was written to the library")
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("a refused replacement changed the library: %s", why)
	}
}

// A REPLACEMENT KEEPS A ROUTINE'S SCOPE, INCLUDING THE SCOPE THAT CHANGES WHAT IS GENERATED.
//
// The case the `app-anywhere` tests above cannot reach. A routine saved to work only while its
// application is in front carries NO `Activate`, because it switches nothing — so a replacement
// that regenerated under the wrong scope would write a routine that brings an application
// forward, and the file would move from `<app>/context` to `<app>/focus`. Both halves are checked:
// where the file is, and what is in it.
//
// `Resave` rescopes on purpose and this must not, which is the whole reason the two are separate
// operations.
func TestAReplacementKeepsAScopeThatChangesWhatIsGenerated(t *testing.T) {
	_, dr := recordedWith(t, Unstated)
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	rt, err := d.Save(dr, ScopeApp, "arkascended")
	if err != nil {
		t.Fatalf("saving a context-scoped routine: %v", err)
	}
	path := d.Reg.Path(rt)
	was, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// THE ARRANGEMENT HELD: this routine switches nothing, which is what the
	// regeneration has to reproduce.
	if strings.Contains(string(was), "OS's Activate") {
		t.Fatalf("a context-scoped routine was generated with an Activate in it, so "+
			"this case does not establish what it says:\n%s", was)
	}
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if err := d.Replace(p); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the routine is no longer where it was: %v", err)
	}
	if strings.Contains(string(now), "OS's Activate") {
		t.Errorf("the replacement brings an application forward and the routine it "+
			"replaced switched nothing, so it was regenerated under a scope "+
			"nobody chose:\n%s", now)
	}
	if string(now) != string(was) {
		t.Errorf("a routine today's generator wrote was changed by being rebuilt "+
			"under its own scope:\n--- was ---\n%s\n--- now ---\n%s", was, now)
	}
	if rt.Focus {
		t.Error("the route's own identity gained focus")
	}
}

// AND THE REGISTRY'S REPLACE REFUSES A ROUTINE WITH NO NAME, LEAVING WHAT IS THERE.
//
// The one guard on the write itself. `Save` has the same one and for the same reason: a write
// keyed on an empty slug is a write to a directory, and finding out by watching it fail is
// finding out after the original is gone.
func TestTheRegistryRefusesToReplaceSomethingWithNoName(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{Slug: "a-thing", App: "ark", Focus: true}
	if err := reg.Save(rt, "use os.\n"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Replace(routes.Route{App: "ark", Focus: true}, "something else"); err == nil {
		t.Error("the registry replaced a routine with no name")
	}
	raw, err := os.ReadFile(reg.Path(rt))
	if err != nil || string(raw) != "use os.\n" {
		t.Errorf("the refused write disturbed the routine beside it: %q (%v)", raw, err)
	}
	// AND IT DOES REPLACE ONE THAT HAS A NAME, or the guard above is satisfied by a
	// function that refuses everything.
	if err := reg.Replace(rt, "// replaced\nuse os.\n"); err != nil {
		t.Fatalf("replacing a named routine: %v", err)
	}
	if raw, _ := os.ReadFile(reg.Path(rt)); string(raw) != "// replaced\nuse os.\n" {
		t.Errorf("the replacement wrote %q", raw)
	}
}

// ── TWO PREPARATIONS OF ONE ROUTINE ───────────────────────────────────────────

// EACH PREPARATION IS ITS OWN REVIEW, AND A REPLACEMENT CARRIES THAT REVIEW'S TOKEN.
//
// A window can have two preparations of one routine in flight. Held under the ROUTINE they
// overwrote each other — and the calls are asynchronous, so the page could be displaying A's two
// sources while the slot held B. Press Replace and B is written: every check below passes, the
// success names the routine the person was looking at, and the bytes are ones they never saw.
//
// So a token identifies the preparation, and it is derived from the content: the two files it was
// built from, what it would write, and why it would write nothing.
func TestTwoPreparationsOfOneRoutineAreTwoReviews(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	statingNoGeneration(t, d, rt)

	first, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("the first preparation: %v", err)
	}
	// THE ROUTINE CHANGES BETWEEN THE TWO, which is the case that makes them different
	// reviews rather than two copies of one: a hand edit, another window, a sync.
	edited := strings.Replace(first.Was, "// Auto-generated",
		"// a person edited this\n// Auto-generated", 1)
	if edited == first.Was {
		t.Fatal("the edit changed nothing, so the two preparations would be identical")
	}
	if err := d.Reg.Save(rt, edited); err != nil {
		t.Fatal(err)
	}
	second, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("the second preparation: %v", err)
	}

	if first.Token == "" || second.Token == "" {
		t.Fatal("a preparation carries no token, so nothing can say which review a " +
			"replacement is of")
	}
	if first.Token == second.Token {
		t.Fatalf("two preparations of one routine built from different sources share "+
			"the token %q, so a replacement cannot tell them apart", first.Token)
	}
	if first.Was == second.Was {
		t.Fatal("the two preparations show the same saved source, so this is not the " +
			"case it says it is")
	}
	// AND THE OUT-OF-ORDER CASE: the FIRST one is replaced with, after the second was
	// prepared. It must be refused, because the routine it was built from is not the
	// routine on disk — and the refusal says to review again rather than writing a
	// rendering of a file that has moved on.
	err = d.Replace(first)
	if err == nil {
		t.Fatal("the earlier preparation was written over a routine that had changed " +
			"since it was built")
	}
	if !strings.Contains(err.Error(), "saved again") {
		t.Errorf("the refusal is %q", err)
	}
	if raw, _ := os.ReadFile(d.Reg.Path(rt)); string(raw) != edited {
		t.Errorf("the refused replacement disturbed the routine:\n%s", raw)
	}
	// AND THE SECOND ONE, WHICH IS THE CURRENT REVIEW, GOES THROUGH.
	if err := d.Replace(second); err != nil {
		t.Fatalf("the current preparation was refused: %v", err)
	}
	if raw, _ := os.ReadFile(d.Reg.Path(rt)); string(raw) != second.Now {
		t.Errorf("the file holds something other than the reviewed version:\n%s", raw)
	}
}

// AND A PREPARATION WHOSE CONTENT WAS CHANGED AFTER IT WAS PREPARED NO LONGER MATCHES ITS TOKEN.
//
// The token is derived from the fields a replacement acts on, so a `Prepared` edited on its way
// back — by a caller, by a bug, by a struct assembled from parts of two — says so. Cheap, and it
// means the one thing a caller hands over cannot be quietly wrong.
func TestAPreparationThatWasAlteredDoesNotMatchItsToken(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	statingNoGeneration(t, d, rt)
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	was, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	tampered := p
	tampered.Now = "// something else entirely\nuse os.\n\nthe App is a script.\n"

	err = d.Replace(tampered)
	if err == nil {
		t.Fatal("a preparation whose source was swapped after it was prepared was " +
			"written anyway")
	}
	if !strings.Contains(err.Error(), "not the one that was prepared") {
		t.Errorf("the refusal is %q", err)
	}
	if raw, _ := os.ReadFile(d.Reg.Path(rt)); string(raw) != string(was) {
		t.Errorf("the refused replacement wrote to the routine:\n%s", raw)
	}
	// AND THE UNALTERED ONE STILL GOES THROUGH, or the check above is satisfied by a
	// `Replace` that refuses everything.
	if err := d.Replace(p); err != nil {
		t.Fatalf("the preparation as it was prepared was refused: %v", err)
	}
}

// ── ELIGIBILITY IS READ OFF THE SOURCE THIS PREPARED ──────────────────────────

// A ROUTINE THAT BECAME UNEXPLAINABLE BETWEEN THE LISTING AND THE PREPARATION IS REFUSED.
//
// Studio greys its button from a LISTING, which is one read of the library. `Prepare` is a second
// read, and between the two the file can be replaced with a version stating a generation this
// build cannot explain — by a hand edit, another window, a sync. Checking only the listing lets
// exactly that through the one restriction where this build refuses to say what a replacement
// would change.
//
// So the rule is applied where the bytes are. This is that interval: a routine that was eligible
// when it was listed, and is not when it is read.
func TestARoutineThatBecameUnexplainableBetweenTheReadsIsRefused(t *testing.T) {
	d, rt := aSavedRoutineWithItsRecording(t)
	// AS IT WAS LISTED: eligible, which is what makes the refusal below about the
	// interval rather than about a routine that was never offerable.
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !GenerationOf(string(raw)).Offerable() {
		t.Fatal("the seeded routine was never offerable, so this establishes nothing " +
			"about the gap between two reads")
	}
	// AND THEN THE FILE MOVES ON, to a generation this build has no account of.
	ahead := strings.Replace(string(raw), "// Generation: 1.", "// Generation: 97.", 1)
	if ahead == string(raw) {
		t.Fatalf("the routine does not state its generation where this expects:\n%s",
			raw)
	}
	if err := d.Reg.Save(rt, ahead); err != nil {
		t.Fatal(err)
	}
	before := theWholeLibrary(t, d)

	_, err = d.Prepare(rt)
	if err == nil {
		t.Fatal("a routine stating a generation this build cannot explain was " +
			"prepared for review anyway, so the restriction is only as good as " +
			"whatever a caller looked at earlier")
	}
	if !strings.Contains(err.Error(), "does not recognise") {
		t.Errorf("the refusal is %q and does not say what it could not read", err)
	}
	if why := differsFrom(before, theWholeLibrary(t, d)); why != "" {
		t.Errorf("the refused preparation changed the library: %s", why)
	}
}

// ── THE PICTURES ARE NOT ADDITIVE ─────────────────────────────────────────────

// anAnchoredRoutine is a saved routine with an anchor image beside it.
//
// Which is what makes the next test possible: a rebuild produces the SAME image path, because the
// name is made from the routine's — `<base>-anchor-1.png`. Not guarded, because an anchored press
// cannot be stopped at and a guarded routine containing one is refused at Save; this is the shape
// an anchored routine really has.
func anAnchoredRoutine(t *testing.T) (Deps, routes.Route, string) {
	t.Helper()
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, Image: []byte("the first picture"),
			T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
	}
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("anchored thing", events, "arkascended", "a test", Unstated)
	if dr == nil || len(dr.Steps) == 0 {
		t.Fatal("the anchored demonstration produced no draft")
	}
	rt, err := d.Save(dr, ScopeApp, "arkascended")
	if err != nil {
		t.Fatalf("saving the anchored routine: %v", err)
	}
	if _, ok := d.Reg.LoadRecording(rt); !ok {
		t.Fatal("the save left no demonstration beside the anchored routine")
	}
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing the anchored routine: %v", err)
	}
	if len(p.assets) != 1 {
		t.Fatalf("the rebuild produced %d image(s), so this test has nothing to be "+
			"about", len(p.assets))
	}
	for path := range p.assets {
		return d, rt, path
	}
	return d, rt, ""
}

// A REPLACEMENT THAT WOULD OVERWRITE A DIFFERENT IMAGE IS REFUSED, AND WRITES NOTHING AT ALL.
//
// The images are NOT additive, which this used to assume. They are named after the routine, so a
// rebuild produces the same paths and `WriteAssets` overwrites what is there. Writing them first
// and calling that safe was wrong in the one direction that matters: write the new images, fail to
// write the source, and the OLD source is still on disk pointing at paths that now hold different
// pictures — a routine that presses what a different image matched, having been left alone.
//
// So the source AND the images both have to be as they were, which is what this asserts.
func TestAReplacementThatWouldOverwriteADifferentImageIsRefused(t *testing.T) {
	d, rt, image := anAnchoredRoutine(t)
	statingNoGeneration(t, d, rt)
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if !p.Changes() {
		t.Fatal("the rebuilt source is identical to the saved one, so 'the source " +
			"was not replaced' would be true however this behaved")
	}
	// THE IMAGE BESIDE IT IS NOT THE ONE THE REBUILD WOULD WRITE. A real state: the
	// routine was recorded again and only its pictures were kept, or somebody replaced
	// one by hand.
	const byHand = "a different picture entirely"
	if err := os.WriteFile(image, []byte(byHand), 0o644); err != nil {
		t.Fatal(err)
	}
	was, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}

	err = d.Replace(p)
	if err == nil {
		t.Fatal("a replacement that would overwrite a different image went ahead")
	}
	if !strings.Contains(err.Error(), "not the one the rebuilt version needs") {
		t.Errorf("the refusal is %q and does not say what it found", err)
	}
	if !strings.Contains(err.Error(), image) {
		t.Errorf("the refusal does not name the image: %q", err)
	}
	// THE SOURCE IS AS IT WAS.
	if raw, _ := os.ReadFile(d.Reg.Path(rt)); string(raw) != string(was) {
		t.Errorf("the refused replacement wrote the source anyway:\n%s", raw)
	}
	// AND SO ARE THE IMAGES, which is the half this finding was about: a routine left
	// alone beside pictures that changed is a routine that presses something else.
	if raw, _ := os.ReadFile(image); string(raw) != byHand {
		t.Errorf("the refused replacement overwrote the image: %q", raw)
	}
}

// AND AN IMAGE THAT IS ALREADY THE ONE THE REBUILD NEEDS IS NO CONFLICT.
//
// Without this the rule refuses every re-review of an anchored routine, which would make the flow
// useless for exactly the routines that have images. Writing a file the bytes it already holds
// changes nothing that runs.
func TestAnImageThatIsAlreadyTheRightOneIsNoConflict(t *testing.T) {
	d, rt, image := anAnchoredRoutine(t)
	statingNoGeneration(t, d, rt)
	p, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing: %v", err)
	}
	// THE IMAGE ON DISK IS THE ONE THE REBUILD WOULD WRITE, which is the ordinary case:
	// the same recording renders the same picture.
	if raw, rerr := os.ReadFile(image); rerr != nil || !bytes.Equal(raw, p.assets[image]) {
		t.Fatalf("the image beside the routine is not the one its own rebuild "+
			"produces, so this is not the ordinary case (%v)", rerr)
	}
	if err := d.Replace(p); err != nil {
		t.Fatalf("a replacement whose images are already right was refused: %v", err)
	}
	if raw, _ := os.ReadFile(d.Reg.Path(rt)); string(raw) != p.Now {
		t.Errorf("the file holds something other than the reviewed version:\n%s", raw)
	}
	// AND A PATH THAT HOLDS NOTHING IS NO CONFLICT EITHER: a file nobody references yet
	// is not a change to anything that runs.
	if err := os.Remove(image); err != nil {
		t.Fatal(err)
	}
	again, err := d.Prepare(rt)
	if err != nil {
		t.Fatalf("preparing again: %v", err)
	}
	if err := d.Replace(again); err != nil {
		t.Fatalf("a replacement whose image is not there yet was refused: %v", err)
	}
	if raw, rerr := os.ReadFile(image); rerr != nil ||
		!bytes.Equal(raw, again.assets[image]) {
		t.Errorf("the replacement did not write the image it needs (%v)", rerr)
	}
}
