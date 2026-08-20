---
type: experiment
status: complete
result: "cause B — provider geometry: Explorer's dominant group is 24 virtualised navigation-tree items reporting desktop rectangles outside the window; the frame was reliable and the refusal is correct"
date: 2026-08-12
affects:
  - passive-observation
  - perception
source_paths:
  - internal/director/observe/referent.go
  - internal/director/observe/placereferent.go
  - cmd/director/observesnapshot.go
---

# Experiment 012 — why Explorer's START could not be pointed at

A live `director teach` against File Explorer established its START and then refused to show it:

```
start grounding: state=state_1 unavailable=coordinate_mapping_unreliable
```

That string is returned from two places in `ReferentForSubject` that mean opposite things, and no
surface could say which had happened. This is the measurement that settled it.

## The two candidates

| | cause | whose defect |
|---|---|---|
| **A** | `!live.Reliable` — no trustworthy sampled frame to convert against | ours: lifecycle or wiring |
| **B** | `regionsOf` found the group and every member was outside the window | the provider's geometry |

A would be fixable. B must stay an honest refusal.

## Method

`observe.ReferentDiagnosis` was added to `VisualReferent` — bounded counts, booleans and reasons
from the closed vocabulary, written nowhere, carrying no coordinate and no text. It records the
member funnel in the order `regionsOf` applies it. Watch mode prints it; Normal mode does not.

Then the same product path was run again: cold `$MARCO_HOME`, real Explorer, vision and OCR off.

## Result

```
watching=yes frame=yes/yes settled=yes stands_for=yes subject=yes
members=24 present=24 sized=24 placeable=0 whole_window=0 outside_window=24 enclosing=0 regions=0
```

**Cause B, unambiguously.** `frame=yes/yes` — a rectangle was recorded and it was usable, so A is
excluded. The screen was settled, a group stood for it, the subject resolved, and all 24 of its
members normalised outside 0..1.

## Why the members are outside the window

Not a coordinate-space error, and this is the part worth keeping. Sampling elements directly
(`director inspect -window hwnd:200774`) against a window at `x=164 y=336 1386x641`:

| element | role | desktop bounds | inside? |
|---|---|---|---|
| `Home` | tree_item | `225,-1768 37x32` | no |
| `Downloads` | tree_item | `225,-1416 63x32` | no |
| `OneDriveTemp` | tree_item | `241,-744 83x32` | no |
| `Videos` | tree_item | `257,1496 39x32` | no |
| `Start` | tab | `176,344 248x32` | **yes** |
| `world` | button | `943,388 76x24` | **yes** |
| title bar | unknown | `172,344 1370x23` | **yes** |

The visible chrome places correctly. Only the navigation pane's `tree_item`s do not, and their Y
values run from −1768 to +1496 — a **virtualised tree reporting scrolled-out items at their
virtual positions**. There is no constant offset, no DPI factor and no monitor origin that
explains a spread like that, and any of those would have moved the tab and the buttons too.

So the geometry is correct and the elements really are not on the screen.

## Why THOSE members formed the group

The navigation tree is 115 evenly-spaced items that never move, which makes it the most stable,
longest vertical run on the screen — exactly what `Groups` selects and what `standsFor` then picks
as the group that characterises the state. The screen's most recognisable structure is the one
part of it nobody can see.

## The control

The same measurement on two other applications, same Director, same build:

| application | members | placeable | whole_window | outside_window | regions |
|---|---|---|---|---|---|
| explorer | 24 | 0 | 0 | **24** | **0** |
| chrome | 24 | 18 | 6 | 0 | **15** |
| code | 24 | 9 | 15 | 0 | **8** |

Chrome and VS Code ground. VS Code's 8 is the menu-bar case
[[ADR-046-grounding-a-screen-points-at-its-structure]] was validated against. Explorer is the
outlier, and it is the only one whose dominant group is offscreen.

## Conclusion, and what was deliberately NOT done

`EXPLORER_GROUNDING: provider geometry, not wiring.` The refusal is correct and stays.

Not done, on purpose: no clamping, no shifting, no falling back to the durable `Envelope`, and no
filtering of offscreen elements out of the screen composition. That last one is tempting and is the
trap — composition decides screen IDENTITY, so dropping members would change which places Marco
recognises and invalidate every subject already stored under the current roles. That is a Sight
change, not a pointing fix.

## What it costs, stated plainly

Explorer cannot be the application for a visual teach walkthrough. **Teaching is unaffected** — the
START was established, the place is durable, the session reached `ready_for_demo`, and only the
highlight is missing. See [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] and
[[Passive-Observation]].

## Open

Whether a screen whose dominant group is entirely offscreen should fall back to its next-largest
group is a real question and a Sight one. Recorded, not opened.
