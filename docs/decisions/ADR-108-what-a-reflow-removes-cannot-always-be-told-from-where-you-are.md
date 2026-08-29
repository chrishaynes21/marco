---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - semantic-memory
  - perception
source_paths:
  - internal/director/observe/recall.go
  - internal/director/observe/responsive_test.go
  - cmd/identityprobe/main.go
---

# ADR-108 — what a reflow removes cannot always be told from where you are

## Context

[[ADR-106-a-place-is-not-how-long-you-looked-at-it]] measured that one Windows Settings page is
one durable Place from 1500px down to 850px with a **byte-identical** signature, and that below
that it stops being recognised. 37G declined to fix the lower half because the obvious remedy
would have cost the discrimination that keeps Settings pages apart.

[[ADR-107-a-sensor-appearing-is-not-the-screen-changing]] then established a vocabulary that
looked like it might generalise: evidence ABOUT a screen is not the same as evidence a screen is
identified BY. This asks the analogous question for responsive layout.

## What was measured

Three destinations — Mouse, Bluetooth & devices, Printers & scanners — at six widths each,
through the production perception path into `SignatureOfState`. Isolated store, no detector.

The breakpoint is **between 850 and 800px and identical for all three pages**, and it was
verified semantically rather than by width: at the collapsed presentation a `button "Open
Navigation"` and a `button "Expand search box"` appear, and the navigation list is gone.

```
                    name                  terms                            identity roles
mouse 1500/1000/850 "Mouse"               back controls settings           button 14 combo_box 3 image 13 list_item 15 … slider 2 text_field 1 window 4
mouse 800/750/650   —                     back controls search settings    button 15 combo_box 3          list_item  3 … slider 2              window 3
bt    1500/1000/850 "Devices"             audio back display settings      button 30 image 13 list 3 list_item 12 … text_field 1 window 4
bt    800/750/650   —                     audio back display search set.   button 31          list 3            …              window 3
print 1500/1000     "Printers & scanners" back settings                    button 18 image 13 list 1 list_item 12 … text_field 1 window 4
print 800/750/650   —                     back search settings             button 19          list 1            …              window 3
```

**Lost on every page:** thirteen `image`, one `text_field`, the navigation's `list_item`s, and
the selected navigation entry — which is where `AdmittedPlaceName` reads what the page is called,
so the collapsed reading does not know its own name. **Gained:** the `search` term.

## The decision

**No change. The collapsed presentation stays unrecognised, and that is the correct answer on
this evidence.**

Two independent findings, and either alone is sufficient.

### The same interface event is a contradiction on one page and a silence on another

Mouse keeps three list items of its own. When the navigation's twelve leave, `list_item` goes
15 → 3: a **positive count disagreement** on a role both readings have. Bluetooth and Printers
have no content list items, so for them the identical event removes the role entirely — an
**absence**.

Nothing in the durable signature can tell those apart, because `StructureSignature` is a flat
role histogram. `ScreenSignature` decomposes a surface into cells; the durable signature
deliberately does not, and adding geometry to it would make identity more presentation-sensitive
rather than less — the exact thing [[ADR-091-a-place-is-not-its-presentation]] moved away from.

So "tolerate what a reflow removed" does not even reach the page 37J was written for.

### The loosening that would work is measurably a false merge

Every same-destination pair (45) and every cross-destination pair was evaluated under candidate
rules. Tolerating role-set absence and accepting one term set nested in the other raises
same-destination matches from 18 to 36 — **and produces fifteen false merges, all Mouse against
Printers**, because `back settings` is a subset of `back controls settings` and every role the
two share agrees within tolerance.

Requiring a shared *distinctive* term removes them. But "distinctive" cannot be declared
statically: `settings` is the generic term of an operating system and the destination-bearing
term of a game's settings menu, which is what the interface-term vocabulary was built for.
Deriving distinctiveness from the rest of the store would make recognition depend on what else
has been learned, and drift as the store grows.

**Printers, collapsed, is the case that settles it.** Its whole account is `back search settings`
plus button, list, menu, menu_item, window. Those are the furniture of every page of that
application. There is no evidence in that reading saying which destination it is, and no rule can
invent one. Any rule lenient enough to recognise it would recognise anything.

## What this establishes for the next attempt

- **Absence and contradiction are genuinely different**, and the matcher does conflate them —
  `sameRoleSet` treats a role one side lacks as decisive. That remains a real seam. It is not
  the seam that fixes this, because Mouse's loss arrives as a count disagreement instead.
- **Terms are compared exactly on purpose.** A screen that is about fewer things is not the same
  screen; a subset rule gives every specialised page its parent's identity.
- **The destination anchor survives, and is not admitted.** The collapsed reading contains
  `button "Mouse"` and `text "Mouse"` in a breadcrumb — the page names itself at both
  presentations. `AdmittedPlaceName` reads the *selected navigation item*, which is what
  disappears. That is the most promising route, and it is a perception change rather than a
  matcher change.
- **And the naming rule is not yet stable enough to carry identity.** Measured in the same run:
  Printers named itself `"Printers & scanners"` at 1500 and 1000, and `"Bluetooth & devices"` —
  its *section* — at 850. A name that moves between presentations would move identity with it.

## Consequences

- `35D_RESPONSIVE_ACCEPTANCE_STATUS` is **PASSED to 850px** and `CONSERVATIVE_UNKNOWN` below it.
- A Learn performed while the navigation is collapsed produces a Place with no semantic name,
  and a play cannot be written down against an unnamed screen. That is the same breakpoint
  arriving in a different subsystem, and it is the strongest argument for taking the naming route
  next.
- Nothing was loosened, no role was blacklisted, no width or application name entered production
  code, and no presentation variant is persisted.

## Enforced by

- `internal/director/observe` `TestTwoSettingsDestinationsNeverMergeAtAnyPresentation` — the
  firewall, over all three destinations at both presentations; killed by role-set absence
  tolerance and by both loosenings together
- `internal/director/observe` `TestAScreenAboutMoreThingsIsNotTheScreenAboutFewer` — why terms
  are exact; killed by term nesting, which the Settings fixtures alone do not catch
- `internal/director/observe` `TestACollapsedReadingWithNoDistinctiveEvidenceResolvesToNothing`
- `internal/director/observe` `TestPositiveEvidenceForAnotherDestinationIsNotMissingEvidence`
- `internal/director/observe` `TestTheResponsiveBreakpointIsStillAFalseMiss` — states the current
  answer and the field that decides it, so a future change has to be deliberate

## Related

- [[Experiment-019-the-responsive-breakpoint]]
- [[ADR-091-a-place-is-not-its-presentation]]
- [[ADR-106-a-place-is-not-how-long-you-looked-at-it]]
- [[ADR-107-a-sensor-appearing-is-not-the-screen-changing]]
- [[ADR-031-the-user-names-the-stage]]
