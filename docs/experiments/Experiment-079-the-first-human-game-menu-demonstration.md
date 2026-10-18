---
type: experiment
status: complete
date: 2026-10-17
backend:
  - structure
game: arkascended
fixture: "the owner's own hands, on a live Ark: Survival Ascended menu, through `director demonstrate-visual` at 10 looks a second. No input was injected and none was simulated. The artifact is at $APPDATA/marco/sequences/ark_menu_test.json and every number below is read from it or from the command's own report."
result: "all-three-presses-captured-one-transition-of-three-observed-no-control-read-at-all-and-the-live-loop-turns-out-never-to-set-a-looks-text"
supersedes: []
affects:
  - perception
source_paths:
  - cmd/director/visualsession.go
  - cmd/director/demonstratevisual.go
---

# Experiment 079 — the first human game-menu demonstration

The first time a person showed Marco a game menu with their own hands. Three clicks in Ark's
menu, ENTER to finish, and one saved artifact.

## What the session reported

```
2 screen(s) seen, 2 visited in order, 3 demonstrated control(s)
presses: 4 of 8 classified, 1 key_release, 1 no_session,
         1 unplaceable_pointer, 1 unsupported_navigation

Steps
  1 visual_1 -> visual_2 via target_1   [geometric]

Controls, and what each press was taken to be
  target_1   1 press(es), nothing was read on it, led somewhere
  target_2   1 press(es), nothing was read on it, NOTHING CHANGED after it
  target_3   1 press(es), nothing was read on it, NOTHING CHANGED after it

"ark menu test" is saved: 1 step(s), 2 screen(s), 1 control(s).
```

## 1. Was every intended press captured? — Yes, all three

Three clicks, three identified controls. The four ignored events are all accounted for and none
of them is a lost game click:

| ignored | what it was |
|---|---|
| `unsupported_navigation` | the ENTER keydown that ended the session |
| `key_release` | its keyup |
| `unplaceable_pointer` | a press Marco could not place inside the window — the click back onto the terminal |
| `no_session` | navigation seen while nothing was watching, during the five-second delay |

One of the four CLASSIFIED events did not become a control. Three did, which matches three
clicks, so the fourth is a navigation intent rather than a lost press — but the accounting does
not say which, and that is a gap in the report rather than in the capture.

**The provenance rule did its job and cost nothing.** No injected press appeared, because none
was made.

## 2. Which controls got useful identity? — None

**All three: `nothing was read on it`.** And the artifact confirms it, over the whole file: there
is no `text` key on any screen or any control.

So the only identity `control_1` has is one rectangle:

```json
"readings": [{ "x": 0.3354, "y": 0.2981, "width": 0.3385, "height": 0.1981 }]
```

On a 1920×1080 window that is **650 × 214 pixels at (644, 322)** — a third of the window's width
and a fifth of its height. That is not a button. It is a panel, or a block of the menu, and
there is no `containers` key beside it, which means it was the only region the proposer found
containing the press.

The screen itself was segmented into **253 regions**. So the proposer is not under-segmenting
globally; it produced no region matching the button that was clicked, and the smallest thing
containing the click was that 650×214 block.

A 650×214 rectangle a third of the way across the screen will match almost anything. It is the
same shape of evidence that let a Rocket League artifact "find" 12 of its 13 controls in an Ark
window with 2100 colliding matches, in the verification run that preceded this one.

## 3. Were the expected transitions observed? — One of three

```
target_1   led somewhere          visual_1 -> visual_2
target_2   NOTHING CHANGED after it
target_3   NOTHING CHANGED after it
```

Two screens were seen and two visited, so the session settled exactly twice over three clicks.
The two later presses were seen, placed and identified, and no screen change was attributed to
either.

Three readings of the artifact, in order of what the evidence supports:

- **The menu may genuinely not have changed.** A click that opens a submenu in place, or lands on
  a disabled entry, changes pixels without changing what the arrangement counts.
- **The arrangements may have been too close to separate.** screen_a carries 253 regions and
  screen_b 78 — a large difference, which is why those two WERE separated. Two pages that differ
  by a few rows would not be, and
  [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured that the words
  separate screens the arrangement merges.
- **The settle rule needs three consecutive looks**, 300ms at this cadence. A click followed
  quickly by another gives the second screen no chance to settle.

The second of these cannot be distinguished from the first **without a reading**, which is the
same missing evidence as question 2.

## 4. Can the controls be reacquired? — not yet measured

`director reacquire "ark menu test" --application arkascended` has not been run. What the
artifact says in advance: one control, one reading, no text, no containers, and the session's own
note on the step —

```
clicked once, so nothing independent confirms that this control can be found again
nothing was read on this control, so a repair has only the workflow's own name to go on
```

Both sentences were written by the recorder about its own evidence, before anybody asked.

## 5. What evidence would have prevented an incorrect binding? — the reading, and the live loop cannot take one

**`cmd/director/visualsession.go` never sets `lk.Text`.** `fastobserve.Look` has carried the
field since the screen model existed, and the only code that fills it is in
`cmd/perceptionreplay` — `binding.go`, `reload.go`, `semantic.go`, `workflow.go`, all offline.

So every live command runs blind to text by construction: `director observe-screen`,
`director demonstrate-visual` and `director reacquire`. The consequence is not subtle, and it is
the whole of this experiment:

- `Control.Text` is described in its own doc as *"the strongest evidence of identity the artifact
  can carry"* and as *"empty in every artifact recorded so far, because no demonstration writes
  it yet"*. [[ADR-230-a-demonstration-from-pixels-and-what-the-screen-was-doing]] said a desktop
  demonstration now writes it and a frame recording still does not. **A live PIXEL demonstration
  cannot write it either**, and that was not known until a person tried.
- `Screen.Text` is the term Experiment 048 measured as separating screens the arrangement merges
   — which is candidate explanation two for the missing transitions.
- And it is the one term at control scale that
  [[Experiment-078-the-conjunction-is-two-terms-wearing-three-hats]] named as untested, because
  the corpus had no reading at control scale to test it with.

Everything needed is built and used offline: `ocrclient.Engine`, `observe.TextOf`,
`observe.ScreenReading` for a reading that arrives a few looks late, and `Look.Readings` for the
live arrival path that `semantic.go` already exercises. Nothing here is a new subsystem; it is a
provider that was never wired into the loop that needs it.

## What this does not license

No pixel execution. The evidence got worse, not better: the one control that was learned is a
650×214 rectangle with no name, and the honest thing the commands now do is say so rather than
offer to run it.

## Enforced by

`cmd/director` `TestTheLiveLoopReadsWhatTheScreenSays` holds the gap closed once it is — a live
session with a reader must produce a look that carries text, and the four offline call sites are
not evidence that the live one does.
