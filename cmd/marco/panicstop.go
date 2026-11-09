package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// withPanicStop runs fn under everything that is allowed to abort it.
//
// # It used to ask one question and answer two
//
// There was a single `if` here, and `MARCO_NO_PANIC_STOP` decided both halves of it: whether this
// process installs global keyboard hooks, and whether the run it wraps can be cancelled at all.
//
// The variable exists for the FIRST half only. A front end that already owns a low-level keyboard
// hook — the overlay does — must not have a second, competing WH_KEYBOARD_LL installed in the
// child it spawns; duelling low-level hooks, plus the play's own injected input, is a
// deadlock/zombie risk, and the hook invariants in CLAUDE.md are about exactly that thread. The
// front end owns the abort GESTURE. That is all it was ever saying.
//
// It was never meant to say "and this play may not be stopped". But that is what it said: the
// early return handed `fn` a `context.Background()`, and nothing can cancel one. So every play the
// overlay spawned — which is most plays a person runs — was uncancellable from the instant it
// started, and the only lever the overlay had left was to KILL the child. `TerminateProcess` runs
// no deferred function, so `finally` never ran: a held key stayed held and the cursor stayed
// wherever the play had dragged it.
//
// # So the two decisions are now two decisions
//
// Hook ownership is still `MARCO_NO_PANIC_STOP`'s, and the stop-key behaviour when hooks ARE
// installed is unchanged, down to the message it prints. Cancellability belongs to nobody's
// environment variable: EVERY run — hooks or not, real input or not, dryrun or SendInput — is
// watched by [[internal/stopsignal]], so one "stop" from any entrance and any process reaches it
// as a CANCELLATION and the runtime unwinds through `finally` on the way out.
//
// A dryrun run is watched too, deliberately. It costs one stat of a twenty-byte file every 100ms,
// and it means "stop" behaves the same way while somebody is trying a play out as it will when
// they run it for real — which is the only way a person ever comes to trust the word.
//
// Deleting the stopsignal.Watch must fail TestASpawnedPlayIsCancellableWithoutHooks.
func withPanicStop(realInput bool, fn func(context.Context) error) error {
	// THE ONE STOP, arm (a), RECEIVED here. Taken before anything else, because Watch reads its
	// baseline generation synchronously: a stop raised a millisecond after this process started
	// is a stop somebody meant for this play, and it must not be adopted as the baseline.
	ctx, release := stopsignal.Watch(context.Background(), stopsignal.Home())
	defer release()

	if !realInput || os.Getenv("MARCO_NO_PANIC_STOP") != "" {
		return fn(ctx)
	}
	rec := recorder.New()
	if err := rec.Start(); err != nil {
		return fn(ctx)
	}
	stop := recorder.ParseStopKey(stopKeySpec())
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for ev := range rec.Events() {
			if stop.Triggered(ev) {
				fmt.Fprintf(os.Stderr, "\n[aborted — %s]\n", stop.Label())
				cancel()
				return
			}
		}
	}()
	err := fn(ctx)
	cancel()
	rec.Stop()
	return err
}

// appOf returns the current foreground app for a Deps (used for scoped play
// resolution), or "" when unavailable.
func appOf(d orchestrator.Deps) string {
	if d.App == nil {
		return ""
	}
	return d.App()
}

// ran is what a performance actually executed, for its record to cite.
//
// Returned rather than worked out a second time by the caller. The digest and the contract both
// come off the bytes this function read, in the one place that reads them: deriving them again
// around the call would be a second answer about one file, and the day the two disagreed the
// record would cite source that did not run.
type ran struct {
	// Digest is `routes.DigestOf` over the SAVED source — the file, not the text with
	// arguments substituted into it. Somebody holding this record and the library can look
	// at exactly what ran; the arguments are a separate fact and are not in it, which also
	// keeps anything a person typed out of a durable file.
	Digest string
	// Contract is which contract the saved source STATES, in the three words
	// [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] gives them: `legacy` when no
	// press states a window, `guarded` when one does and carries every term the check
	// needs, and `incomplete` when it states a window and lacks a term — which refuses
	// rather than presses, naming what it lacks.
	//
	// Read by `orchestrator.ContractOf`, which is the ONE reader of this and whose list of
	// terms is held against the guard's own requirement by test. The first version of this
	// change asked the source itself — `strings.Contains(src, "RelTo \"")` — which is a
	// second answer to the question, and a weaker one: it cannot see `incomplete` at all,
	// so a record would have called an incomplete route guarded.
	Contract string
	// Mode is what the saved source STATES it was recorded as, or empty when it says
	// nothing — which is every routine saved before Studio could state one.
	//
	// Read by `orchestrator.ModeOf` off the same single read as the digest and the
	// contract, and for the same reason: the file is the account, and a second reading of
	// it is a second answer. Empty travels to the record as `unstated`, which is the word
	// the record already used before anything could put another one there.
	Mode string
	// Generation is what the saved source STATES about the generation of generator rules
	// that wrote it, verbatim, and GenerationRead is what this build made of that.
	//
	// BOTH, because they are two different facts and a record that held only the second
	// would lose the first exactly when it matters. A routine written by a LATER build
	// states a generation this one cannot explain; the reading is `unknown`, and the
	// declared value is the only thing that identifies the routine to whoever can explain
	// it. An older executor that wrote down its own verdict and dropped what the file said
	// would be the record contradicting the file it cites.
	//
	// Read by `orchestrator.GenerationOf` off the SAME snapshot that executes — the one
	// read `take` took — like the digest, the mode and the contract, and never from this
	// binary's own revision: `build` already says which marco ran, and a routine's
	// generation is a fact about the file.
	//
	// Both empty when the source could not be read at all. `unstated` is a real reading of
	// a real file and would be a reading of something nobody read; see contractUnstated for
	// the same distinction one field up.
	Generation     string
	GenerationRead string
}

