package main

// SIMPLIFYING THE RECORDING UNDER REVIEW, BEFORE IT IS SAVED.
//
// The saved routine's card has offered this since there was one, and `marco learn`'s terminal
// review has offered it for longer — so the one surface where somebody LOOKS at a demonstration
// they just made, with the steps in front of them and Save under their hand, was the only one
// that could not simplify it. They had to save it and then simplify the saved routine: two writes
// to get the version they wanted.
//
// Three claims here, and the first is the one that makes the rest safe: NOTHING IS WRITTEN by
// either button.
//
// See [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// slowTypingIntoNotepad types with long gaps between the characters, which is what there is to
// fold: the pipeline keeps human-paced waits unless it is asked to fold them harder.
func slowTypingIntoNotepad() []recorder.RecordedEvent {
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
	}
	ms := 10
	for _, k := range []string{"h", "e", "l", "l", "o"} {
		out = append(out,
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: true,
				T: at(ms), App: "notepad"},
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: false,
				T: at(ms + 20), App: "notepad"})
		ms += 400
	}
	return out
}

// slowTypingWithTheArgKey types two characters and then taps the RESERVED KEY where the supplied
// text goes — the old way to declare a text argument, which the routine's name must agree with
// (`say hi with message`).
func slowTypingWithTheArgKey() []recorder.RecordedEvent {
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
	}
	ms := 10
	for _, k := range []string{"h", "i", "f9"} {
		out = append(out,
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: true,
				T: at(ms), App: "notepad"},
			recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: k, Down: false,
				T: at(ms + 20), App: "notepad"})
		ms += 400
	}
	return out
}

// aRecordingUnderReview is a window holding a finished recording nobody has saved.
//
// Through the real recorder-free path: `aStudioLibrary` saves, which is not what this is about, so
// this builds the draft and parks it on the capture the way `StopRecording` leaves it.
func aRecordingUnderReview(t *testing.T, name string,
	events []recorder.RecordedEvent) *Acts {

	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	d := orchestrator.Deps{Reg: routes.Registry{Dir: dir}, Out: &strings.Builder{}}
	dr := d.DraftOf(name, events, "notepad", "the stop key", orchestrator.Unstated)
	if dr == nil {
		t.Fatal("the fixture produced no draft")
	}
	done := make(chan struct{})
	close(done)
	a.mu.Lock()
	a.capture = &capture{name: name, window: "Notepad", draft: dr,
		stop: make(chan struct{}), done: done}
	a.mu.Unlock()
	return a
}

// routesDirOf is the library this window writes to, so a test can see that nothing was written.
func routesDirOf(t *testing.T, a *Acts) string {
	t.Helper()
	got := a.Library()
	if got.Dir == "" {
		t.Fatal("the window reports no library")
	}
	return got.Dir
}

// wroteNothing is whether the library is still empty.
func wroteNothing(t *testing.T, dir string) bool {
	t.Helper()
	var found []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			found = append(found, p)
		}
		return nil
	})
	if len(found) > 0 {
		t.Logf("the library holds %v", found)
	}
	return len(found) == 0
}

// THE REVIEW CAN BE SIMPLIFIED, AND NOTHING IS WRITTEN.
func TestTheRecordingUnderReviewCanBeSimplified(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())
	dir := routesDirOf(t, a)

	was := a.Recording()
	if !was.Review {
		t.Fatalf("the fixture is not under review: %+v", was)
	}
	if was.Simplified || was.Original {
		t.Errorf("a fresh recording claims to be simplified: %+v", was)
	}

	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	now := a.Recording()
	if !now.Simplified {
		t.Error("the card does not say the steps are a simplification")
	}
	// AND THERE IS A WAY BACK, which is the other half of the ruling.
	if !now.Original {
		t.Error("the card offers no way back to what was demonstrated")
	}
	// AND IT IS STILL UNDER REVIEW, with steps to look at.
	if !now.Review || len(now.Steps) == 0 {
		t.Errorf("the review went away: %+v", now)
	}
	// NOTHING WAS WRITTEN. The claim the other two rest on.
	if !wroteNothing(t, dir) {
		t.Error("simplifying wrote something")
	}
}

