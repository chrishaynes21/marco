---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
  - vision
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: five-of-eight-screens-bound-one-ambiguous-two-absent-and-three-of-four-controls-found-again-from-the-file-alone
supersedes: []
source_paths:
  - internal/director/workflowstore/workflowstore.go
  - internal/director/observe/reacquire.go
  - cmd/perceptionreplay/reload.go
---

# Experiment 033 — a workflow asked for in another session

**Status: RUN offline, 2026-10-13, on the existing recording and recorded detections. No new
capture, no inference during the measurement, no actuation, nothing executed.**

## The question

The owner's next milestone, after durable storage landed:

> I'd ask Opus to prove that a newly initialized Director can load the saved workflow, establish
> its current visual context, and safely resolve the next action.

With the constraint that is the whole difficulty:

> the binder must not become a slightly more elaborate version of the arrangement matcher that
> previously produced two wrong bindings.

## How it was arranged

Three observers over one recording, the shape
[[Experiment-029-binding-a-screen-across-two-independent-folds]] established:

```
fold A    looks 1-450    learns 11 steps and SAVES them
fold B    looks 451-900  a fresh observer that has never heard of fold A
oracle    all 900        the only thing that knows which screens are really the same
```

A and B share no counter, no registry and no screen identity. **The artifact crosses between them
as 8,511 bytes and nothing else.**

**The oracle supplies the correspondence**, standing exactly where a person would say "that is the
main menu". That is not a cheat and it is not recognition — it is the owner's own instruction:

> A useful first implementation could be deliberately conservative: it may require externally
> supplied screen correspondence and use the stored evidence to validate or reject that
> correspondence.

So what is measured is not whether Marco can recognise a screen. It is whether a saved workflow
can be reconstructed, **validated**, bound, and its controls found again by a session that never
saw the demonstration.

## 1. Validating the correspondence

```
screen_a  ambiguous       visual_1=0.787  visual_8=0.653  visual_7=0.600
screen_b  bound           visual_2=0.901
screen_c  no-counterpart  (nothing proposed)
screen_d  bound           visual_3=0.611
screen_e  no-counterpart  (nothing proposed)
screen_f  bound           visual_4=0.688
screen_g  bound           visual_5=0.556
screen_h  bound           visual_6=0.833

5 bound, 1 ambiguous, 2 with no counterpart
```

**`screen_a` is the result worth having.** Three of fold B's screens clear the floor, the best at
0.787 — and the binder refuses. A best-match rule would have taken `visual_1` and called it
established, which is precisely the rule Experiment-029 measured producing two confidently wrong
bindings out of seven. Here the conservative rule is not hypothetical restraint; it fires, on the
first real screen, in the direction that costs nothing but an answer.

The two `no-counterpart` screens are honest absence: fold B never visited them.

### And the workflow as a whole is therefore NOT bound

```
"enter free play" cannot be bound: nothing has been established about
[screen_a screen_c screen_e]
```

That is the right answer and not a failure. A workflow that bound what it could would plan three
steps and then discover it has no idea where step four goes — acting before finding out.

## 2. Finding the saved controls again, from the file's own readings

No session registry is involved. `observe.ReacquireStored` matches the readings the artifact kept
against what fold B's proposer found, through the same matcher the live registry uses — one
matcher, two callers, so the offline number means something about the live path.

```
control    on          looks  matched  ambiguous  unobservable
control_2  visual_2       28        0          0            28
control_4  visual_3       54       52          2             0
control_6  visual_4       90        0          0            90
control_7  visual_5       19        2          0            17

2 of 4 saved controls on a bound screen were found again
```

**`control_4` matched on 52 of 54 looks of its screen** — a control demonstrated in the first half
of a recording, written to a file, and found again by an observer that started from nothing. That
is the first time anything in Marco has done that.

`control_2` and `control_6` are the answer to "which require stronger visual identity evidence":
both had plenty of looks on a bound screen and matched on none.

## Corrected: this under-reported, and the harness was why

[[Experiment-034-two-controls-and-why-each-was-lost]] found a defect in how the numbers above
were taken. Re-acquisition was run as a second pass that asked fold B's **finished** observer
what was on each look — and an observer holds admitted detections until its observation moves on,
so after a fold its held set is the last look's. Every look was offered the end of the recording.

Run inside a fold, as the live path does, the result is **3 of 4 controls, not 2**:

```
control_2  visual_2   28 looks   21 matched   (was 0)
control_4  visual_3   54 looks   52 matched
control_6  visual_4   90 looks    0 matched
control_7  visual_5   19 looks   12 matched   (was 2)
```

The binding numbers above are unaffected: they are computed from the folds' own arrangements and
never touched the held set.

## What this establishes, and what it does not

**Established.**

- A durable artifact is sufficient to reconstruct a workflow's steps, screens and control
  evidence in a session that never saw the demonstration.
- Stored-evidence reacquisition works, through the production matcher, with no registry.
- The conservative binder refuses where a best-match binder would have been wrong — demonstrated
  on real data rather than argued.
- Every refusal survives the round trip: unbound, unconfirmed, unexecutable.

**Not established, and some of it is now measurable for the first time.**

- **Nothing was recognised.** The correspondence was supplied. Validation can REJECT a
  correspondence the evidence contradicts; it cannot confirm one. Agreement is necessary and not
  sufficient, and Experiment-029 is why.
- **Two of four controls could not be found again** on screens that were bound. Whether that is
  the proposer, the screens being genuinely different, or the readings being too thin is the next
  question and is not answered here.
- **Nothing was executed, planned or confirmed.** The workflow never became bound, so there was
  nothing to confirm.
- **Fold B is not another day.** Same resolution, same window, same build, same theme. It is a
  fresh observer, which is a weaker claim than a fresh session and a much weaker one than
  tomorrow.

## What remains before a first authorised execution experiment

1. **A complete binding.** On this split, three screens could not be established. A run needs all
   of its screens or it needs to refuse.
2. **The two unresolvable controls**, or a workflow whose steps avoid them.
3. **Fresh confirmation**, per session, by a person. Built and untested against a bound workflow,
   because nothing has been bound yet.
4. **Live execution authorisation and a destination to verify against** — unchanged, unbuilt, and
   the only remaining piece that can cause an effect.

## Enforced by

- `internal/director/workflowstore` — `TestAProposedCorrespondenceIsValidatedOrRefused` (the
  three outcomes), `TestABetterScoreIsNotATieBreak` (two qualifying candidates refuse however
  they score), `TestTargetsCarryTheirEvidenceToTheBoundScreen`,
  `TestEachScreenGetsItsOwnLabelInWalkOrder`, `TestAPartiallyBoundWorkflowIsRefusedEntirely`,
  and the round-trip, corruption, versioning, provenance and authority tests from
  [[ADR-183-remembering-recognising-and-being-allowed-to-act]].
- `internal/director/observe` — `ReacquireStored` is the registry's own matcher, so the existing
  reacquisition tests cover both callers.
- **Mutation: 27 of 27 killed.**
