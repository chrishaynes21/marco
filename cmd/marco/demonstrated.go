package main

// THE OTHER HALF OF THE PRODUCT, REACHED BY ITS NAME.
//
// # What was missing
//
// A person can show Marco a game menu with their own hands: `director demonstrate-visual` watches
// them click, saves the screens it settled on, the control they pressed and what the press did.
// And `director perform-workflow --name` performs one of those, supervised — grounded on a fresh
// look, authorised a single action at a time, pressed at the moment that was authorised, and
// verified against what the demonstration saw. Both halves worked, and were validated
// separately.
//
// They did not meet. Typing the name into the overlay reached `marco do`, which asked the one
// door what the words meant; the door knew about PLAYS, found none, and handed the words to
// Director — which read them against the screen and reported that nothing matching them was
// there. The thing the person had pointed at was reachable only by typing a developer's command
// into a terminal. So the product could be demonstrated and could not be used.
//
// # What this is, and what it is NOT
//
// It is the bridge from the door's verdict to the ONE runner that can perform a demonstration. It
// resolves nothing (`invoke.Decide` did that, through this file's own resolver), decides nothing
// about permission (the runner asks, per action, with a fresh look), and performs nothing itself.
//
// It is NOT a second performer. `marco` cannot do this work: a demonstration carries no verbs,
// and grounding a control in pixels needs an observation loop, a stabiliser, a consent desk and a
// run record — all of which exist, once, in the Director. The same reasoning as perform.go, where
// a LEARNED play is delegated rather than run: `marco` is the front end a person is looking at,
// and the half that can see is next door.
//
// # Why a child process and not a service request
//
// Because the supervised lifecycle is a COMMAND, not a query. It owns a countdown somebody
// prepares during, a consent desk that reads real function keys off the desktop, a run record
// reserved before anything is printed, and a terminal transcript that is the product's own
// account of what happened. `marco` already locates and launches `director.exe` — it starts the
// service that way — so spawning the runner is not a new dependency, and its stdout is already
// the stream every front end reads.
//
// Putting the lifecycle behind a service request instead would mean a second home for it, which
// is the one thing this phase was told not to build.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

// demoSights answers the door's question about what a person has demonstrated.
//
// It reads the store directly rather than through the service, for the reason `savedSequences`
// does the same on the Director's side: a saved routine is a file, read fresh on every call, and
// the daemon holds no copy. Asking a service would mean starting one to answer a question about
// a file — on every phrase somebody types.
type demoSights struct {
	// dir is the store's home, which is MARCO_HOME when it is set. The same directory the
	// runner will read the routine back out of, so the two cannot disagree about which
	// store this is.
	dir string
}

// Demonstration implements invoke.Sights.
//
// The APPLICATION it answers with is the artifact's own scope, and the name is the artifact's own
// name rather than the words that found it: the runner looks the name up again, and handing it
// back what somebody typed would be a second chance to miss.
//
// # Why it answers only for a routine that can actually be performed
//
// Three states, one predicate — `workflowstore.Artifact.PerformableBySight`, the same one the
// Director's listing uses to tell a person how to run a routine. A verb routine is not claimed
// here, so a name that runs something today keeps running it. An unscoped routine is valid for no
// application and the runner would refuse it. A routine with no visual evidence has nothing to
// ground.
//
// Answering for one of those would mean the door promising something the runner then declines,
// which from the person's side is Marco accepting a request and then arguing with itself.
func (s demoSights) Demonstration(app, name string) (invoke.Demonstrated, bool) {
	if strings.TrimSpace(name) == "" {
		return invoke.Demonstrated{}, false
	}
	store, err := demo.Open(s.dir)
	if err != nil {
		// NO STORE IS NO DEMONSTRATION. Not an error to report here: the door is
		// asking whether Marco already knows this name, and "the store could not be
		// opened" is answered honestly by "not from here". Logged, because a person
		// whose demonstrations have stopped answering deserves a trail.
		mlog.Error("intake: the demonstration store could not be read",
			"dir", s.dir, "err", err)
		return invoke.Demonstrated{}, false
	}
	a, ok := store.Sequence(name)
	if !ok || !a.PerformableBySight() {
		return invoke.Demonstrated{}, false
	}
	return invoke.Demonstrated{Name: a.Name, Application: string(a.Application)}, true
}

