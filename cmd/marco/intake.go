package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
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

// Outcome is what happened, in the six words every surface can render.
//
// # Why an exit code was not enough
//
// Because three of these used to be indistinguishable from success. A play the door DECLINED
// returned nil ("you said no" is not an error). A cancelled local play returned nil. A generated
// play whose Screen guard refused caught its own failure, logged it, and returned nil — so the
// process exited 0 while nothing had happened. The overlay had three words for all of it and
// picked them by exit code, so "Marco refused", "you cancelled" and "it worked" all rendered `ok`.
type Outcome string

const (
	// OutcomePerformed means it ran AND arrival was positively verified. Nothing else may
	// claim this word — see [[ADR-070]]: finishing is not arriving.
	OutcomePerformed Outcome = "performed"
	// OutcomeClarify means Director asked something and is waiting.
	OutcomeClarify Outcome = "clarify"
	// OutcomeRefused means Marco declined, or was not permitted: the door said no, a guard
	// did not recognise the place, an edge would not verify.
	OutcomeRefused Outcome = "refused"
	// OutcomeUnavailable means the request was never delivered — nothing tried it.
	OutcomeUnavailable Outcome = "unavailable"
	// OutcomeCancelled means somebody stopped it.
	OutcomeCancelled Outcome = "cancelled"
	// OutcomeFailed means it was tried and it went wrong.
	OutcomeFailed Outcome = "failed"
)

// Exit is the process code for an outcome.
//
// `exitUnavailable` (3) keeps the meaning it already had — "never delivered, a caller may try
// something else" — because plugins/overlay already reads exactly that. The rest are distinct so
// that a front-end which can only see an exit code still cannot mistake a refusal for a success.
func (o Outcome) Exit() int {
	switch o {
	case OutcomePerformed:
		return exitOK
	case OutcomeUnavailable:
		return exitUnavailable
	case OutcomeClarify:
		return 4
	case OutcomeRefused:
		return 5
	case OutcomeCancelled:
		return 6
	default:
		return exitNotVerified
	}
}

// resultPrefix is the wire line a front-end reads to know what happened.
//
// Beside `[route] `, which says WHICH play resolved and is deliberately unchanged. This one says
// what became of it. Both are protocol, not user text; plugins/overlay matches these literals and
// the two sides ship as separate Go modules, so nothing would fail to compile if either drifted.
//
// Changing this literal must fail TestTheOutcomeIsAnnouncedOnTheWire.
const resultPrefix = "[result] "

// announce publishes the outcome for whatever is reading this process's stdout.
func announce(o Outcome) { fmt.Printf("%s%s\n", resultPrefix, o) }

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
	dec := invoke.Decide(d.Reg, req)
	traceIntake(dec, req)

	switch dec.Kind {
	case invoke.KindControl:
		return runControl(), nil
	case invoke.KindPlay:
		return performDecidedPlay(d, dec, req)
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
	steps := []playInvocation{{rt: dec.Play, named: req.Named, positional: req.Positional}}
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
func outcomeOfDirectorExit(code int) Outcome {
	switch code {
	case exitOK:
		return OutcomePerformed
	case exitUnavailable:
		return OutcomeUnavailable
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
	resolved := orchestrator.Classify(d.Reg, p.rt, plays.Pretty(p.rt.Slug))
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
		if err := performLearned(d, resolved, p.named, p.positional); err != nil {
			return OutcomeFailed, err
		}
		return OutcomePerformed, nil
	}

	var cancelled bool
	err := stopper(true, func(ctx context.Context) error {
		e := runRoute(ctx, d, p.rt, p.named, p.positional)
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
	switch {
	case cancelled:
		return OutcomeCancelled, nil
	case err != nil:
		return OutcomeFailed, err
	}
	return OutcomePerformed, nil
}
