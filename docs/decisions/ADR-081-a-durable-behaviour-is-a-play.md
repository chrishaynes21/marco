---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - plays
  - learned-plays
  - visibility
source_paths:
  - internal/plays
  - internal/routes/origin.go
  - internal/orchestrator/authority.go
  - cmd/marco/assistant.go
  - cmd/marco/edit.go
---

# ADR-081 — a durable behaviour is a Play

## The word that meant six things

[[34F-legacy-marco-product-audit]] §2.3 traced every use of "route" through the code and found
**six** senses wearing one word:

| sense | mechanism | is it really a route? |
|---|---|---|
| the user's phrase | `Registry.Resolve(app, name)` | no — a binding from words to a behaviour |
| a reusable behaviour | the `.marco` file | **yes — this is a Play** |
| a graph path | `Origin.From`/`To`, `director reach` edges | no — a Director edge |
| compiled Marco | `driver.CheckSource` output | no — a language artifact |
| a hotkey binding | `routes.Binding` | no — a trigger |
| an action sequence | `macroir.Step[]` | no — an intermediate representation |

Three of the six are distinct product nouns: **Play**, **Binding**, **edge**. The other three are
implementation layers that were never routes at all. A person meets all six under one word, and the
word tells them nothing about which one they are holding — the file they can edit, the phrase that
finds it, or the plan the Director recomputes on every read.

## The decision

**The product noun for a durable behaviour is a PLAY**: legal Core Marco on disk, with a past.
What reaches one — a phrase, a scope, a hotkey — is a **Binding**. What the Director computes over
edges is a **plan**, and it is neither.

**`internal/routes` keeps its name**, and so do `Registry.List`, `Registry.Resolve`, `marco routes`,
the `routes/` directory and `$MARCO_ROUTES`. Product vocabulary and implementation vocabulary are
allowed to differ.

**`internal/plays` is the seam.** One package, holding the projection AND the product wording:
`List` / `Registered` / `Staged` / `Find`, `ScopeOf`, `LifeOf`, `AfterLearn`, `KindWord`. It
computes no second account of anything — every field is a rendering of something `internal/routes`
already knows.

### The consequences that were designed for, not discovered

- **Kind and provenance come from one decider.** `routes.Registry.KindOf` is what
  `orchestrator.Classify` reads on the way to the authority door, and what the listing reads to draw
  a badge. A surface therefore cannot call a play *learned* that the door calls *authored*.
- **Registered is not a stored flag.** It is which of two enumerations a row came out of.
  `Registry.List` is what the resolver can see; `ListStaged` is what it structurally cannot; they
  are joined in `plays.List` and NOWHERE lower down, because `Resolve` walks `List` and widening it
  would make every staged play answerable from every application.
- **`KindTaught` presents as "Recorded".** [[ADR-048-learn-teach-and-do-are-three-different-sentences]]
  reserves *Teach* for Marco guiding the person — the opposite direction of travel — and that
  feature is a short step from the visual grounding that already exists. Spending the word twice
  would leave it with no name. *Recorded* says what actually happened.
- **Listing is read-only.** Every call under `plays.List` is `os.ReadDir`, `os.ReadFile` or
  `os.Stat`; browsing creates no directory and writes no file.

## Considered and rejected

- **Rename the Go package to `internal/plays` and move the store into it.** The diff crosses
  `cmd/marco`, `cmd/director` and every test, and the benefit to a person who never sees a package
  name is zero. Worse, it destroys the property that makes the rest of this phase reviewable — the
  ability to tell a behaviour change from a spelling change. The audit recommended against it
  explicitly ([[34F-legacy-marco-product-audit]] §8, *"do not rename the Go package"*), and the
  repository already runs this exact divergence for Learn/`teach` without confusing anybody.
- **Keep "Route" as the product word and simply document the six senses.** Documentation cannot fix
  a word that names a file, a phrase, a hotkey and a plan at once; the person still has to hold the
  disambiguation in their head at first contact, which is when they are least equipped to.
- **Call a Director plan a Route.** A plan over verified edges is recomputed on every read, holds no
  authority and has no file. [[ADR-056-a-goal-is-a-destination-not-a-route]] already says a goal is a
  destination and not a route; naming the plan Route would resurrect precisely the confusion being
  removed.
- **Make `marco routes` an alias for `marco plays`.** They answer different questions and both
  answers are wanted. `marco routes` is what may be OFFERED — a front end that printed a staged name
  would be advertising a capability `marco do` cannot find — and `marco plays` is what a person HAS.
  One command with a flag would have made the safe answer the one you have to remember to ask for.

## Consequences, including the costs

- **Two vocabularies now coexist in one repository, permanently.** A reader will meet `Route`,
  `Registry`, `routesDir()` and `$MARCO_ROUTES` in code whose product surface says Play. That is a
  real tax on every future session, paid to avoid a rename that buys nothing; [[Plays]] and
  [[Glossary]] exist so the tax is charged once.
- **Two enumerations must both be called** to answer "what do I have". Any future surface that calls
  only `Registry.List` will silently omit staged plays. `plays.List` is the one that is complete, and
  `plays.Registered` is the one that is safe to offer — the names are the whole warning.
- **`marco routes` cannot grow.** Its result set and its first four JSON keys (`name`, `slug`, `app`,
  `scope`) are a published contract for out-of-module consumers. Keys may be added and never renamed.
- **The projection can drift from the store** if somebody re-derives a fact locally instead of
  calling it. The two known cases are already fixed and pinned: a second `scopeOf` in `cmd/marco/edit.go`
  that tested `Focus` before `App`, and a Learn panel that composed its own status sentence.

## Enforced by

- `internal/plays` — `TestStagedPlaysAreListedAndStayUnresolvable` (the join is in the projection,
  and a staged play still resolves to nothing), `TestEveryKindIsPresentedAsItself`,
  `TestARecordedPlayIsNotShownAsAuthored`, `TestNoSurfaceCallsAStagedPlayReady`,
  `TestFocusIsPresentedDistinctlyFromContext`,
  `TestTheDoorAndTheListingAgreeAboutEveryProvenance` (the listing and
  `orchestrator.Classify` are checked against each other for every provenance state),
  `TestBrowsingPlaysChangesNothingOnDisk`, `TestABindingReachesAPlayWithoutBecomingOne`,
  `TestLearnAndThePlaysListingSayTheSameThing`.
- `internal/routes` — `TestStagedPlaysAreListedAndStayUnresolvable` (the enumeration itself),
  `TestKindOfDoesNotBelieveAnUnreadableSidecar`.
- `cmd/marco` — `TestMarcoRoutesOffersOnlyPlaysThatCanAnswer` and
  `TestMarcoPlaysShowsTheSavedPlayMarcoRoutesMustNotOffer` (the two commands answer two questions),
  `TestMarcoRoutesJSONKeepsItsPublishedKeys`, `TestTheUnknownCommandErrorIsOnePrefix`,
  `TestThePlaysListingShowsRegisteredAndStagedPlaysDifferently`,
  `TestThePlaysTabIsLabelledPlaysOverTheRoutesIdentifier`, `TestMarcoUiPlaysOpensThePlaysView`,
  `TestLearnReportsTheSameStandingThePlaysListWould`.
- `plugins/overlay` — `TestNoPlayMatchesPrefix` (the suppression constant is the engine's literal).

## Related

[[Plays]] · [[Learned-Plays]] · [[34F-legacy-marco-product-audit]] ·
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]] ·
[[ADR-080-a-learned-play-is-asked-for-from-anywhere]] ·
[[ADR-082-a-plays-past-travels-with-the-file]] · [[Glossary]]
