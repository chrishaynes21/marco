---
type: decision
status: accepted
date: 2026-10-09
affects:
  - perception
  - vision
  - game-packs
source_paths:
  - internal/director/perception/selection
  - cmd/director/selectionread.go
---

# ADR-145 — a chosen thing is one that differs

## Context

[[ADR-144-the-corpus-must-say-which-entry-is-chosen]] established that selection is the only
thing that changes in a menu, and therefore the signal a menu search is bounded by. A capture
then supplied the evidence ([[Experiment-023-menu-selection-state]]), and the evidence
immediately refused the obvious design.

**Within a single Trackmania menu, selection has two completely different visual
representations.** Eight entries are chosen by a white filled bar with their text INVERTED to
dark. The ninth, EXIT, is chosen by its text simply getting brighter, with no bar at all. A
reader that looks for the bar reports nothing on every frame where EXIT is chosen — silently,
and that is 8% of the recording.

Rocket League, measured on the existing corpus, uses a third thing: a pale fill whose text
stays light.

So "find the highlight" is not a specification. There is no highlight; there are highlights.

## Decision

**A chosen thing is one that differs. The reader knows nothing about what a highlight looks
like.** Two kinds of evidence, and both are needed:

```
SPATIAL    among rows that look alike, the chosen one does not
TEMPORAL   the row standing away from its OWN unselected level is where the choice is
```

The first covers every filled style. The second exists because of EXIT, which sits apart from
the stack and has no siblings to be unlike — the only evidence available for it is that it is
not what it was. Neither test assumes a direction: a menu that marks its choice with a dark
fill deviates downwards and reads the same way.

### A row's baseline is its unselected level, not its recent average

The obvious temporal test — compare a row against its own recent frames — fails exactly where
it matters, and fails quietly. Hold a selection for a second and the window fills with
selected levels, the row stops looking different from itself, and the reader drops a selection
that is still plainly on screen. So a row's samples are collected **only from frames in which
it was not being claimed**, and they do not expire merely because time passed.

### Two features, and the second was earned

A row's overall level is enough for a bar, which repaints most of a row, and useless for a
selection drawn by brightening the TEXT: text covers a small part of a wide row, so a change
obvious to a person moves the mean by two or three units. Measured, that cost every frame of
EXIT — the reader held the previous entry instead, confidently and wrongly.

So a row is described by its mean **and by the level of its brightest quarter**, scored
separately, each row keeping its strongest deviation. The brightest quarter is the general
form of "what is drawn here" and it moves for all three shapes a selection takes.

### A change must be seen twice

Every transition in the measured menu lasts exactly one frame, and that frame is a blend: the
departing entry's text brightens while the arriving entry's bar is washed out and its text
grey. By mean luminance the blend is 204 against a settled 206 — a per-frame rule answers
confidently and wrongly, and did, on a measured flicker that moved the bar to a neighbour and
back with no input anywhere near it.

The blend is distinguishable by one thing: it does not last. So a change of winner is not
asserted until a second frame agrees. It costs one frame of latency on every real change and
refuses every one-frame artefact.

Holding is **bounded**. After enough unreadable frames the reader stops claiming the old
selection, because an old observation is not a current one.

### Three answers, and NoneSelected requires evidence

```
Selected(row)   fresh visual evidence supports this row
NoneSelected    fresh evidence affirmatively supports NOTHING being chosen
Insufficient    this frame cannot be read
```

**An unrecognised cue is Insufficient, never NoneSelected.** A reader that has failed to
recognise a style has learned nothing about whether something is chosen, and must not dress
that up as a finding. [[ADR-006-unknown-is-not-false]], seventh time.

The capture contains no frame in which nothing is chosen — what looked like one turned out to
be EXIT, drawn in the second style. So **NoneSelected is unvalidated**, every report says so,
and no number about it may be quoted.

## Finding the rows is not this package's job

The reader is handed candidate regions. Where they come from — perception, a learned detector,
an accessibility tree — is the caller's business, and keeping it out is what stops one game's
geometry leaking in.

`FindRows` exists so the reader can be measured from pixels alone, and **it is the weak half**.
Measured on the same session: with rows supplied, 98.8% correct and 0.7% confidently wrong;
with rows found by `FindRows`, 8% correct. A list's rows and the letterforms drawn inside them
are both real changes in brightness, and an over-segmented list is just as regular as a
correct one, so regularity alone cannot choose between them. Locating rows is a perception
problem and belongs to [[Perception]] and [[Vision]], which have far better evidence than a
luminance profile.

## Enforced by

Every test in `internal/director/perception/selection` but one is drawn rectangles, because a
reader tested only on the recording it was built from learns that recording.

- `TestTheOddRowOutIsTheChosenOne`, `TestADarkHighlightReadsTheSameAsABrightOne` — the spatial
  rule, and that it carries no assumption about which direction means chosen.
- `TestARowWithNoSiblingsIsJudgedAgainstItself` — EXIT's situation, and the reason the
  temporal half exists.
- `TestASelectionSurvivesBeingHeld` — forty held frames, the failure a sliding window causes.
- `TestAOneFrameFlickerIsNotASelection`, `TestAConfirmedChangeIsAccepted` — both sides of
  confirmation: refuse the artefact, accept the real move.
- `TestTwoRowsDeviatingTogetherIsUnreadable` — what a blend looks like from inside.
- `TestNoRecognisedCueIsNotAFindingThatNothingIsChosen` — the distinction the package turns on.
- `TestHoldingAPreviousSelectionIsBounded` — temporal context without carrying an old answer
  forever.
- `TestTheReaderCrossesToAGameItWasNotBuiltOn` — the one test on photographs: the Rocket
  League pause corpus, a different renderer and a different highlight, read 8 of 9 with zero
  confident errors by a reader developed entirely on the other game.
- `TestTheSelectionReaderIsReachable`, `TestSuppliedRowsAreHarnessOnly` (`cmd/director`) — the
  door, and the line between harness input and the reader. The second greps the package for
  either measured game's coordinates and vocabulary.
