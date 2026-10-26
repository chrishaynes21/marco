package main

// SELECT A SAVED ROUTINE, AND PRESS PLAY. The other half of Record → Stop → Review → Save.
//
// # What this closes
//
// Saving worked and the flow stopped there. A routine recorded in this window went to disk and
// the only way to run it was to leave: open a terminal, find the right directory, type
// `marco do "ark graphics"`. That proves a Studio-recording-to-CLI path and it is not the
// product — and the Act surface next door does not establish it either, because an Act is a
// different thing made a different way: composed from a demonstration Director watched, pinned to
// a revision, performed by the supervised runner under an authorisation.
//
// A recorder-first routine is none of that. It is a `.marco` file in the library, it presses what
// somebody showed it, and the thing that performs it is `marco do`. So this surface is the
// library and one button.
//
// # Why pressing Play spawns `marco do` rather than running it here
//
// Because `marco do` is the entrance, and everything a run of a routine needs is already behind
// it: the authority door, the one cross-process stop, the guarded contract's account of what did
// not happen, the seven-word outcome, and the exit code that carries it. Studio must not acquire
// an execution loop of its own — a second place that decides what a routine's result was is a
// second answer to "did that work", and the whole of ADR-251 is about not having two of those.
//
// So the window spawns the command, raises the application first because only the foreground
// process may (see play.go), and reads the result off the published wire line. That is the same
// arrangement `plugins/overlay` already has with the same binary, through the same shared
// `internal/outcome`, which is why reading the child's line here is protocol rather than
// transcript-parsing: the words are a closed set and the package that owns them is imported by
// both ends.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
)

// kindAct and kindRoutine are what the one run slot is holding, which the page has to know
// because the two kinds of run can say different amounts about themselves.
//
// An Act's run is a supervised performance: it cites a canonical reference, publishes its own
// state, can pause and can be asked a question. A routine's run is a program pressing things: it
// has a transcript and an answer and nothing in between. A window that rendered the second as the
// first would report "the run cited no reference, so there is no canonical account of it" about a
// routine that behaved perfectly.
const (
	kindAct     = "act"
	kindRoutine = "routine"
)

// Routine is one saved recorder-first routine, as a person choosing one has to see it.
type Routine struct {
	// ID is the route's WHOLE identity, and it is the term the page hands back when
	// somebody chooses this routine or presses Play on it.
	//
	// # Why a name is not enough, and never was
	//
	// Because a name is not unique and a slug is not either. A route IS its scope and
	// its slug — `settings` saved for two applications is two routines, and so is
	// `settings` saved twice for one application under two scopes. Everything here used
	// to travel by name: the page kept `r.name`, looking a routine up returned the FIRST
	// row that matched, and `PlayRoutine` matched a name or a slug and took the first of
	// those. So clicking the second `settings` card displayed the first one's source and
	// its contract, and pressing Play performed the first one's route.
	//
	// Handing the child the route's full identity was already right and was happening
	// AFTER the wrong routine had been chosen, which is the shape of defect this project
	// keeps finding: a check that is correct about the wrong subject.
	//
	// It is the route's place in the library, which is what a route's identity is —
	// `global/<slug>`, `<app>/context/<slug>`, `<app>/focus/<slug>`. Readable on
	// purpose: when two of these disagree they say WHICH part differs, to anybody
	// reading them side by side.
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	App  string `json:"app,omitempty"`
	// Scope is where it works, in the product's own three words.
	Scope string `json:"scope"`
	// Focus is that as the registry holds it, and is how this is found again when
	// somebody presses Play on it. Never rendered: the scope above is the readable form,
	// and a page showing both would be showing one fact twice.
	Focus bool `json:"-"`
	// Contract is which of the three things this routine says about its own presses —
	// legacy, guarded or incomplete. The BASIS is the contract and everything else is a
	// measurement, so this is a state the saved file is IN. See ADR-251.
	Contract string `json:"contract"`
	// Checks is that in a sentence, which is the part a person needs: "every press is
	// checked against the window it was recorded in" is the promise, and "it does not
	// say what scale that window was drawn at, so it will refuse rather than press" is
	// the one case where reading it beforehand saves somebody a minute of refusals.
	Checks string `json:"checks,omitempty"`
	// Mode is what the saved source STATES it was recorded as, and Captures is that in a
	// sentence. Both absent when it says nothing.
	//
	// Absent, not a guess. Every routine saved before this says nothing, and so does one
	// recorded by the overlay or by `marco learn` — and what those captured depended on
	// the environment of a process that has long since exited. A card that filled the
	// blank in with the likeliest answer would be telling somebody how their routine was
	// made on the strength of an average. See orchestrator.ModeOf.
	Mode     string `json:"mode,omitempty"`
	Captures string `json:"captures,omitempty"`
	// Source is the saved Marco. Shown on request, always available: a routine is legal
	// Marco so that somebody can read what they kept and recognise it — the same reason
	// an Act shows its play.
	Source  string `json:"source,omitempty"`
	SavedAt string `json:"saved_at,omitempty"`
	Path    string `json:"path,omitempty"`
	// Runnable is whether it would run AS IT STANDS, and Why the refusal when it would
	// not. Asked now rather than remembered: the file may have been edited by hand, and a
	// routine listed as ready that cannot run is the state this product keeps finding.
	Runnable bool   `json:"runnable"`
	Why      string `json:"why,omitempty"`
}

