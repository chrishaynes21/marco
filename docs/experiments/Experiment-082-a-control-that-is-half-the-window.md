---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "the owner's own hands, on a live Ark: Survival Ascended menu, through `director demonstrate-visual` with live OCR. Four attempts at the twice-visited validation, of which the fourth produced two steps and two controls. No input injected and none simulated. The artifact is at $APPDATA/marco/sequences/ark_twice.json."
result: "a-click-produced-a-control-932x690-pixels-across-carrying-thirteen-words-of-the-menus-own-vocabulary-and-a-second-control-whose-rectangle-is-byte-for-byte-the-container-the-first-was-distinguished-from"
supersedes: []
affects:
  - perception
source_paths:
  - docs/experiments/Experiment-081-the-first-real-negative.md
  - internal/director/observe/clicktarget.go
  - internal/director/observe/controllabel.go
---

# Experiment 082 — a control that is half the window

The twice-visited validation finally captured clicks. What it captured is worse than nothing
learned, and it is the clearest statement yet of the thing standing between Marco and a guarded
pixel press.

## What the demonstration reported

```
  2 screen(s) seen, 3 visited in order, 2 demonstrated control(s)
  presses: 5 of 13 classified, 1 no_session, 2 unsupported_navigation, 5 key_release

  Steps
    1 visual_1 -> visual_2 via target_1   [geometric]
    2 visual_2 -> visual_1 via target_2   [geometric]

  Controls, and what each press was taken to be
    target_1  on visual_1  1 press(es), read as "ark clug discord exit friends main
                           menu mode on photo settings to toggle", led somewhere
    target_2  on visual_2  1 press(es), nothing was read on it, led somewhere
```

Two clicks, two screens, two steps, both attributed. By every count in that report the session
worked.

## What the artifact holds

```json
"controls": [
  { "label": "control_1",
    "readings":   [{ "x": 0.171875, "y": 0.361111, "width": 0.485417, "height": 0.638889 }],
    "containers": [{ "x": 0.420833, "y": 0,        "width": 0.579167, "height": 1        }],
    "text": ["ark","clug","discord","exit","friends","main","menu","mode","on","photo",
             "settings","to","toggle"] },
  { "label": "control_2",
    "readings":   [{ "x": 0.420833, "y": 0,        "width": 0.579167, "height": 1        }] }
]
```

On a 1920 × 1080 window:

| | rectangle | size | words |
|---|---|---|---|
| Experiment 080's control | (804, 432) | **312 × 52** | `["settings"]` |
| `control_1` here | (330, 390) | **932 × 690** | **thirteen** |
| `control_2` here | (808, 0) | **1112 × 1080** | none |

**`control_1` is 932 × 690 pixels — a third of the screen — and its thirteen words are the menu's
whole vocabulary.** *exit*, *main menu*, *settings*, *photo mode*, *discord*, *friends*,
*toggle*: that is not a button's label, it is a list of every button beside the one that was
pressed. (The word for the button actually clicked is not among them.)

### And `control_2` is a container, byte for byte

`control_1.containers[0]` and `control_2.readings[0]` are the same rectangle to the last decimal
digit: `{0.420833, 0, 0.579167, 1}`. 1112 × 1080 — the full height of the window.

So the same rectangle is a CONTAINER on one screen of this artifact and a CONTROL on the other.
Nothing promoted it. The correlator did exactly what it is supposed to do on both screens:

- on `visual_1` the proposer offered both rectangles, so the smaller became the control and the
  larger was recorded as the container it was distinguished from — the distinction working;
- on `visual_2` the proposer offered only the larger, so it was the smallest thing containing the
  press and became the control by default — the distinction with nothing to work on.

The owner's instruction was not violated by a rule:

> Do not automatically promote a containing panel to a clickable control. Preserve the
> distinction between a demonstrated click coordinate and durable target identity.

It was lost by DEFAULT. `Containers` is screen-local, so a rectangle that is known to be a
container on one screen carries nothing about that onto another — and a session that already has
the evidence that this rectangle is a container files it as an identity anyway.

## Why this is the finding and not the 13 words

The words are a symptom with a cheap diagnosis: `controlWords` reads the rectangle it is given,
and given a third of the screen it reads a third of the screen. There is nothing wrong with it.

The finding is that **the demonstrated click position and the durable target identity were
separated by 932 × 690 pixels, and every number in the report said the session succeeded.** The
step is `[geometric]`, the control "led somewhere", the walk is right, and the saved identity is
a slab. A reader cannot tell this run from Experiment 080's from the report, because the report
never printed a size.

And the corroboration of [[ADR-233-a-match-that-can-be-corroborated-by-its-words]] does not save
it. A 932 × 690 rectangle that reads thirteen menu words will be `corroborated` by any menu
carrying any one of them — `agreedWords` needs one. The negative test of
[[Experiment-081-the-first-real-negative]] worked because a 312 × 52 rectangle reads ONE word and
a different menu puts a different word there. At panel scale the text evidence inverts from a
discriminator into a rubber stamp.

## What this licenses, and what it does not

Not pixel execution, and now for a reason that has nothing to do with matching reliability:
**there is no evidence that the thing being matched is a button.**

[[Experiment-080-it-learned-the-word-settings]] already said granularity VARIED rather than
improved and that the panel case was still live. It is no longer live; it is measured, twice over
in one artifact, with a container as the proof.

What it licenses is a report that can tell a button from a panel, which is this session's change
and is reporting only:

- the control's SIZE on its line, as a share of the window, because 16%×5% and 49%×64% are the
  whole difference between the two experiments and neither was printed;
- a flag when a control is most of the window, or when its rectangle is a container recorded
  elsewhere in the same session — both facts already in the artifact, neither read by anything.

The OCR-word-box targeting hypothesis the owner deferred is still deferred, and this experiment
is the first evidence that bears on it: at the demonstrated click position the proposer offers a
panel, and OCR offers thirteen word boxes INSIDE that panel. Whether one of them is the button —
rather than the padding around it — is the measurement that would decide it, and it has not been
made.
