package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// The one intake. Every entrance arrives here, and nothing decides meaning anywhere else.
//
// # What used to happen instead
//
// A phrase TYPED into the overlay ran `marco do`, which fuzzed it to a slug and, on a miss,
// offered to record a demonstration. The same phrase SPOKEN ran `marco director`, which never
// consulted the play registry at all and reinterpreted the words against the screen. So a play
// the person had learned and could run by typing was not found by saying its name, and a request
// Director could have satisfied became "shall I learn this?" the moment it was typed. Two intakes,
// disjoint vocabularies, and which one you got depended on whether you had a microphone.
//
// # The rule
//
// If Marco already knows exactly what the Audience means, it uses the durable thing it knows.
// Otherwise Director works out what they mean from the live world. The decision itself lives in
// [[internal/invoke]] and is pure; this file is the part that has to talk to a filesystem, a
// service and a person.
//
// # Source is carried and never consulted
//
// `--source` records how intent arrived so an acceptance run can prove routing. Nothing here or in
// `invoke.Decide` reads it to decide anything. Typing and speaking are transport differences.

// Outcome is what happened, in the words every surface can render.
//
// # Why an exit code was not enough
//
// Because three of these used to be indistinguishable from success. A play the door DECLINED
// returned nil ("you said no" is not an error). A cancelled local play returned nil. A generated
// play whose Screen guard refused caught its own failure, logged it, and returned nil — so the
// process exited 0 while nothing had happened. The overlay had three words for all of it and
// picked them by exit code, so "Marco refused", "you cancelled" and "it worked" all rendered `ok`.
//
// # Why these are aliases now
//
// Because this file held the FOURTH copy of the set. [outcome] exists to end exactly that — its
// own note says "One package, imported by all three" — and the overlay's note recorded what was
// still outstanding: "What is STILL hand-matched is the PRODUCER: cmd/marco/intake.go prints the
// tag with a literal format string, in the root module, where nothing compares it to anything."
// It is no longer hand-matched. The words, their exit codes and the wire line now have one
// definition and a compiler owns the agreement, which is what made adding the seventh a one-line
// change here instead of a four-file one somebody could do three quarters of.
type Outcome = outcome.Outcome

// The names this package has always used, bound to the one set. Each doc comment lives on the
// constant in [outcome] now, so there is one place to read what a word means.
//
// OutcomePerformed in particular: nothing else may claim it — see [[ADR-070]], finishing is not
// arriving — and OutcomeUnverified is the word for the gap that claim leaves open.
const (
	OutcomePerformed   = outcome.Performed
	OutcomeClarify     = outcome.Clarify
	OutcomeRefused     = outcome.Refused
	OutcomeUnavailable = outcome.Unavailable
	OutcomeCancelled   = outcome.Cancelled
	OutcomeUnverified  = outcome.Unverified
	OutcomeFailed      = outcome.Failed
)

// Outcome.Exit, the wire prefix and the announcement all come from [outcome] now.
//
// What stood here was a second switch over the same words returning the same codes, and a second
// copy of the `[result] ` literal under a comment observing that nothing would fail to compile if
// either side drifted. The comment was accurate, which is the argument against keeping the
// arrangement it described rather than for documenting it.
//
// Changing the literal must fail TestTheOutcomeIsAnnouncedOnTheWire.
const resultPrefix = outcome.ResultPrefix

// announce publishes the outcome for whatever is reading this process's stdout.
func announce(o Outcome) { outcome.Announce(os.Stdout, o) }

// tracePrefix is the cross-surface routing trace, printed only when asked for.
//
// Off by default because it would otherwise land in the HUD log on every command. An acceptance
// run sets MARCO_TRACE_INTAKE=1 and gets one line per invocation saying source, decision, which
// play or phrase, and which rule fired. It keeps no store and writes no file: the words a person
// says are exactly the thing not to start keeping.
const tracePrefix = "[intake] "

func traceIntake(d invoke.Decision, req invoke.Request) {
	line := d.Trace(req)
	mlog.Info("intake: routed", "trace", line)
	if os.Getenv("MARCO_TRACE_INTAKE") != "" {
		fmt.Printf("%s%s\n", tracePrefix, line)
	}
}

