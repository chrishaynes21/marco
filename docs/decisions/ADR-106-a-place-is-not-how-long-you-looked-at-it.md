---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - observation
  - semantic-memory
source_paths:
  - internal/director/observe/reach.go
  - internal/director/observe/reachdrift_test.go
  - cmd/director/escalationwiring.go
  - cmd/director/cyclelock_test.go
  - cmd/director/showingcmd.go
  - acceptance-37g.ps1
---

# ADR-106 — a place is not how long you looked at it

## Context

Three pieces of acceptance debt had converged on one question, and none of them could be
answered without a real store and a real desktop:

- **35D** left `RESIZE_ACCEPTANCE_STATUS: UNMEASURED`. Does one page at two window sizes resolve
  to one durable Place?
- **37D** established that the capture-to-totals adapter cannot build a `StructureSignature`, so
  the question could not be answered offline at all.
- **37F** added a second reason to care: with a detector configured, a Place learned before that
  phase carried visual evidence in its signature, and recognising it from an
  accessibility-only reading compares a smaller signature against a larger remembered one.

So this phase asked the running system, in an isolated `$MARCO_HOME`, through
`ObserveShowing` — the one "where am I standing" door, resolved by `observe.PlaceNow` against
the real `semanticmemory.Store`.

**Two defects had to be fixed before the question could be asked at all**, and both were
invisible to a green 85-package suite. They are recorded here because they were found by this
acceptance and because each is a rule, not a patch.

## What had to be fixed first

### 1. Every observation session stopped after one sample

`liveSampler.Sample` holds `Runtime.mu` for the whole collect-and-fuse so the pinned window
cannot move under the providers. 37F put the escalation gate inside that section —
`s.request(req)` is evaluated as an argument to `Collect` — and the gate's `incompleteFor` took
`Runtime.mu` to guard one timestamp. `sync.Mutex` is not reentrant.

Measured on Windows Settings, three builds:

```
pre-37E                 9 samples / 12s
HEAD, gate bypassed    14 samples / 12s
HEAD                    1 sample, then silence
```

The FIRST sample survived, which is what hid it: nothing had settled, so the gate returned at
`!p.Placed` before reaching the lock. From the second sample on, Learn, Light Mode, ambient
watching and the fresh look a performance takes were all dead.

**The rule: the escalation gate is asked from inside the perception cycle, so it may take no
lock the cycle holds.** `incompleteSince` has its own mutex. 37E's shadow-provider gate is
called deeper still — from inside `Collect` — so hoisting the call would have fixed one caller
and left the other.

Every existing test of this gate calls it directly, holding nothing. The rule they prove is
right; the place production asks it from had never been entered by a test.

### 2. A window emptied because it had been watched for longer

`ReachOfState` divides "structures inside the largest space" by "structures this observation
found". The second half was `tracksInState`, which returned every structure **ever seen** in the
state. One is a property of a reading; the other grows with the session.

One session over one Settings page nobody touched, at 900ms:

```
 14 samples    466 ever-seen    142 present    recognised
 27 samples    817 ever-seen    142 present    recognised
 40 samples   1024 ever-seen     88 present    UNREADABLE
183 samples   1024 ever-seen     88 present    UNREADABLE
```

Eighty-seven structures sat inside the content region throughout. The ratio crossed
`maxVacantOccupancy` between the 27th sample and the 40th and never came back — 1024 is the
track table's cap, so it saturates and the verdict is permanent. 936 of those 1024 carried no
geometry at all.

Recognition therefore stopped working part way through every long look, and the failure was
reported as a fact about the PAGE: *"accessibility described the window but not the content"* —
the one diagnosis this classifier exists to route correctly.

This is the maturing-quantity mistake `recall.go` already names twice: `SignatureOfState` drops
`Recurrence` and a state's `Members` because both grow within a visit. A ratio whose halves are
counted over different spans is the same error, and it is worse here because it silently
disables recognition instead of splitting it.

**The rule: both halves of the occupancy ratio are counted over the same reading.**
`tracksInState` returns the structures that are still there.

A second filter — skip tracks with no usable region, since this is a judgement about
arrangement — was written, measured against the live population, found to change nothing (the
88 present tracks were exactly the 88 with geometry), and removed. An unreachable condition
beside a working one is a guard nobody can hold.

## What the acceptance then measured

Isolated home, cold store, `director learn` to establish, passive sessions to resolve. Marco
performed no desktop input: navigation is `ms-settings:` shell activation and resizing is
`SetWindowPos`.

**One Settings page is one Place across every width at which Windows keeps its layout.**

```
1500px  recognised  subj_71727a02470f      signature identical
1200px  recognised  subj_71727a02470f      signature identical
1000px  recognised  subj_71727a02470f      signature identical
 900px  recognised  subj_71727a02470f      signature identical
 850px  recognised  subj_71727a02470f      signature identical
```

Not "within tolerance" — the durable signature is byte-identical from 1500px to 850px, a 43%
reduction, while the fused element count moves 115 → 84. Restarting the Director twice changes
nothing. Mouse and Bluetooth stay distinct at every width pair tested, including narrow against
narrow, where they look most alike.

**Below about 850px the answer changes, and the reason is not identity.** Windows Settings
removes its navigation pane. Through the one matcher:

```
mouse wide   button 14 combo_box 3 image 13 list_item 15 slider 2 text_field 1
mouse narrow button 15 combo_box 3           list_item  3 slider 2
             terms  [back controls settings] vs [back controls search settings]
```

