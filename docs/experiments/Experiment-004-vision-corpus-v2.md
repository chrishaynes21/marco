---
type: experiment
status: complete
date: 2026-08-07
backend:
  - classical-cv
game: rocket-league
fixture: fixtures/vision/v2/rocketleague
result: classical-cv-fragments
supersedes: []
source_paths:
  - internal/director/visionbench/groundtruth.go
  - internal/director/visionbench/truthmetrics.go
  - internal/director/visionbench/scorev2.go
---

# Experiment 004 — rebuilding the measuring stick

[[Experiment-003-classical-cv-tuning]] established that the vision benchmark could not measure
the thing it was being used to decide. This milestone rebuilt the measurement. It did **not**
build the corpus in its first half; Part 2 below is the corpus, captured the next day.

## What was wrong

Three defects, all measured rather than suspected:

1. **The score was anti-correlated with precision.** Removing 75% of a detector's false
   positives made the total drop 55.2 → 50.9. Temporal persistence carried 40 of 100 points
   and was measured by a proxy — "did this box keep appearing" — over six unrelated crops,
   where it degenerated into "did two rectangles fall in the same coarse ninth".
2. **Normalised geometry could not be calibrated.** A size floor of 0.045 removed 75% of
   false positives on the corpus and rejected 79 of 83 candidates on a real 1920x1080 screen.
   A crop's "fraction of frame" is not a screen's.
3. **False structure had no ground truth.** There was no way to say "this rectangle is arena
   texture", so a detector that clung to scenery for a hundred frames was rewarded for
   consistency.

## What was built

**A versioned ground-truth schema** (`groundtruth.go`) with a generic, game-agnostic
vocabulary and — the half version 1 could not express — **negative regions**: declared
scenery, where a detection is wrong by definition rather than by inference. Frames carry
`interface_present`, sequence membership and an index, and regions carry an `identity` that
ties the same control across frames. Validation reports every problem at once, refuses
game-specific kinds, and rejects a frame that both declares no interface and annotates one.

**Paired metrics** (`truthmetrics.go`). Every dimension has a partner that closes its
loophole:

| metric | closes |
|---|---|
| structural precision + recall | precision won by deleting real controls |
| nameable precision + recall | coverage inflated by relabelling boxes `button` |
| **temporal precision** + recall | *persistent false structure counted as trust* |
| OCR-region precision + recall | proposing many crops instead of good ones |

Temporal precision is the fix for defect 1: **of what persisted, how much was really there.**

**ScoreV2** (`scorev2.go`), all ratios against declared truth, weights documented at the point
of definition. Precision outweighs recall in both pairs, because this detector feeds a system
that acts on what it believes. Version 1's score is kept for historical reports, and a corpus
without ground truth returns **unavailable rather than zero** — a fabricated zero would rank a
good detector last for the corpus's failing.

**Corpus versioning.** The legacy corpus is now marked `legacy-crop-corpus` in its own
manifest and every benchmark report prints:

```
corpus       legacy-crop-corpus, committed, privacy-reviewed
WARNING      this corpus cannot calibrate normalised thresholds or temporal metrics
frames       960x540 largest
```

`CorpusVersion.Calibrating()` is false for anything but v2, and an unmarked corpus defaults
*down* to legacy — so a new corpus has to claim v2 deliberately.

## Proof the anti-correlation is fixed

Synthetic sequences whose truth is known by construction, because a metric validated only
against real frames is validated against whatever the annotator happened to mark.

The decisive case: two detectors over ten frames containing one real button. *Honest* finds
the button. *Sticky* finds the button **and** a patch of declared scenery, every frame. Both
are perfectly persistent; only one is persistently right. Under V1 the sticky detector scored
better. Under V2 it scores worse, and
`TestPersistentFalseStructureIsPenalisedNotRewarded` fails if that ever reverses.

Six mutations, each restored: negative regions ignored; persistent-false counted true; recall
weight zeroed; precision weight zeroed; nameable precision replaced by coverage; a
no-interface frame treated as unknown. **Four initially survived** — the tests were weaker
than they looked, in the same way the old metrics were — and two more tests were added until
all six fail.

## Grid stride, measured

[[Experiment-003-classical-cv-tuning]] left "the 8px grid may miss a 16px control at an
unfavourable offset" open and named a finer stride as the obvious fix. Eight controls placed
at successive sub-grid offsets, plus a real 1920x1080 capture:

| configuration | controls found | candidates (real screen) | median |
|---|---:|---:|---:|
| stride 8 (shipping) | 4 of 8 | 131 | 505µs |
| **stride 4** | **0 of 8** | 73 | 1.53ms |
| **stride 8, offsets 0/4** | **8 of 8** | 240 | 1.01ms |

**The obvious fix is the worst option.** A finer stride finds nothing and costs three times
as much, because of an interaction nobody had looked for: `MinSide` is a pixel floor applied
to `cells x stride`, so at stride 4 an 18px control resolves to 16px and is refused, while at
stride 8 the same control over-merges into a blob big enough to pass. Half of stride 8's
apparent recall is that accident.