// runInvocation is THE entry every entrance reaches.
//
// Deleting any caller's use of this — routing a surface's words anywhere else — must fail
// TestEveryEntranceRoutesThroughTheOneIntake.
func runInvocation(d orchestrator.Deps, req invoke.Request) (Outcome, error) {
	if req.App == "" {
		req.App = appOf(d)
	}
	// WHETHER DIRECTOR IS MID-QUESTION, asked before anything claims the words.
	//
	// Only when there is something to claim: an explicit identity is not words, and a surface
	// that already knows which play is not answering a question with it.
	if req.Play == nil {
		req.Pending = pendingQuestion()
	}
	// BOTH DURABLE HALVES, HANDED TO THE ONE DOOR TOGETHER.
	//
	// `d.Reg` is what Marco can RUN and `demoSights` is what the person SHOWED it. Two
	// stores, two performers, one decision — a front end that consulted them in turn
	// would be a second router, which is the defect `invoke` exists to have ended.
	//
	// Deleting the sights argument must fail TestADemonstratedRoutineIsReachedByItsName.
	dec := invoke.Decide(d.Reg, demoSights{dir: directorDir()}, req)
	traceIntake(dec, req)

	out, err := carryOut(d, dec, req)

	// ── AND WHAT BECAME OF IT, AT INFO, WHERE THE DECISION WAS MADE ──
	//
	// The log said what was asked for and never what happened. A person watching the
	// console behind the overlay saw their own words go in and then nothing: the outcome
	// reached the HUD's history row and the `[result]` wire line, both of which are
	// renderings for a surface, and neither of which is in the log a person reads when
	// they are trying to find out why something did not work.
	//
	// One line, at INFO, carrying the three facts that answer "why did that not work":
	// which half of the product took the words, what it ended as, and the error if there
	// was one. The decision line above says where the words went; this says where they
	// got to.
	kv := []any{"decision", string(dec.Kind), "outcome", string(out), "words", req.Text}
	switch dec.Kind {
	case invoke.KindPlay:
		kv = append(kv, "play", dec.Play.Slug)
	case invoke.KindDemonstration:
		kv = append(kv, "demonstration", dec.Demonstration.Name)
	}
	if err != nil {
		kv = append(kv, "err", err)
	}
	mlog.Info("intake: finished", kv...)
	return out, err
}

// carryOut hands the verdict to whichever half of the product performs it.
//
// Separated from `runInvocation` so that every one of the four arms is followed by the same
// report. Four `return`s with a log line before each is four places to forget it, and the one
// that would get forgotten is whichever arm is added next.
func carryOut(d orchestrator.Deps, dec invoke.Decision, req invoke.Request) (Outcome, error) {
	switch dec.Kind {
	case invoke.KindControl:
		return runControl(), nil
	case invoke.KindPlay:
		return performDecidedPlay(d, dec, req)
	case invoke.KindDemonstration:
		return performDemonstration(dec, req)
	default:
		return askDirector(d, dec, req)
	}
}

// directorPending reports whether Director is holding an unanswered question.
//
// # Why this is asked, and why it is cheap
//
// Because "the second one" is an answer, not a play, and a phrase that happens to name a play is
// STILL an answer while an answer is what was asked for. Nothing else can know that: the question
// lives in the service.
//
// It connects WITHOUT starting anything. A Director that is not running has no pending question,
// so an outage costs one failed dial rather than a twenty-second service start on every command —
// and the answer to "is a question pending" when nothing is asking is simply no.
//
// Deleting this must fail TestAnAnswerIsNotClaimedByAPlayOfTheSameName.
//
// `pendingQuestion` is a package variable for the same reason `dialPerformer` is: a test must be
// able to exercise the intake without a socket, a service, or any chance of reaching the running
// Director on the developer.s own desktop. Production never reassigns it.
var pendingQuestion = directorPending

func directorPending() bool {
	c, err := directorConnect(false)
	if err != nil {
		return false
	}
	defer c.Close()
	st, err := c.Status()
	if err != nil {
		return false
	}
	return st.Clarification != nil
}

