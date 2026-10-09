---
type: decision
status: accepted
date: 2026-10-08
affects:
  - control-flow
  - programs
  - perception
  - semantic-actions
source_paths:
  - internal/director/search
  - internal/director/execute/look.go
  - internal/director/execute/pipeline.go
  - cmd/director/runtime.go
---

# ADR-143 — a search is a wait with a move in it

## Context

Everything above perception acts on what it can see. One reading of the current screen, one
question asked of that reading, and a question the reading cannot answer is refused — honestly,
and finally.

That gap is the shape of most whole-task failures this project has measured. Each individual
operation works; the task fails because the thing the next operation needs is not on screen yet,
and nothing goes to find it. Named in the previous session as the single biggest structural gap:
**Marco can act but it cannot look for anything.**

[[ADR-141-finding-text-by-what-it-says]] closed one slice of it — finding a run of text inside a
value the editor had already read, which needed no new perception at all. This is the general
case, where finding the thing means **changing what can be seen**.

## Decision

**A search is the wait loop with a move where the pause is.** This is deliberately not a new
mechanism.

[[Control-Flow]]'s engine already runs:

```
observe → evaluate → satisfied? stop. otherwise PAUSE, and observe again
```

and its own package comment is careful about the pause: *"The one duration in this package is
the interval between looks, and it is not evidence of anything."* A wait is therefore a search
over TIME with no moves — it looks at the same place repeatedly and waits for the place to
change.

A search replaces the pause with a bounded change to what can be seen. Everything else is the
same: the same `conditions.WorldCondition` to say what is being looked for, the same three-valued
evaluation, the same Unknown gate, the same hard bound. Reusing the condition vocabulary is what
makes the capability shared rather than text-shaped — nothing in the loop knows the move is a
scroll or that the world came from the accessibility tree.

### Four outcomes, and the middle two are the point

```
found        the condition held somewhere, and here is the world it held in
exhausted    every place this pattern can reach was looked at AND READ; it is not there
unreadable   some place could not be read, so "it is not there" is not established
stopped      cancelled, or the move failed — nothing is claimed about where the thing is
```

`exhausted` is why the package exists. A single failed look says "not in this reading". An
exhausted search says "not anywhere I could get to", which is a far stronger statement and the
only one that licenses telling somebody their control is not there.

**The sharpest rule here is the line between `exhausted` and `unreadable`.** One place that
could not be read is one place the thing could have been, so the search has established that it
did not *see* it, which is a different sentence and has to be a different word.
[[ADR-006-unknown-is-not-false]], for the fifth time. The two sentences also have different
remedies: "it is not there" sends somebody to look elsewhere, "I could not read part of it"
sends them to bring the window forward.

That rule would make `exhausted` nearly unreachable on its own. At the measured rate of two thin
readings in twelve ([[ADR-137-a-thin-reading-is-not-an-answer]]), a five-place search would
almost always include one unreadable place. So an Unknown is **read once more before it is
believed**, by the same argument that makes a second look free upstream: looking has no effect,
so nothing can be applied twice. It matters more here, because here an Unknown costs the search
its answer.

### The move set is closed, and that is the load-bearing constraint

A search performs moves **without being asked**. That is only acceptable while each move
changes what can be seen, or what is highlighted, and **commits nothing**. Both halves matter.

Today the list has one entry: a semantic scroll. `MaySearchWith` refuses everything else, and a
pattern built on a click, a keystroke, an edit or even `scroll_here` is rejected before a single
move is performed — including the move that puts the view back, which the search also makes
unasked.

### Bounded, and the view goes back

A budget of six places, as a constant rather than a setting: nobody has asked for a different
one and a knob would invite somebody to set it to fifty.

**An unchanged world is not a place that was searched.** A region already at its bottom accepts
a page-down and reports success forever; without a progress check the search spends its whole
budget re-reading one screen and then claims to have looked everywhere — which turns an honest
`exhausted` into a confident lie. [[ADR-007-no-progress-no-repetition]] in a third disguise, this
time about looking rather than acting.

On anything but `found`, **the view is put back**. A search that found nothing has still moved
somebody's screen, and leaving it at the bottom of a list with nothing to show for it is the
Director changing what a person is looking at as a side effect of being wrong. When a pattern
cannot restore, the result says so rather than letting the sentence imply nothing happened. On
`found` the view stays, because the next step is about to act on what was found.

## Where it is consulted, and where it is not

One point in the pipeline, and the choice of point is the whole safety argument. The resolve
stage has four answers and now treats two of them as recoverable:

| status | recovery |
|---|---|
| `unobservable` | look again, **same place** — the reading did not represent the window |
| `absent` | look **somewhere else** — the window was read and the thing is not in it |

The difference is re-reading a page against turning one. `unobservable` does **not** earn a
search: moving the view would be looking harder at something Marco cannot read, spending the
budget producing more unreadable places and then reporting `unreadable`, which is what it already
knew.

