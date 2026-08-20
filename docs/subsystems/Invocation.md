---
type: subsystem
status: active
owners:
  - marco
depends_on:
  - plays
used_by:
  - service
  - visibility
  - learned-plays
updated: 2026-08-20
source_paths:
  - internal/invoke
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - cmd/marco/bind.go
  - cmd/marco/edit.go
  - internal/director/intent/clarify.go
  - plugins/overlay/intake.go
  - plugins/overlay/outcome.go
---

# Invocation

What happens between the Audience asking for something and something happening. One decision,
made once, in one place — whichever way the words arrived.

This note is about the DOOR. [[Plays]] is about what is behind it and [[Learned-Plays]] is about
how one kind of thing behind it came to exist; read those for the artifact, and this for how a
request finds one.

## The canonical rule

> **If Marco already knows exactly what the Audience means, it uses the durable thing it knows.
> Otherwise Director works out what they mean from the live world.**

Both halves are load-bearing, and each is the reason the other is safe.

*Knows exactly* is what makes Marco work with no model, offline, on a machine with nothing else
running: a Play the person named answers to that name, deterministically, forever. It is why *"the
phrase I saved it under runs the thing I saved"* is a promise rather than a probability.

*Works out from the live world* is what stops the deterministic tier having to guess. Because a
miss is handed to something that can SEE and can ASK, the lookup never has to be generous — and a
lookup that never has to be generous can be exact.

## Source invariance

**Typing and speaking are transport differences, not semantic differences.** The same words mean
the same thing however they arrive; a microphone is not a second vocabulary.

`invoke.Request.Source` exists so a trace can say how intent arrived, and **nothing in `Decide`
reads it**. `TestSourceCannotChangeTheDecision` puts every source through every case and asserts
one verdict per case, so an arm that started consulting it fails immediately.

The overlay holds the same line on its side of the process boundary: `intakeArgs` builds one argv
for both, and `--source=` is the only thing that varies (`TestTypedAndSpokenDifferOnlyBySource`,
`TestTheSourceWordsAreTheEngines`).

### What it cost while transport DID decide meaning

`acts.go` had one `if`. A phrase that arrived as `RunVoice` ran `marco director <phrase>`; a
phrase that arrived as `Run` ran `marco do <phrase>`. So:

- a Play the person had demonstrated, registered and could run by typing **was not found by saying
  its name** — the spoken path never consulted the registry at all;
- a request Director could have satisfied became **"shall I learn this?"** the moment it was typed
  instead of said;
- and the two paths shared no code, so nothing could hold them level.

This was [[34F-legacy-marco-product-audit]] §13's *"the real gap is above all of them"*, and §1.5's
single largest contributor to *weird UX*.

## Explicit identity

**A clicked Play or a resolved Binding STAYS that Play.** It is never converted back into words
and guessed at again.

`Play.Name` is the slug with its dashes back as spaces, and that transformation is lossy in the
direction that matters: `routes.Slug` folds case, punctuation and runs of whitespace, so
re-deriving a slug from a shown name can land on a different file or on none. A surface that
already holds an identity hands the identity over — `invoke.Request.Play` — and `Decide` returns
it without reading `Request.Text` at all.

- The control centre's Run posts `{slug, app, scope}` and spawns `do --source=control-centre
  --play=<slug> [--app=…] [--focus]` (`TestAClickedRunSpawnsAnExplicitIdentity`,
  `TestARunSurvivesANameThatDoesNotRoundTripThroughItsSlug`, `TestTheRunArgvSpellsEachScope`).
- `marco hotkey` resolves its binding to a `routes.Route` **once**, in `bind.go`, and hands that
  Route down. It used to put its own command text back through the phrase matcher on every press,
  so a hotkey bound to one Play could quietly start running a different one as the registry filled
  up (`TestAHotkeyPerformsTheBoundPlayItself`).

`invoke.Plays` is a one-method interface partly for exactly this reason: a test can hand in a
resolver that records every call and assert it was **not** asked
(`TestAnExplicitPlayIsNeverLookedUpAgain`).

Arguments follow identity. A Binding carries its own (`` `e = enter freeplay with 3 ``) and a
clicked Run carries none; only a phrase has to be read for them. Re-reading a phrase that carried
none, over cargo that did, would silently drop arguments somebody supplied.

## The precedence — five arms, in this order

`invoke.Decide(plays, Request) Decision` is pure. It reads a resolver and returns a verdict; it
performs nothing, connects to nothing, and cannot be told what to decide by the surface asking.

