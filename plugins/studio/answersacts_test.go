package main

// WHAT A ROUTINE ANSWERS TO, FROM THE WINDOW, AGAINST A REAL LIBRARY.
//
// The surface half. These go through `Acts` with a routine actually saved on disk, so what they
// hold is the wiring as well as the words: a panel that listed phrases it could not add to, or
// added one the door would not then answer to, would pass a test of either half on its own.
//
// What is NOT here is the collision rule or the normalization — those are the contract's and are
// held in `internal/orchestrator`. What IS here is that this window asks it and shows what it
// said, rather than deciding anything about the library from a card about one routine.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// routineRoute is the route behind one of this window's cards, which is what the registry is
// asked about afterwards.
func routineRoute(t *testing.T, a *Acts, id string) routes.Route {
	t.Helper()
	r, ok := a.routineWithID(id)
	if !ok {
		t.Fatalf("the window does not hold a routine at %q", id)
	}
	return r.route()
}

// A ROUTINE ANSWERING ONLY TO ITS OWN NAME IS AN ORDINARY ROUTINE.
//
// No phrases, no reason, and the control offered: a blank list is the common case and must not
// read as a problem. The `Why` field is for a recording in progress or a file left behind, and
// filling it here would put a sentence on every card in the library.
func TestARoutineWithNoPhrasesIsOfferedTheControlAndNoComplaint(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	got := a.RoutineAnswers(r.ID)
	if got.Why != "" {
		t.Errorf("an ordinary routine is explained: %q", got.Why)
	}
	if len(got.Phrases) != 0 {
		t.Errorf("it answers to %q", got.Phrases)
	}
	if !got.Askable {
		t.Error("a phrase cannot be added to an ordinary routine")
	}
	if got.Name == "" || got.ID != r.ID {
		t.Errorf("the panel is about %q at %q", got.Name, got.ID)
	}
}

// A PHRASE IS ADDED, LISTED, AND THE DOOR ANSWERS TO IT.
//
// The third of those is the one that matters: the write landing and the list redrawing proves the
// window, and only resolution proves the product. A phrase stored where nothing reads it is the
// shape of defect this repository keeps finding.
func TestAPhraseAddedInStudioIsWhatTheDoorAnswersTo(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	got := a.AddRoutineAnswer(r.ID, "Say Hello Twice")
	if got.Failed {
		t.Fatalf("the phrase was refused: %s", got.Said)
	}
	if len(got.Phrases) != 1 || got.Phrases[0] != "Say Hello Twice" {
		t.Fatalf("it answers to %q", got.Phrases)
	}
	if !strings.Contains(got.Said, "Say Hello Twice") {
		t.Errorf("the sentence does not name the phrase: %q", got.Said)
	}
	// AND THE LIBRARY ANSWERS FOR IT, through the reader Director resolves with. Not
	// `Resolve`: that door is exact-or-nothing and does not consult phrases, because
	// whether one is ambiguous is a judgement and judgement is Director's.
	rt := routineRoute(t, a, r.ID)
	back := a.reg().AnsweringTo("say hello twice")
	if len(back) != 1 || back[0] != rt {
		t.Errorf("the phrase is answered by %v", back)
	}
}

// AND IT IS STORED AS IT WAS TYPED, so the list reads back as the person wrote it.
func TestAPhraseIsListedAsItWasTyped(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	if said := a.AddRoutineAnswer(r.ID, "  Say Hello Twice  "); said.Failed {
		t.Fatal(said.Said)
	}
	got := a.RoutineAnswers(r.ID)
	if len(got.Phrases) != 1 || got.Phrases[0] != "Say Hello Twice" {
		t.Errorf("it answers to %q — trimmed, and otherwise as typed", got.Phrases)
	}
}

// AND A PHRASE THE RESOLVER CANNOT TELL FROM THE ROUTINE'S OWN NAME IS NOT A SECOND WAY IN.
//
// Found by writing this fixture wrong, which is the rule earning its place: the routine is called
// "type two letters" and "type two letters please" looks like a different phrase — but `please` is
// a stop word to the matcher, so the two reduce to one key and the phrase could never be told from
// the name it duplicates. Said rather than stored, because nobody did anything wrong.
func TestAPhraseThatReducesToTheRoutinesOwnNameIsSaidRatherThanStored(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	got := a.AddRoutineAnswer(r.ID, "type two letters please")
	if len(got.Phrases) != 0 {
		t.Errorf("it answers to %q", got.Phrases)
	}
	if got.Said == "" {
		t.Error("the panel said nothing about it")
	}
	if a.reg().HasAliases(routineRoute(t, a, r.ID)) {
		t.Error("it was written down")
	}
}