// routineRun is this window's account of one performance of a routine.
//
// Small on purpose. Everything in it either came off the child's own wire line or is this
// window's bookkeeping about a process it started, and there is nothing in between for it to
// conclude.
type routineRun struct {
	name, slug, app string
	// said is the outcome word the child announced, empty until it does. One of
	// `internal/outcome`'s seven, read by the package that owns them.
	said string
	// why is the last thing the child said on its way out, which for a refused guarded
	// press is the sentence naming what could not be checked. Evidence, kept verbatim.
	why string
	// reference is the run's own reference, taken off the citation the child printed
	// before it sent anything.
	//
	// READ, NEVER MINTED. The executor claims it and prints it; this window would be a
	// second author of the same id, and a run somebody started from a terminal would have
	// one this had never heard of. It is what makes `Report` reachable for a routine.
	reference string
}

// Routines is every saved routine in the library, with whether each one would run.
//
// NOT the Acts: those are listed by `Saved`, they are composed rather than recorded, and they are
// performed by a different thing. A play with bindings is an Act and is skipped here for exactly
// the reason `Saved` skips a play without them — fail-closed in both directions, so nothing
// appears in two lists and nothing appears in neither.
func (a *Acts) Routines() []Routine {
	out := []Routine{}
	// THE ONES THAT CANNOT PRESS ANYTHING, asked once rather than per routine: it is a
	// listing of its own, and a question per route would read the whole library again for
	// every row of it.
	// KEYED BY THE WHOLE IDENTITY AND NOT BY THE SLUG.
	//
	// Every routine in that listing is context-scoped by construction, and a slug is
	// shared by every scope it was ever saved under — so `<app>/context/settings` being
	// unable to press anything marked `<app>/focus/settings` unable to press anything
	// too. A valid routine disabled by an invalid namesake, with the namesake's sentence
	// beside it explaining a scope it is not in.
	cannot := map[string]string{}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	for _, u := range d.Unrunnable() {
		cannot[routeID(u.Route)] = "every press in it checks the window it was made " +
			"in, and it was saved to switch nothing — so there is no window for " +
			"those presses to be checked against and every one of them would " +
			"refuse. Save it again from its own demonstration"
	}
	for _, rt := range a.reg().List() {
		if _, _, err := demonstrated.Load(a.reg(), rt); err == nil {
			continue // AN ACT. It is in the other list, performed by the other thing.
		}
		path := a.reg().Path(rt)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		src := string(raw)
		if !strings.Contains(src, "is a script") {
			// NOT SOMETHING ANYBODY CAN RUN. The act surface Marco seeds into the
			// library (`os.marco`) is a module of declarations: it is a route by
			// the registry's reckoning, it is legal Marco, and pressing Play on it
			// would perform nothing. A routine has an entrance.
			continue
		}
		// WHAT THEY CALLED IT, through the one function that reads the header
		// codegen writes. A second reader of that line would be a second answer
		// to what somebody called their own routine.
		r := Routine{ID: routeID(rt), Name: orchestrator.NameIn(src, rt.Slug),
			Slug: rt.Slug, App: rt.App, Focus: rt.Focus, Scope: scopeWord(rt),
			Source: src, Path: path}
		if st, serr := os.Stat(path); serr == nil {
			r.SavedAt = st.ModTime().Local().Format("2006-01-02 15:04")
		}
		contract, lacks := orchestrator.ContractOf(src)
		r.Contract, r.Checks = string(contract), checksSentence(contract, lacks)
		// AND HOW IT WAS RECORDED, WHEN THE FILE SAYS. Off the same source as the
		// contract, by the one reader of that line, and left empty when there is no
		// line — see Routine.Mode.
		if m := orchestrator.ModeOf(src); m != orchestrator.Unstated {
			r.Mode, r.Captures = string(m), capturesSentence(m)
		}
		switch {
		case cannot[r.ID] != "":
			r.Why = cannot[r.ID]
		case contract == orchestrator.Incomplete:
			// IT SAVES AND THEN REFUSES, which is the contract working rather
			// than a broken file — and it is still not something to press Play on
			// and watch. The sentence says what it lacks; see ADR-251.
			r.Why = r.Checks
		default:
			// AND IS IT STILL LEGAL MARCO. Asked through the production checker,
			// because a routine is a file somebody can open and edit and the
			// answer is about today.
			if cerr := driver.CheckSource(src); cerr != nil {
				r.Why = "it is no longer a program Marco can run: " +
					cerr.Error()
			} else {
				r.Runnable = true
			}
		}
		out = append(out, r)
	}
	// BY NAME, AND THEN BY THE REST OF THE IDENTITY, so the order is total.
	//
	// `sort.Slice` is not stable and two routines can share a name, so by name alone the
	// order of those two was whatever the sort happened to leave — and the page keys a
	// redraw on this list's order, so a pair that swapped between two polls would rebuild
	// the cards under somebody's cursor.
	//
	// Not a defect anybody has seen, and it cannot be made to happen here: `Registry.List`
	// already returns a total order, so the input arrives sorted and the sort has no equal
	// keys to shuffle. That is the point — this says the order does not DEPEND on an
	// upstream sort being total, which is not this function's guarantee to borrow.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// routeID is a route's whole identity as one string: its place in the library.
//
// The same three scopes `internal/routes` keeps on disk, because that IS what distinguishes two
// routes of one name — see Routine.ID. Not the file path: `Registry.Path` resolves a legacy
// context route to wherever it actually is, which makes it an answer about the filesystem rather
// than about the route, and an identity that changes when a file moves is not one.
func routeID(rt routes.Route) string {
	if rt.App == "" {
		return routes.GlobalDir + "/" + rt.Slug
	}
	scope := routes.ContextDir
	if rt.Focus {
		scope = routes.FocusDir
	}
	return rt.App + "/" + scope + "/" + rt.Slug
}

// scopeWord is where a routine works, in the three words the product uses for it.
func scopeWord(rt routes.Route) string {
	switch {
	case rt.App == "":
		return string(orchestrator.ScopeEverywhere)
	case rt.Focus:
		return string(orchestrator.ScopeAppAnywhere)
	default:
		return string(orchestrator.ScopeApp)
	}
}

// checksSentence says what a routine's contract means for pressing Play on it.
//
// Three sentences for three states, and the middle one is the only promise: a legacy routine
// presses where it was recorded having checked nothing, which is how every routine saved before
// any of this behaves and is not a fault to report — it is what somebody chose when they saved
// it. See ADR-251.
func checksSentence(c orchestrator.Contract, lacks []string) string {
	switch c {
	case orchestrator.Guarded:
		return "Every press is checked against the window it was recorded in — the " +
			"same program, the same size, the same scale — and the routine stops " +
			"at the first press that cannot be checked."
	case orchestrator.Incomplete:
		return "A press in this routine asks to be checked against the window it was " +
			"made in and the recording is missing " + andAll(saidTerms(lacks)) +
			", so it will refuse rather than press somewhere nobody checked. " +
			"Record it again."
	default:
		return "Its presses are not checked: each one happens at the point it was " +
			"recorded at, whatever is there now. This is how every routine saved " +
			"before Marco measured windows behaves."
	}
}

// saidTerms is the measurements a person can read, in the order they were missed.
//
// The saved Marco spells them `WinW` and `Dpi`, which are terms in a file and not things anybody
// says. The sentences are the same ones the review says at Save, because a person who reads "it
// does not say what scale that window was drawn at" twice has read one fact twice rather than two
// facts once.
func saidTerms(lacks []string) []string {
	says := map[string]string{
		"RelX":    "where across that window the press went",
		"RelY":    "where down that window the press went",
		"WinW":    "how wide that window was",
		"WinH":    "how tall that window was",
		"Dpi":     "what scale it was drawn at",
		"Process": "which program owned it",
	}
	out := make([]string, 0, len(lacks))
	for _, l := range lacks {
		if said, known := says[l]; known {
			out = append(out, said)
			continue
		}
		out = append(out, l)
	}
	return out
}

// ── PRESS PLAY ON A ROUTINE ───────────────────────────────────────────────────

// PlayRoutine performs a saved routine, by the name this window listed it under.
//
// Returns "" when a run started, or the sentence to show the person. Every refusal happens BEFORE
// the application is raised, for the same reason it does in `Play`: a window that came forward and
// then said no is worse than a sentence in the surface they were already looking at.
//
// IT PRESSES FOR REAL. There is no live/dry switch here because `marco do` has none — a dry run of
// a routine is `marco run --host dryrun`, which is a different command performing a file rather
// than the product performing a routine. The button says so, and the page says so beside it:
// offering a tick-box this could not honour would be worse than either.
func (a *Acts) PlayRoutine(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "choose a routine to run"
	}
	// EXACTLY THE ONE THAT WAS CHOSEN, by its whole identity.
	//
	// This matched a name OR a slug and took the first that answered, which for two
	// routines called `settings` is whichever the listing happened to put first — so the
	// window could display one and perform the other, and everything downstream was
	// correct about the wrong route. Case-sensitive on purpose: the identity is a term
	// this surface produced, not something anybody types, and folding it would be
	// inventing a second routine that matches.
	var chosen *Routine
	for _, r := range a.Routines() {
		if r.ID == id {
			got := r
			chosen = &got
			break
		}
	}
	switch {
	case chosen == nil:
		return fmt.Sprintf("there is no routine %q in this library (%s) — press "+
			"Refresh and choose it again", id, a.routesDir)
	case !chosen.Runnable:
		// ITS OWN SENTENCE. "It would not run" is not something anybody can act on;
		// what it lacks, or what to do about it, is.
		return "this routine cannot be performed as it stands: " + chosen.Why
	}
	// AND THE APPLICATION HAS TO BE IN FRONT, WHICH ONLY THIS WINDOW CAN ARRANGE.
	//
	// Windows refuses `SetForegroundWindow` to a process that does not already own the
	// foreground, and the child this is about to start is a process nobody is looking at.
	// Studio owns the foreground at this instant — somebody just clicked a button in it —
	// so this is the one moment the window may hand it on. The same split `Play` draws for
	// an Act, through the same function, and nothing here simulates a keypress to get
	// around the lock.
	//
	// For both scopes that name an application: one brings it forward itself and needs the
	// foreground to do it, the other works only while it is already there.
	if chosen.App != "" {
		if why := a.raise(chosen.App); why != "" {
			return why
		}
	}
	return a.startRoutine(*chosen)
}

