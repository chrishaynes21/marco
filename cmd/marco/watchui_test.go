package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE BOUNDARY THIS SURFACE HAS TO HOLD.
//
// The dogfood strip is a window onto two things the Director already knows — whether it is
// watching, and what its memory has committed — plus one door it already had, marco do. Every test
// here says the same thing from a different side: the page may ASK, and may not DECIDE.

// A SURFACE MAY ASK MARCO TO LOOK. IT MAY NOT WRITE DOWN WHAT MARCO SAW.
//
// The four verbs are attention and permission. There is no fifth, and an unrecognised one is a
// read rather than the nearest match — two of these change what Marco is doing, and routing a typo
// to one of them would turn a mistyped word into a decision about somebody's memory.
//
// It also states the shutdown rule: the ONLY request that stops watching is the one somebody
// pressed Stop for. Nothing on this surface disables ambient observation on a page close, a
// navigation or a failed read, so closing the control centre leaves Marco doing what it was doing.
func TestTheDogfoodStripOnlyAsksAboutAttentionAndPermission(t *testing.T) {
	for _, c := range []struct {
		verb  string
		want  service.ObserveAmbient
		press bool
	}{
		{"watch", service.ObserveAmbient{Enable: true}, true},
		{"learn", service.ObserveAmbient{Learn: true}, true},
		{"stop", service.ObserveAmbient{Disable: true}, false},
		{"unlearn", service.ObserveAmbient{Unlearn: true}, false},
		// A read. Not a verb, not the nearest verb, and it starts nothing.
		{"", service.ObserveAmbient{}, false},
		{"learned", service.ObserveAmbient{}, false},
		{"learn; establish", service.ObserveAmbient{}, false},
	} {
		got, press := watchingRequest(c.verb)
		if got != c.want {
			t.Errorf("watchingRequest(%q) = %+v, want %+v", c.verb, got, c.want)
		}
		if press != c.press {
			t.Errorf("watchingRequest(%q) press = %v, want %v", c.verb, press, c.press)
		}
		// Evidence is the one ambient field that names controls and screens, and this strip
		// never asks for it. It reports counts.
		if got.Evidence {
			t.Errorf("watchingRequest(%q) asked for the evidence ledger", c.verb)
		}
	}
}

// THE FEED ASKS FROM WHERE IT GOT TO.
//
// A cursor, and nothing but a cursor. The nil fields are the point: this request cannot start an
// observation, cannot begin a demonstration and cannot write to memory — so a poll on a timer,
// which is what this is, changes nothing about what Marco does however often it fires.
func TestTheFeedAsksFromWhereItGotTo(t *testing.T) {
	q := learnedRequest("41")
	if q.Learning == nil || q.Learning.After != 41 {
		t.Fatalf("learnedRequest(41).Learning = %+v, want After 41", q.Learning)
	}
	// EVERY OTHER FIELD NIL. Walked by reflection rather than named, so a field added to
	// ObserveQuery next year is covered by this test on the day it appears.
	v := reflect.ValueOf(q)
	for i := 0; i < v.NumField(); i++ {
		if f := v.Type().Field(i); f.Name != "Learning" && !v.Field(i).IsZero() {
			t.Errorf("the feed's request also set %s — it may ask for nothing else", f.Name)
		}
	}
	// An unreadable cursor asks for everything still held, never for nothing: a page that
	// showed the ring twice is untidy, one that silently skipped to the end would let
	// somebody conclude Marco learned nothing while they were away.
	if bad := learnedRequest("not a number"); bad.Learning.After != 0 {
		t.Errorf("a bad cursor became %d, want 0", bad.Learning.After)
	}
}