// contractUnstated is what the record says when the contract could not be read at all.
//
// The source itself always states one of the three. This is for a route that could not be opened,
// where saying `legacy` would be a reading of something nobody read.
const contractUnstated = "unstated"

// snapshot is one read of a saved routine: the bytes, what they become, and what they are.
//
// # Why it is taken before the reference is claimed
//
// Because the record left in that reference has to say WHAT was running, and the moment that
// matters most is the moment the run never reaches its end — a killed child, a machine that went
// down, a stop that did not unwind. The identity used to be worked out from what the performance
// returned and written only at the end, so the one attempt a person has to ask somebody about was
// the one whose record said nothing but that something started.
//
// # And it is the same bytes that execute
//
// One read, not two. Reading the file to describe it and reading it again to perform it is two
// answers about one file, and a save can land between them — after which the record cites the
// digest of source that did not run. That is exactly the confusion `describeRoutine` prints a
// warning about, except arrived at by Marco's own hand rather than by somebody re-recording.
//
// A snapshot that could not be read carries its reason and runs nothing. It is still an attempt
// and still gets a reference, because "your routine is not where the library says it is" is
// something a person needs to be able to cite.
type snapshot struct {
	rt    routes.Route
	path  string
	named map[string]string
	// filled is the source with arguments substituted — what actually executes. The
	// arguments themselves are never in `did`, and so never in the record: somebody's
	// password arriving as an argument is not a thing to write into a durable file.
	filled string
	// did is the identity of what is about to run, and the identity the finished record
	// carries. One value, written twice, so the provisional record and the final one
	// cannot describe two different programs.
	did ran
	// contract is the same reading `did.Contract` holds, kept in its own type because the
	// rule at the end of the performance acts on it. The record wants a word and the rule
	// wants the thing; one read answers both.
	contract orchestrator.Contract
	// err is why there is nothing to perform.
	err error
	// refused is why this invocation may not run AT ALL, which is a different answer from
	// `err`: nothing was attempted, so it is a refusal rather than a failure and no
	// reference is claimed for it.
	//
	// Today it is one thing: a routine that types text somebody has to supply, invoked
	// without it. Taken HERE because the check needs the source and this function is the
	// one read of it -- asking again would be a second answer about one file, which is the
	// whole reason this snapshot exists.
	refused string
	// refusedAt is the stage the refusal belongs to, and refusedNext what to do about it, for the
	// latest-attempt record (lastattempt.go). Neither carries a value.
	refusedAt, refusedNext string
}

