---
type: decision
status: accepted
date: 2026-10-02
affects:
  - execution
  - perception
source_paths:
  - internal/director/execute/pipeline.go
  - internal/director/target/target.go
---

# ADR-137 — a thin reading is not an answer

## Context

Twelve identical requests to one window, nothing changing between them:

```
focus the Name field   ×12  →  9 performed · 2 refused · 1 unverified
```

Both refusals were:

```
FAILED: the application exposed only containers and no content, so its interior was
        never visible — this is not evidence the target is absent
```

The same window answered the other nine times, so the thin reading was transient — a UIA tree
caught mid-population. Marco refused roughly one request in six because of a timing artefact, in
a sentence that correctly declines to blame the target and then offers the person nothing to do
about it.

The resolver had already drawn the right distinction and nothing consumed it.
`ResolutionUnobservable` means *"the world was never capable of answering"*, as against a world
that could see and whose answer is no — which `target.go` notes is the only point at which "there
is no Save button" is an honest thing to say. Both arrived at the pipeline as a terminal refusal.

`unobservable`'s own note even named the remedy: *"a shallow world needs another source, a stale
one needs re-observing."* Neither happened.

## Decision

**On an unobservable resolution, observe once more and resolve again. Then refuse.**

One more look, not a loop, and only for that verdict — re-asking a readable world would cost an
observation to be told the same thing.

## The asymmetry, which is the point

This sits a few lines above the action retry that the 2026-10-02 amendment to
[[ADR-007-no-progress-no-repetition]] had to NARROW, and it goes the other way for a reason worth
stating once:

```
before acting   a failed look is worth repeating; nothing can be double-applied
after acting    a failed confirmation is not, because the action may have landed
```

A resolution that failed sent no click and pressed no key — the desktop does not know Marco
looked. A verification that failed may be sitting on a real effect. Same word, "retry", opposite
risk, and conflating them is how a product ends up both unreliable and unsafe: timid where it
costs nothing and bold where it costs the user.

## Consequences

- Measured on the same twelve-trial protocol: **12/12 after, 9/12 before**. A small sample, and
  the one `unverified` outlier also did not recur, which may be luck; the shallow refusals were
  what this targeted and they went to zero.
- An unreadable window costs one extra observation before being refused. That is the right
  trade: the refusal is still honest and still names the observation as the blocker.
- It applies to program steps too, without a second site. `runStep` → `handleParsed` is the same
  function, which is why the measured multi-step failure (`type Marco into the Name field and
  then click Save`, step 1) is covered by the one change.

## Enforced by

- `TestAThinReadingIsLookedAtAgainBeforeRefusing` — a thin first reading followed by the window
  as it always was; the request succeeds, and acts exactly once.
- `TestAWindowThatStaysThinIsStillRefused` — a window that genuinely exposes nothing is refused,
  with the observation reason intact rather than "the target is absent", and is not looked at
  more than once more.
- `TestAnAbsentTargetIsNotLookedForTwice` — a readable world's answer of no is an answer, and
  costs one observation. Mutating the condition to re-observe on ANY refusal kills this.
