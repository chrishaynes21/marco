package workflowstore_test

// WHAT A WORKFLOW MAY CONCLUDE FROM THE LOOK IN FRONT OF IT.
//
// The pair here is the measured one: Rocket League's Play page and the Training page it leads
// to, whose arrangements sit inside the merge threshold's own resolution, with the vocabularies
// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] scored. The standing screen
// is deliberately the SAME id before and after, because that is the whole case — the global
// observer merged the two and recorded no change, and the question is whether the workflow can
// still tell where it is.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The saved arrangements, taken from the measured artifact.
var (
	savedMenu = observe.VisualArrangement{Total: 236, Cells: map[string]int{
		"r@0,0": 56, "r@0,1": 28, "r@0,2": 27,
		"r@1,0": 35, "r@1,1": 11, "r@1,2": 3,
		"r@2,0": 48, "r@2,1": 21, "r@2,2": 7}}
	savedPlay = observe.VisualArrangement{Total: 136, Cells: map[string]int{
		"r@0,0": 4, "r@0,1": 20, "r@0,2": 38,
		"r@1,0": 3, "r@1,1": 8, "r@1,2": 31,
		"r@2,0": 10, "r@2,1": 15, "r@2,2": 7}}
	savedTraining = observe.VisualArrangement{Total: 99, Cells: map[string]int{
		"r@0,0": 3, "r@0,1": 26, "r@0,2": 2,
		"r@1,0": 1, "r@1,1": 17, "r@1,2": 7,
		"r@2,0": 13, "r@2,1": 30}}

	// lookOnTraining is a DIFFERENT look at the Training page: the same structure at a
	// slightly different region yield, which is what the proposer actually varies by.
	lookOnTraining = observe.VisualArrangement{Total: 96, Cells: map[string]int{
		"r@0,0": 4, "r@0,1": 24, "r@0,2": 3,
		"r@1,0": 1, "r@1,1": 19, "r@1,2": 6,
		"r@2,0": 11, "r@2,1": 28}}
)

// What each page was read to say, from the artifact the measurement re-saved.
func readOut(phrases ...string) observe.ScreenText {
	return observe.TextOf(directorapi.SourceOCR, phrases)
}

var (
	wordsMenu     = readOut("CHALLENGES", "CLUB", "COMPETITIVE", "CREATE", "GARAGE", "MATCH", "ONLINE", "PARTY", "PLAY", "PROFILE", "SETTINGS", "SHOP")
	wordsPlay     = readOut("PLAY", "OFFLINE", "PRIVATE", "MATCH")
	wordsTraining = readOut("AERIAL", "GOALIE", "STRIKER")

	// readingOfTraining is a look at the Training page as OCR actually renders it.
	readingOfTraining = readOut("TUTORIAL", "AERIAL", "CREATE", "PARTY", "STRIKER", "GOALIE")
	// readingOfSubmenu is the page AFTER it, which the arrangement cannot refuse.
	readingOfSubmenu = readOut("FREE", "PLAY", "CREATE", "ONLINE", "FREE", "PLAY", "JOIN")
	// readingTooThin resolved one label of the Training page and nothing else: it cannot
	// contradict anything and it does not identify anything either.
	readingTooThin = readOut("AERIAL")
)

// twoPagesApart is the demonstrated walk main menu -> Play -> Training, saved WITH what each
// screen was read to say.
func twoPagesApart(t *testing.T, trainingArr observe.VisualArrangement,
	trainingWords observe.ScreenText) workflowstore.Artifact {

	t.Helper()
	return pagesSaved(t, savedPlay, trainingArr, trainingWords)
}