// runControl is THE ONE STOP, and it has two arms because Marco drives the desktop from two
// processes.
//
// # Why one arm was not enough
//
//	(a) stopsignal.Raise   every marco Play running against this store, wherever it was started
//	(b) directorStop       the Director's own active command, and whatever it is holding open
//
// It used to be (b) alone. That reached a LEARNED play, because a learned play is performed inside
// the Director service — and reached nothing at all for every other kind, because those run in a
// short-lived `marco` child that nothing in the engine could name. A play typing into a text box
// could not be stopped by saying "stop": the word arrived, the Director said "nothing to stop, I
// am not running", and the typing carried on. The overlay papered over it by KILLING the child it
// had spawned, which stops the process without unwinding it — no `finally`, so a held key stays
// held — and did nothing whatsoever for a play started from a terminal or a hotkey.
//
// # Both arms run, always, and neither may skip the other
//
// There is no early return in this function on purpose. The two arms reach two different
// populations and a failure of one says nothing about the other: a disk that refused the write
// must not stop the Director being told, and a Director that is not running must not stop the
// local broadcast going out. Deliberately sequential and deliberately unconditional.
//
// Arm (a) goes first because it is the one holding the keyboard down.
//
// # What it reports, and why "unavailable" is nearly never right
//
// `unavailable` means NOBODY TOOK IT — the request was never delivered and a caller may sensibly
// try something else. A raised generation IS a delivery: it is published where every running Play
// is already looking, and whether one happened to be running is not knowable and not the point.
// Stop is a broadcast and broadcasts do not get receipts, so the ordinary case — Director off, a
// local Play stopped — is `cancelled`. Reporting `unavailable` there would tell the overlay that
// nothing had heard the word at the exact moment something had.
//
// So `unavailable` is reserved for the one honest case: the broadcast could not even be published
// AND the Director could not be reached either. Then, and only then, was the word not delivered
// anywhere.
//
// Deleting the stopsignal.Raise arm must fail TestOneStopReachesALocalPlayAndTheDirector.
func runControl() Outcome {
	// ARM (a) — every Play running against this store, in whatever process started it.
	home := stopsignal.Home()
	raiseErr := stopsignal.Raise(home)
	if raiseErr != nil {
		// Logged and carried, never returned early: arm (b) is still owed its turn.
		mlog.Error("intake: the stop could not be published to local plays",
			"home", home, "err", raiseErr)
	}

	// ARM (b) — the ACTIVE EXECUTION AUTHORITY. A front end may also have killed a child it
	// spawned for immediacy; that is a local courtesy and this is the part that is authoritative.
	code := stopWhatIsRunning(false, false)

	switch {
	case raiseErr == nil:
		// The word went out where every running Play is looking. That is a delivery.
		//
		// And it is said out loud, because `marco stop` is a NORMAL verb now and a person who
		// types it in a terminal deserves an answer. The sentence names no part of Marco's
		// insides: what used to reach them here was "nothing to stop — the Director is not
		// running", which was backstage AND wrong, since the broadcast had just gone out to
		// every Play on the machine at the moment it claimed nothing had.
		fmt.Println("stop sent — anything Marco was running will stop")
		return OutcomeCancelled
	case code == exitUnavailable:
		// Neither arm delivered anything. Nothing heard it, and saying so is the only honest
		// answer left.
		return OutcomeUnavailable
	default:
		// The broadcast failed but the Director was reached, so the word did land somewhere.
		return OutcomeCancelled
	}
}

// playInvocation is one durable behaviour plus the arguments this invocation gave it.
type playInvocation struct {
	rt         routes.Route
	named      map[string]string
	positional []string
	// flagged names the arguments whose values arrived as explicit flags rather than
	// inside a phrase. See routes.CheckFlagged: a secret-typed argument may not be
	// supplied that way.
	flagged []string
	// expect is the digest of the routine this invocation was authorised against, or ""
	// when nothing pinned a version. See invoke.Request.Expect.
	expect string
	// pinned is the digest the entrance read the routine at, unconfirmed. See
	// invoke.Request.Pinned.
	pinned string
}

