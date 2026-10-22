---
type: experiment
status: partial
date: 2026-10-21
backend:
  - structure
fixture: "The owner's own Ark captures, five probes, not a protocol: the four preview PNGs saved by their `graphic settings` recording (640x360, downscaled from 1920x1080 by the preview path). Two pairs of real translucent game UI — the pause menu with the live world moving behind it (`screen_a`, `screen_b`) and the settings panel with the world behind it and then gone (`screen_c`, `screen_d`). Run with `go run ./cmd/localizetrial -real <a.png> -then <b.png> -click x,y -region wxh`. AND the nine 1920x1080 frames of `fixtures/vision/v2/rocketleague/pause-stable`, already in this repository, for the two-sided calibration and the resolution comparison: `go run ./cmd/localizetrial -calibrate <dir> -click x,y -region wxh -downscale N`. No window was opened, no input sent, and no image committed."
result: "Two measurements. FIRST, on the owner's 640x360 previews — translucency does not defeat a local template. A settings button taught on the settled screen scores 0.9241 on the translucent one and 0.9241 the other way; a pause-menu button with the live world moving behind it scores 0.9876. Their best rivals anywhere in the window score 0.59, 0.63 and 0.72. BUT two things that matter: the wrong-instance hazard is real on this UI — a repeated volume value ranks a DIFFERENT ROW above the demonstrated one, 0.9151 against 0.8926, which no threshold separates and only location does — and the threshold frozen against the opaque fixture does NOT transfer: those true matches at 0.89 and 0.85 sit below 0.90, so the guard refuses a control that is there. The acceptance threshold has to come from the demonstration, not from a constant. SECOND, two-sided calibration on the only full-resolution menu frames in the repository (Rocket League pause-stable, 9 frames at 1920x1080, an OPAQUE panel — so it cannot answer the translucent question): the same control at 1/1, 1/2 and 1/3 of capture resolution matches itself at 1.0000 every time while its nearest rival rises 0.5962, 0.6229, 0.7111 — separation 0.4038, 0.3771, 0.2888. Downscaling to the preview's resolution costs 28 percent of the separation and the self-match does not move at all, so a guard calibrated on self-matches alone is blind to exactly what degrades it. Calibration has to happen on full-resolution pixels."
supersedes: []
affects:
  - demonstrations
  - perception
source_paths:
  - cmd/localizetrial/real.go
  - internal/director/localizetrial/localize.go
---

# Experiment-097 — a translucent menu is not what defeats a local template

## Why this was the next thing to ask

[[Experiment-096-the-click-says-which-one-the-template-only-says-what]] answered its question on
a fixture built for it, and said so: *a fixture result does not establish support for translucent
game menus generally*. The Architect's reply named the proof point:

> I'd carry that approach forward as a guard around the learned coordinate, while keeping real
> translucent game UI as the next proof point — not claiming recognition is solved yet.

Real translucent game UI was already available and had cost nobody anything: the owner's
`graphic settings` recording saved a picture of each screen it saw, and two of those pairs are
the same menu with different things happening behind it. Nothing had to be run on their desktop
to ask this.

## What was probed

Five probes. A probe is not a protocol — there are no seeds, no case counts and no pass
criterion, and nothing here should be read as a second evaluation.

| taught in | located in | control | demonstrated scores | best rival | rival is |
| --- | --- | --- | --- | --- | --- |
| settled settings | translucent settings | RESTORE ALL SETTINGS | **0.9241** | 0.5927 | another button |
| translucent settings | settled settings | RESTORE ALL SETTINGS | **0.9241** | 0.6267 | another button |
| pause menu | pause menu, world moved | SETTINGS | **0.9876** | 0.7181 | the button above |
| settled settings | translucent settings | a volume value | 0.8926 | **0.9151** | **a different row** |
| settled settings | translucent settings | the same + its row label | 0.8548 | 0.8137 | a different row |

The local region was sized to each control rather than taken from the fixture's frozen 176x52,
which is calibrated to the fixture's own 160x36 button. At this capture scale that number would
have swallowed the neighbouring controls, and a region that silently means something else is
worse than one named on the command line.

## What it says

**Translucency is not what defeats a local template.** The thing that made whole-screen
recognition fail on this exact menu — see the measurements behind
[[Experiment-095-thirty-seconds-of-settling-that-had-already-settled]], where the same panel is
filed as two different screens depending on what moves behind it, one of them with no readable
words at all — leaves a template around the demonstrated point almost untouched. 0.9241 and
0.9876, against rivals at 0.59 to 0.72. The menu chrome is stable; it was the OCR and the
whole-window arrangement that were not.

