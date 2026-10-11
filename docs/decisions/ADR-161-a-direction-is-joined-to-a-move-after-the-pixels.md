---
type: decision
status: accepted
date: 2026-10-11
affects:
  - passive-observation
  - hypotheses
  - semantic-memory
source_paths:
  - internal/director/observe/binding.go
  - internal/director/observe/selectionmove.go
  - internal/director/observe/shadow.go
  - internal/director/observe/hypothesis.go
  - internal/director/observe/approach.go
  - internal/director/service/protocol.go
  - cmd/director/reach.go
---

# ADR-161 — a direction is joined to a move after the pixels, never before

## Context

[[ADR-160-activating-an-entry-is-not-moving-the-mark]] gave a route a row to aim at and a chain
of observed moves to reach it with, and left one gap, named in its own text:

> A chain of moves is therefore a route Marco can DESCRIBE and not one it can perform.

The reason was [[ADR-158-the-active-list-is-the-one-that-responds]]'s trust boundary. The
selection reader is handed a COUNT of directional intents and never a direction, so that an
input can never predict the visual outcome it is being correlated with — and an outcome that
was predicted is not one that was observed. The list that responds to navigation is established
from the shading; if the shading's reader also knew which key had been pressed, "this list moved
down when down was pressed" would be a claim with the answer written into the question.

So the record could say *the mark was seen going entry 1 → 2 → 3, and the screen changed with
entry 3 marked*. It could not say what to press. That is the whole remaining distance between a
route and a taught navigation.

## Decision

**The direction is joined to a move the pixels already proved, afterwards, by WHEN alone.**

The order is the decision, and the code's shape enforces it rather than its comments:

| | who | what it has |
| --- | --- | --- |
| 1. the move | `InterfaceWatch` | shading, exclusivity, and a COUNT of intents |
| 2. the presses | `pressLedger` | the directions, by slot, with the slot's own timestamp |
| 3. the join | `groundSelectionMoves` | a move that already exists, and the ledger |
| 4. the record | `SelectionTransition.Directions` | a candidate, with its uncertainty |

Steps 1 and 2 are written on **adjacent lines of the same gate** in `ShadowTotals.Observe`:

```go
t.watch.Navigated(sample.Timestamp, directionalIntents(sample.Shadow.Inputs))
t.presses.bank(sample.Timestamp, sample.Shadow.Inputs)
```

Same moment, same events, same refusals — and the watch gets a number while the ledger gets the
words. The ledger is unexported, has no reader outside this package, and the watch holds no
pointer to one, so ADR-158's boundary is now structural rather than a convention about what
nobody passes where.

### The join reproduces the watch's decision instead of restating it

`Prompted` on a move already means *a press was attributable*, decided by the watch's own
window — the look interval plus the repaint, clamped to `TrackStaleness`. That window is
refreshed by each directional slot, so **the slot that made `Prompted` true is necessarily the
most recent directional slot at or before the moment the move was seen.** Finding that slot
reproduces the decision without copying the policy, which would drift.

It follows that the attribution is never looser and never stricter than `Prompted`, with one
exception that is a refusal rather than a gap:

- **A slot later than the move is never consulted**, even when it is the one the watch's window
  was refreshed from — which genuinely happens, because the fast reads for an interval are
  folded before that interval's own look. Crediting it would be the input predicting the
  outcome, in the one place the ordering actually inverts. Answer: `unknown`.
- **A move with no clock is refused.** Without a timestamp there is no order, and "the last
  slot" would mean "any slot".

### Three claims, kept apart, exactly as ADR-159 keeps them

| | where it lives | what it rests on |
| --- | --- | --- |
| the observed move | `Count` | the exclusivity rule in the shading, no input consulted |
| the attributed press | `Prompted` | a directional intent inside the response window |
| **the attributed direction** | `Directions` | which word that intent was, joined afterwards |
| the verified binding | **nowhere** | the press performed deliberately and the outcome checked |

There is deliberately no field for the fourth. A `Verified bool` nothing could set is how a
candidate comes to be read as a confirmation, and this is the same argument
`SelectionTransition` already makes for having no field for the confirmed effect. The type is
named for what it is — a count of directions observed — and the strongest word in
`BindingSupport` is `repeated`, not `correct`.

### The uncertainty is itemised, and the three counters partition `Prompted`

`Directions`, `DirectionAmbiguous` and `DirectionUnknown` add up to `Prompted` exactly, which is
what makes the record checkable: every observation with a press attributable to it named one
direction, named several, or lost the slot behind it. Without the partition a direction that was
never found and one that was found and dropped would render identically.

`BindingSupport` is a closed vocabulary of five, because the shortfalls call for opposite
responses and a single confidence number renders them all as the same small float:

