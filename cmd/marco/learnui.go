package main

import (
	"encoding/json"
	"net/http"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
)

// The Learn panel's server side: four verbs and one read.
//
// # Why this is thin on purpose
//
// It holds no state. It does not know what a phase is, when a demonstration is finished, whether
// a candidate is ready or whether a rehearsal may run. Every one of those questions is answered
// by the Director's teaching coordinator, and this forwards the person's intent to it and hands
// back what it says.
//
// That is the whole architectural rule for this surface: the UI is a window onto the lifecycle,
// never a second copy of it. A panel that decided for itself when learning had finished would be
// a second implementation of the subsystem — and it would be the one the person believed.
//
// # Why the control centre and not the overlay
//
// It is where the person already goes to see and edit what Marco knows, it is already served by
// this binary, and it costs no new dependency: the engine is stdlib-only and this is net/http and
// encoding/json. The overlay stays what it is — a HUD, not a control panel.

// learnAPI wires the Learn panel's endpoints onto a mux.
//
// One read and four verbs, each named for what the person is doing rather than for what happens
// underneath. Nothing here takes a session id, a proposal id, a window or a flag.
func learnAPI(mux *http.ServeMux) {
	mux.HandleFunc("/api/learn", handleLearnState)
	mux.HandleFunc("/api/learn/start", learnVerb(func(name string) service.ObserveLearn {
		return service.ObserveLearn{Start: true, Name: name}
	}))
	mux.HandleFunc("/api/learn/stop", learnVerb(func(string) service.ObserveLearn {
		return service.ObserveLearn{Stop: true}
	}))
	mux.HandleFunc("/api/learn/try", learnVerb(func(string) service.ObserveLearn {
		return service.ObserveLearn{Try: true}
	}))
	mux.HandleFunc("/api/learn/cancel", learnVerb(func(string) service.ObserveLearn {
		return service.ObserveLearn{Cancel: true}
	}))
	// Naming is the one question answered with the person's own words rather than one of
	// three closed ones. It routes to the same request the command line makes.
	mux.HandleFunc("/api/learn/name", learnVerb(func(called string) service.ObserveLearn {
		return service.ObserveLearn{Called: called}
	}))
	mux.HandleFunc("/api/learn/skip", learnVerb(func(string) service.ObserveLearn {
		return service.ObserveLearn{Skip: true}
	}))
	// AUTHORSHIP, with no question pending. A person correcting what a place is called, or
	// taking the name back — the operation whose absence forced somebody to edit the store
	// by hand. See [[ADR-069-a-name-is-authored-and-can-be-taken-back]].
	mux.HandleFunc("/api/learn/rename", handleRename)
	// ANSWERING one of Marco.s own questions -- the ones it raises itself, counts at
	// the person, and used to offer no way to settle.
	mux.HandleFunc("/api/learn/answer", handleAnswer)
	// LIGHT MODE: watch where you are without teaching anything. Place recognition only
	// happens while something is observing, and starting a demonstration to see it made the
	// instrument require the experiment.
	mux.HandleFunc("/api/learn/remember", learnVerb(func(called string) service.ObserveLearn {
		return service.ObserveLearn{Remember: true, Called: called}
	}))
	mux.HandleFunc("/api/learn/watch", learnVerb(func(string) service.ObserveLearn {
		return service.ObserveLearn{Watch: true}
	}))
	mux.HandleFunc("/api/learn/unwatch", learnVerb(func(string) service.ObserveLearn {
		return service.ObserveLearn{Unwatch: true}
	}))
}

// handleLearnState is the read the panel polls.
//
// A READ, structurally: it sends a query with no verb set, which the Director answers from state
// it already holds. Polling it starts nothing, samples nothing and decides nothing, so the panel
// may refresh at a human-useful rate without changing what Marco does.
func handleLearnState(w http.ResponseWriter, _ *http.Request) {
	writeLearn(w, service.ObserveLearn{})
}

// learnVerb builds a handler for one thing a person can press.
func learnVerb(build func(name string) service.ObserveLearn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeLearn(w, build(body.Name))
	}
}

