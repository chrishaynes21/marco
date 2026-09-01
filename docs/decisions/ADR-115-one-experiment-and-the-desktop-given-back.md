---
type: decision
status: accepted
date: 2026-08-30
affects:
  - passive-observation
  - performing
  - editing
source_paths:
  - cmd/director/experiment.go
  - cmd/director/perform.go
  - internal/director/service/perform.go
  - cmd/marco/watchui.go
  - cmd/marco/edit.go
---

# ADR-115 — one experiment, and the desktop given back

## Context

By the third dogfood Marco was observing well and discovering routes, and a person watching it
still could not answer any of:

```
What is Marco focused on?      What starting Place does it need?
What is it about to try?       Is it waiting for me?
Why is it trying that?         Did it work?
                               Did it give my computer back?
```

Every observation, discovery, attempt and state change competed for the same space. There was no
thought to follow — only neurons firing. And the behavioural half was worse than the presentation:
when Marco wanted to try something it expected the person to arrange the stage for it, and
afterwards left them standing wherever the experiment had ended.

## Decision

### One experiment, stated as a claim

Marco holds at most ONE proposal at a time: a promoted candidate edge it has never walked itself.
Both halves are canonical — `Promoted` is set by the ambient ledger when the policy admitted the
evidence, and "never walked" is the same verified map `plannableEdges` builds, so what Marco offers
to test and what the planner prefers are one answer. A contradicted edge is excluded for the same
reason `ambient.Judge` refuses it.

It is rendered as a hypothesis with three parts, because *"trying Mouse"* is ambiguous between a
goal, a target, a Place, an edge and a route:

```
READY TO TEST
  Bluetooth & devices — open Mouse → Mouse
  I watched you open Mouse from Bluetooth & devices 4 times. I have not tried it myself.
  [ Test what I learned ]
```

**The reason is evidence, never narrative.** Every clause is a field of the record: how many times
it was traversed, and the fact that Marco has not done it. A friendlier sentence with nothing
behind it is the one that gets believed, on a surface whose whole purpose is that somebody can
decide whether to let Marco touch their computer.

### An experiment is a projection, not a second executor

`Runtime.TestEdge` runs on `perform.go`'s machinery, unchanged: `bringForward`, `freshLook`,
`observe.PlanToGoal`, `performPlan`, `confirmArrival`, the command registry, the authority the
walker takes per step and the actuation lease it holds. There is no experiment planner, no
experiment navigation and no learning click.

```
capture where the person is      →  this ADR
bring the application forward    →  bringForward
look, freshly                    →  freshLook
get to the source                →  PlanToGoal + performPlan
require the source               →  the positioning walk's own verification
do the one thing being tested    →  performPlan, one edge
check where that landed          →  confirmArrival
give the desktop back            →  this ADR
```

**The source is required, not assumed.** An experiment is a claim about one edge: from HERE, doing
THIS, you arrive THERE. Running the action from anywhere else tests nothing and presses a control
on a screen nobody chose — so a failure to reach the source ends the attempt before any
experimental input, and says why.

**It does not explore.** The positioning route is the canonical planner over the canonical graph
with the canonical eligibility. If nothing connects where the person is to where the experiment
starts, Marco says so:

```
I need to be at Bluetooth & devices to try that, and I don't know a way there from Downloads.
```

That is a better product outcome than a silent nothing, and dramatically better than pressing
things to find out.

### It enters the same command registry

An experiment drives real input, so every reason a performance takes the mutating slot applies
unchanged: visible to `director status`, refusing a concurrent command, and — the reason that
matters — **stoppable**. A learning path a person cannot stop is the one thing this lifecycle
exists to prevent, and "it is only checking something" makes it worse rather than better. The Here
panel gained a Stop that reaches the same `CANCEL_ACTIVE` the spoken "stop", the leader key and
`director stop` reach.

### The person is not Marco's stagehand

Before anything moves, the foreground application and window title are captured — attempt-scoped,
never durable, and never semantic knowledge: a window is how something is ADDRESSED, never how a
Place is identified ([[ADR-071-a-window-is-not-a-place]]).

Afterwards, on **every** path out including the ones where Marco gave up, the original window is
brought back and **checked**. A restore that reported success because the call returned nil leaves
somebody standing in Marco's experiment believing they were put back — the same defect
`confirmArrival` exists to prevent, one layer down, and worse here because nobody asked to be
moved.

It restores by TITLE. `Activate` matches on the executable and Windows hosts unrelated
applications in one process, so restoring by name could raise a window the person never used and
report success doing it.