// performDecidedPlay runs the play the decision chose, with whatever arguments the words carried.
func performDecidedPlay(d orchestrator.Deps, dec invoke.Decision, req invoke.Request) (Outcome, error) {
	// An EXPLICIT identity carries no words to parse. A clicked Run means that play; a
	// binding resolved to that play. Re-reading a display name for arguments would be the
	// same mistake as re-reading it for identity.
	// Arguments the entrance already peeled off win; otherwise they come out of the words.
	//
	// A hotkey binding carries its own ("`e = enter freeplay with 3"), and a clicked Run carries
	// none. Only a phrase has to be read for them — and re-reading a phrase that carried none,
	// over cargo that did, would silently drop arguments somebody supplied. Which is the exact
	// failure `TestALearnedPlayRefusesArgumentsRatherThanDroppingThem` exists to prevent.
	steps := []playInvocation{{rt: dec.Play, named: req.Named,
		positional: req.Positional, flagged: req.Flagged, expect: req.Expect,
		pinned: req.Pinned}}
	if !dec.Explicit && len(req.Named)+len(req.Positional) == 0 {
		_, named, positional := routes.ParseInvocation(strings.TrimSpace(req.Text))
		steps[0].named, steps[0].positional = named, positional
	}
	return performPlaySteps(d, steps)
}

// performPlaySteps runs each play in order, stopping at the first that does not perform.
func performPlaySteps(d orchestrator.Deps, steps []playInvocation) (Outcome, error) {
	for _, s := range steps {
		out, err := performOnePlay(d, s)
		if out != OutcomePerformed {
			return out, err
		}
	}
	return OutcomePerformed, nil
}

// grammar is the invocation grammar, applied only AFTER the whole phrase failed to name a play.
//
// # Why the order matters
//
// `SplitChain` and `ParseInvocation` used to run over the whole phrase before anything consulted
// the registry, so a play called "wait then click" was split into two commands neither of which
// existed, and "log in with google" lost its tail to argument parsing. A play's own name now wins
// against the grammar that would have taken it apart.
//
// A chain is only a chain when EVERY step names a play. "turn the lights on then make coffee" is
// one request for Director, not two offers to record a demonstration.
//
// Deleting the whole-phrase-first order must fail TestAPlayNamedWithTheGrammarIsStillReachable.
func grammar(d orchestrator.Deps, text string, app string) ([]playInvocation, bool) {
	if base, named, positional := routes.ParseInvocation(text); base != text {
		if rt, ok := d.Reg.Resolve(app, base); ok {
			return []playInvocation{{rt: rt, named: named, positional: positional}}, true
		}
	}
	parts := routes.SplitChain(text)
	if len(parts) < 2 {
		return nil, false
	}
	steps := make([]playInvocation, 0, len(parts))
	for _, p := range parts {
		base, named, positional := routes.ParseInvocation(p)
		rt, ok := d.Reg.Resolve(app, base)
		if !ok {
			// One unknown step and the whole phrase belongs to Director. Running the
			// known prefix and then asking about the rest would be half an answer to a
			// question nobody asked in halves.
			return nil, false
		}
		steps = append(steps, playInvocation{rt: rt, named: named, positional: positional})
	}
	return steps, true
}

// askDirector hands an unresolved request to the one operation that interprets one.
//
// Before it does, it gives the invocation grammar its turn: the whole phrase named no play, but
// "log in with google with alice, secret" might still name one with arguments, and "open notepad
// then save" might name two.
func askDirector(d orchestrator.Deps, dec invoke.Decision, req invoke.Request) (Outcome, error) {
	if steps, ok := grammar(d, dec.Phrase, req.App); ok {
		mlog.Info("intake: the invocation grammar named plays", "steps", len(steps))
		return performPlaySteps(d, steps)
	}
	if dec.Phrase == "" {
		return OutcomeFailed, nil
	}
	out := outcomeOfDirectorExit(submitPhrase(dec.Phrase, false))
	if out == OutcomeUnavailable {
		// NOBODY COULD TAKE IT. No play answers to these words and Director was never
		// reached, so this is the one case where the honest thing to say is that Marco has
		// never heard of it — and the one case where offering to learn it is not an insult.
		//
		// It is deliberately NOT said when Director ran and could not do it. That is a
		// different fact: Marco understood the request and failed at it, and answering
		// "shall I learn this?" would be an offer to record a demonstration of something the
		// person just watched go wrong.
		//
		// "no play matches " is the engine's one unknown-command prefix, shared with bind.go
		// and PREFIX-MATCHED by plugins/overlay (const noPlayMatches) to decide that this is
		// the error it should answer with an offer to learn. The `marco learn` mention is
		// part of that match and stays, verb and all.
		//
		// Deleting the shared spelling must fail TestTheUnknownCommandErrorIsOnePrefix.
		return out, fmt.Errorf("no play matches %q (learn it: marco learn %q, or `m learn … "+
			"in the overlay)", dec.Phrase, dec.Phrase)
	}
	return out, nil
}

