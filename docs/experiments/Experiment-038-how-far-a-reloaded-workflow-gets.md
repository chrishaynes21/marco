---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
  - vision
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: stage-two-blocks-because-bind-wants-every-screen-at-once-and-a-binding-lives-in-one-observation
supersedes: []
source_paths:
  - cmd/perceptionreplay/readiness.go
  - cmd/perceptionreplay/readinessstages.go
  - internal/director/workflowstore/anchor.go
  - internal/director/workflowstore/workflowstore.go
---

# Experiment 038 — how far a reloaded workflow gets

**Status: RUN offline, 2026-10-13. Read-only on every production contract. Nothing executed,
nothing authorised, nothing manufactured. Decisions a person would make are made by fixtures and
labelled `SIMULATED` wherever they appear.**

## The question

> The key question now is not whether each subsystem works in isolation. It's how far a reloaded
> workflow can proceed through the complete chain without borrowing evidence or authority from
> the original recording.

Fold A learns and saves. Fold B is a fresh observer sharing no counter, no registry and no screen
identity. The artifact crosses between them as 8,511 bytes.

## The chain

```
1. artifact reload and validation                READY
     8511 bytes, version 2, 11 steps, 8 screens, 7 controls; validated on the way in
2. fresh screen establishment                    REFUSED   [SIMULATED decision]
     12 screens established over the walk — 7 confirmed, 5 inferred;
     1 still live in the final observation
3. stored-control reacquisition                  not reached
4. control clarification                         not reached
5. fresh confirmation and authorisation          not reached
6. legal Marco lowering and compile validation   not reached
7. verification plan                             not reached
```

## The earliest genuine blocker, and it is a real one

Stage 2. And the refusal is not a perception failure — **twelve screens were established**, seven
by a (simulated) person and five by inference. The problem is the sentence after it:

```
1 still live in the final observation
```

**`Artifact.Bind` requires every screen of the workflow to be bound at once, and a binding is
valid only in the observation generation it was established in. Those two rules cannot both be
satisfied.** Establishing `screen_b` means the observation moved, which means `screen_a`'s
binding has expired. At most one screen can ever be live, so `Bind` can only succeed on a
workflow with exactly one screen.

Both rules are right on their own, and each was measured into existence:

- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] made binding all-or-nothing
  because *a workflow that bound what it could and left the rest would act before discovering it
  was lost*.
- [[ADR-186-a-person-can-say-where-they-are]] tied a binding to one observation because an
  arrangement recurs and a moment does not, and
  [[Experiment-037-why-the-binding-disappeared]] confirmed four of six revisits were real
  navigation.

They were written three days apart against different models of how a run proceeds, and nothing
had asked them to hold at the same time until now. This is what the readiness report was for.

## The smallest change that removes it

**Plan against the step's own screen, not the whole workflow's.**

`Bind`'s all-or-nothing rule exists to stop a run acting before it discovers it is lost. A
per-step requirement gives exactly that protection and is compatible with the observation rule: a
step whose source screen is not established in the current observation simply cannot be planned,
and the walk stops there — which is precisely how the ordered walk in
[[Experiment-032-what-the-recording-can-and-cannot-say]] already behaves.

Concretely: `workflow.Plan` already refuses when the screen standing now is not the step's screen.
What is missing is a binding lookup scoped to the current observation, rather than one Binding
built for the whole artifact up front. `SessionBinding.Established(label, epoch, generation)` is
already exactly that function; nothing consumes it in a planning path yet.

So the change is to stop requiring `Bind` before planning, not to weaken anything. `Bind` stays
as it is for the case it was written for — reporting whether a whole workflow could be
established — and stops standing between a reloaded workflow and its first step.

**It is not implemented.** This is the blocker and the proposal, as asked.

## Resolved, and rerun

[[ADR-187-knowing-the-plan-and-trusting-the-screen]] made the change this section proposed:
`Artifact.StepReadinessAt` grounds one step against one observation, and `Bind` keeps its
all-or-nothing meaning for the question it answers. Rerun with that path:

```
1. artifact reload and validation                READY
2. fresh screen establishment                    READY   6 screens, 1 confirmed, 5 inferred
3. stored-control reacquisition                  READY   3 steps found their control
4. control clarification                         READY
5. fresh confirmation and authorisation          READY   [SIMULATED authorisation]
6. legal Marco lowering and compile validation   READY   3 steps lowered and compiled; NOT run
7. verification plan                             REFUSED

STOPPED SAFELY after 3 of 11 steps
```

Six of seven stages, and three steps of a reloaded workflow lowered into legal Marco and
compiled. Without the authorisation fixture the chain stops at stage 5, which is what the flag is
for.

The next blocker is stage 7 and it is specific: `workflow.Verify` compares screen IDENTITIES, and
a reloaded workflow's destination is an artifact LABEL that nothing has bound — arriving there is
what would bind it. Verifying a reloaded step means asking whether the new observation
establishes as the destination label, which is screen establishment once more rather than a
comparison.

## What stages 3 to 7 would need

Reported as `not reached` rather than guessed at, but the prerequisites are known and worth
stating, because the next run of this report is the measurement that matters:

| stage | prerequisite |
|---|---|
| 3. stored-control reacquisition | a bound screen for the step; measured separately at 3 of 4 in [[Experiment-034-two-controls-and-why-each-was-lost]] |
| 4. control clarification | nothing — it runs on the artifact alone, and on the fold-A artifact it has one open question |
| 5. fresh confirmation | a bound workflow, and a person; the reloaded draft arrives unconfirmed, which is the half already proven |
| 6. lowering and compile | a planned action; `marcoexec.Lower` and `driver.CheckSource` are both reachable from here and neither has been exercised on a reloaded workflow |
| 7. verification plan | a lowered action; `Verify` must report `waiting` against an unchanged screen, which is how a plan is kept from claiming an outcome |

## What this does not establish

- **One recording, one fold split.** The blocker is structural and would appear on any recording
  with more than one screen; the stages after it are untested, not passing.
- **Every simulated decision was correct.** The fixture confirms what the oracle says is true. A
  person would be slower and sometimes wrong, and the report labels every such decision rather
  than averaging it away.
- **Nothing was executed.** Stage 6 compiles Marco source and stops. `workflow` still cannot
  reach the lowering path at all, which is the boundary ADR-181 holds and this did not touch.

## Enforced by

Nothing new. This is a read-only report over contracts that carry their own tests; its value is
that it runs them in order against one reloaded artifact, which no test does.
