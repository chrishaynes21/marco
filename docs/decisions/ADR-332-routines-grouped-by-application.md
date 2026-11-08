---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — STUDIO'S ROUTINE LIST IS GROUPED BY THE APPLICATION EACH ROUTINE IS FOR. One head per application (the routine's `app`), in name order, with the routines that switch nothing last under `Any application`; inside a group the rows keep the list's own order. A head shows the application and how many routines it holds, and pressing it shuts or opens that group. Once there are two or more applications a bar above the list offers Show (All applications, or one application alone), Expand all and Collapse all, each dark when it would change nothing. Shutting and showing only hide rows: nothing is drawn again, so an open routine in a shut group keeps what somebody had typed into it. Which groups are shut is kept in the window's browser storage, a per-viewer convenience; the one-application choice is not kept. A routine that is opened (by a save, a request or a press) is shown: its group opens and Show goes back to All applications if it was filtered away. A group head is a SIBLING of its rows, not their parent, so each row is still a child of the list."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/frontend/index.html
---

# Routines grouped by application

Builds on [[ADR-290-a-routine-opens-where-it-is]] and [[ADR-309-a-routine-row-says-only-what-changes-the-next-action]].

## Why

The owner: "grouping routines by the app they target so you can collapse say all ark ascended
routines or only show one set of routines. Add collapse all and expand all options too".

## Decision

- **Grouped by `app`**, the application the routine was recorded in. That is already in every
  row, so this needs nothing new from the backend. Routines with no application (they switch
  nothing) come last, under *Any application*.
- **Shut and Show hide rows; they don't redraw the list.** The open routine's panel is carried
  between rows and never rebuilt ([[ADR-290-a-routine-opens-where-it-is]]), so a redraw would put
  back focus and typed values the person had moved past. Hiding leaves them where they are.
- **A routine that is opened is always shown.** A routine opened by a save or a request in a shut
  or filtered-away group would otherwise open out of sight.
- **The shut groups are remembered in the window's own storage**, because they are a viewing
  preference, not a fact about the library. If that storage is unavailable, every group starts
  open. Show is not remembered, because a list that opens showing one application looks like
  routines have gone missing.
- **The head is a sibling of its rows**, so every row is still `#routineList > .card.pick`, the
  address the other page tests and the open-in-place panel use.

## Not established

- Tried only in headless Edge, at 1100 and 500px wide, not in Studio's WebView2 window.
- Whether the shut groups survive a restart of Studio depends on WebView2 keeping the page's
  storage between runs. That has not been checked, and nothing breaks if it doesn't.

## Enforced by

- `plugins/studio`: `TestTheRoutinesAreGroupedByTheirApplication`, `TestARoutineOpensWhereItIsAndPlayStaysOnItsRow` (counts one disclosure per row, not per head).