// performDemonstration carries out what the person demonstrated, by sight.
//
// Everything the runner says goes straight to this process's own stdout, because that is the
// stream the overlay is already reading and the Director's own wording is the product's account
// of what is happening. Re-describing a countdown or a question here would be a second
// vocabulary that could drift from the first.
//
// What this adds is the one thing a front end needs and a transcript cannot give it: the run's
// one-word outcome, in Marco's own words, taken from the REFERENCE the runner printed.
func performDemonstration(dec invoke.Decision, _ invoke.Request) (Outcome, error) {
	d := dec.Demonstration
	args := performWorkflowArgs(d)
	mlog.Info("intake: performing a demonstration by sight",
		"name", d.Name, "application", d.Application, "args", args)
	ref, err := spawnRunner(args, runnerTranscript)
	if err != nil {
		// THE RUNNER COULD NOT BE REACHED AT ALL, which is a different thing from a
		// run that refused: nobody took the request. See outcome.Unavailable.
		fmt.Fprintf(os.Stderr, "marco: the supervised runner could not be started: %v\n", err)
		return OutcomeUnavailable, nil
	}
	if ref.Outcome == "" {
		// IT RAN AND SAID NOTHING ABOUT HOW IT ENDED. The reference is the one line
		// whose job is to say, so its absence is not "it worked": it is a run nobody
		// can retrieve, which is the defect ADR-240 named `started`.
		mlog.Error("intake: the supervised run printed no outcome", "name", d.Name)
		return OutcomeFailed, nil
	}
	return marcoWord(ref.Outcome), nil
}

// performWorkflowArgs is the invocation, and every flag on it is load-bearing.
//
// Deleting any of them must fail TestADemonstrationIsPerformedWithTheFlagsItNeeds.
func performWorkflowArgs(d invoke.Demonstrated) []string {
	return []string{
		"perform-workflow",
		// THE NAME THE STORE ANSWERED TO, looked up again on the other side through
		// the same store. See ADR-239.
		"--name", d.Name,
		// THE APPLICATION, because the alternative is the foreground — and at the
		// moment somebody types a name into the overlay, the foreground is the
		// overlay. A selector, not a permission: the artifact's scope is still
		// checked against the window actually acquired.
		"--application", d.Application,
		// REAL INPUT. A person who asks Marco to do the thing they demonstrated is
		// asking for the thing to happen; a dry run here would be Marco reporting
		// success for a press it never sent. Every action is still authorised one at
		// a time, with a fresh look, by a key this process cannot send.
		"--live",
		// ANSWERED BY FUNCTION KEY, which is the only way it can be answered from
		// here. Without this the runner asks on its own standard input, and a child
		// spawned by a front end has no terminal to answer on — so every action
		// would hang until its question expired. It also keeps the game in front,
		// which is where it has to be for a click to land.
		"--hotkeys",
		// THE WORDS ON EACH LOOK. The demonstration's outcome expectation is words,
		// and two visually similar menus are told apart by what they say — see
		// ADR-194. Without it a run can verify nothing and may merge two screens.
		"--reading",
	}
}

// marcoWord translates the run's own outcome into the vocabulary every surface already reads.
//
// # Why a translation and not a passthrough
//
// Marco's outcomes are what a front end's history, exit code and announcement are built on, and
// they already distinguish the things that matter here — `outcome.Unverified` exists precisely
// because finishing is not arriving (ADR-070). A new word for a demonstration would mean every
// surface learning an eighth one, and the surfaces that did not would file it under "unknown".
//
// The mapping is not arbitrary and two arms are worth reading twice:
//
//   - CONTRADICTED is `failed`, not `unverified`. The evidence did not fail to decide; it decided
//     against. Reporting that as "I could not tell" would lose the only clear bad news the
//     verifier produces.
//   - STARTED is `failed`, and it means this command has a bug: the run reserved a reference and
//     left by a path that wrote no outcome. It is reported rather than smoothed over, because a
//     reference that retrieves `started` is the one thing that makes such a path findable.
func marcoWord(out runref.Outcome) Outcome {
	switch out {
	case runref.Verified:
		return OutcomePerformed
	case runref.Unconfirmed:
		return OutcomeUnverified
	case runref.Contradicted:
		return OutcomeFailed
	case runref.Refused:
		return OutcomeRefused
	case runref.Cancelled:
		return OutcomeCancelled
	case runref.Failed, runref.Started:
		return OutcomeFailed
	}
	// A WORD THIS BUILD DOES NOT KNOW. Not success: a front end that read an unknown
	// outcome as performed would announce a press nobody can account for.
	mlog.Error("intake: the supervised run ended in a word this build does not know",
		"outcome", string(out))
	return outcome.Failed
}

