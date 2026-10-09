---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - perception
  - vision
source_paths:
  - internal/director/observe/interface.go
  - internal/director/observe/shadow.go
  - internal/director/perception/providers/vision/provider.go
  - cmd/director/shadowsample.go
---

# ADR-147 — the interface in front of a session is a shadow belief

## Context

[[ADR-146-the-active-interface-is-the-one-handing-a-mark-around]] can identify the list
somebody is using and read what is chosen in it. It did so in a command, over recorded
frames. A capability that only exists in a replay tool contributes nothing to the Director's
decisions, and this repository has eight recorded cases of exactly that.

The question is where it belongs. The Director must be able to answer, from its own current
state: **what interface am I looking at, what is selected, and what changed since the last
observation?**

## Decision

**It is folded with the shadow evidence, and it is a belief about NOW rather than a finding
about a session.**

Shadow, because that is what it is made of: regions a detector proposed, and two numbers
describing how each one is painted. Putting it on the authoritative sample would make it
something fusion could reach, and *"a menu is open and Settings is selected"* is not
something Marco should be able to believe its way into acting on — the same argument
`shadow.go` already makes for player input.

[[ADR-102-a-detector-earns-its-place-where-perception-fails]] is why it is worth having at
all: a detector is admitted by the ABSENCE of better evidence, and a game has no
accessibility tree. That is the case ADR-102 anticipated, and this is the first thing to use
it.

### Two numbers travel with the geometry

A session cannot see. Regions reach it as window-relative normalised rectangles, and the
question an interface hypothesis turns on — does one row of this list look unlike its
neighbours — cannot be asked of a rectangle.

So the vision provider measures each region's shading **where the pixels still exist** and
carries two floats along: an overall level and a brightest-quarter level. There is nowhere in
them for a label, a word or a screen coordinate to hide, which is the trade [[Navigation]]
already makes for key codes. The same function measures them as reads them downstream, so a
live reading and a reading from a recording are the same measurement.

A region nobody measured is dropped rather than defaulted. A zero would read as the darkest
thing on screen.

### It may never say the entries are all of them

`Complete` exists, is always false, and is in the payload. The grouping that produced the
entries is a guess about arrangement, and on the measured menu it found seven of nine — the
last two sat in their own candidates. A belief reporting "a menu of seven" would be wrong in
a way nothing downstream could detect, so a consumer counting entries has to pass the field
that tells them not to.

It also never says an entry can be pressed.
[[ADR-004-vision-cannot-establish-actionability]].

### Three statuses, and two of them are answers

```
unknown      nothing has been seen behaving like a list being used
ambiguous    more than one list has; picking between them would be a guess
provisional  one list has, and provisional is the strongest this evidence supports
```

### What changed is told, not diffed

A consumer that polled and diffed two snapshots would own its own idea of when a selection
"moved", and the Director would stop being the authority on its own beliefs. `live.go` makes
this argument for the whole event stream; the same applies here. A look that changes nothing
says so.

### A look nobody took is not an empty screen

Found by preparing a live run rather than by a test, and it is the difference between a replay
and a Director. Replaying a recording, every look carries a frame. Live, the shadow cadence
lets an inference start every two seconds while the session samples twice a second — so three
looks in four bring nothing at all, and the first version folded those in as evidence of an
empty screen. The belief would have flapped to `unknown` three looks in four and the tracker
would never have accumulated. [[ADR-006-unknown-is-not-false]], at the exact place a recording
cannot show it.

So a look says whether it was taken. Four outcomes are kept apart — no detector configured, the
detector could not run, the cadence gate held it back, and it ran and described the screen — and
only the last may move a belief. A look that brought nothing marks the belief **stale** and says
how long it has been unconfirmed and which sample it still rests on; past ten such looks the
belief is dropped, with the reason, because holding it longer would be a claim about a screen
nobody has looked at. Ten is the order of what it costs to earn one (`selection.MinObservations`):
a belief is held no longer than it took to acquire.

The detector running and finding nothing is the other case entirely, and it still means
`unknown` immediately. That is evidence about the screen.

### When the answer changed

A bounded journal of the looks at which the belief was different, each with its sample and its
time. Not a log of looks — that would be a record of the sampling rate. It exists because a
client polling at its own rate would otherwise report what it happened to catch, and the
question a live session has to answer is what the Director emitted and when.

### What this evidence is allowed to do

The line the session report used to print — *"this evidence cannot reach belief, planning,
policy or input"* — was wider than the rule the code holds, and two of its four nouns were
already false in this build: `observe.Hypotheses` is derived from these totals, and a plan reads
the relationships those hypotheses support. A boundary printed wider than the one enforced
teaches the reader the wrong rule and makes perception look unusable to the agent that owns it.

The rule that holds is narrower and stronger:

> **No authority over belief** — fusion cannot read any of this. It MAY inform hypotheses and
> proposed actions. It can never authorize an effect, and it can never prove one happened.

Which is what matters for games, where accessibility evidence does not exist: the alternative
reading would mean building perception the Director is forbidden to use.

## What this is not

It is not learning. Nothing is remembered across sessions and no relationship is recorded.

The place for that already exists and should be extended rather than replaced:
`observe.PossibleTransitionAction` — *"an intent that repeatedly preceded one change"* — is
generated from `ShadowTotals`, which is now where this belief lives. Its contract already
holds the hard parts: `supported` needs **two independent sources and two episodes**,
navigation evidence can support *an action related to a change* but never *what the
destination is*, contradiction is first-class and terminal, and `validated` requires asking a
person. See [[Hypotheses]].

## Enforced by

- `TestASessionAnswersWhatItIsLookingAt` — the acceptance question, driven through
  `ShadowTotals.Observe`, which is the fold call site shadow evidence actually goes through.
  Deleting the publication kills it.
- `TestASessionAnswersWhatChangedSinceTheLastLook` — both ends of a move, and that a look
  which changed nothing still says so.
- `TestASessionThatSawNoListSaysSoRatherThanGuessing` — abstention as an answer.
- `TestASessionWithNoShadingBelievesNothing` — unmeasured regions support no belief.
- `TestADetectedRegionCarriesHowItIsShaded`,
  `TestShadingDescribesTheRegionAndNotTheFrame`
  (`internal/director/perception/providers/vision`) — the measurement happens, and describes
  its own region rather than the frame. Tested at the provider because if it stops happening
  everything downstream keeps working and quietly believes nothing.
- `TestAShadedRegionKeepsItsShadingOnTheWayToASession` (`cmd/director`) — the join through a
  map of `any`, which is the kind that fails silently.
- `TestASkippedSlotIsNotAnEmptyScreen` — the live cadence, one inference in four samples, still
  settling on a list. The shape the first version could not have survived.
- `TestAnUnconfirmedBeliefSaysItIsStale`, `TestABeliefNobodyConfirmsIsEventuallyDropped` — the
  age of a belief, and the end of one.
- `TestAMenuGoingAwayIsNotTheSameAsNobodyLooking` — both sides of the distinction in one test.
- `TestASessionSaysWhenItsAnswerChanged` — the journal, and that it is a journal of changes
  rather than of looks.
- `TestTheSessionReportSaysWhatItIsLookingAt`, `TestAStaleBeliefSaysSoInTheReport`,
  `TestTheReportStatesTheBoundaryItActuallyHolds` (`cmd/director`) — the surface a person
  reads while they navigate, including that it never prints the entries as a count of what the
  menu has.
