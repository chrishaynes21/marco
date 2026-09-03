---
type: decision
status: accepted
date: 2026-09-02
affects:
  - passive-observation
  - semantic-memory
source_paths:
  - internal/director/observesession/runner.go
  - internal/director/observe/sample.go
  - internal/director/observe/terms.go
  - internal/director/observe/screenstate.go
  - internal/director/observe/establish.go
  - internal/director/observesession/runner.go
  - internal/director/semanticmemory/store.go
  - cmd/director/observewiring.go
  - cmd/director/observelook.go
  - cmd/director/observeledger.go
  - cmd/director/observepromote.go
  - cmd/director/observemap.go
---

# ADR-123 — a control you can see is worth knowing about

## Context

Watch & Learn acquired knowledge through exactly one channel: somebody pressed something, a
destination settled, the transition was attributed, and an edge was learned. Everything else Marco
could see went unremembered.

That is a narrow way to learn about a computer. A person who opens Settings and walks four pages
has shown Marco four screens, and Marco kept three connections and nothing at all about what those
screens *contain* — though it read every control on every one of them to attribute the presses.

## Decision

**A second, independent channel.** A settled Place, a healthy current world, and a control that
keeps being there may become a durable Target scoped to that Place.

```
ACTIVE USE     press → settled destination → attributed transition → edge
PASSIVE SEEING settled Place → current world → stable admitted control → target at that Place
```

Neither depends on the other. **A target does not need to be clicked to be known. An edge still
needs a destination somebody was observed arriving at.**

### It is not topology, and cannot become topology

From *"Home offers a control called Bluetooth & devices"* Marco may store that the control exists
at Home. It may not store an edge, however exactly the label matches a Place it knows.

This is held by shape rather than by rule. `TargetsToRecord` returns signatures, whose type cannot
express a relationship; the write path holds a store whose only method writes a subject. There is
no path from acquisition to an edge to be disciplined about.

### Recurrence, not a cap

A control seen on one reading may be a menu that was open, a toast, a list still loading, or a
transition frame carrying the controls of the page being left. A control seen across separate
readings of a **settled** state is a property of that state. This is `settledPlaceName`'s rule and
its threshold, applied to a second kind of fact — one recurrence machine, not two.

No per-Place policy cap. `MaxAffordancesPerPlace` is a bound against pathology, set far above
anything measured, and if a real interface reaches it that is a finding to report rather than a
number to raise.

### The label gate is narrower than the target gate, deliberately

`AdmittedAffordanceLabel` shares the shape filter verbatim with `AdmittedTargetLabel` and differs
in one stage. The target gate admits an activatable role — a list item, a tree item, a link — under
a demonstration licence, and [[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]]
states the limit in the same breath:

> *"the gate still admits only what one input event's own resolution touched, never a sweep"*

This is the sweep. There is no per-element provenance — nobody aimed at anything — so the widening
does not travel, and what remains is the unconditional plaintext allowlist.

**What that costs, plainly.** Windows Settings navigates by `list_item`, so its navigation rail is
refused and Marco learns a page's buttons and toggles rather than its rail. That is the
conservative side of the trade, chosen because the interfaces whose rails this refuses are the same
interfaces whose rows are somebody's private life, and no rule available at this point can tell a
Settings rail from a chat list without naming applications.

So the refusals are **counted and reported** — `39 actionable, 6 admitted, 33 withheld (list_item
33)` — and whether to widen is a question for measurement. Counts and role names only: the refused
text is what the gate exists to withhold.

### A fourth permission

`AcquireVisibleAffordances`, off by default, granted by `LearnLicence` and by ambient learning
through the same two doors `mayNameTargets` reads. It could not fold into `NameActivatedTargets`
because that permission's whole justification is the sentence ADR-114 quotes above. This is a
different question and gets an answer a caller can decline.

### Recurrence is observational, never scope

`PlacesOffering` answers *which known Places contain this control*. A list of Places, which is what
was observed. There is no ANYWHERE node, no application-wide entry, and nothing executable:
**scoped-affordance execution stays deferred** until there is a dense enough map to design it from.

### One line in the feed, not sixty

`Noticed` / `KindAffordance`, carrying a count, announced once per commit by the store — which is
the only thing that knows how many were new. *"noticed 6 things I can do at Mouse"*, never the
control names, and demoted out of the headline so a way Marco learned still wins the one line
people read. It extends the distinction [[ADR-122-a-movement-is-not-a-way]] drew, one level down.

### And watching had to start making Places, because nothing else did

Found by dogfood, on the first run where it mattered. A Place became durable in exactly three
ways: a crossing was promoted, a licensed session ran, or a person named the screen themselves.
**Ambient sessions declare the zero episode, so the second never applied to them** — and nothing
noticed, because promoting a crossing covers the case anybody was testing.

Measured over 25 seconds of Watch & Learn on a settled screen offering two named controls:

```
1219 readings carried an establishable shape
   0 durable places, 0 durable targets
```

Every one of those readings was accepted by `PlacesToEstablish`. Nothing wrote it down. So on a
fresh store the passive channel could never fire at all: it waited for a Place that only the active
channel creates, which makes two channels meant to be independent into one with a bootstrap
problem.

