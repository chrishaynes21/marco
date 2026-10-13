---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
  - vision
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: the-completed-prefix-goes-from-1-of-21-to-9-of-21-with-no-false-matches-and-no-change-in-cadence
supersedes: []
source_paths:
  - internal/director/observe/candidate.go
  - internal/director/fastobserve/fastobserve.go
  - cmd/perceptionreplay/slowfeed.go
  - plugins/vision/cmd_batch.go
---

# Experiment 031 — the workflow gets nine steps in

**Status: RUN offline, 2026-10-13, on the existing recording and recorded detections. No new
capture, no inference during the measurement, no actuation, nothing durable written.**

## The question

[[Experiment-030-asking-the-learned-detector-for-the-tile]] established that the vendored detector localises the TRAINING tile that the
pixel proposer cannot see. [[ADR-184-two-speeds-of-looking]] designed the wire. The owner
authorised building it, and named the measurement that decides whether it was worth it:

> Does the learned 21-step workflow advance past step 1?

With a warning that turned out to matter more than the rest of the brief:

> There's another subtlety: adding ONNX candidates at replay time won't automatically repair a
> target identity learned from the stadium slab. The learning path must also be able to
> associate the original demonstrated press with the ONNX tile proposal. Otherwise, Marco may
> see the correct tile but still be looking for the wrong learned rectangle.

## How it was measured

`vision batch` runs the detector over all 900 frames once and records what it found —
900 frames, 17,007 detections, 0 failures, **mean inference 820ms**. The replay then plays those
back as a slow source: a request when a screen settles, at most one outstanding, and the answer
delivered **8 looks later** carrying the context the REQUEST was made in. 820ms at 10fps is 8.2
looks, so the delay is the measured one and not a convenient one.

Inference happens in the batch and never during the measurement. Running the model inside the
replay would have turned a measurement of the observer into a measurement of the machine.

## The result

| measure | pixels only | with detections |
|---|---|---|
| **completed workflow prefix** | **1 of 21** | **9 of 21** |
| where it stopped | step 1, `target_2` unresolvable | step 9, the recording ran out |
| controls resolvable somewhere | 12 of 13 | **13 of 13** |
| `target_2` reacquisition | **0** of 33 looks | **26** of 33 looks |
| **false target matches** | **0** | **0** |
| ground truth | 3 verified / 0 contradicted / 2 declined | **unchanged** |
| matched | 674 (52.4%) | 593 (46.3%) |
| ambiguous | 163 (12.7%) | 152 (11.9%) |
| unobservable | 450 (35.0%) | 537 (41.9%) |
| fold latency p95 | 0.51ms | **0.511ms** |

And the admission ledger, which is the integration reporting on itself:

```
requested 20   delivered 20   empty 0   skipped-while-busy 2
offered 249    admitted 193   held-and-offered on 543 of 900 looks
refused: stale-epoch 56   stale-window 0   look-not-evidence 0
expired while held: 164
```

### The product answer

**The workflow now runs nine steps instead of one**, and it does not stop because perception
failed. It stops at step 9 — an automatic transition — because *the recording ended while that
step was still waiting*. The walk ran out of demonstration, not out of controls.

### The owner's subtlety was the real risk, and it did not bite

The worry was that offering the tile at replay time would leave the learned identity still
pointing at the stadium slab. It does not, and the reason is structural rather than lucky: the
replay re-learns from the recording, so the press is attributed on a look where the tile is in
the candidate set, and `Press` takes the smallest containing rectangle. The tile is 0.93x its
true area and under half the slab's, so it wins on the rule that was already there.

The evidence that this actually happened is in the registry rather than in the prefix.
`target_2` carries **2 presses** where it previously carried one: both visits to the TRAINING
tile now resolve to one identity, where the pixel-only run minted a second identity for the
second press because the slab it had learned did not agree with the slab proposed later.

### Richer proposals did not introduce mistakes

**Zero colliding pairs, both ways.** Ground truth unchanged at 3 verified / 0 contradicted / 2
declined. Nothing became a false match.

What did change is that **matches moved into abstentions**, and that is the conservative
direction working as designed. A target whose identity is a detected rectangle can only be
re-acquired on a look where a detection is held — 543 of 900 — so on the others it is honestly
unobservable rather than approximately matched. The aggregate percentages are also not strictly
comparable between the two runs: the registries differ, because learning with a richer candidate
set allocates identities differently.

### Stale results were refused, in quantity

**56 candidates refused as stale-epoch.** The rule is not decorative: across a recording that
visits nine screens with 820ms of inference latency, a quarter of what came back described a
screen the observer had already left. A further **164 expired while held**, which is the same
judgement applied to candidates that had been admitted and whose screen then moved on.

`stale-window 0` and `look-not-evidence 0`: the window never changed under this recording and
no detection landed on a blind look, so those two counters are reporting their own absence.

### The fast loop did not notice

p95 fold latency **0.511ms** against 0.51ms. The observer never waits for inference — it is
handed answers that are already finished — and the ledger is what proves the second source was
actually there: held and offered on **543 of 900 looks**, not on the twenty it was asked on.

