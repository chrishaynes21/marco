---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - semantic-memory
  - passive-observation
source_paths:
  - internal/director/observe/recall.go
---

# ADR-062 — a scroll bar is not a screen

## Context

`CompareStructure` opens with a role-SET check, and its comment states the reasoning: *"A
screen that gained a whole role — a progress bar, a text field — is a different screen, not a
jittered one."* That is right, and it was written against the structural detector's small
class vocabulary.

Accessibility supplies a far larger one. Measured live on 2026-08-17: five Learn attempts
against Windows Settings minted **five durable subjects for the same pages**. Two recordings
of the Home page agreed on their terms and on every shared role count — button 15 vs 16,
inside `RoleCountTolerance` — and differed by exactly one thing:

```
run 2:  … pane=3              text=32 …   terms=[back+settings]  envelope=none
run 5:  … pane=3 scroll_bar=1 text=32 …   terms=[back+settings]  envelope=none
```

A whole role had arrived, so the verdict was `different`, so neither endpoint of the person's
demonstration resolved, so no durable edge formed, and Learn could not complete on mainstream
software however well every layer above it worked.

Windows 11 auto-hides scroll bars and shows them while the pointer is over the region. So the
role's presence tracks **where the person's hand is** — the act of demonstrating is itself
what changed the screen's identity.

## The decision

A closed set of CHROME roles is excluded from identity comparison. It currently contains one
member: `scroll_bar`.

The test a role must fail to be listed: *does its arrival tell a person they are somewhere
else?* A progress bar arriving says the screen started loading. A text field arriving says it
now offers somewhere to type. A scroll bar arriving says the content is a few pixels taller
than the space for it, or that the mouse moved.

Chrome roles are still **recorded** — a signature says what was seen — and merely not
**compared**.

## Consequences, including the risk

This widens identity, and over-merging is the failure this design fears most: two screens with
identical structure that differ only in what their text says. The widening is confined so that
it cannot reach that case. Terms are compared exactly as before and still separate two
otherwise-identical screens; the envelope rule is untouched; counts are untouched; and a screen
differing by any role a person could act on is still a different screen. The entire change is
that two screens which are otherwise identical are no longer told apart by a scroll bar.

It is deliberately the narrowest fix the evidence supports rather than a general tolerance. The
related open debt — `StructureSignature.Envelope` stranding a subject — is NOT addressed here
and remains open; this ADR does not license loosening identity again without the same kind of
measurement.

## Enforced by

- `TestAScrollBarDoesNotMakeItADifferentScreen` — the measured case, both directions
- `TestARealRoleArrivingStillMakesItADifferentScreen` — a progress bar, text field, slider,
  checkbox or tab arriving still separates two screens
- `TestChromeToleranceDoesNotWeakenTheTermDiscriminator` — text still discriminates
  (all `internal/director/observe/recall_test.go`)

Mutation-gated in both directions: emptying the set kills the first test, widening it to the
actionable roles kills the second.

## Superseded in part

The role-name rule here is generalised by
[[ADR-071-a-window-is-not-a-place]], which classifies a window.s machinery by OWNERSHIP: a scroll
bar and everything inside it, a title bar and everything inside it. The role list survives because
it still guards signatures stored before that, and because a scroll bar that does have area is
still not a place.

## Related

[[ADR-016-cross-session-identity-is-structural-and-conservative]] ·
[[ADR-017-structure-earns-a-name-text-never-earns-structure]] · [[Semantic-Memory]] ·
[[Experiment-011-two-level-identity-against-real-software]]
