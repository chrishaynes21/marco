---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - execution
  - perception
source_paths:
  - cmd/director/tryitout.go
  - cmd/director/surfacelive.go
  - cmd/director/surfacelearnphases.go
---

# ADR-209 — the assistant operates the harness

## Context

[[ADR-208-one-guarded-test-configuration-and-no-second-one]] ended with six commands and a list of
human actions, handed over for the owner to type. They rejected that as the testing experience:

> We've spent considerable effort building a system that lets someone control a computer through
> natural language. Requiring you to manually run six commands to test it defeats that goal.
>
> Do not hand me terminal commands and ask me to execute them. You are responsible for operating
> the test harness in my local development environment. … The goal is for you to test Marco's
> behavior, not operate Marco's development infrastructure.

The harness itself was not the problem. The division of labour was.

## Decision

**The assistant operates the harness. The person says when they are ready, demonstrates, decides
whether to authorise, and watches.** Nothing about the gates changes; what changes is who drives.

```
  assistant   build, launch, the disposable store, preconditions, the Stage check,
              starting each phase, reading its output, diagnosing a failure, cleaning up
  person      "I'm ready" · the demonstration · approve or decline · judge the result
```

### A front end, not a second harness

`director try-it-out` delegates every phase to the function `demonstrate-surface` already calls,
in the same order, behind the same `resetGuard`. There is no copy of any decision here, because a
copy is a thing that drifts. It adds exactly two things an operator needs and a person typing did
not.

**The disposable store, established by the command.** Under the OS temp directory, set in the
process environment so that the whole Director reads it from one place. An existing `MARCO_HOME`
is left alone: somebody who set one meant it, and if what they meant was the real store then the
guard refuses the experiment, which is the answer rather than something to route around.

**One checkpoint line per phase, in a closed vocabulary.**

```
  READY → OBSERVING → DEMONSTRATION NEEDED → LEARNED → READY TO EXECUTE → RESULT
```

Plus `BLOCKED`, which is what any of them becomes when its phase refused. An operator that reads
prose to decide what happened will eventually read it wrong, so the checkpoint is derived from the
exit code the phase already returns — it restates nothing and can therefore disagree with nothing.
`RESULT` prints either way: a click that did not do what was hoped still has a result, and filing
that as `BLOCKED` would hide an outcome behind a refusal.

### Two checkpoints belong inside the phase

`OBSERVING` and `DEMONSTRATION NEEDED` are printed by the code that reaches those moments, not by
the front end that started it. The reason is the whole point of a checkpoint: `DEMONSTRATION
NEEDED` is only true once the Stage, the live ambiguity, the durable place and a request that
actually raised the question have all succeeded, and somebody told to click before that is
pressing at a screen with no open question — a press that is dropped rather than banked.

### Nothing about the gates moves

One phase per invocation. The demonstration phase starts only when the person has said they are
ready. The one live click still needs `-authorise-one-click` on its own command line, and that
refusal happens before any runtime is built. Nothing retries. The three facts a demonstration
establishes are still reported apart, and `intended outcome verified` is still NOT IMPLEMENTED.

**"I'm ready" is not authorisation to press anything.** It licenses watching, and watching is the
phase in which the only thing that touches the desktop is the person's own hand.

## Consequences

**The six commands are no longer the workflow.** They still exist and still work;
`demonstrate-surface` is the harness and `try-it-out` is how it is driven.

**A failed precondition is the assistant's to diagnose.** The first live run proved the point
immediately — see [[Experiment-062-what-a-stage-is-made-of]], where the configuration was correct,
the provider list was correct, and the Stage was empty for a reason no amount of retyping would
have found.

**This is not Studio.** The owner was explicit that it should not be:

> I wouldn't build a full Studio UI just to solve this. Claude can orchestrate the first live test
> through its development tools. Once we've validated the interaction, we can bring that same
> experience into Studio.

## Enforced by

`cmd/director` — `TestTryItOutPressesNothingWithoutItsOwnAuthorisation`,
`TestEveryRefusalEndsInExactlyOneCheckpoint`, `TestTryItOutNeedsExactlyOnePhase`,
`TestTheDisposableStoreIsEstablishedWithoutBeingAskedFor`.

## Related

- [[ADR-208-one-guarded-test-configuration-and-no-second-one]] — the harness this drives.
- [[Experiment-061-the-live-focused-demonstration]] — the run it exists for.
- [[Experiment-062-what-a-stage-is-made-of]] — what the first operated run found.
