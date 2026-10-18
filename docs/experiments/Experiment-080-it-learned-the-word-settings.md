---
type: experiment
status: complete
date: 2026-10-17
backend:
  - structure
game: arkascended
fixture: "the owner's own hands, on a live Ark: Survival Ascended menu, through `director demonstrate-visual` with live OCR running beside the loop. Three clicks, ENTER to finish, then `sequences` and `reacquire` against the reopened menu. No input injected and none simulated. The artifact is at $APPDATA/marco/sequences/ark_menu_2.json."
result: "one-control-of-three-learned-as-the-word-settings-on-a-312x52-rectangle-and-reacquired-on-98-percent-of-300-looks-with-zero-collisions-while-the-proposer-granularity-varied-rather-than-improved"
supersedes: []
affects:
  - perception
source_paths:
  - docs/experiments/Experiment-079-the-first-human-game-menu-demonstration.md
  - cmd/director/visualsession.go
  - internal/director/observe/controllabel.go
---

# Experiment 080 — it learned the word "settings"

The owner's question, put as plainly as it can be put:

> When you click PLAY in Ark, does Marco learn PLAY — or does it learn a rectangle that happens
> to contain PLAY?

The second human demonstration, with the reader wired in. **It learned the word.**

## What the artifact holds

```json
"controls": [{
  "label": "control_1",
  "readings":   [{ "x": 0.41875,  "y": 0.4, "width": 0.1625,  "height": 0.0481 }],
  "containers": [{ "x": 0.02083,  "y": 0,   "width": 0.97917, "height": 1      }],
  "text": ["settings"]
}]
```

On a 1920×1080 window the reading is **312 × 52 pixels at (804, 432)** — a button — and its
container is the whole window, so the control was distinguished from the only thing that
contained it. Beside the first demonstration:

| | demonstration 1 | demonstration 2 |
|---|---|---|
| control's reading | 650 × 214 at (644, 322) | **312 × 52 at (804, 432)** |
| containers recorded | none | the window |
| text | none | **`["settings"]`** |
| reacquired | 2% of 250 looks | **98% of 300 looks** |
| collisions | 0 | 0 |

## The six questions

### 1. Were all three intended presses captured? — Yes

`4 of 7 classified`, with `no_session`, `unplaceable_pointer` and `unsupported_navigation`
ignored — the event before the subscription opened, the click back onto the terminal, and the
ENTER that ended it. Three controls were identified, which matches three clicks.

### 2. Which words were observed near each click? — one of three

```
target_1   read as "settings"
target_2   nothing was read on it
target_3   nothing was read on it
```

1442 words were read across the session, so the reader was working throughout and two of the
three controls got none of them. `controlWords` keeps a word only when it was read on a
`TextCommon` share of the looks of that control's screen, and the two unnamed controls are on
screens the session barely stood on.

### 3. A specific button, or a broad panel? — a button, and OCR did not cause that

312 × 52 with one word on it is a button. But the honest reading of the pair of demonstrations is
not "OCR fixed the granularity":

**`structure.Propose` runs on pixels and knows nothing about words.** The region proposer is
unchanged by this session's work. The difference between 650 × 214 and 312 × 52 is which button
was clicked and what the proposer happened to find there — so granularity **varied** rather than
improved, and the panel case is still live. What OCR changed is that the button, once found, has
a name.

### 4. Were the expected transitions observed? — one click, one automatic, two nothing

```
1 visual_1 -> visual_2 via target_1   [geometric]
2 visual_2 -> visual_3 via (no control — the screen changed for another reason)   [none]
```

Three screens seen and three visited. One transition attributed to a click; one that happened by
itself, recorded as `automatic` with the recorder's own note that *the response to this step is
to wait and verify*; and two presses after which nothing changed.

Two of three clicks changed nothing in both demonstrations. That is now a pattern rather than an
incident, and the `automatic` step is evidence the screen model sees changes it cannot attribute
— so "the menu did not change" and "the change was not attributed to the press" are both live and
still not separated.

### 5. Can the controls be reacquired? — 98%, with one caveat that matters

```
300 usable look(s)
words: offered 300, read 54, dropped while busy 245, 2295 word(s), mean 525ms, slowest 663ms
control_1   98% matched   0% ambiguous   1% unobservable   2 indistinct
colliding matches: 0
```

**`reacquire` compares rectangles and nothing else.** It does not re-check that the matched
rectangle still reads "settings". So 98% means *a 312 × 52 rectangle of this shape is in this
place on 98% of looks* — strong, and not yet *the settings button is there*. The artifact now
carries the word and the matcher does not consult it.

### 6. Incorrect, ambiguous, abstentions?

| | |
|---|---|
| collisions — the only wrongness detectable without a person | **0** |
| ambiguous | 0% |
| unobservable | 1%, of which 2 looks indistinct from a container |
| **incorrect** | **unknown.** No independent check exists yet |

## And one reporting gap this run exposed

**The screens carry no text at all**, though the session delivered 30 readings and one control was
named from them. The wiring is right — `SaveWith` is given `obs.SaidBy(id)` for every screen —
so `SaidBy` returned unread for all three.

Which cannot be diagnosed from what is printed, because the report says how many readings reached
the LOOP and not what the observer did with them. The observer counts
`ReadingsAdmitted`, `ReadingsStaleWindow`, `ReadingsStaleEpoch`, `ReadingsSilent` and
`ReadingsRefusedMerge`, and none of them is shown. So three explanations remain open and the
session cannot choose between them: the readings were refused as stale because the epoch moved
while they were in flight; or they were admitted and the vocabulary did not reach
`TextStableLooks`; or they were filed against a screen other than the one being asked about.

