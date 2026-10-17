---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the committed desktop perception corpus — Windows Settings on two pages to build against, File Explorer, Microsoft Edge and Paint held out — plus llama3.2:3b running locally through Ollama as the interpreter. Seventy-six ordinary instructions with oracles written down first. No desktop was touched: nothing executed, no window raised, no input watched or injected, and nothing the interpreter proposed was acted on."
result: fixing-a-match-is-not-an-answer-raised-settings-to-82-percent-with-extra-word-phrasing-at-11-of-11-on-both-sets-and-the-shadow-interpreter-came-out-a-wash-four-added-four-broken-with-abstention-that-did-not-transfer-to-held-out-applications
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-219-a-match-is-not-an-answer.md
  - docs/decisions/ADR-220-asking-what-a-request-meant-and-believing-none-of-it.md
  - internal/director/meaning
---

# Experiment 070 — the interpreter knows some meaning, and does not know what it does not know

Two changes, measured separately on purpose:
[[ADR-219-a-match-is-not-an-answer]] in the deterministic decision boundary, and
[[ADR-220-asking-what-a-request-meant-and-believing-none-of-it]] as a shadow experiment that
influences nothing.

## Part one: a match is not an answer

The regression case from [[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]]:
`"open bluetooth settings"` resolving confidently to the Bluetooth on/off switch at 0.485,
because the label "Bluetooth" is contained in the request. The reading that would have answered
correctly existed and never ran, because the literal answer arrived first.

The fix reconsiders a resolved answer only when the request named a KIND of control and the
chosen control's role conflicts with it. All of its evidence comes from the person's own
sentence.

| | Settings | held out |
|---|---|---|
| correct before | 24 / 33 | 31 / 39 |
| **correct after** | **27 / 33 (82%)** | **31 / 39 (79%)** |
| wrong control | 0 | 0 |
| extra-words phrasing | 8/8 → **11/11** | **11/11** |
| literal answers reconsidered | 3 | 0 |

Three answers changed, all three the Bluetooth case, all three to the control the oracle named.
Nothing changed on the held-out applications — the right result there, because the defect was
specific rather than everywhere.

Three regression cases were added to prove it is a reading of the person's sentence rather than
a taste for navigation items, and all three pass:

- `"the bluetooth switch"` and `"the bluetooth toggle"`, same page, almost the same words,
  **the opposite answer** — the check must not fire, because a switch is what somebody who
  says switch means.
- `"the rotate panel"` in Paint, held out: "panel" names a destination, Paint has none, the
  check fires, finds nothing that fits, and must leave the right answer exactly where it was.

### The gap left open on purpose

Paint's `Zoom` slider beside its `Zoom` combo box, asked for as `"the zoom slider"`, still
ASKS. The person named the kind that distinguishes them, so their own sentence could settle it
— but the check only reconsiders a RESOLVED answer, deliberately, because the instruction for
this work was to preserve existing ambiguity behaviour. It is in the held-out set with that
reasoning beside it, scored as "asked unnecessarily", and it is the next small thing to decide
rather than a failure.

## Part two: the interpreter, in shadow

`llama3.2:3b`, local, through Ollama. Every case also put to it; nothing it said was acted on.

| | Settings (tuned) | held out |
|---|---|---|
| Marco's own answers | **27 / 33 (82%)** | **31 / 39 (79%)** |
| interpreter correct | 18 / 33 (55%) | 21 / 39 (54%) |
| wrong-control proposals | 7 | 12 |
| of those, nothing on screen matched | 0 | 4 |
| ambiguities answered without asking | 2 / 2 | 4 / 5 |
| abstained | 6 / 36 (17%) | 2 / 40 (5%) |
| **WOULD ADD** | 0 | **4** |
| **WOULD BREAK** | 2 | **4** |
| cost | ~300ms and 30 controls described per request | ~300ms and 53 per request |

What it would have added, held out: `dark mode` → Theme. `turn off notifications` → Send
notifications. `the zoom slider` → the slider. `refersh` → Refresh. Three of those are real
paraphrases, which is exactly the gap it was brought in for, and the fourth is a transposed
spelling the one-edit rule cannot reach.

