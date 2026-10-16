---
type: experiment
status: blocked
date: 2026-10-16
backend:
  - accessibility
  - structure
  - ocr
fixture: "the real Director runtime on the owner's Windows 11 machine, through `director demonstrate-surface -check` against a throwaway MARCO_HOME"
result: five-of-seven-preconditions-hold-on-the-real-machine-including-the-installed-input-hook-and-both-adr-207-seams-and-the-experiment-is-blocked-by-a-structural-one-no-production-observation-request-includes-sourcestructure-so-on-any-surface-an-accessibility-tree-describes-identity-and-interaction-surface-are-one-object-and-no-surface-question-can-arise-which-is-exactly-the-case-on-windows-settings
supersedes: []
affects:
  - execution
  - perception
---

# Experiment 060 — the preconditions for one live demonstration

**Status: BLOCKED, 2026-10-16.** Harness: `director demonstrate-surface -check`, which reads,
installs the production input hook, collects at most one Stage, and injects nothing — the runtime
it builds is a dry run, so the only host that could reach the desktop is `runtime.DryRunHost{}`.

## The question

> Can the bounded live focused-demonstration test run on Windows Settings?

## What holds, measured on the real machine

```
  yes  disposable learning record       MARCO_HOME is a throwaway directory; resetGuard — the
                                        same predicate `reset-test-state` uses — accepts it
  yes  accessibility bridge             plugins\uia\uia.exe
  yes  input observation hook installed navsource reports the hook installed
  yes  durable memory                   <sandbox>\semantic-memory.json
  yes  surface grounder wired           Pipeline.Surfaces non-nil; Pipeline.Place non-nil
  --?  input observation hook active    not measured; needs a hand to click
  --?  the instruction against the Stage not collected; needs the application in front
```

Five of seven, including the two [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]]
wired and the install half of the owner's first prerequisite. The two unmeasured ones need the
owner's hands and screen and are what `-watch` and `-instruction` exist for.

## And the one that blocks it, which no amount of hands fixes

**No production observation request includes `SourceStructure`.**

`internal/director/perception/providers/structure` is registered in the daemon's provider list and
gated on `req.Includes(directorapi.SourceStructure)` —
[[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] made it opt-in because it
costs a capture and a pass over the pixels. Every production request is
`observation.Request{}`, `observation.WithVision(nil)` or `observation.WithPixels(nil)`, and none
of those includes it.

So on a live Stage the shape proposer never runs, and there are no `RoleUnknown` rectangles for a
surface question to be about.

**And where an accessibility tree describes the control, there is nothing to ask anyway.** That is
not a gap, it is the premise: identity and interaction surface are one object when a source says
"here is a button and here is how to work it". Windows Settings is XAML with a full UIA tree, so
`click Bluetooth & devices` resolves to a real control, `Any()` is true, the licence is not even
reached, and the request executes. A surface question is unreachable there by design.

The two conditions compound: the experiment needs a surface where identity and surface come apart,
and the one application the authorisation permits is one where they do not.

## The three ways out, and which one is honest

1. **Include `SourceStructure` in production cycles.** Does not help: on Settings the
   accessibility-described control still outscores a bare rectangle, so the request still resolves
   and still never asks. It would also be a perception change ADR-203 deliberately did not make.
2. **Use a surface with no accessibility tree.** That is a game, a canvas, or a custom-drawn
   player — all excluded by this authorisation, and the game is the one the owner moved away from
   on purpose.
3. **Run Windows Settings in the pixel-only configuration**: the experiment requests the shape
   proposer and does not register the accessibility provider. That reproduces the exact condition
   the feature exists for, on a safe, non-game, inspectable application the owner chose, and it
   weakens no guard — the resolver, ADR-202's licence, policy, the ladder and the Marco runtime are
   all unchanged. What changes is the EVIDENCE AVAILABLE, not the rules applied to it.

Option 3 is the only one that is both bounded and faithful, and it is a configuration decision
rather than a code relaxation — which is why it is reported here rather than taken.

## What this command may not become

It has `-check` and nothing else, on purpose. The learning and execution phases are not written,
because the diagnosis above decides whether they can exist at all and in what configuration.
Writing them first would have been building on the assumption this experiment exists to test.

## Related

- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the wiring whose
  seams this verifies.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — why the shape proposer is
  opt-in.
- [[Experiment-059-one-demonstration-from-a-person-into-durable-memory]] — the loop, proven on
  recorded events.
- [[Experiment-051-the-stage-two-real-applications-produced]] — the measurement that established
  identity and surface coming apart only where nothing but pixels describes the screen.
