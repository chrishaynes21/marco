---
type: experiment
status: complete
date: 2026-10-13
backend:
  - vision
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: the-detector-proposes-the-tile-at-iou-0.93-on-every-settled-look-at-600ms-a-frame
supersedes: []
source_paths:
  - plugins/vision/cmd_detect.go
  - plugins/vision/backend_onnx.go
  - internal/director/perception/providers/vision/provider.go
  - internal/director/perception/providers/vision/structureshapes.go
---

# Experiment 030 — asking the learned detector for the tile

**Status: RUN offline, 2026-10-13, on the existing recording. No new capture, no actuation,
nothing wired into any production path.**

## The question

[[Experiment-028-the-tile-that-was-never-a-candidate]] established that the pixel proposer never
offers Rocket League's TRAINING tile as a candidate, that the tile's artwork shatters into 405
components none larger than 125x31, and that three attempts to make the proposer see it were all
rejected on measurement. It named the missing capability rather than forcing the existing cue to
supply it, and the owner authorised exactly one bounded follow-up:

> Proceed with one bounded offline ONNX capability probe using the existing Rocket League TRAINING
> tile frame. […] Determine whether it produces a usable candidate for the TRAINING tile. […]
> If the detector does not produce a usable candidate, report that directly. Do not tune it
> specifically for Rocket League during this probe.

With the caveat that framed the whole exercise:

> a generic learned UI detector may still fail on Rocket League's stylized menus. Having an ONNX
> detector in the repository doesn't guarantee it recognizes this kind of control. So this should
> be a capability test, not an integration commitment.

Four questions, answered in order below: does it find the tile, with what geometry and confidence,
does it separate the tile from its neighbours and the background, and what does it cost.

## What was run

`plugins/vision/build.cmd` (gcc 16.1.0, cgo, `-tags onnxvision`), then the plugin's existing
`vision detect <frame.png>` spike — which reads a FILE and captures nothing — against frames of
`rl-freeplay-1` already on disk.

The model is the one this repository vendors and already approved: `screenparser-1280.onnx`,
exported from `docling-project/ScreenParser` (`tools/vision-export/provenance.json`), 55 classes,
1280x1280 input. The calibration is the one `cmd/director/visionwiring.go` freezes —
`MARCO_VISION_SIZE=1280`, `MARCO_VISION_CONF=0.15`, `MARCO_VISION_IOU=0.45` — so this probe ran at
the configuration the rest of the tree already runs the model at, rather than at one chosen to
flatter it. Nothing was tuned, for this game or at all.

**`models/icon_detect.onnx` was NOT used.** It is a one-class OmniParser-style icon detector whose
class list the tree already records as a trap (`config.go`: it reported 56 desktop icons as
buttons). ScreenParser is the model the Director's own vision provider is calibrated against.

## 1. Does it find the tile?

Yes, on the frame the owner named — look 109, `frames/rl-freeplay-1-0108.png`, the look whose 196
proposals contained no tile at all:

```
20 element(s):
  [ 0] image   0.45  (761,256  395x247)      COMPETITIVE
  [ 1] image   0.43  (1184,256  394x246)     TOURNAMENTS
  [ 2] image   0.42  (779,529  357x222)      PLAY OFFLINE
  [ 3] image   0.42  (397,527  360x226)   <- TRAINING
  [ 4] image   0.39  (337,254  396x249)      CASUAL
  [ 5] image   0.34  (1162,529  355x222)     PRIVATE MATCH
  [ 6] button  0.33  (52,887  190x40)        BACK
  [ 9] button  0.24  (242,886  190x41)       QUICK PLAY
  [ 8] button  0.25  (1824,989  45x43)       friends
  … 11 more, all text/icon on real controls
```

All six tiles, not just the one. The proposer offered one of the six (TOURNAMENTS, because its
artwork quantises flat) out of 196 candidates.

## 2. What geometry and confidence?

The tile's true bounds were measured **independently of the detector being judged**, by luminance
profile across seven scan rows and seven scan columns bounded to the inter-tile gaps, since the
tile is a bright panel on a stadium in shadow:

| | rectangle | size | IoU with truth | area |
|---|---|---|---|---|
| ground truth | (391,521)-(761,758) | 370x237 | — | 1.00x |
| **detector [3]** | **(397,527)-(757,753)** | **360x226** | **0.928** | 0.93x |
| proposer's `target_2` slab | (0,476)-(778,706) | 778x230 | 0.345 | 2.04x |

Confidence **0.42**, against a model emission floor of 0.15 and a structural floor of 0.25. The
detection sits just inside the truth because the truth includes the selected tile's outer glow.

The recorded press (482,602) falls inside both the truth and the detection.

### It is not a single-frame fluke

Seventeen frames across both of the recording's visits to the PLAY menu:

```
14 of 14 settled PLAY-menu looks      tile detected, score 0.37-0.44
                                      geometry stable within +/-3px
 1 transition look (076)              main-menu list still fading out over the
                                      incoming tiles; no tile, and NOTHING proposed
                                      over the press point
 2 looks on the main menu (534,538)   a different screen; the tile is genuinely
                                      absent and nothing was proposed there either
```

The three looks without a tile are the honest cases: on none of them did the detector offer some
other rectangle covering (482,602). It was silent where the control was not, which is the
behaviour that makes a candidate usable rather than merely frequent.

