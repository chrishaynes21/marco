---
type: milestone
status: complete
date: 2026-08-09
affects:
  - hypotheses
  - passive-observation
source_paths:
  - internal/director/observe/proposal.go
  - internal/director/observe/hypothesis.go
  - internal/director/observesession/runner.go
  - cmd/director/observeregistry.go
  - cmd/director/observecmd.go
---

# The proposal loop: asking, and what an answer is allowed to change

Canonical: [[Hypotheses]], [[ADR-015-a-question-is-evidence-not-settlement]], Roadmap item 9.

## What the audit found

Three existing question-shaped mechanisms, **none of them a fit**, and one principle worth
stealing:

| found | verdict |
|---|---|
| `service.ConfirmationBroker` | Blocks for an answer, scoped to one *command*, implements `execute.Confirmer`. Execution-time approval. Blocking contradicts "a pending question must not freeze observation", and a passive session is not a command with a watching client. |
| `service.Clarification` | Ambiguity resolution *during execution*. Also command-scoped. |
| `Insight.Validation` / `Hypothesis.Validation` | A string describing **how a person could settle it** — not user validation. A separate typed field was added rather than overloading it. |
| `observe.LiveRecorder` | Not a question mechanism, but it had already solved **materiality**, keyed on the evidence-set digest, with the failure recorded in-file: keying on a growing count republished an unchanged claim forever. Reused as a principle. |

Nothing uninvoked. The loop is genuinely new; the *channel* is the existing observation session
and its `ObserveQuery` request path.

## The shape

```
evidence → hypothesis → eligibility → question → answer → validation evidence → Result → CLI
```

- **Proposal generation:** `ProposalLedger.Refresh` in the runner's per-sample locked section,
  beside `foldShadow` and `live.Observe`. One call attaches existing validation *and* asks about
  newly eligible hypotheses — two halves of one idea, so there is a single mutation target.
- **Response consumption:** `ObserveQuery.Answer` → `Runtime.Observation` → `registry.Answer` →
  the active runner's ledger **or a finished session's stored Result**. The second is the ordinary
  case: a three-minute session finishes long before somebody reads the question.
- **CLI:** the question is printed *above* the evidence with the exact command to answer it.
  `director answer <session> <question-id> yes|no|not-now`.

## Three decisions worth keeping

**A confirmation does not clear a contradiction.** It adds `FromUser` support and promotes to
`validated` only when the record is clean. This is the one place a human answer could overwrite a
measurement, and it must not — a user agreeing does not un-observe the evidence that disagreed.

**A contradiction does not delete the support.** The observations stay listed under a `FromUser`
contradiction, so Marco can say *"I observed this repeatedly and you told me I was wrong"* —
which is the most useful sentence it could produce about its own model.

**A decline touches no semantic evidence at all.** It only stops the asking. Collapsing it into
"no" would let being busy become disagreement, and the symptom of that error is silence — which
is also what getting it right looks like.

## The identity defect, found by a failing test

Question identity originally included the interface terms. `TestADeclinedQuestionReturnsOnlyWhenTheEvidenceChangesShape`
failed with `Asked: 1` where it expected 2 — because adding a term changed the *identity*, so a
declined question came back as a brand-new one rather than a re-ask.

That is precisely the nagging the decline exists to prevent, and it would have shipped invisibly:
the user declines, OCR reads one more label a few seconds later, and the same question returns
wearing a different id.

Identity is now **structural only** — kind, subject kind, role composition, member count. Terms
moved to the evidence digest, where a change to them makes a declined question *materially new*
and re-asks it properly, recording `Asked: 2`.

The cost, accepted: two structurally identical screens classified the same way collapse into one
question. If Marco cannot tell them apart from structure, it is the same question about both.

## Mutations — seven, and two initially survived

| mutation | result |
|---|---|
| **1.** delete the production proposal-generation call | 3 production tests fail |
| **2.** answer a throwaway copy, never the stored ledger | 2 production tests fail |
| **3.** collapse `declined` into `contradicted` | 2 tests fail |
| **4.** bind the answer to the earliest open question, not the id | **survived at first** |
| 5. allow a contested hypothesis to be asked about | **survived at first** |
| 6. remove the eligibility status gate entirely | `TestATentative...` fails |
| 7. let a confirmation clear a contradiction | `TestConfirmationDoesNotClearAContradiction` fails |

**Both survivors were real test gaps, not false alarms.**

Mutation 4 survived because the test answered the *first* of two open questions, which any
"earliest open" implementation also gets right by coincidence. The test now answers the **second**,
and the mutation fails it.

Mutation 5 survived because `contested` implies contradictions by construction, so the
no-contradictions gate caught it and the status gate never ran. Mutation 6 — removing the status
gate outright — proved it *is* load-bearing for `tentative`. The overlap is recorded in the ADR
rather than tidied away.

## Known gaps

- **No cross-session identity.** A validated hypothesis dies with its session; the same screen
  next run is a new subject and the user is asked again. This is Roadmap item 10 and is the
  largest remaining limitation of the loop.
- **No free-form correction.** The response vocabulary is a closed enum. A user who wants to say
  *"it's not settings, it's the audio page"* cannot, deliberately: a note field is how a record
  becomes a place to put anything, and prose corrections need their own privacy design.
- **One question per session in practice.** `MaxOpen` is 1 and nothing re-asks after an answer,
  so a session yields at most one interruption. Conservative on purpose; revisit with evidence
  about whether users answer at all.
- **Never exercised live.** Proven by scripted sessions, production-path tests, replay and seven
  mutations. Nobody has yet answered a real question about a real game.
