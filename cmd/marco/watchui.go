package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
)

// TELL ME WHEN YOU LEARN SOMETHING, WHERE I AM ALREADY LOOKING.
//
// # Why this is here and not a second window
//
// The first dogfood ran out of two PowerShell consoles: one holding the Director, one printing
// `marco observe --follow`, and a person alt-tabbing between them to find out whether Marco had
// learned anything. That is a test of the plumbing. Somebody who has to watch a log to discover
// what their assistant knows has not been given an assistant.
//
// So the loop lands on the surface that already answers "where am I and what does Marco make of
// it" — the control centre's Here panel. It already had the current place, the Audience's name for
// it and the places Marco knows. What it could not say was whether Marco was allowed to REMEMBER
// any of it, what it MADE of what somebody just did, what it had committed, and what to type to
// use that. Those are this file.
//
// # What it is not allowed to be
//
// A window onto the lifecycle, never a second copy of it — the rule learnui.go states and the same
// one here. Specifically:
//
//   - It never writes to semantic memory. Three of these four endpoints are reads, and the fourth
//     toggles ATTENTION and PERMISSION — whether Marco may look and whether what it sees may
//     become durable. Neither of those is a fact about a Place.
//   - It never announces learning. The events are the store's own, after its own write committed
//     (see cmd/director/learningfeed.go); this renders what it is handed and cannot manufacture an
//     event the store did not commit.
//   - It never names a Place. `Description` arrives already worded, by the only process holding
//     the store, and is printed.
//   - Try it starts nothing itself. It spawns `marco do`, exactly as a clicked Run has since
//     Phase 0, so authority, planning, compilation, the actuation lease and verification all
//     happen where they already happen. There is no UI executor.

// watchAPI wires the Here panel's additions onto a mux.
func watchAPI(mux *http.ServeMux, e *editor) {
	mux.HandleFunc("/api/watching", handleWatching)
	mux.HandleFunc("/api/learned", handleLearned)
	mux.HandleFunc("/api/made", handleMade)
	mux.HandleFunc("/api/map", handleMap)
	mux.HandleFunc("/api/experiment", handleExperiment)
	mux.HandleFunc("/api/test", handleTest)
	mux.HandleFunc("/api/stop", handleStop)
	mux.HandleFunc("/api/try", e.handleTry)
}