**The wrong-instance hazard is real on this UI and not only in the fixture.** The volume list
holds seven identical values. Taught on one of them, appearance ranks a DIFFERENT ROW first —
0.9151 against the demonstrated 0.8926, a gap no threshold can be placed in. Whole-window
matching refused it as ambiguous, which is the safe answer and not a useful one; what separates
those two rows is where the person clicked and nothing else. This is 096's thirty wrong-instance
selections, in the owner's own window.

**And the frozen threshold does not transfer.** Both of the last two probes are TRUE matches
below 0.90, so the guard refuses a control that is there — safe, and an Act that can never run.
The acceptance threshold cannot be a constant carried from a synthetic fixture: it has to come
from the demonstration, where the recorder already watches the control over several looks and
can measure what its own template scores against itself.

**Context in the region helps the ordering and costs the score.** Including the row's label
lifted the demonstrated row to the top (0.8548 over 0.8137) and pulled every number down. It is
a real lever and not a free one.

## What capture resolution costs, measured

The caution that came back on the first result was the right one:

> positive self-matches alone can make the guard too permissive, so evaluate them against
> nearby non-targets as well

So the measurement became two-sided — what the control scores against ITSELF across several
looks, and what the best thing NEAR IT scores against the same template — and the number that
matters is the gap between them. `localizetrial -calibrate <frames> -click x,y -region wxh`.

The only full-resolution game menu in this repository is the Rocket League pause panel
(`fixtures/vision/v2/rocketleague/pause-stable`, nine frames at 1920x1080). It is OPAQUE: its
menu region is pixel-identical across all nine frames while the scene behind it is not (a patch
of the arena scores 0.9801 between the first frame and the last). **So the translucent question
at capture resolution is still unanswered** — see below.

What those frames do answer, with the same control and the same pixels at three scales:

| resolution | matches itself | nearest rival | separation |
| --- | --- | --- | --- |
| 1920x1080, as captured | 1.0000 | 0.5962 | **0.4038** |
| 960x540 | 1.0000 | 0.6229 | 0.3771 |
| 640x360, what a preview keeps | 1.0000 | 0.7111 | **0.2888** |

The rival is the sibling button 45px below — EXIT TO MAIN MENU against the demonstrated
SETTINGS. **Downscaling to the preview's resolution costs 28% of the separation**, and the
self-match does not move at all: it is 1.0000 at every scale. A guard calibrated on self-matches
alone is therefore exactly blind to the thing degrading it — which is the caution, with a number
on it.

It also explains the volume-row probe above. Separation falls with scale because what falls is
the distinguishing detail; a large button with large text keeps 0.29 of it at a third of
resolution, and a small value in a list keeps none — its separation is NEGATIVE at that scale,
which is what -0.0225 means.

**So calibration has to happen on full-resolution pixels.** The preview path keeps a 640x360
picture of a 1920x1080 window, which is the right thing for a person looking at a review and the
wrong thing to measure a guard from. The demonstration should keep a full-resolution crop of the
control's own region, which is small, which it is already cropping anyway, and which would make
every future demonstration carry its own calibration.

## What this does not say

- **The translucent case at full resolution.** The probes above are 640x360 previews; the only
  full-resolution menu frames in the repository are an OPAQUE panel over a nearly-static scene,
  so they cannot answer it. What would: one capture of a translucent menu over moving content at
  1920x1080 — or, better and without asking anybody to interrupt a game, a demonstration that
  keeps a full-resolution crop of the control region, which answers it permanently for every
  control anybody ever demonstrates.
- **Nothing with a count.** Five probes, two screens, four controls, no seeds.
- **Nothing about a control that moves.** Both images of each pair have the menu in the same
  place; this measured the background changing, not the layout.
- **Nothing about the press.** No input was sent and no outcome was checked. Localization only,
  as in 096.

## What it changes about the next step

The guard is worth building: it survives the condition that defeated the alternative, and the
hazard it exists to prevent is present on the owner's own screen. Two things come with it, and
both are engineering rather than research:

1. **The acceptance threshold is demonstrated, not configured.** The recorder sees the control
   over several looks; what its template scores against those looks is the number, and a control
   whose own looks disagree is one to say so about at recording time rather than at run time.
2. **The local region is derived from the control**, in the control's own pixels, and is named
   wherever it is reported. A constant is a number that means something different on every
   capture scale.

## Related

[[Experiment-096-the-click-says-which-one-the-template-only-says-what]] is the protocol this is
the next question after. [[Experiment-095-thirty-seconds-of-settling-that-had-already-settled]]
measured the same menu from the other side.
