package observe_test

// ONE reading, and what it is allowed to do.
//
// The failure these exist for is measured twice. By arrangement, Rocket League's Play page and
// its Training page agree at 0.592 on average against a threshold of 0.55 — two pages inside the
// measure's own resolution, so whether the navigation is visible depends on which look became a
// signature. By words they share nothing but `create` and `party`.
//
// The words were already allowed to refuse the merge, and could not reach it: the rule needed two
// consecutive readings of the new page, and [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] measured what that costs. A screen
// a workflow passes THROUGH is read once, at six hundred milliseconds a reading, and the reading
// that would corroborate the first is a reading of the next page.
//
// So one reading may now raise the objection, on a bar the corroborated rule does not have to
// clear. What has to hold is both halves, and the second half is most of these tests: a screen
// nobody has read twice still cannot be contradicted, a rotating panel is still one screen, a
// partial reading is still not a contradiction, and a refusal still mints nothing.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// theTwoPages is the measured pair of vocabularies, from the Free Play recording: 300 OCR
// readings of 900 real frames, the words of the page Marco left and the page it arrived on.
//
// Two words in common out of eight and six. `create` and `party` are the party panel, which is on
// both pages, and they are exactly what makes this a question rather than an arithmetic.
func theTwoPages() (play, training observe.ScreenText) {
	return read("MATCH", "OFFLINE", "PLAY", "PRIVATE", "CREATE", "PARTY", "QUICK",
			"TRAINING"),
		read("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER", "TUTORIAL")
}

// ── what must change ──────────────────────────────────────────────────────────

// ONE READING OF THE NEXT PAGE REFUSES THE MERGE, on a screen that has said something.
//
// This is Experiment-046 in a test. The Play page settles, the Training page replaces it, and the
// two arrangements agree well enough that the second settles onto the first — which is the merge
// that cost the navigation. One reading of the Training page arrives, nothing has corroborated
// it, and it is enough to refuse.
func TestOneReadingOfTheNextPageRefusesTheMergeOnAScreenThatHasSpoken(t *testing.T) {
	page, next := twoMenus()
	if sim := observe.ArrangementSimilarity(page, next); sim < observe.VisualScreenMatch {
		t.Fatalf("the fixture's arrangements agree at %.3f, below the %.2f that merges "+
			"them -- this is not the measured failure", sim, observe.VisualScreenMatch)
	}
	play, training := theTwoPages()
	v := &observe.VisualScreens{}

	// THE PAGE MARCO IS ON, read enough times to have established what it says.
	var settled observe.VisualScreenID
	for i := 0; i < 5; i++ {
		settled = v.ObserveWith(page, play, when)
	}
	if settled == observe.VisualScreenUnknown || settled == "" {
		t.Fatalf("nothing settled: %q", settled)
	}

	// ONE READING OF THE NEXT PAGE, arriving late, which is the only kind a live source
	// produces.
	refused, why := v.Read(reading(training, 11, 0, 0))
	if !refused {
		t.Fatalf("one reading of the next page refused nothing: %s", why)
	}
	if id, yes := v.Contradicting(); !yes || id != settled {
		t.Fatalf("the refusal stands against %q, want %s", id, settled)
	}
	st := v.Stability()
	if st.TextRefusedMerge != 1 || st.TextRefusedAlone != 1 {
		t.Errorf("the run says %d refusal(s), %d of them on one reading; want 1 and 1",
			st.TextRefusedMerge, st.TextRefusedAlone)
	}
	t.Log(why)

	// AND THE NEXT PAGE BECOMES A SCREEN, once its arrangement has held.
	var arrived observe.VisualScreenID
	for i := 0; i < observe.VisualSettleLooks; i++ {
		arrived = v.Observe(next, when)
	}
	if arrived == settled {
		t.Fatalf("the next page was folded into %s after all", settled)
	}
	if arrived == observe.VisualScreenUnknown || arrived == observe.VisualScreenUnobserved {
		t.Fatalf("the next page settled as %q rather than a screen", arrived)
	}
}

// AND THE SAME THING THROUGH THE LOOK-LOCAL PATH, which is the one the replay measures with.
//
// Two paths, one rule. A measurement that folds readings onto their own looks and a live session
// that takes them late must refuse the same merge, or the number the experiment prints is about
// something production does not do.
func TestOneReadingRefusesTheMergeOnTheLookLocalPathToo(t *testing.T) {
	page, next := twoMenus()
	play, training := theTwoPages()
	v := &observe.VisualScreens{}

	var settled observe.VisualScreenID
	for i := 0; i < 5; i++ {
		settled = v.ObserveWith(page, play, when)
	}
	// ONE LOOK AT THE NEXT PAGE, carrying its own reading. The look's side has no
	// agreement to offer: the reading before it was of the page Marco left.
	if got := v.ObserveWith(next, training, when); got != settled {
		t.Fatalf("one look became %s; a refusal is not a promotion", got)
	}
	if id, yes := v.Contradicting(); !yes || id != settled {
		t.Fatalf("the refusal stands against %q, want %s", id, settled)
	}
	if st := v.Stability(); st.TextRefusedAlone != 1 {
		t.Errorf("%d refusal(s) on one reading, want 1", st.TextRefusedAlone)
	}
}

