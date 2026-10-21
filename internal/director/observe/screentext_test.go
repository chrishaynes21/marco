package observe_test

// Text, where a screen's identity is decided.
//
// The failure these exist for is measured: Rocket League's Free Play submenu agreed with the
// Training page it replaced at 0.721 by arrangement against a threshold of 0.55, so it settled
// onto the screen Marco was already on and no new identity was minted. By words the same two
// screens agree at 0.231.
//
// What has to hold is both halves. Stable, materially different text must prevent the merge --
// and nothing else may. A screen that nobody read, a screen read once, a counter, a hover
// highlight and a genuine return to a screen Marco has seen before must all behave exactly as
// they did before any of this existed.

import (
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var when = time.Unix(1700000000, 0).UTC()

// twoMenus is the measured pair: two arrangements alike enough to merge.
//
// The cell counts on the left are screen_c's, exactly as the Rocket League artifact stores them.
// The ones on the right are NOT screen_d's stored counts -- those agree at 0.540, just under the
// threshold, which is the whole point of the margin finding and would not exercise the merge at
// all. These are a LIVE look at the second menu: the same structure with fewer proposals
// resolved, which is what the two menus actually did to each other in Experiment-041 at
// **0.721**. The test asserts the number rather than trusting this comment.
func twoMenus() (observe.VisualArrangement, observe.VisualArrangement) {
	training := observe.VisualArrangement{Total: 99, Cells: map[string]int{
		"0,0": 3, "0,1": 26, "0,2": 2, "1,0": 1, "1,1": 17, "1,2": 7,
		"2,0": 13, "2,1": 30,
	}}
	freePlay := observe.VisualArrangement{Total: 73, Cells: map[string]int{
		"0,0": 5, "0,1": 19, "0,2": 1, "1,0": 1, "1,1": 12, "1,2": 4,
		"2,0": 9, "2,1": 22,
	}}
	return training, freePlay
}

func read(words ...string) observe.ScreenText {
	return observe.TextOf(directorapi.SourceOCR, words)
}

// THE FIXTURE IS THE MEASURED FAILURE, or none of the rest means anything.
func TestTheTwoMenusAgreeByArrangementAndDisagreeByWords(t *testing.T) {
	training, freePlay := twoMenus()
	sim := observe.ArrangementSimilarity(training, freePlay)
	if sim < observe.VisualScreenMatch {
		t.Fatalf("the fixture's two arrangements agree at %.3f, below the %.2f that "+
			"merges them -- this is not the measured failure", sim,
			observe.VisualScreenMatch)
	}
	// AND IT IS THE MEASURED NUMBER, not merely over the line. A fixture that merged by
	// a hair would pass every test below while testing a case that does not happen.
	if sim < 0.65 || sim > 0.80 {
		t.Errorf("the fixture's arrangements agree at %.3f; the live pair agreed at "+
			"0.721 and this is supposed to be that case", sim)
	}
	// THE VOCABULARIES THE PRODUCTION FOLD ACTUALLY BUILT for these two screens, from
	// 300 OCR readings of the recording -- not a transcription of what is printed on
	// them. "FREE PLAY" is on the Training page as a tile label and did not survive into
	// its stable set, because OCR resolves it on fewer than half the looks. One shared
	// word, "create", out of five each.
	a := read("AERIAL", "CREATE", "GOALIE", "PARTY", "STRIKER")
	b := read("CREATE", "FREE", "JOIN", "ONLINE", "PLAY")
	words := observe.TextSimilarity(a, b)
	kept, novel := observe.TextOverlap(a, b)
	if kept >= observe.TextKept || novel <= observe.TextNovel {
		t.Fatalf("the fixture's readings keep %.2f and bring %.2f new, which is not a "+
			"contradiction under %.2f/%.2f", kept, novel,
			observe.TextKept, observe.TextNovel)
	}
	apart, why := observe.TextsDisagree(a, b)
	if !apart {
		t.Fatalf("the two readings do not disagree: %s", why)
	}
	t.Logf("arrangement %.3f, words %.3f, kept %.2f, novel %.2f -- %s",
		sim, words, kept, novel, why)
}

// ── what must change ──────────────────────────────────────────────────────────

// TWO SCREENS THAT SAY DIFFERENT THINGS DO NOT MERGE, however alike their arrangements.
func TestAScreenThatSaysSomethingElseIsNotMergedIntoTheStandingOne(t *testing.T) {
	training, freePlay := twoMenus()
	v := &observe.VisualScreens{}
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE", "FREE PLAY")
	other := read("BACK", "CREATE", "FREE PLAY", "JOIN", "ONLINE")

	// THE FIRST SCREEN SETTLES, and is read enough times to have a vocabulary.
	var first observe.VisualScreenID
	for i := 0; i < 5; i++ {
		first = v.ObserveWith(training, say, when)
	}
	if first == observe.VisualScreenUnknown || first == "" {
		t.Fatalf("nothing settled: %q", first)
	}

	// THE SECOND ARRIVES. By arrangement it is the first screen; by words it is not.
	var second observe.VisualScreenID
	for i := 0; i < 5; i++ {
		second = v.ObserveWith(freePlay, other, when)
	}
	if second == first {
		t.Fatalf("the second menu was folded into %s, which is the measured failure",
			first)
	}
	if second == observe.VisualScreenUnknown || second == observe.VisualScreenUnobserved {
		t.Fatalf("the second menu settled as %q rather than a screen", second)
	}
	// AND THE RUN SAYS THE TEXT IS WHAT DID IT.
	st := v.Stability()
	if st.TextRefusedMerge == 0 {
		t.Error("two screens were separated and the run does not say text refused a merge")
	}
	if st.TextRead == 0 {
		t.Error("the run read text and does not say so")
	}
}

// AND THE TRANSITION BETWEEN THEM IS RECORDED, which is what a route is read from.
func TestSeparatingTwoMenusProducesATransitionBetweenThem(t *testing.T) {
	training, freePlay := twoMenus()
	v := &observe.VisualScreens{}
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE")
	other := read("BACK", "CREATE", "FREE PLAY", "JOIN", "ONLINE")

	var from observe.VisualScreenID
	for i := 0; i < 5; i++ {
		from = v.ObserveWith(training, say, when)
	}
	var to observe.VisualScreenID
	for i := 0; i < 5; i++ {
		to = v.ObserveWith(freePlay, other, when)
	}
	if from == to {
		t.Fatalf("both menus are %s", from)
	}
	var found bool
	for _, tr := range v.Transitions() {
		if tr.From == from && tr.To == to {
			found = true
		}
	}
	if !found {
		t.Errorf("no transition from %s to %s in %+v", from, to, v.Transitions())
	}
}

// ── what must NOT change ──────────────────────────────────────────────────────

// A SCREEN NOBODY READ BEHAVES EXACTLY AS IT ALWAYS DID.
//
// Missing OCR is not contradictory OCR. The pixel-only path has to be bit-identical, because
// every other caller in the tree is on it and because a frame OCR happened to miss must not look
// like a screen that went silent.
func TestWithNothingReadTheFoldIsTheOneItAlwaysWas(t *testing.T) {
	training, freePlay := twoMenus()
	plain, withText := &observe.VisualScreens{}, &observe.VisualScreens{}
	var a, b []observe.VisualScreenID
	for i := 0; i < 5; i++ {
		a = append(a, plain.Observe(training, when))
		b = append(b, withText.ObserveWith(training, observe.ScreenText{}, when))
	}
	for i := 0; i < 5; i++ {
		a = append(a, plain.Observe(freePlay, when))
		b = append(b, withText.ObserveWith(freePlay, observe.ScreenText{}, when))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("look %d: Observe says %q and ObserveWith(nothing) says %q",
				i, a[i], b[i])
		}
	}
	// AND THE TWO MENUS STILL MERGE, which is the behaviour this does not change.
	// The last look of each run, not the first: the opening look of a session is
	// warming and is not a screen.
	if a[4] != a[9] {
		t.Errorf("the two arrangements settled as %q and %q with nothing read; "+
			"the pixel-only path has changed", a[4], a[9])
	}
	if st := withText.Stability(); st.TextRead != 0 || st.TextRefusedMerge != 0 {
		t.Errorf("a run with nothing read reports %+v", st)
	}
}

