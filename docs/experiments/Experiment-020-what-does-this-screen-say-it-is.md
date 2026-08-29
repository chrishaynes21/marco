---
type: experiment
status: complete
date: 2026-08-29
backend:
  - production-perception
  - semantic-memory
fixture: cmd/director/nameprobe.go
result: no-rule-over-current-evidence-separates-a-flat-destination-from-an-unread-hierarchy
supersedes: []
source_paths:
  - cmd/director/nameprobe.go
  - internal/director/observe/sample.go
  - internal/director/observe/destinationname_test.go
---

# Experiment 020 — what does this screen say it is?

## Question

[[Experiment-019-the-responsive-breakpoint]] found that a collapsed Windows Settings page still
contains its own name in a breadcrumb, while `AdmittedPlaceName` reads the selected navigation
item — which is what the collapse removes. It also caught the Printers page naming itself after
its section at one width and not the others.

> Can Marco name the destination it is on, rather than the shell, the section, or the navigation
> control that led there — stably across presentations?

## Method

`director name-probe`: one collection through the production collector and fusion, then
`placeNameEvidence` and `observe.ExplainPlaceName` — the producer and the rule a live sample
calls, in that order. Nothing in the probe parses anything itself, so a claim it prints is a claim
production made.

`AdmittedPlaceName` was first refactored into `ExplainPlaceName` with the verdict discarded — the
`ExplainStructure`/`CompareStructure` shape — so the reasoning could be read without a second
implementation existing to disagree with it. Suite green before any behaviour question was asked.

The presentation is reported as **whether anything on screen reports itself as the selected
destination**, not from a window width (which describes one machine's DPI and font scaling) and
not from a label like `Open Navigation` (which describes one operating system in one language).

## A screen carries several true names at once

Windows Settings, Printers page, 1500px:

```
Settings              the application shell
Bluetooth & devices   the section, and the selected item in the navigation rail
Printers & scanners   where the person actually is
```

None is wrong. Only the last may be a Place's name, and the existing rule separates them with the
TRAIL: the group of sibling buttons that contains the selected word is the path taken, and the
entry that is not the selected word is the leaf.

## The Printers trap, explained by the rule's own account

```
1500  navigation [list_item "Bluetooth & devices"]
      siblings   [Bluetooth & devices, Printers & scanners]
      -> "Bluetooth & devices" section (trail ancestor)
       * "Printers & scanners" destination (trail leaf)          DESTINATION "Printers & scanners"

 850  navigation [list_item "Bluetooth & devices"]
      siblings   [More, Printers & scanners]
       * "Bluetooth & devices" destination (selected item)       DESTINATION "Bluetooth & devices"

 750  navigation no element reports itself as the selected destination
      siblings   [Bluetooth & devices, Printers & scanners]
      claims     none                                            NO NAME
```

At 850 the breadcrumb's ancestor has collapsed into an overflow control, so no group holds the
selected word, the trail lookup finds nothing, and the section is admitted by default. At 750 the
breadcrumb is back and complete — and there is no selected item, so the producer emits nothing
and the rule never sees it.

The same shape on the other two destinations: Mouse names itself correctly at 1500/1000/850 and
not at all below; Bluetooth likewise, naming itself `Devices` — a content button that shares the
breadcrumb's parent, which is a third weakness of "sibling buttons" as a path.

## Why the obvious fix was rejected

Requiring the trail to corroborate a selected item was implemented and measured.

**It fixes two real wrong names.** Printers at 850 stops claiming to be its section. And Visual
Studio Code, probed on the same build, today names itself:

```
navigation [tab "Terminal (Ctrl+`)"]
 * "Terminal (Ctrl+`)"  destination  selected item     DESTINATION "Terminal (Ctrl+`)"
```

a keyboard hint — the exact failure ADR-076's own comment warned about.

**And it breaks Settings Home.** Measured:

```
navigation [list_item "Dark" list_item "Home"]
siblings   [Add device, View all devices]
siblings   [Close Settings, Maximize Settings, Minimize Settings]
siblings   [Ethernet 2 Connected, Windows Update Last checked: 2 hours ago]
siblings   [Get more storage, Manage cloud storage, PC backup]
   "Dark"  unknown  selected value
 * "Home"  destination  selected item                   DESTINATION "Home"
```

No group holds `Home`. ADR-076 recorded a one-entry trail there; this Windows build no longer
produces one. Home is named correctly today **by the same path that gets Printers wrong**.

So `Home` and `Printers at 850` arrive as the identical shape — one selected navigable item, no
trail containing it — with opposite correct answers. Nothing in the evidence the rule receives
separates them.

## Result

`STAGE_A = PARTIAL`. Ownership is unified and the rule explains itself; the destination/section
distinction is explicit and mutation-proven where a trail exists; ambiguity is honest. Naming does
**not** survive the responsive breakpoint, and two wrong names are recorded rather than fixed.

`STAGE_B = NOT ATTEMPTED`, on the roadmap's own condition: identity integration requires strong
Stage A naming, and a name that disappears at the breakpoint cannot bridge one. Place identity is
byte-identical and the 37J firewall passes unchanged.

## Multi-application

| surface | destination naming |
|---|---|
| Windows Settings, wide | **strong** — section and destination separated correctly on all three pages |
| Windows Settings, at the overflow width | **wrong** — the section is admitted |
| Windows Settings, collapsed | **none** — no selected item; the breadcrumb is present and unreachable |
| Visual Studio Code | **wrong** — an uncorroborated selected tab yields `Terminal (Ctrl+`)` |
| Explorer, Chrome | **not measured** — both had several windows open and the probe refuses an ambiguous selector rather than guessing |

## What the next attempt needs

Not a better rule over this evidence. It needs evidence that distinguishes a breadcrumb from a row
of content buttons without naming an application: the accessibility hierarchy above the group, or
whether the group's members are reachable as destinations elsewhere in the application's
remembered topology. Marco already stores the second, and it would make "is this a path" a
question about what Marco knows rather than about how one operating system draws a header.

Neither is measured. See [[ADR-109-a-screen-carries-several-true-names-at-once]].
