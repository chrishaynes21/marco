---
type: adr
status: accepted
date: 2026-10-17
decision: an optional external interpreter may be asked which OBSERVED control a request meant — a third question on the existing plugin protocol, behind its own switch because it is the only one told what is on screen — and in this experiment nothing acts on the answer: the only caller is the evaluation that scores the proposals against an oracle and throws them away
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - internal/resolver/which.go
  - internal/director/meaning
  - plugins/llama/which.go
  - cmd/director/assessinterpreter.go
---

# ADR-220 — asking what a request meant, and believing none of it

## Why there is a question here at all

[[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]] measured the
boundary of a deterministic reading exactly. Wrapping words and spellings close — 11/11 and
3/4 on applications nobody tuned against. PARAPHRASE closes at one in thirteen: `themes` for
Personalization, `bookmarks` for Favorites, `dark mode` for Theme, `wifi` for Network &
internet.

Every one fails for the same stated reason — rewording may only ask with words the screen is
showing, and none of those words is on the screen. They are knowledge questions. No further
lexical rule answers them, and the recommendation was to stop trying.

## The decision, in two halves

### One: a third question, behind its own switch

The same plugin protocol — one executable, one JSON object in on stdin, one out on stdout —
because one boundary between Marco and a model is worth keeping.

```
→ {"input":"open dark mode","which":{"application":"msedge","controls":[
     {"id":"e12","role":"combo_box","label":"Theme"},
     {"id":"e18","role":"checkbox","label":"Send notifications"}]}}
← {"control":"e12","because":"a theme setting is where a dark appearance is chosen"}
← {"control":""}                                        (nothing here fits — an abstention)
```

**`$MARCO_INTERPRETER`, not `$MARCO_RESOLVER`**, and that is the first decision rather than a
detail of it. The existing two questions are never told anything about the screen, and
`internal/resolver`'s own note says why in as many words: the target travels on as the user's
PHRASE, and Marco's own resolver decides what it refers to against a world it observed itself.
This question cannot be answered without crossing that boundary. A plugin somebody configured
to pick between their saved play names has not agreed to be sent the labels of whatever is in
front of them, and widening what an existing setting means would be taking that decision for
them. Unset — the default — means the question is never asked.

What is sent is bounded and the bound is the shape of the type, not a rule in a comment. Three
fields per control: id, role, label. Only ADDRESSABLE controls — named, operable, enabled, on
screen — capped at 80, in a deterministic order. No values, because a text field's contents are
the person's own writing. No bounds, which an interpreter could only use to point somewhere. No
native ids. No window title, which on a file manager is a path.

Four refusals, three of them structural:

1. **It cannot name a control that was not observed.** The id must be in the list that was sent
   AND still be in the world when the answer comes back.
2. **It cannot make anything operable.** Only already-addressable controls are offered, and
   what comes back is an id.
3. **It cannot authorise.** There is nowhere in a `Proposal` to say so.
4. **It must be able to say it does not know.** An abstention is a first-class answer.

### Two: shadow mode, which is the whole of this experiment

**Nothing in resolution or execution asks.** The only caller is the evaluation that measures
it, and `cmd/director TestTheInterpreterIsNotWiredIntoAnything` walks the tree to keep it that
way.

Not caution for its own sake. The question being settled is whether semantic interpretation
identifies the right control when a person describes something naturally AND recognises when it
does not know enough. Both halves need measuring before anything acts on either, and a
mechanism that is already influencing decisions cannot be measured against what it displaced.

Four numbers decide it, kept apart on purpose:

- **WOULD ADD** — cases Marco got wrong or refused, where the interpreter was right.
- **WOULD BREAK** — cases Marco got RIGHT, where the interpreter proposed something else.
- **ABSTAINED** — cases it declined. The capability, not the shortfall.
- **WRONG CONTROL** — proposals naming a control the person did not mean.

An interpreter whose ADD is large and whose BREAK is zero has earned a role in control
selection. One that buys paraphrases by occasionally overruling a correct literal answer has
not, however good its total accuracy looks.

## What the first run said

`llama3.2:3b`, local, through Ollama. Full numbers in
[[Experiment-070-the-interpreter-knows-some-meaning-and-does-not-know-what-it-does-not-know]].

| | Settings | held out |
|---|---|---|
| Marco | 27 / 33 | 31 / 39 |
| interpreter | 18 / 33 | 21 / 39 |
| WOULD ADD | 0 | 4 |
| WOULD BREAK | 2 | 4 |
| abstained | 6 / 36 | 2 / 40 |
| ambiguities answered | 2 / 2 | 4 / 5 |

**A wash, bought with four correct answers overruled.** It is not wired in, and on this
evidence it should not be.

The finding worth keeping is not the accuracy. It is that the model does not know what it does
not know, and that this did not transfer: after one bounded prompt change — offering "nothing
here is what the user asked for" as an ITEM IN THE LIST rather than as a rule in the
instructions — abstention on absent controls went from 0/5 to 5/5 **on Settings**, the
application the prompt was tuned against, and stayed at 2/6 on the held-out ones. The
mechanism's own held-out discipline caught the tuning, which is what it is for.

## What this bounds, and what it does not

It bounds THIS interpreter: a 3B model on one machine. It says nothing about what a larger one
would do, and the arrangement exists so that finding out costs one environment variable and
one re-run of a committed evaluation — no code, no new boundary, no new thing to configure.

## Consequences

- An interpretation can be measured against an oracle on applications nobody tuned against,
  before anything is allowed to act on one.
- `plugins/llama` answers the third question; `plugins/claude-resolver` does not and does not
  have to — an unrecognised field is answered as the question it does know, which the client
  reads as an abstention.
- The evaluation needs no model, plugin or network: with `$MARCO_INTERPRETER` unset it says so
  and reports Marco's own numbers, which is what keeps it usable as a committed gate.

## Enforced by

- `internal/resolver` `TestAnIdThatWasNeverOfferedIsAnAbstention`,
  `TestNoInterpreterMeansTheQuestionIsNeverAsked` (including that `$MARCO_RESOLVER` is never
  asked what is on screen), `TestAnythingThatGoesWrongIsAnAbstention`, `TestTheReasonIsBounded`.
- `internal/director/meaning` `TestOnlyOperableNamedControlsAreDescribed`,
  `TestNothingButIdRoleAndLabelIsSent`, `TestWhatIsDescribedIsBoundedAndDeterministic`,
  `TestAProposalMustBeAPropOfThisWorld`, `TestWithoutAnInterpreterNothingIsAsked`.
- `cmd/director` `TestTheInterpreterIsNotWiredIntoAnything`,
  `TestTheEvaluationNeedsNoInterpreter`.
