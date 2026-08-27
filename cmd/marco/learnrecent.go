package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `marco learn "open mouse settings" --recent` — learn what Marco has just watched.
//
// # Why this goes to the Director and the ordinary `marco learn` does not
//
// Because the recent past is something only the Director saw. `marco learn` without this flag is
// the recorder: it watches keystrokes and writes a program from them, and it has no idea what was
// on screen a minute ago. Asking IT to learn what just happened would have nothing to read.
//
// So this is a pure client of the one Learn request the Director already answers — the same
// `ObserveLearn`, with `Recent` set. There is no second router here, no second name validation and
// no second view: what comes back is an ordinary finished learn session.
//
// # It does not start a Director
//
// Same rule as `marco observe status`, and for a sharper version of the same reason. A Director
// that has just been started has watched nothing, so the only answer it could give is "I don't
// know what you just did" — and it would have started a background process to say so. If nothing
// is running, nothing was watching, and that is the honest answer on its own.
//
// Deleting the false must fail TestLearningTheRecentPastDoesNotStartADirector.
func runLearnRecent(name string) int {
	client, err := learnRecentDial(false)
	if err != nil {
		fmt.Fprintln(os.Stderr,
			"marco: I haven't been watching, so I don't know what you just did.\n"+
				"  Turn it on with: marco observe")
		return 1
	}
	defer client.Close()

	raw, err := client.Observation(service.ObserveQuery{
		Learn: &service.ObserveLearn{Recent: true, Name: name},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	// Only the two fields a person needs. The view carries a great deal more — phases,
	// grounding, evidence — and printing any of it here would make this command a diagnostic
	// rather than an answer.
	var v struct {
		Saying  string `json:"saying"`
		Learned bool   `json:"learned"`
		Play    string `json:"play,omitempty"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Fprintf(os.Stderr, "marco: the Director's reply was unreadable: %v\n", err)
		return 1
	}
	if v.Saying != "" {
		fmt.Println(v.Saying)
	}
	if !v.Learned {
		// NOT AN ERROR EXIT, and this matters for the shell. "I don't have enough of what
		// you just did" is an answer to the question, not a failure to answer it, and a
		// non-zero status would make every script treat an honest no as a broken Marco.
		return 0
	}
	return 0
}

// learnRecentDial is how this command reaches the Director.
//
// A package VARIABLE for the one reason `observeDial` is: the flag it is handed — whether an
// autostart is permitted — is the whole of the claim above, and a test cannot observe that flag
// through a real dialler without actually spawning a Director.
//
// Production never reassigns it.
var learnRecentDial = directorConnect
