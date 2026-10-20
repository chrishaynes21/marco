---
type: adr
status: accepted
date: 2026-10-19
decision: "ACCEPTED — a demonstrated visual routine is executable as ONE complete legal Marco play whose sentences are the order and whose `when ok?` nesting is the gate between them. The play's only door to the world is a narrow act of its own, `Demonstrated's Press`, answered by a host that drives the production coordinator for exactly one interaction and returns ok ONLY for a verified one. It is not the Theater's `Activate`: a Theater Actor returns source and performs nothing, so it cannot ask a person or wait for a verdict, and a sentence that meant `activate whatever control has this name` on one machine and `press the thing somebody demonstrated, with a person agreeing to it` on another would be a file whose meaning depended on who ran it. Which interaction each name means lives in a REQUIRED binding record beside the play — not in the optional provenance sidecar, whose fail-closed direction is `an ordinary authored play` and must not be inherited by a record that decides what gets pressed. Every binding is resolved before any action, and a play edited since its bindings were saved refuses until it is saved again. Director keeps every judgement: fresh grounding, per-action authorisation, bounded verification, the evidence, and the canonical run reference."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - internal/demonstratedmod/demonstrated.marco
  - internal/director/demonstrated/bindings.go
  - internal/director/demonstrated/generate.go
  - internal/director/demonstrated/preflight.go
  - internal/director/demonstrated/supervised.go
  - internal/director/demonstrated/account.go
  - internal/director/demonstrated/store.go
  - internal/director/workflowrun/begin.go
  - cmd/director/demonstratedplay.go
  - cmd/director/observationloop.go
  - cmd/director/performplay.go
  - cmd/director/composeact.go
  - internal/director/runstate/runstate.go
  - plugins/studio/acts.go
  - cmd/marco/demonstratedwiring.go
  - internal/routes/origin.go
  - internal/orchestrator/authority.go
---

# ADR-243 — a door of its own, and the order in the file

Before this, a demonstrated routine's ORDER lived in Go: `workflowrun.Runner` walked a cursor from
the front of the saved artifact, while each individual effect was lowered to legal Marco and
compiled on its way to the machine. Individual effects were legal Marco; the sequence was not.

The order now lives in one saved `.marco` play. Nothing else moved.

An earlier draft of this contract proposed reusing the Theater's `Activate` and adding "one
guarded setter" to the coordinator. The Architect rejected both, and the reasons are in §2 and §5;
the two corrections are most of what this page is about. One word also changed: the brief's
**taught** is **demonstrated** throughout, because TEACH is reserved for Marco guiding a person
and `cmd/marco`'s `TestNoLiveAcquisitionCodeIsNamedTeach` holds that line.

## 1. The words, and the Core declaration each one is

Core's `act`, `scene` and `actor` are settled; `internal/spectest` holds the line. None is
redefined here.

| Product term | Core declaration | Why that one |
|---|---|---|
| **Scene** — one teachable interaction | a `scene` | *"a place in the play. It holds the actors that are there and knows verbs of its own."* An interaction happens somewhere, and the press is the verb it knows. |
| **Act, executable** — an ordered composition | an `actor` plus the `script` | *"a thing in the play… does what it can do."* The composition is a thing that does its verbs in the order a reader reads them. |
| **Act, organisational** — a collection | **nothing** | A collection has no order and performs nothing. Giving it a declaration would give it a second order, and two orders is the defect this page exists to prevent. |
| the way in | the `act` — `use demonstrated.` | *"only an act offers capabilities outwards."* |

The product word **Act** and the Core word `act` are not the same thing, and a generated file
contains exactly one `act` — which is not the Act the product means. Studio should say
**Routine**, so a person never meets two senses of one word.

## 2. The door is its own, and the reason is the Actor interface

The first draft said the Theater's `Activate` already fit. It does not, and the interface says so:

```go
// Cast is the legal Marco this Actor would run to activate one candidate, one way.
//
// # It performs NOTHING
```

`theaterhost.Actor` offers `Find` and `Cast`. `Cast` returns SOURCE; the Theater's Production
boundary runs it through an injected `MarcoRunner`, which is the invariant that every real input
passes a compile gate. An Actor therefore cannot ask anybody for permission, cannot wait for a
verdict, and cannot drive a run — it expresses one way of doing one thing and hands the sentence
back. The supervised runner is the opposite shape: `workflowrun.Runner` owns the whole loop and
*is* the thing doing the casting. It cannot be cast.

And it must not share the sentence either. `Theater's Activate` means *do to this target what
pressing it would do*, resolved by whichever Actor the stage can cast tonight. That is a different
authority — it finds a control BY NAME among whatever is in front, and it is right that it does. A
demonstrated press may only land on the thing somebody pointed at, re-found from that
demonstration's own evidence, with a person agreeing to each one. One sentence meaning either,
depending on who ran the file, would be the worst available outcome: the play reads the same and
what it does to the computer does not.

So the act is a narrow one of its own — the third instance of the `osmod` / `theatermod` pattern,
no new syntax and no new Marco word:

```marco
the Demonstrated is an act.

this exports Press.   // press the demonstrated interaction of this name
```

Two consequences worth stating. There is no Actor roster to fall back to, because the capability
is not the Theater's to answer — which closes "fallback to another Theater provider" structurally.
And `cmd/marco` registers a host for this act that REFUSES with a sentence naming the other door,
because `marco`'s catch-all is the real OS host and would otherwise answer
`OS host has no action "Press"` — true of the OS host, and nothing to do with what the person hit.

## 3. The adapter, and where the engine stops being the Director's business

`internal/director` MAY NOT IMPORT THE ENGINE. `boundary_test.go` walks every import under that
tree and refuses `internal/runtime` and `internal/driver`:

> The Director must not depend on platform or engine code: Marco programs are run through
> directorapi.MarcoRunner, wired in cmd/director.

The first version of this bridge put the `runtime.Host` and the `driver` call in
`internal/director/demonstrated`, and the guard caught it on five files and on a second count —
`TestDirectorHasNoDuplicatePlatformImplementation`, which flags a `runtime.HostCall` in that tree
because a host call skips the parser and compiler. It is right, and the split it forces is the
right one:

| where | what |
|---|---|
| `internal/director/demonstrated` | the POLICY: bindings, preflight, the supervised driver, the account, the outcome. No engine import. |
| `cmd/director/demonstratedplay.go` | everything that knows the engine exists: the act host, the refusing catch-all, running the saved bytes. Where `performworkflow.go` already builds act host maps. |
| `cmd/marco/demonstratedwiring.go` | the standalone runtime answering this act with a refusal. |

The one thing policy needs from the engine is *is this legal Marco*, and it takes it as an
interface — `demonstrated.Compiles` — implemented in one line over `driver.CheckSource`, so the
question a saved play is checked with is the question the runtime asks. A second implementation of
that question is the two-lists-one-fact defect waiting to happen: a spec test was once green while
production refused the same play, because the two were asking different things about one source.

## 3a. Where a bound demonstrated target enters the supervised runner

```
the play       do Demonstrated's Press with "the graphics tab".
runtime        invokeForeign → hostFor("Demonstrated") → demonstratedHost
host           the name → the preflighted binding → interaction 0
Supervised     Begin(0), then Advance per look: ground, authorise, press, verify
back           VerdictVerified → "ok"; anything else → failed, with the sentence
account        one Ask per sentence, under the run's canonical reference
```

`Supervised` is not a second executor. `director perform-workflow` has an observation loop —
acquire the window, capture, propose, observe, build a `Sighting`, call `Runner.Advance` — and
that remains the only one; `Supervised` drives the same call with the same runner, bounded to one
interaction. The looks come from outside: in production that closure is the capture-and-observe
body of `performworkflow.go`, and in the acceptance it is a scripted world that answers each look
from how many presses have actually gone out.

One Runner serves the whole play, so the session binding, the press accounting and the stop state
are continuous across sentences rather than reconstructed between them.

**`ok` is verified and nothing else.** `contradicted` is evidence against the action,
`unconfirmed` is the deadline passing with nothing to go on, and `unasked` is an interaction whose
demonstration recorded no outcome. None is a performed-and-working action, so none returns ok, and
each failure carries the sentence that says which. `runref.Every` is already exactly
`cancelled, refused, verified, unconfirmed, contradicted, failed, started`, so the outcome
vocabulary did not have to be invented.

