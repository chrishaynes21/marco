---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - hypotheses
  - passive-observation
  - navigation
  - game-packs
---

# A hypothesis carries its evidence, its contradictions, and no application's name

## Context

Everything below this layer measures. A screen recurred nine times; five controls appear
together at a normalised envelope; `pause` preceded a change in 9 of 9 observations; the word
SETTINGS was read in every inference of one screen. None of that is an interpretation, and none
of it is useful to a person as it stands.

The product goal is that Marco watches an application it has **never been taught**, and
eventually offers something like *"I think this is a settings screen, and you open it a lot —
want me to learn how to get here?"* The path from measurements to that sentence is short, and
that is the danger: a plausible interpretation is far easier to produce than to justify.

Three specific failure modes were identified before any code was written.

1. **A generator that only accumulates support will eventually believe everything.** Every
   additional observation makes every hypothesis stronger, and nothing ever gets weaker.
2. **Geometry is nearly meaningless on its own.** Four evenly spaced buttons are a settings
   screen, a level select, an inventory, a save-file list and a difficulty picker. A layer that
   named screens from arrangement would be confidently wrong most of the time, in a way no
   downstream consumer could detect.
3. **The obvious shortcut is a per-game dictionary**, and it is not a shortcut. `if title
   contains "Rocket League" { ... }` produces impressive demos and zero generalisation, and once
   one exists every new application is a code change.

## Decision

**A hypothesis is a record of evidence with an interpretation attached, not a conclusion.** It
carries what was observed, what supports it, what argues against it, how many independent
episodes it rests on, how to settle it, and a status from a three-value vocabulary.

### Contradiction is checked first and cannot be outvoted

`classify` returns `contested` if any contradiction exists, before it looks at support at all.
That ordering is the entire difference between a hypothesis generator and a confirmation-bias
engine. A contested hypothesis is a terminal state, not a waypoint: more supporting observations
do not clear it.

Contradictions are **itemised, never netted off**. There is deliberately no confidence float: a
score of 0.62 cannot distinguish "thin evidence" from "half the evidence points the other way",
and those call for opposite responses.

Modelled contradictions include: the same change happening with no navigation observed before it
at all; a second intent competing for the same change; an interface concept that appeared in one
visit and never recurred with the screen; a structure or a screen seen in only one episode.

### `supported` requires independent sources

Two structural facts about the same rectangle are one observation described twice. `supported`
requires evidence from at least two of `structure`, `recurrence`, `text`, `navigation`,
`accessibility` — and at least `MinEpisodes` (2) independent recurrences, because a transition
frame and a screen are indistinguishable until one of them comes back.

### Each source has a ceiling, and the ceilings are enforced

- **Geometry alone never names a screen.** A recurring group of aligned controls supports
  `possible_choice_group` and `possible_menu_like_state`, and nothing further. The settings
  interpretation additionally requires *text*.
- **Text alone never names a screen.** OCR reads HUDs, scoreboards and chat constantly. The word
  SETTINGS on a screen with no grouped controls supports nothing.
- **Navigation alone never names a screen.** That an intent preceded a change says an action and
  a change are related; it says nothing whatever about what the destination is.
- **Text-entry requires an accessibility role.** A bordered rectangle and a search box are the
  same picture, so `possible_text_entry_state` rests on a control reporting itself editable and
  never on shape.

### Nothing branches on application identity

The generator's whole input is `ShadowTotals`: role counts, normalised geometry, recurrence
counts, closed-vocabulary interface terms, navigation intents. The executable, the window title
and the application name are not reachable from it. Identical evidence from an unknown
`game.exe` produces identical hypotheses — asserted directly, including that no hypothesis
quotes an application name anywhere in its prose.

Application identity remains on the session for **provenance** — which session watched what —
which is a different question from what the evidence means.

### OCR text is classified to meaning at the boundary and then discarded

The same trade [[ADR-013-navigation-is-meaning-not-keys]] makes for keys. `SemanticEvidenceFrom`
reads labels the privacy classifier already released in the clear, matches **whole words**
against a closed vocabulary of generic interface concepts, and returns terms. The label text does
not travel with the result.

