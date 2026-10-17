---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the owner's live desktop -- Windows Settings on its Apps page, read-only through production observation defaults, plus an isolated dry-run Director in its own MARCO_HOME. VS Code was the intended surface and could not be used: an installer held the vscode-updating mutex, so no second instance would start."
result: intent-parsing-and-observation-both-work-and-the-join-between-them-is-lexical-so-an-added-word-a-spelling-variant-or-a-synonym-makes-a-control-absent-that-is-plainly-on-screen
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-217-proposed-grounding-a-request-in-what-is-on-screen.md
---

# Experiment 068 -- Marco understands, and sees, and does not join the two

The first evaluation of an unfamiliar real application. Read-only throughout: nothing was
executed, no setting changed, no input watched or injected.

## Intent parsing

All three ordinary instructions decomposed, and the natural phrasing took the better route:

```
Goal: open the Explorer panel          Goal: click Explorer
[1/1] semantic "Explorer panel"        [1/1] click "Explorer"
      verification required                  marco  OS's Click
```

`open` is one of 34 semantic actions -- risk medium, target required, ladder *activate through
Invoke* then *double click* then *refuse rather than approximate*. "Each lowers to legal Marco;
none replays a coordinate."

## Observation

94 elements, 28 named clickable controls, accessibility alone. Identity stability across five
snapshots: 94/94 kept, 0 lost, 0 new, four times running. Admitted claims:
`selected_navigation "Apps"` and `window_title "Settings"`.

## Resolution, which is where it breaks

| the person says | the control says | verdict |
|---|---|---|
| `Bluetooth & devices` | Bluetooth & devices | RESOLVED 0.97, exact |
| `bluetooth` | Bluetooth & devices | RESOLVED 0.82, "allowing for a keyboard hint" |
| `installed apps` | Installed apps | RESOLVED 0.97, exact |
| `Personalization` | Personalization | RESOLVED 0.97, exact |
| `Apps` | Apps (nav) / Apps (breadcrumb) | **AMBIGUOUS**, two at 0.97 |
| `bluetooth settings` | Bluetooth & devices | **ABSENT** |
| `the bluetooth panel` | Bluetooth & devices | **ABSENT** |
| `personalisation` | Personalization | **ABSENT** |
| `themes` | Personalization | **ABSENT** |

A prefix works. An added word, a one-letter spelling variant and a synonym do not.

This also answers the VS Code question without VS Code. The planner extracted the target
`"Explorer panel"` verbatim; a control labelled `Explorer` would be ABSENT for exactly the reason
`the bluetooth panel` is.

## The ambiguity that should be kept

Two operable controls labelled exactly "Apps" -- a nav `list_item` and a page-header `button` --
both 0.97, doing different things. Around them it ranked `Apps for websites` 0.82,
`Installed apps` and `Default apps` 0.58, and rejected `text Apps` as "not an interactive
control". It is not confused; it is correctly stuck.

And on that Stage `selected_navigation` already said "Apps". The honest answer to "open Apps"
there is that it is already open -- a verification decision rather than a choice between two
controls.

## One reporting imprecision

`bluetooth settings` was refused as "is not an interactive control, so it cannot be acted on",
which reads as *found it, cannot use it*. It almost certainly matched the `Settings` title text.
The other three absences got the honest sentence.

## Two safety defects, found by reading before running

Both are [[ADR-216-a-dry-run-is-dry-at-the-boundary]], and both were found while checking what was
safe to run rather than by running it.

1. `director execute --dry-run` sent `DryRun: true` and the server never read it, so against an
   ordinarily started Director the flag performed real input.
2. `disambiguateWindow` called `winctx.ActivateTitle` outside the host abstraction, so a runtime
   built with `DryRunHost` could still raise windows. With an empty store no Place resolves, so it
   would have walked every `applicationframehost` window in turn during an evaluation whose whole
   premise was that it touched nothing.

## What was not reached

The request path through `Handle` against Settings, and therefore capability choice, policy
verdict, confirmation behaviour and Place identity. It needs the Settings window in the
foreground, which needs a human click, which did not come within the gate's three minutes.
Element and application identity were measured; **Place identity was not**.

## What follows

The limitation is neither intent understanding nor observation. It is the join, and it is lexical.
See [[ADR-217-proposed-grounding-a-request-in-what-is-on-screen]] -- a proposal, not an
implementation.

## Enforced by

Nothing here is a test; it is the record of a read-only evaluation. The safety fixes it produced
are held by the tests named in [[ADR-216-a-dry-run-is-dry-at-the-boundary]].
