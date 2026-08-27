---
type: decision
status: accepted
date: 2026-08-27
supersedes: []
affects:
  - passive-observation
  - semantic-memory
  - demonstrations
source_paths:
  - internal/director/observe/watched.go
  - internal/director/ambient/promote.go
  - internal/director/semanticmemory/watched.go
  - cmd/director/observeledger.go
  - cmd/marco/observecmd.go
  - acceptance-36c.ps1
---

# ADR-095 — Repeated observation may become knowledge

> **Amended 2026-08-27 (36C.1).** The original decision required **two independent
> demonstrations** before ambient evidence became knowledge. That threshold was wrong, and wrong
> in a way worth stating plainly rather than quietly editing away.
>
> It came from thinking of the unit as a *demonstration*: something a person performs, which one
> performance is too thin a sample of. The unit is not a demonstration. It is **one edge of a
> semantic graph of the computer** — and a person going from A to B by pressing X and arriving at
> B is not a sample of a habit, it is a **fact about what the interface is**. Waiting for a second
> crossing was asking somebody to prove that a door they had just walked through was still a door.
>
> The cost was not only latency. Under the old rule, two crossings watched a week apart were two
> pieces of pending evidence rather than two known edges, so the route composed from them did not
> exist; Marco could give back only what it had been shown as a whole. That is the behaviour of a
> workflow recorder, and the point of this ADR is that Marco is not one.
>
> **The corrected rule: one clean traversal is already knowledge.** *Clean* still means everything
> it meant before — uncontradicted, both endpoints describable, the control nameable, the action
> in the closed vocabulary, human work only. Only the count changed, from two to one.
> Repetition still matters, but as **strength on an edge Marco already has**, not as a gate in
> front of it. `Policy.Traversals` remains, so a deployment that wants corroboration can ask for
> it; the default is 1. What used to be `Occasions` is now `Sessions` and is **provenance** — it
> says how widely an edge has been evidenced and gates nothing.
>
> Sections below are amended in place. `DefaultOccasions`, `IndependentGap` and
> `ambient.Independent` no longer exist.

## Watching and remembering are two things to agree to

`marco observe` is attention. It reads the desktop, keeps up with where somebody is, and holds a
bounded transient trail — and [[ADR-093-observe-is-attention-not-recording]] spent its whole length
establishing that it makes nothing durable.

Turning what Marco sees into permanent memory is a different thing, and it gets a different
switch:

```
marco observe                 watch
marco observe learn           and remember what recurs
marco observe learn off       stop remembering; keep watching
marco observe status          say which, either way
```

**Off by default, and it does not survive a restart** — the same two rules watching itself
follows, for a sharper version of the same reason. A durable toggle that makes Marco build
permanent memory from somebody's desktop is a consent conversation, and inventing one here would
be arriving at it by implication.

## The progression, and each step is a different claim

```
I SAW YOU CROSS IT ONCE          candidate evidence
IT WAS A CLEAN CROSSING          durable EdgeObserved knowledge — one edge of the graph
I SEE YOU CROSS IT AGAIN         the same edge, stronger
THE PERSON ASKS ME TO DO IT      ordinary authority
I DO IT                          Theater
I PROVE IT WORKED                EdgeVerified evidence
```

36C implements exactly the second arrow. It is a memory operation and nothing else: no grant, no
lease, no input, no rehearsal, no play, and no name invented for anything.

**The arrows are about one edge, never about a route.** Two edges learned on different days, in
different watching sessions, with nothing connecting them, compose into a route nobody ever
demonstrated as a whole — because a graph gives back what its edges imply, and a recording gives
back only what it was shown. That is the whole reason the unit is an edge, and
`TestEdgesWatchedApartComposeIntoARouteNobodyDemonstrated` is where it is held.

## A candidate is not knowledge

Two records, deliberately:

| | |
|---|---|
| `observe.WatchedEdge` | *I have seen this.* Counts, times, a semantic identity. Nothing plans over it. |
| `observe.RememberedRelationship` | *I know this.* A planner handed one will walk it. |

Collapsing them would make the first sighting of anything immediately plannable, which is how a
system comes to confidently do something it saw once by accident. So candidate evidence
accumulates on its own, behind its own interface — `observe.WatchedStore` cannot write a subject, a
relationship, a goal or a judgement — and becomes knowledge only by passing through the same
admission boundary an explicit Learn passes through.

