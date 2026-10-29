package main

// BEFORE OFFERING TO LEARN A PHRASE, ASK WHETHER MARCO ALREADY HAS IT.
//
// # The moment this is
//
// The widget's one door is `marco do --source=… <phrase>`, and the engine's intake matches a play
// EXACTLY on purpose: "near enough" is a judgement, and judgement is Director's. So a phrase that
// is a routine's name runs, and everything else goes to Director to be read against the screen —
// and when nothing at all took it, the widget offers to record one.
//
// That offer is the moment. A person who typed "make the pictures nicer" may well have a routine
// for it under another name, and being offered a recording session is the one answer that is
// certainly wrong. So before the offer, Director is asked which saved routine the words named.
//
// # It asks DIRECTOR, through `marco`, and holds no policy
//
// Which routines are eligible depends on what is in front, and which library the answer is about
// is Director's. The widget holds neither fact and does not get a copy of either: it runs
// `marco resolve --json`, which is a client, and renders what comes back. No scope rule lives
// here, no exact matching, no resolver plugin, and no Director connection of its own.
//
// AND THE OFFER IS STILL AN OFFER. A resolution selects; it never runs. What a person confirms
// goes back through the same door with the whole identity — `--play`, `--app`, `--focus` — which
// is the intake's explicit-identity arm, where no phrase is looked up again and every guard is
// the one that was already there.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// resolved is `marco resolve --json`'s answer, as much of it as this surface renders.
//
// A subset on purpose: the widget shows a sentence and offers one routine. Everything else in the
// answer is for a surface with room to draw it.
type resolved struct {
	Answer struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		App     string `json:"app"`
		Slug    string `json:"slug"`
		Focus   bool   `json:"focus"`
		Name    string `json:"name"`
		Scope   string `json:"scope"`
		Reply   string `json:"reply"`
		Context struct {
			Says string `json:"says"`
		} `json:"context"`
		Source   string `json:"source"`
		Resolver string `json:"resolver"`
	} `json:"answer"`
	Mismatch string `json:"mismatch"`
	Why      string `json:"why"`
}

// resolveWithin bounds the ask.
//
// Short, because this sits between a person's request and an offer they are waiting for, and the
// honest answer to a slow resolution is the offer they would have got anyway. The plugin's own
// cold-model bound is longer than this; a first call that pays it will miss, and the second will
// not.
const resolveWithin = 6 * time.Second

// askWhichRoutine is a package variable for the same reason `spawnIntakeChild` is: this surface
// has to be exercisable without spawning `marco.exe`. Production never reassigns it.
var askWhichRoutine = runResolveChild

// runResolveChild runs `marco resolve --json` and reads the one object it prints.
func runResolveChild(phrase string) (resolved, bool) {
	cmd := exec.Command(marcoBin(), "resolve", "--json", phrase)
	out, err := cmd.Output()
	// AN EXIT CODE IS NOT THE ANSWER HERE. `resolve` exits 3 when there is no answer and
	// STILL prints why, which is the thing worth showing; so the output is read either
	// way and only unreadable output is a failure.
	if len(strings.TrimSpace(string(out))) == 0 {
		mlogI("overlay: resolve said nothing", "phrase", phrase, "err", errText(err))
		return resolved{}, false
	}
	var r resolved
	if jsonErr := json.Unmarshal(out, &r); jsonErr != nil {
		mlogW2("overlay: resolve answered something unreadable", "err", jsonErr.Error())
		return resolved{}, false
	}
	return r, true
}

// errText is an error as a log value, and "" for none.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// offerWhatMarcoHas asks Director which routine a phrase named and offers it, or falls back to
// offering to learn.
//
// It answers whether the learn offer was replaced, so the caller keeps exactly one offer on
// screen: two prompts at once is a HUD asking two questions with one answer line.
func offerWhatMarcoHas(h *model, phrase string) bool {
	done := make(chan resolved, 1)
	go func() {
		if r, ok := askWhichRoutine(phrase); ok {
			done <- r
			return
		}
		done <- resolved{}
	}()
	var r resolved
	select {
	case r = <-done:
	case <-time.After(resolveWithin):
		mlogI("overlay: resolve took too long", "phrase", phrase)
		return false
	}

	// NO ANSWER IS NOT AN ANSWER OF NONE. A Director that could not be asked, or that
	// refused, or a library that is not the one this phrase is about: all of them are said
	// in the HUD and NONE of them becomes a silent offer to record something.
	switch {
	case r.Why != "":
		h.log(r.Why)
		return false
	case r.Mismatch != "":
		h.log(r.Mismatch)
		return false
	}
	a := r.Answer
	if a.Kind != "run" || a.ID == "" {
		// IT RESOLVED TO NO ROUTINE, which is the ordinary case for something Marco
		// has never been shown. The sentence is still worth having — it says what was
		// eligible and who answered — and then the offer to learn stands.
		if a.Reply != "" {
			h.log(a.Reply)
		}
		return false
	}
	if a.Context.Says != "" {
		h.log(a.Context.Says)
	}
	h.log(a.Resolver)
	offerRoutine(h, a.Name, a.ID, a.App, a.Slug, a.Focus)
	return true
}