A control got its word, so admission works at least sometimes. Printing the observer's own
counters is the next diagnostic, and it is a reporting change rather than a perception one.

## What this licenses, and what it does not

Not pixel execution. One control, one press, no independent confirmation that the 98% match is
the right rectangle, and the recorder's own note on the step says so: *clicked once, so nothing
independent confirms that this control can be found again.*

What it licenses is a far narrower next step than a visual target model: the word is already
saved and the matcher does not read it. See
[[ADR-233-a-match-that-can-be-corroborated-by-its-words]].

## Afterwards: the corroboration, and the counter that explained the silence

[[ADR-233-a-match-that-can-be-corroborated-by-its-words]] is implemented as measurement only.
Re-run on the same saved control, with Ark genuinely in front:

```
readings: 35 offered, 31 admitted, 4 stale (epoch 4, window 0)
control_1   99% matched   corroborated
            it still reads "settings", from 27 readings
```

**The rectangle Marco matches is the rectangle that reads "settings".** That statement did not
exist in this experiment's first half.

And the controlled negative, with the SAVED word changed to "inventory" and the geometry and the
fresh reading both real:

```
control_1   100% matched   contradicted
            it was saved as "inventory" and now reads "settings", over 18 readings
```

Geometry matched completely and the text refused it.

### The silence, explained

The first run of the corroboration printed this, and it is the answer to the open question above:

```
readings: 37 offered, 0 admitted, 37 on a look that could host none
```

**`ReadingsUnusable`.** `intakeReadings` refuses a reading on a look whose window is not in
FRONT, because window capture is not occlusion-safe and a covered window's words are another
application's. The person had alt-tabbed to the terminal to press ENTER, so the looks that
carried readings were looks of a window that was no longer in front — and nobody printed that
counter, so three explanations had stayed open.

The same run also exposed a defect in the corroboration's own first version: it reported
`corroborated` from 28 of those 37 refused readings, because it asked `Admissible` — which checks
the window generation and the observation epoch and not usability. Corroborating against the
terminal's pixels is worse than not corroborating at all. Fixed, and
`TestAMatchIsReportedWithWhatItSays` holds the gate.

## Prepared: the two validations that need a real menu

The owner's gate before any pixel action:

> 1. Capture the same Settings button twice across separate visits to the Ark menu.
> 2. Evaluate the saved Settings target against a different real Ark menu with similar geometry.
>
> Do not use a synthetic saved-word substitution as the only negative test.

Both need hands on the game. What was done instead of substituting for them is to make sure a
live run ANSWERS, and to establish which link in the chain the answer would be about.

### The twice-visited chain works, on the production observer

`TestOneButtonOnTwoVisitsIsOneControlInTheReport` drives `fastobserve.Observer` through the exact
shape of the demonstration — settle on the menu, press the middle entry, settle on the second
screen, come back with no press, press the same entry again — and all three links hold:

| link | holds | held by |
|---|---|---|
| the revisited menu is the same screen | yes | the arrangement relation, `screens seen < walk` |
| the two presses merge into one identity | yes | `sameControl`, mutual centre containment |
| the pair walked twice becomes two steps | yes | ADR-174's occurrence walk |

`Presses: 2`, `Repeated()` true, and the step's caveat — *clicked once, so nothing independent
confirms that this control can be found again* — is GONE, which is the sentence the whole
validation exists to retire.

**What that does not prove is the first link on Ark's pixels.** The regions fed to the observer
are identical on both visits by construction, so the arrangement relation cannot fail; a real
menu redrawn with a different highlight, a loading spinner or a changed player count can. That
is the live question, and it is the one the second visit is being asked for.

### So two controls is the interesting result, and the report now says which kind

A control identity is scoped to a screen on purpose, so there are exactly two ways to come back
with two controls and they need opposite answers. The report printed the control and its press
count and not its screen, which could not separate them. It now prints the screen:

```
control_1   on visual_1   1 press(es), read as "settings", led somewhere
control_2   on visual_1   1 press(es), read as "settings", led somewhere
```

Two on ONE screen — the menu was recognised and the presses did not merge. Evidence about the
proposer's rectangles.

```
control_1   on visual_1   1 press(es), read as "settings", led somewhere
control_2   on visual_3   1 press(es), read as "settings", led somewhere
```

Two on TWO screens — the menu was not recognised on the way back, so the second press was never
offered the first one's identity. Evidence about screen identity, and correct behaviour on a
wrong premise.

### The different-menu negative needs no code, and one instruction

`reacquire` already separates the four outcomes the owner asked for, and reports geometric
matches beside them:

```
  N of M control(s) were found on at least one look.
  P text-corroborated, Q text-contradicted, R unread.
```

The one thing a run can get wrong is the operator's: the verdict accumulates over every look of
the run, so a run that begins on the demonstrated menu and navigates away folds both menus'
readings into one answer. Stand on the other menu for the whole run. `E2E.md` carries that and
the three honest outcomes — `0% matched`/`unread` is the abstention, `N% matched`/`contradicted`
is the refusal working, and `N% matched`/`unread` is an abstention and **not** agreement.

A fourth would be a finding against Marco: `corroborated` on a menu with no Settings button.
