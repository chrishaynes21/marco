package main

import (
	"bufio"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The control centre's account of what a clicked Run actually did.
//
// # It used to say "ok" before anything had happened
//
// `handleDo` called `exec.Command(...).Start()` — no `Wait`, no pipe — and answered
// `{"ok":true}` on the next line. The browser rendered "running: X" and never changed it. So a
// play the authority door DECLINED, a play somebody STOPPED, a play that REFUSED because it could
// not recognise the screen, and a play that worked were all reported identically, and the one that
// worked was the only one the message was true for.
//
// The HUD had been reading the engine's own answer off the child's stdout for a while. The control
// centre was the third surface with an opinion about one run, and the only one not looking.
//
// # Why a run id and not a blocking handler
//
// Because a play can run for a minute. An HTTP handler that waited would hold the browser's
// request open for the length of it, and a browser that gave up first would leave the page with no
// answer at all for a run that was still going perfectly well.
//
// So the handler starts the run, answers immediately with an id, and the page asks about that id
// until it is finished. What it asks about is the ENGINE'S OWN WORD — [outcome] is the one
// vocabulary, imported by the engine that prints it, by the HUD, and by this — so the two surfaces
// cannot drift into two accounts of one run without failing to compile.
//
// # What this deliberately is not
//
// It is not a durable history. Nothing here is written to disk, and it is bounded and swept, so a
// control centre left open for a week does not accumulate a second record of everything Marco has
// ever done beside the Director's. "What happened over time" has an owner already; this answers
// only "what became of the thing I just pressed".

// runRecord is one clicked Run, as it stands.
type runRecord struct {
	// Play is what to call it on screen.
	Play string
	// Done is false while it is still going.
	Done bool
	// Outcome is the engine's own word, once there is one.
	Outcome outcome.Outcome
	// Route is the play a loose phrase resolved to, when the engine announced one.
	Route string
	// Detail is the last line of the child's output when it ended badly, so a person is not
	// left with only a word.
	Detail string
	// Started is when it began, for the sweep and for a duration on screen.
	Started time.Time
}

// runAccount holds the runs this control centre started.
//
// Bounded by time rather than by count: a run is interesting while somebody might still be looking
// at the row that started it, and not afterwards.
type runAccount struct {
	mu   sync.Mutex
	next int
	runs map[string]*runRecord
}

// runMemory is how long a finished run stays answerable.
//
// Long enough that a person who pressed Run, looked away and looked back still gets an answer;
// short enough that this never becomes a log. A finished run nobody asked about in five minutes is
// a finished run nobody is going to ask about.
const runMemory = 5 * time.Minute

// begin records a run about to start and returns its id.
func (a *runAccount) begin(play string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Created on first use rather than by a constructor. An editor is built as a struct literal
	// in nine places, most of them tests, and a field somebody must remember to initialise is a
	// nil dereference waiting for whoever adds the tenth.
	if a.runs == nil {
		a.runs = map[string]*runRecord{}
	}
	a.sweepLocked()
	a.next++
	id := strconv.Itoa(a.next)
	a.runs[id] = &runRecord{Play: play, Started: time.Now()}
	return id
}

// finish records what became of it.
func (a *runAccount) finish(id string, o outcome.Outcome, route, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r, ok := a.runs[id]; ok {
		r.Done, r.Outcome, r.Detail = true, o, detail
		if route != "" {
			r.Route = route
		}
	}
}

// get answers "what became of the thing I pressed".
func (a *runAccount) get(id string) (runRecord, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.runs[id]
	if !ok {
		return runRecord{}, false
	}
	return *r, true
}

// sweepLocked drops runs nobody is going to ask about.
func (a *runAccount) sweepLocked() {
	cut := time.Now().Add(-runMemory)
	for id, r := range a.runs {
		if r.Done && r.Started.Before(cut) {
			delete(a.runs, id)
		}
	}
}

// start launches a play and begins listening for what became of it.
//
// It returns as soon as the process exists, because the caller is an HTTP handler and the play may
// run for a minute. Everything after that happens on the watcher goroutine.
//
// Note what is NOT here: no authority check, no scope decision, no resolution. This spawns
// `marco do` with an explicit play identity and the engine does all of that in one place
// ([[internal/invoke]]). A control centre that decided any of it would be the second intake this
// campaign exists to remove.
func (a *runAccount) start(rt routes.Route) (string, error) {
	return a.spawn(plays.Pretty(rt.Slug), doArgv(rt))
}

// startPhrase launches the person's OWN WORDS, and is otherwise the same act.
//
// Try it, on the Here panel. The only difference from a clicked Run is which of the two things the
// person actually did: chose a play from a list, or said what they wanted. Everything after that —
// resolution, authority, the actuation lease, verification — happens inside `marco do` either way,
// which is why this is a second argument vector and not a second mechanism.
//
// The label is the phrase itself, because that is what they will be looking for in the answer.
func (a *runAccount) startPhrase(text string) (string, error) {
	return a.spawn(text, phraseArgv(text))
}

// spawn is the body both doors share: open an account, start the process, listen for its word.
func (a *runAccount) spawn(label string, argv []string) (string, error) {
	id := a.begin(label)
	cmd, out, err := runSpawn(argv)
	if err != nil {
		a.finish(id, outcome.Unavailable, "", err.Error())
		return id, err
	}
	if cmd == nil {
		// A test seam handed back no process. Nothing to watch, and nothing happened.
		a.finish(id, outcome.Unavailable, "", "nothing was started")
		return id, nil
	}
	go watchChild(cmd, out, func(o outcome.Outcome, route, detail string) {
		a.finish(id, o, route, detail)
	})
	return id, nil
}

// watchChild reads a running play's output and reports the engine's own word.
//
// # Every line is read, and almost none of it is kept
//
// A play's stdout carries its own logging as well as the two protocol lines. This keeps the LAST
// result announced, the route the phrase resolved to, and the last ordinary line — the last line
// because a failure the engine could only call "failed" usually said something more useful
// immediately before it, and a person reading a single word is entitled to the sentence too.
//
// A child that dies before announcing anything gets `failed`, which is the truthful reading of a
// non-zero exit with nothing said: the intake always announces, so silence means it never got
// there.
//
// Deleting the outcome.FromLine read must fail TestAClickedRunReportsWhatTheEngineSaid.
func watchChild(cmd *exec.Cmd, out io.Reader, done func(outcome.Outcome, string, string)) {
	var (
		got    outcome.Outcome
		spoke  bool
		route  string
		detail string
	)
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if o, ok := outcome.FromLine(line); ok {
			got, spoke = o, true
			continue
		}
		if r, ok := outcome.RouteFromLine(line); ok {
			route = strings.TrimSpace(r)
			continue
		}
		if s := strings.TrimSpace(line); s != "" {
			detail = s
		}
	}
	err := cmd.Wait()
	if !spoke {
		got = outcome.Failed
		if err == nil {
			// It exited cleanly and said nothing. Every subcommand that is not the intake
			// behaves this way, and for those "it did the thing or it errored" is the whole
			// vocabulary — so a clean exit is honestly `performed`.
			got = outcome.Performed
		}
	}
	if got == outcome.Performed {
		detail = ""
	}
	done(got, route, detail)
}

// prettyRun is what to show for a run: the play the engine resolved, or what was asked for.
func prettyRun(r runRecord) string {
	if r.Route != "" {
		return plays.Pretty(r.Route)
	}
	return r.Play
}
