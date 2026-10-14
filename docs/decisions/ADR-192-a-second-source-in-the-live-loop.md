---
type: decision
status: accepted
date: 2026-10-14
affects:
  - passive-observation
  - execution
---

# ADR-192 — a second source in the live loop

## Context

[[Experiment-040-the-first-menu-Marco-opened]] got Marco through one real menu transition and then
stopped:

```
step 1: refused — unobservable: no proposal on this look agrees with any reading of this control
```

`control_2` is the Training tile on Rocket League's Play page. Running the production proposer
over a live capture of that page says why:

```
window 1920x1080, 256 proposal(s)
wanted   x  398.. 753  y  529.. 750  (355x220)

2 proposal(s) overlap the saved reading at all:
   100.0% of it   x  276..1920  y    0..1032  (1644x1032)
     0.2% of it   x  732.. 748  y  623.. 632  ( 16x  8)
```

The only thing covering the tile is 82% of the window, which `observe.Region.Locates` refuses
because the middle of it is the middle of the screen. Every other proposal on that page is a thin
horizontal strip in the left column: the proposer is resolving text rows, and a large tile is not
a text row.

The learned detector, over the same pixels, puts a box at `x 399..754, y 529..752` against a
saved reading of `x 399..754, y 530..751`. **IoU 0.991.**

This is the difference [[ADR-184-two-speeds-of-looking]] built two sources for, with a concrete
cost attached for the first time: one source cannot see this control at all, and the other sees
it exactly.

## Decision

`cmd/director/learnedeyes.go` runs the detector beside the live observation loop and hands its
results to the seam that already exists.

### Nothing about lateness is re-solved here

Inference is about 550ms on a 1920x1080 frame, against a loop that runs every 100ms. So a
detection describes pixels roughly ten looks old and arrives in an observation that may be of a
different screen. `Look.Slow` and `admitSlow` were built for exactly that: candidates carry the
epoch and generation of the frame they came from, the observer admits or refuses them against
what is standing now, and `SlowAdmitted` / `SlowStaleEpoch` / `SlowStaleWindow` / `SlowUnusable`
say which.

Offering is non-blocking and the queue holds one frame, newest wins. A busy detector costs
detections and never costs cadence — a live run offered 99 frames, answered 8 and dropped 89,
which is the design working rather than failing.

### Both sources, one matcher

Re-acquisition matches against the union of proposer regions and detector regions through the
same `ReacquireStored`: the same `sameControl` relation, the same container exclusion, the same
refusal when two candidates support one control and describe different ones. A detection is a
region like any other region, and what makes it a target is agreeing with a reading the
demonstration recorded.

Only the fresh ones. `fresh` drops candidates whose epoch or generation has moved — the same two
numbers `admitSlow` asks — because a target matched against a stale rectangle is a point on a
screen that has gone.

### A detection is a place, not a meaning

`Candidate.Class` carries the detector's word and nothing reads it. The Training tile comes back
as `image`, which is true of the pixels and is not a claim that it is a control. The comment
saying so was written when the field was added, before there was a live path to prove it on.

### The settings are the frozen ones

`visionChildEnv` supplies the model, 1280, conf 0.15 and iou 0.45 from the benchmark wiring — not
new values. A live path that swept its own thresholds would be measuring a different detector
from the one the decision record approved.

## What it cost to find out

Three things, and all three were already written down somewhere in the tree.

**The child environment was hand-rolled**, and `MARCO_ONNXRUNTIME` was left out of it. The plugin
then loaded whichever `onnxruntime.dll` the environment named and refused with *"The requested
API version [28] is not available ... Current ORT Version is: 1.26.0"* — the exact message
`defaultONNXRuntime` exists to prevent, three functions away in the same package. `visionChildEnv`
is the one place that knows what a child needs, and using it is the fix.

**`CLAUDE.md` names the wrong variable.** The code reads `MARCO_VISION_MODEL`;
`MARCO_SCREENPARSER_MODEL` only chooses which path to pass. And `MARCO_VISION_SIZE` defaults to
640 while the vendored model is `screenparser-1280.onnx`, so a bare `vision.exe detect` refuses
with `Got: 640 Expected: 1280`. The claim that the runtime and model "are chosen automatically"
holds for Director's own wiring and not for the plugin on its own.

**The build script's name is mangled in the docs** — `pluginsvisionuild.cmd` is
`plugins\vision\build.cmd` with its backslashes eaten.

## Consequences

- **A control the proposer cannot see is plannable.** `control_2` re-acquired at `(577,640)`
  against an agreed centre of `(576,640)`, lowered into legal Marco.
- **The limiting factor moves.** Execution works; perception now has two sources, and what Marco
  can see is a function of which sources are running.
- **`--learned` is opt-in**, and refuses to start rather than falling back quietly: a run asked
  for the detector and given the proposer would be a different experiment wearing the same name.
- **The detector is still not a Director dependency.** Without `--learned` nothing changes, and
  the plugin stays a plugin.

## Enforced by

`cmd/director` — `TestADetectionBecomesATargetOnlyByAgreeingWithTheFile`,
`TestAStaleDetectionIsNotMatchedAgainst`,
`TestTheLearnedSourceNeverBlocksTheLoop`.