// AND SIMPLIFYING A DRAFT THAT TAKES TEXT KEEPS IT TYPED.
//
// THE CASE THAT WOULD HAVE BEEN SILENT. A draft declares its arguments through its NAME —
// `say hi with message`, with the reserved key tapped where the text goes — and simplifying
// re-derives it, which means handing `codegen` a draft that must still declare them. Handed the
// bare name instead, the `{{message}}` in the steps is undeclared and renders as
// `do OS's Secret with "message"`: a routine that reads somebody's credential store, produced by
// pressing Simplify.
//
// A mutation that dropped the arguments from that name survived every other test here, because
// none of them simplified a draft that took anything.
//
// It is also the old declaration path, exercised: the phrase's `with message` clause and the f9
// tap, which is what [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]] exists to
// give somebody a findable alternative to.
func TestSimplifyingADraftThatTakesTextKeepsItTyped(t *testing.T) {
	a := aRecordingUnderReview(t, "say hi with message", slowTypingWithTheArgKey())
	before := a.Recording()
	if !strings.Contains(strings.Join(before.Steps, "|"), "{{message}}") {
		t.Fatalf("the fixture does not take text: %v", before.Steps)
	}

	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	if !strings.Contains(strings.Join(a.Recording().Steps, "|"), "{{message}}") {
		t.Errorf("simplifying lost the supplied value: %v", a.Recording().Steps)
	}

	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d plays", len(rs))
	}
	// TYPED, NEVER READ FROM THE CREDENTIAL STORE.
	if !strings.Contains(rs[0].Source, `do OS's Type with "{{message}}"`) {
		t.Errorf("the simplified play does not type the supplied value:\n%s",
			rs[0].Source)
	}
	if strings.Contains(rs[0].Source, "OS's Secret") {
		t.Errorf("the simplified play reads the credential store:\n%s", rs[0].Source)
	}
	// AND IT STILL ASKS FOR IT.
	if needs := a.PlayNeeds(rs[0].ID); !needs.Needs || len(needs.Fields) != 1 ||
		needs.Fields[0].Name != "message" {
		t.Errorf("the simplified play asks for %+v", needs.Fields)
	}
}

// AND THE ORIGINAL COMES BACK.
func TestTheOriginalRecordingComesBack(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())
	dir := routesDirOf(t, a)
	before := strings.Join(a.Recording().Steps, "|")

	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	if why := a.UseOriginalDraft(); why != "" {
		t.Fatalf("going back refused: %s", why)
	}
	back := a.Recording()
	if strings.Join(back.Steps, "|") != before {
		t.Errorf("what came back is not what was demonstrated:\n was  %s\n back %v",
			before, back.Steps)
	}
	if back.Simplified {
		t.Error("the card still says the steps are a simplification")
	}
	if back.Original {
		t.Error("the card still offers a way back to itself")
	}
	if !wroteNothing(t, dir) {
		t.Error("going back wrote something")
	}
}

// AND SIMPLIFYING TWICE STILL GOES BACK TO WHAT WAS DEMONSTRATED.
//
// Not to the previous simplification. There is one thing a person wants to return to and it is
// what they did; a stack of intermediate versions is a history nobody asked for.
func TestGoingBackReachesTheDemonstrationAndNotThePreviousVersion(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())
	before := strings.Join(a.Recording().Steps, "|")

	for i := 0; i < 2; i++ {
		if why := a.SimplifyDraft(); why != "" {
			t.Fatalf("simplifying refused on pass %d: %s", i+1, why)
		}
	}
	if why := a.UseOriginalDraft(); why != "" {
		t.Fatalf("going back refused: %s", why)
	}
	if got := strings.Join(a.Recording().Steps, "|"); got != before {
		t.Errorf("after two simplifications it came back to\n %s\nwant\n %s",
			got, before)
	}
}

// strandedCtrlInNotepad holds ctrl, leaves the window with it down, comes back and clicks.
//
// The state [[ADR-274-the-way-out-of-a-window-is-not-part-of-the-demonstration]] refuses: a hold
// written from this would have no release and would leave ctrl down on the desktop after the
// routine finished. Borrowed from `internal/orchestrator/exitgesture_test.go`, which is where the
// shape is established.
//
// What makes it the right fixture here is that the draft UNDER REVIEW is fine — three ordinary
// steps — and only the SIMPLIFICATION of it is refused. So there is a real original worth
// protecting, which is the whole point of the arm below.
func strandedCtrlInNotepad() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "ctrl", Down: true, App: "notepad", T: at(100)},
		{Kind: recorder.EvLeft, T: at(200)},
		{Kind: recorder.EvHeldAcross, KeyName: "ctrl", T: at(200)},
		{Kind: recorder.EvEntered, T: at(300)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "notepad", T: at(400)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "notepad", T: at(450)},
	}
}