// take reads the routine once and says what is about to run.
func take(d orchestrator.Deps, rt routes.Route, named map[string]string,
	positional []string, flagged []string, expect, pinned string) snapshot {

	s := snapshot{rt: rt, path: d.Reg.Path(rt), named: named}
	mlog.Debug("run: reading route", "route", rt.Slug, "path", s.path)
	src, err := os.ReadFile(s.path)
	if err != nil {
		mlog.Error("run: read failed", "path", s.path, "err", err)
		s.did = ran{Contract: contractUnstated}
		s.err = fmt.Errorf("read %s: %w", s.path, err)
		return s
	}
	// WHETHER IT MAY RUN WITH THESE VALUES, BEFORE ANY OF THEM IS SUBSTITUTED.
	//
	// `ApplyArgs` leaves an unfilled placeholder EMPTY, which is right for the routines
	// that have none and wrong for a routine whose purpose is to type what somebody
	// supplied: it would press on and type nothing, and report `performed`. The product
	// already refuses the mirror case -- arguments given to a play that takes none -- for
	// the reason `performLearned` states, and this is that sentence with the subject
	// changed. See routes.CheckArgs.
	// ── AND IT IS THE VERSION THAT WAS AUTHORISED, COMPARED AGAINST THESE BYTES ──
	//
	// Against `src` — the one read this snapshot is made from, which is the same bytes
	// `filled` is built out of and the same bytes that execute. Asking the file again
	// would be a second answer about one file and would reopen the gap it is closing.
	//
	// THE GAP: a surface reads the routine, shows somebody what it would type, takes
	// their confirmation, and starts this process. A save landing between the
	// confirmation and this read runs a version nobody confirmed — with the values they
	// gave, which is the worst of it, because the text was approved for other steps.
	//
	// Refused, and nothing is pressed: this is before `ApplyArgs` and before any
	// reference is claimed. Empty `expect` pins nothing, which is every entrance that
	// confirmed nothing.
	// AND THE BYTES A SHORTCUT PRESS PINNED. Not a confirmation (see Expect): the press read
	// this routine under its lock while the key still named it, and anything else at this
	// identity now is not the routine the key was bound to.
	if pinned != "" && routes.DigestOf(string(src)) != pinned {
		mlog.Info("run: refused", "route", rt.Slug, "why", "not the play the shortcut "+
			"was pressed for")
		s.refused = fmt.Sprintf("%q changed between the shortcut being pressed and the "+
			"run starting, so nothing has been run — press it again",
			plays.Pretty(rt.Slug))
		s.refusedAt, s.refusedNext = lastattempt.StageVersion, "press the shortcut again"
		s.did = ran{Digest: routes.DigestOf(string(src)), Contract: contractUnstated}
		return s
	}
	if expect != "" && routes.DigestOf(string(src)) != expect {
		mlog.Info("run: refused", "route", rt.Slug, "why", "the play changed since "+
			"it was confirmed")
		s.refused = fmt.Sprintf("%q has been saved again since you confirmed what it "+
			"would do, so nothing has been run — look at it again",
			plays.Pretty(rt.Slug))
		s.refusedAt, s.refusedNext = lastattempt.StageVersion,
			"ask for it again, check the read-back of what it will do, then confirm"
		s.did = ran{Digest: routes.DigestOf(string(src)), Contract: contractUnstated}
		return s
	}
	// AND A VALUE OFFERED THROUGH AN ENTRANCE THAT MAY NOT OFFER IT.
	//
	// Asked first, because it is about the request rather than about the routine: a flag
	// carrying a secret-typed argument is refused whatever else is right about the
	// invocation. See routes.CheckFlagged.
	if why := routes.CheckFlagged(string(src), flagged); why != nil {
		mlog.Info("run: refused", "route", rt.Slug, "why", why.Error())
		s.refused = fmt.Sprintf("%q: %v", plays.Pretty(rt.Slug), why)
		s.refusedAt, s.refusedNext = lastattempt.StageValues,
			"say the value as part of the request, or open it in Studio — a secret is never given as a flag"
		s.did = ran{Digest: routes.DigestOf(string(src)), Contract: contractUnstated}
		return s
	}
	if why := routes.CheckArgs(string(src), named, positional); why != nil {
		mlog.Info("run: refused", "route", rt.Slug, "why", why.Error())
		s.refused = fmt.Sprintf("%q: %v%s", plays.Pretty(rt.Slug), why,
			whereToReviewIt(why, expect))
		s.refusedAt, s.refusedNext = lastattempt.StageValues, valuesNext(rt, why)
		s.did = ran{Digest: routes.DigestOf(string(src)), Contract: contractUnstated}
		return s
	}
	s.filled = routes.ApplyArgs(string(src), named, positional)
	// WHAT IS ABOUT TO RUN, DESCRIBED ONCE. See `ran`.
	s.contract, _ = orchestrator.ContractOf(s.filled)
	gen := orchestrator.GenerationOf(s.filled)
	s.did = ran{Digest: routes.DigestOf(string(src)), Contract: string(s.contract),
		Mode:       string(orchestrator.ModeOf(s.filled)),
		Generation: gen.Declared, GenerationRead: string(gen.Reading)}
	return s
}

