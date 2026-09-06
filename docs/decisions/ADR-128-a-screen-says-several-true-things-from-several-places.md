---
type: decision
status: accepted
date: 2026-09-05
affects:
  - passive-observation
source_paths:
  - internal/director/observe/terms.go
  - internal/director/observe/screenstate.go
  - cmd/director/observewiring.go
  - cmd/director/main.go
---

# ADR-128 — a screen says several true things, from several places

## Context

Natural dogfood — four applications, eight minutes, no route — produced three durable Places all
named `Sometimes Silly`, and affordances including `Dhjango`, `:perfection:` and
`Click to remove fire`. Settings could never have shown this.

Measuring two applications through the production accessibility path:

| | Discord | Explorer |
|---|---|---|
| selected navigation | `Sometimes Silly` — the **server** | `Documents` — the **folder** |
| container label | `Messages in irl` — the **channel** | `Items View` — **generic** |
| window title | refused by the shape filter | `Documents - File Explorer` |
| content region | `list > list_item` — messages | `list > list_item` — files |
| navigation region | `tree > tree_item` — channels | `tree > tree_item` — folders |

**Both true, at different levels, and in different places.** A rule reading the container label as
the state is right about Discord and blind to Explorer. A rule reading selected navigation is right
about Explorer and names the server in Discord.

And content dominates: scrolling `#irl` changed the role signature by **46%** while `tree_item`
stayed at exactly 17. A different channel's composition sits *between* two scroll positions of the
same one — composition carries no information about which channel you are in. Three Places for one
screen is what a signature does when 90% of it is the conversation.

## Decision

**No source of semantic evidence is privileged.** A reading gathers claims and keeps their
provenance — the source, and the container role where one made the claim — and decides nothing
about which of them names anything.

```
selected_navigation      what you navigated to
container_label (role)   what a collection calls itself
window_title             what the window calls itself — a claim, never identity
```

The collector cannot judge usefulness without knowing that `Items View` is furniture, which is
exactly the application-specific knowledge this refuses. `Items View` is admitted faithfully;
discovering it is generic needs evidence across readings, which is interpretation's job.

Admission is the EXISTING policy: the one shape filter every word off a screen passes, the label
bound, a per-reading cap, and de-duplication per source. `ExplainClaims` keeps the reason;
`AdmitClaims` is the same rule with the explanation discarded, the shape `AdmittedPlaceName` has
over `ExplainPlaceName`.

Claims are tallied per screen state, keyed **by source and text**, because `Documents` from
navigation and from a title are two independent witnesses and collapsing them would lose the
corroboration that makes either believable.

### The result nobody designed

The shape filter refuses `#irl | Sometimes Silly - Discord` outright — `#` and `discord` are both
`privateMarkers`. So the title, which is where Explorer keeps its folder, is unavailable on exactly
the application whose content is most sensitive. And it does not matter, because that is the
application whose container label names the state.

> Where one source is refused, another still speaks. Any single privileged source is either refused
> or generic somewhere.

That is the argument for collecting from several, and it is now a test.

## Consequences

- **No behaviour change.** `PlaceName` still comes from selected navigation and everything
  downstream decides exactly as before. What changed is that the other claims survive the reading.
- The window title is a claim and structurally cannot be identity: `NewScreenSignature` takes
  regions, and a claim is not a region. *A window is not a place* is unchanged.
- Nothing here classifies content. No `list == content`, no `ancestor == list_item → transient`, no
  churn rule. The measurement forbids all three: Explorer's file list churns exactly like Discord's
  messages and is the thing Marco most needs to act on.
- A third application arrived unplanned in the closeout reading. VS Code follows Explorer's
  arrangement — navigation carries the state, container labels are furniture (`Terminal tabs`,
  `Active View Switcher`), the title names the project.
- What remains open, and deliberately: durable affordance policy. The representation now carries
  what that decision needs without making it.

## Enforced by

- `internal/director/observe` `TestEveryClaimKeepsWhereItCameFrom`
- `internal/director/observe` `TestAGenericClaimIsAdmittedRatherThanJudged`
- `internal/director/observe` `TestAClaimPassesTheOneShapeFilter`
- `internal/director/observe` `TestWhereOneSourceIsRefusedAnotherStillSpeaks`
- `internal/director/observe` `TestClaimsSurviveTheProductionEvidencePath`
- `internal/director/observe` `TestAClaimReachesTheScreenStateThroughTheSegmenter`
- `internal/director/observe` `TestAWindowTitleIsAClaimAndNotIdentity`
- `cmd/director` `TestAScreenIsHeardFromEverySourceThatSpoke` — both measured shapes

## Related

- [[ADR-124-a-screen-may-say-which-screen-it-is]]
- [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]
- [[Experiment-022-the-first-dogfood]]