// submitPhrase is how the intake reaches Director, and stopWhatIsRunning is how it cancels.
//
// Package variables for the same reason `dialPerformer` is one: the intake has to be exercisable
// end to end without a socket, a service, or any chance of starting — or reaching — the Director
// running on the developer.s own desktop. A test that dialled the real service would be slow,
// flaky, and capable of driving a real mouse. Production never reassigns them.
var (
	submitPhrase      = directorSubmit
	stopWhatIsRunning = directorStop
)

// outcomeOfDirectorExit reads what `marco director` reports back into the shared vocabulary.
//
// It is the inverse of exitOfCommandState, and the two are only correct together: a Director
// verdict that reaches a truthful exit code and is then read back as the wrong word is still a
// lie. TestTheDirectorsVerdictSurvivesTheCrossing walks every terminal state through BOTH halves
// for that reason, and TestNoTwoVerdictsShareAnExitCode is why its passing means anything —
// without it, two states could share a code and the inverse would be right about one of them.
//
// The default stays `failed` deliberately. An exit code this does not recognise came from a build
// that knows a word this one does not, and the only reading of an unknown code that is never a
// false claim of success is a failure.
func outcomeOfDirectorExit(code int) Outcome {
	switch code {
	case outcome.ExitPerformed:
		return OutcomePerformed
	case outcome.ExitUnavailable:
		return OutcomeUnavailable
	case outcome.ExitClarify:
		return OutcomeClarify
	case outcome.ExitRefused:
		return OutcomeRefused
	case outcome.ExitCancelled:
		return OutcomeCancelled
	case outcome.ExitUnverified:
		return OutcomeUnverified
	default:
		return OutcomeFailed
	}
}

// stopper wraps a run in the kill-switch that can abort it.
//
// A package variable so a test can cancel a run from underneath the way the stop key does, without
// installing a global keyboard hook. Production never reassigns it.
var stopper = withPanicStop

