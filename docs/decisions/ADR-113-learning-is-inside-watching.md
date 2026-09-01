---
type: decision
status: accepted
date: 2026-08-29
affects:
  - passive-observation
  - semantic-memory
  - editing
source_paths:
  - cmd/director/observelook.go
  - cmd/director/observeledger.go
  - cmd/director/observeambient.go
  - cmd/director/observepromote.go
  - cmd/director/sightplace.go
  - cmd/marco/edit.go
---

# ADR-113 — learning is inside watching, and naming a place you know is not a route

## Context

The first human dogfood ([[Experiment-022-the-first-dogfood]]) produced two complaints that turned
out to be one thing.

**The screens had no names.** The real store held forty-five durable Windows Settings places and
not a single `semantic` field, while `director name-probe` said `DESTINATION "Mouse"` on the same
desktop, at the same width, in the same session. The JUST LEARNED feed was telling the truth —
`learned place`, then a subject hash — because the Place genuinely had no name.

**Watch and Learn read as two peer switches.** The strip said `MARCO IS watching` with `Also learn`
beside it, and the CURRENT panel carried a *second* button also called Watch (Light Mode). A person
had to combine two switches in their head to work out what their assistant was doing, and the
vocabulary they were being asked to operate was the architecture's, not the product's.

### The naming rule was never wrong

Measured branch by branch: 37K's `ExplainPlaceName` is admission-identical to the
`AdmittedPlaceName` it replaced — every `continue` preserved, every bail-out preserved, and
`NameLevel` gates nothing. [[Experiment-020-what-does-this-screen-say-it-is]] measured the rule
working at 1500px. **The defect was entirely in the wiring**, and this is the fifth time on this
branch that a correct mechanism has been reached by nothing a person uses.

The path, as it was:

```
AdmittedPlaceName → ScreenState.PlaceNames[n]++ → settledPlaceName → …nothing
```

Ambient watching's ONLY naming write lived inside `promotion.establish`, which fires once, at the
instant a Place is created, from whatever the transient shape happened to carry. Two consequences,
both fatal, both measured:

- **A name settles later than a Place is established.** The two almost never coincide — a Place is
  established the first time Marco can recognise it, a name settles by RECURRENCE. A Place made
  durable before its word settled stayed unnamed forever, and `ambientLook` returns early for an
  established place, so there was not even a shape carrying a word to write.
- **`establish` is only reachable by edge promotion**, which needs an attributed human action
  between two screens. A screen somebody simply went to could not be named at all.

`Runner.establishPlace` had already found and fixed exactly this defect one layer up — its own
comment describes it — by sweeping `PlaceNamesToRecord` separately from establishing. Ambient
watching never got the same sweep.

## Decision

### Naming an established Place is its own operation

Ambient watching, when learning is on, runs the canonical naming sweep on every reading:
`observe.PlaceNamesToRecord`, unchanged, over every state the session has seen. It is computed in
`ambientLook` **above** the established-place early return, because an already-established Place is
precisely the case that needs it.

It needs **no walk, no edge, no goal, no explicit Learn, no second Place and no desktop input**. It
cannot mint anything: `PlaceNamesToRecord` skips a state memory cannot recall, and
`ObserveSemanticName` refuses a subject the store does not hold — so enrichment can never fork an
identity into `subj_ABC` unnamed beside `subj_XYZ` called Mouse.

**Settlement is untouched.** `director name-probe` reads one sample; production requires recurrence
and refuses a tie, and it still does. The dogfood defect was that a settled name never reached
persistence — not evidence that settlement should go. A screen that says two different things
equally often is still Marco not knowing.

The naming rule itself — `ExplainPlaceName`, `AdmittedPlaceName`, `placeNameEvidence`, breadcrumbs,
selected navigation, topology, responsive matching — is **unchanged**. The known-wrong cases
(collapsed Settings, Printers at 850px) remain known wrong, and persistence now reflects the
existing naming contract faithfully rather than reflecting nothing at all.

### One naming door, one licence check

`promotion.call` is the only path from a promotion to a semantic name, and `promotion.establish`
goes through it too. It hangs off `EstablishPlaces` — the permission that is already about Places —
because one word against an identity that already exists is not route acquisition, and tying it to
`AcquireRouteEvidence` is what made it need an edge in the first place.

