---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - execution
  - collections
  - programs
  - safety
---

# No verified progress means no repetition

## Context

An action did not visibly work. Retrying is the reflex, and it is right about a minority of
cases — a transient miss, a control that was not ready yet.

For the rest it is actively harmful. The most common reason an action appears not to have
worked is that it *did* work and verification could not see it. The second most common is
that it genuinely does not work, which repeating will not fix and which repeating makes
worse. An edit that could not be confirmed, applied twice, has silently doubled.

The same failure appears in a collection loop: without a progress test, an iteration that
cannot advance re-acts on the same member forever.

## Decision

**Repetition requires verified progress.** An action is repeated only when the Director has
positive evidence that the previous attempt did not take effect — never merely because it
could not confirm that it did.

Progress is *classified*, not assumed. An iteration that cannot demonstrate progress on the
current member stops rather than advancing or retrying.

## Amendment, 2026-10-02 — an unchanged world is not positive evidence

The rule above was being broken by the guard that enforces it.

`visualRetryVerdict` permitted a retry when the screen was byte-for-byte what it was, on the
reasoning that nothing can have happened and so nothing can be applied twice. That is not
positive evidence the attempt did not take effect; it is an ABSENCE of evidence, which this ADR
and [[ADR-006-unknown-is-not-false]] both say does not authorise a repeat. The guard's own package
note says the same thing in capitals — FAILURE TO CONFIRM IS NOT PROOF THAT NOTHING HAPPENED —
directly above the branch that concluded exactly that.

Measured against the acceptance fixture, whose Save button counts its presses and writes a status
label. Pressed a second time it set the label to the value it already held, so not one pixel and
not one element differed:

```
request: click Save   →  save_presses 1 → 3   ·  [result] failed
```

Two presses for one request, and then the word that invites a third. The application counted both.
An application may act on a press while moving nothing a screen can show: a counter nobody
displays, a message sent, a state it was already in.

There was also a contradiction in the justification. The retry is described in `pipeline.go` as
recovering from *"the target moved or the screen changed under us"* — and it was permitted only
when the screen had not changed. If nothing moved, re-observing finds the same target in the same
place and re-delivers the same action.

The other direction is [[ADR-137-a-thin-reading-is-not-an-answer]]: a look that failed BEFORE
acting is worth repeating, because nothing can have been double-applied. Same word, opposite
risk.

**So the retry now turns on DELIVERY evidence rather than on world change.** `verify.Delivered`
reports whether anything says the action reached the control Marco aimed at — keyboard focus
sitting on the clicked element, a control's own word about its selection — and an action that
arrived is never repeated. That is the positive-evidence test this ADR asked for, read the right
way round: the evidence says the attempt DID take effect as far as delivery goes, so there is
nothing to recover.

The complement is kept deliberately. A click that moved nothing AND took no focus may genuinely
have missed, and that is the one case a retry can fix.

`verify.WeightDelivered`'s note had already named the distinction the retry needed: delivery
evidence *"is what separates 'the button did nothing' from 'Marco clicked the wrong thing', and
those send a reader to two different places"*. A retry is for the second. One fact, two consumers —
the verdict weighs it below the threshold and concludes `unverified`
([[ADR-135-a-delivered-action-is-not-a-failed-one]]); the retry guard reads it and declines to act
again.

## Consequences

- The Director will sometimes stop when a retry would have worked, and will say why.
- Idempotence is never assumed on the Director's behalf. Some steps are safe to repeat, but
  that is a property of a specific step, declared, not a default.
- Collection loops terminate by construction rather than by an iteration cap alone.
- This interacts with [[ADR-006-unknown-is-not-false]]: "could not verify" is Unknown, and
  Unknown does not authorise a repeat.

## Enforced by

- **implementation** — `internal/director/execute/pipeline.go`: *"an edit that could not be
  confirmed is not repeated, because repeating it may apply it twice"*
- **implementation** — `internal/director/execute/iterate.go`: *"Iteration stopped: no
  verified progress for the current member."*
- **implementation** — `internal/director/execute/edit.go` — the multi-step double-apply case
- `TestADeliveredClickIsNotRetried` — one request, one press, when something says the click
  arrived; and the word is `unverified`, not the `failed` that invites a person to repeat it
- `TestAClickWithNoSignOfDeliveryIsStillRetried` — the complement, so narrowing the retry did not
  delete it
- `verify.Delivered` — the predicate, with the evidence kinds named once and read at both sites
- **milestone record** — *Progress is classified, not assumed* in [[director-collections]];
  *Verification* in [[director-programs]]

## Related

- [[ADR-006-unknown-is-not-false]]
- [[Collections]], [[Programs]], [[Editing]]