// performOnePlay takes one durable behaviour through the door and then performs it.
//
// # The door is here, and it is the same door
//
// Resolving which play the Audience means is not permission to perform it. Every invocation
// crosses `orchestrator.Authorize` before anything happens, whichever entrance it came from and
// whether the identity arrived as words or as an explicit choice. A clicked Run is still asked
// about. See [[ADR-029-resolution-is-not-permission]].
//
// # What is new here is only the ANSWER
//
// A declined play used to return nil, because "you said no" is not an error — and then the front
// end read exit 0 and rendered `ok`. It is a refusal, and now it says so.
//
// Deleting the Authorize call must fail TestExactPlayMatchDoesNotBypassAuthority.
func performOnePlay(d orchestrator.Deps, p playInvocation) (Outcome, error) {
	// AND THE VALUES IT WILL RUN WITH, COPIED ONTO THE THING BEING AUTHORISED.
	//
	// Everything below reads them off `resolved` rather than off `p`, which is what makes
	// "what runs is what was authorised" true rather than merely intended: a caller still
	// holding the map it passed in cannot change what gets typed after the door has
	// answered. See orchestrator.Resolved.WithValues.
	// ── AN ONLY-IN ROUTINE RUNS ONLY IN ITS APPLICATION, WHICHEVER ARM DECIDED IT ──
	//
	// Here, because this is the one place every performance of a routine passes: a phrase,
	// an explicit identity from Studio or the control centre, a binding, a confirmed offer.
	// The phrase arms already offer an in-place routine only while its application is the
	// context; an explicit identity was never checked, and its route carries no `Activate`
	// and no check of its own, so it ran into whatever was in front. The foreground is read
	// NOW, not carried from the intake, which for `--play --app` held the route's app rather
	// than a reading. A window the host validated for this run is the other way in — Studio's
	// handoff — and only when it belongs to the routine's application.
	//
	// Deleting this must fail TestAnExplicitOnlyInRoutineIsRefusedWithAnotherAppInFront.
	foreground := appOf(d)
	// ── AND A GUARDED ONE IS BOUND TO THE WINDOW IN FRONT, FROM THE SAME READING (S9) ──
	//
	// A press that checks its window needs a window BOUND to check it against, and an Only-in
	// routine has no Activate to bind one. So the window in front is read ONCE, here: the one
	// reading admits the routine and names the window bound, because a second reading would be
	// a second instant. Nothing is raised and nothing is searched for. The window in front
	// becomes this run's target, whichever window of the application it is; a different one
	// coming to the front afterwards is refused at the next press, which is where every
	// measurement still happens (oshost.confirmWindow). A window handed over by Studio is bound
	// already. A legacy routine attempts none of this and keeps its admission exactly.
	//
	// Deleting this must fail TestAGuardedOnlyInRunIsBoundToTheWindowInFront.
	var inFront *oshost.InFront
	if p.rt.App != "" && !p.rt.Focus && d.HandedTo == "" && checksItsWindow(d, p.rt) {
		if h, ok := d.Hosts["*"].(*oshost.Host); ok {
			w := h.ReadInFront()
			inFront = &w
			foreground = strings.TrimSuffix(strings.ToLower(w.Program), ".exe")
		}
	}
	if why := orchestrator.AdmitInPlace(p.rt, orchestrator.InPlace{
		HandedTo: d.HandedTo, Foreground: foreground}); why != "" {
		mlog.Info("intake: not admitted here", "play", p.rt.Slug, "why", why)
		fmt.Fprintln(os.Stdout, why)
		return OutcomeRefused, nil
	}
	if inFront != nil {
		if err := d.Hosts["*"].(*oshost.Host).BindInFront(*inFront); err != nil {
			mlog.Info("intake: the window in front could not be bound", "play", p.rt.Slug, "err", err)
			fmt.Fprintln(os.Stdout, err)
			return OutcomeRefused, nil
		}
	}
	resolved := orchestrator.Classify(d.Reg, p.rt, plays.Pretty(p.rt.Slug)).
		WithValues(p.named)
	decision := orchestrator.Authorize(resolved, d.Authority)
	mlog.Info("intake: authority", "play", p.rt.Slug,
		"verdict", string(decision.Verdict), "reason", decision.Reason)
	if !decision.Allow() {
		if decision.Sentence != "" {
			fmt.Fprintln(os.Stdout, decision.Sentence)
		}
		return OutcomeRefused, nil
	}
	// `[route] ` IS WIRE PROTOCOL, NOT USER TEXT — see the note on its other producer in
	// panicstop.go. It tells a front-end which play a loose phrase actually became.
	fmt.Printf("[route] %s\n", plays.Pretty(p.rt.Slug))

	// THE FORK. A play Marco wrote by watching is performed by the Director, which can see;
	// everything else runs here, exactly as it always has.
	if resolved.Learned() {
		if err := performLearned(d, resolved, resolved.Values, p.positional); err != nil {
			return OutcomeFailed, err
		}
		return OutcomePerformed, nil
	}

	// ── THIS ATTEMPT'S OWN REFERENCE, CLAIMED BEFORE THE FIRST INPUT ──
	//
	// Here and not in whatever surface pressed Play: the thing that sends the input is the
	// only thing present for every run of a routine, and a reference minted by a front end
	// would differ by who asked. See routinerecord.go.
	//
	// After the authority door on purpose. A run it declined pressed nothing and never
	// reached the machine, so there is no attempt to account for; the refusal is the
	// decision's own and is said in its own sentence.
	began := time.Now()
	// WHAT IS ABOUT TO RUN, READ ONCE AND BEFORE THE CLAIM. The reservation's provisional
	// record carries this identity, so an attempt that is killed leaves a reference that
	// still says which routine, which revision, which source and which contract — and the
	// bytes described are the bytes performed. See `snapshot`.
	// THE AUTHORISED VALUES, NOT THE ONES THE CALLER STILL HOLDS.
	//
	// STRUCTURAL, AND NO TEST CAN SEE IT SWAPPED: the two are equal by construction at
	// this line, and nothing in this process mutates the map between the door and here,
	// so the mutation that reads `p.named` instead survives the suite. It is written this
	// way because the property is "what runs is what was authorised", and a line that
	// reads the caller's map is one caller away from that being false — which is the same
	// reasoning `Resolved.WithValues` copies for, and the same shape ADR-271 records
	// about a review's `Was`.
	s := take(d, p.rt, resolved.Values, p.positional, p.flagged, p.expect, p.pinned)
	// ── AND AN INVOCATION THAT MAY NOT RUN AT ALL IS REFUSED, NOT FAILED ──
	//
	// Before the reference is claimed, for the reason the authority door's own refusal sits
	// where it does: a run that was declined pressed nothing and never reached the machine,
	// so there is no attempt to account for. A routine that types text somebody has to
	// supply, invoked without it, is that case -- and the alternative is what it did
	// before, which was to type nothing and report `performed`.
	if s.refused != "" {
		fmt.Fprintln(os.Stdout, s.refused)
		return OutcomeRefused, nil
	}
	res := citeRoutineRun(began, p.rt, s.did)

	var cancelled bool
	err := stopper(true, func(ctx context.Context) error {
		e := runRoute(ctx, d, s)
		// A CANCELLED RUN IS NOT A FINISHED ONE.
		//
		// `runFromEntryCtx` returns whatever the script left behind, and a script that was
		// cancelled leaves no error — so a stopped play used to exit 0 and every front end
		// that reads an exit code called it a success. The context knows better.
		//
		// Deleting this must fail TestACancelledPlayIsNotReportedAsPerformed.
		if ctx.Err() != nil {
			cancelled = true
		}
		return e
	})
	out, because := OutcomePerformed, ""
	switch {
	case cancelled:
		// AND A CANCELLED RUN CARRIES NO ERROR OUT, as it never did — EXCEPT THE PART
		// OF IT THAT IS STILL HAPPENING.
		//
		// The outcome stays `cancelled`, and that is right: somebody pressed Escape and
		// the record says so. But a chord that was stopped between its modifiers and
		// its key can also report a modifier the machine would not RELEASE, and that is
		// not an account of something that did not happen — it is a key held down on
		// this computer now, modifying every keystroke typed after it. Discarding the
		// whole error discarded that too, so the durable record read `cancelled` with
		// nothing beside it while Ctrl was down, and the window that reads the record
		// had nothing to show either.
		//
		// ASKED OF THE HOST AND NOT READ OUT OF THE ERROR. The error is flattened into
		// a sentence on its way up — `runRoute` builds "<routine> stopped: <what>" out
		// of the host's account, so nothing above it can recognise anything — and a
		// cancelled run discards it anyway. The host keeps this one where neither can
		// happen to it, which is the same shape as `Unperformed` and for the same
		// reason: an account, not a returned value.
		out = OutcomeCancelled
		if warn := leftBehind(d); warn != "" {
			because = warn
			// AND SAID OUT LOUD, ON THE RUN.S OWN NARRATIVE. The record is read
			// later and the person is here now — and it is how the window sees it
			// at all: Studio keeps the last thing the child said before the
			// outcome word. A warning that existed only in a file would be a
			// warning nobody gets.
			fmt.Println(warn)
		}
		err = nil
	case err != nil:
		out, because = OutcomeFailed, err.Error()
		// AND A FAILED RUN SAYS IT TOO, IF IT IS NOT ALREADY IN THERE. The error for a
		// refused keystroke carries the whole sentence, so this is usually a no-op; it
		// is here for the delivery failure whose account was built before the release
		// was attempted, which would otherwise read as a keystroke that did not happen
		// and nothing about the key still down.
		if warn := leftBehind(d); warn != "" && !strings.Contains(because, warn) {
			because += " — " + warn
			fmt.Println(warn)
		}
	default:
		// AND A RUN THAT DID EVERYTHING AND LEFT A KEY DOWN DID NOT SUCCEED.
		//
		// The branch that was missing, and the one case that reaches it: a routine that
		// finishes normally while holding a key, whose route-end sweep then cannot
		// release it. There is no cancellation and no execution error — every action
		// happened — so it fell through to `performed` with nothing said, which is the
		// product reporting success about a computer it has left in a changed state.
		//
		// `failed` is the word for it: "it having been tried, and gone wrong" is
		// exactly what happened, and a successful word here would be read by every
		// front end as nothing to look at. The key is named in the record, said on the
		// run's own output, and shown on the card, by the path the chord slice built.
		if warn := stillHeld(d); warn != "" {
			out, because = OutcomeFailed, warn
			fmt.Println(warn)
		}
		// AND TEXT LEFT HALF-SUBMITTED IS SAID WITHOUT CHANGING THE WORD.
		//
		// Which run reaches here holding one: a LEGACY route whose typing the machine
		// refused. Its contract is that a step that did not happen does not stop the
		// run — that is what every route saved before the chain existed promises, and
		// the one thing that must not happen to those routes is acquiring half of a
		// guarantee. A guarded route never gets here: the chain stops at the typing and
		// the account above fails the command.
		//
		// So the word stays `performed`, because the routine did run to its end, and
		// the field with part of a value in it is said beside it. That is a smaller
		// claim than the held-key branch above makes, and the difference is the
		// difference between the two: a key still down changes every keystroke anybody
		// types next, and this does not change anything — it is simply not finished.
		if warn := partlyTyped(d); warn != "" && !strings.Contains(because, warn) {
			because = alsoSay(because, warn)
			fmt.Println(warn)
		}
	}
	// AND WHAT BECAME OF IT, INTO THE REFERENCE THAT WAS ALREADY PRINTED. The same
	// identity the reservation was given, not a second reading of the file.
	keepRoutineRun(res, began, p.rt, s.did, out, because, askedOf(d))
	return out, err
}