// handleWatching reads or changes what ambient observation is doing.
//
// GET is a read and starts nothing. POST carries one verb, and only the one that turns attention
// ON may start the Director — the same distinction handleWake draws, for the same reason: a read
// that silently paid for a twenty-second service start would charge it to somebody who opened a
// tab, and a press is a person saying "start it".
func handleWatching(w http.ResponseWriter, r *http.Request) {
	verb := ""
	if r.Method == http.MethodPost {
		var body struct {
			Verb string `json:"verb"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		verb = body.Verb
	}
	q, press := watchingRequest(verb)
	if press {
		if c, err := wakeDirector(); err == nil {
			c.Close()
		}
	}
	writeWatching(w, q)
}

// watchingRequest is the whole vocabulary of this strip, and the reason it is a function.
//
// FOUR VERBS, AND EVERY ONE OF THEM IS ABOUT ATTENTION OR PERMISSION. Nothing here can name a
// Place, establish one, strengthen one or bind a word to one — this surface asks Marco to look and
// asks whether what it sees may be remembered, and the answers to both are the Director's.
//
// An unrecognised verb is a READ. A typo must not be routed to the nearest verb that happens to
// match, because two of these change what Marco is doing.
//
// It reports whether the press may START the Director. Only turning something ON does: `stop` and
// `unlearn` on a machine with no Director are already true.
//
// Deleting the four-verb restriction — letting this build any ambient request — must fail
// TestTheDogfoodStripOnlyAsksAboutAttentionAndPermission.
func watchingRequest(verb string) (q service.ObserveAmbient, press bool) {
	switch strings.TrimSpace(verb) {
	case "watch":
		return service.ObserveAmbient{Enable: true}, true
	case "stop":
		return service.ObserveAmbient{Disable: true}, false
	case "learn":
		// Learn may turn watching on with it — see ObserveAmbient. Asking Marco to learn
		// from what it sees is meaningless while it is not looking, and refusing would be
		// pedantry about a state the person plainly did not want.
		return service.ObserveAmbient{Learn: true}, true
	case "unlearn":
		return service.ObserveAmbient{Unlearn: true}, false
	}
	return service.ObserveAmbient{}, false
}

// writeWatching sends one ambient request and renders the answer.
//
// "The Director is not running" is a STATE this panel draws, not a failure of the page — the same
// call learnui.go makes and the same reason. The strip says Marco is not watching, because it is
// not.
func writeWatching(w http.ResponseWriter, q service.ObserveAmbient) {
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false, "watching": false, "learning": false})
		return
	}
	defer c.Close()
	raw, err := c.Observation(service.ObserveQuery{Ambient: &q})
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true, "watching": false, "learning": false,
			"saying": err.Error()})
		return
	}
	var view map[string]any
	if json.Unmarshal(raw, &view) != nil {
		writeLearnJSON(w, map[string]any{"available": true, "watching": false, "learning": false})
		return
	}
	view["available"] = true
	writeLearnJSON(w, view)
}

// handleLearned is the feed: what durable knowledge has changed since a cursor.
//
// THE NARROWEST READ ON THIS SURFACE. It sends nothing but a sequence number, it cannot start an
// observation, and it cannot create the knowledge it reports. A cursor older than the Director's
// ring comes back with `missed` set rather than quietly starting late, and the page says so —
// somebody who looked away must not be shown silence and conclude nothing happened.
//
// Deleting the cursor — always asking from zero — must fail TestTheFeedAsksFromWhereItGotTo.
func handleLearned(w http.ResponseWriter, r *http.Request) {
	q := learnedRequest(r.URL.Query().Get("after"))
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false})
		return
	}
	defer c.Close()
	raw, err := c.Observation(q)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	var view map[string]any
	if json.Unmarshal(raw, &view) != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	view["available"] = true
	writeLearnJSON(w, view)
}

// handleTry runs a sentence, through the door a typed `marco do` uses.
//
// # Why it spawns rather than calls
//
// Because `marco do` is where intake, planning, compilation to legal Marco, the authority check,
// the actuation lease and verification live, and a control centre that reached past any of them
// would be the second intake this campaign exists to remove. It is the identical mechanism a
// clicked Run already uses — the only difference is that a Run carries a play identity and this
// carries the person's words, which is exactly the difference between the two things they did.
//
// The answer is a run id, not a verdict: the play may take a minute, and this is an HTTP handler.
// `/api/run` says what became of it.
//
// Deleting the spawn — acting from this process — must fail TestTryItRunsThePhraseThroughMarcoDo.
func (e *editor) handleTry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	text := strings.TrimSpace(body.Text)
	if text == "" {
		http.Error(w, "say what to try", http.StatusBadRequest)
		return
	}
	id, err := e.runs.startPhrase(text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "run": id, "name": text})
}

// phraseArgv is what Try it launches.
//
// The source is the control centre because that is where the press happened. It is the same
// argument vector `doArgv` builds for a clicked Run, with the person's words in place of a play
// identity — and the words go LAST, unflagged, exactly as they arrive from a terminal.
func phraseArgv(text string) []string {
	return []string{"do", "--source=" + string(invoke.SourceControlCentre), text}
}

// learnedRequest is the feed's whole question: a sequence number, and nothing else.
//
// A WHOLE ObserveQuery rather than the inner value, because what makes this read safe is not what
// it sets but what it LEAVES NIL. No Learn, no Ambient, no Sight — this cannot start an
// observation, cannot begin a demonstration and cannot write to memory, and a reader can see that
// from the one place the request is built.
//
// An unparseable cursor reads as zero, which asks for everything the Director still holds. That is
// the harmless direction: a page that showed the ring twice is untidy, a page that silently
// skipped to the end would let somebody conclude Marco learned nothing while they were away.
//
// Deleting the cursor — hard-coding zero — must fail TestTheFeedAsksFromWhereItGotTo.
func learnedRequest(after string) service.ObserveQuery {
	n, _ := strconv.ParseUint(strings.TrimSpace(after), 10, 64)
	return service.ObserveQuery{Learning: &service.ObserveLearning{After: n}}
}

// handleMade is what Marco made of what you just did — learned, seen again, or refused and why.
//
// # The silence this ends
//
// The second dogfood: a person turned Watch & Learn on, walked Home → Bluetooth & devices → Mouse
// twice with real clicks, and the page said nothing. Not "learned", not "couldn't" — nothing. The
// Director had the answer the whole time and said it only to `marco observe --evidence`, in a
// terminal:
//
//	x activate in applicationframehost
//	    traversed 4 times
//	    I couldn't read what the control was called, so I can't say what to press
//
// A mode indicator with no observable result is a product that says it is working and cannot show
// it. So the same read the command line makes lands here, unchanged, and the page renders the
// Director's own sentences.
//
// # It is a READ and a wider one, deliberately
//
// `Evidence` is checked before every lifecycle verb in the registry, so asking what Marco has
// recorded about you can never be answered by recording more. It is wider than the plain status:
// it NAMES the control somebody pressed, because "I couldn't read what that was called" is
// meaningless without saying which. That widening is the Director's, already reviewed, and this
// carries it no further — the page prints what it is handed.
//
// Its own endpoint rather than a field on `/api/watching`, so the poll that draws the strip stays
// the narrow read it was: this one is asked only while Marco is watching, and only from Here.
//
// Deleting the Evidence field — asking for plain status — must fail
// TestTheHereViewAsksWhatMarcoMadeOfIt.
func handleMade(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false})
		return
	}
	defer c.Close()
	raw, err := c.Observation(madeRequest())
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	var seen []map[string]any
	if json.Unmarshal(raw, &seen) != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	writeLearnJSON(w, map[string]any{"available": true, "seen": seen})
}

// madeRequest is the whole question: what has ambient watching noticed, and what does the policy
// make of each of them.
//
// A WHOLE ObserveQuery for the reason learnedRequest is one: what makes this read safe is what it
// LEAVES NIL. No Learn, no Sight, and inside the ambient request no Enable, Disable, Learn or
// Unlearn — so a poll on a timer cannot start watching, cannot grant learning and cannot promote
// the very candidate it is asking about.
//
// Deleting the Evidence field must fail TestTheHereViewAsksWhatMarcoMadeOfIt.
func madeRequest() service.ObserveQuery {
	return service.ObserveQuery{Ambient: &service.ObserveAmbient{Evidence: true}}
}

// ── one thought at a time ─────────────────────────────────────────────────────

// handleExperiment is what Marco is focused on: the ONE thing it would like to try.
//
// # The dogfood failure this answers
//
// Marco was observing well and a person could not tell what it was focused on, what it was about
// to try, why, or what it needed. Every observation and discovery competed for the same space, so
// there was no thought to follow.
//
// A READ, and the narrowest one that can carry a decision: it starts nothing, chooses nothing
// durable and cannot act. The proposal is Marco's; the attempt is the person's.
//
// Deleting the Experiment field — asking for something that could act — must fail
// TestTheHereViewAsksWhatMarcoWouldLikeToTry.
func handleExperiment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false})
		return
	}
	defer c.Close()
	raw, err := c.Observation(experimentRequest())
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	var view map[string]any
	if json.Unmarshal(raw, &view) != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	view["available"] = true
	writeLearnJSON(w, view)
}

// experimentRequest asks what Marco would like to try, and can ask nothing else.
//
// A WHOLE ObserveQuery, for the reason learnedRequest is one: what makes it safe is what it
// leaves nil. No Test, no Perform, no Learn — a poll on a timer cannot start an attempt however
// often it fires, and this is the request a timer sends.
func experimentRequest() service.ObserveQuery {
	return service.ObserveQuery{Experiment: &service.ObserveExperiment{}}
}

// handleTest asks Marco to try one connection it learned by watching.
//
// # This is not the page executing anything
//
// It sends ONE request and renders the answer. The Director takes the mutating slot, brings the
// application forward, plans over its own graph, takes the authority and the actuation lease per
// step, verifies where it landed and puts the desktop back — all of it where it already happens.
// The press is what makes the attempt legitimate; the page contributes a from and a to it was
// handed by the proposal and did not choose.
//
// # Why it addresses the connection by id
//
// Because a description is not an identity. A surface that asked to test "Mouse" would be asking
// Marco to work out what it meant, and the whole point of the proposal is that Marco already
// decided and said so.
//
// Deleting the edge — letting this build a request of its own — must fail
// TestTestingAsksForTheConnectionItWasOffered.
func handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Application string `json:"application"`
		From        string `json:"from"`
		To          string `json:"to"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	q, ok := testRequest(body.Application, body.From, body.To)
	if !ok {
		http.Error(w, "say which connection to try", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false})
		return
	}
	defer c.Close()
	raw, err := c.Observation(q)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true, "say": err.Error()})
		return
	}
	var view map[string]any
	if json.Unmarshal(raw, &view) != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	view["available"] = true
	writeLearnJSON(w, view)
}