| # | arm | fires when | why it sits here |
|---|---|---|---|
| 1 | **control phrase** | `intent.IsControlPhrase(text)` | it acts on what is ALREADY running, so nothing may look at it first |
| 2 | **pending question** | `Request.Pending` | Director asked something; these words are the answer |
| 3 | **explicit identity** | `Request.Play != nil` | the surface already knows; the words are not read |
| 4 | **exact durable match** | `plays.Resolve(app, text)` succeeds | Marco already knows precisely this |
| 5 | **Director** | everything else | Marco does not know it, so it is a question about the world |

**1 — control, always first, from every entrance.** "stop" said while a play is running has to
reach the thing that is running before anything else looks at it. A "stop" that went to Play
lookup would find nothing, miss, and be offered as something to *learn* — which is exactly what
typing it used to do. `TestStopIsNeverOrdinaryText`.

**2 — an answer belongs to its question, ahead of Play matching on purpose.** While Director is
waiting to be told which one, a phrase that happens to name a Play is still an answer. Claiming it
at arm 4 would silently discard the question and start something else.
`TestAnAnswerToAPendingQuestionIsNotAPlay`, and in production
`TestAnAnswerIsNotClaimedByAPlayOfTheSameName`.

The intake asks the service whether a question is outstanding, and asks **without starting
anything**: a Director that is not running has no pending question, so an outage costs one failed
dial rather than a twenty-second service start on every command.

**3 — explicit identity before the words**, for the reason above. It is consulted only when there
are words to claim: an explicit identity is not an answer to a question, so `Pending` is not even
asked for one.

**4 — exact, and never fuzzy.** `Resolve` applies `routes.Slug`, which already folds case,
punctuation, surrounding quotes and runs of whitespace onto one durable identity — the
normalization the product wanted was there all along and nothing called it first
(`TestNormalizationIsTheSameFromEveryEntrance`, `TestAnApostropheDoesNotHideAPlay`). A staged Play
is structurally unable to answer here, because `Resolve` walks `List` and `List` does not scan
`<app>/learned/` — [[ADR-028-a-learned-play-is-a-file-with-a-past]], held by
`TestAStagedPlayDoesNotInterceptAnInvocation` and, in production,
`TestAStagedPlayNeverInterceptsFromAnyEntrance`.

The **whole phrase** is tried before any invocation grammar is applied to it, so a Play called
"wait then click" is reachable by its own name instead of being split into two commands neither of
which exists. The grammar (`ParseInvocation`, `SplitChain`) gets its turn afterwards, in
`cmd/marco/intake.go`'s `grammar`, and a chain is only a chain when EVERY step names a Play —
otherwise "turn the lights on then make coffee" is one request for Director rather than two offers
to record a demonstration. `TestAPlayNamedWithTheGrammarIsStillReachable`.

**5 — Director gets the words as spoken**, trimmed and no more. Director reads sentences, and
rewriting one changes its meaning. `TestDirectorReceivesTheWordsAsSpoken`.

### Resolution is still not permission

Deciding which Play the Audience means is not permission to perform it. Every invocation crosses
`orchestrator.Authorize` afterwards — whichever entrance it came from, and whether the identity
arrived as words or as a click. A clicked Run is still asked about.
[[ADR-029-resolution-is-not-permission]], held by `TestExactPlayMatchDoesNotBypassAuthority`,
which also asserts there is exactly **one** `orchestrator.Authorize(` call in the intake.

## The cancellation exception

A control phrase may be recognised **locally, for immediacy, and for nothing else**.

`plugins/overlay` matches `intent.IsControlPhrase` itself and kills the child it spawned the
moment the word lands, instead of after a process spawn and a socket dial. The phrase then still
goes through the intake, because that is what reaches the **active execution authority** — the
Director, which is doing the work for a learned Play and which deliberately treats a dropped
client as *"not a cancellation; the work continues"* (correct: a front end that crashed must not
abort a replay).

So the local half **decides nothing**. It cannot route, refuse or consume the phrase. Killing the
child alone made the HUD go quiet while the desktop carried on being driven, with nothing left on
screen offering to stop it — which is why `cancelRun` also spawns `marco director stop` detached
(`TestCancelAlsoStopsTheDirector`, `TestASpokenStopKillsTheChildImmediately`).

**One definition of the word, shared.** `intent.IsControlPhrase` is used by `invoke.Decide`, by
the overlay, and by the Director service's own phrase routing. A second list anywhere would be a
second answer to *"did they say stop"*, and the two would drift the first time somebody added a
word to one of them (`TestControlWordsUseTheOneDefinition`).

