---
type: adr
status: accepted
date: 2026-10-25
decision: "ACCEPTED — A CHORD IS A SEQUENCE OF KEY EVENTS AND A PROMISE TO UNDO IT, AND THE PROMISE IS NOW KEPT, CHECKABLE AND CHECKED. `ctrl+shift+a` is not one thing the machine does: it is hold Ctrl, hold Shift, settle, tap A, hold, release A, release Shift, release Ctrl — and the releases are the promise, because a modifier this process pressed stays down on somebody's computer until this process lets go of it and nothing else will clean that up. Three defects lived in the middle of that sequence, inside the function that calls `SendInput`, where no test could reach them: a REJECTED modifier returned from the middle of the loop leaving the earlier modifiers held down, so a refused Shift left a stuck Ctrl and reported only the failed keystroke; a STOP during the modifier settle stopped nothing, because `sleepCtx` returns early on cancellation and the next line tapped the key regardless, so Escape pressed over a chord in flight sent the chord; and a RELEASE the machine refused was discarded entirely, so the one outcome a person most needs to hear about — a modifier still down, every keystroke afterwards arriving modified — was the one outcome that reported success. The sequencing now lives in `sendChord` against a `keyMachine` that takes one key event at a time, so a refused modifier, a stop mid-flight and a release the machine will not take are all arrangeable without a desktop; every key pressed gets a RELEASE ATTEMPT before the function returns, in reverse, whether the chord worked, was refused or was stopped — an attempt and not a release, because the machine can refuse one and that refusal is the whole point; a stuck key outranks a refused keystroke in what a caller is told, EXCEPT that a cancellation keeps its identity so a person pressing Escape does not become a routine that failed. AND A STOPPED RUN MAY NOT CARRY A STUCK KEY AWAY WITH IT: `performOnePlay` discards a cancelled run's error on purpose, so the key still held down is kept by the HOST in an account of its own — the error is flattened into a sentence on its way up and discarded at the top, and an account survives both — and from there into the durable record and onto the run's own output, where the window shows it beside "was stopped part of the way through". The outcome stays `cancelled`: a stop is not a failure. And the desktop half is held by `plugins/e2e` `keys/a chord arrives as a chord`, on three accounts that no unit test can supply: the order of the messages the application received, the modifier state the application read from the system at the instant the key arrived — which is what tells a chord from four keystrokes, and catches a sender whose modifier events the system never applied — and what this desktop is still holding down afterwards, asked of Windows directly, because no message arrives to tell an application about a key nobody released. THE SAME RULES THEN GOT A SECOND SLICE: A HOLD — `hold Q, click, release Q` — is a key held down on purpose for as long as the routine says, and the thing that cleans up after one is the END OF A RUN, so a hold whose release goes wrong goes wrong when nobody is watching. Four of the same defects were there one layer up, plus a fifth: `doKeyUp` forgot the key before asking whether the release worked, so the sweep that could have tried again was told it had come up; the sweep discarded every answer, making the one path whose purpose is "nothing is left down" the one that could not say otherwise; the keys were held in a MAP and released in whatever order Go produced; `keyDown` reported a stop after a successful send, so a run cancelled in that instant pressed a key, called the hold failed, never wrote it down as held, and left it down for ever invisible to everything; and nothing stopped a stopped run from starting a hold at all. Holds are an ordered list now, released in reverse, a refused release stays on the list for the sweep to retry once, and whatever the sweep still cannot release goes into the same account — record, output, card. IT DOES NOT RETRY A RELEASE MORE THAN THAT: a modifier the machine refuses twice is a different problem and a retry loop around input is a decision of its own."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/oshost/chord.go
  - internal/oshost/backend_windows.go
  - plugins/e2e/scenario_keys.go
  - plugins/fixture/Fixture.cs
---

# A chord is a promise to let go

## What a chord actually is

`ctrl+shift+a` reads like one keystroke and is six events:

	hold Ctrl → hold Shift → settle → tap A → hold → release A → release Shift → release Ctrl

The settle is there because a combination sent without one is missed: the modifier has to be
down, as far as the system is concerned, before the key arrives. The hold is there because an
instant down-and-up is what a game drops.

The last two events are of a different kind from the rest. They are a **promise**. A modifier
this process pressed is held down on somebody's computer until this process releases it. Windows
does not clean that up when the routine ends, the application cannot see that it is happening,
and the person's next keystroke — and every one after it — arrives as something else.