**One admission path, not two.** `promotion` is 36B's object: four methods, each refusing without
the specific permission it needs. Ambient promotion builds one and hands it a one-step
demonstration. There is no `PromoteAmbientEdge` with rules of its own, because two admission paths
would eventually be two policies and only one of them would be reviewed.

## The policy is dull on purpose, and it is pure

`ambient.Judge` is a function of a summary, a policy and a clock. No store, no I/O, nothing
reachable from it that could write anything — which is what makes the rule testable without a
desktop, a session or a file.

Every condition is binary and every refusal names itself. **There is no score.** A number between
nought and one is a way of avoiding a decision, and the decision is the product: it is better for
Marco to hold something back and be able to say why than to permanently learn an accident.

The failure mode is what settles it. A play somebody did not want can be deleted; a piece of
semantic memory Marco is confident about quietly changes what it plans forever, and nobody asked
for it.

### What it asks for

**One clean traversal.** Not because one is generous, but because one is what the fact costs.
Somebody pressing X on screen A and arriving at screen B has told Marco what that control does;
asking for a second crossing asks them to re-prove a door. The old rule wanted two, and the
reasoning — *one is evidence of a thing that happened, not evidence of a habit* — was sound about
habits and wrong about edges. Marco is not learning what this person tends to do. It is learning
what this program is.

**Repetition is strength, not a gate.** A crossing of an edge Marco already knows folds into that
edge: the count rises, the last-seen moves, and nothing new is created — no second screen, no
parallel record, no duplicate edge. That is what separates a way somebody takes every day from one
they took once, and everything downstream can read it. `Policy.Traversals` is still there for a
deployment that wants corroboration before believing anything; the default is 1.

**Sessions are provenance and gate nothing.** *Twice in a minute* and *twice on different days* are
different strengths of the same fact, and the second is worth recording: it has survived a restart,
a different window generation, very often a different day. It is written on the edge and read by
eviction and by the report. It is not a threshold, and the old `IndependentGap` — which existed to
stop a person flicking between two pages from counting as several demonstrations — is gone with the
threshold it protected. Flicking back and forth six times is now six traversals of an edge that was
already knowledge after the first, which is the correct answer.

**No contradiction.** The same screen, the same control, arriving somewhere else. That is `Never`
rather than `Wait`, because more of the same evidence deepens the problem instead of settling it:
what it means is that Marco does not understand the screen, and promoting the more frequent
destination would be a coin toss dressed as knowledge. Explicit Learn is the way through, because a
person saying what they mean is information a repetition is not.

**A contradiction cuts both ways, and it arrives later than it used to.** The observation that
causes a disagreement is marked as contested along with the one it disagrees with — otherwise the
newer record, whose whole significance is that it disagreed with something, sails through on its
own. But learning on the first clean traversal has a consequence worth saying out loud: the first
edge is often already knowledge by the time the second crossing contradicts it. Per follow-on 3
there is no unlearning, so what happens is that the contradiction is recorded, the second edge is
refused, and the first stays. That is a deliberate trade and the honest description of it is: the
faster rule is more willing to be wrong about an ambiguous control, in exchange for being useful
about the overwhelming majority that are not ambiguous. Unlearning is the roadmap that fixes it,
not a higher threshold — a higher threshold buys the ambiguous case by taxing every other one.

**Both endpoints describable, and the control nameable.** A screen nothing could establish is
`Wait` — memory improves, and the same evidence is re-judged every time, per
[[ADR-021-a-judgement-is-recomputed-not-recorded]]. A control whose name was withheld is `Never`,
because the name is not going to arrive by repetition and there would be no way to say what to
press.

**Human work only**, gated where evidence is collected rather than in the policy. A play running
while watching is on moves the screen exactly as a person would, and counting that as "I have seen
the human do this again" is how a system comes to learn its own behaviour from itself — and then to
be more confident about it every time it runs. A `ByMarco` crossing creates and strengthens
nothing, so there is no refusal word for it: a vocabulary entry nothing can produce is a reader
being told about a decision made nowhere.

**No recency bound, by default.** A thing somebody does twice a year is still a thing they do, and
after admission the durable store's own semantics decide validity. The policy has the field, and a
configured one applies it.

## Identity is structure, and the handle is only a handle

A candidate is the same candidate when the application, the action, the control's name and both
endpoints match. Endpoints match through `observe.CompareStructure` — the canonical identity test,
which is what 35D's aliasing already runs through, so a wide Settings Home and a narrow one
strengthen one candidate rather than two.

