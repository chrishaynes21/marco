---
type: subsystem
status: active
owners:
  - director
depends_on:
  - hypotheses
used_by:
  - hypotheses
updated: 2026-08-17
source_paths:
  - internal/director/observe/recall.go
  - internal/director/observe/theater.go
  - internal/director/observe/screennaming.go
  - internal/director/observe/remember.go
  - internal/director/observe/establish.go
  - internal/director/semanticmemory
  - internal/director/observe/relationship.go
  - internal/director/observe/learning.go
  - internal/director/observe/demonstration.go
  - internal/director/observe/assessment.go
  - internal/director/observe/rehearsal.go
  - internal/director/observesession/runner.go
  - cmd/director/graph.go
  - cmd/marco/screenwiring.go
---

# Semantic Memory

What the user told Marco, surviving a restart. It is the difference between a system that
accumulates understanding and one that begins again every morning.

It remembers **structure and answers**. It does not remember screens, pixels, text, keys or
sessions. See [[ADR-016-cross-session-identity-is-structural-and-conservative]].

## The two claims, kept apart

| claim | answered by |
|---|---|
| "this is the same subject I saw before" | `observe.CompareStructure` — structural, tolerant, discrete |
| "this subject is a settings screen" | the `SemanticKnowledge` a matched subject carries |

Keying identity on the semantic label would make every settings screen in every application one
object. They are separate on purpose.

They are separate in **who may make them**, too, and until [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
they were not:

| claim | kind | made by |
|---|---|---|
| "I can recognise this place again" | observational | Marco, from its own evidence — under an explicit licence |
| "this place is a settings screen" | semantic | a person, by answering |

A durable subject used to be written **only** in `Runner.Respond`, so the first claim could not be
made without the second. That is what blocked `learn "…"` at its first step against any
application nobody had happened to answer a question about. A place established under the Learn
licence carries an **empty** interpretation list, which is the same shape a subject has had all
along — the store's records were always structure plus a list of answers, and the list may be
empty.

## Four verdicts, and only one of them inherits an answer

| verdict | meaning |
|---|---|
| `same` | structure agrees **and** a discriminator agrees. The only verdict that may carry knowledge forward. |
| `candidate` | structure agrees and nothing distinctive confirms it. Reported; inherits nothing. |
| `different` | a signal positively disagrees. Marco knows this is new. |
| `insufficient` | cannot tell — no evidence, or several subjects fit equally. |

No similarity score. Two subjects at 0.71 and 0.69 are not "the first one".

## What identity is made of

**In:** subject kind, role composition (±1 per role), member count (±1), normalised envelope
(IoU ≥ 0.90 when both have one), interface terms (exact set).

**Out, deliberately:** `Recurrence` (it grows), session-local `state_3` / `shadow_8` / `group_1`,
window generation, process id, absolute coordinates, proposal ids, and anything read off a screen.

A **discriminator is required** for `same`: matching non-empty terms, or a matching envelope.
Structure alone is `candidate`, because "five buttons" describes a settings screen, a level
select, a save-file list and a confirmation dialog.

## What is remembered about a subject

`observed` · `confirmed` · `contradicted` · `declined` — per interpretation, with the evidence
sources present at the time and the structural digest it was settled on.

- **Confirmed** survives and suppresses the question.
- **Contradicted** survives just as durably. A correction the system forgets overnight is one the
  user must give every day.
- **Declined** suppresses until the evidence changes **shape** — not by time passing, and not by
  seeing the same thing again.
- **Observed-only never becomes validation.** A record existing is not a person agreeing.

A subject may also carry **no** interpretation at all. That is a place established because the
user asked Marco to Learn something — Marco can find it again and claims nothing whatever about what
it is. `describeSubject` renders it *"recognised, nothing known about what it is"*.

## Two ways a subject becomes durable

| write | trigger | what it records |
|---|---|---|
| `Remember` | a person answered a proposal (`Runner.Respond`) | the subject **and** one interpretation |
| `EstablishPlace` | an explicit `learn "…"` pass, at session end | the subject, and **nothing else** |

Both go through `subjectLocked`, the one canonical match-or-append: same identity test, same ids,
same discriminator rule, same `MaxSubjects` bound. `EstablishPlace` leaves an existing subject
completely untouched — it is not a route by which observation can edit what somebody settled.

The licence is `observesession.Episode.EstablishPlaces`, set by `learnPasses.episode` and by
nothing else. Every other session gets the zero value, so **passive observation still persists no
subjects**. See [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] for the bound and the
refusal vocabulary (`Result.Places`).

A licensed pass establishes **every place it settled on**, not only the one it ended at — each
clearing the same four gates independently, in first-sighting order, capped by
`MaxCheckpoints`. A route with a middle step needs its middle place durable or both edges either
side of it have an endpoint nothing can resolve; that is what
[[ADR-063-a-pass-remembers-every-place-it-settled-on]] fixed, and it amends only ADR-047's count.
The others (`PlaceEstablishment.Also`) are reported beside `Subject`, which still means the place
the pass ended on.

## Where memory sits in the flow

```
perception → hypotheses → RECALL → proposal policy → question (or not) → answer → REMEMBER
                                                        Learn pass ends → ESTABLISH PLACE
```

Recall runs after hypotheses and before the policy, so it can suppress a question rather than
withdraw one. **Nothing in memory can add to what the providers saw** — the arrow only points
one way, and `TestMemoryDoesNotManufactureObservations` holds it.

A remembered answer is seeded into the proposal ledger as a question already answered, so every
existing behaviour — annotation, suppression, the material-change re-ask rule, the report —
applies unchanged rather than through a parallel path.

## The durable topology

Since 2026-08-09 memory also holds `relationships`: which remembered subjects have been
observed becoming which others, and what navigation was seen around it. See
[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]].

