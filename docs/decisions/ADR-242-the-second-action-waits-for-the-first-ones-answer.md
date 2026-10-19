---
type: adr
status: accepted
date: 2026-10-19
decision: "ACCEPTED — a step whose two ends are DIFFERENT screens no longer finishes on its destination arriving while its own outcome is still being decided. The arrival is kept, counted and reported with everything it rested on, and what it may not do is carry the walk forward by itself — the same shape as the thin-arrival rule beside it. A screen change is weaker evidence than an outcome, not stronger: the destination arriving says the interface moved, the outcome says it moved the way the demonstration showed. So on a two-action workflow the second action is not authorised until the first action's verdict is VERIFIED; contradicted and unconfirmed both stop the run, and nobody is asked to authorise an action whose predecessor did not work. The wait is bounded by the verification deadline rather than by patience, and a step demonstrated with NO outcome expectation arrives exactly as it did before — there is nothing to wait for, and gating on a question nobody can answer would stop every routine saved before outcomes were recorded."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflowrun/verify.go
  - internal/director/workflowrun/twostep_test.go
---

# ADR-242 — the second action waits for the first one's answer

## The gap

[[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] traced it and deferred it as
larger than a lifecycle fix:

> **Inter-step verification** (`internal/director/workflowrun/workflowrun.go`) can advance a
> different-screen step on destination arrival while a previous step's outcome verification is
> still pending.

Everything this subsystem does for ONE action had been built and measured. The thing that makes a
workflow out of it is a second action — and the gate between them was missing.

`Advance` asks what the observation says about the current step's destination, and a destination
that is established advances the cursor:

```go
if arrival.Established() {
    …
    a.Outcome, a.Because = OutcomeArrived, arrival.Because
    r.advance()
}
```

So a step whose two ends are different screens finished the moment its destination appeared, with
its own post-action evidence never consulted. The second action could then be authorised and sent
while the first action's outcome was still being decided — and the verdict reached afterwards,
about a screen the second press had already changed. The report would carry a verdict for step 0
that describes the world after step 1.

The same-screen case never had this hole, and not by design: such a step CANNOT arrive — its
destination is standing whenever its source is — so `verify.go` was the only way to finish it, and
`verify.go`'s policy was already the right one. *Nothing proceeds to another action on an
unconfirmed one.* The hole was in the path that rule had been written around.

## The decision

**A screen change is weaker evidence than an outcome, not stronger.** The destination arriving
says the interface moved; the outcome says it moved the way the demonstration showed. So where a
step's own outcome is still being decided, the arrival is kept, counted and reported with
everything it rested on — and what it may not do is carry the walk forward by itself. That is
word for word the thin-arrival rule sitting ten lines above it
([[ADR-200-a-destination-a-workflow-recognises-for-itself]]), applied to a second way of being
insufficient on its own.

Three answers and they are not two:

- **verified** — the step is done and the walk moves on, which is what makes a two-action workflow
  possible at all;
- **contradicted** — evidence against the action. The run stops and the second action is never put
  to anybody;
- **unconfirmed** — no evidence either way, at the deadline. The run stops. A verifier that could
  not tell this from the first would report success whenever OCR was slow, which is the most
  dangerous thing a verifier can do.

**The wait is bounded by the verification deadline, not by patience.** Past it the verdict is
`unconfirmed` and the run stops, which is why this cannot wait forever — and why the gate needed
no new timer of its own.

**And a step demonstrated with NO outcome expectation arrives exactly as it did before.** Its
verdict is `VerdictUnasked` forever: there is nothing to check against, every routine saved before
outcomes were recorded is in that state, and step 1 of the first routine this product performed by
hand is too. A walk that gated on it would stop on every such step. The report still says the
action could not be verified, which is the honest end for a file that cannot be checked.

`awaitingOutcome` is where that distinction lives, and it is the whole of the care in this change:
two of the three states `verdict` calls `Unasked` mean opposite things to the walk — *there is
nothing to ask* and *the question is in flight* — and telling them apart is the difference between
advancing over a pending verdict and stalling on a question nobody can answer.

## What this does not do

- **It does not make a two-action run likely to succeed.** It makes the second action conditional
  on the first being verified, which means a routine whose steps carry no outcome expectation, or
  a run without `--reading`, reaches the second action on the terms it always did or stops with an
  honest `unconfirmed`. The expectation has to be in the demonstration.
- **It is not a live confirmation.** Everything here is deterministic. The CLI trial — two
  authorised actions on a real game menu, the first verified before the second is asked about — is
  the owner's, and E2E.md walks it.
- **It does not revisit the same-screen path**, which already behaved this way, nor the arrival
  rules themselves.

## Enforced by

`internal/director/workflowrun/twostep_test.go`, over the production coordinator, the production
verifier and a notebook where the mouse would be. Each one asserts the thing an acceptance has to
assert and a unit test usually does not: not only the verdict, but **whether the second action was
ever asked about**.

- `TestTheSecondActionIsAskedAboutOnlyAfterTheFirstIsVerified` — the positive case, in order: the
  press goes out; the destination arrives and the step does NOT finish on it; nothing has been
  asked about step 1 and exactly one press has gone out; the outcome arrives and holds; the step
  is verified; and only then is the second action asked about and sent.
- `TestAContradictedFirstOutcomeNeverReachesTheSecondAction` — the panel says something else: the
  run refuses, the verdict is `contradicted`, and step 1 is never put to anybody.
- `TestAnUnconfirmedFirstOutcomeNeverReachesTheSecondAction` — nothing is ever read: the deadline
  passes, the verdict is `unconfirmed`, the run stops, and the wait is shown to be bounded.
- `TestAStepWithNoOutcomeStillAdvancesOnItsArrival` — the limit of the gate, which is what keeps
  it from being an overcorrection.

Three mutations, three killed, including the one that matters: restoring `advance()` on arrival
alone fails all three of the first tests. A fourth clause — asking about the step the walk is ON
rather than whichever verification is open — is structural: `advance` does not clear `r.verify`,
so a no-outcome step's verification really does outlive the cursor, and every path today reaches
the right answer without the clause.

## Links

- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]
- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
- [[ADR-200-a-destination-a-workflow-recognises-for-itself]]
- [[Passive-Observation]]