// whereToReviewIt is the clause added when a routine needs text and nothing could review it.
//
// # The surfaces that cannot show a proposal, saying so
//
// A request can already say what to type — *write a note saying hello there* — and Director reads
// which words those are. But a PROPOSAL is only useful where somebody can see it and correct it
// before anything is typed, and three entrances have no field and no confirmation step: the CLI,
// a hotkey, and the overlay's command line (which the voice path reaches it through). For those,
// a routine that needs text and was given none has three possible behaviours: run with a guess,
// fail quietly, or EXPLAIN.
//
// The first is what every boundary in this slice exists to prevent and the second is worse than a
// refusal — so this is the third. The refusal already names the argument; this says what to do
// about it.
//
// # And it is gated on whether anything CONFIRMED, not on which surface asked
//
// `expect` is the digest of the routine an invocation was authorised against, and
// `invoke.Request.Expect` already says what empty means: *the CLI, a hotkey and the overlay pin
// nothing because nothing was confirmed to them*. So an empty `expect` IS "nothing reviewed this",
// which is exactly the condition, read off the request rather than off a list of surface names
// that would have to be kept right. Studio sends one, so Studio is never told to open Studio.
//
// `Request.Source` is deliberately not consulted: it is recorded and never read, and this is not
// the place to start.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].
func whereToReviewIt(why error, expect string) string {
	var needed *routes.ValuesNeeded
	if !errors.As(why, &needed) || expect != "" {
		return ""
	}
	return ". This request needs text values reviewed before running. Open it in Studio, " +
		"or supply the value with the request"
}

// runRoute performs a snapshot: the source already read, with its args already substituted into
// its {{name}}/{{N}} placeholders.
func runRoute(ctx context.Context, d orchestrator.Deps, s snapshot) error {
	if s.err != nil {
		return s.err
	}
	// Expose the named args to the OS host so a secret-typed arg (password/pin/…) can
	// be provided inline and remembered (see oshost.doSecret).
	if h, ok := d.Hosts["*"].(*oshost.Host); ok {
		h.SetArgs(s.named)
	}
	rt, path := s.rt, s.path
	mlog.Info("run: executing", "route", rt.Slug, "scope", rt.App, "named_args", len(s.named))
	if err := driver.RunSourceWithHostsCtx(ctx, s.filled, filepath.Dir(path), path, os.Stdout, d.Hosts); err != nil {
		return err
	}
	// ── AND A ROUTINE THAT DID NOT HAPPEN IS NOT A ROUTINE THAT RAN ──
	//
	// A guarded route stops at the first thing that did not happen: a press it could not
	// check, a press the machine would not deliver, or an application that never came
	// forward. The program itself ends tidily either way — a root handler is compulsory
	// in Marco, so the failure is caught and logged — which means without this, the
	// command would exit successfully about a routine that pressed nothing.
	//
	// The host keeps the account so the answer is data rather than a line of output
	// somebody has to read. See oshost.Host.Unperformed.
	//
	// WHICH CONTRACT THIS ROUTE WAS SAVED UNDER decides whether that account stops the
	// run, and it is read ONCE, by `contractOf` above. A point that states the window it
	// was recorded in IS the guarded contract — it is what makes the three saved contracts
	// distinguishable at all (see ADR-251) — and a route carrying none reports exactly what
	// it always did.
	//
	// Read from the source rather than from a field, because there is no field: the
	// contract lives in the saved Marco, which is the only executable truth here. The
	// record cites the same answer this rule acts on, which is why `ran` carries it rather
	// than the caller asking the file a second time.
	//
	// LEGACY IS THE ONE THAT LEAVES, and the test it is written against matters: `!= Guarded`
	// is the plausible-looking form and it is wrong, because an INCOMPLETE route states a
	// window and refuses the press naming what it lacks. Skipping the account for it would
	// let exactly that refusal pass as a successful run — the case ADR-251 made a third
	// contract for. Held by TestAnIncompleteRoutinesRefusalStillFailsTheCommand.
	if s.contract == orchestrator.Legacy {
		return nil
	}
	if h, ok := d.Hosts["*"].(unperformer); ok {
		if undone := h.Unperformed(); len(undone) > 0 {
			mlog.Error("run: stopped", "route", rt.Slug, "why", undone[0])
			return fmt.Errorf("%s stopped: %s", rt.Slug, undone[0])
		}
	}
	return nil
}

// unperformer is a host that can say what it did not do.
//
// An interface rather than `*oshost.Host` so the rule above can be exercised with a host that
// reports a DELIVERY failure — a guard that passed and a machine that then rejected the input.
// The concrete assertion could only be satisfied by the real host, whose backend sends real
// input, so the only command-level case that could be written was an absent application: the
// cheapest failure and the one furthest from the press.
type unperformer interface {
	// Unperformed is everything the run did not do, in order, each with the sentence
	// that said why. See oshost.Host.Unperformed.
	Unperformed() []string
}
