---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - navigation
source_paths:
  - internal/director/observe/reacquire.go
  - internal/director/observe/clicktarget.go
  - internal/director/fastobserve/fastobserve.go
  - cmd/perceptionreplay/reacquire.go
---

# ADR-180 — finding a demonstrated target again

## Context

[[ADR-179-the-first-live-observation]] closed the watching half: screens recognised on return,
presses correlated with the changes that followed. The owner named the next capability and the
standard it has to meet:

> Given a previously observed screen, can Marco find the same clickable target again without
> relying on the original click coordinates?

> A system that confidently identifies the wrong button is worse than one that says it cannot find
> the button.

Without the coordinates is the whole point. A stored pointer position re-presses a place on the
screen rather than finding a control, and the first time a menu scrolls or a window resizes it is
a confident wrong answer.

## Decision

`observe.ClickTargets.Reacquire(screen, regions)` takes a screen identity and the proposals of the
look in front of it. The press position is not an argument and is not reachable from inside. It
returns one outcome per demonstrated target on that screen — **including the ones it could not
find**, because a report of five targets that names two matches and says nothing about the other
three is the shape of a measurement that flatters itself.

```
  matched       one control, agreed on by every supporting proposal
  ambiguous     more than one control could be it — reported, never resolved
  unobservable  nothing the proposer found supports it
```

`Reacquisition.Actionable()` is matched and certain, and it says **could**: the authorisation is
somebody else's to give and the legal Marco boundary is untouched.

### No new threshold, and that was the hard part

Everything is decided by `sameRect` — mutual centre containment — the relation `identify` already
uses to decide that two readings are one control. A reacquisition rule with its own tolerance
would be a second opinion about what "the same control" means, free to drift from the first, and
this subsystem has a measured history of invented ratios being wrong.

Two consequences fall out of that choice and both are load-bearing:

**How many CONTROLS, not how many proposals.** A proposer that read one row twice — as a row and
as a span of it — offers two rectangles that agree with each other. Reporting that as an ambiguity
would report a disagreement nobody has, so the supporting proposals are partitioned into groups by
`sameRect` and the count of GROUPS decides. One group is a match; more than one is ambiguous.

**A region that merely contained the press is not support.** `sameRect` is not a size test, and
that matters more than it sounds: a thin row that straddles its panel's centre contains the
panel's centre, and the panel contains the row's, so **the panel is "the same rectangle" as the
row**. Without the exclusion a reacquisition matches the menu and offers its middle as the button.
A test found it, not reasoning — the row a third of the way down the panel hid it and the row at
the panel's centre exposed it. Any container that can do this was necessarily recorded, because
the press was inside the control and the control inside the container.

Where a proposal cannot be told from a recorded container, the outcome is `unobservable` and says
**which** refusal it is, because "the control is gone" sends a reader to the screen and "the
control cannot be told from its panel" sends them here.

### An accumulation that could never accumulate

`visualTarget.where` was documented as every rectangle a control had been found as — "keeping them
is what let the second demonstration recognise the first one's click". It always held exactly one.

`identify` is given the smallest candidate **alone**, a single rectangle. A later press reaches the
accumulating loop only by AGREEING with a stored reading, and the loop appended only readings that
DISAGREED with all of them: the condition that admitted the press and the condition that would
have stored it were each other's negation. `MaxTargetRegions` on that field was unreachable too.

Deleted rather than excused, which is this repository's rule for an unreachable guard. The
tolerance that actually makes a control survive a proposer reading it slightly differently is
`sameRect`, and that needs one reading rather than a collection. Reaching the accumulation is a
change somebody should make deliberately, after measuring what merging does to the identity count
— it is the identity rule, and ADR-172 measured four of those.

## Measured on rl-freeplay-1

900 looks, 9 screens, 12 demonstrated targets, folded through the production observer with
reacquisition computed **before** each look was folded, so nothing a look teaches can help
recognise what that look was meant to recognise.

```
  repeated evidence: 4 of 12 target(s) clicked more than once

  over 1287 target-look(s) on a target's own screen, EXCLUDING the looks it was pressed on:
      matched         672   52.2%
      ambiguous       156   12.1%   (abstention, not an error)
      unobservable    459   35.7%   (abstention, not an error)
        of which 36 could not be told from a region that merely CONTAINED the press

  GROUND TRUTH, from second presses only:
      3 verified, 0 contradicted, 2 declined

  false matches: 0 colliding pair(s) over 900 looks
```

### The ground truth, and why it is the only number worth much

A match rate cannot say whether a match is RIGHT. Four targets were clicked on more than one
occasion, and a second press is independent evidence about where the first one's control was: if
reacquisition — computed before that press was folded — names a rectangle the press then lands on,
the control was genuinely found again.

**Three verified, none contradicted, two declined.** Five repeat presses: three times the right
rectangle had already been named, twice the matcher abstained on the look that could have proved
it, and **never once did it name a rectangle the press did not land on**. That is the owner's
standard met on the evidence available — and it is five data points, which is not a rate.

