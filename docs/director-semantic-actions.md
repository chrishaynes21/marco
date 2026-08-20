---
type: milestone
status: historical
---

# Semantic UI actions

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> Marco owns mechanics. Director owns semantics.
> Every semantic action lowers to one legal Marco program.
> No action is implemented by replaying historical coordinates.

## The gap this closes

The Director could already perceive durable targets, plan sequences, and verify what it
did. Its **action words** were the constraint: click, type, key, focus, move a window,
edit text. Everything else decomposed.

"Expand the folder" became *click at (921, 381)*. That is wrong three separate ways:

- it is not what the user said;
- it cannot be verified as an expansion — a click that missed and a click that worked
  look identical to a screen-difference check;
- it replays as a coordinate, which means nothing tomorrow.

So the vocabulary grew. The planner now emits **Expand** and stops. What that becomes is
decided at execution time, against the control's real capabilities.

## The model

`directorapi.SemanticActionKind` is 33 verbs — `expand`, `collapse`, `toggle`, `check`,
`select`, `choose`, `invoke`, `open`, `close`, `dismiss`, `submit`, `confirm`, `cancel`,
`refresh`, `back`, `forward`, `next`, `previous`, `undo`, `redo`, `copy`, `cut`, `paste`,
`select_all`, `scroll_here`, `show_context_menu`, `maximize`, `minimize`, `restore`,
`pin`, `unpin`, `deselect`, `invoke`.

They carry no keyboard shortcut and no pattern name. A kind that encoded `ctrl+z` would
have thrown the choice away before anyone could make it — the mistake the editing
milestone documented for text, where a plan committed to `Ctrl+A then type` could never
use a value API.

**Why a separate enum from `ActionType`.** `ActionType` classifies a plan step's SHAPE
and each value has a struct behind it. Thirty-three more would break that correspondence
and grow every switch in the system. One `SemanticAction` carrying a kind keeps the plan
shape stable and puts the vocabulary where it can grow.

## The capability ladder

Each verb declares its acceptable implementations in preference order
(`internal/director/uiact`). Expand:

```
1. ask the control to expand through its ExpandCollapse pattern
2. activate the control, which expands it in most tree implementations
3. click the disclosure arrow
4. refuse rather than approximate
```

The order is one argument repeated: **prefer the implementation that cannot miss and can
report failure, over the one that can do neither.** A pattern call is the application
performing the operation. A click is a guess about geometry that succeeds identically
whether it hit the control or the thing behind it.

Every ladder ends in a refusal, and that is the design. A ladder whose last rung was
"click somewhere sensible" would make every unsupported verb succeed at doing something
nobody asked for — worse than refusing, because a refusal can be reported and a wrong
click cannot be taken back.

Some ladders are deliberately short. `deselect` has one rung: a plain click SELECTS, and
ctrl+click deselecting is a convention rather than a guarantee. `scroll_here` has one:
wheel notches would scroll an unknown container by an amount nobody chose.

Every rejected rung is recorded with the reason it was unavailable. *"The Director
clicked"* and *"the Director asked for an expand, the control had no such pattern, and it
clicked instead"* are different events, and only the second explains a click that landed
somewhere surprising.

### Silence is not denial

The rung's availability is evidence-driven, and the evidence has three values, not two:
supported, not supported, and **not reported**. Most providers say nothing about
patterns. If silence read as "supports none", every control everywhere would fall through
to clicking — the ladder would be decoration.

So an unreported pattern set offers the strong rung and lets the host refuse explicitly.
A refusal is recoverable and recorded; a premature fall to clicking is a geometric guess
nobody logs as a fallback.

### Already-satisfied is an outcome

`Check` against a checked box does **nothing**, and that is success. Implementing it as a
toggle would uncheck it — the opposite of the request. `Toggle` deliberately has no
satisfied state: it asks for the other one, whichever that is.

A state that could not be READ is never treated as satisfied. The dangerous direction is
the other one: "check that box" silently doing nothing whenever the provider is quiet.

## Lowering

Every chosen mechanism becomes typed `marcoexec.Operation`s, which lower to Marco source,
compile, and run — the same single path every other Director effect takes.

Seven new Marco capabilities were declared in `internal/uiamod/uia.marco` and implemented
in the C# bridge: `Invoke`, `Expand`, `Collapse`, `Toggle`, `Select`, `Deselect`,
`ScrollIntoView`. Each **fails** rather than silently succeeding when the control does
not implement the underlying pattern, using the same `unsupported:` prefix `SetValue`
established — the one place a refusal is distinguished from a fault.

`TestEveryOperationLowersToLegalMarco` compiles all seven through the real compiler.

Most verbs lower to one operation. The context menu lowers to two — focus, then
Shift+F10 — because the chord acts on whatever holds focus, and sending it without
focusing first would open the context menu of something else.

Keyboard verbs are addressed to the target's **window**, so the foreground guard can
refuse a chord that would land in whatever the user alt-tabbed to.

## Verification

Each verb is verified by its own semantics, not by "something moved":

| verb | evidence |
|---|---|
| expand / collapse | the node reports itself expanded/collapsed, **or** its children appear/disappear |
| check / uncheck | the control's checked state |
| toggle | the state is the *other* one |
| select / deselect | the item's selection state |
| dismiss / cancel / close | a window or dialog is gone, or the target is |
| back / forward / refresh | the view became a *different* view — title change, or content replacement |
| undo / redo | the focused control's contents changed |
| maximize / minimize / restore | the window's state |
| scroll_here | the target is in view |