Restoration is **bringing one window back and verifying it**, and deliberately nothing else: no
closing, no undoing, no blind Back, no synthesised inverse actions, no reconstructing where inside
an application somebody had got to. Every one of those is a change to a person's work made on
Marco's initiative.

**Restoration belongs to an experiment, not to a performance.** Somebody who asked to be taken to
Mouse settings asked to be there; putting them back would be undoing what they wanted.

### Testing and going there are two acts

```
TEST WHAT I LEARNED          GO THERE
proves a connection          accomplishes what was asked
needs a specific SOURCE      starts from wherever you are
gives the desktop back       leaves you where you asked to be
```

A surface labelling both *Try it* hides the difference between doing somebody a favour and
borrowing their computer. They are separate requests, separate buttons and separate words.

### Watching and learning still never acts

The proposal is Marco's; the attempt is the person's. Observation permission is not actuation
permission and learning permission is not actuation permission — a Watch & Learn that started
testing on its own would be an autonomous clicking bot somebody switched on believing they were
switching on a notepad. `Experiment` is a read that may be polled forever and moves nothing.

### And the surface shows one thought

```
NOW              what Marco sees
READY TO TEST    the one experiment, its reason, and the way to run it
LAST RESULT      what Marco wrote down
▸ EVERYTHING MARCO HAS NOTICED    (repeated evidence, demoted behind a disclosure)
```

The headline is the most important thing that happened rather than the most recent: walking a
familiar route is not a discovery, and a line reading *saw way again* over the top of *learned
destination* trains a person to stop reading it. The demoted detail is still there — "why has it
not learned that yet" is a real question with a real answer — it simply does not compete.

## Consequences

- An experiment can move somebody's desktop before they see any result: it may walk several
  screens to reach the source. `Positioned` says so on the report, and Stop is on the panel.
- **Foregrounding is `winctx.Activate`, a Director platform call, not a Marco act.** That is
  pre-existing and shared with every performance since Phase 0; making it a legal-Marco act is a
  change to the execution substrate and is deliberately NOT part of this correction. Recorded here
  rather than done quietly.
- The attempt's steps are rendered when it finishes rather than streamed while it runs. The panel
  shows what it is doing and what it did; a live step-by-step needs a status read this correction
  does not add.
- `winctx` calls in the Director now go through package-var seams so the suite can exercise
  foregrounding and restoration without activating windows on whoever runs it.

## Enforced by

- `cmd/director` `TestTheDirectorSaysWhatItWouldLikeToTry` — From, Action, To, and never an id
- `cmd/director` `TestTheReasonToTestComesFromEvidence`
- `cmd/director` `TestMarcoDoesNotOfferToTestWhatItHasAlreadyProved` — all three exclusions
- `cmd/director` `TestAnExperimentWillNotActWithoutItsSource`,
  `TestAnExperimentWithNoWayToItsSourceSaysSo`
- `cmd/director` `TestAnExperimentWithNoRouteToItsSourceTriesNothing`,
  `TestPositioningWillNotTakeAnEdgeThePlannerRefuses`
- `cmd/director` `TestAnExperimentGivesTheDesktopBack`,
  `TestRestorationIsCheckedRatherThanAssumed`, `TestRestorationAddressesTheExactWindow`,
  `TestRestorationDoesNothingWhenTheDesktopNeverLeft`, `TestRestorationDoesNotGuessAWindow`
- `cmd/director` `TestAStoppedExperimentDoesNotFightForFocus`,
  `TestWatchAndLearnDoesNotActOnItsOwn`
- `internal/director/service` `TestAnExperimentEntersTheCommandRegistry`
- `cmd/marco` `TestTheHereViewAsksWhatMarcoWouldLikeToTry`,
  `TestTestingAsksForTheConnectionItWasOffered`
- `cmd/marco` `TestTheSurfaceThatCanStartAnAttemptCanStopIt`,
  `TestTheExperimentIsStatedAsAHypothesis`
- `cmd/marco` `TestTestingAndGoingThereAreDifferentButtons`,
  `TestTheHeadlineIsNewKnowledgeRatherThanTheLatestEvent`

## Related

- [[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]]
- [[ADR-113-learning-is-inside-watching]]
- [[ADR-112-the-loop-belongs-where-a-person-is-already-looking]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[Experiment-022-the-first-dogfood]]
- [[ADR-116-watching-follows-the-window-not-the-executable]]
