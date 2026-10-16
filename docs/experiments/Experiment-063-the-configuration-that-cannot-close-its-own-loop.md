---
type: experiment
status: complete
date: 2026-10-16
backend:
  - structure
  - ocr
  - accessibility
fixture: "Windows Settings on the owner's machine, the same page probed twice — once under the pixel-only test configuration of ADR-208 and once under production observation defaults — with one variable changed and nothing pressed"
result: the-pixel-only-configuration-cannot-establish-a-durable-place-because-a-screens-signature-discriminates-on-interface-terms-which-is-what-accessibility-supplies-so-the-configurations-two-requirements-are-mutually-exclusive-and-a-surface-question-turns-out-to-arise-under-production-observation-anyway
supersedes: []
affects:
  - perception
  - execution
---

# Experiment 063 — the configuration that cannot close its own loop

**Run 2026-10-16, on the owner's machine, read-only, nothing pressed.** The first operated attempt
at [[Experiment-061-the-live-focused-demonstration]].

## What was being attempted

The demonstration phase, on Windows Settings, under the configuration
[[ADR-208-one-guarded-test-configuration-and-no-second-one]] approved. It reached the thing the
whole experiment is about and then stopped one precondition later.

```
the live Stage: 65 Props; "installed apps" → two or more defensible surfaces
  candidate e28 {637 424 126 18}  area 0.1%  DEFENSIBLE
  candidate e27 {613 400 492 66}  area 1.6%  DEFENSIBLE
  2 surfaces containing "installed apps" are equally defensible on this
  evidence (e28, e27), and nothing here chooses between them

STOP: no durable place was established for this screen
```

**The surface question is real on a live screen.** Until now it had only ever been measured on
recorded fixtures. That part of ADR-206 and ADR-207 is confirmed outside a test.

## The question this experiment answers

A demonstration needs two independent things to be true of a screen. A **question** — two
defensible surfaces around one name — and a durable **place**, because a target with no place has
no durable identity and an answer could not be kept. The first attempt had the question and not
the place, on a page that had been still for three minutes.

So: is that about this screen, or about the configuration?

## Measured, one variable

The same page, the same probe, twenty seconds each, nothing pressed.

```
A. PIXEL-ONLY                          B. PRODUCTION OBSERVATION
   sources [window_system structure ocr]  defaults, accessibility included
   65 Props                               159 Props

   PLACE: no                              PLACE: yes — subj_e452fe50a697
     12 x not_discriminating
      4 x not_settled
     settled=true coherent=false
     named=false

   2 words would raise a question         1 word would raise a question
     "installed apps"  2 options            "Dark"  2 options
     "Recommended"     2 options
```

**It is the configuration.** A screen is establishable when its signature can ever match again,
and `StructureSignature.Discriminating()` is `(TermsKnown && len(Terms) > 0) || Envelope != nil`.
Terms are interface concepts classified from the screen's text and credited to the state that was
active. Under pixel-only nothing was credited at all — `named=false`, and not one word tallied
against the screen — so the signature discriminated nothing and twelve readings out of sixteen
were refused for exactly that.

### So the configuration's two requirements are mutually exclusive

```
  a surface question exists      only where accessibility does not describe the control
  an answer can be kept          only where the screen is recognisable
  the screen is recognisable     by interface terms, which is what accessibility supplies
```

Nothing is wrong with any one of those three. Together, under one configuration, they cannot all
hold — and no choice of screen changes it, which is why this is a record about the configuration
and not a search for a better page.

## And the premise of the detour turns out to be false

[[Experiment-060-the-preconditions-for-one-live-demonstration]] recommended pixel-only on this
reasoning:

> where an accessibility tree describes the control there is nothing to ask — Settings is XAML with
> full UIA, so `click Bluetooth & devices` resolves to a real control and never asks.

Column B says otherwise. Under production observation, with 159 Props all from the accessibility
walk and no pixel evidence in the cycle at all, `click Dark` produces **two tied defensible
surfaces** on the Personalisation panel: a caption and the tile it labels, both described by the
tree, with nothing choosing between them. The question the whole feature exists for arises from
the accessibility tree on its own.

That was not predicted and it is the most useful thing in this record.

## What was not done, deliberately

Nothing was pressed and nothing was demonstrated. The loop was not run under column B either,
because that is a different experiment from the one that was approved: the owner's purpose was

> to prove that Marco can learn when accessibility information is unavailable

and column B proves something else — that Marco can learn when accessibility describes a different
extent of the thing than the words do. Worth proving, and not this.

`click Dark` is also not a safe target for the execute phase: pressing that tile changes the
desktop theme, which is a real settings change.

## Related

- [[ADR-208-one-guarded-test-configuration-and-no-second-one]] — the configuration this blocks.
- [[ADR-209-the-assistant-operates-the-harness]] — the orchestration that made four live
  diagnoses cost one session instead of four.
- [[Experiment-062-what-a-stage-is-made-of]] — the blocker before this one.
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] — what a place is and what licenses one.
