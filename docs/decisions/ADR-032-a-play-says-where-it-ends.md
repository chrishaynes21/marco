---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - programs
  - marco-boundary
  - demonstrations
source_paths:
  - internal/director/marcoexec/play.go
  - internal/director/observe/lowering.go
  - cmd/director/learnedplay.go
  - internal/platform/screenhost/screenhost.go
---

# ADR-032 — a play says where it ends

A learned play could already say where it begins ([[ADR-030-a-play-says-where-it-begins]]) and
the user could already name that screen ([[ADR-031-the-user-names-the-stage]]). It still had no
way to tell whether it had **worked**.

## Decision 1 — emitting every effect is not succeeding

A host can send `down` and `confirm` perfectly while a dialog eats them, a frame drops, or the
menu had one more row than the demonstration did. Before this milestone the play reported `ok!`
as soon as the last key went out, which is a claim about the *host* dressed up as a claim about
the *application*.

So `this is ok!` moved **inside** the arrival check. There is exactly one success ending in a
generated play and the only path to it runs through a positive identification of the screen the
play said it would finish on.

## Decision 2 — the destination is verified semantics, not a parameter

`LoweringJudgement.EndsOn` is resolved from durable memory for `c.Relationship.To` — the same
edge the rehearsal completed and the same one the evidence digest is about. No caller may supply
it, exactly as no caller may supply the source name. `LowerPlayBetween` takes both or neither.

## Decision 3 — one naming need, not two

There is no `AskNameDestinationScreen`. The place being named is still just a remembered screen,
and a subject that is the destination of one play may be the source of the next. `Called` lives
on the subject; the procedure only refers to it. `LoweringJudgement.Unnamed` lists the subjects
still needing a name, **source first**, and the lifecycle asks about `Unnamed[0]`. Naming it and
recomputing is what surfaces the second — there is no queue and no remembered question.

## Decision 4 — the two failures are different in kind

|  |  |
|---|---|
| wrong start | zero effects, and the arrival is never even asked about |
| wrong destination | the effects have already happened, and the play still fails |

Marco does not undo, retry, replay, or send anything to recover. It does not invalidate the
learned play and it does not touch semantic memory. Runtime evidence says *this invocation did
not end where it expected*, in the play's own words.

## Decision 5 — no new syntax, and no new capability

`EndsOn` is the same `Screen's Showing`. Nothing was needed in the language or in the act to ask
the question after the effects rather than before them, because "is the screen the user named the
one in front?" is the same question whenever it is asked. Core v1 is unchanged.

## Decision 6 — nothing hidden is required at invocation

A saved play needs its `.marco`, semantic memory, the Screen host and the ordinary invocation
machinery. No `ProcedureCandidate`, no `RehearsalEvidence`, no `.origin.json` enforcement. A user
who deletes the arrival check owns a play that no longer makes that claim, and Marco stops saying
it verified the file.

## Enforced by

- `TestAPlayThatArrivesWhereItSaidSucceeds` — effects, then a fresh look, then success.
- `TestAPlayThatDoesNotArriveFails` — effects happened, play failed, nothing sent afterwards.
- `TestOnlyAPositiveArrivalIsSuccess` — different / ambiguous / unobservable / recogniser gone /
  unknown all fail after the effects.
- `TestADestinationNamedInAnotherApplicationDoesNotCount`.
- `TestAWrongStartNeverReachesTheDestinationCheck` — Roadmap 28's guarantee, re-proved.
- `TestRemovingTheDestinationCheckRemovesIt` — the sidecar enforces nothing.
- `TestAnUnnamedDestinationIsNotWrittenDown`, `TestTheSourceIsNamedBeforeTheDestination`,
  `TestTheEntryConditionIsTheNameTheUserGave`.
- `TestTheNamingLifecycleUnblocksTheLearnedPlay` — both names through the production path.
- `TestAGuardedPlayCompilesAndIsStillCoreMarco` — one success ending, after the arrival check.
