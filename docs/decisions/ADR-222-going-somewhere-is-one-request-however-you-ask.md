---
type: adr
status: accepted
date: 2026-10-17
decision: the ordinary English ways of asking to go somewhere — show, show me, go to, jump to, navigate to, switch to, take me to, bring up — are surface forms of the existing `open` action and belong in the phrase table beside "open up" for expand and "tick" for check; no new action, no application knowledge
supersedes: []
affects:
  - execution
source_paths:
  - internal/director/uiact/parse.go
---

# ADR-222 — going somewhere is one request however you ask

## What was measured

A live Windows Settings window, Personalization page, production observation, through the
ordinary request path:

```
open installed apps              →  list_item "Apps"            resolved
show installed apps              →  I only understand click, focus, move, repeat
show me installed apps           →  I only understand click, focus, move, repeat
go to installed apps             →  I only understand click, focus, move, repeat
take me to installed apps        →  I only understand click, focus, move, repeat
navigate to installed apps       →  I only understand click, focus, move, repeat
I want to see my installed apps  →  I only understand click, focus, move, repeat
installed apps                   →  I only understand click, focus, move, repeat
```

One English word opened a destination and its ordinary synonyms did not. The control was on
screen, perception found it, and the resolver — after
[[ADR-218-a-request-reworded-in-the-screens-own-words]] and
[[ADR-219-a-match-is-not-an-answer]] — resolves it correctly from any of these phrasings. None
of them ever reached it.

So on a real desktop the limiting factor was neither perception nor resolution. It was the
first three words of the sentence.

## The decision

Those phrasings join `verbPhrases` as surface forms of `open`.

That table already holds exactly this kind of entry — "open up" and "unfold" for expand, "tick"
for check, "pick" for choose, "press"/"push"/"tap" for invoke. It is the English surface of a
vocabulary the table owns, and nothing in it is about any application. No action is added: the
34 stay 34, and `open`'s ladder — activate through Invoke, then double click, then refuse
rather than approximate — is what carries every one of these.

## Why this is not the start of a synonym habit

Because the test of a general rule is whether it is about language or about an application, and
these are about language. "Go to X" is how English says *open the place called X*; it would be
the same sentence about a browser tab, a spreadsheet sheet, a game menu or a settings page.

The line to keep: a phrasing earns a place here when it is a way of saying an action Marco
ALREADY HAS. A phrasing that names something Marco cannot do does not get an entry, it gets a
refusal — which is why `make a new folder` is not here. That is a procedure, two steps, and it
already has one: the `create_folder` goal with its `explorer create folder` procedure.

## The collisions, and why none of them breaks

- **"go to the top", "jump to the bottom".** Already whole phrases in `scrollPhrases`, which is
  matched BEFORE the verb loop. They still move the view rather than looking for a control
  called "the top".
- **"show context menu".** The longest-prefix rule protects the form with a target, and the
  bare form did not previously parse at all — so "show" would have newly captured it and read
  it as *open a control called "context menu"*. Caught by the regression test before this was
  committed, and fixed by giving the bare form its own whole-phrase entry alongside "show the
  context menu", which was already there.
- **"open up"** still expands, because it is longer than "open".
- **"select"**, **"close up"** and the rest are untouched: nothing was removed or redirected.

## What it changed on the live desktop

Every phrasing above now parses and resolves to the right control. End to end, with real input:

```
take me to bluetooth settings
  resolve   list_item "Bluetooth & devices" (0.66)
  execute   open via double_click · after 1 stronger option(s) were unavailable
  verify    "Bluetooth & devices" became selected
  COMPLETED (1 action(s) performed)
```

See [[Experiment-071-marco-goes-where-you-ask-and-says-when-it-need-not]].

## Enforced by

`internal/director/uiact` `TestADestinationIsReachedHoweverThePersonAsks` (ten phrasings, one
action, the target preserved) and `TestTheDestinationPhrasingsDoNotSwallowWhatTheyResemble`
(the scroll destinations, the context menu, "open up", "close up", "select" — each still read
as what it meant).
