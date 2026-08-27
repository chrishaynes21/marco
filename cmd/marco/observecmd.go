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
	evidence := fs.Bool("evidence", false,
		"with status: what Marco has seen repeatedly, and what it is waiting for")
	_ = fs.Parse(rest)

	q, ok := observeRequest(sub, rest)
	q.Evidence = q.Evidence || (*evidence && sub == "status")
	if !ok {
		fmt.Fprintf(os.Stderr, "marco: I don't know `observe %s`. Try: observe, "+
			"observe status, observe learn, observe learn off, observe stop\n", sub)
		return 2
	}

	// AUTOSTART ONLY FOR THE VERB THAT MEANS IT, through the one dialler this binary
	// already has. `marco observe` is a request for something to happen, so it may start the
	// Director that will do it. Status and stop are not: a question about whether Marco is
	// watching must never be answered by making it watch, and asking it to stop must never
	// bring something into existence to stop.
	//
	// `observe learn` may start one for the same reason `observe` may: it is a request for
	// something to happen, and it turns watching on with it. `observe learn off` may not —
	// asking Marco to stop remembering must not bring into existence the thing that would.
	//
	// Deleting the distinction must fail TestAskingWhetherMarcoIsWatchingStartsNothing.
	client, err := observeDial(q.Enable || q.Learn)
	if err != nil {
		if q.Enable || q.Learn {
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
	// THE EVIDENCE READ COMES BACK AS A LIST, not a status. Decoded on its own branch rather
	// than by probing the JSON, because two shapes down one wire is the sort of thing that
	// works until somebody adds a field.
	if q.Evidence {
		var seen []service.WatchedView
		if err := json.Unmarshal(raw, &seen); err != nil {
			fmt.Fprintf(os.Stderr,
				"marco: the Director's reply was unreadable: %v\n", err)
			return 1
		}
		if *jsonOut {
			out, _ := json.MarshalIndent(seen, "", "  ")
			fmt.Println(string(out))
			return 0
		}
		printWatched(seen)
		return 0
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

// printWatched is what Marco has seen repeatedly, and what it is waiting for on each.
//
// # The report that had no answer
//
// "Noticed four, learned none" is true and useless. Is it one occasion short? Is the control
// unnameable? Does that button lead two different places? Four situations, four different things
// to do about them, and the counts cannot tell them apart. The policy has always had the
// sentences; this is what says them out loud.
//
// Grouped by what is BLOCKING rather than by application, because the thing somebody wants is
// "what do I do about this", and the answer is the same for everything in a group.
func printWatched(seen []service.WatchedView) {
	if len(seen) == 0 {
		fmt.Println("Marco hasn't seen anything happen twice yet.")
		fmt.Println("  it records a relationship the first time it sees you do something,")
		fmt.Println("  and remembers it once it has seen the same thing on another occasion")
		return
	}
	learned, waiting, never := 0, 0, 0
	for _, w := range seen {
		switch {
		case w.Learned:
			learned++
		case w.Verdict == "never":
			never++
		default:
			waiting++
		}
	}
	fmt.Printf("Marco has seen %s happen more than once.\n",
		plural(len(seen), "thing", "things"))
	fmt.Printf("  %d remembered, %d waiting for more, %d it can't learn as they stand\n\n",
		learned, waiting, never)

	for _, w := range seen {
		mark := " . "
		switch {
		case w.Learned:
			mark = " * "
		case w.Verdict == "never":
			mark = " x "
		}
		fmt.Printf("%s%s in %s\n", mark, describeWatched(w), w.Application)
		fmt.Printf("     seen %d time(s) on %s\n", w.Seen,
			plural(w.Occasions, "occasion", "separate occasions"))
		fmt.Printf("     %s\n", w.Said)
		if !w.FromKnown || !w.ToKnown {
			// SAID PLAINLY, because it is the commonest reason for waiting and it
			// sounds like a fault when it is not. A screen Marco has not established
			// yet is ordinary; it becomes durable the moment something learns it.
			fmt.Println("     (one or both screens are ones Marco hasn't established yet)")
		}
		fmt.Println()
	}
}

// describeWatched names one relationship in the words a person used, not Marco's.
func describeWatched(w service.WatchedView) string {
	switch {
	case w.Control != "":
		return fmt.Sprintf("pressing %q", w.Control)
	case w.Did != "":
		return w.Did
	}
	return "something"
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
	// AND WHETHER ANY OF IT IS BECOMING PERMANENT, said either way.
	//
	// This is the sentence that must not be missing. Somebody who typed `marco observe` and
	// walked away is entitled to know whether Marco is building durable memory from what it
	// sees, and a line that appeared only when it was would make its absence mean two things.
	//
	// Deleting either arm must fail TestWatchingSaysWhetherItIsAlsoLearning.
	if v.Learning {
		fmt.Printf("  learning from what it sees%s\n", learnedSoFar(v))
		fmt.Println("  stop learning with: marco observe learn off")
	} else {
		fmt.Println("  watching only — it isn't remembering any of this")
		fmt.Println("  let it learn with: marco observe learn")
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

// learnedSoFar is the parenthetical after "learning from what it sees".
//
// Counts and nothing else. What Marco has learned by watching is a real thing a person may want
// to know the size of; WHICH things is a question for the store, and putting a screen or a
// control's name in front of somebody here would be this command reporting their own afternoon
// back to them.
func learnedSoFar(v service.AmbientView) string {
	switch {
	case v.Learned > 0:
		return fmt.Sprintf(" (%s remembered so far)",
			plural(v.Learned, "thing", "things"))
	case v.Noticed > 0:
		return " (nothing remembered yet — it waits until it has seen a thing twice)"
	}
	return ""
}

// observeRequest turns a verb into the one request the Director answers.
//
// # Its own function so the decision can be tested where it is made
//
// Everything else in `runObserve` needs a Director on the other end of a socket. This does not:
// it is the whole of what the command DECIDES, and a test that had to spawn a service to find out
// whether `observe learn off` asks for less learning or more would be testing the socket.
//
// Reports false for a verb it does not know, so a misspelling is refused rather than falling
// through to something that changes state. `observe stpo` silently turning watching on is the kind
// of mistake somebody would not notice.
func observeRequest(sub string, rest []string) (service.ObserveAmbient, bool) {
	var q service.ObserveAmbient
	switch sub {
	case "", "on", "start":
		q.Enable = true
	case "off", "stop":
		q.Disable = true
	case "status":
		// A read. Deliberately asks for nothing: somebody asking whether Marco is paying
		// attention must not thereby make it start.
		//
		// `--evidence` widens the same read to what Marco has SEEN repeatedly and what it
		// is waiting for on each; the flag is applied by the caller, which is where the
		// parsed flags are. The counts alone answer "how much" and cannot answer "why not
		// yet", which is the question somebody actually has.
	case "learn", "learning":
		// LEARNING FROM WHAT IT SEES, which is a second thing to agree to and therefore a
		// second verb. `marco observe learn` turns it on; `marco observe learn off` turns
		// it off and leaves Marco watching.
		//
		// Under `observe` rather than beside it, because it is a property of watching and
		// meaningless without it — and named `learn` rather than `promotion` because the
		// person is deciding whether Marco may LEARN from them, not configuring a policy.
		//
		// Deleting the off arm must fail TestTurningLearningOffIsItsOwnVerb. A person
		// switching learning off and being given more of it is the worst possible
		// misreading of a command about permanence.
		if len(rest) > 0 {
			switch strings.ToLower(rest[0]) {
			case "off", "stop", "no":
				q.Unlearn = true
				return q, true
			}
		}
		q.Learn = true
	default:
		return service.ObserveAmbient{}, false
	}
	return q, true
}