// A COLLISION IS THE BACKEND'S SENTENCE, AND NOTHING IS STORED.
//
// This window does not hold the collision rule and must not: it is about the whole library, and a
// card about one routine is the worst place to re-derive it. What the panel does is show the
// refusal, which names the routine the phrase is already how to reach.
func TestACollidingPhraseIsRefusedWithTheBackendsOwnSentence(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	rt := routineRoute(t, a, r.ID)
	got := a.AddRoutineAnswer(r.ID, rt.Slug)
	if !got.Failed && len(got.Phrases) != 0 {
		t.Fatalf("a routine was taught to answer to its own name: %+v", got)
	}
	// ITS OWN NAME IS SAID RATHER THAN REFUSED — nobody did anything wrong — and
	// either way nothing is stored.
	if a.reg().HasAliases(rt) {
		t.Error("a phrase that can never be reached was written down")
	}
	if got.Said == "" {
		t.Error("the panel said nothing about it")
	}
}

// AND A PHRASE THE CONTRACT REFUSES IS RENDERED AS A REFUSAL, IN ITS OWN WORDS.
//
// WRITTEN FOR A SURVIVOR. The case above is one the contract SAYS rather than refuses — nobody
// did anything wrong adding a routine's own name — so it never takes the error path, and deleting
// the error path entirely survived it. This takes it: a phrase with no words in it is refused,
// and what the panel shows is the sentence the contract produced rather than one written here.
func TestARefusedPhraseIsShownAsTheContractsOwnRefusal(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	got := a.AddRoutineAnswer(r.ID, "!!!")
	if !got.Failed {
		t.Fatalf("a phrase with no words in it was accepted: %+v", got)
	}
	if !strings.Contains(got.Said, "!!!") {
		t.Errorf("the refusal does not name what was refused: %q", got.Said)
	}
	if len(got.Phrases) != 0 {
		t.Errorf("it answers to %q", got.Phrases)
	}
}

// AND A PHRASE IS REMOVED BY NAMING IT, leaving the others alone.
func TestAPhraseIsRemovedFromTheCardByNamingIt(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	for _, p := range []string{"first way", "second way", "third way"} {
		if got := a.AddRoutineAnswer(r.ID, p); got.Failed {
			t.Fatalf("%q was refused: %s", p, got.Said)
		}
	}
	got := a.RemoveRoutineAnswer(r.ID, "second way")
	if got.Failed {
		t.Fatalf("the removal failed: %s", got.Said)
	}
	if strings.Join(got.Phrases, "|") != "first way|third way" {
		t.Errorf("it answers to %q", got.Phrases)
	}
	if len(a.reg().AnsweringTo("second way")) != 0 {
		t.Error("the removed phrase still answers for the routine")
	}
	if len(a.reg().AnsweringTo("third way")) != 1 {
		t.Error("removing one phrase took another with it")
	}
}

// NOTHING IS OFFERED WHILE A DEMONSTRATION IS BEING RECORDED.
//
// The same hold-off the step panel takes, for the same reason: a window that offers one thing
// while it is doing another is how somebody comes to attach a phrase to the routine they are in
// the middle of replacing.
func TestThePhrasePanelHoldsOffWhileRecording(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	a.recording = func() recorder.Recorder { return &heldRecorder{} }
	if why := a.StartRecording("something else", ""); why != "" {
		t.Fatal(why)
	}
	defer a.CancelRecording()
	got := a.RoutineAnswers(r.ID)
	if got.Askable || got.Why == "" {
		t.Fatalf("phrases were offered during a recording: %+v", got)
	}
	// AND AN ADD IS REFUSED RATHER THAN QUEUED.
	if said := a.AddRoutineAnswer(r.ID, "while recording"); said.Askable {
		t.Error("a phrase was added during a recording")
	}
	rt := routineRoute(t, a, r.ID)
	if a.reg().HasAliases(rt) {
		t.Error("it was written down anyway")
	}
}

// AND A ROUTINE THIS LIBRARY DOES NOT HOLD IS SAID RATHER THAN GUESSED AT.
func TestPhrasesForARoutineThatIsNotThereSayWhy(t *testing.T) {
	a, _ := anEditableRoutineInStudio(t)
	got := a.RoutineAnswers("trackmania/focus/not-here")
	if got.Askable {
		t.Error("a routine that is not in the library was offered a phrase")
	}
	if !strings.Contains(got.Why, "no routine") {
		t.Errorf("the reason is %q", got.Why)
	}
}
