---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - perception
  - fusion
  - vision
  - ocr
  - privacy
---

# UIA may establish structure; OCR may establish visible text

## Context

Sources are not interchangeable. An accessibility tree reports a control because the
application declared one — the structure is a fact the application itself asserts. An OCR
pass reports a word because pixels looked like letters, which says something real about what
is *visible* and nothing at all about what is *there*.

Treating them as equal evidence produces a specific failure: OCR finds a word, fusion
promotes it to an element, and the Director tries to click a label printed on a background
image.

## Decision

**Each source may establish only what it is capable of knowing.**

- **Accessibility (UIA)** may establish *structure*: that a control exists, its role, its
  identity, its real capabilities.
- **OCR** may establish *visible text*, and may attach that text to structure another source
  established. It may not create structure.
- **Vision** may propose structure and regions, but may not establish actionability — see
  [[ADR-004-vision-cannot-establish-actionability]].
- **Visual state** may establish that something *changed*, not what it is.

Standalone text that matched no structure stays in the observation graph. It is retained as
evidence and never promoted.

## Consequences

- Fusion is deliberately conservative, and the Director will sometimes fail to find a control
  that a person can plainly read on screen. That is the intended trade.
- Scoped OCR — reading *inside* a region another source proposed — is the productive
  pattern, because the structure comes from a source entitled to assert it.
- This is upstream of the privacy rule: a label may be kept in plaintext only when it is
  attached to a structural control role. A source that cannot establish structure therefore
  cannot produce a readable name, which is the constraint currently blocking
  [[Vision]].

## Enforced by

- **implementation** — `internal/director/perception/fusion/text.go` (conservative text
  attachment), `internal/director/perception/providers`
- **boundary test** — `TestNothingOutsidePerceptionKnowsThatOCRExists`
  (`internal/director/perception_boundary_test.go`) — a source is an implementation detail,
  so no consumer can special-case one
- **milestone record** — the *Conservative fusion* and *Standalone text stays in the
  observation graph* sections of [[director-perception]]

## Related

- [[ADR-002-fusion-owns-belief]]
- [[ADR-004-vision-cannot-establish-actionability]]
- [[Perception]], [[Fusion]], [[Vision]]
