---
type: decision
status: accepted
date: 2026-10-02
affects:
  - invocation
  - verification
source_paths:
  - internal/outcome/outcome.go
  - cmd/marco/intake.go
  - cmd/marco/director.go
  - cmd/marco/activity.go
  - internal/director/execute/program.go
  - internal/acceptance/report.go
---

# ADR-135 — a delivered action is not a failed one

## Context

Marco reported what became of a request in six words, and the set was deliberately closed:
`performed`, `clarify`, `refused`, `unavailable`, `cancelled`, `failed`. The distinctions it drew
were the right ones — declining is not breaking, and nothing-tried-it is neither — and
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]] held the hardest line in it: only an action whose ARRIVAL was
verified may say `performed`.

That line left a gap on the other side. Having refused to claim success, Marco had nowhere to put
the result except `failed`, so "I will not claim that happened" and "it happened and I cannot
prove it" shared one word.

Measured live against the acceptance fixture:

```
marco do "select Beta"
UNVERIFIED: "Beta" reports itself selected and nothing else changed to confirm it
(1 action(s) performed)
[result] failed
```

Three lines that cannot all be true. The click landed on the control. The list item reported
itself selected. The application's own `SelectedItem` was still null, so Marco correctly declined
to say `performed` — and then said the opposite instead, under a count line claiming an action HAD
been performed.

Four distinct Director verdicts were arriving as `failed`:

| the Director said | a person was told |
|---|---|
| `unverified` — delivered, uncorroborated | failed |
| `cancelled` — somebody pressed stop | failed |
| `blocked` — a door declined, by policy | failed |
| a pending CLARIFICATION_REQUIRED | failed |

Every one of those words already existed in the vocabulary. Nothing reached them, because
`directorSubmit` returned `exitNotVerified` for anything that was not `CommandCompleted` and
`outcomeOfDirectorExit` read every unrecognised code back as `failed`. The Director's own
`classify` had been right the whole time, and said so in a comment: *"the difference decides
whether a user should try again — which is exactly the mistake that sent a browser two pages back
in an earlier milestone."* The verdict was correct and then discarded at a three-value exit code.

## Decision

**`unverified` is the seventh word, and the vocabulary stays closed at seven.**

> Marco delivered it and could not confirm it arrived.

Not a success and not a failure. The argument for spending a word on it is not tidiness — it is
that `failed` and `unverified` call for OPPOSITE responses:

- `failed` invites a retry.
- Retrying a request Marco already delivered acts on the world a second time.
- The actions worth verifying are exactly the ones worth not doing twice.

So the most dangerous thing about reporting `failed` for a delivered action is not that it is
untrue; it is that it is *actionable*, and the action it suggests is harmful.

**Three consequences.**

1. **One crossing, lossless.** `exitOfCommandState` maps a `service.CommandState` to an exit code
   and `outcomeOfDirectorExit` maps it back. They are only correct together, so they are tested
   together, as a round trip, over every terminal state — plus a no-collision test, without which
   every round-trip case could pass by accident.

2. **A stop mid-step is a cancellation.** The program loop checked `ctx.Err()` BETWEEN steps and
   caught a stop that landed there. It did not cover the window inside one: the operation was in
   flight when the context died, came back with an error its own cancellation produced, and the
   loop read that error as the step's verdict. Measured on a real desktop — `type Marco into the
   Name field and click Save`, stopped during the typing — nothing was typed, Save was never
   pressed, and the person who had just pressed stop was told the request FAILED. The step's own
   account is kept in the message; only the status is relabelled.

3. **The count line says what it counted.** `(%d action(s) performed)` was one format string for
   every ending. It now reads `delivered, not confirmed` under an unverified result and
   `performed before it was stopped` under a cancellation.

**And the vocabulary has one definition.** `cmd/marco/intake.go` held the FOURTH copy of the set —
its own `type Outcome string`, its own six constants, its own `Exit()`, and its own `[result] `
literal, in the module that PRINTS the line. `internal/outcome` exists to end exactly that and its
own note says so ("One package, imported by all three"); the overlay's note recorded what was
still outstanding ("What is STILL hand-matched is the PRODUCER: cmd/marco/intake.go"). They are
aliases now. This was not incidental cleanup: it is the difference between adding a word in one
place and adding it in four, three quarters of which somebody could forget.

## What it cost, and why that is the argument for the gate

`TestTheWordsAreAClosedSet` asserts the count and says "change this test deliberately or not at
all". It was changed once, deliberately. The bill:

```
cmd/marco      the crossing (both directions) + the count line
Activity page  the word map AND the CSS class
HUD            the sentence, the glyph, and the colour
harness        internal/acceptance read the words off a hand-copy
```

Five surfaces, and a sixth that said nothing at all. The HUD glyph guard — then named for the six,
now `TestEveryOutcomeHasItsOwnGlyph` — listed its six identifiers by hand, so the HUD drew nothing
for the seventh and the test passed. That is the same shape as the promotion-licence test that let
mutation M5 survive: **a guard that enumerates by hand stops guarding the moment the thing it
guards grows.** It derives its names from `outcome.All` now, and so does the harness.

## What was NOT done

- **No provider workaround.** The selection case that started this is still unverifiable: child
  `IsSelected=true`, container `GetSelection()="Beta"`, application `SelectedItem=null`. The
  container hypothesis was falsified live, so no container evidence was built. Accessibility
  selection state is not an independent proof of application state on all providers, and
  `verify/semantic.go` carries that limitation in writing.
- **`unverified` is not a licence to act.** `bind` and the learn offer still require `performed`.
  An unconfirmed action is not a confirmed one for any purpose except telling a person the truth
  about it.
- **The uppercase header still says the Director's state** (`UNVERIFIED:`, `COMPLETED:`) while
  `[result] ` says the vocabulary's word. Two spellings of one fact, left alone: the header is the
  Director's sentence and the result line is protocol.

## Enforced by

- `TestTheDirectorsVerdictSurvivesTheCrossing` — every terminal `CommandState` round-trips to a
  truthful word, through both halves.
- `TestNoTwoVerdictsShareAnExitCode` — without which the round trip would pass on collisions.
- `TestADeliveredActionIsNeverReportedAsFailed` — names the retry harm; rejects `performed` too.
- `TestAQuestionIsNotAFailure` — a pending clarification reaches `clarify`.
- `TestTheVocabularyHasOneDefinition` — the aliasing is structural; a local redefinition would not
  compile.
- `TestTheWordsAreAClosedSet` — the deliberate-change gate, now at seven.
- `TestNoTwoOutcomesShareAnExitCode` — `ExitUnverified` is distinct from `ExitFailed`.
- `TestActivityRendersEveryOutcomeWord` — walks `outcome.All` for a word AND a CSS rendering.
- `TestEveryOutcomeHasItsOwnGlyph`, `TestEveryOutcomeHasItsOwnStatusLine`,
  `TestTheHudRendersEveryOutcomeInTheSet` — the HUD renders all seven; the glyph test derives its
  names from `outcome.All` rather than listing them.
- `TestAStopDuringAStepIsCancelledNotFailed` — the mid-step window.
- `TestAnUncancelledStepFailureIsStillAFailure` — the other side, so `cancelled` does not become
  the word for everything that goes wrong.
- `internal/acceptance` `selection/select Beta` — both false claims fail it: `performed` on
  nothing, and `failed` on something Marco delivered.
- `internal/acceptance` `stop/during a sequence` — the timing stays a recorded race; the WORD is
  asserted in every branch.
