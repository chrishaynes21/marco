---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: the-tile-does-not-exist-as-a-component-and-three-candidate-changes-were-measured-and-rejected
supersedes: []
source_paths:
  - cmd/perceptionreplay/proposals.go
  - internal/director/perception/structure/structure.go
---

# Experiment 028 — the tile that was never a candidate

**Status: RUN offline, 2026-10-13, on the existing recording. No new capture, nothing executed.**

## The question

[[ADR-182-two-controls-that-could-never-be-found-again]] established that `target_2` fails
upstream of matching. The owner named the consequence precisely:

> Marco didn't fail to recognize the TRAINING tile. It never detected the tile as a candidate
> control in the first place. That's a different class of failure, and it tells us where to
> focus.

And set the standard for any fix:

> I'd investigate a second candidate-generation path for large, visually bounded controls, rather
> than loosening `sameRect` or replacing the existing proposer. […] First report candidate
> recall, candidate explosion, runtime cost, and whether the proposed mechanism generalizes beyond
> this specific screen. Implement only if the evidence supports a bounded improvement.
>
> If the tile requires a different perception capability, identify that explicitly rather than
> forcing the current proposer to solve it.

This reports. **Nothing in the proposer was changed**; three candidate changes were prototyped,
measured and reverted.

## Method

`perceptionreplay -frames -propose <look> -near x,y,w,h [-crops dir]` renders one frame with every
proposal outlined and lists the ones touching a rectangle of interest. A throwaway probe inside
the `structure` package then asked the same question one stage earlier, of the connected
components themselves. Only the probe is gone; the diagnostic is in the tree.

## What the proposer sees on the PLAY menu

Look 109, 1920x1080, the frame the TRAINING press was made on:

```
196 candidate(s):  block 3   line 193
the ten largest:
  line   690,   0 1230x714  fill 0.38  parts 1778
  line   308, 804 1612x226  fill 0.17  parts  419
  line     0, 134 1028x298  fill 0.18  parts  297
  line     0, 476  778x230  fill 0.28  parts  303   <-- this is `target_2`
```

Six large tiles are the obvious controls on this screen. **One of them is proposed** — TOURNAMENTS
arrives as a Block, because its artwork happens to quantize flat. The other five, TRAINING among
them, are not proposed at all.

And the ten largest candidates are all **Lines**: merged runs of hundreds of marks spanning most
of the frame. `target_2`'s "control" is one of them — 303 marks of stadium crowd merged into a
778x230 rectangle that happened to contain the press.

## Why the tile is not a candidate

The probe, at the working resolution of 960x540 (scale 2):

```
6598 components (+133101 too small)
the tile occupies working cells (197,262)-(378,377), 181x115
405 components overlap it; the twelve largest:
   339,376 125x31  fill 0.37  not rectangular (fill 0.37, border 0.19)
   363,367  68x18  fill 0.24  treated as a text mark
   295,276  49x18  fill 0.34  treated as a text mark
   …  every other one of the twelve: treated as a text mark
```

**There is no tile.** Inside a 181x115 region the largest component is 125x31, and the other
eleven of the twelve largest are glyph-sized. The artwork — a graduation cap, a dotted arc, a
gradient — shatters into four hundred pieces, and almost every piece falls inside the text-height
band, so the pieces are classified as marks and merged into lines. That is why this screen yields
90–153 proposals per look and why they are nearly all Lines.

The tile's border is not a component either, which was the hypothesis worth testing and is false:
anti-aliased over a varying ground, it does not survive quantization as one connected ring.

## Three changes measured and rejected

### 1. Make the line-height guard relative to the band, not to the taller mark

`mergeLines` refuses a merge that would make the band more than `1.15 x max(box.Dy(), g.Dy())`
tall. Whenever the incoming mark is the taller of the two, that test is nearly free, so a ladder
of ever-taller marks grows a "line" without bound. Changing `max(box.Dy(), g.Dy())` to `box.Dy()`:

```
before  1230x714 line, 1778 parts        after  1166x714 line, 1673 parts
```

**No material change.** The band compounds at 1.15x over hundreds of merges whichever height it is
measured against. Rejected.

### 2. Bound a merged line by the text-height ceiling

A line of writing has a bounded height and `MaxTextHeight` already says what it is; it was simply
never applied to the merged band. Adding `grown.Dy() <= opt.MaxTextHeight`:

- every frame-scale blob disappears, **including `target_2`'s 778x230 stadium slab**;
- proposals on this frame go **196 to 400**, which is the cap, so candidates are being truncated;
- `TestATexturedSceneProposesAlmostNothing` goes from **24 candidates to 265** on a bare rendered
  scene with no interface on it, and fails;
- the precision floors fail with it: the game-menu fixture drops to 4% of proposals landing on
  known structure against a floor of 10%.

This is the finding worth keeping from the whole experiment. **The merge is a noise sink.** It
collapses 46,203 components of scene texture into two dozen frame-scale rectangles, and that is
why a textured scene proposes almost nothing. Bounding it does not remove the false evidence; it
converts one useless candidate into hundreds of useless candidates. Rejected, and the package's
own test is what rejected it.

### 3. A third cue for outlines

A `Frame` kind: a low-fill component whose pixels trace all four edges of its own bounding box,
measured as the worst of the four edge occupancies. Per-edge and not overall on purpose — a streak
of texture lies along one side of its box, and only a drawn rectangle covers four.

```
per-edge occupancy   0.50 -> 7 frames    0.70 -> 1 frame    0.85 -> 0 frames
```

**None of them is the tile, at any setting.** The premise is false: there is no ring component to
find. Prototyped, measured, reverted.

## What this means

**The tile needs a perception capability Marco does not have.** Both existing cues are local and
component-based — a region of one flat colour, or a run of small marks on a baseline. A tile is
neither. It is a *region whose contents differ from its surround*, bounded by an edge too weak and
too anti-aliased to survive quantization. No arrangement of the current cues reaches it, and the
two attempts to stretch them both made the proposer worse in measurable ways.

Named explicitly rather than attempted, as the owner asked:

- a **region-level cue** — segmentation or local colour statistics over a window, rather than
  connected components of a quantized colour. Different in kind from what is there.
- or the **learned detector that already exists in the tree**. `plugins/vision` is behind
  `-tags onnxvision` and `go build` produces a detector that detects nothing; the `structure`
  package's own note says the detector "contributes semantic classes this cannot" and that the two
  are "separate evidence about the same screen". Nothing in this path unions them. A tile with
  artwork in it is precisely what a learned detector sees and a flat-colour segmenter cannot, so
  the next bounded question is whether the capability is missing or merely unwired.

## And the reason recall is the fix rather than suppression

`ClickTargets.Press` takes the **smallest** proposal containing a press as the control. The
stadium slab only became `target_2` because nothing smaller contained the click. Propose the tile
— 360x230, less than half the slab's area — and the slab stops mattering without being removed:
the press lands on the tile because the tile is smaller.

So the owner's framing is the one the evidence supports. The goal is to make the correct tile a
usable candidate, not to stop the proposer producing noise aggregates, and the two attempts to do
the latter both cost more than they returned.

## Enforced by

- `cmd/perceptionreplay` — `TestEveryRelationIsReachableAndOrdered` and the diagnosis fixtures
  cover the reporting; the proposer itself is unchanged, so no new test asserts anything about it.
- `internal/director/perception/structure` — `TestATexturedSceneProposesAlmostNothing` is what
  rejected change 2, and is why it is not in the tree.
