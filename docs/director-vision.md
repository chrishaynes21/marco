---
type: milestone
status: historical
---

# The vision perception provider

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

    Vision produces observations.
    Fusion produces belief.
    The Director reasons over belief.
    Marco executes mechanics.

This milestone does not make the Director understand games, or screenshots, or anything
else. It teaches the Director how to accept observations produced from **images** instead
of from accessibility — under exactly the rules every other provider already obeys.

Vision may never execute, resolve a target, choose an action, verify an outcome, or reach
the world without passing through fusion. Those are not conventions. Each one is a
consequence of where the code sits, and several are enforced by tests that fail the build.

## Where it sits

```
internal/director/perception/capture     the frame, and its transform back to the desktop
internal/director/perception/providers/vision
    vision.go     what a detector IS, and the class vocabulary this build understands
    provider.go   capture → detect → filter → observations
    grid.go       repeated shapes read as a grid
    frame.go      the frame log (no pictures kept)
internal/platform/visionclient           the bridge to the detector plugin
plugins/vision                           the detector itself (OpenCV/ONNX; has dependencies)
cmd/director/vision{cmd,diag,wiring}.go  the CLI and the composition root
```

The provider is in `perception/providers`, beside accessibility, window-system, OCR and
visual-state. It has no other privileges. `perception_boundary_test.go` already forbade any
package outside `internal/director/perception`, `internal/recorded` and `cmd/director` from
seeing observations at all, so vision cannot address the Director even if it wanted to.

The detector is a **subprocess**, for the same reason OCR is: a model needs ONNX or OpenCV,
and the engine module permits no external dependencies. The engine launches it and speaks
JSON over stdio.

## Opt-in, always

Vision costs a screen capture and a model pass. It runs only when the request asks:

```go
obs, err := collector.Observe(ctx, observation.WithVision(region))
```

`Provider.Observe` returns `nil, nil` when `req.Includes(SourceVision)` is false. A cycle
that does not ask never captures anything, which is why an ordinary Director with the
provider wired behaves exactly as it did before it existed.

## The image goes down; the coordinates do not come back up

The Director captures the frame and sends it. The plugin answers in **image-local**
coordinates. The provider maps those back to the desktop through the `capture.Transform`
that came with the frame.

That direction is deliberate. A plugin that captured the screen itself would be a second
observation path with its own idea of what was on screen. A plugin that returned desktop
coordinates would be *placing* observations — which requires the window bounds, the DPI
scale and the monitor origin, none of which the plugin has any way to get right.

`plugins/vision`'s `Detect` therefore has two modes, and they differ precisely in this:
given an `Image` it answers image-local; given no image it captures a region itself and
answers in absolute screen coordinates. The second mode is the older one, kept for
`marco vision detect` and the anchor resolver, which have no frame to hand it.

## What gets refused, and why the diagnostics lead with it

A detector returns whatever it returns. The provider is the layer that decides what is
worth believing, and every rejection is counted:

| gate | default | what it protects against |
|---|---|---|
| unknown class | — | a model whose vocabulary this build has no word for |
| confidence | 0.35, **0.50 for structural classes** | low-conviction boxes becoming controls |
| geometry | ≥ 6px, ≤ 90% of the frame | slivers and whole-window boxes |
| stale capture | frame must match the window it was taken from | acting on a window that moved |
| ceiling | 500 detections | a runaway model flooding the world |

`director vision` prints the rejections as prominently as the acceptances. A provider that
reported "12 observations" while silently dropping forty is one whose output nobody can
calibrate, and calibrating it is the whole purpose of the command. Unknown classes are
marked `?`, because a model this build cannot speak to produces nothing and otherwise looks
identical to a model that found nothing.

Structural classes carry a **higher** confidence bar than text. Asserting "there is a
button here" is a stronger claim than "there is text here", and the thresholds say so.

## Vision never fabricates actionability

This is the safety rule, stated in the package doc and held by tests.

Seeing the word *Delete* does not produce a Delete **button**. Text remains text; controls
remain controls. Exactly like OCR.

Concretely, `Provider.element` leaves `Enabled`, `Visible` and `Focused` **nil**. Vision
has no way to know any of the three — a greyed-out button and an enabled one are a colour
apart, and vision does not read colour semantics. Reporting `Enabled: true` would be an
invention, so it reports nothing and lets the layers above carry the doubt:

- **Fusion** ranks vision at 2, below OCR's 3 and far below accessibility's 6. Anything
  structural that agrees with vision outranks it and wins the disputed field.
- **Policy** gates on `ObservationQuality`, which `ObservationSource.Structured()` feeds.
  A world seen only by vision is not safe to act in, and
  `internal/director/policy/vision_test.go` proves it against a control world with the
  identical role composition seen structurally.