// AND A SCREEN READ ONCE CANNOT CONTRADICT ANYTHING.
//
// One reading is not a vocabulary. A screen the proposer has settled but OCR has seen once has
// not been established to say anything, and letting it disagree would make the first reading of
// every screen a potential split.
func TestOneReadingIsNotAVocabulary(t *testing.T) {
	training, freePlay := twoMenus()
	v := &observe.VisualScreens{}
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE")
	other := read("BACK", "CREATE", "FREE PLAY", "JOIN", "ONLINE")

	// The first screen settles with exactly ONE reading, the rest unread.
	first := v.ObserveWith(training, say, when)
	for i := 0; i < 4; i++ {
		first = v.ObserveWith(training, observe.ScreenText{}, when)
	}
	var second observe.VisualScreenID
	for i := 0; i < 5; i++ {
		second = v.ObserveWith(freePlay, other, when)
	}
	if second != first {
		t.Errorf("a screen read once contradicted a later look: %s then %s",
			first, second)
	}
	if st := v.Stability(); st.TextRefusedMerge != 0 {
		t.Errorf("one reading refused %d merge(s)", st.TextRefusedMerge)
	}
}

// AND A COUNTER, A CLOCK AND A HOVER HIGHLIGHT DO NOT SPLIT A SCREEN.
//
// Each of them puts words on one reading. The screen's stable vocabulary is the words that
// survived a second reading, so none of these ever joins it -- and a look carrying one of them
// still agrees with the screen on everything else.
func TestTransientWordsDoNotSplitAScreen(t *testing.T) {
	training, _ := twoMenus()
	v := &observe.VisualScreens{}
	base := []string{"TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE",
		"FREE PLAY"}

	var id observe.VisualScreenID
	for i := 0; i < 4; i++ {
		id = v.ObserveWith(training, read(base...), when)
	}
	settled := id
	if settled == observe.VisualScreenUnknown {
		t.Fatal("nothing settled")
	}
	// TWENTY LOOKS, each with a different transient word on it: a frame counter, a
	// clock, a selection highlight bringing one label into reach.
	for i := 0; i < 20; i++ {
		noisy := append([]string{}, base...)
		noisy = append(noisy, "PING", "FPS")
		noisy = append(noisy, []string{"SELECTED", "HOVER", "READY", "ONLINE"}[i%4])
		noisy = append(noisy, string(rune('a'+i%26))+string(rune('a'+(i*7)%26)))
		got := v.ObserveWith(training, read(noisy...), when)
		if got != settled {
			t.Fatalf("look %d of the same screen became %s; a transient word split it",
				i, got)
		}
	}
	if st := v.Stability(); st.TextRefusedMerge != 0 {
		t.Errorf("transient words refused %d merge(s)", st.TextRefusedMerge)
	}
}

