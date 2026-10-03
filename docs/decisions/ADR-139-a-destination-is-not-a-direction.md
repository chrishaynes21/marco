---
type: decision
status: accepted
date: 2026-10-02
affects:
  - semantic-actions
  - safety
source_paths:
  - internal/director/uiact/uiact.go
  - internal/director/uiact/ladder.go
---

# ADR-139 — a destination is not a direction

## Context

Six verbs name a state to end up in: `check`, `uncheck`, `expand`, `collapse`, `select`,
`deselect`. Some of the mechanisms that perform them SET that state; others FLIP whatever state
the control is in. The difference is a property of the platform call, not a preference:

```
ExpandCollapse(hwnd, id, bool expand)      takes the direction as an argument  → directional
SelectionItem.Select / RemoveFromSelection                                     → directional
TogglePattern.Toggle()                     takes no argument at all            → a flip
invoke / click on a checkbox or arrow                                          → a flip
```

**A flip only reaches a named destination from a known starting point.** So performing one of
these through a flip, with the control's current state unreadable, can move the world the opposite
way to the request.

The ladder said the opposite, out loud:

```go
case directorapi.SemanticCheck, directorapi.SemanticUncheck:
	// Reached only when the control is NOT already in the requested state — see
	// alreadySatisfied — so flipping it is exactly right. When the state could not
	// be read at all, flipping is still the best available answer and the
	// verification is what catches a wrong guess.
```

Both claims are false. It is *also* reached when the state is unknown, because `alreadySatisfied`
asks `p != nil && *p == want` — which answers "no" to "already in the requested state?" when the
real answer is "I cannot tell". And verification does catch it, after the world has moved the
wrong way: the measured run reported `unverified` and left the box checked.

[[ADR-006-unknown-is-not-false]] had already decided this, and said why in the same breath: "the
failure is always in the unsafe direction, because Unknown arises exactly when something has gone
wrong."

The missing state was itself a defect, fixed in
[[ADR-138-what-the-control-was-like-travels-with-the-choice]]. This ADR is about what to do when
it is genuinely missing, which it still can be.

## Decision

**A verb that names a destination may not be performed by a flipping mechanism when the current
state is unknown.** The refusal names the control and the state it could not read.

## Why not every destination verb

The first version of this guard covered all six, and two existing tests caught what that cost.

`Expanded` is read from `ExpandCollapsePattern`'s own state property. A control without that
pattern therefore has no state to report — always, structurally, not as a timing artefact. For
expand and collapse, "the state is unknown" and "there is no directional mechanism" are the SAME
condition, so refusing on it would delete the invoke and click rungs the ladder provides on
purpose for trees that expose no expansion support.

`Checked` is different in both halves. The state and the mechanism arrive through separate
channels — the reported pattern list and the element's states — so they can disagree, and the
measured defect is exactly that disagreement: `toggle` reported while ToggleState had not arrived.
And the misdirection is neither visible nor reversible in the same way: these two verbs address
mute, autosave, notifications, "share my location" and every other setting somebody asks a
computer agent to turn off.

So the guard goes where the harm is silent and the refusal is rare, and not where the refusal
would be permanent and the harm obvious. `blindFlipIsUnsafe` is that line, and it is one function
with the argument written next to it.

`Selected` never arrives nil — the resolver records a real `false`, deliberately — so `select` and
`deselect` never reach the question.

## Consequences

- An unreadable checkbox is refused rather than guessed at. With ADR-138 wired, the state is
  normally present, so this is rare; before it, it was permanent and silent.
- The residual hazard on expand/collapse is accepted and recorded: invoking a node that is already
  expanded collapses it. `TestExpandDeliberatelyKeepsItsFlippingFallback` holds that as a decision
  rather than leaving it as an accident.
- An existing test, `TestAnUnknownStateIsNotTreatedAsSatisfied`, had its assertion moved and its
  claim kept. It required the resolve to SUCCEED on an unreadable control, on the reasoning that
  such a control "must be ACTED on". There were two options in view then; the one it rejected was
  the right one to reject, but the one it chose turned out to be worse than a third:

  ```
  satisfied   silently does nothing and claims success   — what it forbids, correctly
  flip it     may do the OPPOSITE of what was asked      — what it used to require
  refuse      says it cannot tell, and does nothing      — what happens now
  ```

## Enforced by

- `TestADestinationVerbIsRefusedWhenTheStateCannotBeRead` — check and uncheck, refused, naming the
  state.
- `TestADestinationVerbWithAKnownStateStillFlips` — a known state still flips.
- `TestAlreadyInTheRequestedStateIsStillSuccess` — idempotence kept.
- `TestToggleWithAnUnknownStateIsStillAllowed` — `toggle` asks for a flip, so it is unaffected.
- `TestADirectionalMechanismDoesNotNeedTheState` — expand and collapse through the pattern.
- `TestExpandDeliberatelyKeepsItsFlippingFallback` — the accepted residual hazard.
- `TestAnUnknownStateIsNotTreatedAsSatisfied` — the older claim, still held.
- Mutations both ways: letting a blind flip through kills the first; widening the guard to every
  destination verb kills the two pre-existing fallback tests.
