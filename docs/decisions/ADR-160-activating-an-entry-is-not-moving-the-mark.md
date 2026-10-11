---
type: decision
status: accepted
date: 2026-10-11
affects:
  - passive-observation
  - hypotheses
  - semantic-memory
source_paths:
  - internal/director/observe/activation.go
  - internal/director/observe/approach.go
  - internal/director/observe/screenstate.go
  - internal/director/observe/hypothesis.go
  - internal/director/observe/relationship.go
  - internal/director/observe/goal.go
  - cmd/director/reach.go
---

# ADR-160 — activating an entry is not moving the mark

## Context

[[ADR-159-a-move-of-the-mark-is-a-transition-too]] made the mark's movement something a session
keeps: entry 2 became entry 3, in a list that was independently established as the one being
navigated, with the screen the same throughout.

That is half of using a menu, and it is the half that never arrives anywhere. A route made of
moves alone can walk a highlight up and down for ever. The other half is the screen changing
while a particular row was the selected one, and the two are not interchangeable:

> Moving a highlight is not the same as opening a menu.

Nothing recorded that. A screen transition said `state_1 → state_8`, with `confirm` observed
before it — and a nine-entry pause menu produces exactly the same record whichever row the
person was on. Every destination out of that menu was indistinguishable in the graph.

There is a second, larger gap behind it. ADR-159 deliberately wrote nothing durable: a
session-local ledger and a hypothesis, and nothing that outlives a restart. So even where the
evidence was perfect, the next session renumbered every state and the knowledge was gone. The
question that matters for a reusable macro is not whether the evidence was detected but whether
it can be **retained, distinguished, and later retrieved when a route is built**.

## Decision

**Two things, and they are separate claims kept in separate places.**

### 1. A confirmed screen change records what was selected on its origin

`ScreenTransition` gains `SelectedBefore` — the rows the origin was pointing at, one entry per
distinct selection with how often it was that one — and `SelectedUnknown`, how often nothing
readable was.

| | where it lives | what it rests on |
| --- | --- | --- |
| **the observed change** | `Count` | the same `StabilityLooks` gate every other transition passes |
| **the selection beside it** | `SelectedBefore` | exclusivity in the shading, with no input consulted |
| **the press beside it** | `Preceded`, `Sequences` | a navigation intent observed before the change |
| **the confirmed effect** | **nowhere** | the action performed deliberately and the outcome checked |

`SelectedBefore` is named for what was observed, not for what it would take. "Screen A became
screen B four times, three of them with entry 3 of 9 selected" is a correlation. A field called
`Requires` would have been read as a precondition within a week, and nothing here establishes
one.

The generator is `possible_entry_activation`, and it is a different KIND from
`possible_selection_transition` rather than the same kind with extra fields. A reader can tell
them apart without consulting the kind at all, because the subjects differ: a move names the
same screen on both sides, an activation names two.

**Why the entry count travels with the entry.** "Entry 3" is not a fact on its own. A list of
nine rows and a list of four both have a third row, and a list that gained a row has renumbered
everything below it.

**Why the selection is the PREVIOUS look's.** The look a change is first seen on is already
showing the new screen. Measured: on a destination with nothing shaded on it — a fade, a loading
frame, a screen of plain text, which is the commonest thing in the live traces — the belief goes
stale on that very look, so a recorder reading it records *no row at all*. On a destination that
does present a list, the belief still reports the OLD list's row for several looks, because the
selection reader refuses a row it has seen once. Either way the row a route needs is the one
from the look before the divergence, which is the single look of history `lastInputs` already
keeps and for the same reason.

### 2. The evidence crosses the durable boundary, and a plan carries it

`RelationshipEvidence` and `RememberedRelationship` gain `SelectedBefore`, `SelectedUnknown` and
`Moves` — the latter being the moves of the mark observed inside the ORIGIN's list, joined on the
screen the change starts from, projected onto a durable `SelectionStep` that carries an entry, a
count of entries and two counters and nothing else.

`GoalPlan` gains `Approaches`, aligned with `Steps`: per step, the row the origin was observed
pointing at, how many of the readable observations had it, and the moves seen inside that list.
`Approach.Reach` assembles a chain from wherever the mark is now to that row.

**Why the durable step carries no geometry.** A `SelectionTransition` holds a normalised
rectangle because a session has to tell one list from another within a look. A durable record
does not: the subject already identifies the screen, and a second durable identity for the same
list would be a weaker answer to a settled question. The split is structural — a different type
— rather than a matter of remembering to strip fields, which is the same reason `NavSequence`
exists beside `TargetedSequence`.

**Why the approach is on the plan and not fetched beside it.** Because it is part of what the
route is. A chain of screens with nothing to say about which of nine rows each is entered from is
not a route through a menu, and a caller that assembled the selection half itself would be a
second reader of the same topology. `PathRank.Because` is on the plan for the same reason.

**Why the chain is a search over observations and not arithmetic.** "Entry 1 to entry 3 is two
moves down" is a rule about how lists work, and this subsystem is not allowed to have one: a list
may wrap, may skip disabled rows, may scroll under the mark. The chain is assembled from the
moves that were SEEN, and a gap in them is reported as a gap.