## Three defects, all of them in the middle of a function nothing could reach

The whole sequence lived inside `winBackend.key`, between two calls to `SendInput`. That is why
these survived: there was no seam at which a test could refuse a modifier, stop a run mid-chord,
or decline a release.

**A rejected modifier left the earlier ones down.** The loop that holds the modifiers returned
its error from the middle of itself. `ctrl+shift+esc` with the Shift rejected returned *SendInput
rejected key "shift"* with Ctrl still held on the desktop. The reported half — a keystroke that
did not happen — was the smaller half.

**A stop during the settle did not stop the chord.** `sleepCtx` returns early when the run is
cancelled, which is how a stop is supposed to arrive. The next line pressed the key anyway. So
Escape or the panic key, pressed in the one window where a chord was in flight, sent the chord.
This had never fired as far as anybody knows, which is its own argument for a test.

**A release the machine refused was discarded.** The reverse loop ignored every return value, so
the single worst outcome reported success: nothing in the run record, nothing in the command's
exit status, and a modifier held down on the desktop.

## So the sequence moved, and the machine became an argument

`sendChord` holds the order and the undertaking. `keyMachine` is the machine as this needs it —
`down`, `up`, `settle`, three methods and no state — and the Windows backend supplies the real
one, where `settle` is the wait that returns early on a stop. Resolution stays in the backend and
happens for **every** part before anything is pressed, so an unknown key in the middle of a chord
is an error returned with nothing held.

The rules, in the order they are read:

- **Nothing is sent to a run that has already stopped.** Checked before the first event, not only
  around the settle.
- **Every key pressed gets a release ATTEMPT before the function returns**, in reverse, whether
  the chord worked, was refused, or was stopped. An attempt and not a release, and the distinction
  is precisely the one that matters here: the machine can refuse a release, which is the failure
  this whole decision exists to report. Every key is attempted even after one of them fails,
  because the alternative is leaving the rest down in order to report the first.
- **The tap does not happen if the run was stopped while the modifiers settled.**
- **A key the machine would not press is not released**, because `SendInput` takes an event or it
  does not, and sending the release produced a second refusal and a sentence naming two failures
  for one.
- **A stuck key outranks a refused keystroke** in what the caller is told: a refusal is something
  that did not happen, a stuck modifier is something that is still happening. The exception is a
  cancellation, which keeps its identity — a stop that stopped reading as a stop would turn a
  person pressing Escape into a routine that failed — and says the stuck key in the same breath.
- **The failure names the part and the chord**, not the whole spec. *The machine refused the "c"
  in "ctrl+c"* rather than *SendInput rejected key "ctrl+c"*.

## And the desktop half, which no unit test can supply

`sendChord`'s tests establish the sequencing. They cannot establish that the real `SendInput`,
the real scan codes, the real foreground window and the real application agree that a chord
happened. `plugins/e2e` `keys/a chord arrives as a chord` does, on three accounts:

	the ORDER of the messages        the fixture's own log: Ctrl down, Shift down, A down,
	                                 A up, Shift up, Ctrl up — every down with its up
	what the application BELIEVED    the modifier state the window read from the system at the
	                                 instant A arrived
	what the DESKTOP is still holding asked of Windows directly, after the run

The second is new in the fixture and is the fact rather than the evidence. A sender whose
modifier events the system did not apply — a wrong scan code, a lost extended flag — produces
exactly the same four messages in exactly the same order and delivers a plain A. The window asks
the system what is held at the moment the key reaches it, with the same call any application
would use, so the two cases are distinguishable.

The third is not about the fixture at all. No message arrives to tell an application that a key
nobody released is being held, so the only account that can see a broken promise is the desktop's
own. It is read before the run as well as after: a modifier already held when the scenario starts
is a fact about this desktop and not about Marco, and the scenario declines rather than fails.

The chord is `ctrl+shift+a`: two modifiers, so the reverse-order release is exercised rather than
assumed, and harmless in every application including the one it would reach if activation failed.
Deliberately not `ctrl+shift+esc`, which opens the task manager. The harness sends no input — the
chord is Marco's, through the real executor — but choosing a chord that would be destructive if
it went somewhere unexpected would be choosing one.