// TRY IT GOES THROUGH THE DOOR A TYPED marco do USES.
//
// The whole product requirement in one assertion: the words are handed to the engine, and the
// engine does intake, planning, compilation to legal Marco, the authority check, the actuation
// lease and verification. This process starts no play, resolves no phrase and takes no lease — if
// it did, it would be a second intake, and it would be the one the person believed.
//
// The spawn seam is what makes this safe to run at all: marco do performs REAL INPUT, and nothing
// in a test may ever actually start it.
func TestTryItRunsThePhraseThroughMarcoDo(t *testing.T) {
	var argv []string
	old := runSpawn
	runSpawn = func(a []string) (*exec.Cmd, io.Reader, error) { argv = a; return nil, nil, nil }
	defer func() { runSpawn = old }()

	e := &editor{}
	w := httptest.NewRecorder()
	e.handleTry(w, jsonPost("/api/try", `{"text":"open mouse settings"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/try = %d: %s", w.Code, w.Body.String())
	}
	if len(argv) == 0 {
		t.Fatal("Try it started nothing — it must spawn marco do")
	}
	if argv[0] != "do" {
		t.Errorf("argv[0] = %q, want do", argv[0])
	}
	if argv[len(argv)-1] != "open mouse settings" {
		t.Errorf("the phrase did not reach the engine: %q", argv)
	}
	// The press happened in the control centre and says so. A source is how the engine tells
	// a clicked run from a spoken one, and inventing a new one here would make this surface
	// invisible to everything that reasons about where an invocation came from.
	if !strings.Contains(strings.Join(argv, " "), "--source=") {
		t.Errorf("Try it did not say where it came from: %q", argv)
	}
	// NO PLAY IDENTITY. Try it carries the person's words; resolving them to a play is the
	// engine's job, and a surface that picked the play would be choosing on their behalf.
	for _, a := range argv {
		if strings.HasPrefix(a, "--play=") {
			t.Errorf("Try it resolved the phrase itself: %q", argv)
		}
	}
}

// AND IT ASKS FOR NOTHING WHEN NOTHING WAS SAID.
func TestTryItWithNoWordsStartsNothing(t *testing.T) {
	started := false
	old := runSpawn
	runSpawn = func([]string) (*exec.Cmd, io.Reader, error) { started = true; return nil, nil, nil }
	defer func() { runSpawn = old }()

	e := &editor{}
	w := httptest.NewRecorder()
	e.handleTry(w, jsonPost("/api/try", `{"text":"   "}`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty Try it = %d, want 400", w.Code)
	}
	if started {
		t.Error("an empty Try it spawned a play")
	}
}

// THE FEED CANNOT CARRY WHAT MARCO SAW, ONLY WHAT IT CONCLUDED.
//
// Walked by reflection rather than by reading the struct, because the risk this guards is somebody
// adding a helpful field — the window title, the text that matched, the region it came from — to
// make a future surface easier to write. Screenshots, OCR transcripts, typed text and coordinates
// have no way to a person's screen through this path, and this is where that is held.
func TestTheFeedCarriesNoScreenContent(t *testing.T) {
	allowed := map[string]bool{
		"Change": true, "Kind": true, "Application": true, "Description": true,
	}
	et := reflect.TypeOf(service.LearningEvent{})
	for i := 0; i < et.NumField(); i++ {
		if !allowed[et.Field(i).Name] {
			t.Errorf("LearningEvent gained %s — the feed reports a conclusion, not a reading",
				et.Field(i).Name)
		}
	}
}

// THE WHOLE LOOP IS ON ONE PAGE.
//
// The reason this test exists: the first dogfood was run out of two consoles, one holding the
// Director and one printing the feed, with a person alt-tabbing between them to find out whether
// their assistant had learned anything. That is a test of the plumbing. Watching, what was
// committed, and a way to use it have to be reachable where a person is already looking, or the
// product IS the plumbing.
//
// Deleting any one of the three must fail this.
func TestTheDogfoodLoopIsOnOneSurface(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for what, marker := range map[string]string{
		"the watching strip":    `id="wbar"`,
		"turning learning on":   `watchVerb('learn')`,
		"what was just learned": `id="lbox"`,
		"a way to try it":       `tryIt()`,
	} {
		if !strings.Contains(here, marker) {
			t.Errorf("the Here view no longer offers %s (%s)", what, marker)
		}
	}
	for what, marker := range map[string]string{
		"the watching read":       `/api/watching`,
		"the committed-feed read": `/api/learned?after=`,
		"the canonical run":       `/api/try`,
	} {
		if !strings.Contains(editPage, marker) {
			t.Errorf("the page no longer reaches %s (%s)", what, marker)
		}
	}
	// AND IT LANDED ON HERE, not on a tenth view. A new tab would be a second place to look,
	// which is the thing being fixed.
	if strings.Contains(editPage, `data-view="live"`) || strings.Contains(editPage, `data-view="dogfood"`) {
		t.Error("the loop grew its own view instead of landing where a person already looks")
	}
}

// A COLD MACHINE RENDERS.
//
// Both reads answer 200 with a state the page can draw, and neither starts a Director to do it —
// the temp home makes sure they cannot reach a real one. "Marco is not running" is a state this
// strip draws, not a failure of the page.
func TestTheDogfoodStripRendersWithNoDirector(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	for path, h := range map[string]http.HandlerFunc{
		"/api/watching": handleWatching,
		"/api/learned":  handleLearned,
	} {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, w.Code, w.Body.String())
		}
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("%s: decode: %v (%s)", path, err, w.Body.String())
		}
		if got["available"] != false {
			t.Errorf("%s available = %v, want false", path, got["available"])
		}
	}
}

func jsonPost(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// EVERY DOOR THE PAGE KNOCKS ON IS ANSWERED.
//
// # Why this test is shaped like this
//
// Three times on this branch a mechanism has been written, wired, and reached by nothing a person
// actually uses. The pattern is always the same: the code is complete, a test proves the code
// works, and no test enters where production enters — so the handler and the page agree only
// because somebody remembered to make them.
//
// So this reads the page, finds every /api/... the JavaScript fetches, and asks the REAL mux
// whether it serves that path. It names nothing itself: an endpoint added to the page next month
// is covered on the day it is added, and one deleted from the mux fails here rather than at a
// person's first click.
//
// Mutation: delete watchAPI(mux, ed) from serveMux. Three paths go unanswered.
func TestEveryDoorThePageKnocksOnIsAnswered(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	mux := (&editor{}).serveMux()

	seen := map[string]bool{}
	for _, quote := range []string{"'", `"`} {
		rest := editPage
		for {
			i := strings.Index(rest, quote+"/api/")
			if i < 0 {
				break
			}
			rest = rest[i+1:]
			j := strings.Index(rest, quote)
			if j < 0 {
				break
			}
			path := rest[:j]
			// A fetch template — '/api/learn/'+v or '/api/run?id='+id — carries its
			// query and its concatenation with it. The PATH is what the mux routes on.
			if k := strings.IndexAny(path, "?"); k >= 0 {
				path = path[:k]
			}
			seen[path] = true
		}
	}
	if len(seen) < 10 {
		t.Fatalf("only found %d endpoints in the page — the scan broke, not the wiring", len(seen))
	}
	for path := range seen {
		// A trailing slash is a fetch prefix the page appends a verb to (/api/learn/ + stop).
		// Ask about the prefix itself; ServeMux matches the registered pattern either way.
		ask := strings.TrimSuffix(path, "/")
		_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, ask, nil))
		// THE CATCH-ALL IS NOT AN ANSWER. An unregistered /api path falls through to "/",
		// which serves the page itself with a 200 — so a fetch for missing JSON would come
		// back as HTML and look like a parse error rather than a missing handler. Matching
		// the root is exactly the failure this test exists to catch.
		if pattern == "" || pattern == "/" {
			t.Errorf("the page fetches %s and nothing serves it", path)
		}
	}
	// AND THE THREE THIS PHASE ADDED, named explicitly. The scan above proves the page and
	// the mux agree; these say WHICH doors the dogfood loop cannot lose.
	for _, path := range []string{"/api/watching", "/api/learned", "/api/try"} {
		if !seen[path] {
			t.Errorf("the page stopped fetching %s", path)
		}
	}
}