### How much latency the result can stand

The delay was swept, because a prefix that only appears at one convenient value of it would be
an artefact rather than a finding.

| delivered after | prefix | resolvable | admitted | refused stale |
|---|---|---|---|---|
| 4 looks (0.4s) | 9 of 21 | 13 of 13 | 297 | 0 |
| **8 looks (0.8s, measured)** | **9 of 21** | **13 of 13** | 193 | 56 |
| 12 looks (1.2s) | 9 of 21 | 13 of 13 | 141 | 45 |
| 20 looks (2.0s) | 9 of 21 | 13 of 13 | 92 | 60 |
| 40 looks (4.0s) | **1 of 21** | 12 of 13 | 34 | 93 |

The result holds across a five-fold range around the latency actually measured, and the model
would have to get **five times slower** before the benefit disappears. When it does disappear,
it disappears in the right direction: at 40 looks the admission rule refuses nearly everything,
and the workflow returns to **exactly the pixel-only answer** — 1 of 21, 12 of 13 — rather than
to a wrong one. A source too slow to be about the present stops contributing instead of
contributing something stale.

That is also the honest reading of the whole integration's safety: its failure mode is the
behaviour it had before it existed.

## Two design corrections the measurement forced

**1. A screen is not an observation.** ADR-184's first draft said a detection stayed admissible
while its SCREEN still stood. The owner rejected it before a line was written:

> Suppose Marco settles on screen A, requests inference, navigates to B, and returns to A before
> inference completes. The old result might now appear to belong to the current screen, even
> though the interface has changed.

That is correct and no arrangement comparison can fix it, because the two visits to A *are* the
same arrangement. The implementation therefore carries an **observation generation** — a counter
that moves on every screen change and every window replacement — and `TestReturningToTheSame
ScreenDoesNotRevalidateAnOldDetection` holds it.

**2. The duplicate rule nearly deleted the capability.** When both sources describe one
rectangle, one copy is kept, or agreement would be reported as ambiguity. The obvious rule keeps
the pixel copy so a stored reading never depends on inference timing. A test written with the
recording's real rectangles found what that costs: `sameRect` is deliberately loose, and a few
pixels of difference would make the 778x230 slab and the 360x226 tile "the same rectangle" — at
which point the tile is discarded as a duplicate, the slab wins the press, and **every admission
counter still reports success.** The more specific reading survives instead.

A third thing was found by the integration failing quietly: a detection delivered on one look
and then forgotten is useless, because the press that needs it lands about twenty looks after
the menu settled. Detections are **held** while their observation holds, and dropped all at once
when it ends.

## What this does not establish

- **One recording, one resolution, one window.** `stale-window 0` means that path has never been
  exercised by evidence, only by a test.
- **Nothing is live.** No detector runs in any live path; the Director's sampler still does not
  call the fast observer at all.
- **Nothing is durable.** No Place, no entry identity, no stored workflow. A draft is still
  session-local, and [[Experiment-029-binding-a-screen-across-two-independent-folds]]'s finding
  is untouched by this.
- **The prefix is bounded by the recording, not by perception.** Nine steps is where this
  demonstration ends, not a measurement of how far the design can go.
- **A class is still not an identity.** The tile is detected as `image`. `Support` is still
  two-valued, and `TestNoDecisionReadsTheDetectorsClass` holds the detector's vocabulary out of
  every decision.

## Enforced by

- `internal/director/fastobserve` — `TestADelayedDetectionIsAdmittedWhileTheObservationHolds`,
  `TestReturningToTheSameScreenDoesNotRevalidateAnOldDetection`,
  `TestADetectionIsRefusedOnceTheScreenHasChanged`,
  `TestADetectionFromAnEarlierWindowGenerationIsRefused`,
  `TestADetectionOnALookThatSawNothingIsRefused`,
  `TestAHeldDetectionIsDroppedWhenTheScreenMovesOn`,
  `TestTheArrangementDoesNotSeeSlowCandidates`, `TestNoDecisionReadsTheDetectorsClass`,
  `TestNeitherPixelSourceCanActuate`.
- `internal/director/observe` — `TestAdmissibleNeedsBothTheWindowAndTheObservation`,
  `TestTheMoreSpecificReadingSurvivesADuplicate`, `TestAgreementBetweenSourcesIsNotAmbiguity`,
  `TestTheMergedSetIsOrderedByAreaAndNotByArrival`.
- `cmd/perceptionreplay` — `TestADetectionArrivesAtTheDelayAndNotBefore`,
  `TestOnlyOneRequestIsOutstandingAtATime`, `TestAnEmptyAnswerStillCompletesTheRequest`,
  `TestNoFeedIsSilentRatherThanAbsent`.
- **Mutation: 16 of 16 killed.** Two survivors were found first and both were real: the
  admission rule's window clause was unreachable because the observer re-decided it (one
  decision, two labels now), and the arrangement test asserted a screen NAME, which is robust
  enough to hide contamination (it asserts the arrangement itself now).
