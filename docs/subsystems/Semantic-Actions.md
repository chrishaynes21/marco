---
type: subsystem
status: active
owners:
  - director
depends_on:
  - fusion
  - marco-boundary
used_by:
  - programs
  - goals
  - collections
updated: 2026-08-06
source_paths:
  - internal/director/uiact
  - pkg/directorapi/semantic.go
  - internal/uiamod/accessibility.marco
  - plugins/uia
---

# Semantic Actions

The action vocabulary: **33 verbs**, from six originally — expand, collapse, toggle, check,
select, choose, invoke, open, close, dismiss, submit, confirm, cancel, refresh,
back/forward, next/previous, undo/redo, copy/cut/paste, scroll_here, show_context_menu,
maximize/minimize/restore, pin/unpin.

## The central idea

**The planner emits the verb and stops.** It does not decide how to expand a tree node. The
`uiact.Ladder` picks the implementation at execution time from the control's *real*
capabilities — accessibility pattern → generic invoke → click → refuse — recording every
stronger rung it rejected and why.

This is evidence-driven rather than a guess because the C# bridge reports each control's
**patterns** and its expanded/checked state. Without that the ladder would be inferring from
class, which is the thing [[ADR-004-vision-cannot-establish-actionability]] forbids.

## Responsibilities

- Define the verb vocabulary in `pkg/directorapi/semantic.go`.
- Choose a lowering at execution time from real capabilities.
- Refuse rather than fall through to a click when nothing is appropriate.
- Verify **per verb**: an expand is proved by the node reporting itself expanded or its
  children appearing — never by "the screen changed".

## What it may not do

- Store the mechanism. The [[Action-Graph]] records the verb and the query, so a replay
  chooses the lowering again against the world as it is then.

## Related systems

- [[Programs]] — emits the verbs
- [[Marco-Boundary]] — seven new capabilities (`Accessibility's Invoke/Expand/Collapse/
  Toggle/Select/Deselect/ScrollIntoView`) were declared to make this expressible
- [[Action-Graph]] — records verb, not mechanism

## Decisions

- [[ADR-004-vision-cannot-establish-actionability]]
- [[ADR-005-legal-marco-only]]
- [[ADR-007-no-progress-no-repetition]]

## Validated by

- `internal/director/uiact` unit tests
- `director actions [name]`, `director explain action` — diagnostics that show the ladder's
  reasoning

## Known gaps

See the *Known gaps* section of [[director-semantic-actions]].

## Milestone record

[[director-semantic-actions]] — the ladder, lowering, verification, policy, what changed for
existing behaviour.
