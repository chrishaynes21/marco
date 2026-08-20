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
  - cmd/marco/main.go
  - cmd/director/main.go
  - internal/director/learn
  - internal/plays/plays.go
---

# ADR-048 — learn, teach and do are three different sentences

A product-vocabulary decision, recorded during Roadmap 34 and implemented in Roadmap 35.

> **Implemented 2026-08-20.** When this was written it changed no code and licensed no rename. The
> rename it anticipated has since been carried out, vertically: the acquisition flow is spelled
> `learn` from the CLI verb down to `internal/director/learn`, and nothing on that path is named
> **Teach** any more, so the word is free for the feature this ADR reserves it for. See
> [[ADR-086-one-acquisition-one-word-one-request]], which also lists what still spells it and why.
> Everything below is left as written — in particular 'What was wrong' and 'Considered and rejected' quote the word in
> its wrong sense **as the evidence for this decision**, and rewriting them would destroy the
> record.

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
flow. (Written while that flow was still spelled `teach` internally; it is spelled `learn` there
too now — [[ADR-086-one-acquisition-one-word-one-request]].)

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

> **Since decided.** Roadmap 35 decided that they were, and carried the migration out once the
> milestone had closed rather than mid-flight — [[ADR-086-one-acquisition-one-word-one-request]]. The
> divergence this section asked a reader to expect is gone; nothing above is amended, because the
> hold and its reasoning are the record of why the rename waited.

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

When this was written the answer was "nothing in code, deliberately" — the enforcement point was to
arrive with the surfaces Roadmap 35 built. It has arrived, and [[Decisions]] is clear that an ADR
without one is an intention rather than a constraint:

- `cmd/marco` — `TestNoLiveAcquisitionCodeIsNamedTeach`: walks the acquisition packages and the
  product surfaces and refuses any identifier spelling the reserved word, with the compatibility
  aliases named one by one so they cannot grow silently. It deliberately does **not** police prose,
  so the record above stays legible. Mutation: reintroduce any Teach-spelled acquisition identifier.
- `cmd/marco` — `TestTheLearnVerbAnswersToItsOldName`: `marco learn` and `director learn` are the
  verbs, and `teach` still reaches the same function on both. Mutation: drop either arm of either
  case.
- `internal/plays` — `TestEveryKindIsPresentedAsItself`: a demonstrated play may not present as
  "Taught", because Teach is spent on the other direction of travel. This is the reservation held
  against a user-visible word.

## Related

[[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[ADR-045-teaching-is-a-section-of-the-playbill]] ·
[[ADR-046-grounding-a-screen-points-at-its-structure]] ·
[[ADR-086-one-acquisition-one-word-one-request]] ·
[[Demonstrations]] · [[Learned-Plays]] · [[Roadmap]] · [[Glossary]]
