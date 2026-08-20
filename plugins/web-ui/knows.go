package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

// What Marco Knows, in the browser.
//
// # Still no UI authority path
//
// Exactly as with answering: these handlers shell out to the ORDINARY `director knows` command,
// which reaches the service, which reaches the canonical durable verbs. A button that wrote to
// memory directly would be a second door onto a person's own judgements, and the second one is
// always the one nobody audits.
//
// # Why this surface has to exist at all
//
// Every other correction path reaches an answer through the question that produced it, and a
// question only exists while some session can still recognise its subject. When that stops being
// true the answer becomes uncorrectable — permanently, silently. This is the door that does not
// depend on Marco still being able to find the thing.

// handleKnows lists what a person has told Marco.
func handleKnows(w http.ResponseWriter, _ *http.Request) {
	out, err := exec.Command(directorBin(), "knows", "--json").Output()
	if err != nil {
		writeJSON(w, map[string]any{"known": []any{}, "reach": "absent"})
		return
	}
	var v struct {
		Known []map[string]any `json:"known"`
	}
	if json.Unmarshal(out, &v) != nil {
		writeJSON(w, map[string]any{"known": []any{}, "reach": "absent"})
		return
	}
	// No-store, for the same reason the account is: a stale list of your own judgements is a
	// person correcting something they already corrected.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"known": v.Known, "reach": "present"})
}

// corrections is the closed vocabulary this surface may send.
//
// `withdraw` sits alongside the three answers rather than inside them: taking an answer back is
// not a fourth thing you can say about the world, it is a statement about the answer.
var corrections = map[string]string{
	"yes": "yes", "confirmed": "yes",
	"no": "no", "contradicted": "no",
	"not sure": "not-sure", "not-sure": "not-sure", "declined": "not-sure",
	"withdraw": "withdraw",
}

// handleCorrect changes or withdraws one durable judgement.
//
// The subject and kind travel as opaque strings: this file cannot pick a subject, cannot invent
// one and cannot decide what a judgement means. It passes back the identifiers the list gave it,
// with one word from a closed set.
func handleCorrect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subject string `json:"subject"`
		Kind    string `json:"kind"`
		Change  string `json:"change"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil ||
		strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Kind) == "" {
		http.Error(w, "which judgement?", 400)
		return
	}
	word, ok := corrections[strings.ToLower(strings.TrimSpace(req.Change))]
	if !ok {
		// Refused rather than guessed, as everywhere else on this boundary.
		http.Error(w, "that is not a correction; say yes, no, not-sure or withdraw", 400)
		return
	}
	out, err := exec.Command(directorBin(), "knows", req.Subject, req.Kind, word).CombinedOutput()
	writeJSON(w, map[string]any{"ok": err == nil, "output": string(out)})
}

// handleShowMe points at what one remembered judgement refers to.
//
// # Why this is allowed to exist next to the correction buttons
//
// Because the two answer the same person's question in the same breath. "Change this" is
// unanswerable while "this" is a subject id nobody can see, and a page that offered a correction
// without a way to look at what is being corrected is asking somebody to edit blind.
//
// # What it cannot do
//
// It carries an identity, never geometry. The subject string came from the list this page was
// given; the Director resolves it to somewhere on the screen through what a session currently
// RECOGNISES, and refuses when nothing does. There is no field on this request through which a
// surface could supply a rectangle, so a page cannot make Marco appear to mean anything.
//
// Stored geometry is never consulted — see observe.WhatRemembersThis. When Marco remembers the
// judgement and cannot find it, the honest sentence comes back and nothing is drawn.
func handleShowMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subject string `json:"subject"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Subject) == "" {
		writeJSON(w, map[string]any{"said": "I need to know which one you mean."})
		return
	}
	// --watch 0: pointing must not start a new observation. Looking at what Marco already
	// believes is a read, and a click that quietly began three minutes of watching would be a
	// side effect nobody asked for.
	out, err := exec.Command(directorBin(), "show-me",
		"--subject", strings.TrimSpace(req.Subject), "--watch", "0", "--json").Output()
	if err != nil {
		writeJSON(w, map[string]any{"said": "I couldn't work out where that is right now."})
		return
	}
	var v struct {
		Say   string           `json:"say"`
		Boxes []map[string]any `json:"boxes"`
	}
	if json.Unmarshal(out, &v) != nil {
		writeJSON(w, map[string]any{"said": "I couldn't work out where that is right now."})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"said": v.Say, "shown": len(v.Boxes)})
}