This is what makes the privacy property structural rather than procedural: a typed username
matches nothing in a vocabulary of interface concepts, so it cannot become semantic evidence —
not because a rule forbids it, but because there is nowhere to put it. Matching is word-level, so
`backpack` is not `back` and `researcher` is not `search`. A redacted label is not consulted at
all; this layer does not get a second opinion on the classifier's decision.

The vocabulary is *generic interface semantics*, and the test for adding a term is whether a
word processor could plausibly have it. `settings`, `audio`, `search`, `back` pass. Anything that
only makes sense in one title does not.

### Session-local ids are never identity

`state_3` is a counter; the same screen is `state_1` in the next run. A hypothesis carries a
`Fingerprint` — composition, normalised envelope, member count, recurring terms, recurrence
count — and the session-local ref only as a cross-reference into the same report, printed with a
note that it means nothing outside that session.

Cross-session identity is **not solved here**. No matcher exists. The representation is arranged
so that building one later does not require rewriting recorded hypotheses, and so that nobody
can accidentally persist `state_3` as though it meant something.

### The layer ends at hypotheses

No execution authority, no input replay, no generated Marco, no capability persistence, no
automatic action. Hypotheses reach nothing authoritative — the same isolation
[[ADR-012-presence-is-state-relative]] gives screen states.

## Consequences

Marco can now say, of an application nobody has told it anything about: this screen recurs, it is
a set of choices, the player reaches it with one action and leaves with another, its text
repeatedly uses configuration concepts — and here is what argues against each of those.

It cannot say what the screen is *for* beyond that generic vocabulary, and it will not try. The
honest weak claim survives where the strong one fails: a recurring panel with no readable text
stays `possible_menu_like_state` rather than being promoted on geometry.

The cost is real and worth stating: an application whose menus carry no text this vocabulary
recognises — a foreign-language interface, a heavily stylised game, a screen of icons — produces
only structural hypotheses. That is the intended failure. The remedy is a larger generic
vocabulary or better accessibility, never a per-application table.

## Enforced by

- `TestTheProductionSessionPathGeneratesHypotheses` — THE wiring test. Deleting the generator
  call from `Runner.Run` fails it; unit tests over the generator stay green.
- `TestHypothesesDoNotDependOnApplicationIdentity` — identical evidence under three different
  application names, including `game.exe`, produces byte-identical hypotheses, and no prose
  mentions the application.
- `TestGeometryAloneNeverNamesAScreen` — the same structure with and without vocabulary; only
  the second is settings-like, and the first still yields the honest weaker claim.
- `TestTextAloneNeverNamesAScreen` / `TestNavigationAloneNeverNamesAScreen` — the other two
  ceilings.
- `TestUnattributedTransitionsContestATransitionAction` — 3/3 is `supported`, 3/5 is
  `contested`, and the counter-example is readable in words.
- `TestCompetingIntentsContestAnActionAndBothRemainVisible` — the losing intent survives.
- `TestATransientWordDoesNotBecomeSemanticIdentity` — a word in 1 of 5 visits never reaches
  `supported`.
- `TestOneVisitIsNeverSupported` / `TestAOneOffArrangementIsContestedNotSupported`.
- `TestSupportedStatusRequiresIndependentSources` — no promotion on one kind of evidence.
- `TestATypedNameCannotBecomeAnInterfaceTerm` / `TestARedactedLabelIsNotMatched` /
  `TestVocabularyMatchingIsWordLevel` — the text boundary.
- `TestAHypothesisCarriesAFingerprintAndNotJustASessionLocalID` — no accidental durable identity.
- `TestHypothesesReachNothingAuthoritative` — the isolation guard.
- `TestTheCompositionRootTurnsLabelsIntoInterfaceTerms` — the OCR boundary is actually invoked.
- `TestACapturedTraceReplaysToTheSameHypotheses` — production and replay agree on interpretation.
- `TestTheRenderedSessionReportShowsHypothesesAndTheirContradictions` — the last leg: it reaches
  the page a person reads, contradictions included.
