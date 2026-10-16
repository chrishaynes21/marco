---
type: experiment
status: ready
date: 2026-10-16
backend:
  - structure
  - ocr
fixture: "Windows Settings on the owner's machine, observed pixel-only — the shape proposer and OCR, with accessibility neither requested nor registered — through the real Director runtime, the real input hook and the real crossing correlator"
result: the-harness-is-built-and-six-of-eight-preconditions-hold-on-the-real-machine-including-the-installed-hook-the-registered-provider-list-with-accessibility-absent-and-both-adr-207-seams-and-the-two-that-remain-need-a-hand-and-a-screen-so-the-live-run-is-the-owners-to-make
supersedes: []
affects:
  - execution
  - perception
---

# Experiment 061 — the live focused demonstration

**Status: READY, AWAITING THE OWNER'S HANDS, 2026-10-16.** Harness:
`director demonstrate-surface`, four phases, configuration per
[[ADR-208-one-guarded-test-configuration-and-no-second-one]].

## The question

> Can one genuine demonstration from a person, on a real screen where nothing but pixels
> describes the control, reach durable memory and be reused on a later instruction — through the
> unchanged resolver, licence, policy, ladder and Marco runtime?

## What holds, measured on the real machine

`director demonstrate-surface -check`, against a throwaway `MARCO_HOME`:

```
  yes  disposable learning record       resetGuard accepts a throwaway MARCO_HOME
  yes  pixel-only test configuration    sources [window_system structure ocr]
  yes  accessibility bridge             plugins\uia\uia.exe
  yes  registered observation providers [window_system ocr vision structure]
  yes  input observation hook installed navsource reports the hook installed
  yes  durable memory                   <sandbox>\semantic-memory.json
  yes  surface grounder wired           Pipeline.Surfaces and Pipeline.Place non-nil
  --?  the instruction against the Stage needs the application in front
  --?  input observation hook active    needs a hand to click
```

The fourth line is the one worth reading twice: **accessibility is absent from the list the
runtime actually built**, not merely absent from what the request asks for. That is the
configuration asserted about the object rather than about the intent.

## The four phases, and what each may do

```
  -check      reads. Installs the hook, collects at most one Stage, injects nothing.
  -watch N    a bounded, clearly initiated human-input observation phase. Dry run, so
              nothing on the path can press. Reports counts only — observe.InputStats
              carries no key identity and no content by construction.
  -learn      asks, verifies the live ambiguity EXISTS, then watches a person demonstrate.
              Dry run. The only thing that touches the desktop is the person's own hand.
  -execute    one guarded click, behind -authorise-one-click. The one live runtime.
```

`-learn` stops before asking for anything if the live screen produces no surface question, and
reports that as the result. A screen where identity and surface do not come apart is a finding,
never a reason to loosen what counts as one.

## The three facts, reported separately

Every phase that could be mistaken for a success prints them apart:

```
  observed press ............ a hand pressed one surface and not the others
  attributed transition ..... the screen changed, and the correlator attributed the change
                              to exactly that one human action
  intended outcome verified   NOT IMPLEMENTED
```

The third is not a gap being apologised for. An attributed crossing establishes a causal
association under the stated conditions and nothing constructs
`confirmationVerified` — so `-execute` ends by printing *"READ IT YOURSELF AND SAY"* rather than
claiming the right page opened.

## What has happened since

`-check` has been run on the owner's machine, read-only, twice. The first run found the
configuration produced NO STAGE AT ALL — see
[[Experiment-062-what-a-stage-is-made-of]], which is the whole of why the window system now
enumerates when asked. The second found a window, a foreground application and 46 proposed
rectangles, and the one precondition still false is the ordinary one: the application has to
be in front.

The commands below are no longer how this is driven. `director try-it-out` is — one phase
per invocation, one checkpoint line each, operated rather than typed. See
[[ADR-209-the-assistant-operates-the-harness]].

## And then it was attempted, and blocked

The demonstration phase reached the surface question on the live screen and stopped on the
precondition after it: no durable place. That is a property of the configuration rather
than of the screen, and it is not something another page fixes. See
[[Experiment-063-the-configuration-that-cannot-close-its-own-loop]].

## What has not happened

**Nothing has been demonstrated and nothing has been pressed.** No watch, no demonstration,
no click. The two unmeasured
preconditions need the owner's hands and screen, and the owner's own qualification is the reason
this record stops here:

> this is authorization for a bounded test plan, not confirmation that I can operate your Windows
> machine from this chat.

## The commands, in order

```
  set MARCO_HOME=%TEMP%\marco-surface-test

  director demonstrate-surface -check -instruction "click Bluetooth & devices"
  director demonstrate-surface -watch 10
  director demonstrate-surface -learn -instruction "click Bluetooth & devices"
  director demonstrate-surface -reground -instruction "click Bluetooth & devices"
  director demonstrate-surface -execute -instruction "click Bluetooth & devices" \
      -authorise-one-click
  director reset-test-state
```

With Windows Settings in front on a page showing the navigation list, and — for `-learn` — a click
in the row, on a part of it the words do not cover.

## Related

- [[ADR-208-one-guarded-test-configuration-and-no-second-one]] — the configuration and its guard.
- [[Experiment-060-the-preconditions-for-one-live-demonstration]] — the blocker that led to it.
- [[Experiment-059-one-demonstration-from-a-person-into-durable-memory]] — the same loop, proven
  on recorded events.
- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the wiring the
  harness drives.
