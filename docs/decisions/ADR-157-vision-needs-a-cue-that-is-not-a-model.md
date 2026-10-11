---
type: decision
status: accepted
date: 2026-10-10
affects:
  - passive-observation
  - vision
source_paths:
  - internal/director/perception/structure/structure.go
  - internal/director/perception/selection/group.go
  - cmd/perceptionreplay/main.go
---

# ADR-157 — vision needs a cue that is not a model

## Context

Marco's visual perception had exactly one source of geometry: box proposals from a learned
detector in `plugins/vision`. Grouping, the selection tracker and the interface belief all
consume what that model proposed, and none of them can see anything it missed.

Replaying [[Experiment-025-the-menu-was-the-wrong-screen]] through the production path measured
the consequence. The replay is new — `shadowtrace.go` had recorded every slot's regions and
shapes in full since it was written, saying its purpose was that "production and replay are the
same measurement", and nothing had ever replayed one.

```
usable stacks   42 of 107 looks had a group of >=3 rows; 65 had only smaller ones
stack sizes     8 rows x4   7 rows x6   6 rows x2   4 rows x2   3 rows x43
candidates      8; the stable one 3 rows / 32 judged / 0 exclusive / 32 quiet
shortfalls      judged_frames 7   exclusive_marks 2   transfers 2
```

Four minutes at a real menu, and the menu was present as a groupable list on about ten looks of
a hundred and seven. Seven of eight candidates failed for want of judged frames: nothing
survived long enough to be tested, because the evidence for it arrived one look in ten. **That
is a recall problem upstream of every stage that was being instrumented**, and no amount of work
on grouping or selection could repair it — those stages were starved.

It is also an independence problem. "Vision-first" cannot mean "vision, provided a particular
ONNX model is built and loaded": `go build` in `plugins/vision` produces a detector that detects
nothing, and the engine itself could not propose a rectangle from a screenshot at all.

## Decision

**A second, model-free source of visual structure, in the engine, with no external
dependencies.** `internal/director/perception/structure` proposes geometry from pixels using
two classical cues, and the learned detector becomes one source of evidence rather than the
only one.

Interfaces are drawn, and drawing leaves two cues that survive arbitrary content:

| cue | finds | fails on |
| --- | --- | --- |
| connected components of one quantized colour, filtered by rectangularity | panels, buttons, filled rows, fields | text on a ground that is not one colour |
| connected components of local CONTRAST against a box-mean background | text and icon rows, on any ground | a flat element on a flat ground of similar brightness |

Neither needs a word, a label, a model or an accessibility tree.

### Alternatives considered

- **A second learned model.** Rejected: the same dependency with the same failure mode, it
  cannot run in the engine (zero external deps is load-bearing), and the measured defect is low
  recall of plainly-drawn rectangles, which is not a problem a model is needed for.
- **Edge detection plus projection profiles**, the document-segmentation approach. Rejected as
  the primary cue: it assumes a quiet background, and a game renders a textured scene behind its
  menu so the whole frame projects as one band. Flatness and local contrast survive that.
- **Template or colour matching against known UI themes.** Rejected: application-specific by
  construction, and would not generalise past the first screen it was tuned on.

### The cues merge separately and union their LINES, not their marks

Measured, and it is the one structural decision inside the package worth stating here: pooling
both cues' marks before line merging let them destroy each other. A rendered scene produces tens
of thousands of small flat-colour components, hundreds in the text-height band, and pooled with
the contrast cue's marks they consumed the bands the real rows needed — the scene fixtures went
from 4 rows of 4 to 1 of 4 while the desktop fixture went from 4 of 6 to 6 of 6. The pooling
moved the failure between families and fixed nothing. Merged apart, a cue that is wrong about a
screen contributes nothing to it instead of taking the other down with it.

### Grouping now accepts alignment as well as equal width

