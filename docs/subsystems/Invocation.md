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
updated: 2026-11-05
source_paths:
  - internal/invoke
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - cmd/marco/bind.go
  - cmd/marco/edit.go
  - internal/director/intent/clarify.go
  - cmd/marco/demonstrated.go
  - internal/director/runref
  - plugins/overlay/intake.go
  - plugins/overlay/outcome.go
  - internal/widgetconfig
  - plugins/overlay/anchor.go
  - plugins/overlay/compat.go
  - cmd/marco/line.go
  - plugins/overlay/line.go
  - plugins/overlay/lineedit.go
  - plugins/studio/widget.go
  - plugins/studio/forgetone.go
  - internal/widgetstack
  - internal/skills
  - cmd/marco/skill.go
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
- A shortcut is resolved to routine identities **once, when it is assigned**, and stores them
  (`routes.Binding.Routes`); `marco hotkey` performs those identities and looks no name up at all.
  It used to put its own command text back through the phrase matcher on every press, so a hotkey
  bound to one Play could quietly start running a different one as the registry filled up
  (`TestAHotkeyPerformsTheBoundPlayItself`), and then still by name once per press, so a namesake
  or a rename could redirect it (`TestAPressedShortcutIsTheRoutineItWasAssigned`). A shortcut set
  up by name before that refuses until reassigned. See
  [[ADR-297-a-shortcut-names-its-routine-and-has-one-owner]].
- The widget's leader lists `marco bindings --here --json` and a key then runs `marco hotkey`; the
  two are held together key by key (`TestTheArmListIsWhatAPressWouldFire`). A key with nothing on it
  says so and announces no outcome. A press ends in the widget's status line and history
  (`TestAPressedShortcutIsInTheHistory`). The session is bound to the application in front at the
  leader: the list is of it (`--armed-in=`), the key carries it, and the engine refuses before
  selecting a binding if another application is in front when it reads
  (`TestAKeyPressedAfterTheFocusMovedRunsNothing`, `TestAKeyIsForTheApplicationItsLeaderWasPressedIn`).
  See [[ADR-299-the-widget-explains-its-keys]].
  A carried press is not sent through an engine that would ignore the context: the widget asks the
  engine first (an `armed_in` echo on a carried listing) and otherwise says shortcuts are off
  (`TestAPressIsNotSentToAnEngineThatWouldNotCheckIt`).
- The widget's settings (`overlay.json`) have one reader and writer, `internal/widgetconfig`, used
  by the widget, the control centre and Studio: the widget's precedence everywhere, and a save
  writes only the keys its person changed, keeping every other field (F5). See
  [[ADR-300-one-owner-for-the-widgets-settings]].
- The widget's `monitor` setting is placed by, on a switch and on every auto-fit resize: the corner
  is given to Ebiten only while Ebiten's cached current monitor is the chosen display, because
  Ebiten v2.9.9 measures SetWindowPosition from that cache and SetMonitor does not update it. The
  panel counts displays from 1; the file keeps the zero-based index. See
  [[ADR-330-the-widget-anchors-to-the-chosen-display]].
- The widget's command line reads back what Enter would run (`marco line --json`, read-only): an
  exact name first, then the intake's grammar, then a routine taking one value followed by it.
  A line with values runs as its identity and the version read (`--play`, `--expect`, `--arg`),
  only once its read-back has been on screen; values Director proposed are reviewed there before
  they run. See [[ADR-301-the-command-line-reads-back-what-it-will-run]].
- ↑ and ↓ on the widget's command line walk the commands, as a shell's history does, and ↓ past the
  newest puts back what was being typed. A command comes back as it ran, values and all, so ↑ then
  Enter runs it again. See [[ADR-326-the-command-line-recalls-commands]].
- The widget's command line has a caret: ← → (Ctrl for a word), Home, End, Delete, Ctrl+Backspace;
  typing inserts at it. See [[ADR-328-the-command-line-edits-like-an-editor]].
- Built-in desktop skills: `focus <app>` brings an open program's most recently active window
  forward, `search the web for <query>` opens the configured provider's results. Read only when no
  routine took the words, run as `--skill=… --arg=…` from the read-back, and a paraphrase is
  Director's resolver's, proposed as the skill's explicit line. See [[ADR-327-built-in-desktop-skills]].