Multi-offset finds every control at two thirds of stride 4's cost, because it changes *where*
the grid lands rather than how fine it is. **The default is unchanged**: whether doubling
candidates is worth it is a precision question, and precision needs corpus v2.

## What is not done, and why

- **The corpus itself.** Capturing full-resolution game frames is straightforward; deciding
  that an image of the user's screen may be committed to a repository is not mine to make.
  Every previous corpus stalled on exactly this. The capture-to-approval workflow is
  specified in [[Vision-Corpus-Workflow]] and needs a session with the user.
- **Rebenchmarking Classical, the shipping detector and Grounding DINO on v2** — blocked on
  the corpus. Running them against v1 again would reproduce a number already known to be
  unsound.
- **OCR execution evaluation**, grid and meter inference, temporal post-filtering: all
  measured through metrics the corpus does not yet support.

## Related

- [[Experiment-003-classical-cv-tuning]] — the measurement that established the defects
- [[Vision]], [[Roadmap]], [[Vision-Corpus-Workflow]]

---

# Part 2 — the corpus, captured 2026-08-07

## What exists

`fixtures/vision/v2/rocketleague`, **39 frames at 1920x1080**, five ordered sequences from one
Rocket League Free Play session. The user played; Director captured; no input was ever sent to
the game.

| sequence | frames | what it carries |
|---|---:|---|
| freeplay-static | 8 | gameplay, mostly stable camera |
| freeplay-camera-motion | 10 | **the load-bearing one** — arena rotates entirely while the boost meter stays fixed in screen space |
| pause-open | 6 | gameplay → menu, menu present from index 3 |
| pause-stable | 9 | 4-button menu held open |
| pause-close | 6 | menu → gameplay, gone from index 2 |

151 annotated regions, 117 negative regions, generic vocabulary throughout — `meter`, not
"boost"; `button`, not "resume button".

**Privacy: review was WAIVED by the user for this game, not performed.** The pause frames
contain an account name, club tag, level, party prompt and a music overlay naming a track and
artist, all in the bottom strip. Gameplay frames do not. This is recorded in the manifest, and
the strip should be cropped before the corpus is published anywhere.

## The result: Classical CV scores 0% recall on real frames

| configuration | dets | precision | recall | temporal precision | median |
|---|---:|---:|---:|---:|---:|
| no size filter | 4680 | 0% | **0%** | 0% | 1.02ms |
| shipping (0.015) | 4240 | 0% | **0%** | 0% | 1.02ms |

`tp=0, fp=1921, matched=0 of 151`. Not one detection reached IoU 0.35 with any annotated
element.

**The annotation was checked before this was believed.** On a pause frame, the detections
cluster exactly on the annotated panel's boundary — x 768-1160, y 416-680, against an
annotation of x 760-1159, y 388-671. The geometry agrees; what disagrees is granularity.
Classical CV fragments a 400x283 panel into about twelve edge slivers, the best of which
reaches **IoU 0.065**.

This is why the crop corpus flattered it. On a 240x240 crop an edge sliver covers a large
fraction of the frame and reads as a find; at 1920x1080 the same sliver is a sliver.

## What that does to Decision B

Decision B — *retain Classical CV as a region-proposal layer* — is **not supported as
measured**. A proposal layer's proposals have to correspond to things, and at element-level
IoU these do not.

It is not quite refuted either, and the distinction matters: the fragments **cluster on real
structure**. Twelve slivers tracing a panel's border is not noise, it is an unmerged outline.
A merge step over adjacent co-linear fragments might turn this into exactly the proposal layer
Decision B imagined, and that is now a specific, cheap, testable experiment rather than a hope.

What is settled is that Classical CV **cannot be promoted** and cannot be used as a proposal
layer *as it stands*.

## ScoreV2 validated on real evidence

| backend | precision | recall | temporal precision | ScoreV2 |
|---|---:|---:|---:|---:|
| box spammer (40 boxes/frame, all "button") | 5% | 26% | 2% | **12.7** |
| narrow but correct (the panel only) | 100% | 11% | 100% | **47.8** |

The spammer has more than twice the recall and scores a third as well. Under V1 volume was
rewarded; under V2 it is not, on real frames and not only on synthetic ones.

## Stride, on real evidence

| configuration | dets | median |
|---|---:|---:|
| stride 8 | 4240 | 1.01ms |
| stride 4 | 1166 | 2.56ms |
| stride 8, offsets 0/4 | 4680 | 2.05ms |

The synthetic finding holds in shape — a finer stride produces *fewer* candidates at more than
twice the cost — but recall is 0% at every stride, so the sweep cannot yet choose between them.
Stride is not the binding constraint; fragmentation is.

## Still not measured

The shipping detector (`icon_detect`) and Grounding DINO were **not** rerun. Both need their
model environments, which this machine does not have — `-tags onnxvision` plus
`$MARCO_VISION_MODEL` for one, a Python/torch install for the other. Marking them unavailable
is the honest outcome; predicting their numbers from a class list is what
[[Experiment-001-vision-backend-comparison]] already warned against.