**A hash would not do**, and this was measured. The place store itself matches signatures with
tolerance precisely because two readings of one screen differ in small ways; keying candidates on
an exact digest split the evidence for one screen across two records, neither of which would ever
reach the threshold, and nothing anywhere said why. The handle is assigned once and kept, exactly
as `RememberedSubject.ID` is — recomputing it on every fold looked tidier and produced the same
split.

## What is durable, and it is a summary

Candidate evidence is the first thing in the ambient path that survives a restart, and the reason
is the product claim: an edge Marco learned by watching yesterday must still be an edge today,
and a candidate that has not qualified yet must not have to be re-earned because a Director
stopped.

| kept | not kept |
|---|---|
| application, action kind, control's name and role | screenshots |
| durable subject ids, or the structure an unrecognised screen was seen as | accessibility trees |
| what a screen appears to be called | pointer trajectories |
| traversals, sessions, contradictions, first and last seen | typed text, clipboard, secrets |
| when it became knowledge | coordinates that mean anything off their window |

The two words that cross — a control's admitted name and a screen's apparent name — are the two
36B already established, under the same rules: the canonical plaintext allowlist and the
unconditional place-name filter. Ambient watching holds no licence, so the WIDER label stage,
where somebody's documents and contacts live, is not open to it.

**It lives in the semantic-memory file** rather than a second one. Same process, same home, same
restart, same atomic rename — a second store would be a second durability implementation to keep
atomic, a second thing to bound, and a second place for a `MARCO_HOME` to be got wrong. Separate
FIELD, separate type, separate interface: that is what keeps *seen* from becoming *known* by
accident, and being in one file is what makes them durable together.

## Bounded by novelty, evicted deterministically

512 relationships, across everything. Growth tracks how many DIFFERENT things somebody does, never
how often they do them or how long Marco watched — a relationship seen ten thousand times is one
record with a bigger number on it.

Eviction is weakest-first: promoted last (it is the provenance of durable knowledge and losing it
would leave that knowledge unable to say where it came from), then uncontradicted, then by
traversals, then by how many sessions have evidenced it, then last-seen, then id.
**Every tie breaks on something**,
so two runs over the same evidence forget the same things. A bound that forgot by insertion order
would drop a candidate one sighting from promotion in favour of a thing somebody did once this
morning.

## Observed, not verified — and the graph is the graph

A promoted relationship carries what was seen: the navigation that PRECEDED the change, a candidate
whose `Verified` is false, and no rehearsal, because none happened.

Once admitted, **provenance does not change planning semantics**. `plannableEdges` asks the same
question of the same assessment the lowering path asks — one predicate, `CleanlyObserved`, which
was two copies of one rule until this roadmap collapsed them. There is no ambient planner. A
planner that treated ambiently-learned edges as second-class would be a second set of rules about
what Marco knows, and only one of the two would ever be reviewed.

Later, if somebody asks Marco to perform it: ordinary authority, foreground, fresh Stage, the
shared walker, verification. 35C's seam is untouched, and a success adds `EdgeVerified` evidence
beside the observation rather than replacing it.

## No name is invented

Ambient promotion grows the topology and the Repertoire. It creates **no goal and no play**, and
this is not a limitation to be lifted casually: Marco cannot infer a human-facing command phrase
from `A → B → C`, and a play called something awkward is worse than no play. A name comes from a
person — through explicit Learn, or later through a natural-language association — and until then
what Marco has is knowledge without a word for it, which is an honest state.

## Explicit Learn does not wait

Somebody saying *learn what I just did* means "this matters now". Ambient promotion means "I have
seen enough". Making the first wait for the second would be the worst of both: somebody who asked
would be told to come back tomorrow.

The two never produce two of anything. The admission boundary is one object, `EstablishPlace` is
idempotent by signature, and a relationship is one record with bigger numbers.

## It is not in Activity, and that is a decision rather than an omission

Every node in the action graph is a replayable desktop action with an intent and a binding.
A memory operation is neither, and putting one there would put a row into a structure whose whole
point is that its rows can be replayed.

So visibility is where the two lifecycles already are: `marco observe` and `director status` both
report learning and its counts, either way, and the candidate ledger records when each thing became
knowledge. A first-class account of *what Marco has learned by watching you* is a surface worth
having and is not this one.

## Marco says what it is waiting for