A target that resolves in the first reading never costs a move, so the search is a recovery and
not a preamble — a Director that scrolled before acting would move the view on every request.

The found world is carried forward and re-prepared, which is the **continuation**: the step that
could not resolve a moment ago resolves now, and the request proceeds without the person being
asked to break it into pieces.

## Two things this was built beside, both unwired

Found while looking for the right place to put this. Both are the failure mode the mutation gate
exists for — complete, correct code that production never invokes — and they are the seventh and
eighth recorded cases:

- **`directorapi.DiscoveryPlan`** — a complete design for "look somewhere not currently
  visible", with enumerated probes, a cap, reversibility and a cleanup step. Built by
  `target.Discoverer`, printed by `director inspect`, and **executed by nothing**.
- **`directorapi.RecoveryScrollIntoView`** — declared, and referenced nowhere in the tree.

Neither was absorbed here, and the reason is the move set. Discovery's probes are **clicks that
open a container**, and `DiscoveryPlan`'s own doc already makes that argument — *"opening a menu
is low risk and reversible; that is precisely why menus are the only thing probed by default"*.
That is the second pattern, with a cleanup requirement scrolling does not have, and it is a
decision of its own rather than a line added to a closed list in passing.

## What is next, and what this is not

The next pattern is a **game's menu**: step through the entries until the one being looked for is
highlighted. It is deliberately absent, because its argument is genuinely harder and has not been
made. Arrow keys in a menu move a highlight, which commits nothing and is reversible; arrow keys
in a list box change a **selection**, which some applications treat as a choice, and in a game
they may be the input the game is reading. Telling those apart needs to know what kind of thing
is in front of Marco — which is what the vision and game-pack work is for. Until then a search
refuses to use them and names the move it refused.

This is not goal-directed planning. The search does not decide **what** to look for or **why** —
it is handed a condition and a pattern. What to look for is the request's own target; choosing
between several places worth looking is a plan, and nobody has needed one.

## Enforced by

Every test in `internal/director/search` is non-actuating: the worlds are written down and the
moves are counted.

- `TestWhatIsAlreadyOnScreenCostsNoMoves`, `TestSomethingThreePlacesAwayIsFound` — the loop, and
  that the found world is what comes back.
- `TestLookingEverywhereAndNotFindingItIsExhausted`,
  `TestAPlaceThatCouldNotBeReadIsNotAPlaceThatLacksIt` — the two negative answers, apart.
- `TestAThinReadingIsReadAgainBeforeItCostsTheSearchItsAnswer` — the re-look that makes
  `exhausted` reachable.
- `TestTheBudgetIsTheBound`, `TestAnUnboundedSearchIsRefused`,
  `TestAViewThatStoppedMovingIsNotMorePlaces`, `TestAViewWithNowhereToGoCostsOneMove` — the
  bounds, including that one move is spent learning the view has nowhere to go.
- `TestTheViewIsPutBackWhenNothingWasFound`, `TestTheViewIsNotPutBackOnSuccess`,
  `TestAPatternThatCannotRestoreSaysSo` — the restore, and the sentence when it cannot happen.
- `TestASearchWillNotClickToLookIsTheLoadBearingRule`, `TestScrollingIsTheOneAdmittedMove`,
  `TestTheHomeMoveIsGatedToo` — the gate. The second walks the whole semantic vocabulary rather
  than naming scrolling, so a second entry cannot arrive without this test being read.
- `TestStopEndsTheSearchAndPutsTheViewBack`,
  `TestAStoppedSearchClaimsNothingAboutWhereTheThingIs`, `TestAMoveThatFailedIsNotAnAnswer`,
  `TestAnObservationThatFailedIsNotAnAnswer` — the stopping paths, none of which may read as
  absence.

And the wiring, which the loop's own tests cannot prove:

- `TestTheDirectorLooksForATargetThatIsNotInTheReading` (`internal/director/execute`) — the
  production pipeline over a scrolling fixture: absent on the first screen, found on the third,
  and the action performed on it. Deleting the `lookElsewhere` call kills it.
- `TestAnUnreadableWindowIsNotSearched`, `TestSomethingAlreadyOnScreenIsNotSearchedFor` — the
  line between the two recoveries, and that the search is not a preamble.
- `TestASearchThatFoundNothingSaysSoMoreStronglyThanOneLookCould`,
  `TestAnUnreadablePlaceStopsTheSearchFromClaimingAbsence`,
  `TestAMoveThatFailedDoesNotBecomeAFindingAboutTheScreen` — the sentence the person gets.
- `TestThePipelineTheDirectorBuildsCarriesTheSearcher`,
  `TestTheSearchMovesThroughTheSameGuardedExecutorAsThePlan` (`cmd/director`) — structural, at
  the Director's own construction site, because the behavioural test builds its own pipeline and
  passes with `Searcher:` deleted from `NewRuntime`. The second holds the rule that a search
  reaches the desktop through the same guarded executor the plan does: by any other route it
  would be outside the foreground guard and outside the lowering record.