// doCommand is the command that performs this routine: the binary, its arguments and its
// environment.
//
// Apart from `startRoutine` so that what is handed to the child can be asserted without starting
// one. That is not a convenience: `marco do` performs real input, so the only test that could
// otherwise reach this would be one nobody may run — and the two things worth getting right here
// are both invisible from outside the process. Which ROUTE it was told to perform, and which
// LIBRARY it was told to read.
func (a *Acts) doCommand(r Routine) (bin string, args, env []string) {
	// THE ROUTE ITSELF, NOT A PHRASE THAT MIGHT RESOLVE TO IT.
	//
	// `--play` carries the identity and the entrance never looks a phrase up again when it
	// has one, which matters here: a routine called "sound settings" and a demonstration of
	// the same name are two things, and a window that handed over words would be asking the
	// resolver to guess which one somebody clicked on.
	//
	// All three parts of the identity, because a route IS its scope and its slug: the same
	// name can be saved under two scopes, and `--play` alone resolves to whichever the
	// registry finds first.
	args = []string{"do", "--play=" + r.Slug}
	if r.App != "" {
		args = append(args, "--app="+r.App)
	}
	if r.Focus {
		args = append(args, "--focus")
	}
	// THE SAME STORE AND THE SAME LIBRARY, EXPLICITLY.
	//
	// The library especially. Both processes resolve it the same way now, which is the whole
	// of the fix that made this surface possible — and a child inheriting a different
	// working directory used to be exactly how the two came to disagree. Said out loud
	// rather than relied upon: this window knows which library it listed the routine FROM,
	// and that is the one the run must read.
	return marcoBin(), args,
		append(os.Environ(), "MARCO_HOME="+a.home, "MARCO_ROUTES="+a.routesDir)
}

