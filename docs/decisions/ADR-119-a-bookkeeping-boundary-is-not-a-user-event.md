---
type: decision
status: accepted
date: 2026-09-01
affects:
  - passive-observation
source_paths:
  - cmd/director/observeaction.go
  - cmd/director/observeambient.go
---

# ADR-119 — a bookkeeping boundary is not a user event

## Context

Normal dogfood, one person walking Windows Settings:

```
Watching: 4 screens, 8 moves noticed
Learning: 3 relationships seen, 3 remembered
```

**Five of eight crossings were seen and produced no edge.** The ledger held no record of the
arrival at Bluetooth & devices at all — not promoted, not refused, not waiting — so a place Marco
had been to, could name, and could not return to.

The mechanism: an action is filed against the session-local screen state it was performed on, and
its destination resolves on the NEXT reading. Ambient sessions end every twenty seconds, so a
person clicking at ordinary speed regularly presses just before one ends. `drain` then clears the
pending map because the new session's state numbering restarts, the crossing arrives with no
action, and `noticed` drops it at `len(s.Did) != 1`.

The comment guarding that clear is right and stays:

> *"placing it against whatever the next session happens to see first is exactly the guess this
> file exists to refuse"*

What was wrong is treating **a session boundary** as equivalent to **evidence became
unresolvable**. They are not the same thing.

## Decision

One unresolved action is held across one boundary, and nothing else is. The map goes, the cursor
goes, the state ids go.

**The same physical interaction produces the same graph fact wherever the counter falls** — before
the press, between the press and the arrival, or not at all. That equivalence is the acceptance,
and it is a test.

Every way it refuses:

| | |
|---|---|
| more than one unresolved press | ambiguity is a reason to refuse, not a detail to resolve |
| older than five seconds | a page that has not arrived did not arrive because of that press |
| a second boundary | one rollover is an accident; two is a person who moved on |
| a degraded reading between | a gap nobody read cannot be crossed |
| a newer press | supersedes; the arrival belongs to the later one |
| watching stopped | nobody leaves a press for the next session to finish |
| a different application | held twice — see below |

Nothing is written down, nothing survives the process, nothing reaches a Place, a plan or an input.
**Naming is untouched:** dwelling still improves what a place is called and still cannot synthesise
a crossing. *"I later recognised Bluetooth"* never implies *"the earlier click reached it"*.

## What the mutation gate found

Five mutations killed, and three that survived — each of which was a claim in a comment that turned
out to be false:

- **An unreachable session field.** `carryAcross` clears before it considers carrying again, so a
  second rollover destroys an unclaimed carry on its way past. A `c.into != session` comparison
  inside `claimCarried` could never fire. Removed; this repository treats a guard for a case that
  cannot arrive as a claim nothing can test.
- **An unreachable application comparison** in the same function, for the same reason. Removed.
- **A test helper that hid the failure it forbade.** `edgesAfter` filtered to one-action crossings,
  so a carry wrongly bringing two presses across produced a two-action step the test could not see.
  Fixed, and the single-action rule is now genuinely held.

Two guards remain that no single mutation can kill — the application check in `carryAcross` and the
supersede in `attribute`. Both are real double defence rather than dead code, and both are now
**written down as redundant** instead of claiming tests they do not hold.

## Consequences

- Edges the person actually demonstrated should now survive a rollover. The live rate was 3 of 8;
  what it becomes is a dogfood question, and 8 of 8 is not promised — some movements legitimately
  carry no attributable action.
- A wrong edge remains worse than a missed one, so every ambiguous case still refuses.
- Scoped affordances stay deferred, and 38C.1 semantic repair is **paused, not abandoned**: the
  Xbox stable-shell problem is separately established and unaffected by this.

## Enforced by

- `cmd/director` `TestAnAttributedCrossingBecomesAnEdge` — the control, no rollover
- `cmd/director` `TestARolloverBetweenAPressAndItsDestinationKeepsTheEdge`
- `cmd/director` `TestASessionBoundaryIsInvisibleToTheGraph` — the equivalence
- `cmd/director` `TestTheCarryRefusesAStalePress`,
  `TestTheCarryDoesNotSurviveASecondBoundary`
- `cmd/director` `TestTwoUnresolvedPressesAreNotCarried`,
  `TestACompetingActionSupersedesTheCarry`
- `cmd/director` `TestADegradedReadingBreaksTheCarry`,
  `TestStoppingClearsAPendingCarry`
- `cmd/director` `TestTheCarryRefusesADifferentApplication`

## Related

- [[ADR-116-watching-follows-the-window-not-the-executable]]
- [[ADR-118-a-reading-can-be-read-and-still-say-nothing]]
- [[Experiment-022-the-first-dogfood]]