`alike` required two rows to match in height **and width** within 35%. That encoded where
geometry came from: the detector proposes a box per row at the row's full width, so a menu's rows
all measure the same. Anything that finds text proposes the extent of the WORDS, and the words in
a list differ — "MAP" against "OPTIONS" differ by 83%.

Measured on a drawn six-entry settings list whose every row was correctly proposed: **the stack
came out empty.** Discovery complete, grouping refused all of it, and the belief reported
insufficient evidence — indistinguishable in the report from having seen nothing.

A list shares a common edge and a regular pitch, so left-aligned or centred now also counts as
alike, with a tolerance of half a row's height. Width is **not** dropped: similar widths still
qualify, so the detector's full-width boxes group exactly as before, which is what keeps the
measured Trackmania corpus scoring as it did. On the real recorded session the change is
marginal and positive (8-row groups ×4 → ×6, 426 rows against 404) and does not alter the
outcome — confirming that run's bottleneck was recall, not grouping.

## Consequences

**What the engine can now do that it could not:** propose interface structure from a frame, with
no model, no plugin, no accessibility tree and no network. Measured on 20 screen/highlight
combinations across four families, 101 of 110 list rows discovered (92%).

| family | rows found | note |
| --- | --- | --- |
| desktop settings list | 30/30 | complete under all five highlight styles |
| dark low-contrast list | 35/35 | complete |
| list beside a heads-up display | 19/20 | one miss, outline-only highlight |
| game menu over a rendered scene | 17/25 | the open gap below |

Cost: ~15ms per 1280×720 frame at the default 960px working resolution, against a measured
observation cycle of about 2.3s. It is affordable per look and is not a candidate for the fast
tracking path.

**What is NOT done, and matters most:** this is not wired into the live observation path. The
frame lives inside the vision provider, which is a plugin, so the engine has no pixels at sample
time; giving it some is its own change with its own provenance and privacy questions. Until then
Marco cannot use any of this in a session — the capability is real and measured, and it is
measured in tests.

### Measured on real pixels, afterwards

The claims above were made on drawn fixtures. [[Experiment-026-the-same-menu-seen-from-pixels]]
reprocessed 864 real frames of the Trackmania menu Experiment-025 failed on, against that run's
own L1 replay:

| | old detector, live | this, on the same game's pixels |
| --- | --- | --- |
| grouping produced a usable stack | 42 of 107 looks (39%) | 864 of 864 (100%) |
| a proposal covered the selected row | — | 829 of 829 labelled frames (100%) |
| a list identified | never | after 28 looks |
| frames naming an entry | 0 | 148, of which 147 matched the label |

Scored against model-assisted labels, which are not independently validated.

**And the cost reframes [[ADR-152-slow-discovery-fast-tracking]].** `Propose` is 37ms per
1920×1080 frame; the ONNX inference that sets the live cadence has a 976ms median. The stage
that made the cadence 2.3s — and therefore made selection tracking impossible, because a
57-second session yields 25 looks and the rule needs 10 judged ones — is about 26× more
expensive than the cue that outperforms it here. Reprocessing the same corpus at ~2.3s intervals
identifies nothing at all; at ~1.3s and faster it identifies the list and reads entries.

So the frame path into the engine should be considered as a REPLACEMENT for the inference on the
discovery path, not an addition beside it.

### Addendum, 2026-10-10 — it is now the default discovery path, in production

Authorised explicitly, on the cadence evidence above, with the condition that the change be
reversible, measured, and use ONE shared frame capture.

**Where it went.** Into `vision.Provider`, because that is where the frame already is. The
provider captures a window, hands the pixels to a detector, and already keeps a channel of
geometry nobody could name — `Shapes`, which reaches the interface belief through
`shadowShapes`. The proposer is a second SOURCE for that channel on the same captured frame.
No second screenshot pipeline exists, and none was created.

**It runs BEFORE the detector**, so a pass whose inference fails or is unavailable still carries
the geometry it could see. That is the point of not depending on one model: an ONNX backend that
is not built answers "no model loaded" to everything.

