package workflowstore_test

// WHAT MARCO MAY CONCLUDE ABOUT WHERE AN ACT IS STARTING, WITH NOBODY TO ASK.
//
// The authorisation contract changed: pressing Play authorises the Act's named interactions, and
// the screen question that used to be answered with a function key is now answered — or
// declined — from evidence. So this file is about the one thing that replaced a person: whether
// the saved screen an Act starts on can be identified positively, and whether it REFUSES in the
// case the keypress was really covering.
//
// The pair is the measured one again. Rocket League's Play and Training pages sit inside the
// arrangement measure's own resolution and are 0.319 apart by vocabulary, which is why the words
// are the discriminator here and the shape is only a gate.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// anActStartingSomewhere is a session with nothing established yet, which is where a run begins.
func anActStartingSomewhere() *workflowstore.SessionBinding {
	return workflowstore.NewSessionBinding()
}

// THE SCREEN AN ACT STARTS ON IS IDENTIFIED FROM THE SCREEN ITSELF.
//
// Both terms present: the shape agrees, and what is written on it keeps what that screen was
// read to say. Nobody is asked, and the binding records what it was identified by and under
// whose authority the run is being performed — because that sentence is the record of why a
// press was made without anybody being asked at the time.
func TestTheScreenAnActStartsOnIsIdentifiedWithoutAsking(t *testing.T) {
	art := twoPagesApart(t, savedTraining, wordsTraining)
	s := anActStartingSomewhere()

	b, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining, readingOfTraining,
		"", 7, 1, "Play, pressed on \"the audio tab\"", looked)
	if err != nil {
		t.Fatalf("the screen the Act starts on was not identified: %v", err)
	}
	if b.Label != "screen_c" {
		t.Errorf("the binding is for %q", b.Label)
	}
	// AND IT IS RECORDED AS AN IDENTIFICATION AND NOT AS SOMEBODY'S CONFIRMATION.
	//
	// The ground vocabulary is an audit trail. A run that recorded this as `confirmed`
	// would be saying a person answered a question the new contract deliberately does
	// not ask.
	if b.Ground != workflowstore.GroundIdentified {
		t.Errorf("the screen was established on ground %q", b.Ground)
	}
	// AND IT SAYS WHAT IT RESTS ON, both halves: the evidence, and the authority.
	for _, want := range []string{"agrees", "keeps", "no other saved screen"} {
		if !strings.Contains(b.Because, want) {
			t.Errorf("the account does not say %q:\n  %s", want, b.Because)
		}
	}
	if !strings.Contains(b.By, "Play, pressed on") {
		t.Errorf("the binding does not say under whose authority it was made: %q", b.By)
	}
	// AND IT IS ESTABLISHED FOR THIS OBSERVATION, which is what the runner then spends.
	if _, ok := s.Established("screen_c", 7, 1); !ok {
		t.Error("the screen was identified and not established")
	}
}

// AND TWO SCREENS THAT BOTH FIT ARE NOT AN IDENTIFICATION.
//
// # Why this is the test that matters
//
// Because it is the case the keypress was covering. A person shown "is this the graphics screen?
// (agrees 0.612)" cannot tell it from the audio screen beside it any better than the arrangement
// can — the owner's own game has two panels under one tab bar — so the yes they pressed was
// agreement with a ranking, not an identification.
//
// Here both saved screens agree with the look by shape AND the reading keeps enough of each
// vocabulary. Nothing in the evidence tells them apart, so nothing establishes either, and the
// refusal names BOTH candidates: a person reading it learns the actual problem rather than
// "could not establish the screen".
func TestTwoScreensThatBothFitEstablishNeither(t *testing.T) {
	// BOTH ENDS SAVED WITH THE SAME ARRANGEMENT, which is what a tab bar over two
	// panels measures as.
	art := pagesSaved(t, savedTraining, savedTraining, wordsTraining)
	s := anActStartingSomewhere()
	// AND A READING THAT KEEPS ENOUGH OF EACH: three of Training's three words, two of
	// the Play page's four.
	both := readOut("AERIAL", "GOALIE", "STRIKER", "PLAY", "MATCH")

	_, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining, both, "", 7, 1,
		"Play", looked)
	if err == nil {
		t.Fatal("an Act started on one of two screens nothing could tell apart")
	}
	for _, want := range []string{"screen_c", "screen_b", "tells them apart"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	if _, ok := s.Established("screen_c", 7, 1); ok {
		t.Error("a screen was established while two of them fitted")
	}
}

// AND A READING THAT MERELY FAILS TO CONTRADICT IS NOT ENOUGH HERE.
//
// The deliberate difference from `Reached`. There, a press has already gone out and a step was
// demonstrated to lead to this destination, so a reading that cannot contradict it is admissible
// evidence of arrival. Here nothing has happened yet and the next thing is a press authorised in
// advance, so the screen has to SAY what it was read to say.
//
// `readingTooThin` resolved one of the Training page's three words: it cannot contradict
// anything, and it keeps 33% where 35% identifies.
func TestAScreenOnlyUncontradictedDoesNotCarryAnActsPress(t *testing.T) {
	art := twoPagesApart(t, savedTraining, wordsTraining)
	s := anActStartingSomewhere()

	_, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining, readingTooThin,
		"", 7, 1, "Play", looked)
	if err == nil {
		t.Fatal("a press was authorised on a screen a reading could only fail to " +
			"contradict")
	}
	if !strings.Contains(err.Error(), "the evidence does not say so") {
		t.Errorf("the refusal does not say that the evidence is short rather than "+
			"wrong: %v", err)
	}
	// AND THE SAME LOOK WITH A READING THAT DOES IDENTIFY IT GOES THROUGH, so what is
	// being measured is the reading and not the look.
	if _, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining,
		readingOfTraining, "", 7, 1, "Play", looked); err != nil {
		t.Fatalf("the same look with a reading that identifies it was refused: %v", err)
	}
}