It hid from this change's own acceptance tests because every one of them established the Place
itself before exercising acquisition — **a fixture agreeing with itself about the precondition it
was meant to be checking.**

So `settlePlace` writes the screen somebody is sitting on, through the same promotion boundary,
under the same licence. It is a widening and it is worth saying plainly: a screen you dwell on
becomes durable memory without your having done anything on it. That is the point — *a control does
not have to be clicked to be known* is unreachable otherwise — and it is bounded by the refusals a
licensed session already gets: not settled, still loading, not discriminating, not describable.
Watching alone still writes nothing.

### And dwelling waits for the word, where a crossing cannot

Establishing on dwell creates Places far earlier than promoting a crossing did — on the first
settled reading rather than after somebody has been somewhere and come back. The next dogfood run
showed what that costs:

```
Home --> Unnamed place --> Mouse
```

A real screen, correctly recognised, carrying a structural identity and an affordance, and nothing
a person can call it. It was a **transit** screen: walked through, never dwelt on, so its word
never recurred — and a name settles by recurrence.

So dwelling now waits for the name. Nothing is lost by waiting: the screen is still there, the
naming sweep runs on every reading, and the first reading that can name it establishes it with the
name already on.

**A crossing is deliberately not subject to this.** An edge whose destination cannot be written
down is an edge that is LOST, and unlike a dwell there is no later reading that recovers it — the
crossing has already happened. A nameless endpoint fills in on the next visit; a dropped edge does
not come back.

## Consequences

- The durable graph semantics are unchanged. Nothing here writes a relationship, and no existing
  reader of the topology sees a difference.
- Targets acquired this way are indistinguishable in the store from targets acquired by
  demonstration, which is correct: the identity of a control is what it is called and where it is,
  not how Marco came to hear of it. Provenance rides on `Learned`.
- Whether the sweep gate is too narrow is now an empirical question with an instrument.
- An explicit Learn acquires affordances too. It has to: `LearnLicence` grants the permission, and
  without a consumer on the licensed path a Learn session would read every admitted control on the
  screen, tally it, and write none of it. **Reading somebody's screen for nothing is worse than not
  reading it** — the same defect ADR-114 recorded from the other direction.

## Enforced by

- `internal/director/observe` `TestAControlThatKeepsBeingThereBecomesADurableTarget`
- `internal/director/observe` `TestAControlSeenOnceDoesNotBecomeDurable`
- `internal/director/observe` `TestATargetIdentityCarriesNoGeometry`
- `internal/director/observe` `TestALabelMatchingAPlaceNameCreatesNoEdge`
- `internal/director/observe` `TestAnUnsettledScreenOffersNothing`,
  `TestAnUnrecognisedScreenOffersNothing`
- `internal/director/observe` `TestAControlWhoseKindIsDisagreedIsNotRemembered`
- `internal/director/observe` `TestASweepDoesNotAdmitTheRolesADemonstrationDoes`
- `internal/director/observe` `TestASweepStillRefusesTextThatDoesNotLookLikeAControl`
- `internal/director/observe` `TestAnAffordanceSurvivesTheProductionEvidencePath`
- `cmd/director` `TestWatchingAndLearningRemembersWhatAScreenOffers`
- `cmd/director` `TestWatchingAloneRemembersNoAffordance`
- `cmd/director` `TestARememberedTargetIsNotACurrentlyVisibleOne`
- `cmd/director` `TestOneControlAtSeveralPlacesIsRecordedAtEachAndScopedToNone`
- `cmd/director` `TestAcquiringAffordancesCreatesNoRelationship`
- `cmd/director` `TestAnAffordanceIsSaidAsACountAndAPlace`
- `cmd/director` `TestAcquiringAffordancesRefusesWithoutItsLicence`
- `cmd/director` `TestASweepThatLearnedNothingAnnouncesNothing`
- `cmd/director` `TestASettledScreenOffersWhatMarcoCanSeeOnIt`
- `cmd/director` `TestWatchingWithoutLearningRemembersNoAffordance`
- `cmd/director` `TestTheSweepCountsWhatItRefused`
- `cmd/director` `TestOneControlSeenTwiceInAReadingIsOneAffordance`
- `cmd/director` `TestAControlSeenRepeatedlyReachesTheStore` — the whole chain, ambient
- `cmd/director` `TestAnExplicitLearnRemembersWhatTheScreenOffered` — the whole chain, licensed
- `cmd/director` `TestEstablishingAPlaceDoesNotLicenseAcquiringItsAffordances`
- `cmd/director` `TestWatchingAndLearningRemembersWhereYouHaveBeenSitting`
- `cmd/director` `TestWatchingAloneRemembersNoPlaceItMerelyLookedAt`
- `cmd/director` `TestDwellingDoesNotEstablishAScreenItCannotName`
- `cmd/director` `TestACrossingStillEstablishesAnEndpointItCannotName` — the control

## Related

- [[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]]
- [[ADR-122-a-movement-is-not-a-way]]
- [[ADR-124-a-screen-may-say-which-screen-it-is]]
- [[ADR-095-repeated-observation-may-become-knowledge]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
- [[ADR-068-the-theater-is-the-durable-semantic-world]]
- [[Experiment-022-the-first-dogfood]]
