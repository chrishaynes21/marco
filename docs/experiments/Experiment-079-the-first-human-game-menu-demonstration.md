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

## Afterwards: the baseline, the wiring, and what the words show

### The baseline reacquire, against the live Ark window

Ark was still in front, so this was taken against the real menu rather than against whatever
covered it. Twice, because the second run had OCR behind it:

```
blind     250 look(s)   control_1   2% matched, 98% unobservable, 0 collisions
reading   200 look(s)   control_1   0% matched, 100% unobservable, 0 collisions
```

**The 650×214 panel is not re-proposed either.** It is not a control that matches everything; on
this menu it matches almost nothing, which is the other way for a target to be useless. The
caveat that matters: the demonstration ended on a different screen from the one `control_1` was
learned on, so a low match rate is expected and the number is a floor rather than a measurement.
Zero collisions over one control is trivially true.

### The reader, wired and measured

`readingEyes` already existed — a source on its own clock, one frame deep, non-blocking, results
stamped with the generation and epoch they were computed under — and `director perform-workflow`
was its only caller. It now lives on `visualSession`, which `observe-screen`,
`demonstrate-visual` and `reacquire` all share. On by default, `--blind` to opt out.

Measured live against Ark at 10 looks a second:

```
offered 200, read 33, dropped while busy 165, 1731 word(s), mean 572ms, slowest 734ms
200 usable look(s)      <- 20 seconds at 10/second, exactly on target
```

- **5.7 looks late on average, 7.3 at worst.**
  [[Experiment-043-a-reading-that-arrives-late]] measured the admission window:
  260 of 300 admitted at 3 looks late, 223 at 8, and the screens MERGE at 12. So the live cost
  sits inside the working range with margin, and past it the failure is the one that existed
  before any of this.
- **The cadence was untouched.** 200 looks in 20 seconds is the 10/second target exactly, with
  165 of 200 frames dropped while a reading was still running. Dropping is the ordinary case and
  the number that says how far behind OCR is.
- **~52 words a look** on this menu, where there were none before.

Degradation is honest in both directions. No plugin prints a warning and records a demonstration
with unread controls, which is the record this command produced before today. A reading that
ERRORS publishes nothing at all rather than an empty reading — a screen recorded as having no
words on it is a screen that can never contradict anything, which is the merge the whole
mechanism exists to refuse.

### And the words show the granularity problem directly

`director ocr --region 644,322,650,214` — the exact rectangle the click was attributed to, on the
live Ark window:

```
accepted 20     rejected_confidence 13     (of 33 engine results)
capture 29ms    recognise 290ms            total 320ms
```

**Twenty accepted words inside the one "control"** (24 on a second run). The machinery that would
name a control from them is already built and already wired —
`ScreenReading.Words` → `Observer.read` → `ClickTargets.Reading` → `LabelOf(where, read)` →
`NameCandidates`, with `controlWords` keeping a word only when it was read on a `TextCommon`
share of looks, because *the person clicked PLAY on the look where OCR had stopped reading the
word PLAY*.

What it would produce for a 650×214 region is a label naming every button in the block. So the
reading is necessary and is not sufficient, exactly as the owner said:

> Even perfect OCR wouldn't necessarily fix that. It might read the words inside the panel, but
> Marco would still need to determine which specific control you clicked.

The candidate that follows from this measurement, and it is a candidate rather than a change: the
smallest OCR WORD BOX containing the demonstrated click is a target of roughly a button's size,
and finding it uses the click coordinate without storing it as identity. It is untested, it needs
a demonstration taken with the reader running, and no part of it has been implemented.

### The mutation run

Eleven mutations across the loop, the two commands and the reader. All eleven killed, after
three rounds, and two of the rounds say something about structural tests:

| it survived because | the fix |
|---|---|
| `if false && s.reader.working()` leaves every line of the OCR block written and unreachable, which a test reading the file cannot see | assert the GUARD — `if s.reader.working() {` — because that one line decides whether any of it runs |
| asserting on the bare word `mean ` matched a COMMENT about the mean while the reported string had been renamed | assert on `", mean "` with its quotes |

The second is the sharper one. **A test that reads prose is a test of the prose**, and every
assertion in this file reads a file. They earn their place by covering what a behavioural test
cannot reach — watching a real window with a real OCR plugin behind it — and they pay for it by
being this easy to fool.
