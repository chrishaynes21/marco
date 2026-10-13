---
type: decision
status: accepted
date: 2026-10-13
affects:
  - passive-observation
  - navigation
---

# ADR-186 — a person can say where they are

## Context

[[Experiment-036-recognising-a-screen-you-were-shown]] located the failure precisely. Five
measures were compared over two independent folds, and the production arrangement relation beat
all four of the alternatives at *discrimination* — 6 of 6, worst margin +0.062. Every one of them
failed identically at *absence*: the two screens the second fold never saw were nominated
confidently and **unopposed** by all five.

So the missing capability was never a better ranking. The owner put it exactly:

> Similarity proposes. A person can establish correspondence. Fresh perception grounds the
> action. Authorization permits execution.
>
> Those four operations should remain distinct.

## Decision

A person may answer **one** question — *does the screen in front of you correspond to this saved
screen?* — with **yes**, **no**, or **not sure**. The answer lives in a `SessionBinding`, which
the artifact cannot reach and `Marshal` cannot write.

### Exactly what one Yes establishes

**One saved screen, as one current screen, in one observation generation.** Nothing else.

The last clause is the whole design, and it is the same rule a delayed detection lives under
([[ADR-184-two-speeds-of-looking]]): the observation generation moves on every screen change and
every window replacement, so leaving a screen and returning costs two increments and the old Yes
no longer holds. An arrangement recurs; a moment does not.

### What invalidates it

Only that: the observation moving. A binding from a previous observation is not returned, not
repaired and not downgraded — it is simply absent, which is the same answer as never having been
given. A revisit needs a fresh answer or a fresh inference.

### No, and not sure

**No** rejects that candidate for this observation and **does not promote the runner-up**.
Experiment-036's wrong winners beat their runners-up by 0.419; the ordering below a wrong answer
is not evidence of anything. **Not sure** changes nothing at all. Both are recorded, both need a
name on them, and the same candidate cannot then be confirmed in the same observation.

### Inference, and why it is allowed when similarity alone is not

A screen may be established without asking, when **both** of these hold:

1. the workflow itself demonstrates a step from a screen already established to this one, and
2. the arrangement agrees with the expected screen at or above the production floor.

Expectation supplies **presence** — the thing Experiment-036 proved no measure supplies.
Similarity supplies **agreement** — the thing expectation cannot, because delivery is not
success. Neither alone is admitted: without the transition this is the two wrong bindings of
[[Experiment-029-binding-a-screen-across-two-independent-folds]]; without the agreement it is
arriving somewhere and calling it arriving where you meant.

An inferred binding is marked `inferred`, not `confirmed`, and carries its **depth** — how many
transitions away from somebody who actually looked at something. A report that counted the two
together would say a person had answered questions they were never asked.

## What it measured

`perceptionreplay -frames -anchor` over `rl-freeplay-1`: fold A learns and saves, a fresh
observer walks fold B, and the oracle answers.

```
12 moments needed a screen established
 7 questions put
   7 answered yes, 0 answered no
 5 established WITHOUT asking, deepest chain 3
 0 left unresolved
```

And the number that matters more than the headline:

```
6 distinct saved screens were established at all
6 of the 7 questions were about a screen ALREADY established earlier in the same walk
4 moments resolved to the same screen as the moment before
```

**Only one question was about a new place.** The other six are the cost of tying a binding to an
observation: the observer loses a screen for a look and finds it again, which is a new
observation by this design and buys a new question.

Whether a transient loss *should* move the observation generation is a real question and is
**not settled here**. `A -> unobserved -> A` and `A -> B -> A` are different, and only the second
is the case the rule was written for. Measuring that is the obvious next thing and it is not
done.

The oracle is **not a simulated person**: it answers one narrow question perfectly and instantly,
which bounds the count and says nothing about whether somebody looking at a monitor could answer
at all.

## Consequences

- **A confirmation never reaches a file.** `SessionBinding` has no JSON tags, is not a field of
  `Artifact`, and a test asserts the saved bytes contain neither the confirmed screen nor the
  name of whoever confirmed it. Reloading a workflow tomorrow starts with nothing established.
- **Other screens stay unbound.** One answer binds one screen; `Bind` still refuses the workflow
  as a whole until every screen is accounted for.
- **The four operations stay distinct.** Workflow confirmation, screen correspondence, target
  re-acquisition and execution authorisation are four things, and establishing one establishes
  none of the others.
- **Nothing is live.** This is offline, over a recording, with an oracle in the chair.

## Enforced by

`internal/director/workflowstore` — `TestOneConfirmationBindsExactlyOneScreen`,
`TestAConfirmationDoesNotOutliveItsObservation` (screen changed, changed back, window replaced),
`TestARejectionEstablishesNothingAndPromotesNobody`, `TestUnsureAbstains`,
`TestEveryAnswerNeedsSomebody`, `TestAnInferenceNeedsATransitionAndAnAgreement`,
`TestAnInferenceCarriesItsDistanceFromAnAnswer`, `TestNoConfirmationSurvivesASaveAndReload`.
**Mutation: 15 of 15 killed.**