// ── the product states, on the page ───────────────────────────────────────────

// THREE STATES, ONE PRIMARY STATUS, AND NO PEER SWITCHES.
//
// # What the first dogfood found
//
// The strip said MARCO IS watching, with a separate "Also learn" beside it, and a person reading
// it had to combine two switches in their head to work out what their assistant was doing. Worse,
// the two read as peers — as though learning without watching were a thing somebody might choose —
// when the relationship is a CONTAINMENT: Marco can watch without learning, and cannot learn
// without watching.
//
// So the page names three states and exactly three, each with one status line and one sentence
// saying what Marco is doing rather than which permission is set. `watchState` is the single
// function that picks one, so the status and the buttons can never disagree.
//
// Deleting a state, or letting the strip say "watching and learning" from the learning flag alone,
// must fail this.
func TestTheStripNamesThreeProductStates(t *testing.T) {
	for _, want := range []string{"Not watching", "Watching &amp; learning", "Watching"} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the strip cannot say %q", want)
		}
	}
	// THE ONE FUNCTION THAT DECIDES. Both the status and the buttons read it, so a state
	// cannot be drawn with another state's controls.
	if !strings.Contains(editPage, "function watchState(s)") {
		t.Fatal("the strip has no single function naming the product state, so the status " +
			"line and the buttons can disagree about what Marco is doing")
	}
	if !strings.Contains(editPage, "const st=watchState(s)") {
		t.Error("the strip renders from something other than the one state function")
	}
}

