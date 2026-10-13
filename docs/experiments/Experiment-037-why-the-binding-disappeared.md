---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: four-of-six-repeats-are-real-navigation-two-are-one-capture-gap-and-no-rule-would-bridge-a-transition
supersedes: []
source_paths:
  - cmd/perceptionreplay/continuity.go
  - cmd/perceptionreplay/anchorrun.go
---

# Experiment 037 — why the binding disappeared

**Status: RUN offline, 2026-10-13. No observation-generation semantics changed, no detector
expiry changed, no binding restored, nothing implemented.**

## The question

[[ADR-186-a-person-can-say-where-they-are]] measured seven questions over one route, six of them
about a screen already established earlier in the same walk. The owner asked what those six
actually are before anything is built:

> First determine what those six repeated confirmations actually represent.
>
> The important number is not just how many questions could be eliminated. It's how many could be
> eliminated without carrying confirmation across an actual change.

## Two instrumentation defects, found before any number was believed

**The wrong signal.** The first run recorded `Observer.Screen()` — the screen the stabiliser
holds — and classified four gaps as `indeterminate` in which, by that record, *nothing whatever
had happened*. The observation generation moves on the id `Observe` RETURNS, which goes
`unobserved` while the stabiliser keeps holding. Both are recorded now, and the difference
between them is the whole investigation: a gap in **what was seen** is not a gap in **what was
there**.

**The wrong window.** The gap scan excluded its closing look. The generation moves *on* the look
whose identity changed, so that look IS the event that ended the binding. Including it moved two
gaps from `indeterminate` to `navigated away` — the correction went in the direction that costs
the proposal, not the one that flatters it.

## The result

```
navigated away      4
no pixels           1
transient loss      1   (the return leg of the same interruption)

2 of 6 repeated questions could be bridged safely
4 are real navigation
0 would be bridged wrongly
```

### Four of the six are `A -> B -> A`

```
looks 453 -> 628   visual_2, visual_3 observed in between
looks 644 -> 775   visual_4, visual_5, visual_6 observed in between
looks 775 -> 826   another screen observed in between
looks 826 -> 848   another screen observed in between
```

These are the case the rule exists for, and the first one is instructive: **the returning
observation agrees with the stored arrangement at 1.000**. The same menu looks identical on both
visits, and that is not evidence that nothing happened in between. A continuity rule built on
similarity alone would bridge it.

### Two of the six are one capture gap

One look with **no pixels at all**, at look 643. The identity went on the way out and came back
on the way in, so **one interruption moved the generation twice and cost two questions**. The
stabiliser never let go, nothing else was ever observed, the window never changed, and the
returning observation agrees at 1.000.

### And the generation is jumpier than the screen

```
of 12 observation-generation moves in this walk, 2 happened while the
stabiliser never let go of the screen
```

Ten of twelve are real changes of screen. The cost under investigation is **two moves out of
twelve**.

## The rule that was tested, and what it refuses

Four conditions, all required: nothing else positively observed, the window unchanged, the
stabiliser never having let go, and **the returning observation agreeing with the stored
arrangement on its own**. The last is the one that matters — a person's earlier answer is not
evidence about now.

It bridges the capture gap and refuses all four navigations. It cannot do otherwise: every
navigation has another screen positively observed, which the rule checks first, and a
`heldThroughout` that is false, which it checks as well.

That redundancy is recorded rather than removed. `TestASeenScreenAlwaysMeansTheStabiliserLetGo`
holds the implication — the stabiliser only ever returns a screen it has promoted, so a gap
containing another screen is a gap in which it let go — and two mutants are identified as
equivalent given it rather than counted as kills.

## The answers

1. **How many of the six have sufficient evidence for safe continuity: two** — and they are one
   interruption, not two events.
2. **How many remain genuinely uncertain: none.** The other four are definitely real navigation,
   positively observed.
3. **Would any proposed rule incorrectly bridge a real transition: no.** Not on this recording,
   and not by construction, since positive observation of another screen is checked first.
4. **The smallest justified change: none, yet.**

## Why "none, yet"

A suspended-confirmation concept would save **2 questions of 7** on this route, and would add a
new state to the one mechanism whose safety rests on bindings simply expiring. The dominant cost
is real navigation — four of six repeats — which is correct behaviour and not a cost to remove.

And there is a cheaper candidate that this investigation was explicitly forbidden to touch: the
observation generation moves on the RETURNED identity, while the stabiliser exists to hold a
screen through exactly the noise that makes an identity flicker. The same two moves of twelve
account for the whole gap. Moving the generation on the STANDING screen instead would be a
smaller change than a new continuity state — but it also governs **detector expiry**
([[ADR-184-two-speeds-of-looking]]), where a longer-lived generation means stale detections
surviving a blind look. That is why it was ring-fenced, and it should stay ring-fenced until
measured on the detector path too.

The cheapest thing of all is upstream of both: **there was one blind look in 450.** Asking why
the capture failed is a smaller question than either of the above.

## What this does not establish

- **One route, one recording, six repeats.** Two bridgeable cases is two.
- **The oracle answered every question correctly and instantly.** The question count is a lower
  bound.
- **Nothing was changed.** No generation semantics, no detector expiry, no restored binding.

## Enforced by

`cmd/perceptionreplay` — `TestATransientLossIsNotANavigation`, `TestANavigationIsNeverBridged`
(whose arrangement agrees at 1.000, which is the trap), `TestATransientLossStillNeedsTheReturning
ScreenToAgree`, `TestAWindowReplacementIsNeverBridged`,
`TestAnObserverThatLostTheScreenEntirelyIsNotTransient`,
`TestTheReturnLegOfAnInterruptionIsNotASecondEvent`,
`TestASeenScreenAlwaysMeansTheStabiliserLetGo`.

**Mutation: 8 of 8 killed**, with two further mutants identified as provably equivalent given the
implication above and recorded rather than counted.
