---
type: decision
status: accepted
date: 2026-08-19
supersedes: []
affects:
  - semantic-memory
  - perception
  - demonstrations
source_paths:
  - internal/director/observe/sample.go
  - internal/director/observe/remember.go
  - internal/director/observe/learning.go
  - pkg/directorapi/world.go
---

# ADR-076 — a Place may say what it appears to be called

The Audience was being asked to read this and work out which screen it meant:

```
Route: about back, settings, 148 things on it → about back, settings, 96 things on it
```

That string is a **diagnostic description** being asked to do the job of a **name**. It is good
evidence — it says exactly why Marco thinks two Places differ — and it is not something a person
should have to decode to answer a question.

## Three concepts, kept apart

**Audience name** (`RememberedSubject.Called`) — a person offered this word. The one durable string
in the system that comes from a person rather than from perception, and highest authority.

**Semantic name** (`RememberedSubject.Semantic`, with `SemanticFrom`) — what the Place appears to be
called, from what an Actor perceived. An inference, and recorded as one.

**Structural description** (`DescribeStructure`) — what the screen is made of. Diagnostics, and the
floor when there is no name.

Writing an inference into `Called` would make Marco indistinguishable from the Audience in its own
records, and every later reader — including the ones deciding whether a play may say a name out
loud — would lose the ability to tell what somebody said from what Marco guessed. The privacy
boundary is only a boundary while exactly one field is on the human side of it.

## Measured before it was designed

Five live applications, dumped and read before this rule existed:

| Application | Window title | Selected navigable |
|---|---|---|
| Settings | `"Settings"` — **on every page** | `list_item "Home"` |
| Chrome | `"Marco - Marco Director - Google Chrome"` | `tab "Marco - Marco Director"` |
| Discord | `"@BeeTeaSea - Discord"` | `tree_item "Direct Messages"` |
| VS Code | `"localhost:8765 - marco - Visual Studio Code"` | **three** selected at once |
| Spotify | `"Trojans • Atlas Genius"` | none |

**The window title is not the signal.** In most applications it is `<place> - <app>`, so using it
means stripping an application suffix — the exact application-baking a Place name must not do — and
in Settings it is the application name on every page, discriminating nothing.

**The selected navigable item is the signal.** It is short, noun-like, and it is what changes as
somebody walks around.

## The rule

**Exactly one admissible selected navigable item names the Place. Zero or several name nothing.**

- Selected, and a **navigable** role — a list item, tree item, tab, menu item or row. A button is
  something you press; it never reports itself as the selected one of its siblings, and the role
  check says so structurally rather than relying on that. `Back` and `Close` cannot reach the rule.
- Not inside a **value chooser**. Settings Home reports two selected items: `Home` in the
  navigation pane and `Dark` inside the Color-mode combo box. One says where you are; the other
  says what a setting is set to.
- Two Actors saying the same word strengthen **one** hypothesis. Two Actors saying different words
  produce **no** name — Actor disagreement may never become confident truth.

VS Code offers three selections and Spotify none, so silence is a first-class outcome. Ranking by
depth would have named VS Code `Explorer (Ctrl+Shift+E)` — a keyboard hint, not a place. **A missing
name costs a line of diagnostics; a confident wrong one is trusted.**

Nothing in a name may come from element counts, geometry, runtime ids, window handles or provider
node ids. Those are evidence.

## The licence

The same one a semantic target's label rides: an **explicit Learn demonstration**. A Place's name is
read off somebody's screen, and passive observation has no business writing that down — the
argument is `AdmittedTargetLabel`'s argument, and the policy lives beside it, in one site. The shape
filter is unconditional either way: a friend tag, an address or a URL is refused whatever the
licence says.

## Presentation

`observe.PlaceWords` is already the one Audience-facing wording function, read by HERE, the recent
trail, the route line, rehearsal questions and naming questions. The order is now:

1. Audience name — a person's word, always wins
2. semantic name — what Marco worked out
3. application context
4. structural description — the floor, which always says something

An **unname** removes only the Audience's word; the grounded inference is still shown. Place
identity, targets and relationships are untouched by any of it — a name is presentation and
semantic grounding, never identity. Two Places inferring the same word remain two Places;
recognition is unchanged and semantic-name equality is not identity equality.

## Enforced by

- `internal/director/observe/placename_test.go` — `TestTheSelectedDestinationNamesThePlace`;
  `TestASelectedValueDoesNotNameThePlace`; `TestSeveralSelectedItemsNameNothing`;
  `TestTwoActorsAgreeingStrengthenOneName`; `TestAControlCannotNameThePlace`;
  `TestPassiveObservationInfersNoName`; `TestTheLicenceDoesNotAdmitPrivateText`;
  `TestAnAudienceNameBeatsAnInferredName`; `TestAnInferredNameBeatsTheStructuralDescription`;
  `TestWithNoNameTheDescriptionIsStillShown`; `TestAnInferredNameCarriesNoEvidence`;
  `TestAnInferredNameIsNotRecordedAsSomethingSomebodySaid`
- `cmd/director/establishwiring_test.go` — `TestAPlaceEstablishedThroughTheProductionPassIsNamed`
  (the whole chain, through the production pass, read back from disk);
  `TestAnEstablishedPlaceKeepsWhatItAppearedToBeCalled`;
  `TestAnInferredNameNeverOverwritesTheAudiences`
- `cmd/director/observeguard_test.go` — `TestTheSamplerNamesThePlaceOnlyUnderTheLicence`;
  `TestADemonstrationOffersThePlaceItsName`; `TestASelectedValueIsNotOfferedAsAPlaceName`
- `internal/director/observe/placename_test.go` — `TestANameSeenOnceDoesNotStick`;
  `TestATiedNameIsLeftUnresolved`; `TestTheMostRecurrentNameWins`;
  `TestActorsDisagreeingProduceNoName`
- `cmd/nameprobe` — the measurement tool the rule was derived from, kept so the next application
  can be measured rather than guessed at

## The chain, and the hop that was silently dropping it

Actor evidence → `liveSampler.placeName` → `SemanticEvidence.PlaceName` → `ScreenState.PlaceNames`
(tallied) → `PlaceCandidate.SemanticName` (promoted by recurrence) → `Store.ObserveSemanticName`.

Six hops. Every one of them passed its own unit test while the chain was **not connected**:
`admissibleTerms` rebuilds the evidence field by field, so a constructor that only knew about terms
dropped the name on the way into the screen state — invisible from both ends. The end-to-end test
that enters through `RunPass` and reads the file afterwards found it on its first run, which is why
it exists.

The tally follows ADR-073: a word seen once is a transition frame, a word that recurs is the
screen.s, and a tie is left unresolved rather than decided by map order.

One line remains ungated by a test — `sem.PlaceName = named` inside `liveSampler.Sample`, which no
fixture drives because it needs a collector and a fusion engine. Both sides of it are gated.

## Related

[[ADR-073-a-place-is-a-composition-marco-saw]] · [[ADR-069-a-name-is-authored-and-can-be-taken-back]] ·
[[ADR-031-the-user-names-the-stage]] · [[Semantic-Memory]] · [[Perception]]