// stillHolder is a host that can say whether this run left a key down.
//
// An interface rather than `*oshost.Host` for the reason `unperformer` is one: the concrete type
// sends real input, so the only way to put a cleanup failure in front of this decision is a host
// that reports one. See oshost.Host.NotReleased.
type stillHolder interface {
	// NotReleased is the key the machine would not let go of, or empty.
	NotReleased() string
}

// stillHeld is what this run may have left held down on the computer, or empty.
//
// WHY A RUN ASKS AT ALL, including one that was stopped: a key the machine would not release is
// not an account of something that did not happen. It is something that DID happen and has not
// finished, and until it is released every keystroke anybody types arrives modified. A cancelled
// run throws its error away on purpose — a stop is not a failure — and this is the half of it
// that must not be thrown away with it.
func stillHeld(d orchestrator.Deps) string {
	h, ok := d.Hosts["*"].(stillHolder)
	if !ok {
		return ""
	}
	return h.NotReleased()
}

// partialTyper is a host that can say whether this run left text half entered.
//
// A SECOND INTERFACE AND NOT A SECOND METHOD ON THE FIRST. A held key and a half-filled field are
// two different states of somebody's computer, and keeping the accounts apart is what lets each
// be reported in its own words — and what stops a routine that typed half a value being described
// as one that may have left a key down.
type partialTyper interface {
	// PartlyTyped is the text the machine did not take all of, or empty.
	PartlyTyped() string
}

