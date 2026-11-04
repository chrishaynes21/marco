package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// OPENING THE WIDGET ASKS FOR THE DIRECTOR.
//
// Before, the widget never started one: its status reads never may (ADR-267), so the Director
// came up only when the first request that needed it arrived, and until then the watch panel and
// the headline said it was not running. Opening the widget is a person asking for it, so the
// widget asks once, when it opens, for a Director started FOR THIS PROCESS. That one stops once
// the widget has exited and nothing is using it (cmd/director servefor.go). One that is already
// running is left exactly as it is, and is not stopped when the widget closes.
//
// MARCO_DIRECTOR=off still means no Director is expected, so nothing is asked for.
//
// Deleting the call in main must fail TestOpeningTheWidgetAsksForTheDirector.

// askDirector runs one marco command and returns what it printed. A var so a test can see the
// arguments without starting anything.
var askDirector = func(args ...string) (string, error) {
	out, err := exec.Command(marcoBin(), args...).CombinedOutput()
	return string(out), err
}

// directorStartArgs is what the widget asks marco for: a Director started for this process.
func directorStartArgs(pid int) []string {
	return []string{"director", "start", "--for=" + strconv.Itoa(pid), "--json"}
}

// startDirectorForWidget asks for the Director, once, and logs what came of it. It never blocks
// the widget: main runs it on its own goroutine, and a Director takes seconds to come up.
func startDirectorForWidget() {
	if !directorEnabled() {
		mlogI("overlay: no Director asked for", "why", "MARCO_DIRECTOR is off")
		return
	}
	out, err := askDirector(directorStartArgs(os.Getpid())...)
	if err != nil {
		mlogE("overlay: the Director could not be started", "err", err,
			"said", strings.TrimSpace(out))
		return
	}
	mlogI("overlay: Director", "said", strings.TrimSpace(out))
}
