---
type: experiment
status: complete
date: 2026-10-08
backend:
  - undecided
game: rocket-league
fixture: .tmp/game-capture-review
result: selection-is-deviation-not-a-highlight-style
supersedes: []
source_paths:
  - cmd/director/capturegamesession.go
  - internal/platform/padsource
  - internal/director/visionbench/temporaltruth.go
  - internal/director/perception/selection
---

# Experiment 023 — menu selection state

## The question

[[ADR-144-the-corpus-must-say-which-entry-is-chosen]] established that the corpus cannot say
which menu entry is chosen, and that selection is the only thing that changes in a menu — so it
is the progress signal a menu search is bounded by. The schema can now express it. Nothing has
been observed.

This experiment collects the evidence and answers four questions:

1. Can a selected entry be identified from visual evidence?
2. Can a **changed** selection be told from an **unchanged** one?
3. Can the legitimate state where **nothing** is selected be recognised?
4. Can an observed input be associated with a subsequent selection change **without confusing
   temporal correlation with causation**?

The fourth is the one that needs saying out loud. Seeing `down` followed by the highlight
moving is evidence of a transition. It is not proof that every `down` causes one, and the
corpus already contains the counter-example that proves the point: at the bottom of a list,
`down` is followed by nothing changing at all.

## Definition of done

> Given previously unseen frames from a visually rendered menu, Marco identifies the currently
> selected entry — or reports that none is selected, or that the evidence is insufficient — and
> detects selection transitions without relying on game-specific rules.

Three answers, not two. "Insufficient evidence" is a first-class outcome here for the reason it
is everywhere else in this system ([[ADR-006-unknown-is-not-false]]): a transitional frame
caught mid-fade is genuinely unreadable, and a detector that guesses on it will be right often
enough to look good and wrong exactly when a search needs it most.

## What the capture must contain

Acceptance needs all five, and a session missing one cannot answer the questions above:

| condition | why it is needed |
|---|---|
| held selection | the baseline, and what "unchanged" means |
| movement in both directions | a detector that only ever sees `down` can encode position rather than selection |
| boundary behaviour | `down` at the last entry: an input with no transition, which is question 4 |
| pointer-driven deselection | the `none-chosen` state, already evidenced in `pause-close` |
| transitional frames | fades and animation, where the honest answer is "insufficient" |

`director capture-game-session --application rocketleague --session <name>` records it:
frames and observed navigation intents on one measured clock, into a private scratch
directory. It injects nothing, approves nothing, and **states nothing about what is selected**.

## The separation this experiment depends on

**Recording is not annotation, and neither is inference.** The capture writes frames,
timestamps and intents. A person annotates afterwards, as [[Vision-Corpus-Workflow]] already
requires of every other frame in this project.

**The brightness measurement is a hypothesis, not a specification.** ADR-144 found the gap by
measuring mean brightness over the annotated entry rects — 215 against 93–100 — and that number
is about one game's art direction. It must not become the acceptance criterion, and it must not
become the recorder's output: if the ground truth and the detector's cue shared an ancestor,
the benchmark would stop being independent evidence and start agreeing with itself. A menu
whose selection is a border, an arrow, a size change or a colour shift at equal luminance would
defeat it, and those are all ordinary.

So the detector is evaluated on **held-out transitions** — sequences it was not tuned against —
and a general visual criterion is what is being looked for.

## What this experiment does not do

No controller input injection. No game-specific production selectors. No corpus expansion
beyond the one session, until these questions are answered — ADR-144's gap is one missing
sequence, not a general shortage of frames.

Autonomous menu navigation is **two** steps away, not one. After this comes learning the
relationship between an input and a selection transition; only then is stepping a menu for
real on the table, and that is the point at which an isolated machine stops being a
convenience. Observing a person play needs no isolation at all.

## What the capture found

One 60-second Trackmania session, 900 frames at a measured 15.0 fps, 31 controller intents,
zero dropped frames. The first 36 frames showed an account screen and were removed; the
sanitized working copy is 864 frames with original indices and timestamps preserved, and the
original was deleted. It is private and unpromoted.

Five of the six conditions are present. **Boundary behaviour is not, and cannot be**: this
menu wraps in both directions, so every input produces a transition. Nor is a stable
none-selected state — what first looked like one turned out to be EXIT, chosen in a second
visual style.

| finding | consequence |
|---|---|
| eight entries use a filled bar with inverted text | selection can be a region's appearance |
| EXIT uses brighter text and no bar | selection cannot be defined by one style |
| every transition is exactly one blended frame | a momentary absent cue is not an absent selection |
| a one-frame flicker with no input near it | per-frame confidence is not confidence |
| the menu wraps both ways | navigation topology must be learned, not assumed |
| no stable none-selected frame | none-selected recognition cannot be validated yet |

## What the reader achieves

[[ADR-145-a-chosen-thing-is-one-that-differs]] is the design. Measured over all 864 frames,
rows supplied by the harness:

```
on annotated selections    98.8% correct   0.7% confidently wrong   0.5% declined
on annotated transitions   88.6% correctly unsure, 4 claimed a selection
none-selected              UNVALIDATED — no examples exist in this session
```

All six confident wrong answers are the same mechanism in the same place: entering EXIT, the
reader is two frames late, and during the lag the DEPARTING entry's residual brightness makes
it the spatial outlier. Two of the six literally report "unlike its siblings" about the entry
the selection has just left.

Cross-renderer, on the Rocket League pause corpus, which nothing in the reader was tuned
against: **8 of 9 correct, zero confidently wrong**, one declined — the first frame, which is
a change from nothing and waits for confirmation like any other.

End to end, with rows found from pixels rather than supplied, 8% correct. Locating the rows is
the limiting component and is a perception problem; see the ADR.

## Status

Capture complete, reader v1 built and measured. Not yet proven: none-selected recognition, any
boundary case, and row finding.

## Enforced by

- `TestTheRecordSaysNothingAboutWhatIsSelected` (`cmd/director`) — the separation, asserted
  over what the command actually writes.
- `TestFrameTimesAreReadFromTheClockNotTheSchedule` — measured times, because correlating a
  press with the frame after it against *scheduled* times mis-orders the one pair that matters.
- `TestTheControllersOwnCodesCannotCrossTheBoundary`
  (`internal/platform/padsource`) — the record says "the player moved down", never which button.
- `TestARestingStickInventsNothing`, `TestAHeldButtonIsOnePressNotAStreamOfThem` — the two ways
  a recorder could manufacture the correlations this experiment exists to test.
- `TestTheGameSessionCommandIsReachable` — the command is dispatched, not merely written.
