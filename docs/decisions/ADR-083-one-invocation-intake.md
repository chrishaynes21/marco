---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - invocation
  - plays
  - visibility
  - service
source_paths:
  - internal/invoke
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - cmd/marco/bind.go
  - cmd/marco/edit.go
  - plugins/overlay/intake.go
  - plugins/overlay/outcome.go
---

# ADR-083 — one invocation intake: transport does not decide meaning

## Two intakes, and which one you got depended on whether you had a microphone

`plugins/overlay/acts.go` had a single `if`. A phrase that arrived as `Run` — typed at the command
line — ran `marco do <phrase>`, which fuzzed it to a slug and, on a miss, offered to record a
demonstration. The same phrase arriving as `RunVoice` ran `marco director <phrase>`, which never
consulted the Play registry at all and reinterpreted the words against the screen.

So the same words meant two different things:

- a Play the person had demonstrated, registered and could run by typing **was not found by
  saying its name**;
- a request Director could have satisfied became **"shall I learn this?"** the moment it was typed
  instead of said;
- and a typed **"stop"** missed Play lookup, fell through to the unknown-command path, and offered
  to record a demonstration called *stop*.

The two paths shared nothing, so nothing could hold them level.
[[34F-legacy-marco-product-audit]] §13 named this *"the real gap is above all of them"* and §1.5
scored it the single largest contributor to *weird UX*.

## The decision

**One intake, and it decides by what Marco knows rather than by how the words arrived.**

> If Marco already knows exactly what the Audience means, it uses the durable thing it knows.
> Otherwise Director works out what they mean from the live world.

- **`internal/invoke` is the one semantic decision, and it is PURE.** `Decide(plays, Request)
  Decision` reads a resolver and returns a verdict. It performs nothing, connects to nothing, and
  cannot be told what to decide by the surface that asks.
- **The precedence is five arms in a fixed order**: control phrase → pending question → explicit
  identity → exact durable match → Director. Each arm's placement is argued in [[Invocation]] and
  each has a test named for it.
- **`Request.Source` is recorded and never consulted.** Typing, speaking, a hotkey, a click and a
  shell are entrances, not different Marcos.
- **Lookup is EXACT.** `routes.Slug` folds case, punctuation, quotes and whitespace onto one
  durable identity; beyond that, a near miss is a miss.
- **An explicit identity is never re-read as words.** A clicked Run and a resolved Binding hand
  over a `routes.Route`, and the phrase is not consulted.
- **`cmd/marco/intake.go` `runInvocation` is the one process-side entry**, and it announces one of
  six outcomes on `[result] `.
- **`dispatchDo` and `resolveTarget` are deleted.** The 0.75-score `internal/nlu` match and the
  `$MARCO_RESOLVER` external model no longer sit in front of the door on the product path.

### Both tiers, and why neither may swallow the other

```
exact durable identity   deterministic, offline, user-owned, no model
                         -- routes.Registry.Resolve, or an identity handed in --
        | miss
        v
Director                 interpretation against the live world, clarification, refusal
```

The deterministic tier is why Marco works with no model, and it is what makes *"the phrase I saved
it under runs the thing I saved"* a promise rather than a probability. The Director tier is what
lets the first one stay exact: because a miss lands somewhere that can see and can ask, the lookup
never has to be generous.

## Considered and rejected

- **Teach Director about Plays — let it pick from the registry.** This puts the deterministic
  answer *inside* the probabilistic one. It makes an offline machine worse than it is today, makes
  the answer to "which Play did that phrase run" depend on a model's mood, and gives up the single
  property the registry exists for. It also inverts the layering: the registry is the tier ABOVE
  interpretation, and a request only reaches Director because the registry did not know it.
- **Make Play lookup fuzzy enough to catch near misses.** This is what `resolveTarget` did, and it
  is the failure that was measured: with both *"open settings"* and *"open the settings"*
  registered, asking for the second ran the first — silently, in front of the authority door.
  Fuzzy matching is a *judgement* about what somebody probably meant, and judgement is Director's
  job; a second judge in front of the real one will disagree with it, and the person has no way to
  tell which one answered. A near miss now goes to the tier that can look at the screen and ask.
- **Keep two intakes and synchronise them.** They had already drifted so far that one of them did
  not know the registry existed. Two implementations of one decision is the duplication being
  removed, and "keep them in step" is a promise no test can hold — every future arm would have to
  be written twice, correctly, by somebody who knew both existed.
- **Decide in the front ends.** Every front end would then need the registry, the control
  vocabulary and the pending-question state, and there are three of them. Routing is a decision
  about *which request*; the overlay does not get to make it, which is the same line
  [[director-service]] drew for interpretation.
- **Keep `--source` out of the wire entirely, since nothing reads it.** Then nothing could prove
  cross-surface sameness. It exists so an acceptance run — and `MARCO_TRACE_INTAKE=1` — can show
  that two transports produced one decision. That it is never consulted is the property, and
  `TestSourceCannotChangeTheDecision` is what keeps it a property.

## Consequences, including the costs

- **A phrase that ALMOST names a Play now goes to Director.** Somebody who types "open the
  settings" for a Play called "open settings" no longer silently runs it: Director reads the words
  against the screen, and may do something else or ask. That is the intended behaviour — the
  silent near-miss was the defect — but it is a real change, and it costs a person who liked the
  loose matching. The confirmed-substitution loop in `marco assistant` is where that habit
  survives, and it *asks* before it substitutes.
