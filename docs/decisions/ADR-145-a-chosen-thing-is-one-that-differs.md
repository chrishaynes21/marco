---
type: decision
status: accepted
date: 2026-10-09
updated: 2026-10-09
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

### A fading cue is not a current selection

Added 2026-10-09, after the first measurement. Six confident wrong answers, all one mechanism:
leaving an entry, its text stays brighter than its neighbours' for a frame while the arriving
entry's cue has not yet established. That is unlike its siblings AND away from its own
baseline, so **both** kinds of evidence name the entry the selection has just left. Magnitude
cannot separate them; direction can. A row whose distance from baseline is collapsing is
losing the selection, not holding it.

And having become unsure that way, the reader must **not** fall back on the previous claim.
Holding is for a frame that could not be read; this frame was read, and what it says is that
the selection is leaving. Continuing to claim it is the reader insisting on a thing it has
just watched end.

Result: six confident wrong answers became six honest declines, correctness unchanged, and
**transition latency unchanged** — median 0 frames, worst 3. That last number is the point: the
dishonest way to stop being wrong during a transition is to wait longer, and only latency
separates the two.

### A difference nobody could see is not a cue

Normalising by a robust spread is not enough on its own. A tight group of near-identical rows
has almost no spread, so the floor does the dividing and a six-unit wobble scores as high as a
bar of a hundred and twenty. Offered every regular stack on a real frame, the reader handed
the answer to a heads-up display. A cue must also clear an absolute threshold: a selection is
something a person can see.

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

`FindRows` exists so the reader can be measured from pixels alone, and **it is the weak half**:
8% correct end to end against 98.8% with rows supplied. A list's rows and the letterforms
drawn inside them are both real changes in brightness, and an over-segmented list is just as
regular as a correct one, so a luminance profile alone cannot choose between them.

So `GroupRows` takes the regions **the production detector already proposed** and asks only
which of them form a list — the same repeating-structure rule, applied to arrangement rather
than to pixels. Two things it learned from real detector output:

**A row contains its caption, and the caption arrives first.** The detector proposes both; a
deduplicator keeping whatever it saw first discards the row as a duplicate of its own label.
Every one of Rocket League's four menu rows vanished that way — the stack was in the proposals
and grouping threw it out before looking. The container is the candidate.

**Every list is offered, not the largest.** A screen holds more than one: the measured
Trackmania frame's medal panel is six evenly spaced rows, as regular as the menu and smaller,
and choosing "the biggest regular stack" picked the wrong pile. Entry discovery went from 4%
to 100% by refusing to choose. There is no general rule for which list a person means, and
inventing one would be guessing — but the reader already decides by evidence, within a group,
and a list nobody is choosing from has no deviant row.

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

## What this does not solve

Discovery is no longer the bottleneck. **Telling which list is live is.**

Run end to end on Trackmania — production detector, then grouping, then the reader — entry
discovery is 100% and selection reading is 0%. The reader picks a deviant row from the
advertisement panel, which rotates, or from the heads-up display, which animates. Those are
real visible changes in genuinely regular lists; nothing distinguishes them from a menu except
that nobody is choosing from them.

On Rocket League the same pipeline works completely (100% discovery, 8 of 9 read, zero
confident errors), and the only reason is that its pause screen has little else moving on it.

This is a different problem from the one it replaces, and it should not be solved by a rule
about where menus live or what they look like. The honest shape of it: a list being navigated
shows selection moving BETWEEN ITS OWN MEMBERS over time, one at a time, while an animating
advertisement does not. That is a judgement about a group's history rather than a frame, and
nothing has measured it yet.