// AND A SCREEN MARCO RETURNS TO IS STILL THE SCREEN IT WAS.
//
// Reuse is what makes a route possible: walking back out of a submenu has to arrive at the screen
// it left, or every demonstration is a chain of new places and no repetition is visible. A veto
// that also fired on a genuine return would break the thing this subsystem is for.
func TestAScreenReturnedToIsStillTheSameScreen(t *testing.T) {
	training, freePlay := twoMenus()
	v := &observe.VisualScreens{}
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE")
	other := read("BACK", "CREATE", "FREE PLAY", "JOIN", "ONLINE")

	var out, in observe.VisualScreenID
	for i := 0; i < 5; i++ {
		out = v.ObserveWith(training, say, when)
	}
	for i := 0; i < 5; i++ {
		in = v.ObserveWith(freePlay, other, when)
	}
	if out == in {
		t.Fatalf("the two menus are both %s; the fixture is not separating them", out)
	}
	// BACK OUT AGAIN, to the screen Marco started on.
	var back observe.VisualScreenID
	for i := 0; i < 5; i++ {
		back = v.ObserveWith(training, say, when)
	}
	if back != out {
		t.Errorf("walking back arrived at %s rather than the screen it left, %s",
			back, out)
	}
	// AND THEN IN AGAIN, which is the repetition a route is read from.
	for i := 0; i < 5; i++ {
		in = v.ObserveWith(freePlay, other, when)
	}
	if in == back {
		t.Errorf("the submenu merged into %s on the second visit", back)
	}
	if n := len(v.Seen()); n != 2 {
		t.Errorf("%d screen(s) for two menus visited twice each: %+v", n, v.Seen())
	}
}