`TestTextDoesNotBecomeAButton` was verified to actually catch a regression: weakening the
structural check in `provider.go` to `if false && !class.Structural()` makes it fail.

### One known consequence, recorded honestly

Fusion defaults an **unreported** `Enabled` to `true`. That is a deliberate pre-existing
decision — most providers do not report it, and defaulting to false would make every
element unusable — but it means the nil that vision so carefully preserves does not survive
into the belief as a nil. The doubt is carried instead by `Confidence` and by `Sources`,
which is what policy actually reads. Recorded here because a reader tracing the nil through
fusion will find it gone, and should know the safety does not rest on it.

The other consequence worth knowing: **OCR outranks vision**, so when both see the same
element, OCR wins every disputed field including the bounds. A vision box that is tighter
or better-centred than OCR's still loses. This is correct — text-shaped evidence is better
evidence about text — but it surprises people who assume the newer provider wins.

## Grids

Repeated same-sized shapes in regular rows and columns are read as a **grid**, and each
member gets `grid`, `grid_row`, `grid_column` and `grid_index` attributes. Inventories,
tile pickers, calendar months and icon views all have this shape, and a cell's *position*
is very often the only thing that distinguishes it.

Detection is geometric and unnamed: cluster by size, sort into rows and columns, and score
regularity. A grid needs at least 4 cells across at least 2 rows and 2 columns, with sizes
within 25% and alignment within 40% of a cell, at 0.6 regularity.

A grid cell is **one observation, not two**. An early version emitted the grid position as
a second same-source observation and got two elements per cell — fusion deliberately never
merges two observations from the same source (`TestSameSourceNeverMerges`), because a
provider that saw a thing twice saw two things. The grid pass therefore runs *before*
observations are built and merges its attributes into the cell's own observation.
`TestAGridCellIsOneObservationNotTwo` holds the line.

Capability packs read those attributes without knowing vision produced them:
`internal/gamepacks/palworld/observe.go` turns `grid_index` plus a stack count into an
`EntityIdentity` for an inventory slot.

## Reading the words inside a control

A detector answers "there is a control here" and stops. What makes a control *addressable*
is what it says — "click Settings" needs a control named Settings — so when a `LabelReader`
is wired, each accepted structural box is cropped, enlarged and read, and the words become
that observation's `Label`.

**The reading is scoped to the box, and that is the whole point.** Measured on a live
Rocket League pause menu, twelve strings a person reads at a glance:

| approach | result |
|---|---|
| whole frame, native size | 1 string |
| whole frame, 2× and 3× upscale | 2 strings |
| tiled 4×3 and 6×4, 2× | 151 and 127 strings, essentially all garbage |
| **scoped to detected boxes** | **4 of 4 button labels, exactly** |

Whole-frame reads fail because a global binarisation threshold is dominated by a lit 3D
scene and the translucent panel text lands on the wrong side of it. Tiling fixes the
threshold and introduces something worse: an OCR engine pointed at arena texture
**hallucinates** — it returned `"Sty 4;"`, `"Feasts)"`, `"itirne"` from regions containing
no text. Structure is what makes the reading trustworthy: vision says where a control is,
and only there is text worth believing.

A label is attached to the detection, never emitted beside it — the same rule as grid
positions, for the same reason. A name is a property *of* a control, and a separate
observation at identical geometry would become a second element that fusion refuses to
merge.

### What is refused, by which of two filters

They are not interchangeable, and a label needs both:

- **Shape.** Symbol soup contains characters no label has. One `»` or `{` is enough.
- **Confidence.** The same frame read a stylised name plate as `"Qovisivre ys"` — eleven
  letters and a space, indistinguishable by shape from a product name nobody anticipated.
  No shape rule will ever reject that; the engine admitting it was unsure is what does.

Edge-trimming was written and reverted. It looks obvious — the real engine returned
`"| RESUME GAME ,"` for a button whose border became characters — but stripping the ends
rescues `"»)  (ee i"` as `"ee i"`. It was also unnecessary: engines report *words*, so the
border marks arrive as their own low-confidence spans and are dropped before joining. The
combined string was an artefact of asking tesseract for one line of stdout, which is not
how the Director asks.

### Cost, and its ceiling

Each reading is a serial round trip. Measured live: **39 boxes cost 9.0 seconds**, about
230ms each. So a pass reads at most `MaxLabels` (24) controls, largest first, skipping boxes
too small to hold a word — and `director vision` reports what it skipped, because a silent
cap reads as "nothing to find here" when it means "we stopped looking".

The two providers never learn about each other. `vision.LabelReader` is its own three-line
interface and the adapter over an `ocr.Engine` lives at the composition root
(`cmd/director/labelreader.go`), because a build with a detector and no OCR — or the
reverse — is the ordinary case.

## The frame log

`director frames` lists the recent frames — id, window, size, how many detections were
accepted, how long it took. The last 20.

