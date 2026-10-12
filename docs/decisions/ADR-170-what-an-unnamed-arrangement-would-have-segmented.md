---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
source_paths:
  - cmd/perceptionreplay/screens.go
  - cmd/perceptionreplay/frames.go
  - internal/director/observe/screenstate.go
---

# ADR-170 — what an unnamed arrangement would have segmented

## Context

The owner's product direction for the first usable version is click-target navigation:

> Marco's first intelligent behavior should be "I saw what you clicked, I learned where it took
> me, and I can find and click that target again."

[[Experiment-027-what-did-you-click-and-where-did-it-take-you]] recorded that demonstration in
Rocket League and established the first half cleanly: **17 of 17 presses landed inside a region
the model-free proposer found in the pixels of that same frame.** The second half did not move
at all. Marco's screen model over 900 frames was `1 states, 0 transitions`, so "where did it
take me" had no answer, and three of the owner's four acceptance questions failed on that one
fact.

The mechanism is not mysterious and is already written down. `ScreenSignature` is built from
NAMED roles; the structure proposer names nothing; `signatureSimilarity` answers **1** for two
empty feature vectors on purpose, because *two screens with no structural evidence are the same
kind of screen* is the right answer for sparse gameplay. Four menus are therefore four looks at
`state_1`.

Closing that means letting unnamed geometry into a signature, and [[Passive-Observation]] is
explicit that this is a decision of its own rather than a widening, because a signature is the
choke point for what reaches durable cross-session memory. A decision of that size should not be
taken on a hunch about whether the cue would even work.

So this decision is: **measure it first, on the recording that exists, with production's own
arithmetic, and change nothing.**

## Decision

`cmd/perceptionreplay/screens.go` reports, per recording, what the segmenter WOULD have produced
if every proposal carried a generic kind instead of none. It is a measurement and it is labelled
as one in the output, beside Marco's actual belief.

Three choices make it honest rather than flattering:

- **One token for everything.** Every region gets the kind `pixels`, so the only discriminator
  is arrangement — how many proposals sit in each cell of the segmenter's existing 3×3 grid.
  That is the weakest possible version of the cue; anything a richer vocabulary could add is
  upside on the number. Enforced: the signature must carry exactly one distinct kind, because a
  kind that varied with a region's size or position would measure a cue nobody has.
- **Production's own similarity and threshold.** `observe.SignatureSimilarity` at
  `observe.StateMatchSimilarity`, which is exported for exactly this reason — a characterisation
  whose numbers drifted from the thing it characterises is worse than none.
- **No promotion count.** This asks what the signature can SEE. `StatePromotionCount` is about
  how long to wait before trusting it, and including it would mix the two questions.

Nothing is wired into the live path. The segmenter is untouched.

## Measured outcome

On `rl-freeplay-1` — 900 frames, Rocket League, Play → Training → Free Play demonstrated twice
with the mouse:

```
pixel screens       7 distinct arrangement(s), 150 change(s) between adjacent frames
                    #0   475 frame(s), first at 0,   241 region(s) over 9 cell(s)
                    #1   135 frame(s), first at 77,  108 region(s) over 9 cell(s)
                    #2    71 frame(s), first at 145,  62 region(s) over 8 cell(s)
                    #3    16 frame(s), first at 193,  69 region(s) over 8 cell(s)
                    #4     2 frame(s), first at 211,   0 region(s) over 0 cell(s)
                    #5   178 frame(s), first at 218, 400 region(s) over 9 cell(s)
                    #6    23 frame(s), first at 338, 117 region(s) over 9 cell(s)
presses followed by
a different screen  13 of 17
Marco's own screen model on this recording is 1 state(s), 0 transition(s).
```

**The cue works.** Seven arrangements against one state, and 13 of 17 presses followed by a
different one. And the route is visible in it: the sub-sequence `#0 → #1`, `#1 → #2`, `#2 → #0`
appears in both demonstrations, at presses 2–4 of the first and 2–4 of the second.

| press | run 1 | run 2 |
|---|---|---|
| 1 | #0 → #1 | #0 → #0 |
| 2 | #0 → #1 | #0 → #1 |
| 3 | #1 → #2 | #1 → #2 |
| 4 | #2 → #0 | #2 → #2 |
| 5 | #3 → #0 | #2 → #0 |

### And it is noisy, which is the part that decides how to build it

**150 changes between adjacent frames over 900 frames**, against seven distinct arrangements. So
the cue separates the screens and then flaps between them: a route built naively on it would see
roughly one spurious transition every six frames. Three things are worth saying about that before
anybody treats the 7 as a result:

- `StatePromotionCount` exists precisely to damp this and was deliberately left out. The real
  segmenter requires an ambiguous composition to recur before it becomes a state.
- Cluster #4 is two frames holding **zero** regions — a blank or mid-transition frame, which is
  the `similarity(empty, empty) = 1` case forming its own cluster. On a path where unnamed
  geometry counts, a frame with nothing in it is no longer "the sparse gameplay state"; it is a
  gap, and it would need handling that does not exist.
- Seven arrangements for four or five real screens means the quantisation is doing some of the
  separating by accident. 3×3 cells with raw counts is sensitive to how many rectangles the
  proposer happened to find — #0 at 241 regions and #5 at 400 may be one screen seen under two
  proposal yields rather than two screens.

### The four presses that changed nothing

Three of them sat in #0 → #0, one in #2 → #2. Either the click did not change the screen, or the
cue missed a change it made. **This measurement cannot distinguish those**, and nothing in the
recording can: there is no independent account of what the game did.

## Consequences

- **Giving segmentation an unnamed-geometry cue is now a decision with evidence behind it rather
  than a hunch.** It is still its own decision, for the reason it always was: the signature is
  what reaches durable cross-session memory, and a screen identified by an arrangement of
  rectangles is a much weaker identity to make durable than one identified by roles. The
  measurement says the session-local half would work. It says nothing about whether the durable
  half should.
- **The noise is the design problem, not the separation.** Any wiring of this has to answer the
  150 adjacent-frame changes, the zero-region frame, and the proposal-yield sensitivity. A
  version that reported seven screens and a transition storm would be worse than one vacuous
  state, because a transition storm looks like evidence.
- This stays a diagnostic in a diagnostic tool. Nothing in `internal/` changed, and Marco's
  belief about the screen in front of it is exactly what it was.

## Mutation gate

16 mutants, all 16 killed.

## Enforced by

- `cmd/perceptionreplay` — `TestAFrameWithNoNamedStructureMatchesEveryOtherOne` (the mechanism
  behind the one vacuous state, asserted rather than described),
  `TestWithAGenericKindTwoArrangementsAreTwoScreens`,
  `TestAScreenChangeIsCountedBetweenAdjacentFrames`,
  `TestAPressIsAskedAboutTheScreenAfterItSettles`,
  `TestAnL0RecordingReplaysThroughPerception` and
  `TestAPointerPressReachesPerceptionWithItsPosition` (that the measurement is reached at all).