// LEARNING WITHOUT WATCHING IS NOT A VALID PRODUCT STATE, AND IS NOT DRESSED UP AS ONE.
//
// The Director makes it unreachable — stopping watching stops learning, held by
// cmd/director TestStoppingWatchingStopsLearning. This is the other half: if a Director ever
// reports the combination anyway, the strip says the state is inconsistent rather than rendering
// it as an ordinary mode somebody chose.
//
// Deleting the broken arm — falling through to "watching and learning", or to "not watching" —
// must fail this.
func TestTheStripHasNoLearningWithoutWatchingState(t *testing.T) {
	if !strings.Contains(editPage, "if(s.learning && !s.watching) return 'broken';") {
		t.Fatal("the strip does not detect learning-without-watching. A surface that drew " +
			"that combination as a normal mode would be teaching somebody a state the " +
			"product does not have.")
	}
	// AND THE ORDER MATTERS: the check has to come before the ones that would happily draw
	// it as watching-and-learning.
	broken := strings.Index(editPage, "if(s.learning && !s.watching) return 'broken';")
	learn := strings.Index(editPage, "if(s.learning) return 'learn';")
	if learn < 0 || broken > learn {
		t.Error("the inconsistent case is decided after the ordinary learning case, so it " +
			"can never be reached")
	}
	if !strings.Contains(editPage, "broken:{") {
		t.Error("there is no wording for the inconsistent state, so it would render blank")
	}
}

// ONE WATCH CONTROL ON THE PAGE.
//
// # The duplicate this removes
//
// The Here view carried TWO switches that both started observation and were both called Watch: the
// ambient strip, and Light Mode's own button inside the CURRENT panel. Either made recognition
// work, they reported different state, and a person reading the page had no way to tell which one
// "watching" referred to. That is Finding A of the first dogfood, and it is a product defect rather
// than a cosmetic one — the whole question this panel answers is what Marco is doing.
//
// The strip's Watch keeps an observation session running, which is what the CURRENT panel needs,
// so nothing was lost.
//
// Re-adding a second control must fail this.
func TestTheHereViewHasOneWatchControl(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for _, gone := range []string{`learnVerb('watch')`, `learnVerb('unwatch')`,
		`id="lwatch"`, `id="lunwatch"`} {
		if strings.Contains(here, gone) {
			t.Errorf("a second watch control is back on the Here view (%s). Two switches "+
				"both called Watch, both starting observation, is what the first "+
				"dogfood could not make sense of.", gone)
		}
	}
	// AND THE ONE THAT REMAINS IS THE AMBIENT STRIP, which is the one that can also say
	// whether Marco may remember.
	if !strings.Contains(here, `watchVerb('watch')`) || !strings.Contains(here, `watchVerb('stop')`) {
		t.Error("the Here view has no way to start or stop watching at all")
	}
}

// CURRENT IS WHAT MARCO SEES; JUST LEARNED IS WHAT IT WROTE DOWN.
//
// The distinction that replaces asking somebody to understand Observe and Learn as mechanisms. It
// is taught by BEHAVIOUR — watching, the CURRENT panel may say the screen calls itself Mouse;
// watching and learning, JUST LEARNED reports the commit — so both panels have to say which
// question they are answering.
//
// Deleting either label must fail this.
func TestThePageSaysWhichQuestionEachPanelAnswers(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for what, marker := range map[string]string{
		"NOW is what Marco sees":          "what Marco sees",
		"LAST RESULT is what it wrote":    "what Marco wrote down",
		"and the perceived name is shown": `id="lheresays"`,
	} {
		if !strings.Contains(here, marker) {
			t.Errorf("the Here view no longer says %s (%s)", what, marker)
		}
	}
	// And the line is actually filled in, from the perceived name and not from memory.
	if !strings.Contains(editPage, `this screen says it is`) ||
		!strings.Contains(editPage, "h.perceived") {
		t.Error("nothing renders what the screen itself says it is called, so \"Marco sees " +
			"Mouse\" and \"Marco remembered Mouse\" look identical")
	}
}

// ── learn must produce something a person can see ─────────────────────────────

// THE HERE VIEW ASKS WHAT MARCO MADE OF WHAT YOU JUST DID.
//
// # The silence this ends
//
// Second dogfood: Watch & Learn on, `Home → Bluetooth & devices → Mouse` walked twice with real
// clicks, and the page said nothing. The Director had the answer — `control_not_named`, with a
// sentence — and said it only to `marco observe --evidence`, in a terminal. A mode indicator with
// no observable result is a light, not a product.
//
// The request must be the EVIDENCE read and nothing else: it leaves every lifecycle verb false, so
// a poll on a timer cannot start watching, grant learning, or promote the candidate it is asking
// about.
//
// Deleting the Evidence field must fail this.
func TestTheHereViewAsksWhatMarcoMadeOfIt(t *testing.T) {
	q := madeRequest()
	if q.Ambient == nil || !q.Ambient.Evidence {
		t.Fatal("the made read does not ask for evidence, so the page could only ever show " +
			"counts — and 'Marco noticed seven relationships and learned none' is a true " +
			"report that tells somebody nothing they can act on")
	}
	if q.Ambient.Enable || q.Ambient.Disable || q.Ambient.Learn || q.Ambient.Unlearn {
		t.Errorf("the made read carries a lifecycle verb (%+v). Asking what Marco has "+
			"recorded about you must never be answered by recording more.", *q.Ambient)
	}
	if q.Learn != nil || q.Learning != nil {
		t.Error("the made read carries a request other than the ambient one")
	}
}

