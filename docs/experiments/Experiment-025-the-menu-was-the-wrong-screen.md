---
type: experiment
status: complete
date: 2026-10-10
backend:
  - screenparser
game: trackmania
fixture: "%LOCALAPPDATA%\\marco-liveiface\\runs\\0002"
result: the-pause-menu-contains-no-list-and-the-map-screen-does
supersedes: []
source_paths:
  - watch-interface.cmd
  - tools/run-report/read_run.py
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
---

# Experiment 025 — the menu was the wrong screen

## The question

The same acceptance test as [[Experiment-024-watching-a-player-navigate-a-real-menu]], run
again after four repairs made specifically so this run could be read:

> Can Marco watch you navigate a game and understand what changed?

Experiment-024 spent its only legible window holding still, which is structurally incapable of
identifying a list — a mark that does not move cannot be seen transferring. So the protocol was
reordered to navigate first, and this run was to be the answer.

## Method

`watch-interface.cmd trackmania`, 240s, six phases: **MOVE SLOWLY** (one press about every 5s),
MOVE FAST, REVERSE, WRAP, HOLD STILL, CLOSE THE MENU. Xbox controller. Nothing injected.

Recorded to `runs\0002`, which no longer destroys the previous run's evidence. Read with
`py -I tools/run-report/read_run.py`, which answers the seven questions the run was set
mechanically rather than by eye.

`$MARCO_SHADOW_UNGATED=1`, `$MARCO_SHADOW_CADENCE=1s`, isolated `$MARCO_HOME`.

**The operator deviated in phase 6**, and it produced the run's most important evidence. Asked
to close the pause menu and leave it closed, they exited the map instead, rejoined, opened the
pause menu and pressed back. So phase 6 does not test belief expiry — and does contain the
map-selection screen, which nothing in the protocol had thought to point the detector at.

## The result: the acceptance test failed again, and for a different reason

Zero selections were ever read. Not borderline:

```
interface   status: unknown   selected: -1   reading: insufficient_evidence
because:    "no list has been seen handing a mark between its members
             (8 candidates watched over 106 frames)"
tracking    offered 938   no_list_identified 938
```

The reader classifies all 116 ticks as **NEVER SAW**, and none as saw-and-declined. The
operator navigated correctly — 92 directional presses, distributed as the protocol asked.

## The finding: the pause menu contains no list

Three independent lines of evidence, and this is what distinguishes 025 from 024. Experiment-024
failed because its navigation phases were blind. These were not blind; they were pointed at a
screen that does not hold the evidence the rule needs.

| phase | presses | looks | boxes/look |
| --- | --- | --- | --- |
| 1 MOVE SLOWLY | 11 directional + pause | 27 | ~5 |
| 2 MOVE FAST | 60 | 13 | 4.2 |
| 3 REVERSE | 4 | 16 | 4.8 |
| 4 WRAP | 15 | 12 | 4.6 |
| 5 HOLD STILL | 1 stray | 16 | 7.9 |
| 6 EXIT/MAPS | back, confirm ×2 | 23 | **20.8** |

1. ~5 boxes a look during navigation. A five-entry menu should put five aligned rectangles on
   screen; what arrived was four or five boxes at unrelated positions and sizes.
2. Zero vertical stacks of three or more similar rectangles in 67 navigation looks.
3. Director formed **no** `possible_choice_group` and **no** `possible_menu_like_state` in
   phases 1–5. Both appeared only in phase 6.

No selection mark was detectable in the shading anywhere in the run, including on the one
genuine list — see below.

### The map-selection screen is the screen this test wants

Phase 6 holds the only uniform list in the run: 8 rows, y 0.457→0.690, gaps uniformly
0.032–0.034, identical x, width and height. And Director recognised it:

| t | what appeared |
| --- | --- |
| 187s | `group_1` + `possible_menu_like_state` |
| 227s | `state_4` |
| 233s | `group_2` — *"5 controls, evenly spaced, uniformity 0.00"* |
| **242s** | **the run ended** |

The destination screen started recurring about fifteen seconds before the clock ran out, so the
exit→map-selection transition never became a `possible_transition_action`. The protocol had
allocated those 72 seconds to looking at nothing.

## What did work

**Input-to-screen-state learning produced real evidence**, which was the other unproven
capability:

```
possible_reversible_place  state_3  SUPPORTED  4 episodes
  "up" preceded arriving in 3 of 3 observations
  "down" preceded leaving in 2 of 4 observations
possible_transition_action state_2  contested  3 episodes
  "up" preceded this change in 3 of 3 observations
relationships.session_local = 5        (all causes unresolved)
```

One question was asked at inference 46. All session-local; nothing persisted; no execution
authority. This is association between inputs and **screen states**, not between inputs and
**menu entries** — a weaker claim than the acceptance test, and worth keeping separate from it.

**[[ADR-154-geometry-is-evidence-even-when-the-words-are-not]] confirmed live**, by the
admission account added for this run — its first real reading:

```
admission  1007 proposed   197 accepted   810 unreadable
shapes     805 preserved   805 measurable
```

That is the addendum's "Unreadable high, geometry kept — the repair is working" row, on live
data. 805 of 810 unreadable text boxes kept their rectangles.

**Cadence improved** under [[ADR-152-slow-discovery-fast-tracking]]: 2.26s per inference against
Experiment-024's 2.60s, median 976ms against 1042ms.

## Three corrections to the record

1. **ADR-154's cited magnitude came from the wrong screen.** "About seventy boxes a frame" — the
   pause menu proposes about five. The 74-box reading in `capture-cost.txt` is a dense menu
   screen captured after the session ended. The decision stands (80% of proposals refused as
   unreadable) but the number attached to it does not describe the screen it was reasoned about.
2. **The occlusion hazard fired once.** Slot 1, at 8.7s, captured the Windows taskbar rather
   than Trackmania: a dense row of icons and buttons at y≈0.967 across x 0.40–0.95, a
   `text_field`, eleven equal 0.005-wide boxes at y≈0.927. One look of 107, before the operator
   was in the game, and it inflated phase 1's region count. `target_losses` was 0 throughout:
   window identity was intact and the pixels were somebody else's.
3. **A mark that never moves is invisible to this tracker, by construction.** A member's
   baseline is built only from frames in which it was *not* marked, so a permanently lit row
   never establishes one and is skipped from the judgement. Holding still cannot show a mark,
   which is a second and independent reason phase 1 of Experiment-024's protocol could not
   have worked.

## What this run could not say, and now can

The whole account of the failure was one sentence: *"no list has been seen handing a mark
between its members (8 candidates watched over 106 frames)"*. Five unrelated defects produce
that sentence:

| what is actually wrong | where to look |
| --- | --- |
| nothing could be measured | capture, or the detector |
| candidate identity churned | grouping and candidate matching |
| no mark was ever visible | the shading, or the screen |
| several members marked at once | nothing — a heads-up display, correctly refused |
| the mark never moved | the protocol, or member matching |

Telling them apart after this run meant regrouping the run's own geometry with a script that
was not the production rule. The tracker had computed all of it on `selection.Evidence` and
never let it out. It is now reported per candidate —
[[ADR-156-a-refusal-is-a-measurement]].

## Next

The map-selection screen, not the pause menu. The evidence the rule needs is there and is not
in the pause menu, and `Experiment-026` is the same protocol pointed at it, starting with the
list already visible.

Still unproven, and unchanged by this run: that Marco can tell *"I found a list of choices"*
from *"I know which choice is currently active"*.
