package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
)

// stopVerb is the word `marco stop` sends, and it is the Audience's word, not a private one.
//
// It is spelled out rather than assembled so that a reader can see the thing that actually
// matters: this is ordinary text going through the ordinary door, and `intent.IsControlPhrase`
// recognises it on the far side exactly as it recognises the same word typed into the overlay or
// said out loud. There is no "CLI stop" path and no second definition of the word.
const stopVerb = "stop"

// runStop is `marco stop` — the Audience's one word for "that's enough", as a shell verb.
//
// # Why it does not just call runControl
//
// Because that would be the fourth intake, and removing the other three is what this phase is for.
// A verb with its own path is a place where the meaning of a word can drift: the moment `marco
// stop` reached the stop machinery directly, "stop" typed at a shell and "stop" said out loud
// would be two decisions in two files, and the next person to change one of them would have no
// reason to look for the other. So this builds an `invoke.Request` and hands it to
// `runInvocation`, which is what the overlay's typed line, the voice plugin, the control centre's
// Run and `marco do` all reach. `invoke.Decide` sees a control phrase at arm one and routes it,
// and this file contains no decision at all.
//
// The visible consequence is that `marco stop` gets the same six outcomes, the same `[result] `
// line on stdout and the same exit codes as everything else — so a script, or a front end, can
// read what happened without knowing which verb produced it.
//
// # Why it takes no arguments
//
// `marco stop <anything>` would have to mean something, and every meaning available is worse than
// refusing. Passing the words through would send "stop the music" to Director as a request to
// interpret against the screen, which is emphatically not what somebody hammering a stop verb
// wants. Ignoring them silently would be a command that reads one way and behaves another. So it
// says so and exits 2, the way every other misused verb here does.
//
// Deleting the runInvocation call must fail TestEveryEntranceRoutesThroughTheOneIntake.
func runStop(args []string) {
	if len(strings.Fields(strings.Join(args, " "))) > 0 {
		fmt.Fprintln(os.Stderr, "usage: marco stop   (it takes no arguments — it stops whatever is running)")
		os.Exit(2)
	}
	out, err := runInvocation(newDeps(), invoke.Request{
		Text: stopVerb, Source: invoke.SourceCLI,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	announce(out)
	os.Exit(out.Exit())
}