// AND A SCREEN WHOSE WORDS GREW IS NOT A NEW SCREEN.
//
// A menu that loads its labels in, or one OCR reads progressively better, says MORE over time
// rather than something else. Growth agrees with what was there; it is not a contradiction.
func TestAScreenThatSaysMoreThanItDidIsNotANewScreen(t *testing.T) {
	training, _ := twoMenus()
	few := read("TUTORIAL", "CUSTOM TRAINING")
	many := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE",
		"FREE PLAY", "CREATE", "PARTY")

	// THE CASE ONE SYMMETRIC NUMBER CANNOT TELL FROM A DIFFERENT SCREEN. By
	// min-over-max these agree at 0.300, and the two menus of the measured failure
	// agree at 0.231 -- no threshold separates those. What separates them is that
	// everything the screen said is still being said.
	if sim := observe.TextSimilarity(few, many); sim > 0.40 {
		t.Fatalf("the fixture's growth agrees at %.3f, which is not the hard case", sim)
	}
	kept, novel := observe.TextOverlap(few, many)
	if kept < observe.TextKept {
		t.Errorf("a growing screen keeps only %.2f of what it said", kept)
	}
	if novel <= observe.TextNovel {
		t.Errorf("the fixture's growth brings %.2f new words, so it would be "+
			"tolerated for the wrong reason", novel)
	}

	v := &observe.VisualScreens{}
	var id observe.VisualScreenID
	for i := 0; i < 4; i++ {
		id = v.ObserveWith(training, few, when)
	}
	settled := id
	for i := 0; i < 6; i++ {
		if got := v.ObserveWith(training, many, when); got != settled {
			t.Fatalf("a screen that read more words became %s", got)
		}
	}
}

// AND A SCREEN READ LESS FULLY IS NOT A NEW SCREEN EITHER.
//
// The other half of the asymmetry, and the commoner case: OCR resolves six labels on one frame
// and three on the next because a transition dimmed the panel. Three of the screen's ten words
// is 0.30 kept -- below the line on its own -- and not one of them is new, so there is nothing
// here that contradicts anything.
func TestAScreenReadLessFullyIsNotANewScreen(t *testing.T) {
	training, _ := twoMenus()
	all := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE", "FREE PLAY")
	some := read("AERIAL", "STRIKER")

	v := &observe.VisualScreens{}
	var id observe.VisualScreenID
	for i := 0; i < 4; i++ {
		id = v.ObserveWith(training, all, when)
	}
	settled := id
	for i := 0; i < 6; i++ {
		if got := v.ObserveWith(training, some, when); got != settled {
			t.Fatalf("a screen read less fully became %s", got)
		}
	}
	if st := v.Stability(); st.TextRefusedMerge != 0 {
		t.Errorf("a partial reading refused %d merge(s)", st.TextRefusedMerge)
	}
}

// ── sparse text, which is the only kind there is ──────────────────────────────

// A REFUSAL SURVIVES THE UNREAD LOOKS BETWEEN READINGS.
//
// OCR costs hundreds of milliseconds against an observation cadence of a hundred, so most looks
// carry no reading. An unread look agrees with the standing screen by arrangement and would
// settle onto it, clearing the pending run on its way past -- so without the refusal being
// carried by the SCREEN, a reading that contradicts every third look refuses a thousand times
// and never separates anything.
//
// Measured on the Free Play recording at one reading in three, which is the ordinary density.
func TestARefusalSurvivesTheUnreadLooksBetweenReadings(t *testing.T) {
	training, freePlay := twoMenus()
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE")
	other := read("BACK", "CREATE", "FREE PLAY", "JOIN", "ONLINE")

	for _, every := range []int{1, 2, 3, 5} {
		t.Run(fmt.Sprintf("one reading in %d", every), func(t *testing.T) {
			v := &observe.VisualScreens{}
			at := func(i int, words observe.ScreenText) observe.ScreenText {
				if i%every == 0 {
					return words
				}
				return observe.ScreenText{}
			}
			var first observe.VisualScreenID
			for i := 0; i < 12; i++ {
				first = v.ObserveWith(training, at(i, say), when)
			}
			if first == observe.VisualScreenUnknown {
				t.Fatal("nothing settled")
			}
			var second observe.VisualScreenID
			for i := 0; i < 12; i++ {
				second = v.ObserveWith(freePlay, at(i, other), when)
			}
			if second == first {
				t.Fatalf("with one reading in %d the second menu was folded "+
					"into %s", every, first)
			}
			// AND THE SEPARATION COST ONE REFUSAL AND ONE PROMOTION, not one
			// refusal per look. A count near twelve means the run was being
			// refused over and over and never resolving.
			st := v.Stability()
			if st.TextRefusedMerge > 3 {
				t.Errorf("%d refusal(s) for one separation; the refusal is not "+
					"being carried", st.TextRefusedMerge)
			}
		})
	}
}

