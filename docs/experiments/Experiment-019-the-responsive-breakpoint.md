---
type: experiment
status: complete
date: 2026-08-28
backend:
  - production-perception
  - semantic-memory
fixture: internal/director/observe/responsive_test.go
result: no-safe-rule-recognises-a-page-once-its-navigation-collapses
supersedes: []
source_paths:
  - internal/director/observe/responsive_test.go
  - internal/director/observe/recall.go
  - cmd/identityprobe/main.go
---

# Experiment 019 — the responsive breakpoint

## Question

[[Experiment-017-live-place-identity-convergence]] measured one Settings page as one Place from
1500px to 850px with a byte-identical signature, and a false miss below that.

> When structure disappears because the interface reflowed, can Marco tell that the semantic
> destination did not change — without merging destinations that were never the same?

## Method

Three destinations — Mouse, Bluetooth & devices, Printers & scanners — at 1500, 1000, 850, 800,
750 and 650px. Isolated `$MARCO_HOME`, cold store, no detector, real Director, real accessibility,
real fusion, `SignatureOfState` on the settled state of each reading. Each capture takes the same
number of samples, because [[ADR-106-a-place-is-not-how-long-you-looked-at-it]] measured that a
session-length difference moves the sufficiency judgement.

Every pair — 45 same-destination, the rest cross-destination — was then evaluated under candidate
rules, so "no safe rule exists" is a measurement rather than an impression.

`identityprobe` was changed to ask `ExplainStructure` instead of re-deriving its own comparison,
and to report terms as LOST and GAINED rather than as two lists. It had been comparing raw roles
where the matcher compares `identityRoles`, so it named layout roles as decisive that the matcher
had already dropped.

## The breakpoint

Between 850 and 800px, identical for all three pages. **Verified semantically, not by width:** at
the collapsed presentation a `button "Open Navigation"` and a `button "Expand search box"` appear
and the navigation list is gone. Windows scales breakpoints with DPI and font size, so the pixel
figure is an observation about this machine and the affordance is the condition.

```
                    name                  terms                          identity roles
mouse 1500/1000/850 "Mouse"               back controls settings         button 14 combo_box 3 image 13 list_item 15 menu 1 menu_item 1 slider 2 text_field 1 window 4
mouse 800/750/650   —                     back controls search settings  button 15 combo_box 3          list_item  3 menu 1 menu_item 1 slider 2              window 3
bt    1500/1000/850 "Devices"             audio back display settings    button 30 image 13 list 3 list_item 12 menu 1 menu_item 1 text_field 1 window 4
bt    800/750/650   —                     audio back display search set. button 31          list 3                menu 1 menu_item 1              window 3
print 1500/1000     "Printers & scanners" back settings                  button 18 image 13 list 1 list_item 12 menu 1 menu_item 1 text_field 1 window 4
print 850           "Bluetooth & devices" back settings                  button 19 image 13 list 1 list_item 12 menu 1 menu_item 1 text_field 1 window 4
print 800/750/650   —                     back search settings           button 19          list 1                menu 1 menu_item 1              window 3
```

**Lost, every page:** 13 `image`, 1 `text_field`, the navigation's `list_item`s, and the selected
navigation entry — which is where `AdmittedPlaceName` reads the page's name, so a collapsed
reading does not know what it is called. **Gained:** the `search` term.

The decisive field is `role_set`, on all three pages: `image` and `text_field` leaving fires
before any count or term is compared.

## Why no rule was shipped

### The same event is a contradiction on one page and a silence on another

Mouse has three list items of its own. The navigation's twelve leaving takes `list_item` from
15 to 3 — a **positive count disagreement on a shared role**. Bluetooth and Printers have no
content list items, so the same event removes the role entirely: an **absence**.

The durable signature is a flat role histogram. `ScreenSignature` decomposes a surface into
cells; `StructureSignature` does not, and adding geometry to it would make identity more
presentation-sensitive rather than less. So a rule built on "absence is not contradiction" —
which is the right distinction, and a real seam in `sameRoleSet` — does not reach the page this
phase was written for.

### The rule that would work is a measured false merge

Over every pair:

| rule | same-destination matches | false merges |
|---|---|---|
| current matcher | 18 / 45 | 0 |
| shared roles only, terms exact | 18 / 45 | 0 |
| shared roles only, terms nested | 36 / 45 | **15** |
| shared roles only, counts tolerant, terms nested | 36 / 45 | **15** |

All fifteen are **Mouse against Printers**: `back settings` is a subset of `back controls
settings`, and every role the two share agrees within tolerance. Requiring a shared *distinctive*
term removes them — but `settings` is generic in an operating system and destination-bearing in
a game's settings menu, which is what the term vocabulary was built for, so distinctiveness
cannot be declared statically.

And none of these rules matches Mouse across the breakpoint anyway.

### Printers, collapsed, settles it

Its whole account is `back search settings`, plus button, list, menu, menu_item, window — the
furniture of every page of that application. Nothing in that reading says which destination it
is. `UNKNOWN` is the only honest answer, and any rule lenient enough to change it would recognise
anything.

## What survives, and where it is not being read

The collapsed reading contains `button "Mouse"` and `text "Mouse"` — a breadcrumb. **The page
names itself at both presentations.** `AdmittedPlaceName` reads the *selected navigation item*,
which is exactly what disappears, so the name is present in the world and not admitted.

That is the most promising route to closing this, and it is a perception change rather than a
matcher change. It is not free: the same run measured Printers naming itself `"Printers &
scanners"` at 1500 and 1000 and `"Bluetooth & devices"` — its section — at 850. A name that moves
between presentations would move identity with it, so the naming rule would have to be made
stable before it could carry any weight.

## Result

`CONSERVATIVE_UNKNOWN`. No production change. The measurement is recorded as a firewall in
`responsive_test.go`, whose fixtures are the signatures above and whose mutation gate kills the
loosenings this experiment rejected.

See [[ADR-108-what-a-reflow-removes-cannot-always-be-told-from-where-you-are]].

## What this does not answer

**Any application but Windows Settings.** One application, three destinations, one machine, one
DPI. The rule rejected here was rejected on that evidence; a different interface might lose less.

**Whether a breadcrumb-derived name would generalise.** Settings has one. Many applications do
not, and nothing here measured what those do at their own breakpoints.
