---
type: decision
status: accepted
date: 2026-10-13
affects:
  - execution
  - passive-observation
---

# ADR-190 — the pointer arrives before the button goes down

## Context

The first live run of [[ADR-189-answering-without-taking-the-foreground]] did everything it was
built to do. Somebody established the screen, agreed to the step, permitted the press with a key
while Rocket League held the foreground, and one real left click went out through legal Marco at
a point in the middle of the PLAY button.

**The menu did not change.**

The point was right. The person at the keyboard confirmed it: Marco clicked the correct control.
What was missing was time.

`winBackend.clickAt` sends an absolute move, a button-down and a button-up in ONE `SendInput`
batch. That is deliberate and documented, and it is right for a desktop control: one batch cannot
be interleaved with anything else, and the down and up each carry their own absolute position, so
the press cannot land at a stale spot the way a separate `SetCursorPos`-then-click can race.

It is wrong for an interface that tracks the pointer. A game menu decides what is under the
cursor on its own frame tick and lights it up then. A button-down arriving in the same batch as
the move is a press on a control the interface has not yet noticed the pointer is over. Nothing
registers, and the only evidence is a click that appears to have missed.

The same package already knew half of this. `winBackend.move` carries a comment explaining that
it sends a real injected `MOUSEEVENTF_MOVE` rather than a bare `SetCursorPos`, because "many apps
— and ESPECIALLY games reading raw input — only fire their hover/highlight on an actual movement
event". The gesture was already right. Nothing was giving the interface a moment to react to it.

## Decision

An action is **move, rest, press**.

```
authorise  →  world?  →  MOVE  →  dwell  →  stopped?  →  world?  →  CLICK  →  observe
```

`DefaultDwell` is 250ms: several frames at any refresh rate a game runs at, and short enough that
the gap it opens is a quarter of a second rather than a pause somebody could walk away during.
`--dwell 0` sends the click on its own, which is right for a desktop control and is what every
earlier measurement of this subsystem was taken with.

### Two operations, not one program

A single Marco program containing the move, a sleep and the click would be one trip through the
compiler, which is tidier. It would also put the dwell **inside** the runtime, where this package
can check nothing.

Splitting them keeps the gap here, where a stop can land in it and the world can be asked again
on the other side of it. The dwell is a gap like the one a person reading a prompt opens, and it
gets exactly the same treatment — which is the rule [[ADR-188-performing-a-saved-workflow]]
established and this is the second place it applies.

### It is still ONE action

One authorisation, one step, one entry in the budget. Moving the pointer to a control and
pressing it are one act, and nobody is asked twice for it. A move that cost its own permission
would ask somebody to authorise the cursor; a move that counted against `--max-actions 1` would
make the first supervised live test send a move and no click.

What the move is not is free: it is a real effect, it goes out through `marcoexec` and the legal
Marco runtime like everything else, and a run that cannot move the pointer reports that and
presses nothing.

### What changed on the screen is its own sentence

The live run's report said the destination was not established, which was true and not useful. It
is equally true of an interface that ignored the press and of one that responded in a way nobody
recognised, and those want opposite things done about them.

`Report.Changed` says whether any observation after the action differed from the one it was
performed in. It is evidence that something happened and never evidence of what. It also answers
the hover question directly: a run that reports a visible change and no destination is a run
where the interface noticed the pointer and the click still did not take.

### And the question says what the point is the middle of

`Intent.Where` and `Intent.Target`. The live run clicked (71,479) on one pass and (79,479) on the
next, and the record could not say what either was the middle of — so "did it find the button, or
a piece of the button" had no answer in the log. A point near the left edge of a wide control and
the middle of a narrow one are the same two numbers and completely different findings.

## Consequences

- **A game menu can be clicked.** Whether Rocket League's responds is the next live run's
  question, and it is now a question with three distinguishable answers rather than one.
- **The dwell is a gap, and it is guarded.** The world is re-observed on both sides of it and a
  stop lands inside it. A run stopped there has moved the pointer and pressed nothing, and says
  so, because a cursor resting on a control nobody pressed is a state somebody should be told
  about.
- **Every existing measurement was taken at dwell 0**, and remains valid for what it measured.
- **No new executor, no new perception, no new Marco.** `KindMove` already existed and already
  lowered to `do OS's Move with at.`

## Enforced by

`internal/director/workflowrun` —
`TestThePointerIsMovedToTheControlBeforeItIsPressed`,
`TestAMoveAndItsPressAreOneAction`,
`TestNoDwellSendsTheClickOnItsOwn`,
`TestAWorldThatMovedDuringTheDwellIsNotClicked`,
`TestAStopDuringTheDwellPreventsThePress`,
`TestAScreenThatChangedAfterTheActionIsReportedSeparately`,
`TestAScreenThatDidNotChangeIsNotReportedAsAChange`,
`TestTheQuestionNamesTheRegionThePointCameFrom`,
`TestATargetWithNoRegionSaysSo`,
`TestTheRecheckIsToldWhichObservationThePlanWasMadeFrom` (twice, now).

`cmd/director` — `TestOneApprovalKeySendsOneLeftClickThroughTheMarcoCompiler` and
`TestAWorkflowStepReachesTheHostAsOneRealLeftClickThroughTheMarcoCompiler`, which assert the
host hears `OS's Move` then `OS's Click` at the same point, through the real front end.