// pagesSaved is the same walk with both ends' saved arrangements chosen, for the one test that
// needs a destination the demonstration could not tell from its source.
func pagesSaved(t *testing.T, playArr, trainingArr observe.VisualArrangement,
	trainingWords observe.ScreenText) workflowstore.Artifact {

	t.Helper()
	d := workflow.Draft{Name: "enter free play"}
	step := func(from, to observe.VisualScreenID, target observe.VisualTargetID) workflow.Step {
		return workflow.Step{
			From: from, To: to, Target: target, Cause: observe.CauseClick, Seen: 2,
			Readings: []observe.Region{{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04}},
		}
	}
	d.Steps = []workflow.Step{
		step("visual_1", "visual_2", "target_1"),
		step("visual_2", "visual_3", "target_2"),
	}
	texts := map[observe.VisualScreenID]observe.ScreenText{
		"visual_1": wordsMenu,
		"visual_2": wordsPlay,
	}
	if trainingWords.Read() {
		texts["visual_3"] = trainingWords
	}
	a, err := workflowstore.SaveWith(d,
		map[observe.VisualScreenID]observe.VisualArrangement{
			"visual_1": savedMenu,
			"visual_2": playArr,
			"visual_3": trainingArr,
		}, texts, saved)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// onThePlayPage is a session standing on the Play page, established as `visual_2` by somebody
// who looked — which is what step 0 of the live run achieved.
func onThePlayPage(t *testing.T) *workflowstore.SessionBinding {
	t.Helper()
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_b", "visual_2", 4, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	return s
}

// THE PREMISE, asserted rather than assumed.
//
// Everything below rests on the Training page's own look agreeing with the saved Training page
// and not with the saved Play page. If `ArrangementSimilarity` ever changes shape, this is the
// test that should say so first, instead of five tests failing for a reason none of them names.
func TestTheLookThisRuleRestsOnAgreesWithTheDestinationAndNotTheSource(t *testing.T) {
	toward := observe.ArrangementSimilarity(savedTraining, lookOnTraining)
	away := observe.ArrangementSimilarity(savedPlay, lookOnTraining)
	if toward < observe.VisualScreenMatch {
		t.Fatalf("a look at the Training page agrees with the saved Training page at "+
			"%.3f, below the %.2f this fixture needs", toward, observe.VisualScreenMatch)
	}
	if away >= toward {
		t.Fatalf("a look at the Training page agrees with the saved Play page at %.3f, "+
			"not below the %.3f it agrees with the Training page at", away, toward)
	}
	if apart, why := observe.TextsDisagree(wordsTraining, readingOfTraining); apart {
		t.Fatalf("a reading of the Training page contradicts the saved Training page: %s", why)
	}
	if apart, _ := observe.TextsDisagree(wordsTraining, readingOfSubmenu); !apart {
		t.Fatal("a reading of the Free Play submenu does not contradict the saved " +
			"Training page, so the corroboration term cannot be tested")
	}
}

// THE CASE THIS EXISTS FOR: the destination is established although the global screen is
// unchanged.
func TestAWorkflowReachesItsDestinationWhileTheGlobalScreenIsUnchanged(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)

	// THE SAME SCREEN ID, before and after. The observer merged the two pages and
	// recorded no change, which is why the epoch has not moved either.
	b, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1)
	if err != nil {
		t.Fatalf("the destination was not established: %v", err)
	}
	if b.Ground != workflowstore.GroundReached {
		t.Errorf("ground is %q, want %q", b.Ground, workflowstore.GroundReached)
	}
	if b.Screen != "visual_2" {
		t.Errorf("bound to %q; a reached arrival names the screen STANDING, because "+
			"the global model has not got a second identity for it", b.Screen)
	}
	if !b.Positive {
		t.Error("a reading that keeps every word of the destination's vocabulary is " +
			"being reported as merely not contradicting it")
	}
	if b.Thin {
		t.Error("three saved words is the floor for a contradiction, not below it")
	}
	if b.Depth != 1 {
		t.Errorf("depth %d, want one step from the confirmation it followed", b.Depth)
	}

	// AND THE ARRIVAL READS IT BACK, for the step whose destination it is.
	got := a.DestinationAt(1, s, workflowstore.Now{Screen: "visual_2", Epoch: 4, Generation: 1})
	if !got.Established() {
		t.Fatalf("the step's destination is not established: %s", got.Because)
	}
	if got.Independent() {
		t.Error("a destination the workflow reached on its own expectation is being " +
			"reported as independent evidence")
	}
	if !got.Positive {
		t.Error("the arrival did not carry the reading's strength through")
	}
	if ok, why := got.Verified(); ok || !strings.Contains(why, "not by itself proof") {
		t.Errorf("arriving is being treated as the action being verified: %v %s", ok, why)
	}
}