**The pictures themselves are never kept.** The header says so, because a user looking at a
list of frames of their desktop is entitled to know the Director is not holding
screenshots.

## The commands

```
director vision [--region x,y,w,h]   run one pass; report what was seen and what was refused
director vision --last               the previous pass, capturing nothing
director explain vision              the same, by its other name
director frames                      the recent frames, and what came of each
director observations                every source and what it contributed, vision among them
```

`director vision` **captures the screen**, which is why it is a command a person runs rather
than something any request triggers. The others read records.

With no detector installed — the ordinary case — it says so, names why, and points at
`$DIRECTOR_VISION`. "No model is installed" and "this window is empty" are different
findings, and reporting the first as an empty result would send a user hunting a model that
was never the problem.

## Any backend, later

`Detector` is three lines: `Detect(ctx, Input) ([]Detection, error)` and `Model() string`.
OpenCV today, YOLO tomorrow, something else after that. The class vocabulary
(`button/icon/text/field/checkbox/radio/slot/bar/panel/menu/image`) is the contract; a
backend that speaks other words has them counted and refused, visibly, in `director vision`.

## Not in scope

No model training or tuning, no game-specific detection, no memory reading, no injection,
no combat automation. Vision is a source of evidence and nothing more.

## Known gaps

What is **not** proven, stated plainly:

1. **No model has ever run.** Every test in this milestone uses a fake detector returning
   scripted detections. `cmd/director/vision_e2e_test.go` runs detect → place → filter →
   fuse → world with a fake capture and a fake detector;
   `internal/platform/visionclient/visionclient_test.go` runs encode → call → decode
   against a fake host, with the real PNG and the real JSON. What has never run is the
   join: a real capture, a real subprocess, a real model, a real screen.

   That seam was worth testing. Writing those tests found `decode` and `errText`
   marshalling a `runtime.Value` directly — whose fields are unexported, so the payload
   came out `{}`, unmarshalled cleanly into the detection struct, and produced **zero
   detections with no error**. Against a live plugin the provider would have reported an
   empty screen forever, and the diagnostics would have agreed with it. Fixed by going
   through `runtime.JSONFromValue`, as `ocrclient` already did.
2. **The thresholds are guesses.** 0.35 / 0.50 confidence, 6px, 90% area, 500 detections,
   and every grid constant were chosen from what the shapes ought to be, not from a
   distribution anyone measured. Expect to move them after the first real pass. This is
   exactly what `director vision`'s rejection counts exist to inform.
3. **The class vocabulary is a guess about the backend.** Eleven classes, mapped to Marco
   roles. A real model's label set will differ; the mismatch shows up as `?` rows in
   `director vision` rather than as silence, which is the most that could be done in
   advance.
4. **Nil actionability does not survive fusion**, as described above. The safety rests on
   source rank, confidence and the policy gate — all tested — not on the nil.
5. **Grid detection has never seen a real inventory.** It is tested against synthetic
   detections laid out on an exact lattice with injected jitter. A real inventory with a
   scrollbar, a partially-visible bottom row, or a selected cell drawn larger than its
   neighbours may not cluster the way the fixtures do.
6. **Stale-capture detection compares window bounds only.** A window that moved and moved
   back within one pass, or one whose contents changed while its frame stayed put, is not
   caught.
7. **Cost is now partly measured, and it is not small.** A detection pass on a 1920×1080
   frame runs 170–730ms. Label reading adds a serial OCR round trip per control: 39 boxes
   cost 9.0 seconds before the ceiling existed, 3.7s with it. Vision remains opt-in for
   exactly this reason, and whether it is ever affordable inside an ordinary observe cycle
   is still unknown.

## What the live bring-up established

Run against Rocket League on 2026-08-05, with `icon_detect.onnx` (Ultralytics YOLO11m via
OmniParser, **one class**, trained on `som_office`). The fixtures are in
`fixtures/rocketleague/`.

Working, verified against the real game: window capture of a fullscreen DX11 title
(including monitors at negative coordinates), the plugin bridge, decoding, placement, the
stale-capture guard (it correctly refused a frame taken while the window moved), grid
inference, and label reading.

Two findings that bound what this model can do:

- **In play, it cannot help.** On a Free Play frame the detector finds *nothing* at its 0.25
  floor. At 0.05 it surfaces exactly one box — the boost gauge, correctly located — at
  **0.13**, which is noise. OCR reads nothing either: Rocket League draws "33" as
  disconnected horizontal bars in a thin translucent font, unreadable at three PSMs even
  cropped and 3× upscaled. No threshold rescues this; it needs a different model.
- **In menus, it works.** The pause menu's four buttons are found at 0.59–0.64 with default
  settings, and reading inside them returns all four labels exactly.

The honest summary is that the framework is sound and this particular model covers menus
and overlays but not in-play HUD.
