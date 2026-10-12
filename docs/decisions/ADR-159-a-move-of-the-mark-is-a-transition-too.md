---
type: decision
status: accepted
date: 2026-10-11
affects:
  - passive-observation
  - hypotheses
source_paths:
  - internal/director/observe/selectionmove.go
  - internal/director/observe/interface.go
  - internal/director/observe/shadow.go
  - internal/director/observe/hypothesis.go
---

# ADR-159 — a move of the mark is a transition too

## Context

The screen-state graph can say "one screen became another, and this intent was observed before
it". That is the right unit for *pause opened a menu*, and it is the wrong unit for almost
everything a menu actually does.

Navigating a menu changes neither screen. The composition is the same before and after, so
[[ADR-151-a-screen-that-changed-for-one-frame-did-not-change]]'s gate correctly refuses to call
it a change, and nothing downstream records anything at all. A person can walk a nine-entry
pause menu from top to bottom in front of a session and leave behind one number: a candidate's
transfer count.

[[ADR-158-the-active-list-is-the-one-that-responds]] established which list the navigation was
reaching, and deliberately stopped there. This is the next step, and the shift is from

> something changed on the screen after an input

to

> the selected entry in this particular list changed after a navigation input.

The second is a much more useful thing to know, and it is also a much easier thing to overclaim.

## Decision

**A move of the mark between two entries of a grounded list is retained, grounded in the screen
it happened on where one is known, and reported to the hypothesis layer as its own kind.**

### Three claims, kept apart, in this order

The whole design problem is that these are easy to collapse and the collapse is invisible
afterwards. So each lands somewhere different:

| | where it lives | what it rests on |
| --- | --- | --- |
| **the observed change** | `Count`, and the `structure` support | the exclusivity rule, in the pixels, with no input consulted |
| **the attributed input** | `Prompted`, and the `navigation` support | a directional intent observed within the response window before the move was SEEN |
| **the confirmed effect** | **nowhere** | an action performed deliberately and the outcome checked — a rehearsal, not an observation |

There is no field for the third, and `Validation` says what a person would have to do to
establish it. A hypothesis whose prose claimed the press moved the mark would be the one
sentence this layer must not produce, and `TestASelectionHypothesisClaimsAssociationAndNotCause`
is what keeps it out.

### A move is the gap between two SIGHTINGS, not a diff of two beliefs

Measured, and it is the detail that made the first implementation record nothing at all. The
selection reader declines on the look a mark arrives — the row it left has not been seen
unmarked yet, so for one look two rows stand out and nothing is exclusive. The belief therefore
reads

	entry 3, entry 3, unreadable, entry 4, entry 4

and a rule comparing neighbours sees two changes, neither of them between two entries. On a
scripted five-row walk it recorded **nothing from fourteen moves**.

So the question asked is the one a person would ask: where was the mark last seen, and where is
it now. The unread looks in between are carried on the move as its `Gap` rather than deleted,
because a sighting four looks after the last one is weaker evidence than one on the very next
look and a reader has to be able to tell.

### What identifies a list, and what resets the hold

Where it is, how many entries it has, and the screen it was on when the session could place
one. Any change to the first two drops the held sighting rather than carrying it across:

- **a different envelope** — the same list scrolled, or a different list became the active one.
  "Entry 1 before and entry 3 after" would then be positions in two different arrangements.
- **a different entry count** — the list renumbered. "Entry 3" means nothing except against a
  stable count of them, and a list that gained a row has renumbered everything below it.

Both are checked rather than assumed, because `settle` is reached from six places and only two
of them can produce a real move.

### The screen is provenance, not identity

`SelectionTransition.State` is empty whenever the session could not place a screen, and that is
an ordinary answer rather than a failure. Two causes produce it and both are real: the pixels
discovery path proposes geometry nobody named and screen segmentation reads named roles, so a
session reading a menu perfectly well may have no screen state to attach it to; or a screen
change was pending while the looks were taken, so which screen the list sat on is exactly what
could not be said.

