---
type: decision
status: accepted
date: 2026-10-13
affects:
  - passive-observation
  - execution
  - navigation
---

# ADR-189 — answering without taking the foreground

## Context

[[ADR-188-performing-a-saved-workflow]] built a coordinator that performs a saved workflow one
authorised action at a time, and the first supervised run of it against Rocket League found a
defect that no offline test could have found, because it is not in the code.

**A person cannot answer a terminal prompt without destroying the thing the answer permits.**

A click carries no address. It lands wherever the pointer is, in whatever window is in front at
that instant, which is why the coordinator re-observes the world immediately before every press
and refuses when the target does not lead the desktop. Typing an answer makes the *terminal* the
front window. So the sequence is:

1. Rocket League is in front. The control is found. The question is printed.
2. The person switches to the terminal to answer it — and Rocket League is no longer in front.
3. The press is refused: *"rocketleague does not lead the desktop, and a click lands in whatever
   is in front."*

The guard is right. The answer channel is wrong. Reading a prompt does not take the foreground;
only typing does.

A second observation from the same run: the first grounding question is also unanswerable for
the same reason, and it arrives before any action is contemplated.

## Decision

### Three keys, observed by the hook that already exists

`navsource.Consent` admits exactly three virtual-key codes — F7 permits, F8 refuses, F9 stops —
and turns them into answers. The keys were checked against the game's own bindings before being
chosen: Rocket League's `TAInput.ini` binds `F1` to the console and `F10` to ToggleStatGraphs and
nothing else in the F range; the overlay's stop key is F12.

It is the same hook, not a new one. Windows delivers a low-level hook callback only while the
installing thread is in a message wait, `pump_test.go` already walks the tree for hook sites, and
a fourth install would be a fourth chance to get the pump wrong and silently unhook the other
three. See the hook-pump invariant in `CLAUDE.md`.

**A consent key is an answer, never an observation.** Everything else this package produces is a
record of a person operating an application: it feeds correlation and is bounded to keys whose
navigation meaning is conventional. A reply addressed to Marco is a different kind of thing. So a
consent key is recognised before classification, never becomes a `NavIntent`, never reaches a
subscription or a recording, and is counted under its own `observe.IgnoreConsent` — because the
total has to add up and [[ADR-184-two-speeds-of-looking]]'s sibling note on
`IgnoreKeyRelease` is the record of what happens to a producer whose total does not.

Three properties come free from the hook as it already is, and all three are load-bearing:

- **Injected events are dropped at the callback.** Marco cannot authorise Marco. This is not a
  check added for this feature; it is the existing rule that the Director must not correlate its
  own actions with the changes they cause.
- **The hook never suppresses.** The application receives the keypress exactly as it would have.
- **Auto-repeat is suppressed by the held set.** A person leaning on F7 gives one answer, not
  forty.

### A desk, because a key is only an answer beside its question

`cmd/director/consentdesk.go`. One goroutine owns the stream, which is what makes "before" and
"after" well defined at all:

- a key pressed **before** a question is drained when the question opens, and counted. Somebody
  leaning on F7 while a run starts must not pre-authorise the first action it decides to ask
  about: an authorisation is permission for a press that was *described* to them.
- one key answers **at most one** question. There is no second reader to hand it to.
- a key pressed with **nothing pending** is counted, except stop, which is always obeyed — a
  person stopping a run between two actions is exactly the case a desk that only listened during
  prompts would miss.
- a question nobody answers **expires**. An open prompt holds the capture loop.

The desk grants nothing and presses nothing. It turns a keypress into an answer.

### Agreeing and permitting stay two keypresses

The distinction [[ADR-188-performing-a-saved-workflow]] drew survives the change of channel:
`agree` and `Authorise` are separate questions, asked at separate moments, each needing its own
press. A build that took one F7 for both would let somebody agree that a step belongs in the
workflow and be taken to have permitted it to happen on the screen in front of them.

### `--from`, because the first screen cannot be typed either

A person naming the screen they will be standing on, before switching to the application, is
still a person establishing correspondence as [[ADR-186-a-person-can-say-where-they-are]]
requires — it is simply done in the only moment a keyboard is available to them. What keeps it
honest is that it is **spent once**, on the first observation it can be spent in, and stamped
with that observation like any other confirmation. Every screen after it is inferred from the
workflow's own demonstrated transitions, or nobody has said what it is and the run stops.

Where nobody can be asked, the ranking is still **printed**. Somebody who cannot answer can still
read, and a refusal with no ranking would make an unanswerable question look like a perception
failure.

### An authorisation is also about a window

`Authorisation.Window`, and a fifth refusal in `Spendable`. Carried separately from the window
generation because they fail differently and a person needs to be told which happened: a
generation moves when the *same* window is replaced; this changes when the press would go to a
*different* window — a second copy, a relaunch, a selector that matched something else.