// startRoutine claims the one run slot and spawns `marco do` for this routine.
func (a *Acts) startRoutine(r Routine) string {
	if why := a.reserve(kindRoutine); why != "" {
		return why
	}
	a.mu.Lock()
	// `live` IS TRUE AND IS NOT A CHOICE. See PlayRoutine.
	a.act, a.live = r.Name, true
	a.routine = &routineRun{name: r.Name, slug: r.Slug, app: r.App}
	a.mu.Unlock()
	release := func() {
		a.mu.Lock()
		a.busy, a.running, a.stopRun, a.done = false, nil, nil, nil
		a.mu.Unlock()
	}
	bin, args, env := a.doCommand(r)
	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, args...)
	// AND NO CONSOLE WINDOW, which would take the foreground off the application this run
	// is about to press something in — after this window went to the trouble of putting it
	// there. See hideConsole.
	hideConsole(cmd)
	cmd.Env = env
	pr, pw, err := os.Pipe()
	if err != nil {
		stop()
		release()
		return err.Error()
	}
	// ONE TRANSCRIPT, IN THE ORDER A PERSON AT A TERMINAL WOULD SEE IT. The command says
	// some things on stderr — a route that could not be read, a press that did not happen —
	// and every one of them is part of the account of what happened.
	cmd.Stdout, cmd.Stderr = pw, pw

	// STARTING AND PUBLISHING ARE ONE ATOMIC STEP, with the closing check inside it, for the
	// reason spelled out in `start`: a shutdown that returned while a launch was between its
	// reservation and its start left a child nobody would stop.
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
	a.running, a.stopRun, a.done = cmd, stop, finished
	a.mu.Unlock()
	pw.Close()
	a.trial.pressing(r.Name, r.App, r.Contract)

	go func() { a.readRoutine(pr); close(drained) }()
	go func() {
		_ = cmd.Wait()
		// EXITED IS NOT FINISHED. A child that has gone with output still in the pipe
		// is a run whose answer is still arriving — and the answer is the last thing
		// it prints.
		<-drained
		stop()
		a.mu.Lock()
		said, why := "", ""
		if a.routine != nil {
			said, why = a.routine.said, a.routine.why
		}
		a.mu.Unlock()
		// AND THE ANSWER IS WRITTEN DOWN BEFORE THE SLOT IS FREE, so a reader
		// watching for the run to end cannot race the evidence of how it went.
		a.trial.pressed(r.Name, said, why)
		release()
		close(finished)
	}()
	return ""
}

