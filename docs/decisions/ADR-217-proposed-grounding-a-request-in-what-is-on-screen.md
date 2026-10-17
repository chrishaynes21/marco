---
type: adr
status: proposed
date: 2026-10-17
decision: PROPOSAL ONLY, not implemented — a semantic grounding stage that proposes candidate Props for a request the deterministic resolver could not match, ranked from evidence already on the Stage, and which cannot invent a control, its operability or its authorisation
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-217-proposed-grounding-a-request-in-what-is-on-screen.md
---

# ADR-217 (proposed) — grounding a request in what is on screen

**Nothing here is implemented.** This is the design asked for after the Alpha 1 evaluation, written
to be argued with before any of it is built.

## The measurement this answers

Windows Settings, production observation, read-only. Three ordinary instructions, then the same
instruction said a few ordinary ways.

| the person says | the control says | Director |
|---|---|---|
| `Bluetooth & devices` | Bluetooth & devices | RESOLVED 0.97, exact |
| `bluetooth` | Bluetooth & devices | RESOLVED 0.82, "allowing for a keyboard hint" |
| `installed apps` | Installed apps | RESOLVED 0.97, exact |
| `bluetooth settings` | Bluetooth & devices | **ABSENT** |
| `the bluetooth panel` | Bluetooth & devices | **ABSENT** |
| `personalisation` | Personalization | **ABSENT** |
| `themes` | Personalization | **ABSENT** |

Intent parsing works: all three instructions decomposed, and `open the Explorer panel` became a
semantic `open` with a target rather than a blind click. Observation works: 94 elements, 28 named
clickable controls, 100% identity stability across five snapshots, and claims as good as
`selected_navigation "Apps"`.

**The join is lexical.** The planner carries the person's words through as the target, and the
resolver matches those words against labels, tolerating a prefix and a keyboard hint. It does not
bridge an added word, a one-letter spelling variant, or a synonym. That is the whole gap, and it is
the one that makes a system which understands and can see feel like it does neither.

## What is proposed

One new stage, reached **only when the deterministic resolver returns ABSENT**, which proposes
candidates and nothing else.

```
intent ──► plan ──► resolve (deterministic)
                        │
                        ├─ RESOLVED   ──────────────────────────────► policy ──► act
                        ├─ AMBIGUOUS  ──────────────────────────────► ask
                        └─ ABSENT ──► GROUND (new) ──► candidates ──► resolve's own
                                                                      actionability rules
                                                                      ──► RESOLVED │ AMBIGUOUS │ ABSENT
```

Four properties define it, and each is a refusal:

1. **Deterministic matching stays first.** Grounding is never consulted for a request the resolver
   already answered. An exact label match must never be overridden by an interpretation, and the
   0.97 cases above must come out bit-identical.
2. **It proposes Props that are already on the Stage, by id.** Its output is a ranked list of
   `directorapi.ElementID` with a reason per entry. It cannot name a control that was not observed,
   and it has no way to express one — the type is an id into the current `WorldState`.
3. **It cannot make anything operable.** Every candidate it proposes goes back through the
   resolver's existing actionability rules. `text Apps` was rejected as "not an interactive
   control" during the evaluation; if grounding proposes it, it is rejected again, by the same code,
   for the same reason. Grounding re-orders the queue; it does not open the door.
4. **It cannot authorise.** Risk, reversibility, confirmation and the capability ladder are
   untouched. A grounded candidate arrives at `policy` exactly as a lexically resolved one does.

### What the evidence is

Everything it reasons over is already collected. No new provider, no new observation pass.

| evidence | already on the Stage | what it is for |
|---|---|---|
| label | `Element.Label` | the words on the control |
| role | `Element.Role` | `list_item` in a nav list is a destination; `text` is not a control |
| structure | parent links, siblings | "one of the twelve items in the nav list" is a stronger reading of "panel" than a button in the content area |
| geometry | `Element.Bounds` | a 280×36 item at x=16 in a 300-wide pane is a sidebar entry |
| application | `WorldState.Windows` | the same word means different things in a browser and a settings app |
| state | admitted claims | `selected_navigation "Apps"` — see the *Apps* case below |
| durable memory | `semanticmemory` targets and surfaces | a target demonstrated here before outranks any interpretation |

### Three mechanisms, in order of cost

The proposal is that they are added **in this order**, each gated on the previous one being
insufficient, because the first two are deterministic and the third is not.

**(a) Normalisation of the request's own words.** Free, deterministic, testable, and it closes
`the bluetooth panel`, `bluetooth settings` and `personalisation` on its own:

- strip the request's function words and UI nouns — "the", "panel", "pane", "tab", "section",
  "settings", "menu" — and retry the resolver with what is left;
- fold Unicode and case, and apply a bounded edit distance (1 for words over six letters) so
  `personalisation` reaches `Personalization`;
- treat `&` and `and` as the same word.

This is **not** an application-specific synonym list. It is a list of words the UI vocabulary uses
to name *kinds of control*, and it applies to every application. It should be written as exactly
that, and the test should be that removing "panel" from the list reopens the VS Code case.

Crucially: normalisation retries **the existing resolver**, so the result keeps its confidence, its
rejections and its ambiguity. It is a second question, not a new answer.

**(b) Structural reading of what the person asked for.** Still deterministic. "Open the X panel" in
an application with a navigation list is a request to activate the nav item named X — because the
role, the container and the geometry say so, not because the application is Settings. This is the
mechanism that distinguishes the nav `list_item Apps` from the breadcrumb `button Apps`, and it is
the one that could, in time, answer the *Apps* ambiguity with evidence rather than a question.

