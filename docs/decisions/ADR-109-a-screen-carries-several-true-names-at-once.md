---
type: decision
status: accepted
date: 2026-08-29
supersedes: []
affects:
  - perception
  - semantic-memory
source_paths:
  - internal/director/observe/sample.go
  - internal/director/observe/destinationname_test.go
  - cmd/director/nameprobe.go
  - cmd/director/observewiring.go
---

# ADR-109 — a screen carries several true names at once

## Context

[[ADR-108-what-a-reflow-removes-cannot-always-be-told-from-where-you-are]] closed responsive Place
identity as `CONSERVATIVE_UNKNOWN` and named the most promising route out: the page names itself in
a breadcrumb at both presentations, and `AdmittedPlaceName` reads the *selected navigation item*,
which is exactly what a collapsed layout removes.

It also recorded a trap. Measured across widths, Windows Settings' Printers page named itself
`Printers & scanners` at 1500 and 1000 and `Bluetooth & devices` — its **section** — at 850. A
name that moves between presentations would move identity with it.

37K went to find out why.

## What a name probe found

`director name-probe` runs one collection through the production collector and fusion, then calls
`placeNameEvidence` and `observe.ExplainPlaceName` — the producer and the rule a live sample calls,
in that order — and prints every claim with what it names and why.

**A screen carries several true names at once.** On Printers at 1500px:

```
Settings              the application shell
Bluetooth & devices   the section, and the selected item in the navigation rail
Printers & scanners   where the person actually is
```

None is wrong. Only the last may be a Place's name, and the rule that separates them already
exists: the TRAIL — the group of sibling buttons containing the selected word is the path, and the
entry that is not the selected word is the leaf.

**The Printers trap is the trail going missing, not a naming rule choosing badly.** The probe's own
account, at three widths:

```
1500  siblings [Bluetooth & devices, Printers & scanners]   -> section + leaf   "Printers & scanners"
 850  siblings [More, Printers & scanners]                  -> no trail         "Bluetooth & devices"
 750  siblings [Bluetooth & devices, Printers & scanners]   -> no selection      no name
```

At 850 the breadcrumb's ancestor has collapsed into an overflow control, so no group holds the
selected word, the trail lookup finds nothing, and the selected SECTION is admitted as the
destination by default. At 750 the breadcrumb is back and complete — and there is no selected item
at all, so the producer emits nothing and the rule never sees it.

## The decision

**The naming rule is unchanged, and the reason is a measurement.**

The obvious fix is to stop admitting a selected item that no trail corroborates. It was
implemented and measured. It fixes two real wrong names:

- Printers at 850, which stops claiming to be its section.
- **Visual Studio Code, which today names itself `Terminal (Ctrl+`)`** — a keyboard hint, from an
  uncorroborated selected tab. This is the exact failure ADR-076's own comment warned about.

And it breaks the most ordinary page in the application:

- **Settings Home has no trail either.** Measured on this build: the selected item is
  `list_item "Home"` and the sibling-button groups are `[Add device, View all devices]`,
  `[Close Settings, …]`, `[Ethernet 2 Connected, …]`, `[Get more storage, …]`. None holds `Home`.
  ADR-076 recorded a one-entry trail on Home; this Windows build no longer produces one, and Home
  is named correctly today **by the very path that gets Printers wrong**.

So `Home` and `Printers at 850` reach the rule as the identical shape — one selected navigable
item, no trail containing it — and their correct answers are opposite. **Nothing in the evidence
the naming rule receives separates them.** Refusing both loses more than it saves.

## What did change

**One rule, and it can now explain itself.** `observe.ExplainPlaceName` returns the admitted name
plus every claim considered, each with its semantic level and why it was or was not admitted;
`AdmittedPlaceName` is that with the explanation discarded — the same shape as `ExplainStructure`
and `CompareStructure`, and for the same reason.

The levels are the distinction this phase exists to make explicit:

| | |
|---|---|
| `destination` | the leaf — where the person is. The only thing a Place may be called. |
| `section` | a true name for the region this destination sits in. Never a Place's name. |
| `unknown` | a claim whose level could not be established. Not admissible. |

**A probe that asks production.** `director name-probe` prints the claims, the trail evidence they
were read from, and the presentation — reported as whether anything reports itself as the selected
destination, rather than from a window width (which describes one machine's DPI) or a label like
`Open Navigation` (which describes one operating system in one language).

## Consequences

- **Stage B was not attempted.** The roadmap's own condition was that identity integration
  requires strong Stage A naming. Naming does not survive the breakpoint, so a name cannot yet
  bridge one. `CONSERVATIVE_UNKNOWN` stands.
- **A Learn performed while the navigation is collapsed still produces an unnamed Place**, and a
  play cannot be written down against an unnamed screen. That debt is unchanged and now has a
  measured cause.
- **Two known wrong names are recorded rather than fixed**: Printers at its overflow width, and
  VS Code's keyboard hint. Both are stated as tests so the next attempt starts from the evidence.
- Place identity is untouched: `StructureSignature`, `sameRoleSet`, `CompareStructure` and `Recall`
  are byte-identical, and the 37J firewall passes unchanged.

## What the next attempt needs

Not a better rule over this evidence — there isn't one. It needs evidence that distinguishes a
breadcrumb from a row of content buttons without naming an application. Two candidates, neither
measured yet: the accessibility hierarchy above the button group (is this group inside something
that describes itself as navigation?), and whether the group's members are also reachable as
destinations elsewhere in the same application's remembered topology.

The second is interesting because Marco already stores that topology, and because it would make
"is this a path" a question about what Marco knows rather than about how one operating system
draws a header.

## Enforced by

- `internal/director/observe` `TestTheTrailLeafNamesTheDestinationAndTheAncestorNamesTheSection`
- `internal/director/observe` `TestTwoDestinationsUnderOneSectionAreNamedApart`
- `internal/director/observe` `TestATrailLessSelectionIsAdmittedAndThatIsNotAlwaysRight` — the
  negative result, with both measured cases, so a future change has to be deliberate
- `internal/director/observe` `TestATrailDeeperThanAnythingMeasuredNamesNothing`
- `internal/director/observe` `TestAValueIsRejectedWithAReasonAPersonCanRead`
- `internal/director/observe` `TestTheApplicationShellCannotNameTheDestination`
- `cmd/director` `TestTheNameProbeUsesTheProductionNamingPath`,
  `TestTheProbeDoesNotDecidePresentationFromGeometryOrLanguage`

## Related

- [[Experiment-020-what-does-this-screen-say-it-is]]
- [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]
- [[ADR-108-what-a-reflow-removes-cannot-always-be-told-from-where-you-are]]
- [[ADR-031-the-user-names-the-stage]]
