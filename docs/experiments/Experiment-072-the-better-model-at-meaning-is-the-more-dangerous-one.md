---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the committed desktop perception corpus with its two evaluation sets — Windows Settings to build against, File Explorer, Microsoft Edge and Paint held out — and three local models through Ollama on one RTX 5070: llama3.2:3b (2.0GB), qwen2.5:7b (4.7GB), llama3.1:8b (4.9GB). Shadow mode throughout: nothing any model proposed was acted on."
result: the-model-with-the-highest-accuracy-turns-every-honest-refusal-into-a-wrong-action-and-the-only-model-that-gains-without-losing-under-an-absent-only-gate-is-the-one-that-abstains-while-no-model-preserves-a-single-ambiguity
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-220-asking-what-a-request-meant-and-believing-none-of-it.md
  - docs/decisions/ADR-223-proposed-an-interpreter-asked-only-where-nothing-matched.md
---

# Experiment 072 — the better model at meaning is the more dangerous one

Three interpreters against the same oracle, the same held-out applications and the same
shadow-mode arrangement as
[[Experiment-070-the-interpreter-knows-some-meaning-and-does-not-know-what-it-does-not-know]].
No prompt was changed between models.

## The held-out numbers

43 cases, on applications nothing was tuned against. Marco's own deterministic answers:
**35/42 (83%)**, zero wrong controls.

| | llama3.2:3b | qwen2.5:7b | llama3.1:8b |
|---|---|---|---|
| correct | 22 (52%) | 25 (60%) | **29 (69%)** |
| wrong-control proposals | 12 | **2** | 7 |
| of those, nothing on screen matched | 4 | **0** | 6 |
| ambiguities answered without asking | 6 / 6 | 6 / 6 | 6 / 6 |
| abstained | 2 (5%) | **9 (21%)** | 0 (0%) |
| paraphrase | 2/7 | 3/7 | **6/7** |
| WOULD ADD | 3 | 4 | **6** |
| WOULD BREAK | 4 | 1 | **0** |
| latency | ~0.2s | ~1.1s | ~0.9s |
| on disk | 2.0GB | 4.7GB | 4.9GB |

## The number that decides it

The only arrangement that could ever be safe is to consult an interpreter **where the
deterministic reading found NOTHING** and nowhere else. Under that gate it cannot override a
correct answer and cannot settle a tie, because it is never asked about either — WOULD BREAK
goes to zero by construction, and two numbers are left:

| held out, asked only where ABSENT | gained | **lost** |
|---|---|---|
| llama3.2:3b | +3 | **−4** |
| llama3.1:8b | +6 | **−6** |
| **qwen2.5:7b** | **+4** | **0** |

`gained` is a control that WAS on screen, that Marco refused, and that the interpreter named
correctly. `lost` is a control that was NOT there, that Marco correctly refused, and that the
interpreter named something for anyway.

**llama3.1:8b has the highest accuracy of the three and loses on every single one.** All six
held-out absent cases — Print, Save, the history panel, Downloads, Export, the airbrush tool —
got a confident proposal. It never abstained once in 43 requests, nor in 37 on the other set.

**qwen2.5:7b gains four and loses none**, twice in a row with identical output. Four
paraphrases Marco cannot reach — `dark mode` → Theme, `turn off notifications` → Send
notifications, `make the brush bigger` → Size, and `refersh`, the transposed spelling the
one-edit rule cannot cross — and not one proposal for a control that was not there.

## What this says, and it is not "bigger is better"

**Accuracy and safety came apart, and they came apart in the direction that matters.** The
model that understood the most meaning was the one that could not tell when there was no
meaning to find. Judged on how many requests it answers, llama3.1:8b is the best of the three
by nine points. Judged on whether it may be allowed to act, it is the worst of the three by
six.

So the property to select a model for is **abstention**, not accuracy — which is what the
owner asked for before these numbers existed, and the numbers are as clear an endorsement of
that instruction as a measurement can be. Had this been chosen on the headline figure, the
choice would have converted six honest refusals into six confident wrong actions on
applications nobody had looked at.

## And one finding no model escapes

**Not one of the three preserved a single genuine ambiguity.** 6 of 6 answered on the
held-out set, 3 of 3 on Settings, across three models and two families. Explorer's toolbar
`Details` beside its view-mode `Details`; its column header `Name` beside the cells; Edge's two
`Close` buttons; Paint's `Zoom` slider beside its `Zoom` combo box — every one answered without
asking.

That is not a weakness of one model. A model given a list and asked which item is meant will
pick an item; "two of these are equally good and the person should choose" is not an answer the
question has room for. Marco preserves all of them, deterministically, from the evidence.

**So an interpreter must never be asked about a tie** — not as policy, but as plumbing. It is
the second half of why the gate in
[[ADR-223-proposed-an-interpreter-asked-only-where-nothing-matched]] is on ABSENT rather than
on a confidence threshold.

## Cost

~1.1s per request for either 7B/8B model against ~0.2s for the 3B and ~60µs for the whole
deterministic path. 53 controls described per request on the held-out set, which is the entire
input an interpreter is charged for. 4.7GB on disk, and it shares a 12GB card with whatever the
person is running — a game was open throughout these runs.

A second's wait is affordable for a request that would otherwise be refused. It would not be
affordable on every request, which is the third reason the gate is where it is: under it, the
interpreter is consulted on the small minority of requests the deterministic path cannot
answer.

## The recommendation

1. **qwen2.5:7b is the model to carry forward**, on abstention rather than accuracy: +4 / −0
   held out, stable across runs, 6/6 on absent controls.
2. **Implement the gate before the interpreter**, exactly as
   [[ADR-223-proposed-an-interpreter-asked-only-where-nothing-matched]] describes it — ABSENT
   only, re-entering the resolver as a candidate, marked and discounted, never consulted about
   a tie.
3. **Keep the oracle as the gate on the gate.** A model swap is one environment variable; what
   must not be one is a model swap without a re-run. `lost` must be zero on held-out
   applications, every time.
4. **Do not raise the deterministic bar to meet the model.** Marco's 83% held out comes from
   rules whose evidence is the person's own sentence and the screen's own words. The
   interpreter is for the remainder, and the remainder is meaning.

## Enforced by

Nothing is wired in, so nothing here is a behavioural gate;
`cmd/director TestTheInterpreterIsNotWiredIntoAnything` holds that, and
`TestTheEvaluationNeedsNoInterpreter` keeps the committed evaluation independent of any model.
Re-run with `MARCO_INTERPRETER=... MARCO_LLM_MODEL=... director assess-rewording --interpreter`.
