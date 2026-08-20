---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - semantic-memory
  - demonstrations
  - hypotheses
---

# A judgement is recomputed, not recorded

## Context

[[ADR-020-watch-me-is-permission-to-observe-not-to-act]] produced a `ProcedureCandidate`: one
bounded, authorised demonstration of an approved transition. It said nothing about whether that
demonstration is worth anything.

The obvious move — write a verdict onto the candidate when it completes — is wrong for one
specific reason. What Marco can conclude from an observation depends on what Marco **remembers**,
and memory improves. A transient checkpoint nobody could recognise today becomes a remembered
subject the moment the user confirms what that screen is, and the same demonstration becomes more
verifiable without a single new observation. A verdict written into the record would freeze a
judgement made when Marco knew less, and nothing downstream would ever question it.

## Decision

**The candidate is the observation. The assessment is a judgement over it, recomputed from the
candidate plus the current topology, every time it is read.**

```
ProcedureCandidate  (durable, fixed, what happened)
      +
Topology            (durable, improving, what Marco can recognise)
      ↓
CandidateAssessment (derived, never stored)
```

Nothing durable holds a verdict — enforced by a test that fails if `ProcedureCandidate` ever
grows a `Verdict`, `Assessment` or `Reasons` field. The dependency on memory is a **parameter**
rather than something hidden, which is also what makes replay parity statable: the same candidate
and the same topology give the same assessment, and there is no third input to go looking for.

### Four verdicts, and the strongest one is deliberately weak

| verdict | means |
|---|---|
| `candidate_consistent` | the observation hangs together and every checkpoint could be checked again |
| `insufficient_evidence` | coherent, with a gap Marco cannot currently close |
| `ambiguous` | the navigation has no clear shape — long runs, backtracking |
| `invalid` | describes nothing Marco can recognise, or never finished |

`candidate_consistent` is the ceiling. Not `plausible`, which reads as "probably works"; not
`verified`, because nothing has been reproduced. **No confidence float** — the useful output is
the list of things Marco cannot check, because that list is actionable and a number is not.

### The central question is verifiability, not performability

For every step: *could Marco later tell whether it succeeded?* A procedure Marco could perform
and could not check is blind replay, and the whole perception stack exists so execution never has
to be. Coverage is reported as a **list** of checkpoints with their gaps named, not as a
percentage — "70% verifiable" tells a reader nothing they can act on; "the second screen cannot
be recognised" tells them exactly what to fix.

### Closed reasons, each saying whether repetition would help

`single_demonstration_only` · `incomplete_demonstration` · `start_unverifiable` ·
`end_unverifiable` · `transient_checkpoint_unverifiable` · `requires_text_entry` ·
`unresolved_pointer_target` · `ambiguous_navigation_run` · `backtracking_run` ·
`near_capture_bound` · `no_steps`.

`ResolvableByDemonstration()` splits them, and that split is the whole preparation for the next
milestone without asking the user for anything now: an ambiguous run is something a cleaner
example would settle; a transient checkpoint and a text-entry boundary are not — one needs the
user to name a screen, the other needs consent and a representation.

`single_demonstration_only` is always present and is deliberately **not** a downgrade. It is the
honest ceiling on every assessment this layer can produce, and if it blocked the best verdict then
the best verdict would be unreachable and meaningless.

### Volume is not strength

A demonstration near a capture bound may be missing the end of itself, so `near_capture_bound` is
a reason for *less* confidence. There is no "more steps → stronger" relationship anywhere.

### Comparison is semantic, and tolerant

Two demonstrations are compared on endpoints, the checkpoint sequence (durable id where there is
one, safe structure where there is not, so a screen that has since been recognised still matches
the transient one it was), the text-entry markers, and the **decisive** navigation of each run.

Directional intents — up, down, left, right — move a selection and commit to nothing, so
`down, down, confirm` and `down, down, down, confirm` are the same move made one row further
away: `compatible`. Everything else is decisive, so `left, back, down, confirm` does not reduce to
`confirm`: `different`. Deliberately not an edit distance over raw input, which would call every
honest repeat different.

Four outcomes: `same`, `compatible`, `different`, `incomparable` — the last for two demonstrations
of different relationships or one that never finished, held apart for the usual reason.

### Nothing about causality

One demonstration establishes that the user did X and then the screen became Y. The vocabulary
stays observational throughout: no step is `required`, no intent is "the action that opens X".

## Consequences

- Marco can say: *I watched one example. It is internally consistent. I could verify these
  checkpoints if I met them again. I cannot yet verify this one. A second example would tell me
  whether these steps are reproducible.*
- It cannot say it knows how, can do it, or learned a procedure. The best verdict leaves
  `Verified` false, and nothing in this layer can set it.
- An old demonstration gets better as memory improves, at no cost and with no new observation.

## Enforced by

- `TestACompletedDemonstrationIsAssessedOnTheProductionPath` — THE wiring test, which never calls
  `AssessCandidate` itself. Deleting the production call fails it.
- `TestAnAssessmentImprovesWhenMemoryDoes` — the same demonstration judged better once its middle
  screen becomes recognisable.
- `TestAStoredDemonstrationIsJudgedFreshlyEveryTime` — nothing durable carries a verdict, and
  recomputation is deterministic over its two real inputs.
- `TestTheAssessmentTable` — nine adversarial cases: clean, transient checkpoint, text entry,
  pointer-only, over-long run, backtracking, missing start, missing end, incomplete.
- `TestADemonstrationNearItsBoundIsWeakerNotStronger`,
  `TestTheAssessmentSaysWhetherAnotherExampleWouldHelp`.
- `TestTwoDemonstrationsAreComparedSemantically` (eight sub-cases) and
  `TestATransientCheckpointStillMatchesItsRecognisedSelf`.
- `TestAnAssessmentGrantsNoAuthority`, `TestNoRawInputCanReachAnAssessment`.