"Noticed four relationships, learned none" is true and tells nobody anything they can act on. A
control with no admitted name, a button that leads two different places, a screen nothing can
establish, a policy that was asked for corroboration and has not had it: four situations, four
different things to do about them, and the counts cannot tell them apart.

`marco observe status --evidence` asks. It reports every relationship the ledger holds — across
every application, because the ledger outlives the observer and evidence from yesterday is still
evidence — with the policy's own verdict and its own sentence for each.

**It names things**, and that is a deliberate widening of the diagnostic surface rather than an
oversight: the control somebody pressed, and whether Marco recognises the screens either side. A
person asking what Marco has recorded about them is entitled to the answer, and a privacy boundary
that made Marco's own memory unreadable to its owner would be protecting the wrong party.

**And it says over how long.** Twelve traversals since Tuesday and twelve in one confused afternoon
are different facts about how somebody works, and neither the count nor the last-seen alone can
separate them — so the summary keeps the first sighting as well as the last, and the report says the
span when there is one. The field existed from the beginning of 36C and nothing read it; that is the
same defect as an unreachable explanation, one field along.

It is a READ. It judges, which is pure, and reports; a diagnostic that promoted what it was asked
about would be the worst possible answer to "what are you waiting for".

## What this does not touch

- **No authority.** A promoted edge is a thing Marco KNOWS, and knowing has never been permission
  here: the grant is minted per invocation at the ordinary door.
- **No desktop lease, no input, no rehearsal.** Every actuating entrance funnels through
  `beginPerformance`; nothing on this path enters it, and the counter says so.
- **No new loop.** Promotion runs on a new semantic transition and on nothing else, so an unchanged
  desktop costs exactly what 36A measured.
- **Stopping watching stops collection.** What was already learned stays learned — durable
  knowledge is not evidence of the mode that produced it — and candidate evidence stays under its
  own bound, because it is what the next enabling would judge.
- **Enabling evaluates future evidence, not the past.** A switch that immediately wrote a dozen
  durable facts would be a surprising thing to do to somebody, and the evidence is not going
  anywhere.

## KNOWN FOLLOW-ONS

1. **Ambient learning is off by default and this ADR does not change that.** The architecture makes
   the two lifecycles separately controllable, which is what 36C was for. Whether the shipping
   product turns learning on with watching is a consent decision with a conversation attached, and
   it is not one to arrive at through a default.
2. **`ByMarco` evidence is discarded rather than kept.** A relationship Marco has performed
   successfully is real evidence about EXECUTION confidence, and it would have a place in a later
   policy. Today it strengthens nothing, because the alternative was a candidate field nothing
   could act on.
3. **No unlearning.** Contradictory evidence after promotion is recorded on the candidate and does
   nothing to the knowledge. Deciding when Marco should stop believing something is its own
   roadmap, and a system that deleted knowledge on one mismatch would be worse than one that keeps
   it.
4. **A multi-action leg is not candidate evidence.** A menu opened and an item chosen inside it,
   arriving somewhere in one transition, describes two things somebody did and one change.
   Splitting it would invent relationships never separately observed; keeping it whole needs a
   durable representation of a compound action that does not exist. Explicit Learn remains the way.
5. **Compound and unsupported actions.** A drag, a scroll that matters, a typed literal: none is
   representable, and the closed vocabulary is checked on the way out of the store as well as in,
   so a summary written by a later Marco is refused rather than flattened into the nearest thing
   that fits.
6. **Cross-application routes are not promoted**, because a Play is application-scoped. Making one
   span two is a change to what a Play IS.
7. **`DefaultTraversals` and `MaxWatchedEdges` are internal constants**, like 36A's three and
   36B's three. They should become configurable before anybody runs this for a
   working month.
8. **Live acceptance is UNMEASURED.** `acceptance-36c.ps1` is the harness. It drives nothing, and
   the step that would drive the desktop is handed to the person.

## Enforced by

The graph claims — the ones 36C.1 corrected — first:

- `cmd/director` — `TestOneCleanTraversalBecomesGraphKnowledge` (one clean crossing of two screens
  Marco has never seen becomes a durable edge, through real stores, with nothing invented);
  `TestEdgesWatchedApartComposeIntoARouteNobodyDemonstrated` (two edges watched in different
  sessions compose into a route nobody walked as a whole — the headline);
  `TestAScreenReachedTwoWaysIsOneScreen` (three edges over four screens, no duplicated tail);
  `TestTravellingAKnownEdgeAgainStrengthensIt` (repetition is strength on the edge, not a second
  edge and not a second screen); `TestATraversedEdgeIsNotRelearnedOnceItsScreensAreKnown` (a
  described end and a recognised end are the same end, so a promoted edge grows no pending twin).