// readRoutine takes the child's lines, keeps them, and reads the answer out of them.
//
// The answer comes off `internal/outcome`'s own wire line, through the package that owns the seven
// words — not off a sentence this window recognised. A surface that matched on wording would be a
// surface whose verdict changed when somebody improved a message.
func (a *Acts) readRoutine(pr *os.File) {
	defer pr.Close()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		a.mu.Lock()
		a.transcript = append(a.transcript, line)
		if a.routine != nil {
			if o, ok := outcome.FromLine(line); ok {
				a.routine.said = string(o)
			} else if r, cited := runref.Parse(line); cited {
				// THE RUN'S OWN REFERENCE, printed before the first input.
				//
				// Before the branch below on purpose: the citation is the
				// last thing a child says before it starts pressing, so a
				// window that let it fall through would keep a file path as
				// the sentence explaining what went wrong.
				//
				// Only the id is taken. The path is the executor's and the
				// outcome arrives on its own wire line, through the package
				// that owns a routine's seven words — a routine's word is
				// not one of the supervised seven, and this parser only
				// recognises those.
				a.routine.reference = r.ID
			} else if said := strings.TrimSpace(line); said != "" &&
				!strings.HasPrefix(said, outcome.RoutePrefix) {
				// THE LAST THING IT SAID BEFORE ANNOUNCING, which for a
				// refused guarded press is the sentence naming what could not
				// be checked: `<routine> stopped: the window …`. Kept verbatim
				// and never parsed — this window does not decide what a
				// refusal means, it shows what the run said about it.
				a.routine.why = said
			}
		}
		a.mu.Unlock()
	}
}