```
from: possible_settings_like_state [confirmed] (subj_a88c…)
to:   possible_menu_like_state [observed] (subj_c5d3…)
observed transitions: 4 · sessions: 2
navigation seen before: confirm 3/4 · no navigation observed: 1/4
observed navigation sequence: down → confirm (2 time(s))
causal claim: none
```

**Adjacency, never a route.** The record says the change was observed and what was around it —
not that the navigation caused it, not that repeating it would reproduce it, and not that Marco
knows how to get anywhere. The disclaimer is printed on every edge rather than once at the top.

| kept | why |
|---|---|
| per-intent support | competing intents stay visible |
| unattributed | the control evidence; an edge mostly preceded by nothing is mostly not about navigation |
| context-admitted count | [[ADR-013-navigation-is-meaning-not-keys]] does not stop at the durable boundary |
| bounded ordered runs | order cannot be reconstructed later, and is not a procedure |
| sessions, apart from observations | twenty times in one sitting is not five separate days |

Both endpoints must reach `same`. A transition with an unrecognised end stays session-local and
is REPORTED as such, because "nothing transitioned" and "nothing was recognised" are different
sessions. Written once per session, at the end — the transition tally grows while a session runs.

Referential integrity is enforced at the write (unknown or cross-application endpoint refused
and counted) and at the load (an edge whose endpoint is gone is dropped and counted). Removing a
subject removes its edges, deterministically.

## Goals: the destination is the capability

Since 2026-08-17 memory also holds `goals`: outcomes a person explicitly Learned, each a
NAME in their own words bound to one destination subject — and structurally nothing else.
[[ADR-056-a-goal-is-a-destination-not-a-route]] is the correction this records: the
demonstrated route is evidence for one way in, held in the topology beside every other
edge; the goal has no start, no waypoints and nowhere to put either. One name per outcome
per application, folded on repeat demonstrations, refused on conflict, dropped at load when
its subject is gone — the same referential rules as everything here.

`observe.PlanToGoal` plans from wherever the person currently is, over whatever edges the
caller's predicate admits; `director reach` is the read surface, planning over edges a
completed rehearsal still vouches for, and refusing honestly otherwise. A goal licenses
nothing: execution still goes through a saved play's resolve → authorize → run.

## Asking to learn a habit

Since 2026-08-09 a sufficiently corroborated edge may earn an INVITATION — see
[[ADR-019-an-invitation-to-learn-is-not-a-correction]].

> "I've seen you go from the settings screen to another screen several times.
> Do you want me to learn how you do that?"

The answer is recorded on the edge as a `LearningRequest`: `pending`, `refused` or `declined`,
with the evidence digest it was given against. A yes buys **a pending request and nothing else** —
no procedure, no capability, no action, no route and no recorder.

