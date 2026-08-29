---
type: experiment
status: complete
date: 2026-08-29
backend:
  - production-perception
  - semantic-memory
fixture: cmd/director/pathevidence_test.go
result: neither-local-hierarchy-nor-remembered-topology-separates-a-destination-from-its-section
supersedes: []
source_paths:
  - cmd/director/nameprobe.go
  - cmd/director/pathevidence_test.go
---

# Experiment 021 — is this group a path?

## Question

[[Experiment-020-what-does-this-screen-say-it-is]] reduced destination naming to one ambiguity.
The same evidence shape — one selected navigable item, no trail containing it — means opposite
things:

```
Settings Home       selected `Home`                 -> Home IS the destination
Printers at 850px   selected `Bluetooth & devices`  -> the destination is its child
```

> Can anything else Marco has separate them: the accessibility hierarchy above the claim, or the
> semantic topology it has already learned?

## Method

`director name-probe --deep`, which adds to 37K's probe what CONTAINS every claim and every
sibling-button group. It still calls `placeNameEvidence` and `observe.ExplainPlaceName` — the
production producer and the production rule — so nothing here is a second parser.

Stage A first, as the roadmap requires: current evidence before memory.

## Stage A — the hierarchy above the selection

```
Home @1500     selected list_item "Home"                in [group(-) pane(-) window(-) unknown(-) window(Settings) window(Settings)]
Printers @850  selected list_item "Bluetooth & devices" in [group(-) pane(-) window(-) unknown(-) window(Settings) window(Settings)]
Printers @1500 selected list_item "Bluetooth & devices" in [group(-) pane(-) window(-) unknown(-) window(Settings) window(Settings)]
```

**Byte-identical.** Whatever contains a selected rail item is the same thing on the screen where
the selection is the destination and on the screen where it is the section.

There IS a structural difference elsewhere. The breadcrumb's buttons sit outside the content pane:

```
Printers @1500  button "Bluetooth & devices"  in [group(-) unknown(-) window window]      depth 4
Printers @850   button "More"                 in [group(-) unknown(-) window window]      depth 4
                button "Printers & scanners"  in [group(-) unknown(-) window window]      depth 4
Home @1500      (no group at that depth)
content buttons in [group(Cloud storage) pane(-) group(-) unknown(-) window window]       depth 6
window chrome   in [window(Settings) window(Settings)]                                    depth 2
```

Every content button passes through a `pane`; the breadcrumb does not, and Home has no such group
at all — which is coherent, since the root has no path to show.

That is a real signal and it was not used. Identifying a group by the roles above it is the shape
of heuristic 37D already measured into the ground; it rests on one application; and it does not
finish the job. At 850 the group is `[More, Printers & scanners]` — knowing it is a path still
leaves two unordered entries, the selected word is in neither, and `PlaceNameEvidence` carries no
rectangle on purpose, so "the last one" is not a question Marco may ask.

**Visual Studio Code does differ locally:**

```
selected tab "Terminal (Ctrl+`)" in [tab_list(Active View Switcher) pane(-) group(-)
                                     text_field(● E2E.md - marco - Visual Studio Code) pane pane pane pane]
```

A `tab_list` rather than a rail group. So a local rule is imaginable for the VS Code failure. One
data point; not built.

`LOCAL_STRUCTURE_SEPARATES_HOME_PRINTERS: NO.`

## Stage B — the graph, and why the rail closes it

The hypothesis: if the selected Place has a remembered edge to a currently visible label, the
visible label is the destination and the selection is only the section.

**Settings Home kills it.** The rail publishes every one of its children:

```
Home, System, Bluetooth & devices, Network & internet, Personalization, Apps, Accounts,
Time & language, Gaming, Accessibility, Privacy & security, Windows Update, Sound, Display, Camera
```

A navigation rail is a list of places you COULD go, and it is on every wide page. The
precondition — "a visible label the selected Place has an edge to" — is therefore satisfied
everywhere, and the rule would demote every parent to a section, beginning with the one screen
whose name is currently correct.

Constraining topology to the breadcrumb group would avoid it, and needs the Stage A signal that is
not available.

A second objection did not have to be reached: the graph's `Printers & scanners` was named by this
same rule at the widths where the trail exists. Refereeing the rule with it is sound only if a
name from a corroborated trail is distinguished from a name from a bare selection. That provenance
exists on `NameClaim.Function` and is not persisted, and building it to rescue an already-unsafe
idea would be the wrong order.

`TOPOLOGY_SEPARATES_HOME_PRINTERS: NO — REJECTED_FALSE_POSITIVE.`

## Result

`CONSERVATIVE_UNKNOWN`. No production change. Both candidate routes closed by measurement.

The findings are kept as fixtures rather than prose: the two screens' evidence shapes are asserted
identical, the rail is asserted to sit beside the selection, and the producer is asserted to read
the current world and nothing else.

## What stays wrong

- Printers at its overflow width is named after its section.
- VS Code names itself `Terminal (Ctrl+`)`.
- A Learn while the navigation is collapsed produces an unnamed Place — measured, two cold Learns
  on the same page: at 1500px `semantic=[Mouse]`, at 750px `semantic=[]`.

## What would move it

Not a cleverer rule over accessibility. The two screens are genuinely indistinguishable to
everything Marco can read, and the discriminating fact — a breadcrumb is a path, a rail is a menu —
is not exposed by UI Automation here.

The next question is not architectural. It is how much this costs in use.