// watchRoutine is a routine's run as it stands, in the same shape an Act's run is shown in.
//
// One run view for both kinds, because a second one would be a second place that renders a
// transcript and an outcome — and they would drift. What differs is said apart rather than shaped
// apart: `Routine` tells the page there is no reference, no phase and no question to expect, and
// `Said` is built from the seven words rather than from an Act's verdicts.
func (a *Acts) watchRoutine() Running {
	a.mu.Lock()
	defer a.mu.Unlock()
	w := Running{Act: a.act, Live: a.live, Routine: true,
		Transcript: append([]string(nil), a.transcript...), Active: a.busy}
	if a.routine == nil {
		return w
	}
	w.Outcome, w.Reason = a.routine.said, a.routine.why
	w.Reference = a.routine.reference
	w.Complete = a.routine.said != ""
	// A RUN NOBODY CAN SPEAK FOR. The slot is free and it never announced one of the seven
	// words — killed, crashed, never started, or a binary that is not the one this expected.
	// Reported apart from a failure, because a routine that refused SAYS it refused.
	//
	// The same rule the other kind of run uses, and asked of the SLOT rather than of a flag
	// of its own: a launch that never reached a child gives the slot back without anything
	// having ended, and a flag set where the child exits would have left that case claiming
	// a run was still in flight.
	if !w.Active && !w.Complete {
		w.Incomplete = true
	}
	w.Said = saidOfRoutine(a.routine.name, w.Outcome, w.Reason, w.Active)
	w.Doing = doingRoutine(a.routine, w.Active)
	return w
}

// saidOfRoutine is how a run of a routine went, in a sentence.
//
// The outcome word is shown beside it and is not a substitute for it: `unverified` is a word
// somebody has to be taught, and a person who pressed Play on their own routine should not have to
// learn a vocabulary to find out what happened to it. The same reason `said.go` exists for an
// Act's verdicts; these are the other seven words.
//
// AND `performed` DOES NOT CLAIM THE APPLICATION DID ANYTHING. A routine presses what it was
// shown; nothing on this path reads the screen afterwards, and a sentence that said "it worked"
// would be this window asserting the one thing it cannot see.
func saidOfRoutine(name, said, why string, active bool) string {
	if active {
		return ""
	}
	switch outcome.Outcome(said) {
	case outcome.Performed:
		return fmt.Sprintf("Marco performed %q — every press happened. Nothing here "+
			"checked that the application did what you wanted.", name)
	case outcome.Refused:
		return fmt.Sprintf("Marco refused to perform %q, and pressed nothing.%s",
			name, alsoSaid(why))
	case outcome.Cancelled:
		// AND WHATEVER THE STOP LEFT BEHIND, which a cancelled run used to drop.
		//
		// A stop is not a failure and this sentence does not pretend otherwise. But a
		// run stopped mid-chord can leave a modifier held down on the machine, and the
		// executor now keeps that sentence in the record of a cancelled run — so a
		// window that printed only "stopped part of the way through" would be the one
		// place a person looks, not mentioning the key that is still down.
		return fmt.Sprintf("%q was stopped part of the way through.%s", name,
			alsoSaid(why))
	case outcome.Unavailable:
		return fmt.Sprintf("Nothing ran %q — it was never delivered.%s", name,
			alsoSaid(why))
	case outcome.Failed:
		return fmt.Sprintf("%q stopped, and nothing after the press that did not "+
			"happen was done.%s", name, alsoSaid(why))
	case outcome.Unverified:
		return fmt.Sprintf("Marco delivered %q and cannot confirm it arrived.%s",
			name, alsoSaid(why))
	case outcome.Clarify:
		return fmt.Sprintf("%q is waiting to be told something.%s", name,
			alsoSaid(why))
	case "":
		return fmt.Sprintf("%q stopped without saying how it ended, so nobody can "+
			"speak for what it did.", name)
	}
	// A RECORD THAT ONLY SAYS THE ATTEMPT BEGAN, which is reachable the moment a
	// routine's run leaves a record: the reference is claimed and a provisional record
	// written before anything is sent, so a `marco` killed between the two leaves exactly
	// this. See runs.Unfinished — a record state and not one of the seven.
	//
	// Said as itself rather than falling through to "ended as started", which reads as a
	// word somebody misspelled.
	if said == runs.Unfinished {
		return fmt.Sprintf("Marco started %q and never said how it ended, so nobody "+
			"can speak for what it did.%s", name, alsoSaid(why))
	}
	return fmt.Sprintf("%q ended as %s.%s", name, said, alsoSaid(why))
}