**(c) Interpretation, last.** For what survives (a) and (b) — genuinely, `themes` → Personalization
is a knowledge question, not a string question. The proposal is that this is a **resolver plugin**
behind `$MARCO_RESOLVER`, where one already lives, handed a prompt containing the observed
candidates and asked to rank them. It returns ids and reasons. It is advisory, offline-capable, and
its absence degrades the system to (a) and (b) rather than breaking it.

It must be structurally unable to do the two things a language model will otherwise do: name a
control that is not there (its output type is an id into the Stage) and assert that something is
clickable (the actionability rules run after it, not before).

## How identical labels stay ambiguous

The *Apps* case is the one to preserve, not fix. Two operable controls labelled exactly "Apps",
both 0.97: a nav `list_item` and a breadcrumb `button`. They do different things.

The rule proposed is: **grounding may break a tie only with evidence that distinguishes the
candidates, never with a preference.** A ranking that put the nav item first "because nav items are
usually meant" is a preference and must not count. A reading that puts it first because the request
said "panel" and only one candidate is in a navigation container is evidence, and the account has
to be able to say which it used.

When nothing distinguishes them, the question is asked — exactly as it is today, with the same
`Hypothesis.Question()` refusal when no press could tell the options apart.

And the owner's observation deserves its own mechanism, separate from grounding: `Apps` was
**already selected**. `selected_navigation "Apps"` was an admitted claim on that very Stage. The
right answer to "open Apps" there is *"that is already open"* — a **verification** decision, before
the question and before any click. That belongs with the existing pre-verification, not here, and it
would remove the ambiguity by making the request satisfied rather than by guessing which control to
press.

## How uncertainty is communicated

The resolver's existing vocabulary already has the right shape — RESOLVED / AMBIGUOUS / ABSENT with
a confidence and a per-candidate rejection reason. The proposal adds no fourth verdict. It adds one
field: **how** the candidate was reached.

```
Resolving "open the bluetooth panel" → RESOLVED (grounded)
  "Bluetooth & devices": the request named a panel and this is the only navigation
    item whose label matches once "the" and "panel" are set aside
  → list_item "Bluetooth & devices" at (16,223 280x36) (confidence 0.74, grounded)
  considered 94: … 
```

Three things that must hold of that line:

- a grounded resolution is **visibly** grounded, never presented as an exact match;
- its confidence is **lower** than any lexical match, so a later exact match always wins;
- the reason names the evidence used, so a person reading a wrong answer can see which step was
  wrong — normalisation, structure, or interpretation.

A grounded resolution should also be the kind of thing that can require confirmation where a
lexical one would not. That is a policy input, not a resolver decision, and the risk ladder already
carries the concept.

## How we would evaluate it

The failure mode to design the evaluation against is **the one this codebase has already had eight
times**: a mechanism that works on the application it was written against. So:

1. **A held-out application.** Build against Settings, evaluate on something never used during
   development — Notepad's menus, Explorer's ribbon, a browser's own chrome. The numbers that count
   come from the application nobody tuned against.
2. **The oracle beside the verdict**, as in the existing acceptance work: for each instruction, the
   control a person says was meant, recorded before the run. Then four counts — correct, wrong
   control, asked when it needn't have, refused when it could have acted. **"Wrong control" is the
   only one that is a regression**, and it must be zero. Asking too often is the thing we are
   trying to reduce; acting on the wrong thing is the thing we must never buy with it.
3. **The deterministic cases must not move.** Every RESOLVED and every AMBIGUOUS from this
   evaluation re-run bit-identical. A grounding change that alters an exact match has broken its
   first property.
4. **Refusal under absence.** Instructions naming controls that genuinely are not on screen must
   still come back ABSENT. A grounding layer that always finds something is worse than none, and
   this is the test that catches it.
5. **Measured without the plugin.** (a) and (b) alone, so we know what the deterministic half
   bought and whether (c) is worth its cost at all.

## What is deliberately not proposed

- No application-specific synonyms, and no Settings or VS Code special cases.
- No weakening of actionability, operability or the Place rules.
- No expansion of the 34-action vocabulary. The evaluation says the actions are not the limitation.
- No new observation pass, provider or Stage collection.
- No new Marco syntax. Grounding is Director's business and names no Marco noun, per the language
  governance rule.

## Open questions I would want answered before building

1. **Does (a) alone close the gap?** It closes four of the five measured failures on paper. If it
   does, (c) may not be worth building this year, and that would be the cheapest good outcome.
2. **Where does normalisation live?** Inside the resolver as a second pass, or before it as a
   request rewrite? Inside keeps one place that matches; before keeps the resolver pure and makes
   the retry visible in the account. I lean *before*, for the account.
3. **What does a grounded resolution do to learning?** A demonstrated surface is stored against a
   target `{label, kind, place}`. If the request was grounded rather than matched, which label is
   stored — the person's words or the control's? The control's, I think, or memory fills with the
   phrasings people happened to use. Worth deciding before anything is stored.
4. **Is `selected_navigation` the first of many?** If pre-verification can answer "already open",
   it needs claims like it from more applications, and that is a perception question rather than a
   grounding one.

## Enforced by

Nothing. This is a proposal; the tests belong with the implementation, and §"How we would evaluate
it" is the specification for them.