// A SIMPLIFICATION THAT CANNOT BE SAVED IS REFUSED, AND DOES NOT DISPLACE THE ORIGINAL.
//
// # Why this one is worth its own fixture
//
// Because the alternative is a window that offers a press the door declines, and this one costs
// more than the usual version of that: swapping the unsavable draft in would take away the
// recording the person actually made. They would be reading steps they can approve, pressing
// Save, being refused — and having nothing to go back to, because the thing they would go back to
// is what was replaced.
//
// So the refusal happens BEFORE anything is swapped, and the three claims are the three things
// that have to be true at once: the reason is shown, the original is still there, and nothing was
// written.
//
// This closes the one mutation in this slice that nothing covered. The arm it protects is
// `next.Unrepresentable != ""` in [Acts.SimplifyDraft], and the mutation is deleting it.
func TestAnUnsavableSimplificationIsRefusedAndKeepsTheOriginal(t *testing.T) {
	a := aRecordingUnderReview(t, "hold ctrl", strandedCtrlInNotepad())
	dir := routesDirOf(t, a)

	// WHAT IS UNDER REVIEW, AND WHAT THE CARD ALREADY SAYS ABOUT IT.
	//
	// This demonstration cannot be saved either — the held ctrl is a property of the
	// EVENTS, so every re-derivation of them refuses — and the card says so in its own
	// summary line. Asserted rather than assumed, because the first version of this test
	// read `Unsupported` (which is `codegen.Unpropagatable`, a different question), found
	// it empty, and concluded the draft was fine.
	//
	// It does not weaken the three claims below. What they are about is the REFUSED
	// SIMPLIFICATION not displacing whatever is being reviewed, and that holds whether
	// the thing being reviewed can be saved or not.
	was := a.Recording()
	if !was.Review || len(was.Steps) == 0 {
		t.Fatalf("the fixture is not under review: %+v", was)
	}
	if !strings.Contains(was.Said, "ctrl") {
		t.Fatalf("the card does not already report the held key, so this fixture is "+
			"not the state it is meant to be: %q", was.Said)
	}
	steps := strings.Join(was.Steps, "|")

	// ── ONE: THE REFUSAL IS SHOWN, AND IT SAYS WHAT IS WRONG ──
	why := a.SimplifyDraft()
	if why == "" {
		t.Fatal("an unsavable simplification was offered as the version to review")
	}
	if !strings.Contains(why, "cannot be saved as it stands") {
		t.Errorf("the refusal does not say it cannot be saved: %q", why)
	}
	// THE GENERATOR'S OWN REASON, carried rather than translated: what a person needs is
	// the key that was left down and what to do about it.
	if !strings.Contains(why, "ctrl") {
		t.Errorf("the refusal does not name the key that was held: %q", why)
	}

	// ── TWO: THE ORIGINAL IS STILL THE ONE UNDER REVIEW ──
	now := a.Recording()
	if got := strings.Join(now.Steps, "|"); got != steps {
		t.Errorf("the refused simplification displaced the recording:\n was %s\n now %s",
			steps, got)
	}
	if now.Simplified {
		t.Error("the card says the steps are a simplification, and they are the " +
			"recording")
	}
	// AND NO WAY BACK IS OFFERED, because nothing was replaced. A button to return to
	// what is already on screen is a window inventing a state it is not in.
	if now.Original {
		t.Error("a way back is offered to the draft that is already being shown")
	}
	// AND GOING BACK SAYS THERE IS NOTHING TO GO BACK TO, which is the same fact asked
	// of the other method.
	if back := a.UseOriginalDraft(); back == "" {
		t.Error("going back was accepted after a refused simplification")
	}

	// ── THREE: NOTHING WAS WRITTEN ──
	if !wroteNothing(t, dir) {
		t.Error("a refused simplification wrote something")
	}

	// AND THE CARD STILL SAYS THE SAME THING ABOUT THE RECORDING.
	//
	// The refusal was about a version that was never adopted, so nothing it said may end
	// up attached to the draft: a card that came back reporting the simplification's
	// reason as though it were the recording's would be describing a thing that does not
	// exist.
	if now.Said != was.Said {
		t.Errorf("the card's summary changed:\n was %q\n now %q", was.Said, now.Said)
	}
	// AND SAVE STILL ANSWERS FOR THE DRAFT ITSELF, with the draft's own reason — not
	// with anything about simplifying, and not with a success it should not give.
	saved := a.SaveDraft("app", "")
	if saved == "" {
		t.Fatal("the unsavable recording was saved")
	}
	if !strings.Contains(saved, "ctrl") {
		t.Errorf("Save refused for some other reason: %q", saved)
	}
	if strings.Contains(saved, "simplified") {
		t.Errorf("Save refused with the simplification's reason: %q", saved)
	}
	// AND STILL NOTHING IS WRITTEN.
	if !wroteNothing(t, dir) {
		t.Error("the refused save wrote something")
	}
}