- `unattributed` — no press was ever attributable. A quiet player, or a cadence that closed the
  window between looks, and nothing here tells them apart.
- `ambiguous_press` — several directions in the window every time. No single one may be named.
- `contradicted` — different directions across observations of the same move, with no clear
  leader. Evidence *against* any of them.
- `thin_evidence` — one direction leads, on too little to rely on.
- `repeated` — one direction leads by `BindingMargin`, on at least `MinBindingAgreement`
  observations. The strongest thing passive observation can say.

`MinBindingAgreement` is **3**, deliberately above `MinNavigationObservations`: two is what makes
a *move* worth retaining, and a binding is a stronger claim resting on top of that one — it has
to survive a single mis-attribution being outvoted, which two cannot. `BindingMargin` is **2**,
the same shape as `selection.NavMargin`, and it is a margin rather than a veto so that one stray
attribution in nine does not poison a binding for the life of the store.

### It refuses to generalise, and that is the point

A binding is attached to ONE move of ONE list on ONE screen. Not "down means next". Not "down
wraps at the bottom". Not "down from entry 1 of any list". A menu may wrap, may skip disabled
rows, may scroll under the mark, and may be laid out left to right so that `down` does nothing
in it at all — so the rule a route uses stays the chain of moves that were SEEN, and this names
each hop's direction without licensing a hop nobody watched.

`TestADirectionLearnedInOneListIsNotAppliedToAnother` is the form that matters: the same two row
indices, travelled with `down` in a six-entry list and `right` in a nine-entry one. A layer that
had learned a rule about lists would mix them into four agreeing observations.

### The durable boundary, and the plan

`SelectionStep` carries `Directions`, `Ambiguous` and `Unknown` across the durable boundary, and
`foldSelectionSteps` accumulates them — so a thin candidate becomes a repeated one over
sittings instead of resetting every restart. The merge passes every direction through the
vocabulary check on the way in, including on a first sighting: `NavIntent`'s underlying type is a
string, and a `confirm` in this field would be a join crediting a screen change to a move of the
mark.

`Approach` gains `Bound` — how many of the step's observed moves name a direction repeated
evidence stands behind — reported **beside** the move count and never instead of it, because the
gap between them is the thing a reader has to see. `Performable(chain)` answers whether every
hop of a chain names one, and returns the weakest support when one does not.

`Performable` **authorises nothing**. True is the absence of a known gap, never the presence of a
verification; every edge still earns execution through its own rehearsal and its own yes.

## Consequences

**A route can now name the controls it was observed being taken with.** On a fixture where a
nine-row list is walked and one row activated, `director reach` prints:

```
  step 1: state_3 → state_8
          with entry 3 of 9 selected (3 of 3 readable observation(s), 4 observed
          move(s) in that list, 2 with a known direction)
          move: entry 1->2 of 9: 4 seen, 4 prompted; repeated — down: 4
          move: entry 2->3 of 9: 3 seen, 3 prompted; repeated — down: 3
          move: entry 3->4 of 9: 6 seen, 6 prompted; contradicted — down: 3, right: 3
          move: entry 4->5 of 9: 9 seen, 0 prompted; unattributed
```

Four observed moves and two Marco can name a control for: the shortfall is on the surface rather
than in a counter, which is the only reason the surface is worth having.

**On the real 864-frame Trackmania corpus the join lands on everything it is offered, and it is
offered almost nothing.** Both retained moves now name a direction, and both say so weakly:

```
  moves retained      2 distinct, 2 observations
    state      entry   seen   prompted unprom  of   press   support
    state_1    1->2    1      1       0       6    down:1  thin_evidence
    state_1    2->3    1      1       0       6    down:1  thin_evidence
  directions          2 of 2 prompted observation(s) named one direction
    thin_evidence    2 move(s)
```

Two of two is the attribution working. One observation each is `thin_evidence`, correctly — and
the replay says in words that no chain over this session is one anybody could be asked to take.
Everything else on the corpus is unchanged: 204.7 proposals per frame, 864 of 864 grouped, 829 of
829 discovered, selection read 43 of 189 (23%), one vacuous state, no transitions, no durable
edges. **The limit is the perception, not the join**, and
[[Experiment-026-the-same-menu-seen-from-pixels]] says by how much.

**What is still missing for a taught "Enter Free Play"**, in the order it blocks:

1. **Selection reading good enough to trust.** 23% of named entries are the labelled row. Every
   count above is built on readings that are mostly wrong about which row.
2. **A recording that changes screens.** The corpus is one pause menu; the activation half of
   ADR-160 remains synthetic.
3. **The reader's confirmation lag.** At a live 2s cadence, 17 moves produce 1 attribution —
   and an unattributed move can never name a direction, so the lag now costs the binding too.
