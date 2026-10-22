---
type: experiment
status: conclusive
date: 2026-10-21
backend:
  - structure
fixture: "A rendered fixture, not a live application: `internal/director/localizetrial`. One 640x480 client area holding two panels — GENERAL and ADVANCED — each with the same three buttons (SAVE, APPLY, RESET) drawn by one function from one set of parameters, so the two APPLY buttons are the same pixels. 120 evaluation cases from fixed seeds: 60 supported positives (2 intended instances x 3 backgrounds x 10), 50 unsupported negatives (10 each: intended absent with the decoy present, intended covered, intended partly covered, reflow beyond the neighbourhood, unsupported client size or scale) and 10 window-movement cases. Parameters frozen against 60 setup examples on disjoint seeds before anything was scored. No window was opened, no input was sent, nobody's store was read."
result: "Local regioning: 70 of 70 supported cases selected the demonstrated instance, 50 of 50 unsupported cases refused, ZERO wrong-instance selections. Whole-window matching of the same template with the same settings: 0 of 70 supported cases selected correctly, and 30 WRONG-INSTANCE SELECTIONS — every case where the demonstrated control was gone, covered or half-covered, where it chose the identical control in the other panel with no doubt at all. Its other 80 cases refused as ambiguous, because two controls that look alike give appearance two answers and no way to prefer either. The ambiguity rule never fired for the local method: a 6px neighbourhood cannot contain two instances of a 160x36 control, so that branch is unreachable at these parameters and is NOT evidenced by this run."
supersedes: []
affects:
  - demonstrations
  - execution
source_paths:
  - internal/director/localizetrial/fixture.go
  - internal/director/localizetrial/localize.go
  - internal/director/localizetrial/protocol.go
  - internal/director/localizetrial/evaluate.go
  - cmd/localizetrial/main.go
---

# Experiment-096 — the click says which one; the template only says what

## The question

> Does local regioning distinguish the demonstrated control instance despite unrelated screen
> noise?

With the hypothesis stated as the thing to be falsified:

> The demonstrated click identifies which control instance the person intended. A template
> identifies appearance; it must not substitute a similar-looking control elsewhere.

This is an experiment. It is not authorization for a recognition system, and nothing in Studio,
button detection, fusion or production execution policy was changed to run it.

## Why a rendered fixture and not a game

Because the question is about the method, and a game answers a different question every time it
is asked. [[Experiment-095-thirty-seconds-of-settling-that-had-already-settled]] and the two live
trials after it are what prompted this, but they cannot measure it: a translucent Ark menu over
live 3D is filed as two different screens depending on what is moving behind it, and no
comparison between two methods can be held still inside that.

So every case here is an image rendered from a seed, and a selection is a point computed from
one. Another person runs `go run ./cmd/localizetrial` and gets this table. Nothing opened a
window, took the foreground, sent input or read anybody's store — which also means the owner's
game was never interrupted to produce any of it.

The cost is stated in the last section: a fixture result says nothing about a translucent game
menu, and this one does not claim to.

## The fixture

A 640x480 client area. Two panels side by side, **GENERAL** and **ADVANCED**, each holding
**SAVE**, **APPLY** and **RESET**. The buttons are drawn by one function from one set of
parameters, so `left.APPLY` and `right.APPLY` are the same pixels — and that is asserted rather
than assumed: `TestTheTwoInstancesAreTheSamePixels` scores one instance's template against the
other's position and fails below 0.99. It measures 0.9959.

The only thing that tells the panels apart is the heading, and the heading is deliberately
outside the local region. `TestTheLocalRegionStopsShortOfTheHeading` holds that as geometry. A
region grown until it swallowed a landmark would hand appearance the answer and the experiment
would be measuring its own fixture.

Three backgrounds, all seeded: a still banner, a banner whose blocks differ per case, and
animated discs. All of them outside the panels, which is what "unrelated" means here.

Each instance is distinguishable by result — `Look.Hit` says which control a point lands on —
and that is used only to score a selection, never by either method.

## The two methods

One template, one correlation, one threshold, one ambiguity rule. **They differ in where they
look and in nothing else.**

- **whole-window** — match the template anywhere in the client area.
- **local region** — check the client geometry; predict the point from the stored
  client-relative offset; match the template only within the tolerance of the prediction. No
  fallback search anywhere else, which `TestTheLocalMethodNeverAnswersOutsideItsNeighbourhood`
  holds over every case rather than inferring from the table.

What a demonstration stores: the client area it was demonstrated in, the display scale, the
click relative to that client area, and a small picture around the click.

## The frozen parameters, and where each came from

Chosen against 60 setup examples on seeds 1000-1059, disjoint from every evaluation seed
(`TestTheSeedRangesDoNotOverlap`), and frozen before anything was scored. Each is re-derived
from those examples by a test, so a value that stops matching its justification fails rather
than drifting.

| | value | why |
| --- | --- | --- |
| local region | 176x52 | the 160x36 button plus 8px of its own surroundings; stops 22px short of the panel heading |
| tolerance | 6px | three times the largest movement seen in setup (2px) |
| accept | 0.90 | setup: the correct match scores ≥ 0.9958, the best thing at a wrong place ≤ 0.7830 |
| margin | 0.03 | setup: the decoy scores 0.9959 against the target's 0.9958 — nothing separates them by score |
| apart | 12px | two peaks closer than this are one broad peak, not two things |

That fourth row is the premise, measured: **appearance cannot separate the two instances**, so
anything that does must be using something other than appearance.

## What the evaluation measured