// AND GOING BACK WITH NOTHING TO GO BACK TO SAYS SO.
func TestGoingBackWithNothingToGoBackToSaysSo(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())
	why := a.UseOriginalDraft()
	if why == "" {
		t.Fatal("going back from an unsimplified recording was accepted")
	}
	if !strings.Contains(why, "not been simplified") {
		t.Errorf("it refused as %q", why)
	}
}

// AND NEITHER IS OFFERED WHEN THERE IS NOTHING UNDER REVIEW.
func TestSimplifyingNothingSaysSo(t *testing.T) {
	a := newActs(t.TempDir())
	t.Setenv("MARCO_ROUTES", t.TempDir())
	if why := a.SimplifyDraft(); why == "" {
		t.Error("simplifying with no recording was accepted")
	}
	if why := a.UseOriginalDraft(); why == "" {
		t.Error("going back with no recording was accepted")
	}
}

// AND SAVE WRITES THE VERSION THAT IS BEING SHOWN.
//
// The rule Save already followed, asserted now that there are two versions it could write. A Save
// that wrote the demonstration after somebody simplified it would be writing the one they
// rejected.
//
// READ OFF THE SAVED SOURCE, because that is what runs. The step panel is a second instrument with
// preconditions of its own — see the test below, which is about exactly that.
func TestSaveWritesTheVersionOnScreen(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())
	before := a.Routines()
	if len(before) != 0 {
		t.Fatalf("the fixture starts with %d plays", len(before))
	}
	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	// WHAT THE CARD SHOWS: one typed run, not five characters and the waits between.
	shown := strings.Join(a.Recording().Steps, "|")
	if !strings.Contains(shown, `type "hello"`) {
		t.Fatalf("the simplified card shows %s", shown)
	}

	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d plays", len(rs))
	}
	// AND THE SAVED SOURCE IS THAT ONE.
	if !strings.Contains(rs[0].Source, `do OS's Type with "hello"`) {
		t.Errorf("the saved play does not type the folded run:\n%s", rs[0].Source)
	}
	if strings.Contains(rs[0].Source, `do OS's Type with "h".`) {
		t.Errorf("the saved play is the version that was rejected:\n%s", rs[0].Source)
	}
}