- `internal/director/ambient` — `TestOneCleanTraversalIsAlreadyKnowledge`;
  `TestAPolicyMayStillAskForCorroboration`; `TestAFastSecondTraversalIsASecondTraversal`;
  `TestADifferentSessionIsProvenanceRatherThanAThreshold`;
  `TestNoTraversalsIsNotOneTraversal` (one means one, not "any number including none").
- And what a fold must not lose, since repetition now lands on knowledge rather than in front of
  it: `TestWhenARelationshipWasFirstTakenIsKept`;
  `TestTheFirstDescriptionOfAScreenIsTheOneKept`;
  `TestAnIdentifiedScreenNeverGoesBackToBeingAShape` (all `internal/director/ambient`);
  `TestAnEdgeOutOfAKnownScreenReusesIt` (`cmd/director` — the second edge out of a screen the
  first traversal established must look that screen up, not establish a second copy);
  `TestAContradictionMarksWhatItDisagreesWith` (`cmd/director` — both halves of a disagreement,
  under a policy asked for corroboration, which is the only configuration where the older half
  is still pending long enough to see).

And the rest of the decision:

- `internal/director/ambient` — `TestTheSameThingAgainLaterIsRemembered`;
  `TestADifferentSessionIsADifferentOccasion`; `TestOneControlThatLeadsTwoWaysIsNeverLearned`;
  `TestAnActionMarcoCannotNameIsNeverLearned`;
  `TestAnActionWordThisMarcoDoesNotKnowIsNeverLearned`;
  `TestAScreenNothingCouldEstablishIsNotAnEndpointYet`;
  `TestWatchingWithoutLearningRemembersNothing`; `TestWhatIsAlreadyKnownIsNotLearnedAgain`;
  `TestSightingsAfterPromotionStrengthenTheSameRecord`;
  `TestEvidenceThePolicyCallsStaleIsNotActedOn`;
  `TestACandidatesHandleIsItsIdentityAndNothingElse`;
  `TestARecognisedScreenReplacesItsDescription`;
  `TestEvictionOrdersCandidatesDeterministically`;
  `TestCandidateEvidenceIsASummaryNotARecording`.
- `internal/director/semanticmemory` — `TestRepeatedEvidenceIsOneRecordNotAPile`;
  `TestCandidateEvidenceSurvivesAReopen`; `TestEvictionForgetsTheWeakestCandidateFirst`;
  `TestAPromotedCandidateIsNotEvicted`; `TestCandidateEvidenceIsNotPlannableKnowledge`;
  `TestACandidateWithNoHandleIsRefused`; `TestEvictionMakesRoomRatherThanRefusingNewEvidence`;
  `TestALedgerFullOfProvenanceRefusesRatherThanForgetting`.
- `cmd/director` — `TestMarcosOwnWorkIsNotEvidenceOfAHabit`;
  `TestOneControlThatLeadsTwoWaysIsNotLearned`; `TestAWideAndANarrowHomeAreOneCandidate`;
  `TestWatchingDoesNotStartLearning`; `TestLearningFromWhatYouSeeMeansWatching`;
  `TestTurningLearningOffLeavesMarcoWatching`; `TestStatusSaysWhetherMarcoIsLearning`;
  `TestAmbientPromotionCannotWriteWithoutItsLicence`;
  `TestAmbientPromotionTouchesNothingThatActs`; `TestEvidenceSurvivesARestart`;
  `TestCandidateStorageTracksNoveltyAndNotTime`; `TestAskingWhatMarcoIsWaitingForSaysWhy`
  (the verdict, the reason, the counts and the span, all four);
  `TestWhatMarcoIsWaitingForCoversEveryApplication`.
- `cmd/marco` — `TestTurningLearningOffIsItsOwnVerb`; `TestWatchingSaysWhetherItIsAlsoLearning`;
  `TestTheEvidenceReadSaysOverHowLong`.

## Related

[[ADR-094-observe-gathers-evidence-learn-promotes-it]] ·
[[ADR-093-observe-is-attention-not-recording]] ·
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] ·
[[ADR-021-a-judgement-is-recomputed-not-recorded]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[Passive-Observation]]