// spawnRunner is how the demonstration path reaches the Director's command line.
//
// A package variable for the same reason `dialPerformer` is one: `marco do` has to be exercisable
// end to end without launching a process that drives the real mouse. Production never reassigns
// it.
var spawnRunner = runDirectorCommand

// runnerTranscript is where a performed run's own account of itself goes.
//
// This process's stdout, because that is the stream every front end already reads: the overlay
// puts each line into the HUD's tail and the console behind it keeps the lot. A test that drives
// the REAL runner reads it here instead, which is the only way to prove the transcript survives
// the hop between two binaries. Production never reassigns it.
var runnerTranscript io.Writer = os.Stdout

// runDirectorCommand runs one `director` sub-command, copying its transcript to `out` and
// returning the run reference it printed.
//
// # Why the transcript is read rather than inherited
//
// Inheriting stdout would be simpler and would lose the reference: the one line that says which
// attempt this was, where its record went and how it ended. So every line is copied through AND
// parsed, which also means the front end reading this process's stdout sees exactly what a person
// at a terminal sees, in the same order.
//
// The LAST reference wins, because the runner prints one before anything happens — so that a run
// which dies early is still citable — and prints it again with the outcome at the end.
func runDirectorCommand(args []string, out io.Writer) (runref.Reference, error) {
	cmd := exec.CommandContext(context.Background(), directorBin(), args...)
	// THE SAME STORE, EXPLICITLY. `marco` and the runner both read MARCO_HOME, and a
	// child that inherited an empty environment would read the real store while this
	// process read a test one — which is how a test drives the real desktop.
	cmd.Env = os.Environ()
	// ONE TRANSCRIPT, IN THE ORDER A PERSON AT A TERMINAL WOULD SEE IT.
	//
	// Both streams into one pipe, which is what the overlay already does with this
	// process's own output. The runner says some things on stderr — a provider that is
	// not installed, a record that could not be written, a flag it does not know — and
	// every one of them is part of the account of what happened. Splitting them would
	// mean the front end showing one half and losing the other, and the half it lost
	// would be the half that says why.
	pr, pw, err := os.Pipe()
	if err != nil {
		return runref.Reference{}, err
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return runref.Reference{}, err
	}
	// THE PARENT'S COPY OF THE WRITE END, CLOSED NOW, so the read below ends when the
	// child exits rather than waiting on a handle this process is holding.
	pw.Close()
	ref := referenceFrom(pr, out)
	pr.Close()
	// THE EXIT CODE IS NOT THE OUTCOME, and is deliberately not consulted.
	//
	// `perform-workflow` exits 1 for a run that did not complete and 2 for a usage
	// refusal, and a run that refused every action exits 1 while having behaved exactly
	// as it should. The reference carries the answer.
	//
	// AND AN EXIT CODE IS NOT AN ERROR HERE EITHER. The error this function returns means
	// one thing — the runner could not be run at all — and that is what `Start` reports.
	// Returning `Wait`'s ExitError as well would make a usage refusal, which is this
	// front end asking for something the runner does not understand, come back as
	// `unavailable`: "nobody took it, try something else". Somebody did take it and said
	// no. A run that printed no reference comes back with an empty one, and the caller
	// reports that for what it is.
	if err := cmd.Wait(); err != nil {
		mlog.Info("intake: the supervised runner exited unhappily",
			"err", err, "run", ref.ID)
	}
	return ref, nil
}

// referenceFrom copies a transcript through and picks the run reference out of it.
//
// Separate from the spawn, and pure, because this is the part that can be wrong in a way nobody
// would see: a transcript that still reaches the person, with the one line that says how it ended
// quietly unread. Spawning is visible in a process list; misreading a line is visible nowhere.
//
// The LAST reference wins, because the runner prints one before anything happens — so a run that
// dies early is still citable — and prints it again with the outcome at the end. A later line
// carrying no outcome must not erase an outcome that has already arrived.
func referenceFrom(r io.Reader, out io.Writer) runref.Reference {
	var ref runref.Reference
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(out, line)
		got, ok := runref.Parse(line)
		if !ok {
			continue
		}
		if ref.ID == "" || got.Outcome != "" {
			ref = got
		}
	}
	return ref
}
