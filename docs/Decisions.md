---
type: index
status: active
updated: 2026-08-06
---

# Decisions

Architecture Decision Records. Each states a constraint, why it exists, what it costs, and
**what enforces it** — an implementation path plus the boundary or regression test that
fails if the constraint is broken.

A decision without an enforcement entry is an intention, not a constraint. If you find one,
either enforce it or downgrade it.

| ADR | Decision | Affects |
|---|---|---|
| [[ADR-001-observations-vs-belief]] | Observations are evidence; World State is belief | perception, fusion |
| [[ADR-002-fusion-owns-belief]] | Fusion alone converts evidence into belief | fusion, perception |
| [[ADR-003-evidence-authority-by-source]] | UIA may establish structure; OCR may establish visible text | perception, vision, privacy |
| [[ADR-004-vision-cannot-establish-actionability]] | Vision cannot establish actionability | vision, policy, safety |
| [[ADR-005-legal-marco-only]] | Every desktop effect lowers through legal Marco | execution, safety |
| [[ADR-006-unknown-is-not-false]] | Unknown is not false | control-flow, verification |
| [[ADR-007-no-progress-no-repetition]] | No verified progress means no repetition | execution, collections |
| [[ADR-008-no-stale-window-geometry]] | Stale window geometry is never reused | windows, capture |
| [[ADR-009-window-identity-is-ephemeral]] | Application identity may survive restart; window identity may not | windows, replay |
| [[ADR-010-passive-observation-cannot-execute]] | Passive observation has no reachable execution path | observation, safety |
| [[ADR-011-provenance-is-proven-not-assumed]] | Target provenance is proven by the provider, not assumed from the request | perception, fusion, windows |
| [[ADR-012-presence-is-state-relative]] | An element's presence is measured against the screen state it can exist in | passive-observation, perception |
| [[ADR-013-navigation-is-meaning-not-keys]] | A navigation observer records meaning; the physical key dies in the platform adapter | navigation, passive-observation, privacy |
| [[ADR-014-hypotheses-are-evidence-not-identity]] | A hypothesis carries its evidence and contradictions, and never branches on application identity | hypotheses, privacy, game-packs |
| [[ADR-015-a-question-is-evidence-not-settlement]] | Asking the user is evidence gathering; an answer never erases an observation, and a decline is not a denial | hypotheses, privacy |
| [[ADR-016-cross-session-identity-is-structural-and-conservative]] | Cross-session identity is structural and discrete; without a discriminator it refuses to recognise | semantic-memory, hypotheses, privacy |

## The shape of these

ADR-001 through ADR-005 are structural: they decide what the parts are and what may flow
between them. Breaking one requires redesigning a layer.

ADR-006 through ADR-012 are behavioural: they decide what the Director does when it is
uncertain. Every one of them resolves uncertainty toward *stopping* rather than *proceeding*,
which is the bias the whole system is built on — an automation tool that acts on a guess is
worse than one that asks.

ADR-013 is neither: it is a **privacy** constraint, and the first one whose enforcement is
about what a type can hold rather than about what the Director does. The distinction matters
when reading it — the other twelve can be honoured by careful behaviour, and this one cannot be,
because a promise not to record something is worth less than an inability to.

ADR-011 is the sharpest case of that bias: it decides what happens when a provider cannot say
what it saw, and answers *refuse*. An unproven claim and a disproven one are treated alike,
because the alternative — excusing silence — makes the guard optional for anything that
declines to answer.

## Adding an ADR

Number sequentially. Fill Context, Decision, Consequences, Enforced by. State the cost
honestly in Consequences — an ADR that lists only benefits is advocacy, and the next person
cannot weigh it.

Write the enforcement test first if one does not exist.

An enforcement entry must name a test that fails when the constraint is broken **in
production**, not only in the mechanism. A unit test over a mechanism nothing calls enforces
nothing — see [[Wiring-Tests]] for the two times that has happened here and the mutation gate
that catches it.

## Related

- [[Director]] — the system map
- [[Architecture]] — the layering these constraints protect
- [[AI-CONTEXT]] — how to retrieve these while working