- Studio opens the widget (Open widget). It starts the installer's own stack beside it, with
  Studio's library and store, never a second one beside one already running, and never from pieces
  of different builds. A widget Studio did not start is said to be running with its library and
  settings unverified. In a development tree the stack is the root's, from either `studio.exe`,
  and a problem names the exact command, `plugins\studio\build.cmd`, which builds Studio and the
  whole stack from one clean commit (repaired 2026-11-04). See [[ADR-302-studio-opens-the-widget]].
- Opening the widget asks for the Director: it runs `marco director start --for=<its pid>` once.
  A Director that starts is stopped once the widget has gone and nothing is using it; one already
  running is left alone. Reads still never start one. See
  [[ADR-308-opening-the-widget-asks-for-the-director]].
- Studio forgets one routine through the one forget, after a confirmation naming every part. The
  confirmation is sealed so that nothing it did not show is removed, and each part is reported as
  found afterwards. See [[ADR-303-studio-forgets-one-routine]].
- A narrated recording's window is settled before narration starts: the widget gives it or starts
  nothing, the terminal answer is one line, and the session is of that window. See
  [[ADR-304-narration-is-of-a-window-settled-first]].
- Enter means no at every textual yes/no confirmation, shown as `[y]es / [N]o`. Enter still submits
  a command, or a read-back on screen. See [[ADR-305-enter-means-no]].
- A guarded "Only in" routine is bound at admission to the window in front, read once, with no raise
  or search, and every press still checks it. Legacy in-place routines are admitted as before. See
  [[ADR-306-a-guarded-routine-may-be-only-in-its-application]].
- Studio draws the widget's theme. The ten palettes live in `internal/widgetconfig` as data, and
  one Go mapping gives Studio tokens whose text reads at 4.5:1 on every surface. The page applies
  them at start-up and on its slow poll. The widget still reads its settings only at launch. See
  [[ADR-307-studio-draws-the-widgets-theme]].

`invoke.Plays` is a one-method interface partly for exactly this reason: a test can hand in a
resolver that records every call and assert it was **not** asked
(`TestAnExplicitPlayIsNeverLookedUpAgain`).

**AN IDENTITY STILL HAS TO BE IN THE RIGHT PLACE.** An *Only in <app>* routine is admitted at
`performOnePlay` — whichever arm decided it — only when the window handed to the run is owned,
as the host validated it, by that application (Studio's Play), or when that application is in
front at that instant. `--window` alone is not admission and `Source` is never read. Before this,
only the phrase arms kept the scope, so `--play` ran an in-place routine into whatever was in
front. See [[ADR-296-one-scope-vocabulary-one-default-and-in-place-is-checked-at-the-door]].

**AND A STEP'S TEXT CAN BE MADE INTO ONE FROM THE PANEL THAT SHOWS THE ROUTINE.** A third edit
operation replaces a Type step's literal with `{{name}}` — the same placeholder everything below is
built on — so somebody types the text they actually want and then says which part changes. It was
possible two ways before and both were invisible (the learn phrase's `with name` clause, and a
reserved key tapped mid-demonstration), which is the whole reason for it. A name the generator
reads from the CREDENTIAL STORE is refused, because asking for text you supply must not quietly
produce a password lookup. See [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]].

**A routine may also take CHANGING TEXT.** A declared argument, a placeholder the demonstration
puts in place, and `routes.ApplyArgs` filling it at run time — and a routine that needs text and
is given none now REFUSES rather than typing nothing, which is what it did before. Values travel
with the identity as `--arg=name=value` flags and never as words, because an explicit identity's
words are deliberately not read for arguments; they are copied onto the authorised decision, and a
routine with arguments is confirmed before it presses. See
[[ADR-276-a-routine-may-be-told-what-to-type]] and
[[Text-arguments-proposal-2026-10-30]].