// ── what must NOT change ──────────────────────────────────────────────────────

// A SCREEN NOBODY HAS READ TWICE STILL CANNOT BE CONTRADICTED.
//
// The safeguard that is not being reopened, and the measurement behind it: a main menu was
// contradicted on the strength of its first agreement, a second identity was minted for it, and
// from then on every look at that menu matched whichever of the two vocabularies happened to be
// closer — 23 visits each over 900 looks, for one screen that never changed.
//
// So this test asserts that the live failure is still reachable, which it is: a screen read once
// has not established what it says, and a page passed through in four looks is read once. That is
// the remaining half of Experiment-046 and it is a different question from this one.
func TestASoleReadingMayNotContradictAScreenReadOnlyOnce(t *testing.T) {
	page, next := twoMenus()
	play, training := theTwoPages()
	v := &observe.VisualScreens{}

	// FOUR LOOKS, ONE AGREEMENT. A screen's vocabulary is built from agreements between
	// consecutive readings, so two readings are one reading of what the screen says.
	var settled observe.VisualScreenID
	for i := 0; i < 2; i++ {
		settled = v.ObserveWith(page, play, when)
	}
	for i := 0; i < 2; i++ {
		settled = v.ObserveWith(page, observe.ScreenText{}, when)
	}
	if got := v.SaidBy(settled); got.Read() {
		t.Fatalf("one agreement established a vocabulary: %v", got.List())
	}

	refused, why := v.Read(reading(training, 11, 0, 0))
	if refused {
		t.Fatalf("a screen read once was contradicted: %s", why)
	}
	if id, yes := v.Contradicting(); yes {
		t.Fatalf("a screen read once is being contradicted: %s", id)
	}
	// AND THE MERGE HAPPENS, which is the failure this bound costs.
	for i := 0; i < 6; i++ {
		if got := v.Observe(next, when); got != settled {
			t.Fatalf("the next page became %s without any text to separate it", got)
		}
	}
}

// AND A ROTATING PANEL IS STILL ONE SCREEN.
//
// Rocket League's main menu carries a featured panel whose content changes under one identity:
// 0.946 against itself by arrangement, with different words. These are two real readings of it
// from the recording, nine hundred frames apart in content and both unmistakably that menu.
//
// It is tolerated because it KEEPS most of the vocabulary — the four real separations in the
// corpus keep at most 0.21 and this pair keeps 0.43 — and one uncorroborated reading does not
// get a weaker version of that test.
func TestARotatingPanelIsNotASecondScreenOnOneReading(t *testing.T) {
	menu, _ := twoMenus()
	panelA := read("ASSISTS", "CENTERS", "CHALLENGES", "CLEARS", "CLUB", "EARN", "GARAGE",
		"GOAL", "HONORS", "ITEM", "MATCH", "MATCHES", "MVP", "ONLINE", "PASS", "PLAY",
		"PROFILE", "QUIT", "ROCKET", "SETTINGS", "SHOP", "SHOTS", "VIEW", "WEEKLY")
	panelB := read("CHALLENGES", "CLUB", "COMPETITIVE", "DUELS", "FIRE", "GARAGE",
		"ITEM", "MATCHES", "PASS", "PLAY", "PLAYLIST", "POWERUPS", "PROFILE", "QUIT",
		"ROCKET", "RUMBLE", "SEASON", "SETTINGS", "SHOP", "USE", "VIEW", "WIN")
	v := &observe.VisualScreens{}

	var settled observe.VisualScreenID
	for i := 0; i < 5; i++ {
		settled = v.ObserveWith(menu, panelA, when)
	}
	// THE PANEL ROTATES, and the next reading of the same menu is this.
	if refused, why := v.Read(reading(panelB, 11, 0, 0)); refused {
		t.Fatalf("a rotated panel split the menu on one reading: %s", why)
	}
	for i := 0; i < 8; i++ {
		if got := v.ObserveWith(menu, panelB, when); got != settled {
			t.Fatalf("the menu became %s once its panel had rotated", got)
		}
	}
	if st := v.Stability(); st.TextRefusedMerge != 0 {
		t.Errorf("a rotating panel refused %d merge(s)", st.TextRefusedMerge)
	}
}

