---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
  - vision
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: one-was-an-evidence-coverage-failure-and-is-fixed-the-other-is-an-identity-failure-and-is-not
supersedes: []
source_paths:
  - cmd/perceptionreplay/whynotfound.go
  - cmd/perceptionreplay/reload.go
---

# Experiment 034 — two controls, and why each was lost

**Status: RUN offline, 2026-10-13. No new capture, no inference during the measurement, no
actuation, no threshold was changed.**

## The question

[[Experiment-033-a-workflow-asked-for-in-another-session]] found two of four saved controls on a
bound screen. The owner asked for the two failures to be separated before anything new is built:

> If the source evidence isn't available during reacquisition, the failure is an integration or
> evidence-coverage issue. If the source evidence is present but the wrong control looks
> equivalent, that's an identity issue. Those require different solutions.

With the instruction that shapes the answer: **do not weaken matching thresholds to make these
controls pass.** Nothing here changes a threshold.

## A defect in the harness, found before the diagnosis could be trusted

The diagnostic's first run said "detector evidence available on 28 of 28 looks" about a screen
whose detections were never there, and the number was an artefact of how Experiment-033 asked.

Re-acquisition was run as a second pass over fold B's looks, asking its **finished** observer what
was on each one. An observer holds admitted detections until its observation moves on, so after a
fold its held set is whatever survived to the LAST look of the recording. Asking it about look 460
handed back look 900's detections, on every look.

So re-acquisition now runs **inside** a fold — a third observer over fold B's range, driven look by
look, asked before each look is folded, which is also the order the live path uses. Fold C's screen
is compared against fold B's on every look and any disagreement is printed, because the binding was
computed from B and applied to C.

**This changed the result**, and the direction matters: Experiment-033 under-reported. The corrected
run finds **3 of 4** controls, not 2.

## The two diagnoses

Run twice — pixel proposals alone, and with equivalent ONNX evidence available on both sides.

### `control_2` — the TRAINING tile. **Evidence coverage, and it is fixed.**

```
saved reading   0.2078,0.4907  0.1849x0.2046     (399,530 355x221)

pixels only   0 of 28 looks   best overlap 0.132 (structure)
              -> MISSING EVIDENCE: nothing near it was proposed at all

with ONNX    21 of 28 looks   best overlap 0.997 (vision)
              -> found
```

This is [[Experiment-028-the-tile-that-was-never-a-candidate]] again, one session later. The pixel
proposer cannot see the tile, so with pixels alone there is nothing on any look within 0.132 of the
saved rectangle. Give the second session the same kind of evidence the first one learned from, and
the control is found on 21 of 28 looks at an overlap of **0.997**.

The owner's hypothesis was right, and the answer to "does the result change when equivalent ONNX
evidence is available on both sides" is **yes, completely**. No identity mechanism was involved
and none would have helped.

### `control_6` — **identity, and nothing here fixes it.**

```
saved reading   0.4521,0.5741  0.0885x0.0130     (868,620 170x14)

pixels only   0 of 90 looks   24 agreed with the reading; all 24 ALSO indistinct
                              from a container.  best 0.924, nearest container 0.899
with ONNX     0 of 90 looks   24 agreed; all 24 indistinct.  identical.
```

**Adding the detector changes nothing**, which is what makes this the other category. The evidence
is present — something matches the saved reading on 24 looks — and it is refused, because the same
proposal is also the same rectangle as a region that merely CONTAINED the original press.

The rectangle says what this is: 170x14 at (868,620) is the **EXIT TO MAIN MENU label**, and its
container is the 352x34 button around it — [[ADR-182-two-controls-that-could-never-be-found-again]]'s
`target_6`. ADR-182 fixed this *within* a session with the second-press rule: a rectangle a press
identified the control as is not a container of it. Fold A covers looks 1-450 and contains only the
FIRST press on that control, so the rule never fired and the artifact was saved with the label as
its identity and the button as a container.