**AND A REQUEST MAY PROPOSE THAT TEXT, which is not the same as supplying it.** Director asks the
configured resolver a THIRD question on the one plugin boundary, after it has already selected a
routine: which words of the request are which argument's value. The plugin answers in OFFSETS and
never in text — `resolver.Span` has no field for a value — so Marco copies the words out of its own
copy of the request, verbatim, and a value nobody said is not something the boundary refuses but
something it cannot express. What travels back is a `resolveroute.Proposal` to be shown, confirmed
or corrected, carrying WHICH ARM supplied it and the DIGEST OF THE REVISION its names were read
from; nothing acts on it, and the thing that runs a routine is still the confirmation above. A
SECRET IS LEFT OUT OF THE QUESTION rather than filtered out of the answer.

A surface shows the proposal in the field it would have asked for, where it can be CORRECTED, and
sends back BOTH what it offered and what came back. The provenance is then DERIVED rather than
asserted — left alone is `extracted`, changed is `edited`, never proposed is `typed` — so a window
that prefilled a box and forgot to relabel it cannot credit somebody's own correction to a model;
and the revision travels too, so a proposal cannot be confirmed against a routine saved since. The
provenance is sealed with the values, because the label is the only thing saying whose words these
are. See [[ADR-277-a-request-may-propose-the-text-a-routine-types]] and
[[Extracting-argument-values-proposal-2026-10-30]].

