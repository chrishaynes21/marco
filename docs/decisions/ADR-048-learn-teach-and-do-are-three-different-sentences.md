---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - demonstrations
  - learned-plays
  - visibility
source_paths:
  - docs/Roadmap.md
  - docs/Glossary.md
---

# ADR-048 — learn, teach and do are three different sentences

A product-vocabulary decision, recorded during Roadmap 34 and **implemented in Roadmap 35**. It
changes no code today and licenses no rename.

## The three sentences

Each one is distinguished by **who acts** and **who is the beneficiary of the acting**:

```
LEARN     Human acts.   Marco watches and learns.
TEACH     Human acts.   Marco guides them through it, visually.
DO        Marco acts.   Human delegates.
```

Or, as the person meets them: **Learn it → Teach me → Do it**.

## What was wrong

The word **Teach** was being used for the flow in which the *person* demonstrates something to
*Marco*. Read literally that is backwards — the human is the one teaching, and the sentence
`teach "open downloads"` reads as an instruction to Marco to teach, which is a different feature
that this project genuinely intends to build.

That other feature is not hypothetical, and that is why the collision matters. Marco already has
a learned semantic route between recognisable places and a visual-grounding primitive that can
point at one of them on the live screen. Pointing a person through a route they do not know is
a smaller step from there than any of the perception work already done — and it has no name left
if `teach` is spent.

## The decision

**Learn** is the person demonstrating and Marco acquiring. It is the product-facing name for the
flow currently implemented internally as `teach`.

```
Learn something

What should I learn?
> Open Downloads

Marco:  Show me.        [Marco visually grounds the START]
Marco:  Go ahead.       [the person performs the task]
                        [Marco visually grounds the DESTINATION]
Marco:  I think I got it. Show me once more.
Marco:  Got it. I learned "Open Downloads."
```

**Teach** is reserved for Marco guiding a person through something Marco already knows.

```
Teach me how to open Downloads.

Marco:  Start here.        [highlight]
Marco:  Now choose this.   [highlight]
Marco:  Now this.          [highlight]
```

The person performs every action; Marco points, explains, and follows their progress. This is
**not** written help. The whole idea is that Marco teaches against the person's *actual live UI*,
using the same semantic places and actions it learned.

**Do** is Marco performing a learned behaviour itself, under the existing authority, recognition,
rehearsal, verification and refusal rules, which this decision does not touch.

## The architectural consequence

Visual grounding acquires a second purpose, and stops being decoration around the demonstration
flow:

```
                Visual grounding
                       │
        ┌──────────────┴──────────────┐
     Learn                          Teach
"this is what I think              "this is where you
 you mean"                          should go next"
```

The canonical `VisualReferent` remains the basis for both. Treating a highlight as disposable UI
belonging to one flow is now a design error, not a shortcut.

## What this explicitly does NOT license

**No repository-wide rename.** `teach`, `TeachCoordinator`, `teachPass`, "teach session" and their
tests stay exactly as they are for the whole of Roadmap 34.

```
internal implementation vocabulary  :  may remain "teach"
user-facing product vocabulary      :  Learn
```

Those internals are heavily tested, Roadmap 34 is still open and mid-flight, and mixing a rename
into an unfinished end-to-end transition would destroy the one thing that makes the remaining work
tractable — the ability to tell a behaviour change from a spelling change. Roadmap 35 may decide,
separately, whether the internal names are worth migrating.

Until then a reader should expect the divergence and not treat it as drift.

## Considered and rejected

- **Rename now, while the meaning is fresh.** It is the cheapest moment in one sense and the worst
  in another: the E2E chain is *live*, with durable subjects on disk from a real run. A rename
  would invalidate the run in progress to save a later merge.
- **Keep "Teach" for both, and disambiguate by direction.** "Teach Marco" versus "Marco teaches"
  is a distinction a person has to hold in their head at the moment they are least equipped to —
  first contact.
- **Call the acquisition flow "Show".** It names the person's half accurately, but leaves Marco's
  half unnamed, and the thing being produced is not a showing, it is a learning.

## Enforced by

Nothing in code, deliberately — this is a vocabulary decision whose implementation is Roadmap 35.
The enforcement point arrives with the user-facing surfaces built there, and this ADR is what they
will be checked against.

## Related

[[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[ADR-045-teaching-is-a-section-of-the-playbill]] ·
[[ADR-046-grounding-a-screen-points-at-its-structure]] ·
[[Demonstrations]] · [[Learned-Plays]] · [[Roadmap]] · [[Glossary]]
