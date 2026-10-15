---
type: decision
status: accepted
date: 2026-10-15
deciders: director, owner
affects:
  - perception
  - execution
source_paths:
  - pkg/directorapi/licence.go
  - internal/director/target/target.go
  - internal/director/execute/pipeline.go
---

# ADR-202 — an instruction may license one action on a Prop only pixels describe

## Context

An architectural audit of the repository against the owner's Stage / Props / Actors / Director
model found that **almost all of it already exists**:

```
  Stage      providers.Collector -> observation.Cycle -> fusion.Engine -> directorapi.WorldState
  Props      directorapi.Element, fused across sources, with Provenance and Actions()
  Evidence   directorapi.Observation + ObservationSource
  Actors     uiact verbs and capability ladders -> plan -> execute -> directorapi.Actuator
  Director   internal/director/execute.Pipeline — observe, parse, resolve, plan, policy,
             execute, verify
  Memory     workflowstore.Artifact (durable) and SessionBinding (one session)
```

The interface for *"given this goal and the current Stage, which Prop can I act upon, with what
evidence?"* already exists too:
`target.Resolver.Resolve(*WorldState, ElementQuery) Resolution`, whose result carries the chosen
target, every rejected candidate with its reason, and the authoritative set to ask a person
about.

**The gap was not an interface. It was a permission.**
The rule lives in
`directorapi.ActuatingSource`: accessibility, a native integration, a DOM bridge and a capability
pack can say how to OPERATE a thing; a vision detector and an OCR reader describe pixels. So
`Actionability.Actuable` is false for an element only they saw, `Targetable()` refuses it, and
**every control in an application with no accessibility tree was unreachable except by
demonstration** — which lives in an entirely separate stack.

The owner's decision:

> An explicit user instruction MAY authorize one action on a pixel-only Prop without a prior
> demonstration. However, authorization and grounding must remain separate: the instruction
> authorizes the requested action, not a guessed target.

## Decision

**`directorapi.OperateLicence` is one request's permission to operate a Prop that only pixels
describe.** It is a fact about the REQUEST — the words a person used and the verb they named —
and it says nothing whatever about which Prop is meant.

`Element.OperableUnder(licence, verb)` is the whole of the new rule, and it is asked only of an
element `OnlyDescribesPixels()` already refused. Two conditions:

| | |
|---|---|
| the licence covers this verb | one verb, matched exactly: a licence to `click` does not license `type` |
| something described the OBJECT | `Provenance.DescribesAnObject()` — at least one non-OCR source |

The second is the lesson of [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] one tier
up. OCR reading "TRAINING" says a glyph run is at a rectangle, and the rectangle of a word is a
**caption**, not the control it labels — pressing the middle of one is how Marco came to click
17 px from the edge of a tile. A detector's box is a claim about a thing; a read word is a claim
about letters.

### What it deliberately cannot do

- **Resolve ambiguity.** Two Props matching the request refuse exactly as before. "Click Play"
  authorises a click; it does not say *which* play.
- **Supply grounding.** Geometry, visibility and on-screen-ness are asked where grounding is
  asked, and refuse there.
- **Reach policy.** Risk and destructiveness are evaluated after planning, from the action, and
  nothing in that evaluation can see a licence. "Click delete" is restricted exactly as it was.
- **Outlive the request.** It is a local value built inside one call from that call's own
  request. There is no field to store it in and nothing stores it.
- **Become an identity.** No workflow, no Prop memory, nothing durable.

### Where it is enforced, and why there

At **grounding**, in `target.score` — because that is the only place a pixel-only Prop was
refused, and policy was never reached. Threading it further would have duplicated the decision.

`Targetable()` is unchanged and `ActuatingSource` is unchanged. `Resolve` is unchanged and still
licenses nothing; `ResolveUnder` is the licensed sibling, and `Resolve` calls it with the zero
licence — so **every existing caller in the tree behaves exactly as it did**.

### And a refusal that was lying

`Targetable()` is the conjunction of grounding and permission, and reading it as one question
made a pixel-only control report `"not visible on screen"` — of a control that was plainly
visible, correctly matched and merely unvouched-for. A person told that goes looking for a
missing button. Grounding and permission are now asked apart and each answers for itself.

## Consequences

Measured through the real `execute.Pipeline` — observe, parse, resolve, plan, policy, execute,
verify — over fixture Stages:

```
  accessibility-backed "click File"            unchanged: resolves and acts
  pixel-only "click Play" (detector + OCR)     resolves and acts
  the same Prop with no licence                refused: "only pixels describe it, and
                                               nothing has said it may be operated"
  two Props named Play                         refused, nothing reaches the runtime
  a banner OCR alone read                      refused: "only read text describes it, which
                                               locates a word and not the control it is
                                               written on"
  that banner beside the real tile             the tile is chosen; the banner is not a contender
  no bounds / offscreen / not visible          refused AT GROUNDING, not later by the ladder
  a licence to click, asked about type         refused
  a second request                             carries nothing from the first
```

**What this does not do.** It does not unify the semantic and pixel-workflow stacks; the audit's
finding that `workflowrun.Runner` is a second pipeline stands, and is deliberately left alone.
It adds no perception, no screen identity and no application-specific rule. And it has not been
run against a live application — the fixtures are hand-built Stages, and the two-application
validation the owner asked for is the next step, not this one.

## Enforced by

`internal/director/execute` — `TestAnAccessibilityBackedClickIsUnaffectedByTheLicence`,
`TestAnExplicitInstructionAuthorisesOneActionOnAPixelOnlyProp`,
`TestWithoutAnInstructionAPixelOnlyPropIsStillRefused`,
`TestAnExplicitInstructionDoesNotResolveAmbiguity`,
`TestReadTextAloneIsNotEnoughToAuthoriseAClick`,
`TestAWordOnABannerIsNotAContenderAgainstTheControlThatSaysIt`,
`TestAPixelOnlyPropWithNoUsableGeometryIsRefused`,
`TestALicenceForOneVerbDoesNotCoverAnother`,
`TestALicenceIsNotCarriedIntoTheNextRequest`.

Six mutants run and all six killed: every pixel-only Prop made operable; read text accepted as a
clickable surface; OCR counted as describing the object; a licence covering any verb; the licence
never passed to the resolver; and geometry no longer required to locate.

## Related

- [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] — where the caption-is-not-the-
  control lesson comes from.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the other way a pixel-only
  control is reached, through a demonstration.
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — the three lifetimes; a licence
  is shorter than all of them.
