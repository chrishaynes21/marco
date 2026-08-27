---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - marco-boundary
  - demonstrations
  - programs
---

# What Marco learned becomes Marco

Director must not become a second programming language. A verified procedure is written down as an
ordinary, readable `.marco` program, held to Core v1 by the real compiler — and nothing else
happens.

---
## Context

[[ADR-026-verification-is-derived-from-a-completed-rehearsal]] left a candidate whose verification
is *derived*: a completed attempt is stored as bounded evidence, and whether it still counts is
recomputed from that plus what memory holds now.

The question this milestone answers is what that verification is FOR. The wrong answer is a
procedure graph, an internal step list, or a Director-specific runtime — three shapes of the same
mistake, which is a second language nobody asked for. The right answer is the one the repository
has been building towards since the language reconciliation: **Marco**.

---
## Decision 1 — the eligibility gate is the derived judgement, not a flag

`ProcedureCandidate.Verified` is always false and deliberately vestigial. `JudgeLowering` takes a
`CandidateAssessment` that has already been folded with stored rehearsal evidence, so a route may
be written down only while all of this holds, re-derived on every read:

- a whole route COMPLETED when it ran;
- the digest still matches, so it is the same demonstration;
- memory still recognises both endpoints;
- the application matches;
- no step needs text entry, and no step is a naked pointer press;
- every meaning is one Core has a sentence for.

Eleven discrete refusals. No lowering score — a play either says what the route does or it does
not exist.

---
## Decision 2 — the lowering input is meanings, and nothing else

`LoweringJudgement.Steps` is `[][]NavIntent`. That is the whole semantic description a play needs.

Deliberately absent: subject ids, screen states, group ids, window handles, process ids, window
generations, digests, verdicts, counts, coordinates, OCR strings, raw keys and rehearsal
bookkeeping. Not filtered out at the generator — **never handed to it**, because a field that
exists eventually gets printed.

An assertion on the judgement's own field names holds that: a `Digest` or `Checkpoint` field on the
type fails the test whether or not anything prints it.

---
## Decision 3 — the compiler is the authority, against the real act surface

Generated Marco goes through `lexer → parser → graph.Build → compile.Compile`, against the
canonical `internal/osmod/os.marco` — not the spec harness's convenient stub. A capability the
Director emits that the real act does not export must fail at compile time, which is the whole of
[[ADR-005-legal-marco-only]].

A meaning Core has no sentence for is a **language-expression gap**: reported, and that lowering
stops. Widening the language to make lowering convenient is the one thing this milestone must not
do — see the governance rule in `spec/Core.md`.

---
## Decision 4 — one actor, one verb, and no manufactured structure

A learned route needs an act (`use os.`), an actor, a verb and a script. It does not need a scene:
there is no place in the play, and manufacturing one to look like an AST would be exactly the
compiler-IR shape this milestone exists to avoid.

Names are **provisional and unmistakably so**: `UnnamedShortcut` and `Run`. Core v1 requires a
capitalised declaration name, and Director has no business inventing a meaningful one — guessing
from a screen's text would leak OCR into the language, and guessing from the application would name
the play after the wrong thing. `naming_required` was not needed: a provisional name is mechanically
safe precisely because the artifact is inert and nothing can ask for it by name.

Steps are ONE flat list. Where the demonstration paused is Director's structure, not the play's.

---
## Decision 5 — inert means inert

Lowering produces **text**. It writes no file, registers nothing, adds nothing to any resolution
path, and hands back nothing that can run. The test that matters snapshots the working tree before
and after and requires it byte-unchanged; the outward view carries no id, slug, path or phrase by
which a later request could reach the play.

`internal/director/marcoexec` cannot reach a runtime, a host, a platform adapter, a rehearsal grant
or the execution pipeline. **Generating Marco is not authorization to run Marco.**

---
## Decision 6 — provenance stays out of the source

Where a play came from is carried in the RESPONSE — the route, the refusals, the summary — beside
the source rather than inside it. The repository has no route-metadata sidecar mechanism, and this
milestone does not invent one: the artifact is not persisted, so there is nothing yet to attach
metadata to. **That is the gap the next milestone inherits**, and stuffing evidence ids into
comments would have hidden it while making the play less readable.

---
## Consequences

The sentence is now literally true: *Director watched and verified the behaviour. What it learned
is ordinary readable Marco.* And nothing more than that is true — persistence, naming, registration
and invocation are each separate, and each still needs a person.

## Enforced by

- `TestALearnedPlayCompiles` — the real compiler, against the real `os.marco`.
- `TestALearnedPlaySaysExactlyWhatItWasGiven` — order and repeats survive; `down, down` stays two.
- `TestALearnedPlayIsByteIdentical`, `TestTheSameProcedureAlwaysWritesTheSameMarco` — determinism
  across sessions, states, tracks, generations and map iteration.
- `TestALearnedPlayStaysInsideCoreAndOffTheBackstage`, `TestALearnedPlayHasNothingInItButThePresses`
  — Core vocabulary only, and nothing in the file but the presses.
- `TestAMeaningCoreCannotSayStopsTheLowering` — the language-expression gap is reported.
- `TestTheLoweringRefusalMatrix`, `TestLoweringEligibilityIsRecomputedNotRemembered`,
  `TestARouteMarcoOnlyWatchedCanStillBeWrittenDown`,
  `TestARevisedDemonstrationCanNoLongerBeWrittenDown`.

## Amended 2026-08-26 — the gate is whether the route is KNOWN, not whether Marco performed it

This ADR was written when a route had to be **rehearsed** before it could be written down, and its
lowering gate asked `CandidateAssessment.Verified` — which only a completed live rehearsal can set.
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] changed what *learned* means
and says in as many words that "a Play can now be saved that Marco has never executed". It changed
the Learn coordinator and it changed planning. **It did not change this gate**, so Fast Learn
produced every durable thing except the artifact — places, edges, candidates, the goal, and then
`no route is ready to be written down`.

Nothing caught it: every save test in `cmd/director` writes a rehearsal record into its fixture
first, and the test that named this gate asserted the pre-089 rule and passed. It was found in
Roadmap 36B, where the retrospective path made it unmissable. That test now states the corrected
rule under the name in the list above.

The gate now asks `CandidateAssessment.Writable()` — verified by a rehearsal, **or** cleanly
observed by ADR-089's own admission rule, and never when a rehearsal of the route is on record that
has not verified. Everything else in this ADR stands: the play is still compiled before anybody
sees it, still says WHAT rather than WHY, and is still not registered by being written.
- `TestWritingAPlayDownRegistersNothing` — the working tree is byte-unchanged and the view carries
  no handle.
- `TestTheLearnedGeneratorCannotReachAnythingThatActs`, `TestTheLearnedGeneratorCannotStartAProcess`.
- `TestAVerifiedRouteBecomesOrdinaryMarco` — the whole chain through the real protocol.
