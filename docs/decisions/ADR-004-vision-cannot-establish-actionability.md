---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - vision
  - fusion
  - policy
  - safety
---

# Vision cannot establish actionability

## Context

A detector returns a box and a class. "Button, 0.91." It is very easy to read that as "there
is a button here and you may click it", and every part of that reading beyond the geometry is
invented.

The model was trained to recognise the *appearance* of controls. A screenshot of a button, a
disabled button, a button in a background window, and a picture of a button in a tutorial
video all look like buttons. Confidence measures resemblance to training data, not whether
anything will happen when clicked.

The incident that made this concrete: a one-class model whose single class was mislabelled
`button` announced 56 desktop icons as controls.

## Decision

**Vision may propose structure and regions. It may never establish actionability.**

Actionability is derived from a control's *real reported capabilities* — the accessibility
patterns it exposes — and from nothing else. A vision detection that no other source
corroborates is geometry with a class attached: usable to scope an OCR read, not usable as a
thing to act on.

## Consequences

- Vision is genuinely useful only in combination. Alone it produces boxes nothing may be done
  to, which is exactly what the passive-observation baseline measured.
- The capability ladder in [[Semantic-Actions]] asks the control what it can do rather than
  inferring from class, and refuses rather than guessing when the answer is nothing.
- A better detector does not relax this rule. It makes vision better at proposing, which is
  still not permission to act.
- The class vocabulary matters enormously, because a class that maps to no nameable role can
  never contribute a label either — see [[Experiment-001-vision-backend-comparison]].

## Enforced by

- **implementation** — `pkg/directorapi/actionability.go` (`Element.Actions()` derives from
  reported capabilities), `internal/director/perception/providers/vision/provider.go`
- **regression test** — `internal/director/perception/providers/vision/vision_test.go`
  asserts structural classes map to roles and non-structural ones map to `RoleUnknown`
- **end-to-end test** — `cmd/director/vision_e2e_test.go` runs detect → place → filter →
  fuse → world against a fake detector
- **milestone record** — the *Vision never fabricates actionability* section of
  [[director-vision]]

## Related

- [[ADR-003-evidence-authority-by-source]]
- [[Vision]], [[Semantic-Actions]], [[Fusion]]
