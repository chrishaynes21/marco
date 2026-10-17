---
type: adr
status: proposed
date: 2026-10-17
decision: PROPOSAL ONLY, not implemented — an external interpreter may be consulted only where the deterministic reading returned ABSENT, its proposal re-enters the resolver as one more candidate under the same actionability and policy rules, and it is never asked about a resolved answer or a tie; the model is chosen on abstention rather than accuracy, and a held-out loss of zero is the condition for wiring it in at all
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-223-proposed-an-interpreter-asked-only-where-nothing-matched.md
---

# ADR-223 (proposed) — an interpreter asked only where nothing matched

**Nothing here is implemented.** This is the narrowest safe incorporation the evidence
supports, written to be argued with. The interpreter remains in shadow mode, reachable only by
the evaluation, held there by `cmd/director TestTheInterpreterIsNotWiredIntoAnything`.

## The measurement this answers

[[Experiment-072-the-better-model-at-meaning-is-the-more-dangerous-one]], on three local
models against held-out applications, consulted only where the deterministic reading found
nothing:

| | gained | lost |
|---|---|---|
| llama3.2:3b | +3 | −4 |
| llama3.1:8b (highest accuracy, 69%) | +6 | **−6** |
| qwen2.5:7b | +4 | **0** |

And one finding no model escaped: **not one of the three preserved a single genuine ambiguity**
— 6 of 6 answered on the held-out set, across two model families.

## The decision proposed

```
resolve (deterministic) ──┬─ RESOLVED   ─────────────────────► act.  NOT ASKED.
                          ├─ AMBIGUOUS  ─────────────────────► ask.  NOT ASKED.
                          ├─ UNOBSERVABLE ───────────────────► look again. NOT ASKED.
                          └─ ABSENT ──► reword (ADR-218) ──┬─ answered ─► NOT ASKED.
                                                            └─ still absent
                                                                   │
                                                              INTERPRETER
                                                                   │
                                                         a Prop id + a sentence
                                                                   │
                                            back through the resolver's own rules
                                            ──► RESOLVED (interpreted) │ ABSENT
```

Four properties, and each is a refusal the evidence demanded:

1. **ABSENT only.** Not a confidence threshold, not "when the score is low", not "when the
   model is more sure". The gate is the one place where the deterministic path has already
   said it has no answer, and under it WOULD BREAK is zero *by construction* rather than by
   measurement: an interpreter that is never asked about a resolved answer cannot overrule
   one. Measured: llama3.2:3b and llama3.1:8b each broke correct answers when asked
   everywhere, and llama3.1:8b broke `the bluetooth switch` — a request whose own words said
   which control was meant.
2. **Never asked about a tie.** The second half of the gate, and the finding that forced it.
   Every model answered every ambiguity. A model given a list and asked which item is meant
   will pick one; "two of these are equally good and the person should choose" is not an
   answer the question has room for. Ambiguity stays where it is decided today — from
   evidence, in the resolver ([[ADR-221-a-question-the-person-already-answered]]).
3. **It proposes a candidate, not a target.** The id re-enters the resolver and faces the same
   actionability rules, the same operability licence, the same policy and the same capability
   ladder. Already true of the shape: `meaning.Proposal` is an id and a sentence, with nowhere
   to put a coordinate, a permission or an action.
4. **It is marked and discounted.** `Resolution.Reworded` already carries the sentence that
   made a non-literal answer visible; an interpreted answer carries the interpreter's own
   reason in the same field, at a confidence below any reworded one. A person reading a wrong
   answer must be able to see which step produced it — the words, the rewording, or the model.

## How a model is chosen, which is the part that would be got wrong

**On abstention, not accuracy.** The highest-accuracy model of the three is the one that would
have converted all six honest refusals into confident wrong actions. Had the choice been made
on the headline figure it would have been the wrong one, by six, on applications nobody had
looked at.

So the condition for wiring an interpreter in at all is:

- **`lost` is zero** on held-out applications. Not low. Zero. A refusal turned into a wrong
  action is the failure a person cannot see happening.
- `gained` is meaningfully above zero — four of seven reachable paraphrases, for qwen2.5:7b.
- The run is **reproducible**. llama3.2:3b's proposals moved between runs at temperature 0;
  qwen2.5:7b produced identical output twice. An advisory layer whose answers are not
  reproducible cannot be cleared by an evaluation, because the evaluation that cleared it would
  not be the one that ships.
- A **model swap re-runs the oracle**. Changing `$MARCO_LLM_MODEL` is one environment variable
  and must not be a way to change what Marco will press without measuring it.

## What this is NOT proposed to do

- **Not to raise the deterministic bar.** Marco's 83% held out comes from rules whose evidence
  is the person's own sentence and the screen's own words. The interpreter is for the
  remainder, and the remainder is meaning — `dark mode`, `bookmarks`, `themes`, `wifi`.
- **Not to be asked on every request.** ~1.1s and 53 described controls per request, against
  ~60µs for the whole deterministic path. Under the gate it is consulted on the small minority
  of requests nothing else can answer.
- **Not to decide satisfaction.** Whether a request is already done is read from the
  application's own report, and a model has no standing to say a request is complete.
- **Not to become a second interpreter.** One executable, one question, one switch
  ([[ADR-220-asking-what-a-request-meant-and-believing-none-of-it]]).

## Open questions before building it

1. **What does an interpreted answer do to learning?** A demonstrated surface is stored against
   a target `{label, kind, place}`. If the request reached its control through a model, the
   label stored must be the CONTROL's, not the person's phrasing and not the model's reason —
   or memory fills with one machine's model's vocabulary. ADR-217 asked this about rewording and
   it matters more here.
2. **Should an interpreted answer require confirmation?** It is the weakest claim in the system
   and the risk ladder already carries the concept. A policy input, not a resolver decision.
3. **Is zero `lost` achievable on a fourth application?** Two held-out families is not many.
   The honest answer is that the gate must be measured again on each new surface before it is
   trusted there.
4. **Does the absent-only gate hide a better arrangement?** It cannot help where the literal
   reading answers CONFIDENTLY AND WRONGLY — the defect [[ADR-219-a-match-is-not-an-answer]]
   fixed deterministically. If that class recurs in a form the kind-noun rule cannot reach, the
   gate is the wrong shape and the answer is still not to let a model overrule an answer.

## Enforced by

Nothing. This is a proposal; the tests belong with the implementation. The arrangement it
describes is measured today by `director assess-rewording --interpreter`, whose
"if asked only where ABSENT +N / −N" line is the specification for the condition above.