// SIMPLIFY, SAVE, EDIT, SAVE AGAIN — THE WHOLE OF IT, THROUGH THE WINDOW'S OWN ENTRANCES.
//
// This test used to be called `TestASimplifiedSaveCannotBeStepEditedYet`, and it asserted the
// refusal. The dead end was real and it was in the middle of the one journey this window is for:
// the review card puts Simplify directly above Save, and the next thing it offers is editing the
// steps — which refused, with a sentence about the file and the demonstration no longer agreeing.
// Nobody had done anything wrong. They pressed the two buttons in the order they were offered.
//
// WHY IT REFUSED, which is the thing the fix had to change rather than explain. A fresh save wrote
// no step account, and no account LICENSES rendering the demonstration and treating the result as
// what the file is. True of every ordinary save, because its source came from that same rendering.
// False of a simplified one: its source was folded harder than a rebuild folds, so the two
// disagreed by construction and `basisFor` reported the disagreement as what it looks like from
// there — which reads like file damage.
//
// So a simplified draft now carries its basis (`orchestrator.Draft.Basis`) and the save writes it
// down beside the routine. Nothing is simplified on the way past, the demonstration is stored
// exactly as it was, and a plain save still writes no account at all.
//
// THROUGH THE PRODUCTION ENTRANCES, every step: the same methods the page is bound to, in the
// order the card offers them. A test that reached into the orchestrator would be a test of the
// repair rather than of the journey it was for.
func TestASimplifiedSaveCanBeStepEditedAndSavedAgain(t *testing.T) {
	a := aRecordingUnderReview(t, "say hello", slowTypingIntoNotepad())
	if why := a.SimplifyDraft(); why != "" {
		t.Fatalf("simplifying refused: %s", why)
	}
	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	// AND THE SAVE WAS WHOLE. The third answer is a routine that is on disk with no
	// account beside it, which is the state this test exists to rule out — and it is
	// reported rather than thrown away, so a test that only asked whether Save returned
	// a refusal would pass on exactly the thing that used to be wrong.
	if r := a.Recording(); r.Partial != "" {
		t.Fatalf("the play was saved without its steps beside it: %s", r.Partial)
	}
	id := a.Routines()[0].ID
	e := a.StepEditing(id)
	if e.Why != "" {
		t.Fatalf("a play saved from a simplified draft cannot be edited: %s", e.Why)
	}
	// AND THE LIST IS THE SIMPLIFIED ONE, not a rebuild of the demonstration. The whole
	// of what the account is for: with none beside it, the panel would be listing the
	// five separate keystrokes somebody asked to have folded, and a save from that list
	// would quietly undo the simplification.
	typing := []StepRow{}
	for _, row := range e.Rows {
		if row.Types {
			typing = append(typing, row)
		}
	}
	if len(typing) != 1 {
		t.Fatalf("the panel lists %d typing steps, so it is not showing the "+
			"simplified version: %v", len(typing), e.Rows)
	}
	if !strings.Contains(typing[0].Says, "hello") {
		t.Errorf("the one typing step is not the folded run: %q", typing[0].Says)
	}
	// ── AND THE THING THE JOURNEY ASKS FOR NEXT: ASK ME FOR IT ──
	p := a.ReviewStepEdit(id, e.Basis, []StepChange{
		{Op: "argument", At: typing[0].At, Name: "message"}})
	switch {
	case p.Why != "":
		t.Fatalf("the edit could not be reviewed: %s", p.Why)
	case p.Refused != "":
		t.Fatalf("the edit was refused: %s", p.Refused)
	case !p.Savable:
		t.Fatalf("there is nothing to save after making the typed step supplied: %v",
			p.Doing)
	}
	done := a.SaveStepEdit(p.Token)
	switch {
	case done.Why != "":
		t.Fatalf("saving the edit refused: %s", done.Why)
	case done.Partial:
		t.Fatalf("the edit was saved without its steps beside it: %s", done.Says)
	case !done.Saved:
		t.Fatal("the edit reports neither a save nor a reason")
	}
	// AND THE ROUTINE IT LEAVES BEHIND TAKES THE TEXT, which is the end of the journey:
	// the step that typed `hello` now asks for it.
	rt := a.Routines()[0]
	if !strings.Contains(rt.Source, "message") {
		t.Errorf("the saved play does not take the supplied text:\n%s", rt.Source)
	}
	// AND IT CAN BE EDITED AGAIN. A save that wrote the source and not the account
	// leaves a routine that runs and refuses the next edit, which is a state somebody
	// reaches by doing nothing unusual twice.
	if again := a.StepEditing(id); again.Why != "" {
		t.Errorf("the edited play cannot be edited again: %s", again.Why)
	}
}

// AND A PLAIN SAVE STILL WRITES NO ACCOUNT, which is what keeps the rule above a narrow one.
//
// The sweep in `Deps.write` is right: what lands there is a rendering of a recording, and an
// account from before describes a demonstration that is gone. A simplified draft is the exception
// and says so on the draft; everything else must still be editable BY REBUILDING from its own
// demonstration, which is the inference `routes.StepsNone` licenses. A save that started writing
// an account for every draft would make that inference unreachable and would be a second, quieter
// claim about every routine in somebody's library.
func TestAPlainSaveIsEditedFromItsDemonstration(t *testing.T) {
	b := aRecordingUnderReview(t, "say plainly", slowTypingIntoNotepad())
	if why := b.SaveDraft("app", ""); why != "" {
		t.Fatalf("the plain save refused: %s", why)
	}
	if r := b.Recording(); r.Partial != "" {
		t.Fatalf("a plain save reported a partial write: %s", r.Partial)
	}
	rt := b.Routines()[0]
	if plain := b.StepEditing(rt.ID); plain.Why != "" {
		t.Errorf("a plain save cannot be step-edited: %s", plain.Why)
	}
	// NOTHING WAS WRITTEN BESIDE IT. Read through the same registry the window writes to,
	// because "it can be edited" is true of both answers and only one of them is this.
	if _, state := (routes.Registry{Dir: routesDirOf(t, b)}).Steps(
		routes.Route{Slug: rt.Slug, App: rt.App}); state != routes.StepsNone {
		t.Errorf("a plain save left a step account beside the play (%s), so the "+
			"basis a simplified draft needs is being written for every save",
			state)
	}
}