`worldMoved` gains a matching refusal for the screen standing now, which catches a re-settle
inside one observation: the epoch is unchanged and what is standing is a different screen.

### `--max-actions`, enforced in the coordinator

A limit a caller imposes is a limit on what the caller does with a report. By the time
`OutcomePressed` has been returned a program has been lowered, compiled, run and has reached
whatever is at the bottom. So the budget lives in `workflowrun.Runner`, is asked before the
question — nobody is asked for permission that could not be used — and again beside the counter
it limits.

A spent budget is a **refusal**, not a completion. The run stops safely and says how far it got,
because stopping after one deliberate action is the ordinary shape of a supervised test and must
not read as a workflow that finished.

## What it found

**Four of the saved workflow's nine controls do not locate anything.** `control_3`, `_4` and `_5`
have readings covering the whole window (area 0.957) with no containers recorded at all, and
`control_9` is the bottom half at full width. Every one of them re-acquires perfectly against a
live look, and every one offers the same point: `(960, 517)` on a 1920x1080 window, which is the
middle of the screen.

This is not a fallback and not a substitution. The saved target *is* a band, and re-acquisition
is doing exactly what it was asked. The defect is that matching is about rectangles while
pressing is about a point, and nothing in between said so.

`observe.Region.Locates` says it now: a region running from one edge of the window to the
opposite edge does not locate a control, because its centre on that axis is the window's centre —
arithmetic about the window, carrying no information about where anything inside it sits. It is
not a size rule; a wide button is still a button, and the test says so.

The tempting repair is to pick a better point inside the band. **There is no better point**: the
evidence that would choose one is the evidence that is missing, and inventing one would present a
guess with the same confidence as a position that was actually found. So `StepControlUnlocated`
is its own stage, distinct from `StepControlUnresolved`, because the two tell a person to do
opposite things — look again, or re-teach the workflow.

## Consequences

- **A supervised press is possible on one machine**, with the application keeping the foreground
  for the whole of the authorisation and the action.
- **The foreground guard was not weakened**, moved, or made conditional. It refuses exactly as
  before; what changed is that answering no longer trips it.
- **Five steps of the saved Rocket League workflow are now refused** rather than offering the
  middle of the screen. That is a loss of reach and a gain in honesty, and the workflow needs
  re-teaching to recover it.
- **Marco cannot authorise Marco**, by a property of the hook rather than by a check.
- **No live input has been sent.** Everything here is demonstrated through `recordhost`.

## Enforced by

`cmd/director` —
`TestOneApprovalKeySendsOneLeftClickThroughTheMarcoCompiler` and
`TestTheSameAssemblySendsNothingWhenNobodyPressesTheKey`, which drive the synthetic key source
through the desk, the granter, the coordinator, `marcoexec` and the real Marco front end to
`recordhost`;
`TestTheRefusalKeySendsNothingThroughTheRealPath`,
`TestASpentBudgetRefusesAPlannableStepThroughTheRealPath`,
`TestTheApprovalKeyGrantsForTheStepWindowAndObservationItWasAskedAbout`,
`TestTheRefusalKeyGrantsNothing`,
`TestTheStopKeyIsObeyedWithNoQuestionOpen`,
`TestTheStopKeyEndsAnOpenQuestionWithoutGranting`,
`TestAKeyPressedBeforeTheQuestionDoesNotAnswerIt`,
`TestOneKeyAnswersOnlyOneQuestion`,
`TestAHeldApprovalKeyIsOneAnswer`,
`TestAnUnansweredQuestionExpires`,
`TestACancelledRunDoesNotWaitForAnAnswer`,
`TestAgreeingToAStepIsNotPermittingIt`,
`TestConcurrentQuestionsAndKeysNeverGrantMoreThanWereApproved`,
`TestWorldMovedRefusesEachWayTheWorldCanHaveMoved`.

`internal/director/workflowrun` —
`TestABudgetOfOneSendsOneActionAndRefusesTheNext`,
`TestARunWithNoBudgetSendsAsManyActionsAsItReaches`,
`TestTheReportCarriesTheBudgetAndWhatWasSent`,
`TestAnAuthorisationGivenAboutAnotherWindowIsRefused`,
`TestAnAuthorisationNamingNoWindowIsRefused`,
`TestTheQuestionNamesTheWindowThePressWouldGoTo`,
`TestAControlReacquiredAsABandAcrossTheWindowIsNotPressed`.

`internal/director/observe` —
`TestARegionLocatesSomethingOnlyWhenItIsBoundedAwayFromTheWindow`,
`TestTheRefusalExplainsWhyTheMiddleIsNotAPosition`.

`internal/director/workflowstore` —
`TestAStepWhoseControlIsABandAcrossTheWindowIsNotPlannable`.
