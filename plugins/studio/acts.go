package main

// THE ACT SURFACE STUDIO IS FOR: compose, select, prepare, run, cancel, inspect.
//
// A vertical slice, deliberately thin, over the path the Architect approved — and over the SAME
// functions the command-line path uses, not a second implementation of any of them:
//
//	compose   demonstrated.Compose + demonstrated.Save   the one generator, the one save
//	select    routes.Registry + demonstrated.Load        the plays on this machine
//	prepare   demo.Store.Sequences                        the demonstrations and their steps
//	run       director perform-workflow --play …          the production supervised runner
//	cancel    stopsignal.Raise                            the one cross-process stop
//	inspect   the run record this run wrote               by its canonical reference
//
// # Why running is a child process rather than a library call
//
// Because a supervised run needs a PERSON at the consent desk, and the desk reads F7/F8/F9 from a
// global low-level keyboard hook and reports on a transcript. That is `director
// perform-workflow`'s own composition root: the live observer, the person who grounds a screen,
// the person who permits a press, the executor, the stop key, the run record. Reaching into it
// from here would be a second place where all of that is assembled, and a second place is how two
// answers to "may this be pressed" come to exist.
//
// So Studio spawns it and shows what it says, which is exactly what `cmd/marco` already does for
// a demonstrated name (`runDirectorCommand`, one pipe for both streams, `runref.Parse` over the
// lines). The keyboard answers go to the real desk on the real hook; Studio is the window that
// shows the person what they are answering.
//
// # What it will not do
//
//   - press anything on its own. `--live` is a toggle that defaults OFF and says what off means.
//   - decide anything. Every refusal shown here came from the production code that refused.
//   - hold a question. The desk's prompts are the desk's; this renders the transcript.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// Acts is everything the page can ask about Acts and demonstrations.
//
// Separate from `Backend` because they answer to different things: `Backend` speaks to the
// Director SERVICE for the feasibility proof, and this speaks to the stores and to the supervised
// runner. Nothing here uses the service at all.
type Acts struct {
	// home is the Marco store. Resolved once; see storeDir.
	home string
	// routes is where plays live, honouring $MARCO_ROUTES exactly as marco and director do.
	routesDir string

	mu sync.Mutex
	// busy is the RESERVATION, taken before a child is started and released only once
	// that child has exited AND its transcript has been drained.
	//
	// # Why a reservation rather than a pointer to the child
	//
	// Because the first version checked `running != nil`, released the lock, started the
	// process, and only then stored it. Two calls arriving together both passed the check
	// and both started a supervised runner — two sets of consent questions on one
	// keyboard, where F7 answers whichever desk happens to be listening. The page's Run
	// button is disabled from the next poll, which is up to 400ms of open window.
	//
	// So the slot is claimed under the same lock that tested it, and it stays claimed
	// until there is nothing left of the run: a child that has exited with output still
	// in the pipe is a run whose transcript is still arriving.
	busy bool
	// running is the child, while there is one. Nil between runs and after shutdown.
	running *exec.Cmd
	// saying is where this run writes its structured state, and said whether anything
	// was ever read from it. One file per run, in a temporary directory: it is a
	// conversation between two processes and not something a person keeps.
	saying string
	// stopRun cancels the child's context, and done closes when the run has fully
	// finished — exited and drained. Both nil between runs. See Close.
	stopRun func()
	done    chan struct{}
	// act and live are what THIS run is, kept because the page's controls are not a
	// description of a run already in flight: a person who ticks the live box while
	// something is running must not be shown a dry run as live, or the other way about.
	act  string
	live bool
	// closing says shutdown has begun, and it is the gate on STARTING anything.
	//
	// # The race this closes
	//
	// `Run` claimed the slot, released the lock, built a pipe, started the process, and
	// only then published the child. `Close` returned at once when it saw no child — so a
	// window closed in that interval let the launch publish a runner afterwards, with
	// nothing left to stop it and cleanup already proceeding. A supervised runner alive
	// after the window that was asking its questions is gone.
	//
	// So shutdown covers the slot's WHOLE lifetime. Once this is set no claim may be
	// taken, a claim already taken aborts before anything is started, and `Close` does
	// not return while a launch could still publish a child.
	closing bool
	// afterReserve runs between claiming the slot and starting the child, and exists
	// only so a test can stand inside that interval.
	//
	// A seam for a timing window rather than a seam for behaviour: the interval is real,
	// it is where the race lived, and there is no way to be inside it from outside. Nil
	// in every production path.
	afterReserve func()
	// transcript is what the run has said so far, which is the whole of what a person
	// watching it has to go on.
	transcript []string
	// reference is this run's canonical identity, as the runner itself printed it.
	reference string
	// outcome is how it ended, once it has.
	outcome string
}

