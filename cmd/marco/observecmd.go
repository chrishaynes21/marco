package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `marco observe` — Marco pays attention.
//
// # What the person is turning on
//
// Marco watches the desktop continuously and keeps up with where they are: which application,
// which screen, what just changed. It is what makes "where am I" instant instead of a six-second
// look, and what a future "learn what I just did" will read.
//
// # And what they are NOT turning on
//
// Not a recording: nothing is written to disk, no screenshots, no text, and what is held in
// memory is counts against ids that mean nothing outside Marco. Not a permission: watching gives
// Marco no authority to act and no claim on the keyboard. Not a Learn: no questions, nothing
// remembered permanently, no naming.
//
// # Why it is a command and not a setting
//
// Because always-on watching that somebody did not switch on is the shape of surveillance
// whatever the intent. It is off until asked for, it says so when asked, and it stops when told.
// It also does not survive a restart, deliberately — see ADR-093.
func runObserve(args []string) int {
	sub := ""
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, rest = strings.ToLower(args[0]), args[1:]
	}
	fs := flag.NewFlagSet("observe", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(rest)

	var q service.ObserveAmbient
	switch sub {
	case "", "on", "start":
		q.Enable = true
	case "off", "stop":
		q.Disable = true
	case "status":
		// A read. Deliberately does NOT start watching: somebody asking whether Marco is
		// paying attention must not thereby make it start.
	default:
		fmt.Fprintf(os.Stderr, "marco: I don't know `observe %s`. Try: observe, "+
			"observe status, observe stop\n", sub)
		return 2
	}

	// AUTOSTART ONLY FOR THE VERB THAT MEANS IT, through the one dialler this binary
	// already has. `marco observe` is a request for something to happen, so it may start the
	// Director that will do it. Status and stop are not: a question about whether Marco is
	// watching must never be answered by making it watch, and asking it to stop must never
	// bring something into existence to stop.
	//
	// Deleting the distinction must fail TestAskingWhetherMarcoIsWatchingStartsNothing.
	client, err := observeDial(q.Enable)
	if err != nil {
		if q.Enable {
			fmt.Fprintf(os.Stderr, "marco: %v\n", err)
			return 1
		}
		// Nothing is running, so nothing is watching. That is the honest answer to both
		// remaining verbs and it is not a failure.
		if *jsonOut {
			fmt.Println(`{"watching":false}`)
		} else {
			fmt.Println("Marco is not watching.")
		}
		return 0
	}
	defer client.Close()

	raw, err := client.Observation(service.ObserveQuery{Ambient: &q})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	var view service.AmbientView
	if err := json.Unmarshal(raw, &view); err != nil {
		fmt.Fprintf(os.Stderr, "marco: the Director's reply was unreadable: %v\n", err)
		return 1
	}
	if *jsonOut {
		out, _ := json.MarshalIndent(view, "", "  ")
		fmt.Println(string(out))
		return 0
	}
	printObserving(view)
	return 0
}

// printObserving is what a person reads.
//
// The first line is the whole product answer and the rest is optional detail. No subject ids, no
// sample counts in the ordinary case: somebody who typed `marco observe` wants to know that Marco
// is paying attention, not how many times it read the screen.
func printObserving(v service.AmbientView) {
	if !v.Watching {
		fmt.Println("Marco is not watching.")
		return
	}
	fmt.Println("Marco is watching.")
	switch {
	case v.PerceptionDegraded && v.Application != "":
		fmt.Printf("  it can see %s and can't read the page right now\n", v.Application)
	case v.Place != "":
		fmt.Printf("  on a screen it knows, in %s\n", v.Application)
	case v.Application != "":
		fmt.Printf("  in %s, on a screen it hasn't learned yet\n", v.Application)
	default:
		fmt.Println("  it hasn't settled on a screen yet")
	}
	if v.Places > 0 || v.Transitions > 0 {
		fmt.Printf("  noticed %s and %s so far\n",
			plural(v.Places, "screen", "screens"),
			plural(v.Transitions, "move", "moves"))
	}
	fmt.Println("  stop it with: marco observe stop")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// observeDial is how this command reaches the Director.
//
// A package VARIABLE for the one reason the others in this tree are: the flag it is handed —
// whether an autostart is permitted — is the whole of the claim that asking a question does not
// answer itself, and a test cannot observe that flag through a real dialler without actually
// spawning a Director.
//
// Production never reassigns it.
var observeDial = directorConnect