// partlyTyped is what this run may have left half entered, or empty.
func partlyTyped(d orchestrator.Deps) string {
	h, ok := d.Hosts["*"].(partialTyper)
	if !ok {
		return ""
	}
	return h.PartlyTyped()
}

// leftBehind is everything this run may have left on the computer: a key still down, text not all
// submitted, or both.
//
// BOTH, AND NEVER ONE OF THEM. A run stopped between a modifier and its key can also have been
// stopped between two characters, and a report that showed one of those would be a report whose
// silence about the other reads as "that did not happen". The same reasoning the drag harness
// wrote down for its two accounts: a line that shows one of them is how a press inside a hold came
// to be reported as holding nothing.
func leftBehind(d orchestrator.Deps) string {
	return alsoSay(stillHeld(d), partlyTyped(d))
}

// alsoSay joins two warnings, either of which may be empty.
func alsoSay(said, more string) string {
	switch {
	case more == "" || strings.Contains(said, more):
		return said
	case said == "":
		return more
	}
	return said + " — and " + more
}

// checksItsWindow is whether a saved routine has a press that checks the window it was recorded
// in (guarded, or incomplete and refusing): the routines an Only-in run binds a window for.
func checksItsWindow(d orchestrator.Deps, rt routes.Route) bool {
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return false
	}
	c, _ := orchestrator.ContractOf(string(raw))
	return c != orchestrator.Legacy
}