// writeLearn sends one request and returns whatever the Director says about the session now.
//
// The RESULT IS ALWAYS THE STATE, including when the request failed. A surface that showed an
// error and left its old state on screen would be showing two answers at once, and the person
// would have no way to tell which one Marco believes.
func writeLearn(w http.ResponseWriter, q service.ObserveLearn) {
	w.Header().Set("Content-Type", "application/json")
	c, err := directorConnect(false)
	if err != nil {
		// Not an HTTP error: "the Director is not running" is a state the panel renders,
		// not a failure of the page.
		writeLearnJSON(w, map[string]any{
			"stage":     "unavailable",
			"saying":    "Marco's Director is not running.",
			"detail":    []string{err.Error()},
			"available": false,
		})
		return
	}
	defer c.Close()

	raw, err := c.Observation(service.ObserveQuery{Learn: &q})
	if err != nil {
		writeLearnJSON(w, map[string]any{
			"stage":     "refused",
			"saying":    err.Error(),
			"available": true,
		})
		return
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		writeLearnJSON(w, map[string]any{"stage": "refused", "saying": "unreadable reply"})
		return
	}
	view["available"] = true
	addLifecycle(view)
	writeLearnJSON(w, view)
}

// addLifecycle says where the play Learn just wrote down actually stands, in the words the Plays
// surface uses for the same play.
//
// # Why the panel is not allowed its own sentence
//
// Because it had one, and it was false. "Saved. It is in the Routes tab." was a claim about
// DISCOVERY made from a fact about STORAGE: a saved play lands in `<app>/learned/`, and route
// discovery scans four directories that structurally cannot contain it. Phase 0 split the claim in
// two so the sentence stopped lying. This closes the gap it came out of — the panel now renders
// `plays.Life`, the same value a row in the Plays list carries, so the two surfaces cannot drift
// apart again by somebody editing one of them.
//
// `learned` is the Director's name for saved AND registered (see cmd/director/learnview.go), which
// is the same fact `plays.Registered` names: whether the resolver can reach the file.
//
// Deleting this must fail TestLearnReportsTheSameStandingThePlaysListWould.
func addLifecycle(view map[string]any) {
	life, ok := plays.AfterLearn(truthy(view["saved"]), truthy(view["learned"]))
	if !ok {
		return
	}
	view["life"] = string(life)
	view["life_word"] = life.Word()
	view["life_says"] = life.Says()
}

// truthy reads one of the Director's optional booleans out of a decoded view.
//
// `omitempty` means a false flag is ABSENT, not present-and-false, so a missing key and a false one
// have to mean the same thing here.
func truthy(v any) bool { b, _ := v.(bool); return b }

// writeLearnJSON is this panel's encoder.
//
// Named apart from the editor's so the two surfaces cannot quietly start sharing a response
// shape: they answer different questions and one of them is about a running session.
func writeLearnJSON(w http.ResponseWriter, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// handleRename gives a place the Audience's word for it, or takes that word back.
//
// An empty name is a RETRACTION, not a mistake. Taking a name back is one of the three things
// authorship has to allow and the one that was missing; refusing an empty name here would put the
// person back where they were, stuck with whatever they first typed.
func handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Place  string `json:"place"`
		Called string `json:"called"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeLearn(w, renameRequest(body.Place, body.Called))
}

// renameRequest is what a rename press asks the Director for.
//
// Split out so it can be read without a Director on the other end of a socket. The two fields are
// the whole contract: WHICH place, and what it is called now — never a session, never a proposal,
// never what happens to be on screen.
//
// Deleting Place here must fail TestTheRenameEndpointCarriesWhichPlace.
func renameRequest(place, called string) service.ObserveLearn {
	return service.ObserveLearn{Rename: true, Place: place, Called: called}
}

// handleAnswer settles one of Marco's own open questions.
//
// # Why this endpoint exists
//
// Because Marco raises questions during a teach pass, blocks the rehearsal question behind them,
// and reports "Questions open: 3" at somebody who had no control that could answer any of them.
// Live, three runs in a row ended at a rehearsal that could not be offered because of questions
// the person could see and could not touch.
//
// It carries WHICH question and in WHICH session. Answering "the current one" would settle
// whichever happened to be first when the button was pressed.
//
// Deleting the ID or Session field must fail TestTheAnswerEndpointSaysWhichQuestion.
func handleAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID       string `json:"id"`
		Session  string `json:"session"`
		Response string `json:"response"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeLearn(w, answerRequest(body.ID, body.Session, body.Response))
}

// answerRequest is what answering one question asks the Director for.
func answerRequest(id, session, response string) service.ObserveLearn {
	return service.ObserveLearn{Question: id, Session: session, Answer: response}
}