Mutual centre containment is not a size test. The label straddles the button's centre and the
button contains the label's, so the two are "the same rectangle" and no proposal can tell them
apart. That is an identity problem, and it is exactly the shape the owner predicted: *the source
evidence is present but the wrong control looks equivalent*.

### And one that got worse

```
control_7   pixels 16 of 19   |   with ONNX 12 of 19
```

Reported rather than smoothed over. More candidates means more chances for one of them to be
indistinct from a container — `nearest container 1.000` on this control either way. Richer
proposals are not free.

## What Director already has for visual identity, beyond rectangles

The owner asked for this inspected before anything new is commissioned.

| what exists | where | what it could distinguish | reachable from the observation tier |
|---|---|---|---|
| **64-bucket RGB colour histogram** with intersection similarity | `internal/screen/colorhist.go` | two rectangles of identical geometry whose contents differ — six menu tiles, a label against the button around it | **not as it stands**: see below |
| `Distinctive` — is a patch worth matching by image at all | `internal/screen` | refuses a near-uniform patch that would match anywhere | same problem |
| **OCR text with boxes** | `providers/ocr`, `plugins/ocr` | a control by the word printed on it | a subprocess; slower than the detector |
| **ScreenParser classes** | `providers/vision` | `button` vs `menu` vs `image` | already wired to the fast path |
| **Fusion's identity tiers** | `fusion/identity.go` | native id, automation id, then window/role/label/parent/position scored together | needs an accessibility tree, which a game does not expose |

Two things that look like answers and are not:

- **The detector's class is not identity.** The owner's sentence is exactly right: six Rocket League
  tiles are all `image`. A class narrows a field; it does not name a member of it.
- **`internal/screen` cannot simply be imported.** It contains `CaptureRegion` and
  `CaptureVirtual`. The observation tier's boundary tests exist to prove it cannot reach a live
  desktop, and importing a capture package to get a histogram would hand it one. The histogram
  itself is a pure function over an `*image.RGBA` with no dependencies beyond the standard
  library; it would have to be extracted, not imported.

## The smallest evidence-backed next step

Not a new model, and not a new matcher. **An appearance descriptor stored beside each reading,
used only to break the case that is currently refused.**

Concretely:

1. Extract the colour histogram and its intersection similarity into a capture-free package.
2. Store one histogram per saved reading, and one per container, in the artifact. 64 integers.
3. Where a proposal agrees with a reading AND is indistinct from a container — the 24 looks above,
   and **the only case this touches** — ask which of the two the proposal's current appearance
   resembles. If it is decisively the reading, the match stands; if it is decisively the container,
   the refusal stands; if neither, the refusal stands.

The restraint is the design. A descriptor that could create a match would be a second matcher with
its own error rate. This one can only resolve a tie the geometric matcher has already declared,
which means its worst case is the behaviour Marco has today.

**Measurable offline, with the outcomes the owner named**: `control_6` against its own container is
the positive case; the six PLAY tiles against each other are the negative case the class cannot
separate; two readings of one control across folds are the stability case; and a control whose
screen is absent is `no counterpart`. All on the existing recording.

For **screens**, the same descriptor is a far stronger signature than counts of rectangles in nine
boxes — but that is a bigger claim and should wait until the control case has been measured, because
Experiment-029's failure was a screen failure and a descriptor that fixes controls has not thereby
earned the right to be trusted with screens.

## What this does not establish

- **Nothing was recognised.** The screen correspondence in the underlying experiment is still
  supplied by an oracle. Preserved as the regression baseline it is.
- **`control_2` being found is not a general result.** It is one control, over 28 looks, in one
  recording, with the detector's evidence available on both sides.
- **The proposal above is unbuilt and unmeasured.** It is the smallest next step, not a result.

## Enforced by

`cmd/perceptionreplay` — the diagnosis goes through `observe.ReacquireStored` rather than
reimplementing `sameRect`, so it cannot disagree with the matcher it is diagnosing. Overlap is
reported with `diagnose.go`'s existing `iou`, which exists for reporting and says so.
