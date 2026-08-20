---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - demonstrations
  - learned-plays
source_paths:
  - internal/director/observe/demonstration.go
---

# ADR-053 — a search box on the last page is not a step

*Recorded retrospectively: the decision was made and implemented on 2026-08-13 and the code
has referenced this number since; the note was never written. Content reconstructed from
`textEntryBlocks` and its tests.*

## Context

Marco does not observe what anybody types, so a route that DEPENDS on typed text cannot be
reproduced. The conservative reading was "any leg that arrived anywhere offering somewhere
to type blocks learning" — and it is too broad by one case. A live Settings run pressed
down, down, enter, arrived, and was refused because the page it LANDED on has a search box.
Windows Settings, Explorer, Chrome and VS Code put one on nearly every page.

## Decision

Text-entry blocks a leg only at an INTERMEDIATE checkpoint, where unobserved typing may be
how the person reached the next screen. At the DESTINATION nothing comes after: whether the
final screen offers a search box says nothing about whether text entry was required to GET
there. `textEntryBlocks` exempts the destination and nothing else, and both the armed
capture and the one-shot discovery path go through the one function.

## Consequences

Mainstream software becomes learnable. The rule is not "typing is irrelevant" — an
intermediate editable screen still blocks, and text-entry learning remains a separate
privilege with its own consent.

## Enforced by

- The `textEntryBlocks` cases in `internal/director/observe/observe_test.go` /
  `demonstrationwiring_test.go` (destination exempt, intermediate blocking).

## Related

[[ADR-020-watch-me-is-permission-to-observe-not-to-act]] · [[Demonstrations]]
