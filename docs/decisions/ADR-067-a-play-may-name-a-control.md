---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - learned-plays
  - demonstrations
source_paths:
  - internal/uiamod/accessibility.marco
  - internal/director/marcoexec/play.go
  - internal/director/marcoexec/operation.go
  - internal/director/observe/lowering.go
  - plugins/uia/Uia.cs
---

# ADR-067 — a play may name a control

Marco could learn a click and could not write one down. This adds the one durable way to say which
control a play presses.

## The evidence, and the diagnosis it corrected twice

A live UI-driven Learn against Windows Settings: two clicks, **both resolved to named controls**
(`pointer_resolved=2`, 36 controls on offer), a durable route, a goal, and a rehearsal that
invoked the control through the accessibility boundary and came back `directly_verified`. Then:

```
lowering: eligible=no  [screen_unnamed, unresolved_pointer_target]
step 1: point → expected subj_61ff…, observed subj_61ff…  [directly_verified]
```

`unresolved_pointer_target` reads as *"a click with no semantic control behind it"*, which was
false of this demonstration. It was diagnosed twice as lost evidence — once as "the label doesn't
survive capture", once as "propagation drops it before lowering" — and both were wrong. The label
survives intact: `DemonstrationStep.Targets` is its durable home and it holds `"Mouse"`.

**Lowering never read it.** It refused `NavPoint` unconditionally, and the code said why: a saved
play needs a capability that resolves a control's NAME at run time, and there wasn't one.

So this was a language gap wearing a perception gap's name. `internal/director/observe`
`targetsurvival_test.go` now pins both halves so it cannot be re-diagnosed a third time.

## Why the existing capability was not enough

`Accessibility's Invoke` has existed since the structural actions landed. Its `Control` set names
the control by `Element` — *"the accessibility source's own id"*, a UIA RuntimeId. That is exact,
it is what a value API accepts, and it is **ephemeral**: it identifies a control in the tree as it
stands right now and means nothing after the application redraws, let alone tomorrow. A saved play
holding one would work until the first repaint and then fail obscurely.

## The decision

**`Control.Name` — the control's own label, resolved against the live tree when the play runs.**

- `uia.marco` gains one optional field beside `Element`. **No Core syntax changes.** This is the
  act surface growing by one field, the same way `WindowMove` carries a `Window`, and it is
  justified by `spec/Core.md` governance's own test: a legal effect Director needs and Marco cannot
  otherwise express, demonstrated by a real failure rather than argued for.
- Exactly one of `Element` and `Called`. `Operation.Validate` refuses neither and refuses both —
  two answers to "which control" would leave the host silently preferring one.
- `observe.LoweredAction` replaces a bare `NavIntent` as the unit of a lowered step, because a
  demonstration contains two kinds of thing a play can say: a **meaning** (the person confirmed;
  which key that is belongs to the host) and a **named control**.
- A generated play declares one local per control — `control1`, `control2` — because an act takes
  one input and a set is built before it is passed. Reusing a name would be a different program
  that happened to compile.
- `use accessibility.` is emitted only when a play actually presses something, so a
  navigation-only play does not require an accessibility host to run.

### The host resolves, and refuses rather than guesses

`Uia.Resolve` walks the window and matches on the control's Name — trimmed, case-insensitive —
over controls that are **enabled and actually invokable** (Invoke or SelectionItem). Three
outcomes:

| matches | result |
|---|---|
| 0 | `target_not_found:` — the screen is not what the play expected |
| 1 | invoke it |
| >1 | `target_ambiguous:` — and it stops |

Pressing the first of several controls sharing a name is a coin toss performed on somebody's
computer, and it would be indistinguishable from working. Invokability is part of the MATCH rather
than a check afterwards: a label shared by a button and a static caption is not ambiguous, because
only one of them is a thing you can press.

## The refusal was renamed

`unresolved_pointer_target` → **`no_target_to_name`**. The old name cost a live diagnosis by
describing perception when the problem was expression. The refusal keeps its place in the
vocabulary because clicks with genuinely no name behind them are real — nothing under the pointer,
or a label the admission rule withheld — and it now means what it says.

## The import was renamed too

`use uia.` became **`use accessibility.`**. The act inside has always been called `Accessibility`;
only the import disagreed, and it disagreed by naming a Microsoft API. A Marco program describes
WHAT it means — UIA is how Windows happens to provide it, which is the host's business and stays
in the host's name. Nothing depended on the old spelling: no route, no example and no spec page
used it, so it is a rename rather than a deprecation, and `use uia.` now fails with "no uia.marco
beside the program and no built-in".

The generated header comment stopped citing Go source paths at the same time. It named the files
the act surfaces are embedded from, which is a fact about this repository rather than about the
program a person is reading — and it put the word "accessibility" into every lowered program,
including ones that reach nothing of the kind.

## What did not change

`Element` is untouched and remains how a direct operation names a control it already has in hand.
The rehearsal path is unchanged. No coordinate is ever written into a play.

## Enforced by

- `internal/director/observe/targetsurvival_test.go` —
  `TestANamedControlSurvivesIntoTheDemonstration` (the label arrives),
  `TestALoweredClickCarriesTheControlsName` (it is used; mutation-gated by dropping `Called` from
  the lowered action), `TestAClickWithNoNameIsStillRefused` (the honest refusal survives).
- `internal/spectest/invokeplay_test.go` — the generated play is compiled by the REAL compiler
  against the REAL act surfaces: `TestAPlayThatPressesANamedControlCompiles`,
  `TestAPlayPressingTwoControlsDeclaresTwoLocals` (mutation-gated by reusing one local),
  `TestANavigationOnlyPlayDoesNotImportAccessibility`.
- The host's refusals were verified against the built `uia.exe` directly: a name nothing matches
  answers `target_not_found`, both identifiers answers a refusal, neither answers a refusal.
  **Not yet covered by an automated test** — the success path needs a live window, and belongs
  with the live harness.

## Related

[[ADR-005-legal-marco-only]] · [[ADR-027-what-marco-learned-becomes-marco]] ·
[[ADR-058-a-demonstrated-target-may-keep-its-name]] ·
[[ADR-030-a-play-says-where-it-begins]] · [[Learned-Plays]] · `spec/Hosts.md`

## The field is `Name`, not `Called`

It was `Called` until 2026-08-18. A set field is a NOUN — `a Control with Name "Mouse"` is the
sentence a person reads, and `with Called "Mouse"` is a participle wearing a noun's job. It also
made the Accessibility act disagree with the Theater's `Target.Name`, which had always meant
exactly this. One concept, one word.

The rename runs the whole width of the surface, because the field name IS the wire format: the
act declaration in `accessibility.marco`, the Marco the Actor casts, `marcoexec`'s lowering, and
the key the UIA bridge reads in `Program.cs`.