// AND IT RENDERS THE DIRECTOR'S OWN VERDICT, NEVER ITS OWN.
//
// Whether a traversal was learned is the policy's answer. A page that decided for itself would be
// a second policy in the place nobody reviews — and the one it would get wrong is the case that
// matters: a promoted candidate reports `never / already_known`, which read naively says "refused"
// about the very thing Marco just learned.
//
// Deleting the `learned` arm must fail this.
func TestTheOutcomeListReadsTheVerdictFromTheDirector(t *testing.T) {
	for _, want := range []string{"v.learned", "v.verdict", "v.said", "v.seen"} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the outcome list does not read %s from the Director", want)
		}
	}
	// THE PROMOTED CASE, NAMED. A candidate the policy has already admitted comes back as
	// `never / already_known` — the policy being asked about a thing it has finished with —
	// so a list that read the verdict alone would draw an X beside the one relationship
	// Marco actually learned, which is the worst sentence this panel could say.
	if !strings.Contains(editPage, "v.learned ? 'learned'") {
		t.Error("a candidate the Director marks learned is drawn from its verdict alone. A " +
			"promoted candidate reports `never / already_known`, so it would render as " +
			"a refusal of the very thing that was learned.")
	}
	if !strings.Contains(editPage, "MADETONE") || !strings.Contains(editPage, "never:'var(--err)'") {
		t.Error("a refusal is not drawn as a refusal, so 'couldn't learn this' would read " +
			"like everything else")
	}
	// NOT SILENCE when there is nothing yet: an empty panel reads as broken.
	if !strings.Contains(editPage, "Nothing yet") {
		t.Error("an empty outcome list renders blank rather than saying so")
	}
}

// AND THE GAP BETWEEN KNOWING A WAY AND BEING ABLE TO ASK FOR IT IS SHOWN, WITH THE WAY TO CLOSE IT.
//
// # Why the gap exists at all
//
// Ambient learning admits TOPOLOGY — places and the ways between them. It creates no goal, and
// `admitWatched` says so in as many words: a name for an outcome is a thing a person means, not a
// thing repetition implies. So after a perfectly successful traversal there is still nothing to
// Try, and until this existed there was nothing on the page saying why.
//
// The action is the canonical retrospective Learn — the same request `marco learn --recent` makes,
// which promotes the walk, keeps the demonstration and records the goal. The page says the name
// and nothing else.
//
// Deleting the offer, or routing it anywhere but /api/learn/recent, must fail this.
func TestThePageOffersTheCanonicalWayToNameWhatYouJustDid(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for what, marker := range map[string]string{
		"a place to say the name": `id="mname"`,
		"and something to press":  `nameRecent()`,
	} {
		if !strings.Contains(here, marker) {
			t.Errorf("the Here view no longer offers %s (%s)", what, marker)
		}
	}
	if !strings.Contains(editPage, `learnPost('recent'`) {
		t.Fatal("naming what you just did does not go through the retrospective Learn " +
			"request. A surface with its own path to a goal would have its own idea of " +
			"what a Learn is.")
	}
	// AND IT IS OFFERED ONLY WHILE LEARNING, because a goal built out of a walk Marco was
	// not allowed to remember would be a promise about evidence that does not exist.
	if !strings.Contains(editPage, "show('mgoal', LEARNING") {
		t.Error("the offer to name what you just did is not tied to learning being on")
	}
}

// ── one thought at a time ─────────────────────────────────────────────────────

// THE HERE VIEW ASKS WHAT MARCO WOULD LIKE TO TRY, AND CANNOT ASK IT TO TRY IT.
//
// A poll on a timer sends this request. What makes that safe is what it leaves nil: no Test, no
// Perform, no Learn — so however often the page asks what Marco is focused on, nothing is
// attempted. Observation permission is not actuation permission, and neither is a page refresh.
//
// Deleting the Experiment field, or letting this build something that acts, must fail this.
func TestTheHereViewAsksWhatMarcoWouldLikeToTry(t *testing.T) {
	q := experimentRequest()
	if q.Experiment == nil {
		t.Fatal("the page cannot ask what Marco is focused on, so the one thing it is doing " +
			"stays invisible")
	}
	if q.Test != nil || q.Perform != nil || q.Learn != nil || q.Ambient != nil {
		t.Errorf("the proposal read carries something that acts: %+v", q)
	}
}