// AND IT IS STILL NOT EVIDENCE THAT THE ACTION WORKED, which is the sentence this whole
// subsystem is organised around.
func TestAReachedArrivalIsNeverAnActionVerified(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)
	if _, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1); err != nil {
		t.Fatal(err)
	}
	got := a.DestinationAt(1, s, workflowstore.Now{Screen: "visual_2", Epoch: 4, Generation: 1})
	if ok, _ := got.Verified(); ok {
		t.Fatal("a reached destination is being reported as an action independently " +
			"verified")
	}
	if !strings.Contains(got.Because, "weaker than somebody looking") {
		t.Errorf("the reason does not say what it is weaker than: %s", got.Because)
	}
}

// THE SECOND TERM: no closer to the destination than to the page the step started on.
//
// Two saved screens the demonstration could not tell apart cannot be told apart now either, and
// the rule must say so rather than pick the one the workflow was hoping for.
func TestADestinationNoCloserThanItsSourceIsNotAnArrival(t *testing.T) {
	// BOTH ENDS SAVED AS ONE SHAPE, so the look agrees with the destination well above
	// the threshold and agrees with the source exactly as well.
	a := pagesSaved(t, savedTraining, savedTraining, wordsTraining)
	s := onThePlayPage(t)

	_, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1)
	if err == nil {
		t.Fatal("a destination that is no closer than its source was established anyway")
	}
	if !strings.Contains(err.Error(), "no more the destination") {
		t.Errorf("the refusal does not say which term failed: %v", err)
	}
	if _, ok := s.Established("screen_c", 4, 1); ok {
		t.Error("a refused arrival left a binding behind")
	}
}

// THE THIRD TERM: the arrangement agrees and the words say otherwise.
//
// The measured case for this is the Free Play submenu, which agrees with the saved Training page
// on 21 of 28 looks because it replaces one panel of it. Nothing but the reading refuses it.
func TestAReadingThatContradictsTheDestinationRefusesTheArrival(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)

	_, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfSubmenu, 4, 1)
	if err == nil {
		t.Fatal("an arrival was established on a look whose words said it was " +
			"somewhere else")
	}
	if !strings.Contains(err.Error(), "says otherwise") {
		t.Errorf("the refusal does not say the reading disagreed: %v", err)
	}
}

// AND THE THIRD TERM IS REQUIRED, not consulted when a reading happens to be there.
func TestAnArrangementAloneDoesNotArriveAnywhere(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)

	_, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		observe.ScreenText{}, 4, 1)
	if err == nil {
		t.Fatal("an arrival was established with nothing read on the look; the " +
			"arrangement alone was measured to arrive on the wrong page")
	}
	if !strings.Contains(err.Error(), "nothing has been read") {
		t.Errorf("the refusal does not say what was missing: %v", err)
	}
}

// A DESTINATION NOBODY READ CANNOT BE CORROBORATED, and must not be treated as agreeing.
//
// `Missing evidence is not contradictory evidence` read the other way round: it is not
// corroboration either. A screen saved with no words can never be contradicted, so accepting it
// here would make the required term vacuous for exactly the artifacts that have no text at all.
func TestADestinationSavedWithoutWordsIsNotReached(t *testing.T) {
	a := twoPagesApart(t, savedTraining, observe.ScreenText{})
	s := onThePlayPage(t)

	_, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1)
	if err == nil {
		t.Fatal("a destination with no saved words was established on a reading that " +
			"could not have refused it")
	}
	if !strings.Contains(err.Error(), "without any words") {
		t.Errorf("the refusal does not name the missing evidence: %v", err)
	}
}

// POSITIVELY IDENTIFIED, OR MERELY NOT CONTRADICTED.
//
// The owner asked for these to be reported apart: OCR that does not contradict a destination is
// weaker than OCR that says it is that destination, and the weakest case of all is a destination
// whose saved vocabulary is too short to refuse anything.
func TestWhatAReadingHasToSayBeforeAnArrivalIsIdentifiedRatherThanUncontradicted(t *testing.T) {
	for _, c := range []struct {
		name             string
		savedWords       observe.ScreenText
		read             observe.ScreenText
		positive, thin   bool
		sentenceContains string
	}{{
		name:       "a reading that keeps every word of the destination identifies it",
		savedWords: wordsTraining, read: readingOfTraining,
		positive: true, thin: false,
		sentenceContains: "keeps 100% of what",
	}, {
		name:       "a reading that resolved one label only fails to contradict it",
		savedWords: wordsTraining, read: readingTooThin,
		positive: false, thin: false,
		sentenceContains: "only fails to contradict it",
	}, {
		name:       "a destination of two words could not have refused anything",
		savedWords: readOut("AERIAL", "GOALIE"), read: readingOfTraining,
		positive: true, thin: true,
		sentenceContains: "fewer than the 3 it takes to contradict",
	}} {
		t.Run(c.name, func(t *testing.T) {
			a := twoPagesApart(t, savedTraining, c.savedWords)
			s := onThePlayPage(t)
			b, err := s.Reached(a, "screen_b", "screen_c", "visual_2",
				lookOnTraining, c.read, 4, 1)
			if err != nil {
				t.Fatalf("not established: %v", err)
			}
			if b.Positive != c.positive {
				t.Errorf("Positive is %v, want %v: %s", b.Positive, c.positive,
					b.Because)
			}
			if b.Thin != c.thin {
				t.Errorf("Thin is %v, want %v: %s", b.Thin, c.thin, b.Because)
			}
			if !strings.Contains(b.Because, c.sentenceContains) {
				t.Errorf("the reason does not say how strong the reading was "+
					"(%q): %s", c.sentenceContains, b.Because)
			}
		})
	}
}