Thirteen images and twelve of fifteen list items are the navigation. The sliders and combo
boxes — the Mouse page's own content — survive. `Recall` answers `different` and Marco mints
nothing: the store held exactly two subjects at the end of the run.

## Decision

**Place identity is semantic. It is not sensor identity, element multiplicity, presentation
geometry, window width, exact structural count, or how long the look lasted.**

1. **No change to the matcher.** It was measured before it was doubted and it was right.
2. **The narrow split is recorded, not fixed.** Making it match would mean dropping `image`,
   `list_item` and `text_field` from identity and making terms tolerant. `recall.go` measured
   `list_item` as one of the counts that tells Home (22) from Bluetooth (20-21) from Mouse (15);
   dropping it buys the reflow case by paying with the false-merge case, which this repository
   has decided repeatedly is the worse trade. **A false miss is preferred to a false merge.**
3. **The failure is safe and stays safe.** A presentation Marco cannot match is refused, not
   split. No duplicate Place, no second subject, nothing to un-learn.
4. **A durable subject id is a diagnostic.** `director showing` prints one because an experiment
   comparing identities has to be able to see them. Every surface a person makes a decision on
   still answers in prose — `sight` states outright that it names no subject reference.

## Sensor richness: recognition survives it, establishment does not

Both readings were produced through ordinary production wiring. On a fresh session nothing is
placed, `EscalationOf` answers "I do not know", and its ignorance rule keeps the visual pass —
admission by the policy's own decision, not a bypass. `proven_providers` confirms the detector
proved its target rather than reporting itself unavailable, which matters because a detector that
refused cheaply and one that ran and found nothing are indistinguishable in a result table.

**Recognition is invariant.** A Place remembered from accessibility alone resolves to the same
subject when the detector's evidence is admitted, and the signature is byte-identical either way,
even though the fused world grows 115 → 135 elements and the reading costs 617ms instead of 70ms.
The detector's additions do not reach the structural vocabulary a Place is made of.

**Establishment is not.** One `director learn` on a cold store with the detector configured left
TWO durable Mouse Places:

```
subj_6add57f98cf7   … icon 9  image 13 …                 established first
subj_71727a02470f   … image 13 …  unknown 1              established second
```

The mechanism is the escalation gate acting on itself. The early samples of the pass are taken
with nothing placed, so the gate buys the visual pass and the composition carries nine `icon`
structures. A Place is established from it. The reading is now placed and sufficient, so the gate
declines, the detector stops, the composition changes, the segmenter calls that a new screen
state — and the licence, still open, makes that durable too.

Every subsequent reading, rich or primary, resolves to `subj_71727a02470f`. The icon-bearing
subject is an orphan: minted once, matched never.

**Recorded, not fixed, and the reason is that every available fix is worse.** Holding the sensor
set for the length of a session would make a game viewport — incomplete for as long as it is in
front — buy an inference on every sample of every session, which is precisely the standing-
condition expense [[ADR-104-perception-is-a-budget-not-a-habit]] exists to refuse. Dropping
`icon` from identity is not available either: it is not a detector-only role (`uiaclient` maps
UIA's own `icon`), so removing it would cost real discrimination on icon-heavy interfaces to buy
one case. And a rule keyed on which provider contributed a structure would make sensor provenance
part of identity, which is the thing this ADR exists to deny.

It is bounded: with no `$MARCO_VISION_MODEL` — the default — the detector contributes nothing,
there is one composition and one Place, and the acceptance run's store held exactly two subjects
for two pages. On a configured machine the cost is one unreachable subject per cold Learn.

## What is still unmeasured, and why

**A rich reading of a page already known to be sufficient.** Production correctly has no reason
to buy one, and manufacturing that reason would be measuring a Marco nobody ships. So the
richness result above is measured in the direction production can actually produce.

**The narrow presentation as a Place.** Whether a Settings page with its navigation collapsed
*should* be the same Place is a semantic question this phase deliberately did not answer by
loosening. It is the same page to a person and a different composition to the matcher, and
nothing measured here says which reading the product wants.

## Enforced by

- `cmd/director` `TestASecondSampleIsNotDeadlockedByTheEscalationGate` — three cycles through
  the production collector and fusion engine, entered at `liveSampler.Sample`, which is where
  the lock is actually taken
- `cmd/director` `TestTheSensorGateDoesNotReenterTheCycleLock` — the gate answers while the
  cycle's lock is held
- `internal/director/observe` `TestAWindowDoesNotEmptyBecauseItWasWatchedForLonger` — one page,
  four session lengths, one verdict
- `internal/director/observe` `TestSuspendedContentIsStillAShellHoweverMuchHasBeenSeenBefore` —
  the reading `reach.go` was written for still fails, however much the state remembers
- `internal/director/observe` `TestOneSettingsPageIsOnePlaceAcrossLayouts`,
  `TestASmallCompositionIsStillComparedExactly` — the tolerances this run did not need to move
- `acceptance-37g.ps1` — the live matrix, and `identityprobe` for the matcher's own account of
  any pair that does not match

## Related

- [[Experiment-017-live-place-identity-convergence]] — the run, in full
- [[Experiment-014-identity-variance-across-real-applications]] — the measurement the layout-role
  rule came from, which never resized anything
- [[ADR-091-a-place-is-not-its-presentation]]
- [[ADR-103-acquisition-success-is-not-semantic-completeness]]
- [[ADR-104-perception-is-a-budget-not-a-habit]]
- [[ADR-105-repair-buys-knowledge-not-permission]]