The demonstration is **recorded as a chord rather than written as one**: the seeded events are
modifier downs, the key, and the modifier ups, exactly as the hooks would have left them, because
`simplify` is what folds those into `ctrl+shift+a` and that is production code with its own rule.
A harness that wrote the chord string itself would be asserting against its own opinion of what a
demonstration produces. What comes out is checked against what was asked for before the scenario
runs, so a demonstration that came out as four separate keystrokes is a precondition failure
rather than a scenario that passes about the wrong routine.

## And a stopped run may not carry a stuck key away with it

The three rules above are about the boundary. One more is about the run, and it was found in
review after the rest had landed.

`performOnePlay` discards a cancelled run's error on purpose — a stop is not a failure, and the
outcome is `cancelled` with nothing beside it. So a chord stopped between its modifiers and its
key reported `cancelled` while Ctrl was down: nothing in the durable record, nothing on the run's
output, and nothing for the window to show. `saidOfRoutine` had the same gap one layer further
on, printing *"X" was stopped part of the way through.* and no more.

**Why it is an account the host keeps and not an error anybody reads.** Two things happen to that
error on the way up: `runRoute` turns the host's account into the sentence *"X" stopped: …*, so
nothing above it can recognise anything inside it, and `performOnePlay` then throws it away. An
error cannot survive either. So the host keeps the key it could not release the way it already
keeps what it did not do — `Host.NotReleased`, beside `Host.Unperformed` — and the two are
deliberately separate lists: everything in `Unperformed` is something that did not happen, and
this is the opposite shape, something that DID happen and has not finished.

From there it goes three ways: into `because` in the durable record, onto the run's own output as
the last thing it says (which is how the window sees it at all — Studio keeps the final sentence
before the outcome word), and through `alsoSaid` into the sentence on the card. The outcome stays
`cancelled` throughout. A person who pressed Escape did not break anything, and a person whose
Ctrl is stuck needs to be told.

## And a HOLD is the same promise, kept by the end of a run

A hold — `hold Q, click, release Q` — is a key held down on somebody's computer deliberately, for
as long as the routine says. `simplify` produces one whenever a key stays down across something
else. So it is the same promise as a chord's modifiers, with one difference that makes it worse:
the thing that cleans up after a hold is the END OF A RUN, so a hold whose release goes wrong
goes wrong at the moment nobody is watching any more.

Five defects, four of them the shapes above, one layer up:

- **`doKeyUp` forgot the key before finding out whether the release worked.** The route-end sweep
  is the one thing that could try again, and it was being told the key had come up. A refused
  release left a key down with nothing tracking it.
- **The sweep discarded every answer** — `_ = h.b.keyUp(...)` — which made the one path whose
  entire purpose is "nothing is left down" the one path that could not say otherwise.
- **The keys were held in a MAP**, so they were released in whatever order Go produced. An
  application reading the keyboard while a pile of holds comes apart sees a different thing
  depending on which goes first, and nothing could pin it. They are a list now, released in the
  reverse of the order they went down — the same rule `sendChord` follows.
- **`keyDown` reported a stop after a successful send.** `return ctx.Err()` at the end of the
  primitive, so a run cancelled in that instant pressed the key, reported the hold as failed, and
  was therefore never written down as holding anything: a key down for ever, invisible to the
  sweep, the record and the person. A send the machine took is now reported as having happened,
  because it did; whether the run goes on is decided where the run is.
- **And nothing stopped a stopped run from starting a hold**, which is a key pressed down by a
  routine that is already over.

A refused release is kept on the list, so the sweep tries once more — one refusal then success is
the ordinary transient, and that recovery is the only one here. Everything the sweep still cannot
release goes into the same account a chord's does, so it reaches the record, the run's output and
the card by the path that was already built. Releasing is gated on nothing: a stopped run is
exactly when a key most needs to come up.

### And two ways a hold's failure went nowhere

Found in review of the slice above, and both are about PROPAGATION rather than cleanup.

**The generator propagated `Key` and not `KeyDown` or `KeyUp`.** So a guarded routine's hold was
written as an ordinary step, and `hold Q → click → release Q` could click after Q failed to go
down. The click is a different gesture without the Q — a modified click is not a click — and the
window guard cannot tell, because the window is the same window. The release is the half that
looks harmless and is not: everything after a release the machine refused runs with the key STILL
DOWN, so the presses that follow are modified presses, and the sweep recovering the key afterwards
is not the same as the routine having been right to carry on. Both are links in the chain now, in
newly generated guarded routines; nothing on disk is rewritten.