// ONE BAD READING REFUSES NOTHING AT ALL.
//
// A frame caught mid-transition, a panel dimmed, a title still animating: every one of those is
// ONE reading that shares nothing with the screen around it. Since a look's side of the
// comparison is the agreement of two consecutive readings, a single odd one has no words in it
// to contradict with -- the agreement is empty, and an empty reading is unread.
//
// This is what the measurement forced. Taking a single reading as the look's side split one main
// menu into eight identities over 449 looks, because each reading of it brought thirty fresh
// misreadings and every one of them counted as a word the screen had never said.
func TestOneOddReadingCostsAScreenNothingItKeeps(t *testing.T) {
	training, _ := twoMenus()
	v := &observe.VisualScreens{}
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE")
	odd := read("LOADING", "PLEASE", "WAIT")

	var id observe.VisualScreenID
	for i := 0; i < 4; i++ {
		id = v.ObserveWith(training, say, when)
	}
	settled := id
	for i := 0; i < 6; i++ {
		// One odd reading between good ones, over and over.
		if got := v.ObserveWith(training, odd, when); got != settled {
			t.Fatalf("one odd reading became %s", got)
		}
		if got := v.ObserveWith(training, say, when); got != settled {
			t.Fatalf("the screen split to %s after the reading agreed again", got)
		}
	}
	// AND EVERY OBJECTION IT RAISED WAS TAKEN BACK.
	//
	// [[ADR-199-one-reading-may-refuse-a-merge-and-may-not-mint-a-screen]] lets a single
	// reading refuse, so `loading please wait` -- which keeps nothing of what this screen
	// says and brings three words it has never said -- now raises an objection where it
	// raised none. The next reading of the screen underneath withdraws it, inside the
	// three looks the pending run needs, and the screen is never split. That outcome is
	// what this test was always about; the counter is how it gets there.
	st := v.Stability()
	if st.TextRefusedMerge != st.TextWithdrewRefusal {
		t.Errorf("single odd readings refused %d merge(s) and %d were withdrawn; a "+
			"refusal nothing takes back costs this screen its identity in three looks",
			st.TextRefusedMerge, st.TextWithdrewRefusal)
	}
}

// AND A READING THAT AGREES WITHDRAWS A REFUSAL TWO EARLIER ONES MADE.
//
// Two consecutive readings that say the same other thing DO refuse -- that is the rule, and it
// is how a real screen change is noticed. The refusal is then carried by the screen, so the
// evidence has to be able to take it back: two readings of a loading overlay followed by
// readings of the menu underneath must not cost the menu its identity.
func TestAReadingThatAgreesWithdrawsAnEarlierRefusal(t *testing.T) {
	training, _ := twoMenus()
	v := &observe.VisualScreens{}
	say := read("TUTORIAL", "CUSTOM TRAINING", "AERIAL", "STRIKER", "GOALIE")
	over := read("LOADING", "PLEASE", "WAIT", "CONNECTING")

	var id observe.VisualScreenID
	for i := 0; i < 4; i++ {
		id = v.ObserveWith(training, say, when)
	}
	settled := id
	// TWO consecutive readings of something else: enough to refuse.
	v.ObserveWith(training, over, when)
	v.ObserveWith(training, over, when)
	if st := v.Stability(); st.TextRefusedMerge == 0 {
		t.Fatal("two consecutive readings of something else refused nothing, so " +
			"this tests nothing")
	}
	// AND THEN THE MENU AGAIN, before the run could settle.
	for i := 0; i < 8; i++ {
		if got := v.ObserveWith(training, say, when); got != settled {
			t.Fatalf("the screen split to %s after the readings agreed again", got)
		}
	}
	if st := v.Stability(); st.TextWithdrewRefusal == 0 {
		t.Error("a later reading agreed and the run does not say the refusal was withdrawn")
	}
}

// ── the evidence type itself ──────────────────────────────────────────────────

