---
type: decision
status: accepted
date: 2026-09-07
affects:
  - passive-observation
source_paths:
  - pkg/directorapi/world.go
  - cmd/director/observewiring.go
  - internal/platform/navsource/navsource.go
---

# ADR-130 — watching is not permission to press

## Context

`Role.Clickable()` answers "may Marco treat this as a target for a click it performs itself".
`pushActionables` used it to answer a different question — "what could the person have pressed" —
and the two had silently become one predicate.

Measured live on Epic Games Launcher, whose accessibility tree exposes one window:

```
current fused World      pixel-derived  menu "Library"  87,237 201x20
human click              captured
attributed target        the enclosing window
graph edge               none
```

Nothing had rejected the pixel evidence. Every gate in the path is sensor-blind:
`pushActionables` draws from the fused World and its own comment forbids gating on the detector,
fusion defaults `Visible` to true for anything with bounds, `MaxActionables` is 256, and
`RoleMenu` is already on `NameablePlaintext` so the label "Library" was admissible. The
candidate set was simply narrower than human observation requires.

## Decision

`ElementRole.PointerAttributable()` — could a press inside an element of this role be said to
have landed ON that element — with exactly one consumer, the candidate set a human press is
resolved against.

```go
func (r ElementRole) PointerAttributable() bool {
	if r.Clickable() || roleInteractive(r) {
		return true
	}
	return r == RoleMenu
}
```

`RoleMenu.Clickable()` stays false. The menu arm is not an application-shaped exception: a menu
is already on `NameablePlaintext`, which is this project's existing reviewed judgement that a
menu is a control whose name is safe to keep. A role Marco may NAME as a control is a role a
person may be said to have pressed.

Two things this did not need, found by tracing before changing:

- **A ranking rule.** `actionables.under` already preferred the smallest containing element. "Most
  specific beats the enclosing window" was never missing; the window won because nothing else was
  offered.
- **A label change.** `RoleMenu` was already admissible to `AdmittedTargetLabel`.

## Consequences

- A human click on a perceived control that Marco would not press unprompted can be attributed to
  that control.
- Nothing becomes executable. Authority is unchanged, `Actionable()` is unchanged, and autonomous
  action still resolves its own targets.
- Inert content — text, headings, images — still explains nothing. Reading a word never creates a
  thing somebody pressed.
- Sensor-agnostic by construction: the predicate is a method on a role and can reach no
  provenance, so an accessibility menu and a pixel-derived menu are one case.

## And the diagnostic that made it provable

`InputStats.PointerAttributions` records each recent press: its window-relative position, every
offered control that contained it smallest-first, which won, and why. `containing` is now the one
hit-test rule and `under` takes its head, so a report shows the decision that was made rather
than a second implementation of it.

The tallies alone were not enough and had already misled once — `0 landed on nothing` was
literally true while the thing the person pressed had never been a candidate. Candidates carry a
SIZE, which is why one beat another, and never a position; absolute desktop geometry does not
cross this boundary.

## Enforced by

- `TestAHumanClickOnAPerceivedMenuIsAttributedToIt` — the menu wins over the enclosing window,
  and the test asserts `RoleMenu.Clickable()` is false first, so a fix by widening execution
  fails it.
- `TestInertContentCannotExplainAClick` — text, heading and image explain nothing.
- `TestPointerAttributionIsWiderThanClickabilityAndGrantsNothing` — everything clickable stays
  attributable; the predicate takes no argument, so a sensor-specific rule cannot be added
  without failing it.
- `TestOnePressIsExplainedByItsOwnRecord` — the record names the winner, keeps the losers, and
  carries no `X`/`Y`.