// AND A READING THAT CONTRADICTS IT REFUSES, as it does everywhere else.
func TestAContradictedScreenIsNotWhereAnActStarts(t *testing.T) {
	art := twoPagesApart(t, savedTraining, wordsTraining)
	s := anActStartingSomewhere()

	_, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining, readingOfSubmenu,
		"", 7, 1, "Play", looked)
	if err == nil {
		t.Fatal("the screen was identified although what was written on it disagreed")
	}
	if !strings.Contains(err.Error(), "says otherwise") {
		t.Errorf("the refusal does not say the words disagreed: %v", err)
	}
}

// AND A LOOK THAT DOES NOT AGREE BY SHAPE NEVER REACHES THE WORDS.
//
// The gate, in the direction it is a gate. This is also the ordinary case of a run that is
// waiting: the person is somewhere else in their menus, and the honest thing to say is that this
// does not look like the screen their Act starts on.
func TestALookThatIsNotTheScreenSaysSo(t *testing.T) {
	art := twoPagesApart(t, savedTraining, wordsTraining)
	s := anActStartingSomewhere()

	_, err := s.Standing(art, "screen_c", "visual_9", savedMenu, readingOfTraining,
		"", 7, 1, "Play", looked)
	if err == nil {
		t.Fatal("a screen was identified from a look at a different screen")
	}
	if !strings.Contains(err.Error(), "does not look like the screen") {
		t.Errorf("the refusal does not say what a person would need to do: %v", err)
	}
}

// AND A SCREEN SAVED WITHOUT WORDS CANNOT CARRY THIS AT ALL.
//
// Older demonstrations are exactly this case, and the consequence is worth stating plainly: an
// Act over a wordless screen can still be run, and it will pause instead of pressing. The
// arrangement alone was measured to confuse two screens of one menu, and that measurement is in
// the refusal so that a person reading it knows the demonstration is what needs redoing.
func TestAWordlessScreenPausesInsteadOfPressing(t *testing.T) {
	art := twoPagesApart(t, savedTraining, observe.ScreenText{})
	s := anActStartingSomewhere()

	_, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining, readingOfTraining,
		"", 7, 1, "Play", looked)
	if err == nil {
		t.Fatal("a press was authorised on a screen saved with no words at all")
	}
	if !strings.Contains(err.Error(), "saved without any words") {
		t.Errorf("the refusal does not say which half is missing: %v", err)
	}
}

// AND AN ANSWER SOMEBODY GAVE IS NOT OVERWRITTEN BY ONE OF THESE.
//
// A confirmation from a person is the stronger evidence. If a run was started with `--from`, or
// somebody said where they were, this returns that binding rather than deciding again — the same
// rule `Reached` holds, for the same reason.
func TestAPersonsOwnAnswerStands(t *testing.T) {
	art := twoPagesApart(t, savedTraining, wordsTraining)
	s := anActStartingSomewhere()
	if err := s.Confirm("screen_c", "visual_9", 7, 1, "the person who started the run "+
		"(--from)", looked); err != nil {
		t.Fatal(err)
	}

	b, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining, readingOfTraining,
		"", 7, 1, "Play", looked)
	if err != nil {
		t.Fatalf("an established screen was re-decided and refused: %v", err)
	}
	if b.Ground != workflowstore.GroundConfirmed {
		t.Errorf("the person's own answer was displaced by an identification: %s", b.Ground)
	}
}

// AND WHEN THERE IS NO READING, THE REFUSAL SAYS WHY THERE IS NONE.
//
// The sentence this term refuses with is the one a person watching a paused run reads, and it
// used to be the same sentence whatever the reason: "nothing has been read on it yet". The
// owner's first live Act refused twice in a row on the screen it starts on, inside four seconds
// each time, saying exactly that. It is true of a source that has not answered yet, of one whose
// answers keep arriving about a screen that has already changed, and of a look whose pixels were
// never the application's — and waiting only fixes the first.
//
// So the caller that can tell them apart supplies the reason, and this says it. The default is
// kept for a caller that has none, because an unexplained absence is still an absence.
func TestARefusalForWantOfWordsSaysWhyThereAreNone(t *testing.T) {
	art := twoPagesApart(t, savedTraining, wordsTraining)
	s := anActStartingSomewhere()

	_, err := s.Standing(art, "screen_c", "visual_9", lookOnTraining,
		observe.ScreenText{}, "the target was not the window in front", 7, 1,
		"Play", looked)
	if err == nil {
		t.Fatal("a screen with nothing read on it was identified")
	}
	if !strings.Contains(err.Error(), "not the window in front") {
		t.Errorf("the refusal does not carry the reason it was given: %v", err)
	}
	if strings.Contains(err.Error(), "nothing has been read on it yet") {
		t.Errorf("the refusal says the default over the reason it was given: %v", err)
	}
	// AND NOTHING SUPPLIED STILL SAYS SOMETHING.
	_, err = s.Standing(art, "screen_c", "visual_9", lookOnTraining,
		observe.ScreenText{}, "", 7, 1, "Play", looked)
	if err == nil || !strings.Contains(err.Error(), "nothing has been read on it yet") {
		t.Errorf("a refusal with no reason given does not say the plain one: %v", err)
	}
}
