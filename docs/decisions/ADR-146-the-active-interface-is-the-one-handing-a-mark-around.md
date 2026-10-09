---
type: decision
status: accepted
date: 2026-10-09
affects:
  - perception
  - vision
  - game-packs
source_paths:
  - internal/director/perception/selection/active.go
  - cmd/director/selectionread.go
---

# ADR-146 — the active interface is the one handing a mark around

## Context

[[ADR-145-a-chosen-thing-is-one-that-differs]] ends on a measured failure. Offered the regions
a detector proposed, grouping finds every row of a game's menu on every frame — 100% discovery
— and the reader then answers from the wrong list: an advertisement that rotates, a heads-up
display whose numbers animate. End to end that was 100% discovery and 0% reading.

Nothing about a single frame separates them. On the measured frame the menu was not the
biggest list, not the brightest, and not the one with the most going on. The heads-up display
had **more** exclusive frames and **seven times** the transfers.

## Decision

**A list being used shows ONE member marked at a time, and the mark MOVES between members.**
That cannot be seen in a frame, so this is a tracker and not a predicate: it has to watch
somebody navigate.

```
EXCLUSIVE   frames marking several members at once are evidence AGAINST
TRANSFERS   the marked member must change, more than once
HOLDERS     and more than one member must have held it
```

A rotating advertisement changes the same region over and over and the change never transfers
to a sibling. A heads-up display changes several values at once. Neither survives.

### Abstention is the normal early state, and a real answer

Until a list has been seen handing its mark around there is no evidence, and the honest answer
is that the active interface is not known. **Two lists both passing is also an abstention**: a
tracker that picks one to avoid saying "I cannot tell" is guessing with extra steps, and a
guess that happens to be right teaches nothing about when to trust the next one.

Measured, abstention is most of what this does at first — 60 of 100 frames — and that is the
mechanism working. The alternative was answering confidently from an advertisement.

### The ratio that seemed obvious ranks them backwards

A first version also demanded that a candidate carry a mark in most of the frames it was seen
in: *a menu nearly always has something selected*. On real data it ranked the two candidates
**backwards** — the heads-up display scored 0.44 and the actual menu 0.33, because the menu's
last two entries had been grouped separately and the selection sat on them for most of the
run, leaving the rest of the menu honestly quiet and punished for it.

A list need not always carry a mark. What it must not do is carry several at once.

### A frame too thin to judge is not a frame without a mark

When a detector finds some of a list's rows and not others, that is a statement about the
observation, not about the list. Counting it as "no mark" holds a list's own invisibility
against it. [[ADR-006-unknown-is-not-false]], the eighth time, and it was worth 26 points of
exclusivity on the real menu.

### Candidates have no identity between frames

Grouping runs per frame and numbers its groups by where they fell in that frame's proposals.
Those numbers are not stable and neither is membership. So candidates are matched by WHERE
THEY ARE — and matched **one-to-one per frame**, because grouping emits several stacks whose
extents overlap and without that a single candidate absorbs two or three groups every frame.
Measured: a hundred-frame run produced candidates claiming a hundred and eighty observations.

## What it may not use

**Not coordinates, not a game's name, not the annotation's own answer.** If any of those
reached the tracker, "identify the active interface" would collapse into looking the answer
up. `TestTheActiveInterfaceIsDecidedByEvidenceNotByPlace` greps the file for all three.

**Not the controller log.** Marco records what the player pressed, on the same clock, and it
would help. It is refused here on purpose: an input is evidence about a player, not about what
is drawn, and letting it label visual state would make the measurement circular — the thing we
eventually want to learn is precisely the relationship between the two. The same test greps
for it.

## What it achieves, and what it does not

On a hundred frames of Trackmania, through the production detector:

```
named           a list at y 447..868, holding 7 of the 9 annotated entries
rejected        the heads-up display, on its 26 frames marking several members at once
identified      after 61 observations, about four seconds at the capture rate
abstained       60 frames, every one of them before identification
read            38 of 38 frames after identification, ZERO confident errors
```

It names a list, not a menu. It does not know what the entries mean, it has no durable record
of what it found, and nothing it learns survives the process. The seven of nine is grouping's
limit, not the tracker's: the last two entries sit in their own candidates.

**This is not yet part of the world state.** The active interface is the output of a command,
and it should be a belief the Director holds — that is the next decision, not this one.

## Enforced by

Every test is drawn, and the negative ones are the point.

- `TestAListThatHandsAMarkAroundIsTheActiveOne` — the mechanism.
- `TestAHeadsUpDisplayIsNotAListSomebodyIsChoosingFrom`,
  `TestABusyListThatMarksSeveralAtOnceIsStillRejected` — the display is rejected for marking
  several members at once, not for being quiet. The second is the real one in miniature.
- `TestARotatingAdvertisementIsNotAListEither` — change without transfer, the case that
  defeated "the list with the most activity".
- `TestTwoListsBothHandingMarksAroundIsAnAbstention` — abstention as an answer.
- `TestIdentificationNeedsTheMarkToMoveMoreThanOnce` — one transfer cannot tell a mark that
  moved from a mark that appeared somewhere new.
- `TestAListThatIsOftenQuietIsStillAList` — the backwards ratio, as a regression.
- `TestAFrameTooThinToJudgeIsNotHeldAgainstAList` — unknown is not false.
- `TestTheEvidenceIsKeptSoAWrongAnswerCanBeArguedWith` — why this one, answerable.
- `TestTheActiveInterfaceIsDecidedByEvidenceNotByPlace` (`cmd/director`) — the whole line this
  milestone turns on, asserted by reading the file.
