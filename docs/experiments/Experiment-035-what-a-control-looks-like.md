---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: the-descriptor-reads-a-state-not-an-identity-and-the-ambiguity-is-not-an-appearance-problem
supersedes: []
source_paths:
  - cmd/perceptionreplay/appearance.go
  - cmd/perceptionreplay/appearancerun.go
  - internal/screen/colorhist.go
---

# Experiment 035 — what a control looks like

**Status: RUN offline, 2026-10-13. No new capture, no actuation, no threshold changed, and
NOTHING INTEGRATED — the conclusion is that it should not be.**

## The question

[[Experiment-034-two-controls-and-why-each-was-lost]] left `control_6` as an identity failure: a
170x14 label whose 352x34 button contains it, mutually centre-containing, refused on every look.
The owner approved one bounded experiment with the existing 64-bucket RGB histogram, and set the
bar:

> A successful `control_6` result isn't enough if the same rule introduces false matches
> elsewhere.

And named the exit:

> If histograms cannot reliably distinguish the label from its container, I'd move to spatially
> aware appearance evidence rather than keep tuning a weak descriptor.

Scope held exactly: appearance was examined **only** on looks where the geometric matcher had
already refused for indistinctness, it created no candidate, and it overrode nothing. References
were cropped from fold A and tests from fold B, so every comparison crosses the fold boundary,
and the verdict was scored against which rectangle a proposal actually overlaps more — a ground
truth the descriptor never sees.

## The answer: no, and for a reason worth having

### 1. Positive discrimination — **failed**

```
control_6   reading   0.4521,0.5741 0.0885x0.0130      (868,620  170x14, the label)
            container 0.4177,0.5648 0.1833x0.0315      (802,610  352x34, the button)

20 indistinct looks: appearance agreed with overlap on 17, disagreed on 3
margin (reading - container):  min -0.299   median +0.026   max +0.362
-> NOT SEPARATED: the margin changes sign, so no threshold divides these cases
```

Seventeen of twenty is better than chance and is not a separation. A rule needs a side of a line
to stand on, and this margin crosses zero.

### 2. Stability — **failed first, which is why the rest barely matters**

The descriptor was compared against itself: the same rectangle, on the same screen, across
twenty looks of fold A.

```
reading stability: 0.000-1.000, mean 0.403
  against the first crop:  0.91 0.90 0.91 0.90 0.91 0.00 0.00 0.00 0.00
```

That is a **step function**, not drift. Within one visit the crop agrees with itself at 0.90;
then it drops to **0.000** — no shared buckets at all — and stays there. The pause menu's EXIT
button is highlighted on some looks and not others, and at four levels per channel a highlighted
button and an unhighlighted one share nothing.

So the descriptor is reading a **state**, not an identity. It is less stable across two looks of
one control than it is discriminating between that control and its container, and no tuning
fixes that ordering.

### 3. Negative discrimination — **not tested, and saying so**

The intended negative case was neighbouring rows of one menu: the same button art with different
words, which a colour histogram should be unable to separate. The rows were picked by a size
filter over the proposals, and the filter selected rectangles of varied content rather than
genuine siblings — 28 pairs at min 0.142, median 0.340. That is a measurement of an arbitrary
set, not of neighbouring controls, and it is reported as a gap rather than as a result.

### And `control_7`'s "separation" is not evidence either

```
2 indistinct looks, margins -0.045 and -0.057  ->  "SEPARATED"
```

Two samples and a margin of five hundredths, both saying `container`, which overlap also said.
The label the report prints is the rule working; the number behind it would not survive a third
sample.

## The finding that matters more than the measurement

**This ambiguity is not an appearance problem**, and the geometry says why: the reading is
**nested inside** its container. Every crop of the 352x34 button contains the 170x14 label.

They are not two things that look alike. They are **two extents of one thing**. No descriptor of
the region can separate them, because the smaller one's pixels are a subset of the larger one's —
and OCR could not either, since both contain the same words.

What settles it is not a description of the rectangle but evidence about **what was pressed**:
[[ADR-182-two-controls-that-could-never-be-found-again]]'s rule that a rectangle a press
identified the control as is not a container of it. That rule needs a second press, and fold A
contains one.

The artifact already says this, in the field ADR-183 put there for exactly this purpose:

```
"clicked once, so nothing independent confirms that this control can be found again"
```

So the honest state is: Marco knows this control's identity was never settled, it says so in
words a person can answer, and it refuses. That is the correct behaviour and the descriptor was
never going to improve it.

## Recommendation: do not implement the tie-breaker

The admission rule the owner asked to be proposed if the evidence supported one is **not
proposed**, because the evidence does not support it.

Specifically, against the three requirements set: positive discrimination failed, stability
failed harder, and negative discrimination was not properly tested. A rule built on this would
be a second matcher with its own error rate, resolving ties by a quantity that changes sign
within one control's own looks.

If spatially aware appearance is pursued later, it should be aimed at the case this one was not:
**two different controls that look alike**, where a spatial descriptor has something to tell
apart. It should not be aimed at a reading nested in its own container, where the right answer
is already known and already refused.

The owner's caution was right and is now moot in a useful way:

> appearance matching may correctly identify the label without establishing the full button's
> clickable area

It cannot reliably identify the label at all, so the question of what that would license does
not arise.

## What this changed in the tree

- `internal/screen`'s `ColorHistogram` and `HistIntersection` are exported, so an experiment can
  ask this question without copying the implementation into a second place where the two could
  drift. **`internal/screen` still must not be imported by the observation tier** — it also
  contains `CaptureRegion` and `CaptureVirtual`, and the boundary tests exist to prove that tier
  cannot reach a live desktop.
- `cmd/perceptionreplay -reload -appearance` is the experiment, and it is a diagnostic. Nothing
  in any production path consults appearance.
- One defect in the experiment was found and fixed before its numbers were believed: reference
  crops were chosen by ranging a map, so the sample was different every run and the first
  stability figure could not be reproduced. The looks are sorted and evenly spaced now, and the
  screen they came from is printed.

## What this does not establish

- **One control, one recording, one descriptor.** A 64-bucket RGB histogram failing says nothing
  about a spatial descriptor, an embedding, or OCR.
- **Nothing about screens.** Explicitly out of scope, and Experiment-029's failure was a screen
  failure.
- **The negative case is still open.** Whether appearance would wrongly separate genuinely
  similar neighbouring controls has not been measured.