**`$MARCO_SHADOW_DISCOVERY`** selects `pixels` (default), `model` (the previous behaviour
exactly) or `both`. An environment variable rather than a constant so a run can be reproduced
and a regression bisected without a rebuild. An unrecognised value falls back to the default: a
perception path must not refuse to start over a typo in a diagnostic switch.

**Pixels mode does not also keep the detector's discards**, and that gate is load-bearing rather
than tidy. The proposer has already described the frame; keeping the detector's rectangles too
gives grouping two proposals for every row, and two boxes a few pixels apart define a pitch of
nearly zero — measured to split a correctly-discovered six-row list into one partial stack and
three strays.

**Every pass reports what discovery cost**, as `Timings.Structure` beside `Timings.Detect`, and
how many shapes it kept. The cadence argument is only honest if the number is measured per pass
rather than quoted from a benchmark, and it is also what would reveal the proposer becoming
expensive on some screen nobody tested. Measured through the production sampler on a drawn menu:
5 shapes in 26–31ms.

**What this does NOT establish.** That a live session is better. Every number in this ADR comes
from drawn fixtures or from replaying a recording. A Director now carries pixel-proposed geometry
through the sampler to the belief, with a detector that finds nothing, and whether that produces
a better interpretation of a live game is a question only a live run can answer.

One consequence worth stating: several existing tests had to declare `model` mode, because they
are detector fixtures — a painted ground with one lit row, where the nine rows they reason about
exist only as detections. Left on the default they would have kept passing on the proposer's
rectangles while claiming to measure the detector's.

### Open gaps, with numbers

- **A ground-changing highlight on a translucent panel over a scene loses the marked row AND its
  two neighbours.** Game fixture: 5/5 with no highlight or a brighter one, 2/5 filled, 2/5
  inverted, 3/5 outline-only. One unexplained interaction, not eight unrelated misses.
- **Precision on a rendered scene is poor**: 12% and 30% of proposals land on known structure,
  against 71% and 100% on flat grounds — about twenty proposals a frame that are the scene
  rather than the interface. This is the gap that matters second-most, because every false
  rectangle is a possible list member on every frame, and candidate churn was Experiment-025's
  dominant measured failure.
- **Near-duplicate proposals still split a stack.** Six correctly discovered rows group as
  `[3 1 1 1]`: a usable stack and three strays, because both cues return a proposal for some
  rows and `dedupe`'s mutual-overlap test does not catch the pair. A pair of boxes a few pixels
  apart defines a pitch of nearly zero.
- **Selection from pixels is unproven.** Whether a marked row is distinguishable by the
  two-feature shading measurement on these fixtures is not yet measured end to end, and the one
  stable candidate in Experiment-025 was quiet on all 32 of its judged frames.

## Enforced by

- `TestEveryRowOfEveryFixtureIsDiscovered` — per-family discovery floors set AT the
  measurement, so a regression fails and an improvement shows up as a floor to raise. The
  fixtures draw text from a 5×7 bitmap font rather than from blobs, so line merging is not
  tested against its own design.
- `TestProposalsAreNotMostlyNoise` — the opposite error, bounded, with the two bad floors
  recorded rather than averaged.
- `TestATexturedSceneProposesAlmostNothing` — a scene with no interface on it must not become
  structure, which is the precision claim that protects candidate stability.
- `TestAnEmptyFrameProposesNothing`, `TestProposalsAreDeterministic` — a frame of one colour
  invents nothing, and the same pixels give the same answer, which is what makes a recorded
  frame a fixture.
- `TestDiscoveredRowsGroupIntoOneList` — discovery and grouping kept apart, through the
  production grouper rather than a copy of it.
- `cmd/perceptionreplay` plus `TestTheTraceFormatIsTheReplayContract` — the measurements above
  on real recorded sessions, and a checked contract so a renamed tag cannot silently turn a
  recording into a report of a blind session.