## 3. Does it separate the tile from artwork, background and neighbours?

This is the question the owner was most precise about:

> Distinguish detecting the tile's text, artwork, container, and actual clickable bounds. A text
> label inside the tile is not automatically evidence of the entire clickable region.

The four are separable here, and the answer is specific:

- **the container is what was detected.** One box per tile, at the tile's own bounds, not at its
  artwork and not at its label.
- **the artwork produced nothing of its own.** No detection inside any tile. The stadium —
  46,203 components to the pixel proposer, and the source of the 778x230 slab that became
  `target_2` — produced **zero** detections.
- **the text is detected separately, and for TRAINING it was MISSED.** Five of the six tile labels
  came back as their own `text` boxes (`(344,471)`, `(765,471)`, `(1188,471)`, `(781,721)`,
  `(1162,721)`); the TRAINING label at `(397,721)` did not, because the selected tile renders its
  label without the dark bar the others have.

That last line is the finding worth keeping. **A text-first approach would have missed exactly the
control that blocks the workflow**, on exactly the screen where it matters, while succeeding on its
five neighbours — the quietest possible failure. The container was available when the text was not.

- **the clickable bounds are the container's**, and `ClickTargets.Press` takes the smallest
  proposal containing a press, so offering the tile is sufficient on its own: at 0.93x the truth's
  area and less than half the slab's, the tile wins the containment test without the slab being
  removed. Recall is the fix, as Experiment-028 argued.

### False positives

Across seven sampled frames spanning the recording, 5 to 26 detections each. The ones that are not
controls are few and identifiable: on the pause menu the **player's car** is detected as an `image`
(0.39), and a now-playing music widget as another. Both are honestly `image` — the model is not
claiming they are buttons, and `directorapi.ActuatingSource` already forbids a vision observation
from making anything operable.

For scale: the pixel proposer averages **204.7 proposals per frame** on this recording.

## 4. What does it cost?

Measured by a load/inference split added to the spike tool for this probe:

```
model load   ~450ms   once per process (97MB model)
inference    569-746ms per 1920x1080 frame, 7 frames, CPU
```

So roughly **1.5 frames per second**, against a fast-observer look of 55-65ms (15-18/s) and
`structure.Propose` at about 37ms.

This number decides the architecture and confirms the instinct the owner stated independently: the
detector **cannot** run in the fast loop. It is 10-16x a whole fast look. The tree already knew
this from the other direction — `providers/vision/structureshapes.go` records a 976ms median for
the same model on a Trackmania corpus and made pixels the default discovery path because of it.

## 5. Does it generalise beyond this screen?

The probe's own caveat was that a detector trained on desktop and web UI might simply fail on
stylised game menus. It did not, and it does not appear to be Rocket League specific:

| surface | detections | notes |
|---|---|---|
| RL PLAY menu | 20 | six tiles, BACK, QUICK PLAY, party banner, friends |
| RL pause menu | 16 | all four buttons (RESUME / CHANGE MODE / SETTINGS / EXIT TO MAIN MENU) |
| RL main menu | 26 | the eight-item nav list as `menu`, the challenge panel, its button |
| Trackmania (modded overlay) | 29 | mostly `text`; this surface is text-heavy and overlaid |
| Windows Settings fixture | 88 | `checkbox`, `menu`, `button` — correctly classed |

The pause menu is worth one line on its own: the four-button stack came back as four `button`
detections, and `EXIT TO MAIN MENU` — [[ADR-182-two-controls-that-could-never-be-found-again]]'s
`target_6`, the control whose identity had to be repaired because its reading was a label inside
its own container — is detected at `(794,607 331x38)` as one box.

## What this does and does not establish

**Established.** The missing capability is present in the repository, at the configuration the
repository already approved. The tile becomes a usable candidate — correct geometry, stable across
looks, silent where the control is absent, and separable from its own text and artwork — without
any change to the proposer, the matcher, `sameRect`, or target identity rules, and without a single
Rocket League specific heuristic.

**Not established, and deliberately not attempted.**

- **Nothing is wired.** The fast observer does not call this and still does not. No production
  perception path changed in this experiment.
- **A class is not an identity.** The tile is detected as `image`, which is *correct* — it is an
  image, and it is also the control. The detector supplies **localisation**, not authority, and
  the owner's distinction between geometric evidence and semantic identity is untouched by it.
  Nothing here says two readings of this rectangle are the same control.
- **One recording, one resolution, one theme.** Fourteen looks of one menu in one session. This is
  a capability answer, not a rate.
- **The 21-step workflow was not re-run.** It cannot be: the production proposer is unchanged, so
  the answer would be the same 1 of 21 measured at ADR-182. What it would become is a question for
  an integration nobody has authorised yet.

## Enforced by

Nothing new. This experiment changed one diagnostic — `plugins/vision/cmd_detect.go` now reports
model-load and inference time separately, because the probe was asked for a latency figure and the
tool reported none. The plugin's existing tests cover it; the default (null-detector) build is
unaffected and still compiles and passes without the ONNX tag.

## See also

- [[Experiment-028-the-tile-that-was-never-a-candidate]] — why the proposer cannot, and the three
  rejected attempts to make it.
- [[ADR-184-two-speeds-of-looking]] — the fusion boundary this evidence argues for, designed and
  deliberately unbuilt.
- [[Passive-Observation]] — the subsystem whose known gaps this changes.