// AND TESTING ASKS FOR THE CONNECTION IT WAS OFFERED, BY IDENTITY.
//
// # Why the page may not describe an experiment
//
// A description is not an identity. A surface asking to test "Mouse" would be asking Marco to
// work out what it meant — and the whole point of the proposal is that Marco already decided,
// said which two screens and which control, and handed back the ids. The page gives them back
// unchanged. Half an edge is refused rather than sent, because an experiment Marco has to guess
// the rest of is a guess about what to press on somebody's computer.
//
// Deleting the completeness check, or letting this build a Perform, must fail this.
func TestTestingAsksForTheConnectionItWasOffered(t *testing.T) {
	q, ok := testRequest("settings", "subj_a", "subj_b")
	if !ok || q.Test == nil {
		t.Fatal("a complete connection was refused")
	}
	if q.Test.From != "subj_a" || q.Test.To != "subj_b" {
		t.Errorf("the request asks for %s → %s", q.Test.From, q.Test.To)
	}
	if q.Perform != nil || q.Learn != nil || q.Ambient != nil || q.Experiment != nil {
		t.Errorf("testing carries another request beside it: %+v", q)
	}
	for _, half := range [][2]string{{"", "subj_b"}, {"subj_a", ""}, {"", ""}} {
		if _, ok := testRequest("settings", half[0], half[1]); ok {
			t.Errorf("half a connection (%q → %q) was sent as an experiment", half[0], half[1])
		}
	}
}

// THE SURFACE THAT CAN START AN ATTEMPT CAN STOP IT.
//
// The moment this page can start something that walks somebody's desktop, it owes them a way to
// end it — and it has to be the SAME cancellation the spoken "stop", the leader key and
// `director stop` reach, not a second one. One mechanism, four ways in.
//
// Deleting the stop door must fail this.
func TestTheSurfaceThatCanStartAnAttemptCanStopIt(t *testing.T) {
	if !strings.Contains(editPage, "stopAll()") {
		t.Fatal("the Here view can start an experiment and offers no way to stop it")
	}
	mux := (&editor{}).serveMux()
	if _, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/api/stop", nil)); pattern == "" || pattern == "/" {
		t.Errorf("/api/stop is not registered on the production mux (pattern %q)", pattern)
	}
}

// ONE EXPERIMENT IS THE DOMINANT THING, AND ITS HYPOTHESIS HAS THREE PARTS.
//
// # The dogfood failure
//
// A person could not tell what Marco was focused on, what it was about to try, why, or what it
// needed. Everything competed for the same space. So the primary surface shows ONE experiment,
// stated as a claim — from HERE, doing THIS, you arrive THERE — with Marco's reason beside it.
//
// "Trying Mouse" is ambiguous between a goal, a target, a Place, an edge and a route, which is
// why all three parts are rendered and why the Go-there box says something different.
//
// Deleting any part of the hypothesis, or the reason, must fail this.
func TestTheExperimentIsStatedAsAHypothesis(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for what, marker := range map[string]string{
		"a place for the one experiment": `id="xbox"`,
		"the connection itself":          `id="xedge"`,
		"why Marco wants to":             `id="xwhy"`,
		"and the story as it happens":    `id="xsteps"`,
	} {
		if !strings.Contains(here, marker) {
			t.Errorf("the Here view does not show %s (%s)", what, marker)
		}
	}
	for _, part := range []string{"XNOW.from_words", "XNOW.action", "XNOW.to_words", "XNOW.why"} {
		if !strings.Contains(editPage, part) {
			t.Errorf("the hypothesis does not render %s, so what Marco is about to do is "+
				"ambiguous between a goal, a place, an edge and a route", part)
		}
	}
}

// AND TESTING WHAT MARCO LEARNED IS NOT THE SAME BUTTON AS GOING SOMEWHERE.
//
// Two different acts. Testing proves a connection Marco believes in, needs a specific SOURCE it
// may have to walk to, and gives the desktop back because nobody asked to be moved. Going
// somewhere accomplishes what a person asked for and leaves them there. A surface labelling both
// "Try it" hides the difference between doing somebody a favour and borrowing their computer.
//
// Deleting the distinction must fail this.
func TestTestingAndGoingThereAreDifferentButtons(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	if !strings.Contains(here, "testEdge()") || !strings.Contains(here, "tryIt()") {
		t.Fatal("the two acts do not have two doors")
	}
	if strings.Contains(here, ">Try it<") {
		t.Error("something is still labelled 'Try it', which says nothing about which of the " +
			"two acts it is")
	}
	if !strings.Contains(here, "Test what I learned") || !strings.Contains(here, "Go there") {
		t.Error("the two buttons do not say which act they are")
	}
}