// testRequest is the whole of what a press may ask for: one connection, by identity.
//
// It refuses an incomplete one rather than sending it. An experiment with half an edge is not a
// smaller experiment — it is a request Marco would have to guess the rest of, and guessing what
// to press on somebody's computer is the thing this surface may never do.
func testRequest(application, from, to string) (service.ObserveQuery, bool) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return service.ObserveQuery{}, false
	}
	return service.ObserveQuery{Test: &service.TestQuery{
		Application: strings.TrimSpace(application), From: from, To: to,
	}}, true
}

// handleStop ends whatever Marco is doing, through the one door that already does it.
//
// # Why a Stop has to be here at all
//
// The moment this surface can start an attempt that walks somebody's desktop, it owes them a way
// to end it — and it must be the SAME cancellation everything else uses, not a second one. The
// Director's registry holds one mutating command at a time and `CANCEL_ACTIVE` reaches it: the
// spoken "stop", the leader key, `director stop` and this button are one mechanism.
//
// A stop with nothing running is honest and harmless, which is why it needs no state of its own.
//
// Deleting this — leaving the page able to start an attempt it cannot end — must fail
// TestTheSurfaceThatCanStartAnAttemptCanStopIt.
func handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false})
		return
	}
	defer c.Close()
	res, err := c.Cancel()
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true, "say": err.Error()})
		return
	}
	writeLearnJSON(w, map[string]any{"available": true, "stopped": res.Accepted, "say": res.Message})
}