// alsoSaid is the run's own last sentence, when it said one.
//
// BESIDE THE VERDICT RATHER THAN INSTEAD OF IT, which is the arrangement an Act's pause already
// has for the same reason: the word is what happened and this is why, and a window showing only
// the word is showing a tally. For a refused guarded press this sentence names the press that
// could not be checked and what was different about the window — which is the whole finding, and
// the reason ADR-251 insists the run's answer is data rather than a line of log somebody has to
// go and read.
//
// It was carried as far as this function and then dropped: the parameter was here, unused, and
// the page rendered the outcome word and the transcript with nothing in between. A person reading
// "it stopped" with the explanation four scrolls down in Technical details is a person being
// handed a tally.
func alsoSaid(why string) string {
	if strings.TrimSpace(why) == "" {
		return ""
	}
	return " It said: " + why
}

// doingRoutine is what is happening now, while it is.
func doingRoutine(r *routineRun, active bool) string {
	if !active {
		return ""
	}
	if r.app != "" {
		return fmt.Sprintf("Performing %q in %s…", r.name, r.app)
	}
	return fmt.Sprintf("Performing %q…", r.name)
}

// ── WHERE THE LIBRARY IS, AND WHETHER ANYBODY'S IS SOMEWHERE ELSE ─────────────

// Library is which directory this window's routines live in, and a sentence when there is another
// one nearby that nobody is reading.
type Library struct {
	Dir string `json:"dir"`
	// Elsewhere is a library-shaped directory beside wherever this was launched from that
	// is NOT the one being read, and Says is that in words.
	//
	// It exists because the default moved. Every library saved under the old rule sits in a
	// `routes` folder beside whichever directory a binary was started from, and the honest
	// failure of changing the rule is that such a folder goes quiet: nothing is lost,
	// nothing says so, and a person reads an empty list and concludes their work is gone.
	// So the thing that went quiet says its name. Nothing is moved — adopting a directory
	// somebody did not name would be this window deciding which of two libraries is theirs.
	Elsewhere string `json:"elsewhere,omitempty"`
	Says      string `json:"says,omitempty"`
}

// Library is where this window reads and writes routines.
func (a *Acts) Library() Library {
	out := Library{Dir: a.routesDir}
	// WHERE THIS WAS LAUNCHED FROM, AND WHERE IT LIVES, which are not the same place for a
	// program somebody double-clicked: Explorer hands it the folder the executable sits in,
	// and a Studio in `plugins\studio` has no library beside it either way. Both, searched
	// upwards — see marcopaths.Elsewhere.
	here, _ := os.Getwd()
	beside := ""
	if exe, err := os.Executable(); err == nil {
		beside = filepath.Dir(exe)
	}
	if dir, n := marcopaths.Elsewhere(here, beside); n > 0 {
		out.Elsewhere = dir
		out.Says = fmt.Sprintf("%d routine(s) are in %s, which is not the library "+
			"above. Routines used to be kept beside whichever directory a binary "+
			"was started from, and that is a library from then — nothing has been "+
			"lost and nothing has been moved. Copy them into the library, or set "+
			"MARCO_ROUTES to that directory and start Marco again.", n, dir)
	}
	return out
}

// ── THE ONE RUN SLOT ──────────────────────────────────────────────────────────