**The two `no`s are opposites and are typed apart.** `Proposal.Ask` says which question was put:
`no` to a semantic one is a durable contradiction of the interpretation, `no` to an invitation is
a preference and leaves every observation exactly where it was. Nothing parses the wording.

Eligibility is discrete and every refusal is a closed reason — `insufficient_sessions`,
`navigation_too_weak`, `too_much_unattributed`, `conditional_only`, `runs_inconsistent`,
`endpoint_unresolved`, `already_declined`, `learning_pending`, `another_question_open`. Every
remembered edge is judged and reported, because "Marco did not ask" is otherwise undebuggable.
Reviewed once per session, at the end, so the semantic questions get first claim on the single
interruption slot.

## Watching one demonstration

A `LearningPending` request arms a bounded capture — see
[[ADR-020-watch-me-is-permission-to-observe-not-to-act]]. Its output is a `ProcedureCandidate`:
the start subject, ordered `NavIntent` runs, the checkpoints between them, and the destination.

**Evidence, not a procedure.** No `Execute`, no registration, `Verified` always false. It lives
in the analysis core, whose boundary test already proves nothing there can act.

Start and end are decided by CURRENT evidence, never by the request and never by a timeout. A
demonstration that ends at a different remembered subject is `destination_mismatch`, not a
reinterpreted request. An unrecognised intermediate screen is a TRANSIENT checkpoint and is never
promoted into memory — promotion involves a person.

A completed candidate is kept, one per relationship, beside the topology. An active capture is
never persisted and so can never be resumed: a demonstration is a bounded thing somebody agreed
to give.

## Judging a demonstration

A candidate is the OBSERVATION; what Marco concludes from it is a judgement recomputed from the
candidate plus the current topology, every time it is read — see
[[ADR-021-a-judgement-is-recomputed-not-recorded]]. Nothing durable holds a verdict, so a
demonstration whose middle screen the user later names becomes more verifiable with no new
observation.

Four verdicts — `candidate_consistent`, `insufficient_evidence`, `ambiguous`, `invalid` — and
`candidate_consistent` is the ceiling: it says the observation hangs together, never that a
procedure works. No confidence number; the useful output is the list of checkpoints Marco could
NOT check.

Two demonstrations compare on endpoints, checkpoint sequence, text-entry markers and the
DECISIVE navigation of each run — directional presses move a selection and commit to nothing, so
`down, down, confirm` and `down, down, down, confirm` are `compatible` while `left, back, down,
confirm` is `different`.

## Asking for a second example

A judgement with a gap another demonstration could close earns ONE follow-up request — see
[[ADR-022-ask-only-when-you-can-say-what-it-would-resolve]]. Eligibility is derived from the
assessment (`FollowUpFrom`), never re-derived from the candidate, and one non-resolvable blocker
is enough to stop: a route that needs text entry stays unusable however many examples Marco sees.

Lineage is `relationship → candidate 1 → assessment → follow-up → candidate 2`, with
`ProcedureCandidate.Sequence` and immutable candidates. `LearningFulfilled` stops a satisfied
request re-arming a capture in every later session. Two demonstrations, then
`second_demo_already_captured`.

Two agreeing examples drop `single_demonstration_only`; two that differ add
`demonstrations_disagree` and are never averaged. **Neither promotes anything** — `Verified` stays
false.

## Storage

One JSON file beside the action graph under `$MARCO_HOME`. Written atomically (temp + rename,
`0o600`) like `actiongraph.Save`, and **only on semantic events** — an answer, or evidence that
changed shape. Never per sample.

Growth is per **subject**, not per observation: a screen seen ten thousand times is one record,
because a write matches an existing signature and updates it.

**Corruption is reported, never swallowed.** An unreadable file leaves memory inert-and-visible:
the Director still perceives, still asks, and says why it cannot recognise. The broken file is
neither discarded nor overwritten, because a person may want to recover it. An unknown format
version is refused rather than reinterpreted.

## Application namespacing

Memory is scoped by application — provenance, and a conservative boundary. Two applications
presenting structurally identical screens are not assumed to mean the same thing by them.
Recognising common structures *across* applications is a stronger claim and belongs to a
milestone that can measure whether it holds.

## Related systems

- [[Hypotheses]] — what is remembered, and the proposal loop memory feeds
- [[Passive-Observation]] — where the evidence comes from

## What a completed rehearsal leaves behind