What it would have broken, held out: `the Storage section`, `the rotate panel`, `the view
button`, `make a new folder` — four answers Marco already had right.

**ADD 4, BREAK 4.** A wash, bought with four correct answers overruled. It is not wired in,
and on this evidence it should not be.

### The finding that matters more than the accuracy

It does not know what it does not know, and the attempt to teach it did not transfer.

On the first run it abstained **0 times out of 36** — including all five cases where the
control genuinely was not on the screen, each answered confidently with something else. The
model was not disobeying: it had been asked "which of these" and given a list with no way to
say "not these".

So one bounded change: offer "nothing here is what the user asked for" as an ITEM IN THE LIST
rather than as a rule in the instructions. Abstention on absent controls went **0/5 → 5/5 on
Settings**, the set the prompt was tuned against. On the held-out applications it is **2/6**,
and overall abstention there is 2 in 40.

That is the held-out discipline earning its place for the second experiment running. A number
measured only on the application the work was aimed at would have read as "abstention solved".

And it answers 4 of 5 genuine ambiguities without asking — the *Apps* failure, from a different
direction. Marco preserves all five.

### One more thing a shadow run showed that a wired-in one would have hidden

The proposals **move between runs** at temperature 0. Across three runs the held-out absent
column read 1/6, 1/6 and 2/6, and Settings' WOULD ADD read 2, 1 and 0. An advisory layer whose
answers are not reproducible is a third reason not to let it choose a control yet: the
evaluation that cleared it would not be the evaluation that ships.

## What this bounds

**This interpreter**, which is a 3B model on one machine. It says nothing about a larger one,
and the whole arrangement exists so that finding out costs one environment variable and one
re-run — no code, no new boundary, no new thing to configure. The three capabilities now stand
apart and can be measured apart: intent parsing, observation, and the join.

## The recommendation

1. **Keep the deterministic boundary fix.** It is small, its evidence comes from the person's
   sentence, it cannot refuse or ask or lose an answer, and it moved the application it was
   aimed at from 73% to 82% with extra-word phrasing at 11/11 on both sets.
2. **Do not wire this interpreter into control selection.** ADD 4 / BREAK 4, four confident
   proposals for controls that were not there, four ambiguities answered, and answers that move
   between runs.
3. **Try a better model before deciding about interpretation in general.** The measurement is
   now one command, and what it would take to pass is written down: ADD well above BREAK, zero
   proposals for absent controls, and ambiguities preserved. On the evidence here, abstention is
   the property to select a model for — not accuracy.
4. **The ambiguity-with-a-named-kind case is the cheapest thing left.** `"the zoom slider"`
   needs no model: the person said which kind, and that is distinguishing evidence by the rule
   [[ADR-218-a-request-reworded-in-the-screens-own-words]] already uses. It was left alone here
   only to keep ambiguity behaviour untouched.
5. **Already-satisfied is still unmeasured and still cheap.** One capture with `selected`
   recorded settles it. "Apps is already open" remains the thing that would demonstrate
   reasoning about the computer's state rather than translation of a sentence.

## Clauses not independently killable, reported rather than papered over

Mutation on the new code: 12 mutants, 9 killed, 2 refused by the compiler. One survived — the
clause requiring a reconsidered replacement to FIT the kind that was named. Nothing in either
corpus produces a re-asking that resolves to a different control whose role also conflicts, so
removing the clause changes no measured outcome. It is a guarantee held by reading and by the
harness's own check, which fails the run if a non-fitting replacement ever appears. Writing a
fixture contrived enough to exercise it would have been a test about the test.

## Enforced by

`cmd/director` `TestRewordingNeverChoosesTheWrongControl` runs both expanded sets as a gate,
with the no-regression rule now precise rather than absolute: a literal answer may change only
where its role conflicted with the kind named and the replacement fits.
`TestTheInterpreterIsNotWiredIntoAnything` holds shadow mode;
`TestTheEvaluationNeedsNoInterpreter` keeps the gate independent of any model. Re-run the
report with `director assess-rewording [--verbose] [--interpreter]`.