// reserve claims the one run slot, for whichever kind of run is asking.
//
// Returns "" when it is claimed, or the sentence to show the person. THE SLOT IS CLAIMED UNDER THE
// SAME LOCK THAT TESTED IT — see `busy`, which carries the account of the race that is about.
//
// # Why both kinds of run share it
//
// Because there is one screen and one person in front of it. An Act's run asks for presses at a
// consent desk reading a global keyboard hook; a routine's run presses things. Two of either, or
// one of each, is two things pressing on one desktop — and the second of them would be recorded by
// the first if a recording were also running, which is the third thing this refuses.
//
// It had one caller and the refusals were written inside it. A second kind of run arriving beside
// it would have had to either repeat the three questions or ask none of them, and the second is
// not a thing anybody notices until two runs are in flight.
func (a *Acts) reserve(kind string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.closing:
		return "Studio is closing, so nothing new is started"
	case a.busy:
		return "a run is already in flight; one at a time, because the consent desk " +
			"is a person's attention and there is one of those"
	case a.recordingLocked():
		// AND NOT WHILE SOMETHING IS BEING RECORDED. One screen: a recorder watching
		// it while a runner presses on it would be Marco demonstrating to itself, and
		// the artifact would record Marco's own clicks as the person's.
		//
		// Asked of BOTH recorders through one predicate. It used to name `a.learn`,
		// which meant the recorder-first path arriving beside it was invisible to this
		// refusal — and that one installs hooks in THIS process, so a run started
		// under it would have put the runner's own presses into somebody's draft. See
		// recordingLocked.
		return "Marco is recording a demonstration; one thing at a time on one screen"
	}
	a.busy, a.kind = true, kind
	a.transcript, a.reference, a.outcome = nil, "", ""
	a.act, a.live, a.saying = "", false, ""
	a.routine = nil
	return ""
}

// marcoBin finds the marco executable the same way `directorBin` finds the director's: beside the
// running binary, with $MARCO_BIN overriding.
//
// BESIDE THIS BINARY, which is not a detail — it is the same trap the director sibling already
// carries a paragraph about in CLAUDE.md. With a copy of Studio in `plugins\studio` and another at
// the repo root, a `marco.exe` beside only one of them means a window that looks current
// performing routines through whatever was last left there. `build.cmd` makes all of them in one
// breath for that reason, and TestTheBuildScriptMakesTheBinariesStudioWillSpawn holds it.
func marcoBin() string {
	if b := strings.TrimSpace(os.Getenv("MARCO_BIN")); b != "" {
		return b
	}
	if exe, err := os.Executable(); err == nil {
		c := filepath.Join(filepath.Dir(exe), "marco.exe")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "marco.exe"
}

// awaitRun waits for the run in flight to finish, bounded, and says whether it did.
//
// For a test. Production waits on `done` through `Close`, which is the same channel.
func (a *Acts) awaitRun(d time.Duration) bool {
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// nextOfRoutine is what to do about a run of a routine that did not work, in its own terms.
//
// # Why this is not nextOf
//
// Because `nextOf` answers for an ACT, and its remedies are about demonstrations and evidence:
// *demonstrate it again, resting after the click so the result is read.* A routine has no
// evidence and no reading — it presses what it was shown — so every one of those sentences would
// be advice about machinery it does not have, offered to somebody whose routine did not run.
//
// The remedies here are the ones that exist for a routine: record it again, bring the window back
// to the shape it was shown in, or read what it said. And `performed` gets NOTHING, deliberately:
// there is nothing to do about a run where every press happened, and inventing a next step would
// imply something was wrong with it.
func nextOfRoutine(said string) string {
	switch outcome.Outcome(said) {
	case "", outcome.Performed:
		return ""
	case outcome.Refused:
		return "Nothing was pressed. The sentence above is the run's own reason — a " +
			"press checked against the window it was recorded in refuses when that " +
			"window is a different size or a different program, so put it back the " +
			"way it was when you recorded it and press Play again."
	case outcome.Cancelled:
		return "Nothing was sent after you pressed Stop."
	case outcome.Unavailable:
		return "Nothing performed it at all, so this is not about the routine. Check " +
			"that the marco beside Studio is the one you built."
	case outcome.Failed:
		return "It stopped at the first thing that did not happen, and nothing after " +
			"that was done. Technical details has what it said."
	case outcome.Unverified:
		return "It was delivered and nothing could confirm it arrived."
	case outcome.Clarify:
		return "It is waiting to be told something, which a routine performed from a " +
			"window cannot be — record it again without the part that asks."
	}
	// A RECORD STATE RATHER THAN AN OUTCOME, and the one word both kinds share. See
	// runs.Unfinished: the attempt was claimed and never finished.
	if said == runs.Unfinished {
		return "The run went away without saying how it ended, so nobody can speak " +
			"for what it did. Technical details has the transcript."
	}
	return ""
}
