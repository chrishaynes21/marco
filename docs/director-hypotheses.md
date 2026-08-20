---
type: milestone
status: complete
date: 2026-08-09
affects:
  - hypotheses
  - passive-observation
  - navigation
source_paths:
  - internal/director/observe/hypothesis.go
  - internal/director/observe/terms.go
  - internal/director/observesession/runner.go
  - cmd/director/observewiring.go
  - cmd/director/observecmd.go
---

# Semantic hypothesis generation, and the OCR boundary that makes it safe

Canonical notes: [[Hypotheses]], [[ADR-014-hypotheses-are-evidence-not-identity]].

## What the audit found before anything was written

The audit was the most valuable hour of the milestone, and it changed the design.

`observe.Insight` **already existed** as a well-shaped hypothesis type — kind, confidence,
supporting samples, contradictions, a required validation step — and it **was** production-wired,
at three call sites in `runner.go`. It reads `Findings`: the authoritative fused entity timeline.

Everything built in the previous four milestones — screen states, state-local track eligibility,
structural groups, transition edges, navigation correlation, ordered runs — lives in
`ShadowTotals`, and **no generator read any of it**. Two parallel worlds, one with an
interpreter and no discovery evidence, the other with all the discovery evidence and no
interpreter.

The second finding decided the architecture: **`ShadowRegion` carries no text.** Role, geometry,
confidence, nameable flag. OCR and accessibility labels live on the authoritative side as
`SafeLabel`. So multi-source hypotheses required joining the two worlds, and the join had to
happen somewhere text was still in scope.

`observe.Sample` carries both `Entities` (with labels) and `Shadow` — that is the seam, and it is
where the boundary went.

## The design decision the milestone turns on

**Classify text to meaning at the boundary and discard the text**, exactly as
[[ADR-013-navigation-is-meaning-not-keys]] does for key codes.

`SemanticEvidenceFrom` reads labels the privacy classifier already released, matches whole words
against a closed vocabulary of *generic interface concepts* (`settings`, `controls`, `audio`,
`search`, `invite`, `back` …), and returns terms. The label text does not travel with the result.

This is what makes the privacy property structural instead of procedural. A typed username
matches nothing in a vocabulary of interface concepts, so it cannot become semantic evidence —
not because a rule forbids it, but because there is nowhere to put it. The eventual capability
`invite user <name>` takes the name as a runtime parameter; discovery never needed it.

It also answers the game-specificity problem directly. The vocabulary is *interface* vocabulary,
and the test for adding a term is whether a word processor could plausibly have it. There is no
Schedule I dictionary, no Rocket League dictionary, and no mapping from any application's menu
layout to meaning — only compositional evidence:

```
recurring screen + stable choice group + concept "controls" in 25/25 + "settings" in 25/25
  → possible_settings_like_state, supported
```

## Ceilings, which are the actual product

Each source can support some claims and not others, and the limits are enforced by tests rather
than by intention:

- **Geometry alone never names a screen.** Four aligned buttons are a settings screen, a level
  select, an inventory and a save-file list. It supports `possible_choice_group` and
  `possible_menu_like_state` and stops there.
- **Text alone never names a screen.** OCR reads HUDs, scoreboards and chat constantly.
- **Navigation alone never names a screen.** That `pause` preceded a change relates an action to
  a change and says nothing about the destination.
- **Text entry needs an accessibility role**, never a shape — a bordered rectangle and a search
  box are the same picture.

`TestGeometryAloneNeverNamesAScreen` runs the same session twice, identical in structure and
navigation, differing only in whether the interface had readable words. Only the second is
settings-like; the first still produces the honest weaker claim.

## Contradiction is checked first

`classify` returns `contested` if any contradiction exists, **before** looking at support.
`contested` is terminal — more supporting observations do not clear it. There is deliberately no
confidence float: 0.62 cannot distinguish thin evidence from evidence that half points the other
way, and those call for opposite responses.

Measured on the scripted session: `pause` before a change in 3 of 3 is `supported`; the same
edge after two more arrivals nobody was seen to cause is `contested`, with *"the same change
happened 2 time(s) with no navigation observed before it at all"* printed in words beside it.

## What a session now says

From a scripted run of an application with no name attached to it:

```
[supported] possible_reversible_place (state_2)
  observed: the player went here after "pause" and left after "back", 9 time(s) in and 8 out
  + navigation: "pause" preceded arriving in 9 of 9 observations
  + navigation: "back" preceded leaving in 8 of 8 observations
  + recurrence: the screen was visited 9 separate times

[supported] possible_settings_like_state (state_2)
  observed: a recurring screen of grouped controls whose text repeatedly used configuration
            concepts (controls, settings)
  + structure: 4 controls presented as a set
  + text: the concept "controls" in 25 of this screen's 25 observations, across 9 visit(s)
  + text: the concept "settings" in 25 of this screen's 25 observations, across 9 visit(s)
```

## The mutation gate

Part 9 was declared a hard gate, and it was needed. Six mutations, each killing its test:

| mutation | test that fails |
|---|---|
| delete `observe.Hypotheses(...)` from `Runner.Run` | `TestTheProductionSessionPathGeneratesHypotheses` (+2 more) |
| stop turning labels into terms at the composition root | `TestTheCompositionRootTurnsLabelsIntoInterfaceTerms` |
| stop crediting terms to the state they were read in | `TestGeometryAloneNeverNamesAScreen`, `...GeneratesHypotheses` |
| let support outvote contradiction in `classify` | `TestUnattributedTransitionsContest...`, `TestCompetingIntents...` |
| stop recording semantic evidence into the trace | `TestACapturedTraceReplaysToTheSameHypotheses` (+1) |
| let geometry alone name a settings screen | `TestGeometryAloneNeverNamesAScreen` |

## Known gaps

- **No cross-session identity.** Fingerprints exist and nothing consumes them. A screen
  rediscovered next session is a new subject.
- **No proposal loop.** The milestone ends at hypotheses, deliberately.
- **The vocabulary is English and small**, so a foreign-language or heavily stylised interface
  produces structural hypotheses only. Intended; the remedy is a larger *generic* vocabulary,
  never a per-application table.
- **Sub-sample ordering** is still unavailable, inherited from [[Navigation]].
- **Not confirmed live.** Everything here is proven by scripted sessions, mutation and trace
  replay. No session has been run against a real unknown application while somebody played it.