// handleMap is Marco's map of the interface around where somebody is.
//
// # The primary object of Observe
//
// Not an event feed. A person watching the control centre could not tell what Marco thought the
// screen was called, what it had just discovered, how Places related to each other, or what any of
// it would let Marco do — because the surface showed process vocabulary and not the mental model
// underneath. The map is that model.
//
// A READ, and the widest one on this surface. It carries place NAMES, which is the point: a map
// nobody can read is a diagram. Every word is `observe.PlaceWords`, the one naming function, so
// the map cannot call a screen something the rest of the product does not.
//
// Deleting the Map field must fail TestTheHereViewAsksForTheMap.
func handleMap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": false})
		return
	}
	defer c.Close()
	raw, err := c.Observation(mapRequest())
	if err != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	var view map[string]any
	if json.Unmarshal(raw, &view) != nil {
		writeLearnJSON(w, map[string]any{"available": true})
		return
	}
	view["available"] = true
	writeLearnJSON(w, view)
}

// mapRequest asks for the map, and can ask for nothing else.
//
// A WHOLE ObserveQuery for the reason every other read here is one: what makes it safe is what it
// leaves nil. No Test, no Perform, no Learn, no Ambient — a poll on a timer draws a picture and
// changes nothing about what Marco does, however often it fires.
func mapRequest() service.ObserveQuery {
	return service.ObserveQuery{Map: &service.ObserveMap{}}
}