**And the host recorded neither failure.** `doKeyDown` returned a failed step and `doKeyUp`
returned the error, and neither reached `Unperformed` — so the command read an empty account and
reported success about a routine that ran with a key in the wrong state. Both go through
`delivered` now, like a press and like a keystroke.

Neither half is sufficient. A propagated step whose failure is not in the account lets the command
report success; an account without propagation lets the routine go on pressing. The tests go
through generation, the runtime and the production host for exactly that reason, with a legacy
control that still attempts its input and carries on.

### And a run that finished holding a key did not succeed

`performOnePlay` read the cleanup account in the cancelled and the failed branches, and not in the
one every successful run reaches. So a routine that finished NORMALLY while holding a key, whose
route-end sweep then could not release it, had no cancellation and no execution error — every
action it was asked for happened — and fell through to `performed` with nothing said.

It is `failed` now: *it having been tried, and gone wrong*, which is what happened. The key is
named in the record, said on the run's own output and shown on the card. The control for that
check is the one that matters most, because the branch is reached by every successful run of every
routine: a check that was wrong about the empty case would turn the product's ordinary answer into
a failure.

One imprecision is left and is deliberate. Studio's sentence for a failed routine is *"X" stopped,
and nothing after the press that did not happen was done*, which is exactly right when a hold or
release failed mid-routine and slightly wrong when only the route-end sweep failed — nothing
stopped and nothing was skipped. The specific sentence arrives beside it through `alsoSaid`, and
rewording the shipped sentence for every failure to suit this one case would cost more than it
buys. Recorded rather than quietly changed.

## And the desktop half of a hold, written before it could be run

The chord has a desktop scenario and the hold did not, because the screen it needs was not
available — which is a reason to defer EXECUTION and not a reason to defer writing. Two scenarios
now exist, and neither has run:

- `hold/a hold is held across the press` — a focusing press, a key held across a second press, a
  release. Four questions: the key goes down before the press it brackets; the application read
  the keyboard as holding that key WHEN THE PRESS ARRIVED; the application saw the release; and
  the desktop is holding neither that key nor any modifier it was not already holding.
- `hold/a stopped run lets go of the key` — Stop pressed while the key is down, waited for rather
  than timed. The press inside the hold never arrives, the record says cancelled, and the key is
  not down afterwards.

**The second account needed the fixture to learn something.** `Control.ModifierKeys` answers for
Ctrl, Shift and Alt, which is the whole of what a chord claims and half of what a hold claims: a
held Q is not a modifier, so a window asked about one could only ever have answered "no
modifiers". The fixture now reports the state of keys the harness NAMES — `--watch q` — read with
`GetKeyState` at the instant the message is handled, on PRESSES as well as on keys, because the
claim is about what was held when the click arrived. `GetKeyState` and not `GetAsyncKeyState`
deliberately: the first is the keyboard as it stood for the message being processed, so a press's
record says what was held when THAT PRESS arrived rather than what is held now that somebody is
reading the log. The asynchronous question — is a key still down after the run — is the harness's
to ask of the desktop, and the window could not answer it anyway. Named rather than enumerated: a
window that reported every key's state would be a keylogger, which is not a thing to leave in a
test fixture.

**The key is Q and not a modifier, for two reasons.** A modifier would not work: `simplify` tracks
modifiers to fold them into chords and `markHolds` skips them by name, so Ctrl held across a click
produces no key step at all and the click comes out unmodified. And the consequence of the
scenario failing is a key left down on a real desktop — a letter repeats itself into whatever has
the keyboard, while a modifier changes every keystroke anybody types on the machine until somebody
works out why. If a promise is going to be broken, it should be the cheaper one.

**A refused delivery and a refused release are deliberately NOT on the desktop.** Both are covered
headlessly, above. A desktop scenario that needed a key to be STUCK in order to assert anything
would be arranging to leave one down on somebody's computer. And when one of these scenarios finds
a key held, it reports it and says to press and release the key: the harness sends no input, by the
rule that makes the fixture's log attributable in the first place, so it will not put the key up
itself.

**They are gated out of a default run and named in every report.** `onlyWhenAsked` keeps a
scenario that has never passed on a desktop out of a run that did not ask for it, because adding
an unexercised scenario to a regression gate makes the next ordinary run a debugging session — the
first failure anybody sees is as likely to be the scenario's own as the product's. But a harness
quietly holding back a scenario it owns would make its own green summary mean more than it should,
so the report names them beside the physical-capture limit, in the same words every time:
*implemented and NOT EXERCISED*. They are filed under `hold/` rather than `keys/` so that `-only
keys`, the chord's gate, does not sweep them in. The flag comes off after one green run, and the
first half of `TestTheHoldScenariosAreAskedForByName` comes off with it.