- **Every invocation may now dial the Director once**, to learn whether a question is pending.
  It connects without starting anything, so the cost when nothing is running is one failed dial
  rather than a service start; but it is a syscall on a path that previously had none.
- **Six outcomes are a contract between two Go modules that do not link.** `plugins/overlay`
  matches the `[result] ` literal and the six words at runtime. Rewording either side compiles
  cleanly and breaks the HUD's honesty quietly, which is why both ends are pinned by tests rather
  than by types.
- **The Learn offer got narrower.** It fires only on `unavailable` with no `[route] `, so a
  request that Director tried and failed no longer becomes an offer to record a demonstration.
  Fewer offers, and every one of them true.
- **`internal/nlu` and `internal/resolver` are now developer surfaces with no product caller.**
  They are still built, still tested and still reachable from `marco assistant`, `marco args`,
  `marco simplify`, `marco bind` and `marco dispatch`. Somebody will eventually ask why they are
  there; [[34F-legacy-marco-product-audit]] §22 Phase 6 is where they are retired, and until then
  the risk is that one of them creeps back in front of the door.
- **`marco do` no longer offers to Learn on an unknown phrase.** The engine returns the
  `"no play matches "` error and it is the OVERLAY that offers to Learn. A shell user gets an
  honest error where they used to get a prompt.

## Enforced by

- `internal/invoke` — `TestSourceCannotChangeTheDecision` (every source through every case),
  `TestStopIsNeverOrdinaryText` (arm 1), `TestAnAnswerToAPendingQuestionIsNotAPlay` (arm 2),
  `TestAnExplicitPlayIsNeverLookedUpAgain` (arm 3, via a resolver that records being asked),
  `TestAnExactlyKnownPlayIsNeverInterpreted` (arm 4),
  `TestANearMissIsAMissAndGoesToDirector` and `TestDirectorReceivesTheWordsAsSpoken` (arm 5),
  `TestAStagedPlayDoesNotInterceptAnInvocation`,
  `TestAmbiguityIsResolvedByTheStoresRuleNotByOrder`, `TestNothingSaidDecidesNothing`,
  `TestTheTraceSaysHowAnInvocationWasRouted`.
- `cmd/marco` — `TestEveryEntranceReachesTheSamePlay` and `TestEveryEntranceReachesDirectorOnAMiss`
  (the phase's acceptance, both directions, from every source),
  `TestEveryEntranceRoutesThroughTheOneIntake` (the mutation gate: it names `resolveTarget` and
  `dispatchDo` so neither can return quietly), `TestARegisteredPlayInterceptsBeforeDirector`,
  `TestAStagedPlayNeverInterceptsFromAnyEntrance`, `TestNormalizationIsTheSameFromEveryEntrance`,
  `TestAnApostropheDoesNotHideAPlay`, `TestAHotkeyPerformsTheBoundPlayItself`,
  `TestStopReachesTheRunningWorkFromEveryEntrance`,
  `TestAnAnswerIsNotClaimedByAPlayOfTheSameName`, `TestTheOutcomeIsAnnouncedOnTheWire`,
  `TestExactPlayMatchDoesNotBypassAuthority` (the door is still there, and there is exactly one of
  it), `TestRoutingAnInvocationWritesNothing`, `TestTheUnknownCommandErrorIsOnePrefix`.
- `cmd/marco` (the control centre) — `TestAClickedRunSpawnsAnExplicitIdentity`,
  `TestARunSurvivesANameThatDoesNotRoundTripThroughItsSlug`, `TestTheRunArgvSpellsEachScope`,
  `TestANameOnlyRunPayloadIsResolvedToAnIdentity`, `TestTheEditViewRunsThePlayItHasOpenByIdentity`,
  `TestAStagedPlayCannotBeRunThroughTheEndpoint`.
- `plugins/overlay` — `TestTypedAndSpokenDifferOnlyBySource` (one argv, one flag apart),
  `TestTheSourceWordsAreTheEngines`, `TestATypedPhraseReachesTheIntake`,
  `TestOnlyTheOverlaysOwnVerbsStayLocal`, `TestControlWordsUseTheOneDefinition`,
  `TestASpokenStopKillsTheChildImmediately`, `TestCancelAlsoStopsTheDirector`,
  `TestResultPrefixAndVocabularyArePinned`, `TestTheSixOutcomesComeFromTheWire`,
  `TestStreamChildReadsTheAnnouncedLines`, `TestTheTeachOfferNeedsBothHalves`,
  `TestTheTeachOfferFiresOnlyWhenNothingTookIt`, `TestNoPlayMatchesPrefix`.

## Related

[[Invocation]] · [[Plays]] · [[Learned-Plays]] · [[Service]] ·
[[34F-legacy-marco-product-audit]] · [[ADR-029-resolution-is-not-permission]] ·
[[ADR-066-stop-is-a-product-event]] · [[ADR-081-a-durable-behaviour-is-a-play]] ·
[[ADR-084-a-plays-identity-is-its-subject]] ·
[[ADR-085-a-performance-is-a-registry-command]] · [[Wiring-Tests]]