// marcoChecks answers `demonstrated.Plays` over the production checker, exactly as cmd/director
// does. One line, so the question a saved play is checked with is the one the runtime asks.
type marcoChecks struct{}

func (marcoChecks) Check(src string) error { return driver.CheckSource(src) }

// newActs resolves the stores this Studio reads.
func newActs(home string) *Acts {
	dir := os.Getenv("MARCO_ROUTES")
	if dir == "" {
		dir = "routes"
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return &Acts{home: home, routesDir: dir}
}

// ── PREPARE: what there is to compose from ────────────────────────────────────

// Interaction is one step of a demonstration, as a person choosing has to see it.
type Interaction struct {
	Position int    `json:"position"`
	Says     string `json:"says"`
	// Pressable says this step presses something. A step that pressed nothing cannot be
	// an interaction in an Act, and a person needs to see which is which rather than
	// find out from a refusal.
	Pressable bool `json:"pressable"`
	// Verifiable says the demonstration recorded an outcome for it, which is what makes
	// a verdict possible at all. Without one the interaction can never be verified, so an
	// Act cannot continue past it — see ADR-243. Shown because it is the single most
	// surprising thing about running an Act, and the one a person should know BEFORE
	// composing rather than after a run stops.
	Verifiable bool `json:"verifiable"`
}

// Demonstration is one saved routine and its interactions.
type Demonstration struct {
	Name        string        `json:"name"`
	Application string        `json:"application"`
	SavedAt     string        `json:"saved_at"`
	Steps       []Interaction `json:"steps"`
	// Problem is why it cannot be read, when it cannot. A routine that will not load is
	// a file to look at; one that was never saved is a demonstration to give.
	Problem string `json:"problem,omitempty"`
}

// Demonstrations is everything somebody has shown Marco, with the interactions of each.
func (a *Acts) Demonstrations() []Demonstration {
	store, err := demo.Open(a.home)
	if err != nil {
		return []Demonstration{{Name: "(the store could not be opened)",
			Problem: err.Error()}}
	}
	saved := store.Sequences()
	out := make([]Demonstration, 0, len(saved))
	for _, art := range saved {
		d := Demonstration{Name: art.Name,
			Application: string(art.Application),
			SavedAt:     art.SavedAt.Local().Format("2006-01-02 15:04"),
			Problem:     store.SequenceProblem(art.Name)}
		for _, s := range art.Steps {
			d.Steps = append(d.Steps, Interaction{
				Position:   s.Position,
				Says:       saysOf(s),
				Pressable:  s.Control != "",
				Verifiable: len(s.Outcome) > 0,
			})
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// saysOf is a step in words, preferring the demonstration's own sentence.
func saysOf(s workflowstore.Step) string {
	if s.Says != "" {
		return s.Says
	}
	if s.Control == "" {
		return fmt.Sprintf("%s to %s — nothing was pressed", orElse(s.From, "somewhere"),
			orElse(s.To, "somewhere"))
	}
	return fmt.Sprintf("press %s on %s", s.Control, orElse(s.From, "the screen"))
}

func orElse(s, then string) string {
	if s == "" {
		return then
	}
	return s
}

// ── COMPOSE: an Act from interactions somebody demonstrated ───────────────────

// Composed is what came of composing, for the page to render.
type Composed struct {
	Act string `json:"act"`
	// Play is the generated source. Shown, always: an Act is legal Marco so that a person
	// can read what they saved and recognise it.
	Play     string `json:"play"`
	Bindings string `json:"bindings"`
	Path     string `json:"path"`
	Error    string `json:"error,omitempty"`
}

// Compose writes an Act from a demonstration, in the order the interactions are given.
//
// `picked` is `"<position>=<name>"` per interaction, in run order — the same shape the command
// takes, for the same reason: the position says which demonstrated interaction is meant and the
// name is the person's own words, which is what the play will say.
func (a *Acts) Compose(from, act string, picked []string) Composed {
	store, err := demo.Open(a.home)
	if err != nil {
		return Composed{Error: err.Error()}
	}
	art, ok := store.Sequence(from)
	if !ok {
		if why := store.SequenceProblem(from); why != "" {
			return Composed{Error: fmt.Sprintf("%q cannot be read: %s", from, why)}
		}
		return Composed{Error: fmt.Sprintf("nothing called %q has been saved", from)}
	}
	named := make([]demonstrated.Named, 0, len(picked))
	for _, p := range picked {
		at, name, cut := strings.Cut(p, "=")
		if !cut || strings.TrimSpace(name) == "" {
			return Composed{Error: fmt.Sprintf("%q needs a name: write it as "+
				"<interaction>=<what you call it>", p)}
		}
		var n int
		if _, perr := fmt.Sscanf(strings.TrimSpace(at), "%d", &n); perr != nil {
			return Composed{Error: fmt.Sprintf("%q is not an interaction number", at)}
		}
		named = append(named, demonstrated.Named{Position: n,
			Name: strings.TrimSpace(name)})
	}
	src, binds, cerr := demonstrated.Compose(art, act, named, time.Now(), marcoChecks{})
	if cerr != nil {
		return Composed{Error: cerr.Error()}
	}
	rt := routes.Route{Slug: routes.Slug(act)}
	if serr := demonstrated.Save(a.reg(), rt, src, binds, time.Now()); serr != nil {
		return Composed{Error: serr.Error()}
	}
	raw, _ := binds.Marshal()
	return Composed{Act: act, Play: src, Bindings: string(raw),
		Path: a.reg().Path(rt)}
}

// reg is the route store, built the same way director's is.
func (a *Acts) reg() routes.Registry { return routes.Registry{Dir: a.routesDir} }

// ── SELECT and INSPECT: the Acts on this machine, and whether they will run ───

// Act is one saved play, as a person choosing one has to see it.
type Act struct {
	Name string `json:"name"`
	// From and Revision are the demonstration it performs and the revision it is pinned
	// to. Both, because a record with only the first cannot tell two Acts over one
	// demonstration apart.
	From     string `json:"from"`
	Revision string `json:"revision"`
	// Interactions is what it performs, in the order it performs them.
	Interactions []string `json:"interactions"`
	Application  string   `json:"application"`
	Play         string   `json:"play"`
	// Runnable is whether it preflights RIGHT NOW, and Why the refusal when it does not.
	//
	// Asked here rather than guessed, through the production preflight: the bindings may
	// have been edited, the demonstration may have been re-saved, the play may have been
	// changed by hand. A person should see that before they start, not after.
	Runnable bool   `json:"runnable"`
	Why      string `json:"why,omitempty"`
}

// Saved is every Act on this machine, with whether each one would run.
func (a *Acts) Saved() []Act {
	store, serr := demo.Open(a.home)
	out := []Act{}
	for _, rt := range a.reg().List() {
		src, binds, err := demonstrated.Load(a.reg(), rt)
		if err != nil {
			// NOT AN ACT. A play with no bindings is an ordinary play, which is
			// the right fail-closed direction and not something to report as
			// broken — see ADR-243. Skipped rather than listed.
			continue
		}
		act := Act{Name: binds.Act, From: routineOf(binds), Play: src,
			Application: binds.Application}
		if act.Name == "" {
			act.Name = rt.Slug
		}
		for _, s := range binds.Scenes {
			act.Interactions = append(act.Interactions, s.Name)
			act.Revision = s.Revision
		}
		if serr != nil {
			act.Why = serr.Error()
		} else if _, perr := demonstrated.Preflight(src, binds, store,
			marcoChecks{}); perr != nil {
			act.Why = perr.Error()
		} else {
			act.Runnable = true
		}
		out = append(out, act)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func routineOf(b demonstrated.Bindings) string {
	for _, s := range b.Scenes {
		return s.Routine
	}
	return ""
}

// ── RUN: the production supervised runner, as a child, with its transcript ────

// Running is what the page shows while a run is in flight and after it ends.
type Running struct {
	Act        string   `json:"act"`
	Live       bool     `json:"live"`
	Reference  string   `json:"reference"`
	Outcome    string   `json:"outcome"`
	Transcript []string `json:"transcript"`
	Active     bool     `json:"active"`
	// State is the RUNNER'S OWN latest account of itself, when there is one.
	//
	// Not derived from the transcript any more. Studio used to match on sentences —
	// "May Marco", "Is that the screen you are on?" — so improving the wording of a
	// consent prompt would have silently changed what a window showed somebody about a
	// press they were about to authorise. See internal/director/runstate.
	State *runstate.Event `json:"state,omitempty"`
	// Asking is the question somebody CAN STILL ANSWER, which is not every question the
	// state carries.
	//
	// A separate field because the two claims are different and only one of them is about
	// the keyboard. `State.Question` is what the run was waiting for when it last said
	// anything — evidence, kept. This is an invitation to press a key, and it is only
	// honest while there is a process on the other end of the hook to hear it: a child
	// that exited with a question open left a snapshot that still carried it, and the page
	// went on offering its keys. Somebody pressing F7 at that is pressing it at nothing,
	// in a window that just told them a press was waiting on their answer.
	Asking *runstate.Question `json:"asking,omitempty"`
	// Reason is why it ended, from the final event.
	Reason string `json:"reason,omitempty"`
	// Complete says the run ENDED — it said so itself, with a final event. Incomplete
	// says the child is gone and never did, which is not the same as having failed:
	// one is a run with an answer and the other is a run nobody can speak for.
	Complete   bool `json:"complete"`
	Incomplete bool `json:"incomplete"`
}

// Run performs an Act through `director perform-workflow --play`.
//
// Returns at once; the transcript arrives through `Watch`. A page that blocked on this would be a
// page that cannot show a prompt, which is the one thing a person has to see.
//
// `live` is the whole of the decision to touch the machine, and it is the caller's: without it
// the entire path runs and the host writes each program down instead of pressing anything.
func (a *Acts) Run(act, application string, live, reading bool) string {
	// THE SLOT IS CLAIMED UNDER THE SAME LOCK THAT TESTED IT. See `busy`.
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return "Studio is closing, so nothing new is started"
	}
	if a.busy {
		a.mu.Unlock()
		return "a run is already in flight; one at a time, because the consent desk " +
			"is a person's attention and there is one of those"
	}
	a.busy = true
	a.transcript, a.reference, a.outcome = nil, "", ""
	a.act, a.live, a.saying = act, live, ""
	a.mu.Unlock()
	// AND RELEASED AGAIN ON EVERY PATH THAT DOES NOT REACH A RUNNING CHILD. A slot
	// claimed and then abandoned is a Studio that refuses every later run.
	release := func() {
		a.mu.Lock()
		a.busy, a.running, a.stopRun, a.done = false, nil, nil, nil
		a.mu.Unlock()
	}
	// THE INTERVAL THE RACE LIVED IN. Nil in production; see afterReserve.
	a.mu.Lock()
	pause := a.afterReserve
	a.mu.Unlock()
	if pause != nil {
		pause()
	}

	// WHERE THIS RUN WILL SAY WHAT IT IS DOING. One file per run in a temporary
	// directory: a conversation between two processes, not something a person keeps.
	// A failure to make it is not a reason to refuse the run — the transcript still
	// arrives, and a window with no structured state is worse than no window only if
	// it pretends otherwise, which `Incomplete` prevents.
	saying := ""
	if f, ferr := os.CreateTemp("", "studio-run-*.jsonl"); ferr == nil {
		saying = f.Name()
		_ = f.Close()
	}
	args := []string{"perform-workflow", "--play", act, "--hotkeys"}
	if saying != "" {
		args = append(args, "--events", saying)
	}
	if application != "" {
		args = append(args, "--application", application)
	}
	if live {
		args = append(args, "--live")
	} else {
		// NO DWELL, AND ONLY FOR A DRY RUN.
		//
		// The dwell rests the pointer on the control and then CHECKS that the
		// operating system actually put it there — ADR-198, a move that reported
		// completed and did not happen. In a dry run the move is written down
		// instead of performed, so the pointer never arrives and that check refuses
		// the click, correctly, for a reason that has nothing to do with the Act.
		//
		// A LIVE run must inherit the production default. Passing `--dwell 0s`
		// always, as the first version did, took the pointer-rest, the
		// pointer-arrival measurement and the hover probe out of exactly the runs
		// that need them — the game-menu work those three were built for.
		args = append(args, "--dwell", "0s")
	}
	if reading {
		args = append(args, "--reading")
	}

	// A CANCELLABLE CONTEXT, so closing the window can end the run rather than orphan
	// it. `context.Background()` left a supervised runner alive after the window that
	// was asking its questions had gone.
	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, directorBin(), args...)
	// THE SAME STORE, EXPLICITLY. Both this process and the runner read $MARCO_HOME, and
	// a child with a different answer would ground against one store while the window
	// listed another.
	cmd.Env = append(os.Environ(), "MARCO_HOME="+a.home)
	// ONE TRANSCRIPT, IN THE ORDER A PERSON AT A TERMINAL WOULD SEE IT. The runner says
	// some things on stderr — a provider that is missing, a record that could not be
	// written — and every one of them is part of the account of what happened.
	pr, pw, err := os.Pipe()
	if err != nil {
		stop()
		release()
		return err.Error()
	}
	cmd.Stdout, cmd.Stderr = pw, pw

	// STARTING AND PUBLISHING ARE ONE ATOMIC STEP, under the lock, with the closing check
	// inside it.
	//
	// That is the whole of the fix. Separately, there was always a moment when the slot
	// was claimed and no child existed, and `Close` returning in that moment let this
	// launch publish a runner nobody would stop. Now a shutdown that began before this
	// point is seen here and nothing is started; a shutdown that begins after it finds a
	// child already published. There is no third case.
	//
	// Holding the lock across `Start` costs a page poll a millisecond or two, which is
	// the entire price.
	drained := make(chan struct{})
	finished := make(chan struct{})
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		pw.Close()
		pr.Close()
		stop()
		release()
		return "Studio is closing, so nothing new was started"
	}
	if err := cmd.Start(); err != nil {
		a.mu.Unlock()
		pw.Close()
		pr.Close()
		stop()
		release()
		return err.Error()
	}
	a.running, a.stopRun, a.done, a.saying = cmd, stop, finished, saying
	a.mu.Unlock()
	pw.Close()

	go func() { a.read(pr); close(drained) }()
	go func() {
		_ = cmd.Wait()
		// EXITED IS NOT FINISHED. A child that has gone with output still in the
		// pipe is a run whose transcript is still arriving, and the slot stays
		// claimed until there is nothing left of it.
		<-drained
		stop()
		release()
		close(finished)
	}()
	return ""
}

// Close ends a run this Studio started and waits for it to finish.
//
// # Why the window closing has to do this
//
// Because the child was given `context.Background()`, so closing Studio left a supervised runner
// ALIVE — still grounding, still able to ask for a press, with the window that was showing the
// questions gone. A person would have had no way to see what it was asking and no way to answer
// it except the keys.
//
// The stop is the product's own: `stopsignal.Raise` is what F9 and `director stop` raise, and the
// runner notices at the last moment it could matter and stops at a safe point. Cancelling the
// context as well is the backstop for a child that is not listening.
//
// BOUNDED, and the order matters: the run is given time to write its record before anything is
// removed, because a temporary store deleted first would take the record with it. Past the bound
// the child is killed — a shell that waited forever on shutdown is a shell somebody has to kill.
func (a *Acts) Close() {
	// CLOSING IS DECLARED FIRST, so nothing new can be claimed and a launch already
	// inside its own reservation aborts rather than starting anything.
	a.mu.Lock()
	a.closing = true
	busy, cmd, stop, done := a.busy, a.running, a.stopRun, a.done
	a.mu.Unlock()
	if !busy {
		// NOTHING IS CLAIMED, so there is nothing to stop and nothing that can
		// appear: a later `Run` is refused by the flag just set.
		return
	}

	// AND IF THE SLOT IS CLAIMED WITH NO CHILD YET, WAIT FOR ONE OR FOR THE ABORT.
	//
	// This is the interval the race lived in. A launch between its reservation and its
	// start is about to do one of two things — publish a child, or see `closing` and give
	// the slot back — and shutdown may not proceed until it has done one of them.
	// Returning here was what let a runner outlive the window.
	//
	// Bounded, because a shutdown that waits forever is a window somebody has to kill. A
	// launch cannot be slow: between the two points it creates a pipe.
	if cmd == nil {
		waited := time.Now().Add(2 * time.Second)
		for time.Now().Before(waited) {
			a.mu.Lock()
			busy, cmd, stop, done = a.busy, a.running, a.stopRun, a.done
			a.mu.Unlock()
			if !busy || cmd != nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if cmd == nil {
		// THE LAUNCH ABORTED, or never reached a child. Either way nothing is
		// running and nothing can start.
		return
	}
	// ASK FIRST, the way a person would.
	_ = stopsignal.Raise(stopsignal.Home())
	select {
	case <-done:
		return
	case <-time.After(6 * time.Second):
	}
	// THEN CANCEL THE CONTEXT, which kills the process group.
	if stop != nil {
		stop()
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		// AND SAY SO RATHER THAN HANGING. A run that will not end is worth a line on
		// stderr; a window that will not close is worse than either.
		fmt.Fprintln(os.Stderr, "studio: the supervised run did not finish within "+
			"nine seconds of being stopped; leaving it to the operating system")
	}
}

// read takes the child's lines, keeps them, and reads the canonical reference out of them.
func (a *Acts) read(pr *os.File) {
	defer pr.Close()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		a.mu.Lock()
		a.transcript = append(a.transcript, line)
		// THE RUN'S OWN REFERENCE, taken from the line the runner printed rather
		// than invented here. `runref.Parse` is the shared format both binaries
		// already read, and the outcome arrives on the closing citation.
		if r, ok := runref.Parse(line); ok {
			a.reference = r.ID
			if r.Outcome != "" {
				a.outcome = string(r.Outcome)
			}
		}
		a.mu.Unlock()
	}
}

// Watch is the run as it stands, for the page to poll.
func (a *Acts) Watch() Running {
	a.mu.Lock()
	// THE RUN'S OWN ACT AND MODE, not the page's controls. A person who ticks the live
	// box while something is running must not be shown a dry run described as live.
	w := Running{Act: a.act, Live: a.live,
		Reference: a.reference, Outcome: a.outcome,
		Transcript: append([]string(nil), a.transcript...),
		Active:     a.busy}
	path, started := a.saying, a.saying != ""
	a.mu.Unlock()
	if !started {
		return w
	}

	// ── AND WHAT THE RUN SAYS ABOUT ITSELF, WHICH OUTRANKS EVERYTHING ABOVE ──
	//
	// The latest snapshot is the whole current state, so rendering it cannot leave a
	// stale prompt on screen: a question that has been answered is simply absent from
	// the event after it.
	//
	// Where the event and this surface's own bookkeeping both have an answer, the
	// event's wins — it was written by the thing that decided. The bookkeeping covers
	// the window before the first event arrives, which is real: the run has been
	// started and has not said anything yet.
	e, final, err := runstate.Latest(path)
	if err == nil && e.Version != 0 {
		w.State = &e
		w.Complete = final
		if e.Run != "" {
			w.Reference = e.Run
		}
		if e.Act != "" {
			w.Act = e.Act
		}
		w.Live = e.Live
		if e.Outcome != "" {
			w.Outcome, w.Reason = string(e.Outcome), e.Reason
		}
	}
	// AN INCOMPLETE RUN: the child is gone and it never said how it ended.
	//
	// Not a failure, and reported apart from one. A run that refused says so in a final
	// event; a child that vanished — killed, crashed, or stopped by something outside
	// Marco — leaves a stream with no end to it, and the honest thing to show a person
	// is that nobody can speak for what happened.
	if !w.Active && !w.Complete {
		w.Incomplete = true
	}
	// ── AND WHETHER ANYBODY CAN STILL ANSWER WHAT IT WAS ASKING ──
	//
	// Last, because it depends on both answers above. A run that ENDED has no open
	// question by construction — a final event carries none — and a run whose child is
	// GONE may well have one, which is exactly the case this is here for. See `Asking`.
	if w.Active && !w.Complete && !w.Incomplete && w.State != nil {
		w.Asking = w.State.Question
	}
	return w
}

// ── CANCEL: the one cross-process stop ────────────────────────────────────────

// Cancel stops a supervised run, wherever it is.
//
// `stopsignal` is the one stop broadcast in this product, and it is what F9 and `director stop`
// raise. Not a kill: the runner notices at the last moment it could matter and stops at a safe
// point, which is the difference between ending a run and abandoning a press mid-flight.
func (a *Acts) Cancel() string {
	if err := stopsignal.Raise(stopsignal.Home()); err != nil {
		return err.Error()
	}
	return ""
}

// ── INSPECT: the record the run wrote, by its own reference ───────────────────

// Inspect reads back the durable record of a run, by the reference the run printed.
//
// The record rather than the transcript: the transcript is what was said, and this is what was
// kept — the outcome, the counts, the Act and the demonstration both.
func (a *Acts) Inspect(reference string) map[string]any {
	if reference == "" {
		return map[string]any{"error": "no run reference"}
	}
	p := filepath.Join(a.home, "runs", reference+".json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return map[string]any{"error": err.Error(), "path": p}
	}
	var rec map[string]any
	if jerr := json.Unmarshal(raw, &rec); jerr != nil {
		return map[string]any{"error": jerr.Error(), "path": p}
	}
	rec["path"] = p
	return rec
}