**And the oracle is tested without a desktop.** A scenario is an arrangement and a set of questions
asked of what came back; the arrangement needs a screen and the questions are functions over a
log. So `plugins/e2e/hold_test.go` feeds each one a log the fixture could really have written, in
both directions — the right log must produce no finding, the wrong one must produce one that names
what was wrong — including the case no other account can see: the same messages in the same order
with the system never having applied the key, which delivers a plain press. Six mutations, six
killed, each by the test named for it. The half of a desktop scenario that decides whether a green
run means anything is the half that was never checked before it ran.

## What did not move

- **No retry on a release.** A modifier the machine refuses to release twice is a different
  problem — a different integrity level, a lock screen, a UIPI boundary — and a retry loop around
  input is a decision of its own rather than a consequence of this one. It is said loudly instead.
- **A failed release is REPORTED, not recovered.** Worth stating in those words, because
  everything above is machinery for saying so and none of it puts the key back up. The run
  reports; the record keeps it; the card shows it; the key is still down. Whoever reads the
  sentence is the recovery.
- **And forced termination is outside the undertaking entirely.** The releases happen because
  `sendChord` reaches them — on success, on a refusal, on a stop. A process killed between a
  modifier going down and its release runs no more code, so nothing releases it: not this, and not
  the driver's `ReleaseHeld`, which is a deferred call and covers only the keys an explicit
  `KeyDown` hold is tracking (a chord's modifiers are not in that list, because `sendChord` owns
  them for the few milliseconds they exist). `runs/a killed runner leaves an unfinished record` is
  the harness scenario that kills a runner mid-run; it establishes that nobody can speak for such
  an attempt, which is exactly the right claim and not a recovery either. The guarantee is: every
  pressed key gets a release attempt **by code that gets to run**.
- **And nothing on disk was regenerated.** The propagation fix is in the GENERATOR, so a routine
  saved before it keeps the chain it was saved with: its hold is an ordinary step and its failure
  stops nothing. That is the contract that routine was saved under, and the reason
  `TestALegacyRoutineWhoseHoldFailedCarriesOn` exists rather than being a curiosity. Re-saving one
  picks the new shape up; there is no migration and no automatic rewriting. Worth stating in these
  words because "generated guarded routines now stop on a failed hold" is true of routines
  generated from now on and of no routine already in somebody's library.
- **A Ctrl- or Shift-held CLICK is not established by any of this, and is a separate open
  defect.** The hold work covers a non-modifier hold such as Q. A modifier held across a click is
  a different thing and it is currently LOST: `simplify` emits a plain click and the modifier
  appears nowhere in the steps — measured, not inferred. `markHolds` skips modifiers by name,
  which is right when the thing between the edges is another key (that is how a chord is made)
  and wrong when it is a click, because there is no chord to fold into. It is bounded rather than
  a language question: `hold ctrl → click → release ctrl` already generates, already checks out
  through `driver.CheckSource`, and already propagates with nothing unpropagatable, so no new
  word and no new field is needed. But it changes what `simplify` makes of recordings that
  already exist, which is a decision and not a correction. Recorded in
  [[Demonstrations]], and it is also the reason the desktop hold scenario holds Q: a modifier
  would have been dropped before it ever reached a routine.
- **`keyDown` / `keyUp` — the holds — are untouched.** They are a single key each and already
  report what the machine said.
- **The timings are the timings.** `$MARCO_CHORD_HOLD_MS` and `$MARCO_KEY_HOLD_MS` mean what they
  meant, and an explicit `Hold` still wins over both.
- **Nothing about recording changed.** `simplify` already folded held modifiers into a chord; the
  only new thing on that side is that a harness now demonstrates one.

## Enforced by

- `internal/oshost` `TestAChordHoldsItsModifiersAndLetsThemGoInReverse` — the control, and the
  reason it is first: a sequence that pressed nothing at all would satisfy every assertion about
  what was left down.
- `internal/oshost` `TestAPlainKeyIsPressedWithALingerAndNothingElse` /
  `TestAnExplicitHoldIsHowLongTheComboStaysDown` — a plain key gets the tap linger, a combo gets
  the settle, and an asked-for hold beats both.
