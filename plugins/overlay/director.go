package main

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// Routing spoken phrases to the Director.
//
// This file is the whole of the overlay's involvement in semantic desktop control,
// and its smallness is the point. The overlay captures speech, renders what the
// Director says, and submits the phrase. It does not decide what a phrase means, and
// it does not choose between candidates when the Director asks a question — the next
// spoken phrase is simply submitted, and the SERVICE recognises it as the answer to
// its own pending question. Nothing about that decision lives here.
//
// Two properties matter for voice specifically:
//
//   - Dispatch is asynchronous. A replay can run for minutes, and a spoken "stop"
//     arrives as a second phrase while the first is still going. If dispatch blocked
//     the act handler, the stop could not be heard — which would make cancellation
//     impossible exactly when it is wanted.
//   - A phrase is never silently dropped. If the Director cannot be reached, the
//     phrase falls back to the legacy route path; if that fails too, it is logged.

// directorMode reads the kill switch.
//
// MARCO_DIRECTOR=off restores the pre-Director behaviour exactly: phrases go straight
// to route lookup. Worth having as a plain env var rather than a setting, because the
// reason to reach for it is that something is wrong.
func directorEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_DIRECTOR"))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

// directorUnavailable is the exit code `marco director` uses for "never delivered".
//
// It is deliberately distinct from ordinary failure. A command the Director ran and
// could not verify has been HANDLED, and re-running it as a legacy route would be a
// second attempt at the same thing the user never asked for. Only an undelivered
// phrase is eligible for the fallback.
const directorUnavailable = 3

// directorBusy tracks whether a Director command is in flight, purely so the HUD can
// say so. It never gates dispatch: a phrase spoken during a command is usually "stop".
var directorBusy atomic.Bool

// dispatchVoice sends a final recognised phrase to the Director, falling back to the
// legacy route path if the Director cannot be reached.
//
// Returns immediately; the work happens on its own goroutine.
func dispatchVoice(h *model, phrase string) {
	if !directorEnabled() {
		go runRecord(h, phrase, "do", phrase)
		return
	}
	go func() {
		delivered, ok := runDirectorPhrase(h, phrase)
		if delivered {
			return
		}
		// Not delivered — the Director is unavailable. The phrase still has to go
		// somewhere, so it takes the path it would have taken before any of this
		// existed.
		mlogW2("overlay: director unavailable — falling back to routes", "phrase", phrase, "why", ok)
		h.log("director unavailable — trying saved plays")
		_, outcome, route := runRecord(h, phrase, "do", phrase)
		// Only an UNRESOLVED phrase becomes a teach offer. A learned play resolves,
		// announces its [route], and is then performed by the Director — and that can
		// fail honestly. Offering to teach it again would answer "I couldn't get there
		// from here" with "shall I learn this?", about a play already learned.
		if outcome == "failed" && route == "" {
			offerTeach(h, phrase)
		}
	}()
}

// runDirectorPhrase runs `marco director "<phrase>"` and streams its lines into the
// HUD. It reports whether the phrase was delivered, and a short reason when it was not.
//
// Streaming rather than waiting for a result is what makes a long command legible:
// ACKNOWLEDGED lands within a moment of speaking, PROGRESS lines follow, and the
// terminal state arrives whenever it arrives.
func runDirectorPhrase(h *model, phrase string) (delivered bool, why string) {
	start := time.Now()
	directorBusy.Store(true)
	defer directorBusy.Store(false)

	cmd := exec.Command(marcoBin(), "director", phrase)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err.Error()
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false, err.Error()
	}
	if err := cmd.Start(); err != nil {
		return false, err.Error()
	}

	h.setStatus("director: " + phrase)
	mlogI("overlay: director dispatch", "phrase", phrase)

	// stderr carries only diagnostics; it is logged but never mistaken for a result.
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				mlogW2("overlay: director stderr", "line", line)
			}
		}
	}()

	var last string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		last = line
		h.log(line)
		// The status line shows the most recent thing the Director said, so a glance
		// at the HUD answers "what is it doing now" without reading the log.
		h.setStatus(directorStatusLine(line, phrase))
	}

	err = cmd.Wait()
	code := exitCodeOf(err)
	d := time.Since(start)

	if code == directorUnavailable {
		return false, "the Director could not be reached"
	}
	// Anything else — success, failure, an unanswered question — is a delivered
	// phrase. The Director had it and said something about it.
	outcome := "ok"
	if code != 0 {
		outcome = "failed"
	}
	if strings.HasPrefix(last, "CLARIFICATION_REQUIRED") {
		// A question is not a failure, and marking it as one in the history would
		// read as though the request had been refused.
		outcome = "canceled"
		h.setStatus("director asked — answer it")
	}
	h.addHistory(phrase, outcome, d)
	mlogI("overlay: director done", "phrase", phrase, "code", code, "took", d.String())
	return true, ""
}

// directorStatusLine turns one rendered event into a short status.
//
// The Director's own wording is preferred wherever it exists — re-describing its
// result here would mean maintaining a second vocabulary that could drift from the
// first.
func directorStatusLine(line, phrase string) string {
	switch {
	case strings.HasPrefix(line, "heard: "):
		return "director: " + phrase
	case strings.HasPrefix(line, "CLARIFICATION_REQUIRED"):
		return "director asked — answer it"
	case strings.HasPrefix(line, "["):
		return strings.TrimSpace(line) // "[3/5] iteration 3"
	}
	if len(line) > 60 {
		return line[:57] + "..."
	}
	return line
}

// exitCodeOf extracts a process exit code; -1 when the process produced none.
//
// A child that could never be STARTED is handled earlier, as undelivered — marco.exe
// missing from PATH leaves the phrase exactly as stranded as an absent Director.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