// AND A WEAKER CORROBORATION SAYS SO WHERE SOMEBODY READS IT.
func TestAnArrivalOnlyUncontradictedSaysSoInTheArrival(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)
	if _, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingTooThin, 4, 1); err != nil {
		t.Fatal(err)
	}
	got := a.DestinationAt(1, s, workflowstore.Now{Screen: "visual_2", Epoch: 4, Generation: 1})
	if got.Positive {
		t.Fatal("a reading that only failed to contradict the destination is being " +
			"reported as having identified it")
	}
	if !strings.Contains(got.Because, "weaker again than a reading that said this is") {
		t.Errorf("the arrival does not say the corroboration was the weaker kind: %s",
			got.Because)
	}
}

// IT DOES NOT OVERWRITE SOMEBODY WHO LOOKED.
func TestAReachedArrivalDoesNotDisplaceAConfirmation(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)
	if err := s.Confirm("screen_c", "visual_9", 4, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	b, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1)
	if err == nil {
		t.Fatal("a reached arrival overwrote a screen somebody had confirmed")
	}
	if b.Ground != workflowstore.GroundConfirmed || b.Screen != "visual_9" {
		t.Errorf("the standing confirmation was not returned intact: %+v", b)
	}
	if held, _ := s.Established("screen_c", 4, 1); held.Ground != workflowstore.GroundConfirmed {
		t.Errorf("the binding is now %q, and a person said otherwise", held.Ground)
	}
}

// A STEP THE WORKFLOW DOES NOT DEMONSTRATE IS NOT A DESTINATION TO ARRIVE AT.
func TestAnUndemonstratedStepIsNotReached(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)

	// screen_a is where the walk STARTED; nothing demonstrates the Play page leading
	// back to it.
	_, err := s.Reached(a, "screen_b", "screen_a", "visual_2", savedMenu,
		readingOfTraining, 4, 1)
	if err == nil {
		t.Fatal("a destination nothing demonstrates was established")
	}
	if !strings.Contains(err.Error(), "does not demonstrate a step") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// AND A SESSION WITH NO ANCHOR HAS NOT ARRIVED ANYWHERE.
func TestAnArrivalNeedsSomewhereToHaveArrivedFrom(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := workflowstore.NewSessionBinding()

	_, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1)
	if err == nil {
		t.Fatal("a destination was reached from a screen nothing had established")
	}
	if !strings.Contains(err.Error(), "no step to have arrived from") {
		t.Errorf("the refusal does not say what was missing: %v", err)
	}
}

// A BINDING STILL DIES WITH ITS OBSERVATION.
//
// A reached arrival is evidence about one look. Nothing about reading it from the look rather
// than from a recorded change makes it durable.
func TestAReachedArrivalDoesNotSurviveTheObservationItWasMadeIn(t *testing.T) {
	a := twoPagesApart(t, savedTraining, wordsTraining)
	s := onThePlayPage(t)
	if _, err := s.Reached(a, "screen_b", "screen_c", "visual_2", lookOnTraining,
		readingOfTraining, 4, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Established("screen_c", 5, 1); ok {
		t.Error("a reached arrival outlived the observation generation it was made in")
	}
	if _, ok := s.Established("screen_c", 4, 2); ok {
		t.Error("a reached arrival outlived the window it was made under")
	}
}