A control phrase is also never registered as the cancellable run: it is the thing doing the
cancelling, and putting it in the single run slot would displace the child it was sent to stop, so
the leader key would then offer to cancel the cancellation.

This is the intake's half of one stop, not the whole of it — [[34F-legacy-marco-product-audit]]
§14 and its Phase 3 still own the rest.

## The six outcomes

An exit code was not enough, because three genuinely different things all came back as success. A
Play the door **declined** returned nil ("you said no" is not an error). A **cancelled** local
Play returned nil. A generated Play whose Screen guard **refused** caught its own failure, logged
it, and returned nil. So the process exited 0 and every front end reading an exit code rendered
`ok` — for "Marco refused", for "you stopped it" and for "it worked" alike.

| outcome | means | exit |
|---|---|---|
| `performed` | it ran AND arrival was positively verified | 0 |
| `clarify` | Director asked something and is waiting | 4 |
| `refused` | Marco declined or was not permitted — a door, a guard, an edge that would not verify | 5 |
| `unavailable` | the request was never delivered; nothing tried it | 3 |
| `cancelled` | somebody stopped it | 6 |
| `failed` | it was tried and it went wrong | 1 |

`performed` may not be claimed by anything less: finishing is not arriving
([[ADR-070-one-production-body-and-the-caller-brings-the-verification]]). `unavailable` keeps the
meaning and the code (3) it already had — *"never delivered, a caller may try something else"* —
because `plugins/overlay` already read exactly that.

### The wire

Three prefixes on stdout, all protocol rather than user text:

| line | says | when |
|---|---|---|
| `[route] ` | WHICH Play these words became | a Play was authorized and is about to run |
| `[result] ` | what BECAME of the invocation — one of the six | every invocation, always |
| `[intake] ` | source, decision, play-or-phrase, and which rule fired | only with `MARCO_TRACE_INTAKE=1` |

`[route] ` is unchanged on purpose: it answers *"what did these words become"* and `[result] `
answers *"what happened to it"*. The Learn offer needs both facts and can be derived from neither
alone.

The engine and the overlay are separate Go modules, so nothing links the literals at compile time
and nothing fails to build when one side is reworded — the HUD would simply fall back to guessing
from an exit code and start lying again, quietly. `TestTheOutcomeIsAnnouncedOnTheWire` (producer)
and `TestResultPrefixAndVocabularyArePinned` + `TestTheSixOutcomesComeFromTheWire` (consumer) hold
both ends; `TestStreamChildReadsTheAnnouncedLines` holds the reader.

`[intake] ` is off by default because it would otherwise land in the HUD log on every command, and
it keeps **no store and writes no file**: the words a person says are exactly the thing not to
start keeping. `TestTheTraceSaysHowAnInvocationWasRouted`, `TestRoutingAnInvocationWritesNothing`.

### The Learn offer needs both halves

Offering to record a demonstration is honest in exactly one state: `unavailable` **and** no
`[route] `. `unavailable` alone is not enough — a resolved Play whose bridge could not be reached
is unavailable and already exists, so the offer would invite somebody to Learn a Play they already
learned. And a Director that RAN and failed is not an unknown command: answering *"I could not do
that"* with *"shall I learn it?"* is a non-sequitur about something the person just watched go
wrong. `TestTheTeachOfferNeedsBothHalves`, `TestTheTeachOfferFiresOnlyWhenNothingTookIt`.

The engine's side of it is one shared spelling, `"no play matches "`, prefix-matched by the
overlay (`TestTheUnknownCommandErrorIsOnePrefix`, `TestNoPlayMatchesPrefix`).

## Every entrance, and where it lands