// AND REPEATED EVIDENCE DOES NOT TAKE THE ONE LINE SOMEBODY READS.
//
// Walking a familiar route is not a discovery. A headline that showed "saw way again" over the
// top of "learned destination" would train a person to stop reading it — which is exactly the
// undifferentiated stream this correction is about. The demoted changes stay in the feed and in
// Activity, where repeated evidence belongs.
//
// Deleting the filter — taking the newest event whatever it is — must fail this.
func TestTheHeadlineIsNewKnowledgeRatherThanTheLatestEvent(t *testing.T) {
	if !strings.Contains(editPage, "e.change!=='strengthened'") {
		t.Fatal("the headline is whatever happened last, so a familiar route hides a new one")
	}
	// AND THE DETAIL IS DEMOTED RATHER THAN DELETED: it is still on the page, behind a
	// disclosure, because "why has it not learned that yet" is a real question with a real
	// answer.
	if !strings.Contains(editPage, `<details id="mbox"`) {
		t.Error("the per-candidate evidence is not demoted behind a disclosure, so it still " +
			"competes with the one thing Marco is focused on")
	}
	if !strings.Contains(editPage, `id="mlist"`) {
		t.Error("the per-candidate evidence was removed rather than demoted")
	}
}

// GO THERE IS OFFERED ONLY FOR SOMETHING MARCO CAN ACTUALLY BE ASKED FOR.
//
// # The dead end this closes, reported live
//
// The phrase box was filled from whatever the newest change described, and a `learned place`
// describes a PLACE. So after Marco learned Home the page offered "Home" beside a button saying
// Go there — and pressing it ran `marco do "Home"`, which matched no play and no goal, fell
// through to the Director's read-it-against-the-screen path, and answered:
//
//	FAILED: I only understand click, focus, move, repeat and text editing so far
//
// Knowing where somewhere is and being able to be ASKED for it are different things. The second
// is a name a person gives, which is what "Name what I just did" exists for — and offering Go
// there without one sends somebody to a dead end and then blames the vocabulary.
//
// Deleting the kind test, or filling the box from the headline again, must fail this.
func TestGoThereIsOnlyOfferedForSomethingYouCanAskFor(t *testing.T) {
	if !strings.Contains(editPage, "e.kind==='goal'") {
		t.Fatal("the phrase box is filled from any change, so a learned PLACE is offered as " +
			"something to ask for — and `marco do \"Home\"` is not a request Marco can answer")
	}
	if strings.Contains(editPage, "box.value=top.description") {
		t.Error("the phrase is still taken from the headline change rather than from a goal")
	}
	if !strings.Contains(editPage, "show('ltrybar', !!askable)") {
		t.Error("Go there is shown whether or not there is anything to ask for")
	}
	// AND THE GAP IS NAMED. Hiding the button and saying nothing would leave somebody
	// wondering what happened to it — the missing step is the interesting part.
	if !strings.Contains(editPage, `id="lgap"`) || !strings.Contains(editPage, "show('lgap', !askable)") {
		t.Error("nothing says why Go there is absent, so the next step is invisible")
	}
}

// HIDDEN MEANS HIDDEN.
//
// # The page contradicting itself
//
// `hidden` is only `display:none` in the BROWSER's stylesheet, so any author rule that sets
// display beats it — and `.bar{display:flex}` sets display. Every `show(id, false)` on a bar was a
// no-op. Reported live: the Go there box and the sentence explaining why Go there was absent
// appeared together, one directly above the other.
//
// It is not a cosmetic bug. `show()` is how this page says a thing does not apply, and a page that
// cannot say that offers actions it has just explained are impossible.
//
// Deleting the rule must fail this.
func TestHiddenActuallyHides(t *testing.T) {
	if !strings.Contains(editPage, "[hidden]{display:none!important}") {
		t.Fatal("the page has no rule making the hidden attribute win. `.bar{display:flex}` " +
			"beats the browser default, so every show(id,false) on a bar does nothing — " +
			"the Go there box and the sentence saying why it is absent render together.")
	}
	// AND THE RULE COMES AFTER THE THING IT HAS TO BEAT, since without !important it would
	// not, and with it a reader should still not have to check.
	if strings.Index(editPage, "[hidden]{display:none") < strings.Index(editPage, ".bar{margin-top") {
		t.Error("the hidden rule is declared before the display rules it exists to override")
	}
}