### What this does NOT do

- **It does not say which key.** A `SelectionStep` says the mark was seen going from entry 1 to
  entry 2; it does not say what produced that. The selection reader is handed a COUNT of
  directional intents and never a direction, so that an input can never predict the visual
  outcome it is being correlated with — [[ADR-158-the-active-list-is-the-one-that-responds]]'s
  rule, which this does not touch. **A chain of moves is therefore a route Marco can DESCRIBE
  and not one it can perform**, and that is the next thing needed for a taught "open the third
  entry". It is recorded as a known gap rather than papered over.
  **CLOSED by [[ADR-161-a-direction-is-joined-to-a-move-after-the-pixels]]**, which joins the
  direction AFTER the move exists, outside anything that reads a mark, and keeps ADR-158's
  count-only input to the watch exactly as it was. A hop can now name a direction; what still
  has nowhere to be recorded is that pressing it ARRIVES — which needs a rehearsal, and is the
  third claim ADR-159's table has always left empty.
- **It authorises nothing.** An approach is a reading of remembered observation exactly like the
  plan it hangs off. Every edge still earns execution through its own rehearsal and its own yes.
- **It claims no requirement.** An approach that read `SelectedBefore` as a precondition would be
  claiming the change needs that row.
- **It invents no labels.** Entry 3 is the third row from the top. Not "Training", not "Free
  Play". A later layer may learn that a row with a certain name sits at index 3 on a certain
  screen; this one would be wrong to pretend it already had.

## Consequences

A route can now name a row. On a fixture where a six-entry list is navigated and one row
activated twice, the edge is recorded with `entry 3 of 6` on both observations, becomes a
`possible_entry_activation`, survives `RelationshipsFrom` and `Fold`, and comes back out of
`PlanToGoal` as an approach a person can read on `director reach` — with the chain of observed
moves that would get the mark there.

**On the real 864-frame Trackmania corpus it records nothing, and the reason is the recording.**
The corpus is 864 frames of one pause menu; the screen never changes, so there is no activation
to observe. `0 change(s) over 0 observation(s)` is the honest result and the replay now says it
in those words. What the corpus still measures is the half ADR-159 built, unchanged: 2 moves
retained, selection read 43 of 189 (23%). The limit there remains the perception, and
[[Experiment-026-the-same-menu-seen-from-pixels]] says by how much.

**The approach surfaces on `director reach`.** A step now prints the row it was observed being
taken from, or `no entry known for this step (entry_unknown)`. That surface is the point rather
than a convenience: a field nothing renders is a field that quietly stops being filled, and this
repository has eight recorded cases of exactly that.

## Enforced by

- `TestAChangeRecordsWhatWasSelectedWhenItHappened` (`observe`) — the first claim, through
  `ShadowTotals.Observe`, which is the call the live cycle makes.
- `TestActivatingAnEntryIsItsOwnKindOfTransition` — the two kinds carry different subjects, in
  one session, checked where a consumer would read them.
- `TestMovingTheMarkIsNeverAnActivation` — **the invariant**: a nine-entry menu walked fourteen
  times over, with no screen change, produces moves and no activation. Moving a highlight is not
  opening a menu, and the cheap future edit is to notice that both name an entry and fold them.
- `TestTheSelectionRecordedIsTheOneBeforeTheChange` — the subtle half, and it asserts the
  DIRECTION: a recorder one look late does not lose the row, it credits it to the return edge.
- `TestAStaleOrUnplacedSelectionIsNotActivationEvidence` — the one function that decides what
  counts, with the five refusals it makes.
- `TestAChangeWithNothingSelectedIsCountedAsSuch` — the control evidence, and that no hypothesis
  is generated without a readable selection anywhere.
- `TestAnActivationHypothesisClaimsAssociationAndNotCause` — the sentence this layer must not
  produce.
- `TestADurableEdgeRemembersWhatWasSelected` — survival across the durable boundary, through the
  production chain, and accumulation rather than replacement across two sessions.
- `TestAnUnplacedMoveIsNotEvidenceAboutAnEdge` — a move on a screen nobody could place is not
  attached to an edge.
- `TestAPlannedRouteCarriesWhatEachStepWasSelectedFrom` — the retrieval, at the one place a route
  is constructed.
- `TestAnApproachChainsTheMovesThatWereObserved`, `TestAnApproachWillNotInventAMoveItNeverSaw`,
  `TestAnApproachRefusesAListThatRenumbered`, `TestAnEdgeWithNoSelectionEvidenceSaysSoPlainly` —
  the chain, and the three refusals that keep it from becoming arithmetic.
- `TestTheDiagnosticSaysWhichEntryAStepWasTakenFrom`, `TestAStepWithNoEntryEvidenceStillPlans`
  (`cmd/director`) — the inspection surface, through the production query.
- Mutation-gated across all fifteen new call sites: the fold, the belief's route into the
  segmenter, the one-look history, the generator, both durable projections, both durable folds,
  the plan's approaches, the unplaced-move exclusion, both chain refusals, the staleness refusal,
  the unreadable counter, and the diagnostic's rendering.
