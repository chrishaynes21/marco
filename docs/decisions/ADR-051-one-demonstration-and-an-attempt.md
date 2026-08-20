---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - demonstrations
  - learned-plays
  - semantic-memory
source_paths:
  - internal/director/observe/assessment.go
  - internal/director/observe/rehearsal.go
  - internal/director/learn/learn.go
  - internal/director/learn/say.go
---

# ADR-051 — one demonstration and an attempt

Learn required the person to perform the same route twice. It now requires it once, and Marco
proves it understood by doing the thing itself.

## What the second demonstration was actually costing

A live run had the correct durable route in the store —

```
subj_bef5e3d29af8 → subj_892a4cc30f41   observations 4
  [confirm] ×3, [down, confirm] ×1
```

— with the navigation attributed to the person, both endpoints established and both visually
grounded. Marco had observed exactly the thing it had been asked to learn. It then failed to learn
it, because the machinery for watching a *second* example did not manage to confirm the start, and
reported `demonstration_incomplete`.

That is the wrong product behaviour stated precisely: **Marco successfully observed what the user
wanted to teach, then refused to learn it because the user did not successfully perform an
additional ritual.** Every extra pass is another chance to fail at something that has already
succeeded, and the cost scales — a fifteen-step workflow would have meant performing thirty.

## The two gates, and why only these two

An audit found the requirement was not diffuse. It lived in exactly two places:

| gate | what it did |
|---|---|
| `AssessmentReason.ResolvableByDemonstration` → `NeedsAnotherDemonstration` → `teach.evaluate` | sent every first demonstration to `needs_another_example` |
| `ReasonSingleDemonstration` → `RefusalSingleDemonstration` in `RehearsalEligibility` | refused the rehearsal for the same reason |

Two things it was **not**:

- **Not `LearningThresholds.MinSessions`.** That gates `ReviewRelationships` — whether Marco may
  *ask* about a habit it noticed. Teach bypasses it by writing `LearningPending` directly. It is
  untouched, and now has the test coverage it never had.
- **Not the verdict.** `verdictFrom` already treats `single_demonstration_only` alone as not a
  downgrade, so a clean one-shot demonstration already computed `candidate_consistent`. The
  evidence bar was correct; only the demand for repetition blocked.

## The decision

**One clean demonstration reaches the offer to try. A successful rehearsal is the confirmation.**

```
demonstration → candidate → "want me to try?" → rehearsal → name → save
```

`single_demonstration_only` is reclassified, not deleted. It remains true and remains reported —
there *has* been one example — but it is now `ConfirmableByRehearsal`, the single member of that
set. Everything else that a second demonstration could resolve still blocks.

### Why an attempt is better evidence than a repetition

A second demonstration buys one more observation of the same kind. Marco performing the route and
arriving where it predicted is evidence of a different and stronger kind: that what was understood
is sufficient to act on. Only the *acting* tests that.

Refusing to rehearse for want of corroboration was also circular. The mechanism that would raise
confidence was the one being withheld for lack of confidence.

### Why exactly one reason moved

Every other resolvable reason names something Marco could not **read** — a run it could not tell
from hunting, a screen with no durable identity, a capture that may be missing its own end. An
attempt does not clarify a reading; it acts on one. Acting on a misreading is the failure mode this
whole subsystem is built to avoid.

`single_demonstration_only` is not a misreading. It is a count.

## Uncertainty is preserved, and made specific

One-shot is not "accept whatever came first". When something *was* unreadable, Marco still asks —
and now says what:

```
transient_checkpoint   "I know the route, but there's a screen along the way I can't
                        reliably recognise yet."
no_steps               "I saw where you ended up, but I couldn't tell what you did to get there."
ambiguous_run          "There was a lot of moving around in there and I couldn't tell which part
                        was the actual step."
backtracking           "It looked like you doubled back, so I'm not sure which way was the real
                        route."
```