Arguments follow identity. A Binding carries its own (`` `e = enter freeplay with 3 ``) and a
clicked Run carries none; only a phrase has to be read for them. Re-reading a phrase that carried
none, over cargo that did, would silently drop arguments somebody supplied.

## The precedence — six arms, in this order

`invoke.Decide(plays, sights, Request) Decision` is pure. It reads two resolvers and returns a
verdict; it performs nothing, connects to nothing, and cannot be told what to decide by the
surface asking.

| # | arm | fires when | why it sits here |
|---|---|---|---|
| 1 | **control phrase** | `intent.IsControlPhrase(text)` | it acts on what is ALREADY running, so nothing may look at it first |
| 2 | **pending question** | `Request.Pending` | Director asked something; these words are the answer |
| 3 | **explicit identity** | `Request.Play != nil` | the surface already knows; the words are not read |
| 4 | **exact durable match** | `plays.Resolve(app, text)` succeeds | Marco already knows precisely this |
| 5 | **exact demonstration** | `sights.Demonstration(app, text)` succeeds | the person SHOWED Marco this one; it is performed by sight |
| 6 | **Director** | everything else | Marco does not know it, so it is a question about the world |

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
to record a demonstration. `TestAPlayNamedWithTheGrammarIsStillReachable`. Each step is read as a
line alone is (`lineRoutine`), so "discord mute then notepad type hello" gives `notepad type` the
value "hello" without `with` — but words after a name are its value only when the routine takes
exactly one, or they could land on the wrong one. `TestAChainStepTakesItsValueAsALineDoes`.

**AND "REPEAT <ROUTINE> <N> TIMES" IS NEVER RUN FROM WORDS.** The widget's command line reads it
(`marco line`, with `times`) and reviews it as "Open social ×5"; only `--times` on a confirmed
identity runs it, one ordinary run at a time, stopping at the first that does not perform or at a
Stop. Words in that form reaching this arm are proposed with the count beside the answer. See
[[ADR-325-repeat-a-routine-a-fixed-number-of-times]].

**AND A ROUTINE MAY ANSWER TO MORE THAN ITS OWN NAME — BUT NOT HERE.** Phrases kept beside the
source (`<slug>.aliases.json`) are NOT consulted by this arm. A phrase is exact, which is why
claiming one here looked right; what it is not is necessarily UNAMBIGUOUS, and that is a fact about
the whole library. The first version selected by the same four-scope priority a slug walks, so a
phrase two routines answered to was performed here while Director, asked the same words against the
same library, asked which was meant — one phrase, two answers, and the one that acted could not
ask. So the words fall through to Director, which owns the resolution that needs a judgement and
can ask about it. The cost is real and stated: with Director unavailable a phrase reaches nothing,
where a routine's own name still runs. Nothing about a phrase changes the arms above this one
either — a routine taught to answer to "stop" does not claim the word that has to reach a running
play. See [[ADR-275-a-routine-may-answer-to-more-than-one-phrase]],
`TestAPhraseIsHandedToDirectorRatherThanClaimedHere` and
`TestAPhraseNeverOutranksAControlWord`.

**5 — a demonstration is the OTHER durable half, and it is not a Play.** A routine a person
showed Marco by hand carries no verbs, lives in the demo store rather than the registry, and can
only be carried out by the Director's supervised runner — grounded on a fresh look, authorised one
action at a time, verified. So the two cannot share a store or a performer; what they share is
this door, because a front end that checked one store, then another, then fell through to Director
would be three places deciding what a phrase means.

It sits AFTER the play arm, which is the compatibility guarantee: a name that runs something today
keeps running it, and a demonstration saved under that name does not take it over
(`TestAPlayStillWinsOverADemonstrationOfTheSameName`, which also asserts the demonstration store is
never consulted when a play answered). It sits BEFORE Director, because a demonstration IS
something Marco already has — reading its name against the screen was Director hunting for a
control somebody had already pointed at, and what that did in practice was answer *nothing
matching "ark graphics live" is present in the observed window* about a routine demonstrated on
that very window.

The arm only claims a routine the runner can actually perform — `Artifact.PerformableBySight`, one
definition on the artifact — because a door that promised one it would decline is Marco accepting
a request and then arguing with itself. [[ADR-241-a-name-you-demonstrated-is-a-name-you-can-use]],
`TestADemonstrationIsPerformedBySight`.

**6 — Director gets the words as spoken**, trimmed and no more. Director reads sentences, and
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
(`TestCancelAlsoStopsTheDirector`, `TestASpokenStopReachesTheRunningPlayImmediately`).

**One definition of the word, shared.** `intent.IsControlPhrase` is used by `invoke.Decide`, by
the overlay, and by the Director service's own phrase routing. A second list anywhere would be a
second answer to *"did they say stop"*, and the two would drift the first time somebody added a
word to one of them (`TestControlWordsUseTheOneDefinition`).

A control phrase is also never registered as the cancellable run: it is the thing doing the
cancelling, and putting it in the single run slot would displace the child it was sent to stop, so
the leader key would then offer to cancel the cancellation.

This is the intake's half of one stop, not the whole of it — [[34F-legacy-marco-product-audit]]
§14 and its Phase 3 still own the rest.

## The seven outcomes

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
| `unverified` | Marco delivered it and could not confirm it arrived | 7 |
| `failed` | it was tried and it went wrong | 1 |

`performed` may not be claimed by anything less: finishing is not arriving
([[ADR-070-one-production-body-and-the-caller-brings-the-verification]]). `unavailable` keeps the
meaning and the code (3) it already had — *"never delivered, a caller may try something else"* —
because `plugins/overlay` already read exactly that.

`unverified` was the seventh and the only one added since
([[ADR-135-a-delivered-action-is-not-a-failed-one]]). Refusing to claim success left nowhere to
put "it happened and I cannot prove it" except `failed`, and those two call for OPPOSITE
responses: `failed` invites a retry, and retrying a request Marco already delivered acts on the
world a second time. The actions worth verifying are exactly the ones worth not doing twice.

### The wire

Three prefixes on stdout, all protocol rather than user text:

| line | says | when |
|---|---|---|
| `[route] ` | WHICH Play these words became | a Play was authorized and is about to run |
| `[result] ` | what BECAME of the invocation — one of the seven | every invocation, always |
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
wrong. `TestTheLearnOfferNeedsBothHalves`, `TestTheLearnOfferFiresOnlyWhenNothingTookIt`.

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

The overlay starts the microphone itself and forwards voice.exe's own `Voice` events, so a spoken
phrase arrives the same way from every launcher, and a spoken line naming a routine that takes
values opens the same read-back a typed one does ([[ADR-318-the-widget-starts-its-own-microphone]]).
The installer ships the recogniser, one recogniser listens per microphone, and nothing heard reaches
the widget's log ([[ADR-319-the-installer-ships-voice]]). `runInvocation` also writes down what
became of every attempt that is not a control phrase — refusals before a run included, scrubbed of
anything supplied — and the widget answers "why didn't that work?" from it
([[ADR-320-why-didnt-that-work]]).
The widget asks Director which saved routine was meant after a Director failure as well as when
nothing took the words, and Director asks the local model, found beside marco when
`$MARCO_RESOLVER` is unset, which recovers a misheard name and its value
([[ADR-321-a-misheard-name-goes-to-the-model]]).
And the intake asks that resolver BEFORE reading unmatched words against the screen: a routine it
names is proposed for review and nothing runs ([[ADR-322-resolve-before-reading-the-screen]]).
Before any of that, the recogniser itself is told the routine names: a second Vosk recogniser over
a phrase list the widget writes corrects a whole listed phrase in what the open one heard
([[ADR-323-the-recogniser-is-told-the-routine-names]]).
What the widget can resolve when nothing matches exactly is mapped in
[[Skills-on-a-miss-2026-11-05]].

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
| what an invocation means | `invoke.Decide` — the six arms, in order |
| whether a phrase is a control phrase | `intent.IsControlPhrase`, the one definition |
| whether Director is mid-question | `cmd/marco/intake.go` `directorPending`, from `Status().Clarification` |
| which Play a name reaches | `routes.Registry.Resolve` (exact, via `routes.Slug`) |
| which demonstration a name reaches | `demo.Store.Sequence`, through `cmd/marco`'s `demoSights` |
| whether it may be performed | `orchestrator.Authorize` |
| how a learned Play is performed | `performLearned` → the Director's PERFORM command |
| how a demonstration is performed | `performDemonstration` → `director perform-workflow`, one authorised action at a time |
| how a run says what it was and how it ended | `internal/director/runref` — the reference, printed by the runner and read by the front end |
| what happened | `Outcome`, announced on `[result] ` |
| how a front end renders it | `plugins/overlay/outcome.go`, reading that line |

## And asking for one in words, which is a different door

`invoke.Decide` is EXACT for a play on purpose: "near enough" is a judgement, and judgement is
Director's. So a request that is not a play’s own name goes to Director to be read against the
screen, and nothing in that door ever consults a language model about which saved routine somebody
meant.

**AND THAT SECOND DOOR IS DIRECTOR'S NOW.** The policy below lived in Studio's own module,
where the widget could not reach it; it is `internal/resolveroute` behind a read-only
`RESOLVE_ROUTE`, and every surface asks. See
[[ADR-267-director-owns-route-resolution]] — including why there is no local fallback, and why a
world-model observation of the foreground is evidence rather than proof.

Studio has a second door, and it is a SUGGESTION rather than an invocation. It reads a request in
a person’s own words, offers the eligible routines to the optional resolver plugin, and SELECTS
the one that comes back — by its whole identity, which is then handed to the Play button that was
already there. Nothing in it starts a process, and what it hands over arrives at this door through
the explicit-identity arm, where no phrase is looked up again.

The resolver it asks (`internal/dispatch` — `plugins/llama` — a local Ollama) was built, tested
and configured by the installer long before this, and **was reached by nothing a person uses**:
the widget never called it. That is why a decision now says which arm answered and whether the
configured resolver was absent, answered, or asked and silent. See
[[ADR-265-say-what-you-want-and-studio-offers-the-routine]].

## Related

- [[Plays]] — the durable behaviour on the other side of the door
- [[Learned-Plays]] — the kind of Play the Director performs rather than the engine
- [[Service]] — the Director's command registry, which a performance now enters
- [[Visibility]] — the one account of what Marco is doing
- [[34F-legacy-marco-product-audit]] — §13 counted the deciders; §22 Phase 2 is this note
- [[ADR-277-a-request-may-propose-the-text-a-routine-types]] — a proposal is never a supply
- [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]] — and a way to declare one
- [[ADR-083-one-invocation-intake]] · [[ADR-084-a-plays-identity-is-its-subject]] ·
  [[ADR-085-a-performance-is-a-registry-command]]
- [[ADR-029-resolution-is-not-permission]] · [[ADR-066-stop-is-a-product-event]] ·
  [[ADR-081-a-durable-behaviour-is-a-play]] · [[Glossary]]