> Verification must never rely solely on visual change.

A region changing after an action is consistent with the action having worked and equally
consistent with an unrelated repaint. It corroborates; it never concludes.

**The control's own denial outranks everything.** A tree that still reports itself
collapsed was not expanded, however much else changed on screen.

**Unreadable is inconclusive, not failed.** A box whose state cannot be read may well be
ticked, and claiming it is not would be as wrong as claiming it is.

The open-ended verbs — invoke, open, submit, paste, next — reuse the click verification's
evidence gathering. The question is identical ("did activating this do anything?"), and a
second set of rules for the same question would drift from it.

## Policy

> A click on "Delete" is not a low-risk click.

Risk is classified from the VERB, before the target is known. `submit` and `confirm` are
high whatever they are aimed at, because committing is what they mean; the verb the user
said is better evidence of consequence than the input that carries it.

The target can only raise it: a destructive-looking label makes even a reversible verb
worth confirming, since "toggle" on a control labelled "Delete account" is not a toggle
anybody should do unasked.

## The action graph

A node stores the **verb and the query**:

```
Action:  expand
Target:  {"label":"Explorer","role":"tree_item"}
```

and not the mechanism, and not a coordinate. `TestNoMechanismAndNoCoordinateIsStored`
checks the whole serialised node rather than named fields, so a new field carrying the
mechanism would not slip past.

Storing the chosen mechanism would be the same mistake one level up: a decision made
against a control that may have changed by the time it is replayed. A replay re-resolves
the target and **chooses the lowering again** — so a node recorded when an application
exposed no ExpandCollapse will use it once the application does.

## Diagnostics

```
director actions               the vocabulary: verb, risk, whether a target is needed
director actions expand        one verb's full ladder
director explain action        what the last action chose, and what it rejected
director trace                 semantic action → capability → Marco → verification
```

`director actions` needs no service and touches no desktop: a ladder is a property of the
verb. It exists because the ladder is otherwise folklore — "why did it click instead of
expanding?" is answerable after the fact, but "what *would* it do?" has to be answerable
before.

`director explain action` reports the chosen mechanism, every stronger one that was
unavailable with the reason, the operations it lowered to, and — stated explicitly — when
the chosen mechanism is geometric and therefore able to miss.

## What was measured

The bridge's pattern reporting was validated live against Notepad. Of 45 elements, 36
report patterns, and they are the right ones:

```
menu_item  File          invoke,expandcollapse
text_field Text editor   value
tab        Untitled      selectionitem,scrollitem
button     Add New Tab   invoke,scrollitem
list                     scrollitem,scroll
```

So "expand the File menu" reaches ExpandCollapse and "press File" reaches Invoke, from
the application's own declaration rather than from a guess about the role.

**This found a real bug.** The first live run reported *zero* patterns across seventeen
buttons. The snapshot walk fetches elements in `AutomationElementMode.None` — cached data
— so a property that was not in the `CacheRequest` reads as NotSupported rather than as
its value. Left out, the ladder would have fallen every verb through to a click while
looking like it was making an informed choice. The availability properties are now
cached explicitly.

## What changed for existing behaviour

`press`, `activate`, `select`, `choose` and `open` now parse as semantic actions rather
than as clicks. That is the milestone: pressing a control and clicking at its coordinates
differ in what they can be verified by and in what a replay re-derives.

A literal **"click X" stays a click** — that request names the gesture, and overriding it
would ignore an explicit instruction.

The editing vocabulary is untouched. `undo`, `redo`, `select all`, `copy`, `paste` and
`submit` still reach `internal/director/edit`, which implements them against a control's
value API rather than as chords. The semantic parser declines them: the two vocabularies
overlap by design and the stronger implementation runs first.

## Known gaps

- **Not validated live end-to-end.** The pattern *evidence* was measured against Notepad
  (above), and every layer is covered by tests, but no semantic action has been executed
  against a real application through the full service — the milestone's "Live Validation"
  against VS Code, Chrome and Explorer has not been run. Doing so performs real input,
  which is not something to fire blind.
- **`pin` / `unpin` are honestly thin.** There is no pin pattern: pinning is a toggle on
  some controls, a context-menu entry on most, and nothing at all on the rest. The ladder
  offers what exists and refuses otherwise, which means it will often refuse.
- **`redo` guesses a chord.** `Ctrl+Shift+Z` and `Ctrl+Y` are both conventions and there
  is no way to tell from outside which an application uses. The verification catches the
  case where nothing happened, which is why a guess is tolerable here and nowhere else.
- **`next` / `previous` assume tab semantics** (`Ctrl+Tab`). In a list or a media player
  the user may well mean something else.
- **The ladder cannot see a disclosure arrow.** Its click rung aims at the control's own
  rectangle, which for a tree item is the row — not the triangle. Where ExpandCollapse is
  absent and Invoke does not expand, that rung will click the row and select it instead.
  Nothing detects that; verification reports the expand as unproved, which is correct but
  not helpful.
- **No semantic action participates in bulk collections yet.** `collections.bulkAllowlist`
  still names `focus`, `activate` and `click`; the semantic verbs are not classified for
  bulk, so "expand every folder" is not expressible.