// THE FEED REFRESHES NAMES RATHER THAN CACHING THEM.
//
// # Why a place kept its worst description forever
//
// The Director resolves a place's name at READ time, deliberately: a Place is established on one
// pass and NAMED on a later one, so an event carrying the words it had at the instant of the write
// would say "about back, settings, 96 things on it" forever about a screen that is now perfectly
// well called Home. That is ADR-111's own reasoning, and this page defeated it — it asked from a
// cursor, received only what was new, and cached the rendered line.
//
// Reported live, and it is the whole of "naming propagation leaves the user confused": the naming
// worked and the page was showing a snapshot of the instant before it.
//
// The cursor is for a FOLLOWER, which must not reprint what it has printed. That is
// `marco observe --follow`, which keeps its own cursor in observefeed.go and is untouched.
//
// Deleting the re-read must fail this.
func TestTheFeedRefreshesNamesRatherThanCachingThem(t *testing.T) {
	if !strings.Contains(editPage, "'/api/learned?after=0'") {
		t.Fatal("the page still reads the feed from a moving cursor, so an event keeps the " +
			"words it had when it was committed — and a Place named a few seconds later " +
			"goes on rendering as its structural description forever")
	}
	if strings.Contains(editPage, "fresh.concat(WSEEN)") {
		t.Error("the page still accumulates events instead of taking what it is handed, so " +
			"the cached descriptions survive the re-read")
	}
	if !strings.Contains(editPage, "WSEEN=all.slice(0,WMAX)") {
		t.Error("the page does not replace its list from the read")
	}
}

// AND THE HEADLINE IS THE NEWEST EVENT, NOT THE OLDEST OF THE LATEST BATCH.
//
// The Director returns the ring in the order it happened, oldest to newest, and this page reads
// index 0 as the headline. Concatenating batches put the OLDEST of the newest batch on top — which
// showed on a burst and hid on a quiet desktop, so it was invisible exactly when it was easiest to
// notice.
//
// Deleting the reverse must fail this.
func TestTheHeadlineIsTheNewestEvent(t *testing.T) {
	if !strings.Contains(editPage, "(v.events||[]).slice().reverse()") {
		t.Fatal("the page reads the feed in the order it happened and then treats the first " +
			"entry as the newest, so on any burst the headline is the oldest of them")
	}
}

// THE HERE VIEW ASKS FOR THE MAP, AND THE MAP CANNOT ACT.
//
// Observe's primary object is Marco's map of the interface, not an event feed. A poll on a timer
// draws it, so what makes that safe is what the request leaves nil.
//
// Deleting the Map field, or letting this build something that acts, must fail this.
func TestTheHereViewAsksForTheMap(t *testing.T) {
	q := mapRequest()
	if q.Map == nil {
		t.Fatal("the page cannot ask for the map, so Observe stays an event feed")
	}
	if q.Test != nil || q.Perform != nil || q.Learn != nil || q.Ambient != nil ||
		q.Experiment != nil {
		t.Errorf("the map read carries another request beside it: %+v", q)
	}
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for what, marker := range map[string]string{
		"somewhere to draw the map": `id="gbox"`,
		"the connections":           `id="ggraph"`,
		"what it can reach":         `id="greach"`,
	} {
		if !strings.Contains(here, marker) {
			t.Errorf("the Here view has no %s (%s)", what, marker)
		}
	}
}

// AND THE PRIMARY COPY SAYS FROM AND TO, NEVER ORIGIN AND ARRIVAL.
//
// Reported live as unclear. They are the machinery's words: a person reading a control centre
// should be told `Home → Mouse`, not handed the vocabulary the walker uses internally. Diagnostics
// keep them.
//
// Reintroducing them into the product copy must fail this.
func TestThePrimaryCopyDoesNotSayOriginOrArrival(t *testing.T) {
	here := editPage[strings.Index(editPage, `<section id="view-here"`):]
	here = here[:strings.Index(here, "</section>")]
	for _, word := range []string{"Origin", "origin:", "Arrival", "arrival:"} {
		if strings.Contains(here, word) {
			t.Errorf("the Here view says %q. That is the walker's vocabulary, and a person "+
				"reading it could not tell what it meant.", word)
		}
	}
	// AND THE MAP DRAWS A CONNECTION AS ONE, with an arrow rather than two labelled fields.
	if !strings.Contains(editPage, "e.from_words") || !strings.Contains(editPage, "e.to_words") {
		t.Error("the map does not render a connection from its two ends")
	}
}