- `internal/oshost` `TestARefusedModifierReleasesTheOnesAlreadyHeld` — the first defect, asserted
  on what the computer is left holding and on the key never being tapped without its modifiers.
  Restoring the bare return fails it alone.
- `internal/oshost` `TestARefusedKeyStillReleasesTheModifiers` — the half that always worked, held
  because the fix moved the release and a release that moved too far would take this with it. It
  also asserts that a key the machine would not press is not released.
- `internal/oshost` `TestAStopWhileTheModifiersSettleDoesNotTapTheKey` — the second defect, with
  the stop arriving the way a real one does: during the settle. Removing the context check after
  the settle fails it alone.
- `internal/oshost` `TestAChordSendsNothingToARunThatHasAlreadyStopped` — and the entrance check,
  which fails alone when removed.
- `internal/oshost` `TestAReleaseTheMachineWillNotTakeIsReported` — the third defect, and the
  quietest. Discarding release failures fails this and both of the pair below.
- `internal/oshost` `TestAStuckModifierOutranksARefusedKeystroke` /
  `TestAStopThatLeavesAKeyStuckIsStillAStopAndSaysSo` — which of two bad things leads, and that a
  stop keeps its identity while still naming the key it left held.
- `plugins/e2e` `keys/a chord arrives as a chord` — the desktop half, on all three accounts, with
  a precondition that the routine's own press landed first (a chord goes wherever the keyboard is,
  so a routine whose press missed would send its chord somewhere else and the missing keys would
  have nothing to do with chords).
- `plugins/fixture/Fixture.cs` `ModifiersNow` — the application's own reading of the keyboard,
  written into every key record beside the key name.
- `internal/oshost` `TestARunStoppedMidChordLeavesNothingPressedAndSaysWhatIsStuck` — the
  cancellation path through the **production host**: a demonstration taken through the production
  recorder, simplifier, generator, registry and compiler, performed by `oshost.Host` with only the
  machine substituted, the stop arriving during the modifier settle and the machine refusing to
  release Ctrl. Four claims in one case because they are one event from where a person is
  standing: the key was never tapped, the press after it never happened, every pressed key was
  still asked to come up (including after the refusal), and the host's own account names both the
  chord and the key that may still be down.
- `internal/oshost` `TestARunStoppedMidChordThatReleasedEverythingReportsOnlyTheStop` — the
  control, and the one that keeps the case above honest: an implementation that complained about a
  stuck key after every stop would satisfy all four claims while telling people their keyboard is
  broken whenever they press Escape.
- `cmd/marco` `TestAStoppedRunRecordsTheKeyItLeftHeldDown` — the record of a stopped run says
  `cancelled` AND names the key, and says it on the run's own output as well, because a sentence
  that exists only in a file is not one anybody standing there receives. Asserted on what the run
  SAID rather than on the record beside it, since the record's JSON holds the same words.
- `cmd/marco` `TestAStoppedRunThatReleasedEverythingRecordsOnlyThat` — the control: a clean stop
  records nothing beside it, so pressing Escape on a well-behaved routine does not tell somebody
  their keyboard may be stuck.
- `cmd/marco` `TestTheHeldKeyWarningIsTheLastThingTheRunSays` — the ordering, which is what makes
  the sentence the window's: Studio keeps the last line before the outcome word and does not parse
  it, so a warning with anything printed after it is one the window silently replaces.
- `internal/oshost` `heldkeys_test.go` — eight cases over the hold path through the production
  host, each one of the five defects with its own, plus the controls: a hold and its release leave
  the sweep nothing to do, a hold nobody released is released at the end, holds come up in the
  reverse of the order they went down, a refused release is tried again by the sweep (and reports
  nothing when the second attempt works), one that will not come up at all is reported by name,
  one stuck hold does not strand the others, and a stopped run starts no hold. Five mutations,
  five killed, each by the case named for it.
- `internal/oshost` `TestTheHoldPrimitivesDoNotReportAStop` — read off the source, because this
  one cannot be reached any other way: from above the backend, "the machine took it and the run
  was cancelled" and "the machine refused it" are the same error, and the only way to exercise the
  real primitive is to send real input into somebody's desktop. The rule is that the hold
  primitives do not consult the run's context at all — pressing is gated one layer up in
  `doKeyDown`, releasing is gated nowhere.