One record per (route, demonstration): `observe.RehearsalEvidence`. Bounded, semantic and
**nothing executable** — which candidate, which digest, which endpoints, how many steps and inputs,
and the per-step outcome vocabulary. Reproducing any of it means lowering the candidate again
through [[Marco-Boundary]] under a fresh authorization.

It does not itself mean verified. `CandidateAssessment.WithRehearsal` recomputes that from the
evidence plus what memory holds NOW — the route completed, the digest still matches, both endpoints
are still recognised. A stored boolean would go on saying yes after the demonstration was revised
or a screen was contradicted. See
[[ADR-026-verification-is-derived-from-a-completed-rehearsal]].

Load applies the same referential rule as candidates: evidence whose endpoints are gone is dropped.

## Decisions

- [[ADR-098-the-planner-prefers-better-evidence-and-says-why]] — eligibility decides whether an
  edge MAY be planned and ranking decides which eligible route is preferred; the two are kept
  apart. Ordered classes rather than a score: contradiction first and never traded away,
  verification worth exactly one extra action, the weakest edge rather than an average,
  repetition saturating at "more than once", and route length still costing

- [[ADR-056-a-goal-is-a-destination-not-a-route]]
- [[ADR-016-cross-session-identity-is-structural-and-conservative]]
- [[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]]
- [[ADR-019-an-invitation-to-learn-is-not-a-correction]]
- [[ADR-020-watch-me-is-permission-to-observe-not-to-act]]
- [[ADR-021-a-judgement-is-recomputed-not-recorded]]
- [[ADR-026-verification-is-derived-from-a-completed-rehearsal]]
- [[ADR-031-the-user-names-the-stage]]
- [[ADR-041-a-screen-is-not-its-dominant-group]]
- [[ADR-108-what-a-reflow-removes-cannot-always-be-told-from-where-you-are]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
- [[ADR-063-a-pass-remembers-every-place-it-settled-on]]
- [[ADR-064-the-order-of-a-walk-is-evidence]]
- [[ADR-022-ask-only-when-you-can-say-what-it-would-resolve]]
- [[ADR-015-a-question-is-evidence-not-settlement]]
- [[ADR-014-hypotheses-are-evidence-not-identity]]
- [[ADR-076-a-place-may-say-what-it-appears-to-be-called]] — the selected destination names a
  Place; an inference is never recorded as something the Audience said
- [[ADR-109-a-screen-carries-several-true-names-at-once]] — the shell, the section and the
  destination are three true names, and only the last may be a Place's
- [[ADR-110-a-navigation-rail-is-a-list-of-places-you-could-go]] — neither the hierarchy above a
  claim nor remembered topology can say which

## Validated by

Full list in [[ADR-016-cross-session-identity-is-structural-and-conservative]]. The three that
matter most:

- `TestTheSameSubjectIsRecognisedAcrossSessions` and `TestTwoSimilarSubjectsAreNotMerged` — the
  adversarial pair. One must be recognised, one must not, and they differ only in what ought to
  matter.
- `TestAConfirmedSubjectIsRecognisedInALaterSession` — the production test: two runners over one
  file, session B deliberately renumbered.
- `TestNothingCapturedCanReachDurableMemory` — the recursive privacy sweep, on a file that
  outlives every session.

## Known gaps

- ~~**No recognition without a discriminator on a surface with no accessible names.**~~ Closed
  2026-08-09 ([[Experiment-010-vision-structure-as-a-semantic-path]]). The missing evidence was
  never OCR — [[Experiment-009-ocr-as-a-semantic-discriminator]] measured an IDENTICAL term set
  with OCR on and off — it was structure Marco is allowed to NAME. The shadow detector's own
  `button` regions, read by scoped OCR and gated on the canonical role allowlist, now supply
  terms where accessibility supplies nothing. Two settings screens with identical composition
  are `candidate` on structure and **`different`** with those terms.
- **The exact-set rule was measured, not changed.** A term lost on *every* reading makes a
  screen `different`; a control unreadable on one pass in three does not, because the per-state
  term ratio absorbs it. So exact-set matching is not currently the bottleneck. See
  [[ADR-017-structure-earns-a-name-text-never-earns-structure]].
- **No cross-application recognition.** Deliberate; see above.
- **No compaction or expiry.** Bounded at `MaxSubjects` and never pruned by age. A subject learned
  once and never seen again stays forever, which is cheap and slightly untidy.
- **Never exercised live.** Proven by scripted two-session runs, the adversarial pair, seven
  mutations and a corruption fixture. Nobody has yet restarted a real game and been recognised.

## The one string a user writes

Every other durable field here is a closed vocabulary, a derived id or a count. `Called` is the
deliberate exception: **what the user calls a screen**, in their own words, stored on the
`RememberedSubject`.

It exists because `do Screen's Showing with "…"` is executable meaning rather than a label — the
string is resolved against this store at run time, so there is no name Director may invent. It is
typed as `observe.ScreenName` with a single constructor, `UserSuppliedScreenName`, whose whole job
is to be a call site a reviewer can grep. An OCR line, an accessibility label or a window title
cannot reach durable memory by being assigned to the right variable.

Rules the store enforces:

- **One name per screen, scoped to one application.** `settings` in one program is not `settings`
  in another, and there is no fallback that would let it be.
- **No duplicates within an application.** A second screen may not take a taken name.
- **Ambiguity resolves to nothing.** `NameSubject` refuses to create a duplicate, so the resolver
  must not trust that it cannot happen — a memory file is a plain JSON file a person can edit.
  Two screens under one name resolve to *no screen*, never to the nearest one.
- **Renaming the same screen is ordinary** and drops the old name.

The name is asked for only when something concrete is blocked on it — never by a sweep over
unnamed subjects. See [[ADR-031-the-user-names-the-stage]] and [[Learned-Plays]].

## What a person told Marco, and taking it back

`observe.WhatIsKnown` is the one reading of durable *intentional* judgements — the answers a person
actually gave, through `SemanticKnowledge.Effective()`. Guesses Marco never asked about do not
appear: a list headed "what you told me" containing things nobody said invites people to correct
claims they never made.

`observe.ReviseKnown` and `observe.RetractKnown` reach a judgement by the **subject** it is about,
rather than through the question that produced it. That second door exists because the first one
closes: `ProposalLedger.Revise` needs a live proposal, a proposal needs a session that can still
recognise the subject, and a subject that has stopped being recognisable therefore has an answer
nobody can ever correct. Both verbs enforce the ledger's rule — only a *settled* answer may be
revised — and write through the same `Remember` the session path uses.

Reached from the browser through `/api/knows` and `/api/correct`, and from a terminal through
`director knows`. Neither surface writes to storage; both send a request.

Enforced by `TestAJudgementNothingRecognisesIsStillCorrectableInTheProduct`,
`TestOnlyASettledJudgementCanBeCorrected` and `TestACorrectionLandsOnTheSubjectItNames`.

### Locatable is not the same as remembered

A judgement carries `Locatable`, and it is derived **only** from which durable subjects a recent
session actually recognised — never from stored geometry. Remembering what somebody said and being
able to point at what they said it about are different facts, and a surface that conflated them
would offer to show a person something nothing on screen can match.

Enforced by `TestNamingASubjectIsNotTheSameAsRecognisingIt`.

## Chrome is not identity

Identity compares role composition, and the role SET is part of it — a screen that gained a
whole role is a different screen. At accessibility scale one role fails that reasoning:
`scroll_bar` appears when content is a shade taller than its space, or, on Windows 11, while
the pointer is over the region. Measured live, that alone minted five durable subjects for
the same Settings pages and made Learn impossible on mainstream software.

A closed chrome set is excluded from comparison and still recorded.
[[ADR-062-a-scroll-bar-is-not-a-screen]] states the test a role must fail to be listed, and
why the widening cannot reach the over-merge case.

**And the same test excludes four more, because one role was never the whole of it.** A real store
had accumulated TWELVE durable Places for THREE Settings pages: each page recorded at a different
window size, and every size a different screen. `group`, `pane`, `text` and `link` all move with
reflow without the page moving, and a role-count tolerance of one cannot survive a paragraph
unwrapping from 32 runs into 49. So layout roles are recorded and not compared, and the tolerance
above the floor is a SHARE of the count rather than a number — which changes nothing below seven,
where the four-versus-six-item worry lives. `button` is deliberately still compared, and
`progress_bar` deliberately still decisive: a page part-way through loading is not the page.

Several matches stopped meaning ambiguity: records that are the same Place as EACH OTHER are one
Place written down twice, and Marco answers with the lowest id rather than refusing. Records that
match the reading and not each other are still `insufficient`.

**Same Place after a resize does not mean the same live Stage evidence** — the two questions are
unrelated and both answers are true at once.
[[ADR-091-a-place-is-not-its-presentation]].

### Where that stops, measured

One Settings page is one Place from 1500px to 850px with a **byte-identical** signature. Below
about 800px Windows removes its navigation pane and the page is no longer recognised — the
decisive field is the role set, because `image` and `text_field` leave with the navigation.

That false miss is deliberate. Measured over three destinations at six widths each: tolerating
what the reflow removed raises same-destination matches from 18 of 45 to 36 **and merges Mouse
with Printers & scanners fifteen times**, because `back settings` is a subset of `back controls
settings`. And it still does not recognise Mouse, whose own content has three list items — so the
navigation's twelve leaving arrives as a count disagreement (15 against 3) rather than as an
absence, while on pages with no content list items the identical event is an absence. A flat role
histogram cannot tell those apart.

The collapsed reading also loses the page's NAME: `AdmittedPlaceName` reads the selected
navigation item. The page still names itself in a breadcrumb, which is where a future attempt
should look — carefully, because the same run caught Printers naming itself after its section at
one width. See [[ADR-108-what-a-reflow-removes-cannot-always-be-told-from-where-you-are]].

### What a Place is called, and which of the screen's words that is

A screen carries several TRUE names at once. Measured on Windows Settings' Printers page:

	Settings              the application shell
	Bluetooth & devices   the section, and the selected item in the navigation rail
	Printers & scanners   where the person actually is

None is wrong; only the last may be a Place's name. `observe.ExplainPlaceName` is the one rule —
`AdmittedPlaceName` is it with the explanation discarded — and it separates the levels using the
TRAIL: the group of sibling buttons containing the selected word is the path taken, and the entry
that is not the selected word is the leaf.

`director name-probe` prints every claim, its level, the trail evidence it came from, and why it
was admitted or refused. It calls the production producer and the production rule, so a claim it
shows is a claim production made.

**Where it stops, measured.** The rule is reached only through a SELECTED item, so a responsive
layout that removes the navigation removes the name — the breadcrumb is still on screen and
unreachable. And a selected item that no trail corroborates is admitted anyway, which is right on
Settings Home and wrong on Printers at its overflow width, where the section gets admitted. Those
two arrive as the identical shape with opposite correct answers, so requiring corroboration was
measured and rejected: it fixes Printers and VS Code's `Terminal (Ctrl+`)` and loses Home.

A Learn performed while the navigation is collapsed therefore still produces an unnamed Place. See
[[ADR-109-a-screen-carries-several-true-names-at-once]].

**Two routes out were measured and closed.** The accessibility hierarchy above the selection is
byte-identical on the screen where the selection IS the destination and on the screen where it is
the section. And remembered topology cannot referee it, because a navigation rail publishes every
one of a page's children — so "the selected Place has an edge to a visible label" is true on every
wide page, and a rule built on it would demote every parent, starting with Home. See
[[ADR-110-a-navigation-rail-is-a-list-of-places-you-could-go]].

The boundary that rule would have crossed is now enforced: `placeNameEvidence` reads one fused
world and nothing else. Memory may help interpret current evidence; memory may never become
current evidence.


## Known correctness debt: a durable Envelope can strand a subject

**Status: OPEN.** `StructureSignature.Envelope` is durable and load-bearing: `CompareStructure`
requires `EnvelopeIoU ≥ 0.90` when both sides carry one. A sighting captured while a group is
partly outside its window stores a *pathological* envelope, and nothing observed afterwards can
overlap it enough to match. The subject is then permanently unrecognisable, so the question it
belongs to can never be recalled and its answer can never be reached through any session path.

Reproduction, as observed on 2026-08-11: `explorer` / `subj_8be34d320f94` /
`possible_choice_group`, stored as `y: -1.7348, height: 1.2980` — a 24-item navigation tree
captured scrolled. Two live observations of File Explorer (31 and 91 samples) recalled nothing.

This is why "What Marco Knows" reaches judgements by subject rather than by question; that surface
makes the answer correctable, and it does **not** make the subject recognisable again. Fixing the
matcher is a separate correctness task and must not be done opportunistically — it is screen
identity, and every remembered subject depends on it.

## The Theater: durable targets, above any one provider

A learned play used to say `do Accessibility's Invoke with a Control called "Mouse"`, welding a
**provider** into a **behaviour**. [[ADR-068-the-theater-is-the-durable-semantic-world]] moved
target identity above the providers: a **Target** is a durable subject like a place is, identified
by the place it is grounded in, the word on it, and its **kind** — a closed vocabulary of
`button`, `item`, `field`, `menu`, `link`, `tab`, `checkbox`, `window`.

- `observe.TargetSignature(place, label, kind)` builds the identity; `Store.RememberTarget`,
  `ResolveTarget` and `TargetsIn` are its side of durable memory.
- Comparison is **exact** for a target and tolerant for a screen (`compareTargets` in
  `recall.go`). A screen that gained a scroll bar is the same screen; a control with a different
  word on it is a different control.
- `RememberedSubject.Learned` carries **provenance** — which `EvidenceSource` produced it, and
  whether that source is `Authoritative()`. Only the Audience is.
- A demonstrated target is grounded on the place the click was **pressed on**, not the place it
  landed on (`TargetsDemonstrated`).

The Theater production layer lives in `internal/platform/theaterhost`: Actors are castable
capabilities (`Name`/`Available`/`Find`/`Perform`), and `Theater.Activate` casts one, performs,
and verifies, refusing through a closed vocabulary — `target_not_found`, `target_ambiguous`,
`no_actor_available`, `perform_failed`, `not_verified`. A play now says
`do Theater's Activate with target1`, where `target1` is a `Target` set carrying `Name` and
`Kind`, and `Control.Name` is demoted to an executor detail.

## One file, two processes

The store is a single file at `$MARCO_HOME/semantic-memory.json`, chosen by
`cmd/director/graph.go`'s `semanticMemoryPath` and by `cmd/marco/screenwiring.go`'s `memoryPath`.
`$MARCO_MEMORY` names it outright and **both** processes honour it — an override obeyed by one and
ignored by the other is the same split under a different name.

`marco` used to look for `routes/memory.json` instead. Nothing has ever written that file, so
there was no disagreement to reconcile — only an empty store, which made `do Screen's Showing`
refuse with *"nothing in <app> is called <name>"* about a name the Director had in fact recorded.
Pointing it at the one store does not make the guard pass (a learned play is performed by the
Director, and [[ADR-078-a-learned-play-is-performed-by-the-director]] means its `Showing` lines do
not run in `marco` at all); it makes the refusal a true sentence when the guard does run locally.

## Naming is authorship, and authorship is reversible

[[ADR-069-a-name-is-authored-and-can-be-taken-back]] settles what happens when somebody names the
wrong thing — which happened live, and could only be repaired by hand-editing
`semantic-memory.json`.

Two rules: **Marco may not ask the Audience to name something it cannot ground for them**, and
**an audience-supplied name must be reversible**.

| | |
|---|---|
| grounding | a **description** of what a place is made of, never a subject id; two places may not read alike |
| binding | the answer lands on `q.Screen`, the subject the QUESTION named, never on what is current |
| three operations | name, rename, unname — all on the **same** subject id |
| uniqueness | over **live** names only; a word nothing is called is free |
| namespace | read off the subject (`applicationOfPlace`), never from session context |
| focus | none required; the person is necessarily looking at Marco while they type |
| perception | an observed label is `Structure.Label` evidence and never becomes `Called` |

`Store.UnnameSubject` keeps the place, its judgements and every route through it, and releases
only the word. `observe.ScreenAuthor` is the interface that makes both directions available to a
surface; `/api/learn/rename` is the way in, with an empty name meaning **retraction**.

## What Sight says about the Theater

`director sight` now reports what Marco can **act on** here — the durable targets grounded in the
current place, rendered as label and kind — and what it **last did**, read from the action graph.
Targets are claimed only for a place Marco has settled on: `Place.Subject` is filled only for
`MatchSame`, and every target is grounded in a real place id, so an unsettled screen asks for
targets grounded in `""` and there are none.

Enforced by `TestPointAtFillsInWhatMarcoCanActOnAndLastDid`,
`TestTargetsAreNotClaimedForAPlaceMarcoHasNotSettledOn` and
`TestSightSaysWhatItCanActOnAndWhatItLastDid`.