| entrance | sends | source | arrives as |
|---|---|---|---|
| overlay command line (`` `m <phrase> ``) | `do --source=typed <phrase>` | `typed` | words |
| overlay voice (a final recognised phrase) | `do --source=spoken <phrase>` | `spoken` | words |
| overlay leader hotkey | `marco hotkey <key>` → resolves the binding, then `runInvocation` | `hotkey` | **identity** (+ its own arguments) |
| control centre Run (Plays tab, Edit view) | `do --source=control-centre --play=<slug> [--app=…] [--focus]` | `control-centre` | **identity** |
| a shell | `marco do "<phrase>"` | `cli` | words |
| `marco assistant` REPL | `runDo` → `runInvocation` | `cli` | words (after its own *"Did you mean…?"* confirmation) |
| a web front end | — | `web` | declared, and no shipped surface sends it yet |

Every one of them reaches `cmd/marco/intake.go` `runInvocation`, the process-side entry, and
`invoke.Decide` inside it. `TestEveryEntranceReachesTheSamePlay`,
`TestEveryEntranceReachesDirectorOnAMiss`, `TestEveryEntranceRoutesThroughTheOneIntake`.

Both overlay paths are **asynchronous**. A replay runs for minutes and a spoken "stop" arrives as
a second phrase while the first is still going; a blocked act handler could not hear it. That
property used to belong to the spoken path alone, and unifying the two would have taken it away
from speech rather than giving it to typing — so it is now a property of the door itself.

The overlay keeps a small vocabulary in FRONT of the door — voice on/off, `learn`, `ui`, `edit`,
`help`, `exit`, `bind`/`unbind`, `press`, `forget`, `rename`, `simplify` — because those are the
overlay configuring **Marco itself**, not desktop intent. (`teach` and `narrate teach` are still
accepted there, and on the CLI, as undocumented compatibility aliases for `learn` —
[[ADR-086-one-acquisition-one-word-one-request]]. They are the only spellings of the reserved word
left anywhere on the acquisition path.) `forget my play` is an instruction about
the catalogue, and Director reading it against the screen would be answering a question nobody
asked. `TestOnlyTheOverlaysOwnVerbsStayLocal`.

## What is deliberately NOT in the intake

**No fuzzy matching.** Play lookup answers *"does Marco already know this exactly"*. The moment it
starts guessing which Play somebody probably meant, it is a second semantic planner sitting in
front of the real one, and the two will disagree. **A near miss is a miss**, and a miss belongs to
Director, which can see and can ask (`TestANearMissIsAMissAndGoesToDirector`).

Two semantic layers used to sit in front of the door on the product path and both are gone from
it: `internal/nlu` at a 0.75 score, and an optional external model behind `$MARCO_RESOLVER`. With
both *"open settings"* and *"open the settings"* registered, asking for the second ran the first —
measured. Both packages survive as a **developer surface** (`marco assistant`'s confirm loop,
`marco args`, `marco simplify`, `marco bind`'s hint at 0.6, `marco dispatch`); what may not come
back is either of them deciding, unasked, which Play a phrase meant.
`TestEveryEntranceRoutesThroughTheOneIntake` names `resolveTarget` and `dispatchDo` explicitly, so
neither can return quietly.

**No second performer.** There is one door (`orchestrator.Authorize`), one local runner
(`runRoute`) and one Director delegation (`performLearned`). The intake chooses among them; it
does not carry anything out itself, and `performed` is only ever said by the thing that did it.

**No Play knowledge inside Director.** Director is never told which Plays exist and never asked to
pick one. It interprets a request against the live world — that is its whole job — and the
registry is the other tier, above it. Teaching Director about Plays would put the deterministic
answer inside the probabilistic one, which is the arrangement this replaced.

**No durable telemetry.** `Decision.Trace` returns a string a caller may log. It is not a store
and not a file.

## Where each fact comes from

| fact | decided by |
|---|---|
| what an invocation means | `invoke.Decide` — the five arms, in order |
| whether a phrase is a control phrase | `intent.IsControlPhrase`, the one definition |
| whether Director is mid-question | `cmd/marco/intake.go` `directorPending`, from `Status().Clarification` |
| which Play a name reaches | `routes.Registry.Resolve` (exact, via `routes.Slug`) |
| whether it may be performed | `orchestrator.Authorize` |
| how a learned Play is performed | `performLearned` → the Director's PERFORM command |
| what happened | `Outcome`, announced on `[result] ` |
| how a front end renders it | `plugins/overlay/outcome.go`, reading that line |

## Related

- [[Plays]] — the durable behaviour on the other side of the door
- [[Learned-Plays]] — the kind of Play the Director performs rather than the engine
- [[Service]] — the Director's command registry, which a performance now enters
- [[Visibility]] — the one account of what Marco is doing
- [[34F-legacy-marco-product-audit]] — §13 counted the deciders; §22 Phase 2 is this note
- [[ADR-083-one-invocation-intake]] · [[ADR-084-a-plays-identity-is-its-subject]] ·
  [[ADR-085-a-performance-is-a-registry-command]]
- [[ADR-029-resolution-is-not-permission]] · [[ADR-066-stop-is-a-product-event]] ·
  [[ADR-081-a-durable-behaviour-is-a-play]] · [[Glossary]]