4. **A verified binding.** Nothing here presses anything. The experiment that would earn one is
   written on each hypothesis as its `Validation`, which is now a sentence a person can act on:
   *from entry 1 on that screen, press down and check the mark arrives at entry 2.*

## What this does NOT do

- **It does not actuate.** Nothing here reaches World State, fusion, planning or policy, and
  [[ADR-010-passive-observation-cannot-execute]] is unweakened: observing that `down` preceded a
  move gives Director no way to press it.
- **It does not weaken ADR-158.** The watch's inputs are unchanged — `Navigated` still takes a
  count, `InterfaceLook.Navs` is still a count, and the selection reader still sees neither.
  `TestTheDirectionOfAPressChangesNothingAboutWhatIsSeen` runs the same pixels twice, navigated
  entirely with `down` and entirely with `up`, and requires every reading, every move and every
  counter to match.
- **It does not record a device.** Which hardware delivered a direction matters to whoever
  eventually performs one, and it is not recorded here: a binding says which *meaning* was
  pressed, and the meaning is the whole vocabulary. `InputLog` retains the device beside every
  admitted event, so the join is available when something exists that would use it.
- **It does not add a taught binding.** A passive observer has no teaching event to read, so a
  `taught` origin nothing could set would be the same dead field as a `verified` one. The
  explicit-licence path belongs with the Learn demonstration code, where the human semantic event
  actually arrives — the same shape as [[ADR-047-a-place-is-remembered-a-meaning-is-answered]].
- **It does not claim an effect.** `Directions` is named for what was observed, like
  `SelectedBefore` and `Preceded` before it. `TestAMoveHypothesisNamesTheDirectionAndNotTheEffect`
  holds the sentences this layer must not produce.

## Enforced by

- `TestAMoveRemembersWhichDirectionWasObservedBeforeIt` (`observe`) — the positive case, through
  `ShadowTotals.Observe`, which is the call the live cycle makes.
- `TestTheDirectionOfAPressChangesNothingAboutWhatIsSeen` — **the invariant**: two identical
  sessions over the same pixels, one all `down` and one all `up`, agreeing on everything the
  pixels decided. The cheap future edit is to pass the ledger into the watch "so the tracker can
  weight a responsive list by whether the direction fits", and this is what catches it.
- `TestADirectionIsNeverInventedWithoutAMove` — the other half of ADR-158's rule: the input must
  not CREATE an outcome either.
- `TestEveryPromptedObservationIsAccountedFor` — the partition, so a lost press cannot hide.
- `TestAPressObservedAfterAMoveIsNotItsDirection` — the two refusals, plus the controls that
  make them mean something: a press before the move is credited, one after it is not, one past
  the ceiling is not, and an unprompted move takes nothing.
- `TestAnAmbiguousWindowNamesNoDirection` — several directions name none; two presses of one
  direction are not several; a `confirm` never reaches the field.
- `TestConditionalKeysStayWeakerEvidence` — [[ADR-013-navigation-is-meaning-not-keys]] carried
  this far, including that one unambiguous press clears the qualification.
- `TestBindingSupportRefusesAThinLeadAndAContradiction` — all five words of the vocabulary, with
  the margin applying before the floor.
- `TestADurableStepRemembersWhichDirectionWasPressed` — survival across the durable boundary
  through the production chain, accumulation across two sittings, and that the same two rows
  travelled both ways keep both directions.
- `TestOnlyADirectionCanBeTheDirectionOfAMove` — the vocabulary enforced on the way into the
  store rather than declared.
- `TestADirectionLearnedInOneListIsNotAppliedToAnother` — the generalisation this layer may not
  make, asserted where a borrowed direction would be used.
- `TestAnActivationNeverBorrowsAMovesDirection` — ADR-160's distinction held across this change.
- `TestAPlannedRouteSaysWhichMovesHaveADirection` — the retrieval, at the one place a route is
  constructed, including that a chain through a contradicted hop is reported as a gap.
- `TestAChainIsNotPerformableUntilEveryHopNamesADirection` — the completeness answer, and that
  an empty chain is complete.
- `TestAMoveHypothesisNamesTheDirectionAndNotTheEffect`,
  `TestAContradictedDirectionIsReportedAsContradicted` — what the hypothesis layer may and may
  not say, and that contradicted evidence names no key.
- `TestTheDiagnosticSaysWhichDirectionsAreKnown` (`cmd/director`) — the inspection surface,
  through the production query over the production store.
- Mutation-gated across every new call site: the bank, the join, the three-way fold, the two
  refusals in `attribute`, both durable projections, both durable folds, the vocabulary check on
  a first sighting, the plan's `Bound`, the hypothesis evidence, the contradiction, the
  validation sentence, and the diagnostic's rendering.
