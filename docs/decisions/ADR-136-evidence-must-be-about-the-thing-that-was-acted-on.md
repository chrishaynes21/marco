---
type: decision
status: accepted
date: 2026-10-02
affects:
  - execution
  - safety
  - semantic-actions
source_paths:
  - internal/director/verify/verify.go
  - internal/director/verify/semantic.go
---

# ADR-136 — evidence must be about the thing that was acted on

## Context

Found by dogfooding ordinary requests against the acceptance fixture with its oracle printed
beside every verdict. `click Details` came back:

```
COMPLETED: the window title changed from "Marco Fixture" to "marco - Visual Studio Code"
[result] performed
```

The oracle said `details_presses: 0` and `second_view: false`. The button was never pressed. The
editor had taken the foreground mid-request, and Marco verified the action on the strength of it.

Three pieces of evidence were being computed over the **desktop** while being read as facts about
**one control's window**:

| evidence | what it compared | weight | alone sufficient? |
|---|---|---|---|
| `window_title_changed` (click) | the FOREGROUND's title, before vs after | 0.75 | yes |
| `window_appeared` | the COUNT of windows on the desktop | 0.80 | yes |
| `window_title_changed` (navigation) | the FOREGROUND's title, before vs after | 0.85 | yes |

`MinConfidence` is 0.5, so each of those on its own verified whatever Marco had just done. The
helper at the centre of it read:

```go
func windowTitle(w directorapi.WorldState) string {
	if win, ok := w.FocusedWindow(); ok {
		return win.Title
	}
	if len(w.Windows) > 0 {
		return w.Windows[0].Title   // a positional guess
	}
	return ""
}
```

Comparing that before and after asks *"is what is in front now called something different from
what was in front then"*. Two titles differ either because a window renamed itself — navigation, a
document opening, the case the evidence was written for — or because an entirely different program
came to the front. The code could not tell those apart, and the second is not rare: on a real
desktop it is a notification, a chat window, an indexer opening a progress dialog, a build
finishing, or the person alt-tabbing during the settle delay.

The `Windows[0]` fallback made it worse by hiding the absence of an answer. It is right often
enough to look like it works, which is why the live message named a program Marco had never been
asked to touch.

## Decision

**Evidence about an action must be scoped to what the action was aimed at.**

- `targetWindowTitle(world, target)` reads the title of `target.WindowID` specifically. The same
  window, renamed. Returns nothing when the target names no window or that window is absent from
  the snapshot — **no evidence is the right answer there**, and falling back to the foreground is
  precisely the defect.
- `appearedInTargetsApplication(before, after, target)` requires a window that is new AND belongs
  to the application that was acted on. A dialog the clicked control opened is the strongest
  ordinary evidence a click worked; a background program opening one of its own is not evidence
  about somebody else's click.
- Navigation (`back`, `forward`, `refresh`) names no control — it addresses "the focused context"
  — so its subject is **the window that held focus when the action was sent**, looked up again by
  the same id afterwards.
- `windowTitle` is deleted. Its whole semantics was the defect, and leaving it in place is a trap
  for the next person reaching for "the window title".
- `focusedWindowID` replaces its fallback chain with two real sources — the window system's active
  window, and the per-window `Focused` flag — and **no positional guess**.

## Consequences

- Marco will sometimes have less evidence and report `unverified` where it previously said
  `performed`. That is the point: the confidence it was claiming was not about the action.
- The capabilities this could have cost are each held by a test: a target window renaming itself
  still verifies, and a dialog opening in the target's application still verifies.
- It composes with [[ADR-135-a-delivered-action-is-not-a-failed-one]]: an action with no evidence
  about its own target now says `unverified` rather than `performed`, which is the honest word and
  the one that does not invite a retry.

## Related

This is the third defect in one family, all found the same way — by putting an independent oracle
next to Marco's verdict and reading both.

```
focus landing on a clicked control        delivery mistaken for effect     (WeightDelivered)
a control's own word about its selection  the actor as sole witness        (ADR-135)
another program's window                  the desktop mistaken for the target   (here)
```

The shape repeats: **a signal that correlates with success was treated as proof of it.** Each
time, the fix is to ask what the signal is actually about.

## Enforced by

- `TestAForeignWindowTakingFocusDoesNotVerifyAnything` — reproduces the live message; was 0.75 and
  verified.
- `TestAForeignWindowAppearingDoesNotVerifyAnything` — the same defect through the window count;
  was 0.80 and verified.
- `TestNavigationIsNotVerifiedByAnotherWindowArriving` — was 0.85 and verified.
- `TestTheTargetWindowRenamingItselfStillVerifies` — the capability, kept.
- `TestAWindowAppearingInTheTargetsApplicationStillVerifies` — the capability, kept.
- `TestNavigationIsStillVerifiedByTheViewChanging` — the capability, kept.

Each scope was mutated back to its unscoped form, including the `Windows[0]` fallback, and each
mutation dies on its own test.