### LEARN is inside WATCH

Three product states and no fourth:

```
NOT WATCHING          →  Watch          →  WATCHING
                      →  Watch & Learn  →  WATCHING & LEARNING
WATCHING              →  Learn          →  WATCHING & LEARNING
WATCHING & LEARNING   →  Stop learning  →  WATCHING
WATCHING (either)     →  Stop watching  →  NOT WATCHING
```

`DisableAmbient` now clears the learning policy with it. Learning is a permission to remember what
Marco SEES; with nothing being seen it governs nothing, and a status reporting `watching: no,
learning: yes` is a state no person can act on. The asymmetry is deliberate and unchanged in the
other direction: switching learning off asks for less memory, not less attention.

The internal permissions stay separate — [[ADR-095-repeated-observation-may-become-knowledge]] is
not reversed. What changed is that the product stopped asking a person to operate them.

### CURRENT is what Marco sees; JUST LEARNED is what it wrote down

The distinction that replaces explaining Observe and Learn as mechanisms, and it is taught by
behaviour rather than by documentation:

| | watching | watching and learning |
|---|---|---|
| CURRENT | this screen says it is "Mouse" | this screen says it is "Mouse" |
| JUST LEARNED | — | `named Mouse` |

`HerePlace.Perceived` carries the settled reading of the screen in front, through
`observe.SettledPlaceNameFor` — the same recurrence rule that decides whether a word may become
durable, asked as a question and written nowhere. It is rendered as its own labelled line and is
**not** a second naming function: `PlaceWords` remains the one thing that says what a place is
called, which `TestEverySurfaceNamesAPlaceTheSameWay` holds.

The feed keeps its own vocabulary. A Place gaining a word announces `named`, not `learned` — the
identity was already there — and `ObserveSemanticName` is idempotent, so a sweep that runs on every
reading says nothing after the first write.

### And the second Watch button is gone

The Here view carried Light Mode's own Watch beside the ambient strip's. Both started observation,
both were called Watch, and they reported different state. The strip's Watch keeps a session
running, which is what the CURRENT panel needs, so the duplicate cost a person clarity and bought
nothing.

## Consequences

- A screen somebody merely visits gains its name under Watch & Learn. The forty-five unnamed
  Settings places in the real store will be named as they are revisited.
- Watch alone remains zero durable learning, and the difference is now VISIBLE: it perceives the
  name, reports it, and writes nothing.
- The naming sweep runs per reading. It is cheap — a state with no settled name costs a map walk,
  and the store's idempotent return means the file is written once.
- `dogfood.cmd` points `MARCO_HOME` at an isolated store so the graph can be reset freely. There is
  no sandbox-only semantic behaviour; the isolation is one environment variable.

## Enforced by

- `cmd/director` `TestWatchingAndLearningNamesAPlaceItAlreadyKnows` — the production-path test:
  `EnableAmbientLearning`, one supervisor reading, assertion against the reopened file
- `cmd/director` `TestWatchingAloneNamesNothing`
- `cmd/director` `TestNamingAPlaceNeedsNoWalkOrEdge` — no edge, no candidate, no second Place
- `cmd/director` `TestAnUnsettledNameIsNotWrittenDownByWatching`
- `cmd/director` `TestTheFeedSaysAPlaceWasNamedOnceItActuallyWas`
- `cmd/director` `TestStoppingWatchingStopsLearning`, `TestTurningLearningOffLeavesMarcoWatching`
- `cmd/director` `TestWhatTheScreenSaysItIsComesFromWatchingNotMemory`,
  `TestWatchingShowsWhatTheScreenSaysItIs`
- `cmd/director` `TestObserveCannotMakeItsOwnEvidenceDurable` — now covering the naming door
- `cmd/marco` `TestTheStripNamesThreeProductStates`,
  `TestTheStripHasNoLearningWithoutWatchingState`
- `cmd/marco` `TestTheHereViewHasOneWatchControl`,
  `TestThePageSaysWhichQuestionEachPanelAnswers`
- `cmd/marco` `TestEveryDoorThePageKnocksOnIsAnswered`

## Related

- [[ADR-112-the-loop-belongs-where-a-person-is-already-looking]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]
- [[Experiment-022-the-first-dogfood]]
- [[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]]