Carried on `Session.Uncertain`. The old behaviour — *"I'd like to see that once more"* — is the
answer to every uncertainty at once, which makes it the answer to none of them in particular. A
second demonstration remains available as **recovery**, bounded by `MaxExamples`, and is no longer
the protocol.

## What did not change, and is now guarded

- **The rehearsal still needs an explicit yes**, through the ledger. That is where the safety was
  all along; the corroboration count was never what stopped Marco acting.
- **A failed or unattempted rehearsal learns nothing.** One demonstration is enough *because* of
  the attempt, so without the attempt there is one observation and nothing else.
- **Injected input is still unlearnable.** No pass observes a rehearsal.
- **A teach episode is still one session** ([[ADR-044-a-teach-attempt-is-one-episode]]).
- **Passive corroboration is unchanged** at `MinSessions: 2`. A habit Marco merely noticed has no
  explicit request behind it and no rehearsal coming, so cross-session agreement is the only
  evidence it will ever have.

## Audited and deliberately NOT built

- **Raw input capture — no, by design.** `InputEvent` has three fields and no string; the absence
  of a free-form field is the privacy guarantee. `admissibleInputs` zeroes `Where` on every
  non-pointer intent so a coordinate cannot be smuggled onto a keypress. Recording keystrokes,
  click coordinates and wall-clock timing would widen the privacy boundary, which is locked.
  Marco already separates record-from-execute in the form it can honestly support: it records the
  closed semantic vocabulary and executes a subset of it.
- **Semantic simplification — detected, not performed.** Lowering emits the observed run verbatim.
  `backtracking_run` and `ambiguous_navigation_run` *notice* redundancy and respond by asking
  rather than reducing. **The seam is rehearsal**: it already executes a candidate and verifies the
  endpoint, so a simplification pass would propose a reduced run and rehearse *that*. Nothing here
  forecloses it.
- **A recorder-like fallback for actions Marco cannot name — refused, and it should stay a
  decision rather than a default.** `RefusalCannotSayPointer` exists because a coordinate is not a
  meaning ([[ADR-042-a-click-is-a-place-in-a-window]]). A fallback that replayed raw positions
  would be both a privacy widening and the thing that ADR rejected.
- **Route composition** — `known route + new segment + known route` is not implemented. This
  change does not block it.

## Enforced by

- `internal/director/learn/oneshot_test.go` —
  `TestOneCleanDemonstrationOffersToTryRatherThanAskingAgain` (restoring the mandatory second
  demonstration fails it), `TestOneDemonstrationAndOneRehearsalIsEnoughToLearn` (the whole chain,
  three observation passes, one save), `TestWithoutASuccessfulRehearsalOneDemonstrationLearnsNothing`
  (no permission / ended elsewhere / unverifiable), `TestOneShotDoesNotMeanAcceptAnything`.
- `internal/director/observe/passivepolicy_test.go` —
  `TestPassiveLearningStillRequiresCrossSessionCorroboration`,
  `TestTheCrossSessionThresholdIsUnchanged`, `TestOnlyTheCountOfExamplesIsAnsweredByTrying`
  (widening `ConfirmableByRehearsal` fails it), `TestACleanFirstAssessmentDoesNotBlock`.
- `internal/director/learn/learn_test.go` —
  `TestAnotherExampleIsAskedForWhenSomethingWasUnreadable`, which also requires the sentence to
  name the uncertainty.

Eight mutations, eight caught: restore the mandatory second demonstration · accept every first
demonstration · save after a failed rehearsal · weaken passive corroboration · count a teach
episode as two sessions · observe a rehearsal · drop unattributed actions · rehearse without
permission. Every file restored byte-identically.

## Related

[[ADR-044-a-teach-attempt-is-one-episode]] ·
[[ADR-020-watch-me-is-permission-to-observe-not-to-act]] ·
[[ADR-042-a-click-is-a-place-in-a-window]] ·
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] ·
[[Demonstrations]] · [[Learned-Plays]]
