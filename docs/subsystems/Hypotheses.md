---
type: subsystem
status: active
owners:
  - director
depends_on:
  - passive-observation
  - navigation
used_by:
  - game-packs
updated: 2026-08-09
source_paths:
  - internal/director/observe/hypothesis.go
  - internal/director/observe/terms.go
  - internal/director/observe/proposal.go
  - internal/director/observesession/runner.go
  - cmd/director/observecmd.go
---

# Hypotheses

Turning a session's measurements into a sentence a person could act on, without turning them
into a claim the evidence does not support. It is the layer that answers *"what is Marco
learning while I play"* — and the layer most able to be confidently wrong, which is what its
whole design is arranged against.

It knows nothing about what application is running. See
[[ADR-014-hypotheses-are-evidence-not-identity]].

## Two generators, two bodies of evidence

Do not confuse them.

| | reads | asks |
|---|---|---|
| `observe.Insight` (`insight.go`) | `Findings` — the authoritative fused entity timeline | what did the world contain |
| **Hypothesis** (`hypothesis.go`) | `ShadowTotals` — screens, structures, navigation | what might these screens be |

Both reach the session `Result` and both are reported; they are never merged, because a
shadow-derived guess must not appear in a list a reader takes as belief-adjacent.

## The vocabulary

Seven interpretations, all prefixed `possible`, each grounded in evidence that actually exists:

| kind | needs |
|---|---|
| `possible_choice_group` | ≥3 aligned controls recurring together |
| `possible_menu_like_state` | a screen dominated by such a group |
| `possible_settings_like_state` | the above **plus** recurring configuration vocabulary |
| `possible_text_entry_state` | a control reporting itself editable (accessibility) |
| `possible_transition_action` | an intent that repeatedly preceded one change |
| `possible_reversible_place` | navigation evidence into **and** out of a screen |
| `possible_selection_sequence` | an ordered run that repeatedly preceded a change |

It is `possible_choice_group` rather than "navigable choice group" deliberately: navigation
through it is separate evidence that is often absent, and a mouse-driven settings list is a
choice group nobody was seen to navigate.

## What each source can and cannot establish

This table is the subsystem. Everything else is bookkeeping.

| source | can support | can never establish alone |
|---|---|---|
| structure (geometry) | a group, a menu-like screen | what the screen is *for* |
| text (OCR terms) | the settings/look-up specialisation | that a screen exists at all |
| navigation | an action related to a change | what the destination is |
| recurrence | that anything is real rather than a one-off | any interpretation by itself |
| accessibility | a text-entry control | — |

`supported` additionally requires **two independent sources** and ≥2 episodes. Two structural
facts about the same rectangle are one observation described twice.

`validated` is the fourth status and the only one involving a person: the observations agreed
AND the user was asked and agreed with them. It requires no contradictions — a confirmation does
not clear one. See [[ADR-015-a-question-is-evidence-not-settlement]].

## Contradiction is first-class

`classify` checks contradictions **before** support and cannot be outvoted by any amount of it.
`contested` is terminal. There is no confidence float, on purpose: 0.62 cannot distinguish thin
evidence from evidence that half points the other way.

Modelled: unattributed changes on an edge, a competing intent, a term that appeared in one visit
and never recurred, a single-episode structure or screen. All itemised in words, all rendered.

## The text boundary

`SemanticEvidenceFrom` reads labels the privacy classifier already released, matches **whole
words** against a closed vocabulary of generic interface concepts, and returns terms. The text
does not travel with the result — the same trade [[Navigation]] makes for key codes.

A typed username matches nothing in a vocabulary of interface concepts, so it cannot become
evidence: not by rule, but because there is nowhere to put it. `backpack` is not `back`;
`researcher` is not `search`; a redacted label is not consulted at all.

**The vocabulary is generic interface semantics, never a game's.** The test for adding a term is
whether a word processor could plausibly have it.

## Session-local ids are not identity

A hypothesis carries a `Fingerprint` — composition, normalised envelope, members, recurring
terms, recurrence. `state_3` appears only as a cross-reference into the same report, printed with
a note saying so. Cross-session identity is **not solved**; the representation just refuses to
let a counter become durable meaning.

## Where it is invoked

```
Runner.Run  → observe.Hypotheses(stats.Shadow, thresholds) → Result.Hypotheses
            → observationView → protocol → renderHypotheses (CLI)
Runner.LiveAnalysis → hypotheses recomputed for an in-progress session
observationRegistry.Get → an ACTIVE session generates from the evidence so far
```

## Related systems

- [[Passive-Observation]] — the screens and structures this interprets
- [[Navigation]] — the input evidence that gives the transitions meaning
- [[Game-Packs]] — where naming a hypothesis's subject will eventually belong

## Decisions

- [[ADR-014-hypotheses-are-evidence-not-identity]]
- [[ADR-015-a-question-is-evidence-not-settlement]]
- [[ADR-016-cross-session-identity-is-structural-and-conservative]]
- [[ADR-019-an-invitation-to-learn-is-not-a-correction]] — the second kind of question, and
  why its `no` is not a correction
- [[ADR-012-presence-is-state-relative]]
- [[ADR-013-navigation-is-meaning-not-keys]]
- [[ADR-004-vision-cannot-establish-actionability]]

## Validated by

Full list in [[ADR-014-hypotheses-are-evidence-not-identity]]. The three that matter most:

- `TestTheProductionSessionPathGeneratesHypotheses` — deleting the generator call from
  `Runner.Run` fails it and leaves every unit test green. Fourth mechanism in this subsystem to
  need such a test; see [[Wiring-Tests]].
- `TestHypothesesDoNotDependOnApplicationIdentity` — the generalisation property, asserted
  against `game.exe`.
- `TestGeometryAloneNeverNamesAScreen` — the ceiling that keeps the layer honest.

## Known gaps

- **Cross-session memory landed 2026-08-09.** A confirmed subject is recognised in a later
  session and the question is not repeated; see [[Semantic-Memory]]. It requires a discriminator
  — matching interface terms or a matching envelope — so a screen with no readable text is still
  a new subject every run.
- **The proposal loop landed 2026-08-09.** A `supported` hypothesis with no contradictions is put
  to the user as one short hedged question; the answer is recorded as evidence with provenance.
  See [[director-proposals]] and [[ADR-015-a-question-is-evidence-not-settlement]]. Still no
  capability: the output is a validated hypothesis and nothing more.
- **Vocabulary is English and small.** A foreign-language or heavily stylised interface produces
  structural hypotheses only. That is the intended failure; the remedy is a larger generic
  vocabulary or better accessibility, never a per-application table.
- **Confirmed live 2026-08-09** against an application Marco had never learned — one
  supported menu-like hypothesis, four contested, nothing named. See
  [[Experiment-008-unknown-game-discovery]]. Two limits it measured: navigation admission
  refuses nearly everything on a WASD-driven game (7 intents from 1086 events), and the group
  uniformity measure describes a vertical list rather than layout in general.