**And a cancellation reaches the COORDINATOR, not only the runtime.** `HostCall.Ctx` is the
calling frame's context, and there is a window between the run being cancelled and a frame
noticing. One run in roughly six used that window to ground the next interaction and PUT IT TO THE
PERSON — a prompt nobody should have seen, which the run failing a moment later does not excuse.
So `Supervisor` has a `Halt`, the run's cancellation is wired to it, and `Begin` refuses outright
on a stopped run. That is
[[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] again: the last moment a stop
could matter is before the next thing is asked of somebody.

**A run's outcome is read from the account, never from the runtime's error.** The play handles its
own failure in its `or?` arm, so the program completes successfully whether or not the routine
worked. Two defects came of that, and both are in **Enforced by**.

## 4. The order is the file, and the nesting is the gate

```marco
// ark graphics live — what somebody demonstrated, in the order they demonstrated it.
use demonstrated.

the GraphicsTab is a scene.
this can Press.
this's Press does...
    do Demonstrated's Press with "the graphics tab".
    this is that!

the ApplyChanges is a scene.
this can Press.
this's Press does...
    do Demonstrated's Press with "apply changes".
    this is that!

the ArkGraphicsLive is an actor.
this can Run.
this's Run does...
    do GraphicsTab's Press...
        when ok?
            do ApplyChanges's Press...
                when ok?
                    this is ok!
                or?
                    this is that!
        or?
            this is that!

the App is a script.

do ArkGraphicsLive's Run...
    when ok?
        log "ark graphics live: every action verified".
    or?
        log that's error.
```

**A FLAT play is a play that lies**, and this is measured rather than argued. With the two
sentences consecutive, the same host, and the first interaction contradicted:

```
asked:  [the graphics tab, apply changes]
output: [INFO] ark graphics live: every action verified
```

An unhandled failure does not abort a body. So the second control is pressed after the first one
failed, and the run reports success. The nesting is therefore not a style — it is the whole gate,
and `this is that!` inside each scene is what propagates the outcome to it. That is why a
GENERATOR exists rather than a template in a doc comment: the one correct shape is the one nobody
would naturally type, and a flat sequence would be indistinguishable from a working one until an
action failed on somebody's real machine.

`TestAFlatPlayWouldPressTheSecondControlAfterTheFirstFailed` performs that measurement, and it can
only do so with a `Plan` built by hand: `Compose` writes the nested form and `Preflight` compares
the play's digest against the record written with it, so the flat form cannot reach production.

## 5. The binding record is REQUIRED, and it is not provenance

Three files, and only one of them may be missing:

```
<slug>.marco          the play. The order, and the continuation.
<slug>.scenes.json    the BINDINGS. Which demonstrated interaction each name means. REQUIRED.
<slug>.origin.json    provenance. OPTIONAL — the existing sidecar.
```

Keeping the last two apart is the Architect's second correction, and the draft that merged them
had inherited the wrong failure direction. `routes.Origin` is allowed to be absent: a `.marco`
with no sidecar is an ordinary authored play, and that is the right fail-closed direction for
PROVENANCE because the surviving state claims nothing. Bindings are not provenance. They decide
which control a sentence presses, and a missing or stale one must not degrade into "an ordinary
play somebody wrote" — it must stop the run.

```json
{
  "version": 1,
  "source": "2b9e4d7c1a05f863",
  "application": "ArkAscended.exe",
  "scenes": [
    {"name": "the graphics tab", "routine": "ark graphics live", "revision": "9f3c…", "position": 0},
    {"name": "apply changes",    "routine": "ark graphics live", "revision": "9f3c…", "position": 1}
  ],
  "saved_at": "2026-10-19T22:14:03Z"
}
```

**An ordered list, and the order is the generator's input rather than an execution order.** An
earlier version kept a map, on the reasoning that a JSON object has no order to disagree with the
play's about. It also had no order to REBUILD from, which is why its completeness check had to be
inferred — and the inference was wrong. Nothing at run time consults this order: the host resolves
a name, and which sentence comes next is decided by the file's own nesting. So the two cannot
compete, because a record whose order disagrees with the file rebuilds to different bytes, and
different bytes are a refusal rather than a choice between two accounts of what to do
(`TestAReorderedRecordIsRefusedRatherThanObeyed`).

`Bindings.Source` is the digest of the play the record was written with, so one comparison answers
*are these the bindings for this file*.

**It does not answer whether the record is still complete, and an earlier version of this page
claimed it did.** The reasoning was that the play is generated from the bindings, so a matching
file must have every name bound. The hole: the digest is over the SOURCE, and the record is a
separate file. Remove the second binding, leave the digest alone, and preflight accepted the pair
— because it validated only the bindings that were present. Interaction one would then be
grounded, authorised and **pressed**, and interaction two refused as unbound afterwards: real
input, on somebody's computer, in service of a run that could never finish, from a bridge whose
claim is that missing metadata refuses before any input.

So completeness is established by **rebuilding the play from the record and comparing bytes**.
`Preflight` hands the loaded routine and the record's interactions, in order, back to `Compose` —
the one generator — and requires the result to equal the file. Equality establishes four things at
once, none of them inferred:

- every name the play asks for is bound, because the generator wrote the play **from** the
  bindings;
- every binding is asked for, for the same reason;
- the gate is there, because the generator writes the nested form;
- the order the file performs is the order the record describes.

And **inequality is a refusal, not a diagnosis.** A play Marco did not generate is not supported —
not analysed, not partially accepted. The supported form is enforced rather than inspected.

### The attempt in between, and why it was wrong

The first correction ran the play with a host that recorded names and performed nothing, sweeping
prefixes: answer ok to the first *k* asks, fail the rest. Sound for the generated shape, and the
Architect's counterexample shows it is not analysis of arbitrary Marco — ask A, ask B when A
**fails**, ask C when B **succeeds**:

```
k = 0   A fails, B fails        → C never reached
k ≥ 1   A succeeds              → the fallback is never entered
```

C is invisible at every *k*. The checker would have returned `{A, B}` and accepted a record with C
unbound, and execution could then have pressed B before discovering it. A finite set of simulated
runs is not an exhaustive account of a program's reachable calls, and treating it as one was the
mistake — the same mistake as the digest, one level up: trusting a cheap check to say more than it
says. `TestAPlayWithAFallbackThatLeadsOnIsRefusedRatherThanPartlyUnderstood` is that exact play,
refused by name.

Two consequences of dropping the probe are worth noting because they were P2s that no longer
exist: there is no pass bound to exhaust, and no probe execution to bound with a context.

That is also the resolution of the contradiction the Architect named. The draft claimed both
"reordering works" and "every edit invalidates the digest". Both are true and they are not in
conflict once the sequence is stated: **reordering works through an explicit save.** Editing the
play makes its bindings stale, which refuses the run and says so; saving it again revalidates and
writes the record for the sentences it names now. Reordering behind Marco's back does not work,
and that is the point.

**Preflight resolves the whole play before any action**, because a play whose SECOND sentence named
an unbound interaction would otherwise press the first control and then stop — one real press in
service of a run that was never going to finish. It checks the record's version, that there is at
least one binding, that the source compiles through the production checker, that the digests
agree, that one routine is pinned to one revision, that the routine is saved and is still that
revision, and that every position exists. It deliberately does not ask whether the screen is
right or whether anybody will agree to the press: those need a look, and they are asked per action
at the moment it is about to happen.

**The revision is a content digest, not `SavedAt`.** `SavedAt` does not move when a person corrects
a routine — it is the link to the raw demonstration beside it — and `CorrectedAt` moves instead, so
a timestamp revision would call a corrected procedure the same revision as the one it replaced.

**Execution runs the validated bytes, and there is no path to reopen.** The first version ran the
play BY PATH, so the runtime reread the file — and an edit or a replacement between preflight and
execution reached it unchecked. It would be compiled, which establishes that it is legal Marco and
nothing else: not that it is the play whose bindings were resolved, and not that it still has the
nested continuation that is the only gate between one action and the next. A replacement can be
legal Marco with two consecutive sentences and no gate at all.

So the bytes are read once, checked, and carried on the `Plan`. Nothing downstream can open a
file. The import directory is EMPTY as well, which buys a second thing: with a directory, `use
demonstrated.` resolves a sibling `demonstrated.marco` ahead of the built-in act surface, so a
file dropped next to a play could redefine the act it is performed through while the play read
exactly the same.

**Three writes, and the order is the care.** Bindings, then the play, then the provenance
(`SaveWithOrigin` writes the last two in that order itself). Backwards, a crash would leave
bindings describing a play that does not exist, or provenance vouching for one that cannot run.
Forwards, every surviving state is either *not there yet* or *an ordinary file somebody could
read* — the same asymmetry ADR-028 chose, for the same reason.

**And nothing is overwritten.** A name already in use is refused and Studio asks the person for
another, because replacing a play somebody has is a decision only they can make. The precedent is
`demo.Store.SaveSequence`, which refuses when two names would land in one file rather than
replacing a routine with a differently-capitalised one; the rule here is stricter because a play
is executable. Orphaned bindings — left by a crash or by a delete that took the source and not the
companion — are refused rather than reused, because they describe sentences the new play may not
have.

## 6. The single-interaction execution contract

`Advance` walks a cursor that only ever increments. A host answering one sentence must be able to
say WHICH interaction it is answering, or a reordered play would silently perform the artifact's
original order.

The Architect refused "one guarded setter", and was right: the cursor is not the only state an
interaction leaves behind. `Runner.Begin(step)` is therefore an explicit contract.

**Four refusals**, each a guarantee that already exists, asked from the new door:

1. **an unknown interaction** — out of range, nothing to perform;
2. **a pending verification** — the step the walk is on has an outcome still being decided. This
   is [[ADR-242-the-second-action-waits-for-the-first-ones-answer]]. A caller that could re-aim
   past it would have the second press reaching the machine while the first verdict was still
   forming — ADR-242's defect, reopened through a different door;
3. **a press in flight** — a press has gone out whose destination is not established; beginning
   something else would abandon it mid-air and report nothing about it;
4. **a stopped run** — cancellation is a floor, not a suggestion.

**And a stated division of state.** Reset: `cursor`, `acted`, `looks` — they describe the
interaction being started. Kept: `sent` and `budget` (the press accounting), `attempts`,
`verifications`, `reads`, `repairs`, `regrounded`, `stopped` — they describe the RUN, and the run
is still the same run. `reads` being kept is load-bearing rather than tidy: an interaction with an
outcome expectation will not be acted on until the screen it stands on has been read, and those
readings arrived before this interaction was asked for.

A budget reset per interaction would be the sharp edge here — a bounded supervised run made
unbounded by arithmetic rather than by decision — and
`TestABudgetSurvivesBeginningAnotherInteraction` holds it.

**Reordering may refuse, and must never silently perform the old order.** On the three-screen
routine, interaction 1 belongs to a screen that only exists after interaction 0 has happened, so a
play asking for it first is standing somewhere it cannot act. The honest answer is nothing: no
press, nobody asked, and a sentence saying why.
`TestAReorderingThatCannotBePerformedRefusesRatherThanPerformingTheOldOrder`.

## 7. Director's half, which did not move

| Judgement | Who owns it, unchanged |
|---|---|
| Fresh visual grounding | `performworkflow.go` — `freshLook`, `freshFrom`, `worldMoved`'s eight refusals, the bounded boundary read, `promptGrounder.Establish`, the person-confirmed screen correspondence |
| Per-action authorisation | `workflowrun.Granter.Authorise` and the consent desk's F7/F8/F9. One press, once |
| Bounded verification | `workflowrun/verify.go` — verified / contradicted / unconfirmed, bounded by the verification deadline |
| The second action waiting | [[ADR-242-the-second-action-waits-for-the-first-ones-answer]], which is the Go half of the rule the nesting expresses in source |
| Cancellation | `HostCall.Ctx`, checked at the top of every look; `stopsignal` across processes |
| Canonical run identity | `internal/director/runref`, carried onto every record of the run |

## 8. Four words, defined

- **demonstrated** — a person did it with their own hands while Marco watched. Evidence exists. It
  licenses nothing.
- **saved** — the interpretation is durable: artifact plus linked evidence. Still licenses nothing.
- **runnable** — a complete play exists, it compiles, and every name it uses resolves to a saved
  revision that is present and whose digest matches. Compilation is not permission.
- **verified** — one interaction's demonstrated outcome stood in fresh evidence after the press,
  inside the deadline. Per interaction, never per routine; a routine is verified only if every
  interaction was, which the nesting makes true of the file rather than of a report.

## 9. The Architect's dispositions, as implemented

- **Provenance kind** — `routes.KindDemonstrated`, a fourth kind, truthful rather than borrowed:
  `learned` means *rehearsed and verified* and this was neither. Consumers updated explicitly:
  `plays.KindWord` / `KindSays` present it as itself, and `orchestrator`'s authority door now asks
  about it through `composedKind` — because the reason a learned play is put to the person is that
  **they have never read the file**, which is true word for word of a demonstrated one. Without a
  case of its own it fell through to `Allowed`/`ReasonOrdinary`: treated as the person's own
  writing because nothing named it.
- **Storage and naming** — route storage, three writes in the safe order, and a name already in
  use REFUSED rather than replaced. Studio chooses an explicit identity; nothing is overwritten
  or silently shadowed, and orphaned bindings are refused rather than reused. See §5.
- **Legacy routines** — no automatic naming or conversion. `director perform-workflow` runs them
  exactly as it does today.
- **Scene sharing** — explicit references pinned to a revision, and a record drawing on more than
  one demonstration is REFUSED with the sentence, because the adapter holds one coordinator over
  one artifact. Broader composition is not advertised until it is performed.

## 10. What this does not do

- **No second format and no second executor.** No new Marco word, no `.marco` dialect, and
  `perform-workflow` is untouched.
- **No coordinate in source.** The demonstrated coordinate stays in the evidence record.
- **A routine whose demonstration recorded NO outcome cannot be composed into a play that runs
  past its first interaction.** `ok` means verified, and such an interaction can never be
  verified. ADR-242 keeps it advancing on the old path on purpose; here the play stops with an
  honest sentence. The cost is stated rather than hidden, and
  `TestAnInteractionWithNoRecordedOutcomeDoesNotCarryThePlayForward` pins it.
- **No Studio run wiring**, no widget change, clicks only.
- **No live acceptance.** Everything here is deterministic and automated, with an instrumented
  effect host; no test in the new package emits desktop input. The owner's live trial is separate
  and this page claims no part of it.

## Enforced by

`cmd/director/demonstratedplay_acceptance_test.go`, over the production coordinator, the
production verifier, the production executor, the production runtime reading the play off disk,
and a notebook where the mouse would be — so no test here emits desktop input. Plus
`demonstratedplay_flat_test.go`, `internal/director/demonstrated/account_test.go`,
`internal/director/demonstrated/store_test.go`, `internal/director/workflowrun/begin_test.go` and
`cmd/director/demonstratedplay_sealed_test.go` and `cmd/marco/demonstratedwiring_test.go`.

Three of the command-level tests — `TestAStalledPlayEndsAtItsDeadline`,
`TestThePlaysRecordIsWrittenFromItsAccount` and `TestAnAnswerAfterTheDeadlineAuthorisesNothing` —
drive the production `--play` assembly past runner
construction against the window already in front, dry, with only the start key answered. They SKIP
when no foreground window can be photographed, which is the same limit
`TestAHotkeyRunReachesAnAuthorisedActionWithoutFrom` has and for the same reason: nothing can be
grounded on a window that does not lead the desktop, and bringing one forward from a test is
taking somebody's focus. Both ran and both mutations killed on a machine where a window was
readable; a run where they skip proves nothing, which is why the inconclusive result above is
reported rather than averaged away. The mechanism the last of them is about — a key delivered
after the run's deadline granting no authorisation — is ALSO held deterministically, with no
window and no real clock, by `consentexpiry_test.go` over the production granter and the
production desk. That pairing is deliberate: the skippable test proves the wiring on a real
desktop, and the deterministic one proves the refusal everywhere.

`demonstratedplay_sealed_test.go` holds the fail-closed cases specifically: a binding removed, a
binding renamed, an extra binding, a reordered record, two bindings of one name, a play with a
fallback that leads on, a plan with no validated bytes, the file replaced after preflight, and a
sibling act surface beside the play. Each asserts **zero groundings, zero authorization requests
and zero effects** — not merely that it failed.

The eight required acceptances: save/reload/compile; the saved order controlling invocation order,
and a reordered play reordering the run; the second interaction asking for neither authorisation
nor input until the first verifies; a contradicted first interaction never reaching the second;
cancellation blocking what follows while preserving the prior accounting; seven refusal cases
before any input; repetition re-grounding and re-authorising; and one canonical run reference on
every record. Each asserts whether the second interaction was **asked about**, not only how many
presses went out.

**Fifty-two distinct mutations, forty-five killed** (eleven were re-run against a better test after a
first survival, which is where seven of the tests here came from). The five survivors are named
below, and one result is INCONCLUSIVE rather than a kill or a survival: the play branch's call to
`Account.Cut` is pinned only by `TestThePlaysRecordIsWrittenFromItsAccount`, which SKIPS when no
foreground window can be read — as it did when that mutation was run. `Account.Short` carrying the
reason is pinned deterministically in the policy package; the branch calling it is not. Said here
rather than counted as a kill.

- **The three interaction-state resets** — `Begin`'s of `acted` and of `looks`, and `advance`'s of
  `looks`. Removing any one leaves every test green and so does removing all of them: `acted` is
  always already `-1` where `Begin`'s guard permits, and the two `looks` resets cover each other
  on every path that finishes an interaction. Kept, with the finding recorded beside the code the
  way `verify.go` keeps its own structural clause — a reader deserves to know which of `Begin`'s
  three assignments is load-bearing today, and it is the cursor.
- **`Begin`'s stopped guard measured against the integration test**, which survives it because
  `Advance` refuses a stopped run itself. The guard is pinned directly by
  `TestBeginRefusesAfterTheRunIsStopped`; what survived is the integration test's power, not the
  guard.
- **`Supervised`'s stop check**, which nothing pins. What it buys is that a stopped run stops
  consuming looks rather than spending its patience to reach the same answer.

Two earlier survivors are gone because the clauses they covered were **deleted rather than
documented**: `Preflight`'s asks-for-nothing check and its duplicate-name check, both of which the
rebuild refuses with the same sentence or better.

**Three holes were found by the Architect in the fail-closed claim**, and all three are one mistake
in different places — trusting a cheap check to say more than it says:

1. **Partial bindings passed preflight.** A record can lose or rename a binding while the source
   stays byte for byte what it was, and only the bindings present were validated. Action one
   would have been pressed and action two refused afterwards. Fixed by establishing completeness
   from the play; `TestABindingMissingForALaterInteractionRefusesBeforeAnyInput` is the evidence,
   with the digest deliberately left matching in all three cases.
2. **Execution reopened the file after validating different bytes.** Fixed by carrying the
   snapshot on the `Plan` and removing the path entirely;
   `TestReplacingThePlayAfterPreflightDoesNotChangeWhatRuns` replaces the file with legal Marco
   that has no gate and asserts the validated play's gate still held.
3. **The first fix for (1) was a probe, and a probe cannot be exhaustive.** Prefix sweeping never
   reaches an interaction behind a SUCCESSFUL fallback. Fixed by enforcing the generated form
   instead of analysing arbitrary source — see §5.

**Three defects were found by the acceptance rather than by review**, and each is the shape this
product exists to refuse:

1. **A run's outcome cannot be read off the runtime's error.** The play handles its own failure in
   its `or?` arm, so the program completes successfully either way. The first draft asserted on
   the error and passed four tests it should have failed. `Account.Outcome` exists because of it.
2. **`Outcome` read a refusal before a verdict**, reporting every contradicted press as "refused"
   — which says Marco pressed nothing, about a run that pressed.
3. **`verified` about half a routine.** A run cancelled as its first interaction finished never
   reaches the second sentence, so every record said verified and the outcome agreed. Found by a
   test that failed one run in six; `Account.Cut` is the fix, and
   `TestARunCancelledBeforeItStartedIsReportedAsCancelled` pins it without the race.

And one found the same way, in the product rather than in the report: **a cancelled run still
authorised the next interaction**, through the window between the run's context and a frame's.
See §3a.

## Composing one, which nothing could do

`--play` selects a saved Act; nothing SAVED one. `Compose` and `Save` had only test callers, so
the path approved for live acceptance could not be reached by a person at all — there was no way
to get an Act onto a machine. `director compose-act` is that way:

```sh
director compose-act --from "ark graphics live" --name "ark audio by name" \n    --interaction '2=the audio tab'
```

It takes a demonstration somebody already gave, the interactions of it they want performed, in the
order they want them, and the name they want to call the result. The flag order IS the run order,
which is what makes a subset or a reordering expressible. It writes the play, its bindings and its
provenance, presses nothing, and then PRINTS THE PLAY — because the whole reason an Act is legal
Marco is that a person can read what they just saved and recognise it.

Which exposed one more thing worth fixing: the generated play used the DEMONSTRATION's name
throughout, so an Act called "ark audio by name" opened with a comment about "ark graphics live"
and ended by logging that "ark graphics live" had verified. Both true, and neither about the file
somebody is reading. The Act now carries its own name in its header, its actor and its final line,
with the demonstration named as where the interactions came from — and `Bindings.Act` stores it,
because the rebuild that establishes the record has to be able to reproduce the file exactly.

## The named entry, and the one thing it changes

`director perform-workflow --play "<name>"` is the production path: select a saved Act, preflight
it, perform it under supervision, cite the run. A FLAG rather than a command of its own, and that
is the point — everything between choosing what to perform and pressing something is the same
assembly: the window is acquired the same way, the scope gate is the same gate, the screens are
named by the same person, each press is authorised at the same desk, the pointer and hover probes
ask the same questions, the stop key reaches the same runner, and the run is cited by the same
reference. A second command would have been a second place where all of that was assembled, and a
second place is how two answers to "may this be pressed" come to exist.

What differs is one thing: **where the order comes from.** A workflow's order is the artifact's and
the cursor walks it; a play's order is its own sentences' and the coordinator is told which
interaction each one means.

Two pieces of plumbing were needed and both are seams rather than copies:

- **`looker`** (`cmd/director/observationloop.go`) is the observation loop, moved verbatim out of
  `runPerformWorkflow` with the state it carried between iterations turned into fields. A second
  caller arrived and there must not be a second loop: copying a body that acquires, captures,
  proposes, reacquires-before-the-fold, observes, arranges and admits readings would have been two
  copies of ordering that is several ADRs deep. The wiring guards that pin those lines by source
  text now read the whole assembly rather than one file, so a MOVE is not reported as a deletion.
- **`demonstrated.Watch`** is what the cursor loop does between looks, handed to the play driver.
  It exists for one line in particular: *a step nobody has agreed to is a question, not a
  refusal.* That question is deliberately outside the coordinator, because agreeing that a step
  belongs in this run and permitting a press are two different sentences — so the play driver asks
  it in the same place rather than growing a second prompt.

## Three integration defects, and where they were

The mechanism was approved and the CALLER was wrong in three ways. All three are the same shape:
the play path inheriting an assembly built for a cursor, and not inheriting one of its rules.

1. **`--step` could change which interaction a Scene performed.** `--step` extracts one step into a
   one-step artifact in memory, renumbering it to position 0 — and a play's bindings name ORIGINAL
   positions, which nothing renumbers. So `--play X --step 1` left the play's first Scene bound to
   position 0 while position 0 now held what used to be interaction 1: the sentence a person reads
   would press the other control, with every gate satisfied and nothing to notice. REFUSED on
   usage now, because an Act already says which interactions it performs and narrowing it with a
   flag is two things deciding one question.
2. **The canonical result ignored the play's account.** The branch printed `acc.Short()` and then
   classified from `rep.Progress.Complete()` — the artifact cursor. An Act that is a subset of a
   longer demonstration, or its interactions reordered, verified everything it asked for and was
   recorded REFUSED and exited 1. `outcomeOfAct` takes completion from the account and leaves every
   per-action count where it was, and the record now carries the Act's name and its source digest
   beside the demonstration — an audit needs both, or two Acts over one demonstration cannot be
   told apart.
3. **`--seconds` did not bound a play.** The deadline was the cursor loop's own condition, which
   the play driver never saw: an observation count is not a wall clock. It moved into
   `looker.within`, where the looks are produced, so both drivers inherit it.

   **And that was still not enough.** Checking the clock before each look stops the loop and stops
   nothing already WAITING. The press-authorisation prompt is the case: it opens before expiry,
   the desk waits its full `--answer`, and a key pressed after the limit minted a FRESH
   authorisation — so the click could still be sent, past the time the run advertised, with every
   other guard satisfied. None of the runner's boundary checks asks what time it is.

   So the deadline is in the CONTEXT, which is what every wait is already given:
   `consentDesk.ask` selects on `ctx.Done()` and WITHDRAWS its question, the capture takes it, the
   coordinator takes it per observation, the verification windows close with it. One deadline,
   inherited, rather than a clock each of them would have to remember to consult.

   **And expiry is not a cancellation.** `context.WithDeadline` says which — `DeadlineExceeded`
   against `Canceled` — and the two mean opposite things to a person: somebody chose to end this,
   against Marco used the time it was given. `endedBecause` keeps them apart into the record.
   Expiry deliberately gets no outcome word of its own: the shared vocabulary has seven and
   inventing an eighth would be a word every future reader pays for, to say what the per-action
   counts already say truthfully. What it gets is the REASON, in the run's own line and in the
   record's `because`.

   Two layers lost that reason on the way down and both had to be told. `HostCall.Ctx` is the
   calling FRAME's context, and the runtime cancels the frame tree with `Canceled` whatever
   happened upstream — so the supervisor is given the run's context too
   (`Supervised.WithRunContext`) and asks it first. And `Halt` raises the coordinator's stop flag
   for both endings, so `OutcomeStopped` cannot be read as a cancellation without asking the
   context which it was.

## Studio, as a vertical slice

`plugins/studio/acts.go` is the Act surface — compose, select, prepare, run, cancel, inspect —
and it is over the SAME functions the command line uses rather than a second implementation of
any of them: `demonstrated.Compose` and `Save` to compose, `routes.Registry` with
`demonstrated.Load` and `Preflight` to list and say whether each Act would run, `demo.Store` for
the demonstrations and their steps, `stopsignal.Raise` to cancel, and the run record to inspect.

Running is a CHILD PROCESS: `director perform-workflow --play …`, one pipe for both streams,
`runref.Parse` over the lines. Because a supervised run needs a person at the consent desk, and
the desk reads F7/F8/F9 from a global keyboard hook and reports on a transcript. Reaching into
that from a window would be a second place where the observer, the grounder, the granter, the
executor and the stop key are assembled, and a second place is how two answers to "may this be
pressed" come to exist. So Studio spawns it and shows what it says — which is what `cmd/marco`
already does for a demonstrated name.

Two things the surface makes visible per interaction, because they decide whether an Act will do
anything and a person should see them while composing rather than when a run stops: whether the
step PRESSES something, and whether its demonstration recorded an OUTCOME — without one the
interaction can never be verified, so an Act cannot continue past it.

`studio -acts <store>` prints the whole surface headlessly: it opens no window, spawns no runner
and presses nothing, so it can be pointed at a copy of somebody's store before a window is ever
put on their desktop.

### The run's state is the run's, not read out of its prose

Studio first DERIVED what was happening from the transcript, matching on sentences like "May
Marco". Two defects in one coat. A window whose behaviour depends on the exact wording of a
consent prompt breaks when somebody improves the wording — silently, and in the direction of
showing a person the wrong thing about a press they are about to authorise. And prose is not
authoritative: the sentence is printed NEAR the decision, not BY it, so a reader of prose is
always inferring a state that something else already knew for certain.

`internal/director/runstate` is the run's own account, emitted at the points it decides something
— the start wait beginning, a question going up, the same question coming down answered or
withdrawn, an action going out, a verdict arriving, the run ending — and `--events <path>` asks
for it. The ordinary terminal output does not change: these are a second account of the same
decisions, not a replacement for the one a person reads.

SNAPSHOTS rather than deltas, so a front end renders the latest and is correct by construction: a
question that has been answered is absent from the event after it, and a stale prompt — the
likeliest thing for a person to answer twice — cannot survive on screen. The words a person reads
are carried INSIDE the event for the transcript and are never parsed back out of it, which is the
property `TestRewordingAQuestionChangesNothingStudioDoes` holds: the same question in two
completely different sentences, and everything the window decides from is identical.

The preparation deadline is the runner's. Studio counts down to that instant and shows what it
computes; preparation is over when the run says it is, because a window acting on its own
arithmetic would be deciding something it is not in a position to know.

And a run that exits without a FINAL event is INCOMPLETE rather than failed. A refusal is an
answer and says so in a final event — which is why every early exit emits one too. A child that
vanished leaves a stream with no end to it, and the honest thing to show a person is that nobody
can speak for what happened.

The phases are not `pkg/playbill`'s, and that is a choice rather than an oversight: playbill is
the account the SERVICE gives of a command, and this is the account a SUPERVISED RUN gives of an
Act. playbill has no "which screen are you on", and a supervised run's permission is asked once
per press. Reusing that type would have meant adding words to it the service never emits. What IS
reused is `runref.Outcome`, the seven words both binaries already share.

## What is still unproven

The path is connected and refuses before input, and that is all the deterministic tests can say.
`--play` has never performed a press on a real screen, and no Act has been run through Studio by
a person. The owner's trial is what settles both. The widget stays frozen.

And one thing about the structured events is proven only in parts: the EMISSION is tested in
`cmd/director` and the READING in `plugins/studio`, each against the other's format, but no test
watches a real supervised run drive a real window. The seam between them is a file, which is the
thing a test can stand on either side of and a person cannot.

The Wails prototype under `plugins/studio/` remains SHELL EVIDENCE. Its store defaults to an
isolated one per launch and it refuses to attach to a service it did not start, which closes the
launch defect; it establishes nothing about supervised consent keys, and its own evidence says so.

## Links

- [[ADR-242-the-second-action-waits-for-the-first-ones-answer]]
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]]
- [[ADR-068-the-theater-is-the-durable-semantic-world]]
- [[ADR-028-a-learned-play-is-a-file-with-a-past]]
- [[ADR-181-a-demonstration-marco-can-keep]]
- [[ADR-048-learn-teach-and-do-are-three-different-sentences]]
- [[Marco-Boundary]]
- `spec/Core.md` — the four nouns, the four endings, the governance rules
- `spec/Hosts.md` — the host-FFI boundary and custom status words
