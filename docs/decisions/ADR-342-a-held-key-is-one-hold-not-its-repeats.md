---
type: adr
status: accepted
date: 2026-11-09
decision: "ACCEPTED — A KEY HELD DOWN IS ONE HOLD, NOT ITS REPEATS. Windows sends a key-down about thirty times a second while a key is held, and the recorder keeps them; `simplify` read each as a new press, so the owner's \"return to orbit\" (Tab, then J held for seconds) came out as about a hundred holds of J folded into loops. `simplify.Simplify` now drops a down of a key that is already down before anything else reads the events, so the hold is timed from the first press to the release: KeyDown j, the time it was held, KeyUp j. And Simplify's clamp of waits to 250ms (SimplifiedMaxWaitMs) no longer touches a wait while a key is held, which is how long it is held: a five-second hold clamped to a quarter of a second lets go before the game acts. Pauses either side are still clamped."
supersedes: []
affects:
  - execution
source_paths:
  - internal/simplify/simplify.go
---

# A held key is one hold, not its repeats

Follows [[ADR-339-a-modifier-pressed-alone-is-a-key]], which left this alone and said so.

## Why

The owner recorded "return to orbit" in Destiny 2: Tab, then J held about three and a half seconds.
The recording had one Tab, 96 downs of J and one release; the play had the Tab and then J held,
released and held again in nested loops.

## What it changes for a text box

A key held in a text box types the character over and over because Windows repeats it; an injected
hold does not repeat by itself. So a held key now types one character where holding it typed many.
A game reads the key as down for the whole time, which is what a game wants, and the old output was
not a faithful copy of the typing either: it was holds folded into loops.

## Not established

- Not tried in the game. The owner's own recording was put through the new rules, read-only, with
  the options a save uses: Activate, 1.82s, Tab, 2.40s, KeyDown j, 3.64s, KeyUp j.
- A play saved before this keeps its loops until it is rebuilt (More, Review today's rules, Replace)
  or recorded again.

## Mutation run

Five mutants (the repeat skipped, the pass called, a release forgetting the key, the held wait left
alone, the hold ending); all killed. Three orchestrator tests and one simplify test typed two of
the same letter as two downs with no release between, which a recording never contains and which
now reads as one held key; each was given its release.

## Enforced by

- `internal/simplify`: `TestAKeyHeldForSecondsIsOneHoldNotItsRepeats`, `TestSimplifyClampsPausesButNotHowLongAKeyIsHeld`, `TestShiftedPlaceholderRecords`.
- `internal/orchestrator`: `TestLearnSimplifyOption`, `TestLearnSimplifySaves`, `TestSimplifyRoute` (two l's in "hello").