The move was still observed and the list was still established by its own responsiveness.
Dropping a measurement for want of a label is [[ADR-006-unknown-is-not-false]] at the one point
where a graph is most tempted to do it.

### Nothing here knows what an entry MEANS

Entry 3 is the third row from the top. Not "Training", not a label, not a target. A later layer
may learn that a row with a certain name sits at index 3 on a certain screen; this one would be
wrong to pretend it already had.

### An unattributed move contests rather than being dropped

It is the control evidence, exactly as `Unattributed` is for a screen transition, and it has two
readings that call for opposite responses: the mark moved on its own, or the cadence was too
coarse to attribute the press that moved it. Both are stated in the contradiction, because this
layer cannot tell them apart and a generator that picked one would be guessing in the direction
that flatters it.

The consequence is that most real moves are `contested`, and that is the honest reading of
evidence this thin. A clean `supported` means every observation of that one move had a press
attributable to it.

## Consequences

The mark's movement is now something a session keeps. On a scripted five-row list navigated
forty times, 5 distinct moves over 27 observations, all attributed, becoming 5
`possible_selection_transition` hypotheses naming entries and offering a test.

On the real 864-frame corpus it retains **two moves**, because the belief ends `ambiguous` and a
move is only grounded in a list that was established as the one being navigated. The machinery
is not the limit there; the perception is, and
[[Experiment-026-the-same-menu-seen-from-pixels]]'s addendum says by how much.

### What this does NOT do

- **Nothing durable.** No `RememberedRelationship`, no topology edge, no goal. A session-local
  ledger and a hypothesis, which is where [[ADR-158-the-active-list-is-the-one-that-responds]]
  stopped and one layer further on. **Superseded** by
  [[ADR-160-activating-an-entry-is-not-moving-the-mark]], which carries these moves across the
  durable boundary on the edge whose origin they happened on.
- **Nothing executable.** A retained move licenses no actuation and is not a step. Still true
  after ADR-160: a durable chain of moves says the mark was seen travelling, never which key
  sent it.
- **No activation transition.** "With entry 3 selected, confirm arrived at another screen" is a
  different and equally necessary claim — moving a highlight is not opening a menu — and it is
  not made here. **Made in**
  [[ADR-160-activating-an-entry-is-not-moving-the-mark]], as its own hypothesis kind with its
  own subject, so that the two can never be substituted for one another.

## Enforced by

- `TestTheSessionRetainsTheMovesTheMarkMade` (`observe`) — the point of the whole change, as one
  assertion: a menu being navigated leaves something behind, through `ShadowTotals.Observe`,
  which is the call the live cycle makes.
- `TestNavigationAloneRetainsNoMove` — **the invariant**: sixty looks of somebody pressing down
  at a list whose mark never moves retain nothing. An input cannot manufacture a visual outcome,
  and the ledger is the thing a route would be built from.
- `TestEveryMoveOfTheMarkIsOfferedForGrounding` — including the moves only a FAST look saw,
  which live will be most of them.
- `TestAListThatScrolledRetainsNoMoveAcrossTheScroll` — the envelope half of the identity guard.
- `TestARowThatIsNotAnEntryChangesNothing` — the entry-count half.
- `TestAPressDuringASkippedSlotStillAttributesTheMoveItExplains` — navigation reaches the window
  on a slot the detector sat out, which live is three slots in four.
- `TestAFastMoveLongAfterThePressIsNotPrompted` — the other side of the window, on the path that
  carries no navigation of its own.
- `TestAMoveOfTheMarkBecomesAHypothesis` — the ledger reaches the hypothesis layer. Nothing
  downstream reads the ledger, so a ledger nothing generated from would be another case of
  complete code with no consumer.
- `TestASelectionHypothesisClaimsAssociationAndNotCause` — the sentence this layer must not
  produce.
- Mutation-gated across the retention, the guards, the wiring and the generator. One bound,
  `MaxUnreadBetweenSightings`, is **not** verified and says so at its definition: every fixture
  that would reach it runs into the dwell defect first.
