package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// Answering a question from the browser.
//
// # There is no UI answer path
//
// These handlers shell out to the ORDINARY commands — `director answer` and
// `director name-screen` — which reach the service, which reaches the ProposalLedger. That is
// the same journey a person's typing makes at a terminal, and it is deliberately the only one:
// a button that reached further in than the CLI does would be a second authority path, and the
// second one is always the one nobody audits.
//
// In particular this file cannot name a subject, cannot create a rehearsal grant and cannot
// start anything. It can pass three closed words and one string the user typed, to a proposal
// id it was given. Everything that decides what any of that MEANS is behind the service.
//
// # The session id is bookkeeping
//
// A question is found by its own identity across every session record — the registry says so and
// the reason is that people answer minutes later, after the screen has changed and often after
// the session has ended. The session is looked up here only because the command takes one.

func directorBin() string {
	if b := os.Getenv("MARCO_DIRECTOR_BIN"); b != "" {
		return b
	}
	return "director"
}

// currentSession is the observation session an answer should be addressed to.
//
// A READ, and empty is a perfectly good answer: the registry matches a proposal by identity
// across every record it holds, so an empty id is "whichever session this question came from"
// rather than a failure.
func currentSession() string {
	out, err := exec.Command(directorBin(), "observation-session", "--json").Output()
	if err != nil {
		return ""
	}
	var v struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(out, &v) != nil {
		return ""
	}
	return v.ID
}

// answers is the closed vocabulary, mapped onto the words the command accepts.
//
// A map rather than a normalisation rule, because the vocabulary is CLOSED: anything not on this
// list is refused rather than coerced. The only defaults available would be "yes" and "no", and
// both would be putting words in somebody's mouth.
var answers = map[string]string{
	"confirmed":    "yes",
	"yes":          "yes",
	"contradicted": "no",
	"no":           "no",
	"declined":     "not-now",
	"not now":      "not-now",
	"not-now":      "not-now",
}

// handleAnswer records one of the three closed answers against a proposal.
func handleAnswer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Response string `json:"response"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.ID) == "" {
		http.Error(w, "missing question id", 400)
		return
	}
	word, ok := answers[strings.ToLower(strings.TrimSpace(req.Response))]
	if !ok {
		// Refused rather than guessed. See the note on `answers`.
		http.Error(w, "that is not an answer; say yes, no, or not-now", 400)
		return
	}
	out, err := exec.Command(directorBin(), "answer",
		currentSession(), req.ID, word).CombinedOutput()
	writeJSON(w, map[string]any{"ok": err == nil, "output": string(out)})
}

// handleName answers the ONE question whose answer is the user's own word.
//
// A separate handler from the one above, deliberately, exactly as it is a separate command and a
// separate typed response in the ledger. Folding it in would mean the generic path had to accept
// arbitrary text for any question, which is how a closed vocabulary quietly stops being one.
//
// The raw string travels as a raw string exactly this far. It becomes a ScreenName at the
// request boundary inside the Director, which is where validation and provenance are established
// for every caller — there is no path from here that writes a name anywhere.
func handleName(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil ||
		strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "a name and the question it answers are both needed", 400)
		return
	}
	out, err := exec.Command(directorBin(), "name-screen",
		currentSession(), req.ID, req.Name).CombinedOutput()
	writeJSON(w, map[string]any{"ok": err == nil, "output": string(out)})
}

// handleStop cancels whatever is running, through the canonical stop.
//
// One control, and it is the existing one. A UI-only cancellation flag would be a surface that
// believed it had stopped Marco while Marco carried on.
func handleStop(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command(directorBin(), "cancel-observation").CombinedOutput()
	writeJSON(w, map[string]any{"ok": err == nil, "output": string(out)})
}