// MISSING TEXT, READ-AND-SILENT, AND READ-WITH-WORDS ARE THREE STATES.
func TestNobodyLookedIsNotTheSameAsNothingWasThere(t *testing.T) {
	none := observe.ScreenText{}
	silent := observe.TextOf(directorapi.SourceOCR, nil)
	spoke := read("FREE PLAY")

	for _, tc := range []struct {
		name       string
		t          observe.ScreenText
		read, said bool
	}{
		{"nobody looked", none, false, false},
		{"read, and the screen had no words", silent, true, false},
		{"read, with words", spoke, true, true},
	} {
		if got := tc.t.Read(); got != tc.read {
			t.Errorf("%s: Read is %v", tc.name, got)
		}
		if got := tc.t.Said(); got != tc.said {
			t.Errorf("%s: Said is %v", tc.name, got)
		}
	}
	// AND UNREAD TEXT IS NEVER ALIKE ANYTHING, including other unread text.
	if sim := observe.TextSimilarity(none, none); sim != 0 {
		t.Errorf("two unread readings agree at %.3f", sim)
	}
	if sim := observe.TextSimilarity(silent, silent); sim != 0 {
		t.Errorf("two silent readings agree at %.3f; nothing is known about either",
			sim)
	}
	if sim := observe.TextSimilarity(spoke, spoke); sim != 1 {
		t.Errorf("one reading against itself agrees at %.3f", sim)
	}
}

// AND A SCREEN NEEDS ENOUGH OF A VOCABULARY TO BE RECOGNISED BY IT AT ALL.
//
// The one predicate three places ask, because a weaker version of it in any one of them is how a
// demonstration is presented as ready and then waits forever: the run refuses to identify a
// screen it cannot tell from another, so a recorder or a review that called such a screen fine
// was promising something the run would not honour. `Read` and `Said` are both true of a screen
// with two words on it, and two words tell nothing apart.
func TestAScreenNeedsEnoughWordsToBeRecognisedByThem(t *testing.T) {
	for _, tc := range []struct {
		name       string
		t          observe.ScreenText
		identifies bool
	}{
		{"nobody looked", observe.ScreenText{}, false},
		{"read, and the screen had no words",
			observe.TextOf(directorapi.SourceOCR, nil), false},
		{"one word", read("AUDIO"), false},
		{"two words", read("AUDIO", "VIDEO"), false},
		{"three words", read("AUDIO", "VIDEO", "DISPLAY"), true},
		{"a whole menu", read("AUDIO", "VIDEO", "DISPLAY", "CONTROLS",
			"PROFILE"), true},
	} {
		if got := tc.t.Identifies(); got != tc.identifies {
			t.Errorf("%s: Identifies is %v, want %v", tc.name, got,
				tc.identifies)
		}
	}
	// AND IT IS THE SAME NUMBER A DISAGREEMENT NEEDS, which is the point: a screen
	// that cannot be on either side of "these are different screens" cannot be
	// positively identified either.
	if observe.TextDisagreementWords != 3 {
		t.Errorf("the words it takes to be a claim is %d, and the cases above are "+
			"written for 3", observe.TextDisagreementWords)
	}
}

// AND A DISAGREEMENT NEEDS ENOUGH ON BOTH SIDES TO BE ABOUT ANYTHING.
func TestTooFewWordsIsNotADisagreement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		a, b  observe.ScreenText
		apart bool
	}{
		{"two words against two", read("BACK", "JOIN"), read("AERIAL", "GOALIE"), false},
		{"three against three", read("BACK", "JOIN", "ONLINE"),
			read("AERIAL", "GOALIE", "STRIKER"), true},
		{"three against two", read("BACK", "JOIN", "ONLINE"),
			read("AERIAL", "GOALIE"), false},
		{"nothing against three", observe.ScreenText{},
			read("AERIAL", "GOALIE", "STRIKER"), false},
	} {
		got, why := observe.TextsDisagree(tc.a, tc.b)
		if got != tc.apart {
			t.Errorf("%s: %v, want %v (%s)", tc.name, got, tc.apart, why)
		}
		if !got && why == "" {
			t.Errorf("%s: no disagreement and no reason", tc.name)
		}
	}
}

// AND A WORD IS A WORD THE ONE WAY THIS PACKAGE SPLITS THEM.
func TestWordsAreSplitOneWay(t *testing.T) {
	// "12" and "I l x" are gone: nothing shorter than three characters is a word, which
	// is where most of OCR's output from a dark game menu goes. A screen whose only
	// distinguishing text is a two-character label cannot be told apart this way, and the
	// honest answer there is that the arrangement is all there is.
	got := observe.TextWords("  CUSTOM  TRAINING: 12 kills, I l xy")
	want := []string{"custom", "training", "kills"}
	if len(got) != len(want) {
		t.Fatalf("%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v, want %v", got, want)
		}
	}
}
