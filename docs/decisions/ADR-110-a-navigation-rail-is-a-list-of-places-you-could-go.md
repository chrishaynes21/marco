---
type: decision
status: accepted
date: 2026-08-29
supersedes: []
affects:
  - perception
  - semantic-memory
source_paths:
  - cmd/director/nameprobe.go
  - cmd/director/pathevidence_test.go
  - cmd/director/observewiring.go
---

# ADR-110 — a navigation rail is a list of places you could go

## Context

[[ADR-109-a-screen-carries-several-true-names-at-once]] reduced destination naming to one
ambiguity. A selected navigable item with no trail containing it means opposite things on two
real Windows Settings screens:

```
Settings Home       selected `Home`                 -> Home IS the destination
Printers at 850px   selected `Bluetooth & devices`  -> the destination is its CHILD,
                                                       `Printers & scanners`, whose breadcrumb
                                                       ancestor collapsed into an overflow
```

Requiring corroboration fixes the second and breaks the first. 37L asked whether anything else
Marco has could separate them: the accessibility hierarchy above the claim, or the semantic
topology Marco has already learned.

**Both were measured. Both are closed.**

## The hierarchy above the selection is identical

`director name-probe --deep` reports the ancestry of every claim. On the two screens:

```
Home @1500     selected list_item "Home"                in [group pane window unknown window window]
Printers @850  selected list_item "Bluetooth & devices" in [group pane window unknown window window]
```

Byte-identical. Whatever contains a selected rail item, it is the same thing on the screen where
the selection IS the destination and on the screen where it is the section. The evidence
`placeNameEvidence` hands the rule differs only in the word.

There is a real structural difference elsewhere on the screen — the breadcrumb's buttons sit
outside the content pane, at `group → unknown → window`, while content buttons sit inside a
`pane`. But identifying a group by the roles above it is the kind of rule 37D already measured
into the ground, it rests on one application, and it does not finish the job: at 850 the group is
`[More, Printers & scanners]`, and knowing that group is a path still leaves two unordered entries
with nothing to choose between them. `PlaceNameEvidence` carries no rectangle on purpose
([[ADR-076-a-place-may-say-what-it-appears-to-be-called]]), so "the last one" is not a question
Marco may ask.

## The graph cannot referee it either, and the reason is the rail

The topology hypothesis: if the selected Place has a remembered edge to a label that is currently
visible, perhaps the visible label is the destination and the selection is only the section.

**Settings Home kills it.** Measured — the rail on Home publishes every one of its children as a
visible list item:

```
Home, System, Bluetooth & devices, Network & internet, Personalization, Apps, Accounts,
Time & language, Gaming, Accessibility, Privacy & security, Windows Update, Sound, Display, Camera
```

A navigation rail is a list of places you COULD go. It is present on every wide page. So the
precondition — "a visible label the selected Place has an edge to" — is satisfied *everywhere*,
and the rule would demote every parent to a section, starting with the one screen whose name is
currently right. Marco would rename Home after whichever section it happened to have learned an
edge to.

Constraining topology to the breadcrumb group would avoid that — and requires the structural
identification above, which is the part that is not available.

There is a second reason to be wary, and it did not need to be reached: the graph's Place named
`Printers & scanners` was itself named by this rule, at the widths where the trail exists. Using it
to referee the rule is only sound if a name established from a corroborated trail is distinguished
from one established from a bare selection. That provenance exists on `NameClaim.Function` and is
not persisted, and building it to rescue an already-unsafe idea would be the wrong order.

## The decision

**No change to naming. `CONSERVATIVE_UNKNOWN` stands, with both candidate routes closed by
measurement rather than by argument.**

And one boundary is now enforced rather than merely intended:

**Memory may help interpret current evidence. Memory may never become current evidence.**
`placeNameEvidence` reads one fused world and nothing else — no store, no topology, no goal, no
play, no remembered name. A remembered Place is not a visible Place; a remembered label is not a
visible label.

## What is known to be wrong and stays wrong

- **Printers at its overflow width** is named after its section.
- **Visual Studio Code** names itself `Terminal (Ctrl+`)` — a keyboard hint from a selected tab
  inside a `tab_list(Active View Switcher)`. Its ancestry *does* differ from a Settings rail, so a
  local rule is imaginable here; it rests on one data point and was not built.
- **A Learn performed while the navigation is collapsed produces an unnamed Place.** Measured, two
  cold Learns on the same page: at 1500px `semantic=[Mouse]`, at 750px `semantic=[]`.

## Consequences

- Place identity is untouched. `StructureSignature`, `sameRoleSet`, `CompareStructure` and `Recall`
  are byte-identical and the 37J firewall passes unchanged; responsive identity remains
  `CONSERVATIVE_UNKNOWN`.
- `director name-probe --deep` now reports what contains every claim and every sibling group, so
  the next person asking this question starts from evidence rather than from a rebuild.
- No topology query, no authority, no lease, no input, no application name, no width, no role
  blacklist entered production.

## What would actually move this

Not a cleverer rule over accessibility. The two screens are genuinely indistinguishable to
everything Marco can currently read, and the discriminating fact — that a breadcrumb is a path and
a rail is a menu — is not exposed by UI Automation on this application.

The honest next step is not another naming phase. It is to find out how much this costs in use:
whether an unnamed collapsed Place is a real obstacle to somebody working, or a diagnostic
blemish. That question is answered by using Marco, not by reading more trees.

## Enforced by

- `cmd/director` `TestTheHierarchyAboveTheSelectionDoesNotSayWhetherItIsTheDestination` — the two
  screens produce indistinguishable evidence, so a future claim that the hierarchy separates them
  has to change this test and say what changed
- `cmd/director` `TestAPagePublishesItsOwnChildrenSoVisibilityIsNotAPath` — the rail beside the
  selection, which is what makes visibility-plus-edge unsafe
- `cmd/director` `TestTheNameProducerReadsOnlyTheCurrentWorld` — the memory boundary
- `internal/director/observe` — 37K's naming fixtures, unchanged and still green

## Related

- [[Experiment-021-is-this-group-a-path]]
- [[ADR-109-a-screen-carries-several-true-names-at-once]]
- [[ADR-108-what-a-reflow-removes-cannot-always-be-told-from-where-you-are]]
- [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]