// AND THE RULE ITSELF, CONDITION BY CONDITION.
//
// `said` is what the screen has been read to say, `everSaid` every word ever folded into it, and
// `now` one uncorroborated reading. Each row is a thing that must not be enough.
func TestWhatOneReadingNeedsBeforeItMayRefuseAMerge(t *testing.T) {
	play, training := theTwoPages()
	for _, tc := range []struct {
		name            string
		said, ever, now observe.ScreenText
		want            bool
	}{
		{"the next page, on a screen that has spoken", play, play, training, true},
		{"a screen nobody established says nothing to contradict",
			observe.ScreenText{}, observe.ScreenText{}, training, false},
		{"nobody read this look", play, play, observe.ScreenText{}, false},
		{"a reading that resolved fewer labels brings nothing new", play, play,
			read("MATCH", "OFFLINE", "PLAY"), false},
		{"two new words are not a claim", play, play,
			read("MATCH", "AERIAL", "GOALIE"), false},
		{"words this screen has said before are not what makes it another",
			// `said` has lost the rotated panel's words to TextCommon;
			// `everSaid` remembers them, and that is enough.
			read("CHALLENGES", "CLUB", "GARAGE", "ITEM", "PROFILE", "QUIT",
				"ROCKET", "SETTINGS"),
			read("CHALLENGES", "CLUB", "GARAGE", "ITEM", "PROFILE", "QUIT",
				"ROCKET", "SETTINGS", "COMPETITIVE", "DUELS", "RUMBLE",
				"SEASON", "PLAYLIST", "POWERUPS"),
			read("COMPETITIVE", "DUELS", "RUMBLE", "SEASON", "PLAYLIST",
				"POWERUPS", "CLUB"), false},
		{"a rotated panel with three misreadings on it is still that screen",
			// The same menu, and the hard version: this reading keeps NONE
			// of what the screen usually says, brings three words nobody
			// has ever read on it, and is still mostly words that screen
			// has said. Every one of those three garbage tokens is from the
			// recording.
			read("CHALLENGES", "CLUB", "GARAGE", "ITEM", "PROFILE", "QUIT",
				"ROCKET", "SETTINGS"),
			read("CHALLENGES", "CLUB", "GARAGE", "ITEM", "PROFILE", "QUIT",
				"ROCKET", "SETTINGS", "COMPETITIVE", "DUELS", "RUMBLE",
				"SEASON", "PLAYLIST", "POWERUPS"),
			read("COMPETITIVE", "DUELS", "RUMBLE", "SEASON", "PLAYLIST",
				"POWERUPS", "SILY", "VISIVISILY", "NCARE"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := observe.TextSoleReadingDisagrees(tc.said, tc.ever, tc.now)
			if got != tc.want {
				t.Errorf("disagrees = %v, want %v: %s", got, tc.want, why)
			}
			t.Log(why)
		})
	}
}

// AND ONE READING MINTS NOTHING.
//
// The whole safety argument of this evidence is the direction it may push. A reading may decline
// to believe that this is that screen; the arrangement still has to hold for VisualSettleLooks
// before anything is a screen, and a reading that agrees takes the refusal back before it ever
// does.
func TestOneReadingRefusesAMergeAndEstablishesNoScreen(t *testing.T) {
	page, next := twoMenus()
	play, training := theTwoPages()
	v := &observe.VisualScreens{}

	var settled observe.VisualScreenID
	for i := 0; i < 5; i++ {
		settled = v.ObserveWith(page, play, when)
	}
	before := len(v.Seen())
	if refused, _ := v.Read(reading(training, 11, 0, 0)); !refused {
		t.Fatal("the reading refused nothing, so this tests nothing")
	}
	// ONE LOOK AT THE OTHER ARRANGEMENT is not a screen, refusal or no refusal.
	if got := v.Observe(next, when); got != settled {
		t.Errorf("one look after a refusal became %s", got)
	}
	if n := len(v.Seen()); n != before {
		t.Errorf("a refusal minted a screen: %d, want %d", n, before)
	}
	// AND A READING THAT AGREES TAKES IT BACK, so the screen is not split by the
	// reading that was passing through.
	if refused, why := v.Read(reading(play, 13, 0, 0)); refused {
		t.Fatalf("a reading of the standing screen refused it: %s", why)
	}
	if id, yes := v.Contradicting(); yes {
		t.Errorf("the refusal about %s still stands after a reading agreed", id)
	}
	for i := 0; i < 8; i++ {
		if got := v.Observe(page, when); got != settled {
			t.Fatalf("the screen split to %s after the refusal was withdrawn", got)
		}
	}
	if n := len(v.Seen()); n != before {
		t.Errorf("%d screen(s) after the whole exchange, want %d", n, before)
	}
}