`0 colliding pairs over 900 looks` is the other wrongness check: no two targets on one screen ever
matched the same rectangle, so no match was wrong in the way that is detectable without a label.

### Two targets of twelve can never be found again

`target_2` on `visual_2` matched on none of its 33 looks, and `target_6` on `visual_6` on none of
its 96 — 25 of those because its reading cannot be told from a region that contained it. These are
honest refusals rather than errors, and they are also a real cost: a control in that position is
permanently unreacquirable, and the conservative side is the right one to err on.

`target_1` on `visual_1` matched 139 of 376 looks, was ambiguous on 74 and unobservable on 163. A
target that flickers is not an error — a dialog covers a button — but it is not one a plan could
rely on either.

### The limit geometry cannot see, recorded as a test that asserts the wrong answer

If a list scrolls by one row, the row that now sits where the demonstrated one sat satisfies every
geometric test there is, and reacquisition returns `matched` for the wrong control.
`TestAShiftedNeighbourIsMatchedAndThatIsTheLimitOfGeometry` asserts that wrong answer on purpose,
so the limit is recorded rather than discovered later; the day something fixes it, that test is
what fails and says what changed. The recording has no scrolling list, so this limit is reasoned
rather than measured on real pixels.

So what a `matched` means is **"a control of this shape is in this place"**. A second demonstration
is what turns that into evidence about a control, and four of twelve targets have one.

## A red measurement of nothing

The first version of the replay scoring reported **7 contradictions of 10** and it was entirely an
artefact: it checked every reacquisition on the standing screen against every press, so a press on
one control "contradicted" the three other controls on the same menu. Recorded beside
ADR-178's green measurement of nothing, because the two failures are the same failure — a number
that answers a question nobody asked, read as though it answered the one that was.

A press is a repeat of the target whose stored reading agrees with the region the press landed on,
which is `identify`'s own rule. With that corrected the same run reports 3 verified and 0
contradicted.

## Consequences

- **Nothing is durable and nothing is executable.** Target identities stay session-local; the
  session-local tier's allowlist grew a third time, deliberately, with the test failing first.
  `reacquire.go` produces the most promotable-looking thing the tier has ever held — a `matched`
  verdict — which is why it belongs inside it.
- **`fastobserve.Observer` exposes `Registry`, `Screen` and `Reacquire`**, so the capability is
  reachable through the type the live command runs rather than only from a replay.
- **A match is not permission.** `Actionable` is a precondition, not an authorisation, and no
  caller of this may inject anything.
- **Two of twelve targets are unreacquirable** and one flickers. Reacquisition is not a solved
  problem; it is a measured one.
- **The keyboard is unchanged.** [[ADR-179-the-first-live-observation]]'s single unknown cause is
  still unknown, and nothing here infers keyboard causality.

## Enforced by

- `internal/director/observe` — `TestADemonstratedTargetIsFoundAgainOnALaterLook`,
  `TestAContainerIsNotTheControlItContained`,
  `TestAControlIndistinguishableFromItsContainerIsRefused`,
  `TestTheSameControlReadTwiceIsOneControl`,
  `TestTwoDisagreeingCandidatesAreAmbiguousAndNeitherIsChosen`,
  `TestADisappearedTargetAndAnUnreadableScreenAreBothUnobservable`,
  `TestAnUncertainIdentityStaysUncertainWhenItIsFound`,
  `TestTheRegistryCarriesTheWholeEvidenceForATarget`,
  `TestMoreProposalsDoNotChangeTheAnswer`,
  `TestATargetSurvivesLeavingTheScreenAndComingBack`,
  `TestAShiftedNeighbourIsMatchedAndThatIsTheLimitOfGeometry` (records a limit),
  `TestAVisualScreenCannotReachDurableMemory`.

## Mutation gate

21 mutants, all 21 killed. Three survivors were thin TESTS rather than missing guards, and all
three were the same mistake — a fixture with one of something, so a filter that returned
everything looked correct: one target in the registry (so "this target's destinations" had nothing
to be wrong about), one target clicked twice (so "repeated" was true either way), and no target
minted on an unidentified screen (so the guard against matching on one had nothing to guard).

## The live command reports it too, before a run is asked for

`director observe-screen` prints the registry and a per-target reacquisition tally, computed
before each look is folded and excluding the looks a target was pressed on — the same rule the
replay uses, so a live session and the recording produce comparable numbers.

It distinguishes **"no target's screen ever came back"** from **"the target could not be found"**.
Two silences with the same shape and opposite meanings: the first has measured nothing about the
matcher, and printing `0%` for it would report a failure at a test that was never given. A second
bounded live run would be proposed on the strength of this output, so it had to be unambiguous
before anybody is asked to sit through one.

Mutation gate: a further 7 mutants, all killed.