```
METHOD         CASE                                           n   right   WRONG   nowhere   refused
local region   quiet background                              20      20       0         0         0
local region   changing banner outside the region            20      20       0         0         0
local region   animated content outside the region           20      20       0         0         0
local region   window moved on screen                        10      10       0         0         0
local region   intended control absent, decoy present        10       0       0         0        10
local region   intended control covered                      10       0       0         0        10
local region   partly covered, locally ambiguous             10       0       0         0        10
local region   reflow beyond the neighbourhood               10       0       0         0        10
local region   unsupported client size or scale              10       0       0         0        10
whole-window   quiet background                              20       0       0         0        20
whole-window   changing banner outside the region            20       0       0         0        20
whole-window   animated content outside the region           20       0       0         0        20
whole-window   window moved on screen                        10       0       0         0        10
whole-window   intended control absent, decoy present        10       0      10         0         0
whole-window   intended control covered                      10       0      10         0         0
whole-window   partly covered, locally ambiguous             10       0      10         0         0
whole-window   reflow beyond the neighbourhood               10       0       0         0        10
whole-window   unsupported client size or scale              10       0       0         0        10

whole-window   0/70 supported selected correctly, 20/50 unsupported refused, 30 wrong-instance
local region   70/70 supported selected correctly, 50/50 unsupported refused,  0 wrong-instance
```

**Wrong-instance selections: 30, all whole-window.** Every one of them is a case where the
demonstrated control was absent, covered or half-covered — so the decoy was the only strong peak
left, and appearance-only matching chose it with full confidence and no ambiguity. That is the
hypothesis's negation failing exactly where it is most dangerous: not a near miss, but a
confident click on a control nobody demonstrated, in a panel nobody asked about.

**False refusals: 70, all whole-window**, every one of them "more than one thing looks like the
control that was demonstrated, and nothing here says which was meant". With two identical
controls present, appearance has two answers and the ambiguity rule is the only honest thing it
can do.

**Selected where it should have refused: 0, both methods.**

### Why each refusal happened

```
  40  local region · below the acceptance threshold
  10  local region · the client area is not the demonstrated one
  80  whole-window · ambiguous: two things that look alike
  10  whole-window · below the acceptance threshold
```

### Window movement, tested separately

The same client pixels at four different screen origins produce the same client-relative answer
and a screen point that moves with the window (`TestAMovedWindowChangesNothing`). Nothing stored
is a screen coordinate. The whole-window baseline refuses all ten movement cases, for the same
reason it refuses every other supported case — the move is not what defeats it.

### Resize and scale

**No resize transform is supported, and that was declared before evaluation.** A client area of
another size is refused as a different layout, which is 10 of the 50 unsupported cases. The
display scale is checked separately — a rendered scale change is refused on size before the
scale is ever consulted, so `TestADifferentDisplayScaleIsRefused` holds the pixels identical and
changes only the claim.

## The branch this run did not reach

**The ambiguity rule never fired for the local method.** All 50 of its refusals were the
threshold or the geometry check. The reason is geometric: a ±6px neighbourhood cannot contain
two instances of a 160x36 control, so at these parameters local ambiguity between two instances
cannot arise and that branch is untested by this evaluation.

It is reported rather than engineered around. Widening the tolerance until the rule became
reachable would be enlarging a frozen parameter to make a case interesting, which is the one
thing the protocol forbids. The ten "partly covered" cases exercise local ambiguity in the other
sense — what is left under the prediction is no longer unambiguously the control — and are
refused on the threshold.

## Localization is not verification

Measured separately throughout, and stated as a test: `TestLocalizationIsNotVerification` shows
a correct selection, shows that the two instances differ in what a click on them does, and shows
that the same point in a look where the control has been removed lands on nothing. Finding the
intended button is not evidence that pressing it worked, and nothing in this experiment claims
otherwise. The fixture's post-action result is a separate mechanism and is not part of any
number above.

## Representative evidence

Regenerated byte-identically from the seeds rather than committed — each image is about 190KB
because of the per-case pixel noise, and a command that reproduces them is better evidence than
a copy of them:

```sh
go run ./cmd/localizetrial -evidence <dir>
```

- `00-demonstrated.png` — the click and the 176x52 region kept around it, clearly short of the
  GENERAL heading.
- `04-intended-control-absent-decoy-present.png` — the one worth looking at. The GENERAL panel's
  APPLY is gone; a red cross sits on the ADVANCED panel's APPLY, which is where whole-window
  matching decided to click. The local region drew no mark, because it refused.

## The recommendation

**Continue local-region work for the specifically tested layout**, and do not read this as
support for recognition generally.

The reason is the 30, not the 70. The supported cases say local regioning works here; the
unsupported ones say the alternative does not merely fail, it presses the wrong control. A
fixed-layout MVP built on a stored point and a template, without the neighbourhood check, has
that failure available to it on any screen with two similar controls — and a game menu is mostly
such screens.

The MVP can move forward on durable coordinate replay **with the geometry and neighbourhood
check in front of every press**, which is what this measured: the click is authoritative about
which instance was taught, the template about whether that instance is still there, and the
geometry about whether the question can be asked at all. Any one of the three alone is a wrong
click waiting for a layout that moved.

## What remains untested

- **A translucent menu over live 3D.** The fixture's panels are opaque. The owner's two live
  trials failed on exactly this, and nothing here addresses it.
- **Local ambiguity between two instances**, for the geometric reason above.
- **Any real application.** No live window, no capture path, no scaling from a real desktop, no
  DPI change observed rather than declared.
- **The post-action result**, deliberately: this measured localization only.
- **Resize**, by declaration rather than by failure.
- **Anything about a layout that is not this one.** Fixed layout means fixed; nothing here says
  what happens to a control in a list that scrolls.

## Related

[[Experiment-095-thirty-seconds-of-settling-that-had-already-settled]] is the live trial that
prompted it. [[ADR-246-a-trial-collects-its-own-evidence]] is how the two live trials after it
were read without asking the owner to describe them.