- `internal/oshost` `holdchain_test.go` — the propagation, through generation → runtime →
  production host, in both failure positions. A refused HOLD sends no press after it; a refused
  RELEASE lets the press inside the hold happen and nothing after it; both are named in
  `Unperformed`. With the generator's two cases removed, the stopping tests fail and so does the
  control, because the control asserts the link shape; with the host's two `delivered` calls
  reverted, the account tests fail and so does the legacy case. Two halves, two mutations, and
  neither half is sufficient alone.
- `internal/oshost` `TestAHoldTheMachineTakesLetsTheRoutineGoOn` /
  `TestALegacyRoutineWhoseHoldFailedCarriesOn` — the control, and the contract that must not
  change: a legacy route attempts its input and carries on, and the account is recorded for both
  while being acted on for one.
- `cmd/marco` `TestARunThatFinishedHoldingAKeyIsNotReportedAsPerformed` /
  `TestARunWhoseSweepReleasedEverythingIsStillPerformed` — the branch every successful run
  reaches. No cancellation, no execution error, a sweep that could not release: `failed`, with the
  key named in the record and on the output. The second is the control that matters most, since
  the same branch answers for every ordinary run.
- `cmd/marco` `TestAKeyTheRouteEndSweepCouldNotReleaseReachesTheRecord` — an ORDERING claim: the
  sweep is a deferred call inside the run, and the account is read after it, so a key only the
  sweep discovered still reaches the record. The stand-in says nothing until its sweep has run, so
  a reader that asked too early would get the empty string.
- `plugins/studio` `TestAStoppedRoutineShowsWhatItLeftHeldDown` /
  `TestAnOrdinaryStopSaysOnlyThatItStopped` — the card, both ways. The window does not understand
  the warning and must not; it shows what the run said.

- `plugins/e2e` `hold_test.go` — the hold scenario's oracle, without a desktop, which is the only
  thing about those two scenarios that any automated run can establish today. The case that
  matters most is the one no other account can see: identical messages in an identical order with
  the system never having applied the key, which delivers a plain press — the harness must notice,
  and the order assertion must NOT fire, or the case is not isolating the reading. Beside it: a
  key that arrived after the press it brackets, a key that never arrived, a hold the application
  never saw released, a press that landed on the wrong control, a modifier somebody was already
  leaning on (which must not be a finding), and a stuck key reported in words that say what to do
  about it.
- `plugins/e2e` `TestTheHoldScenariosAreAskedForByName` — the gate, the prefix, and the rule that
  nothing else wears the flag. Half of it comes off when the scenarios have run green once.
- `plugins/e2e` `TestEveryCountOfWhatArrivedIsTakenAfterTheBarrier` — now over key reads as well
  as press reads, because the absence a key scenario asserts is an absence of KEY messages. The
  two reads that could not cross the barrier — a failure already decided because no run record
  appeared — return PROSE rather than events (`arrivedSoFar`), so nothing taken without the
  barrier can be counted, indexed or compared. Structurally unusable beats remembering not to use
  it.

## Mutations

Four applied to `sendChord`, four killed, each by the test named for it and by no other: the
bare return from the modifier loop, the missing context check after the settle, the discarded
release failures, and the missing context check at the entrance. The first, second and fourth
each failed exactly one test; the third failed three, which is the one place the tests overlap on
purpose — the stuck key, its precedence, and its survival through a stop.

[[ADR-251-a-press-checks-the-window-it-was-recorded-in]] is why a chord in a routine is preceded
by a guarded press. [[ADR-257-a-desktop-harness-that-says-what-it-did-not-prove]] is the harness
this scenario runs in, including the drain barrier the key count crosses.

Eight more for the hold scenarios, eight killed, each by the test named for it and by no other:
the held-state reading never firing, a stuck key reported as nothing, the gate coming off one
scenario, a late key accepted as a hold, `addedTo` ceasing to compare, a key read moved in front
of the drain barrier, the stop scenario's guard on its own absence claim deleted, and that same
guard left in place but moved after the claim it guards.

Two of them taught something. The barrier one found a real gap: that test guarded press reads and
not key reads, so every absence either key scenario asserts was unguarded. And the
guard-deletion one SURVIVED the first attempt, because the tests beside it exercised the
assertion functions and not the chain that calls them — a check nothing calls is a check that is
not there, which is why `TestTheStoppedHoldScenarioGuardsItsAbsenceClaim` reads the chain off the
syntax rather than exercising it.
