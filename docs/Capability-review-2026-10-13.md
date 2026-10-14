---
type: reference
status: current
date: 2026-10-13
source_paths:
  - internal/director/workflowstore
  - internal/director/observe
  - internal/director/fastobserve
  - cmd/perceptionreplay
---

# Capability review — teach once, use later, at `f1a6030`

An architectural checkpoint before deciding whether a separately authorised, supervised
execution experiment is justified. Everything below is offline, over `rl-freeplay-1`. Nothing has
ever been executed.

## What is demonstrated

Measured on real recorded evidence, with the folds kept independent.

| | |
|---|---|
| **Record and learn** | 900 frames, 17 presses, 10fps. A 21-step ordered draft with 12 target identities, learned from one demonstration. |
| **Two perception sources** | The pixel proposer and the vendored ScreenParser detector, fused by provenance. Completed workflow prefix **1 → 9 of 21**; controls resolvable **12 → 13 of 13**; **0 false matches** either way. [[Experiment-031-the-workflow-gets-nine-steps-in]] |
| **Coverage, honestly scored** | **19 of 21** steps verified across both demonstrated routes, **0 unresolved**; the remaining 2 are where the recording ends. [[Experiment-032-what-the-recording-can-and-cannot-say]] |
| **A durable artifact** | 8,511 bytes, version 2. Round-trips; refuses corruption, unknown versions and unattached clarifications; carries no session identity at all, including inside prose. [[ADR-183-remembering-recognising-and-being-allowed-to-act]] |
| **Re-acquisition across sessions** | A control demonstrated in fold A, written to a file, found again by fold B from the file's own readings: **3 of 4**, one at **52 of 54 looks**. [[Experiment-033-a-workflow-asked-for-in-another-session]] |
| **Step-scoped planning** | A reloaded workflow grounded one step at a time: 6 screens established, **3 steps lowered into legal Marco and compiled**. [[ADR-187-knowing-the-plan-and-trusting-the-screen]] |
| **Refusal that fires** | A conservative binder that refuses an ambiguity on the first real screen; a clarification mechanism that asks only where the matcher genuinely cannot tell; a walk that stops rather than guessing. |

## What is simulated

Each is a labelled fixture in the readiness report, never a claim.

| | stands in for | what it proves |
|---|---|---|
| **Screen correspondence** | a person saying "that is the Play menu" | nothing about recognition. An oracle answers instantly and correctly, which no person does. |
| **Step confirmation** | a person agreeing to each step | nothing about whether they would. |
| **Execution authorisation** | a session carrying live permission | nothing. Without the `-authorise` flag the chain stops at stage 5, which is why it is a flag. |
| **The action boundary** | a press happening | nothing at all. No press has occurred; the boundary is a marker. |

## What requires a person, today

1. **Which screen is this.** Similarity ranks a present screen correctly 6 of 6 and nominates an
   absent one just as confidently — absence cannot be read out of a ranking.
   [[Experiment-036-recognising-a-screen-you-were-shown]]
2. **Which region is the control**, where one press left it ambiguous. `control_6` is still
   unanswered, and nothing may answer for a person.
   [[ADR-185-asking-the-smallest-missing-thing]]
3. **Agreeing to each step**, per session. A confirmation in the file is audit and never reusable.
4. **Authorising execution.** Unbuilt and unreachable; the planner cannot grant its own.

Measured cost on one route: **7 questions**, of which **6 were re-asks** of a screen already
established — 4 of those being real navigation, which is correct, and 2 one black frame.
[[Experiment-037-why-the-binding-disappeared]]

## What remains unbuilt

- **Autonomous screen recognition.** The gap is absence detection, and no measure available
  provides it. This is the single thing standing between the current chain and one that does not
  need a person per screen.
- ~~**Executing anything.**~~ **Built, and never run live.** `director perform-workflow` performs
  a saved workflow one authorised action at a time, through `internal/director/workflowrun`.
  `workflowstore` and `workflow` still cannot reach `marcoexec`, which their import walks hold;
  the coordinator sits outside both. **No live run has happened** — what is demonstrated is the
  whole path with `recordhost` at the bottom, which is a notebook.
  [[ADR-188-performing-a-saved-workflow]]
- **Verifying an action.** A destination established is not an action verified, and no
  post-action evidence exists. `Verified()` returns false with its reason.
- **Durable cross-session screen identity.** Deliberately: arrangements bind the wrong screen at
  0.886 while binding right ones at 0.556. [[Experiment-029-binding-a-screen-across-two-independent-folds]]
- **Recovery after partial progress.** `Progress` distinguishes stopping safely from finishing;
  nothing acts on the distinction.
- **Live integration.** The fast observer has never run against a live detector, and
  `stale-window` has only ever been exercised by a test.
- **Two controls that cannot be found again**, one of which is an identity problem no descriptor
  solves. [[Experiment-035-what-a-control-looks-like]]

## The honest one-line state

A workflow taught in one session can be saved, reloaded in another that shares nothing with it,
grounded a step at a time with a person answering where Marco cannot, compiled into legal Marco,
and now **performed** — one action that somebody permitted, in the observation they permitted it
in, with the world re-checked at the instant of the press. Every press so far has gone into a
notebook, because a live run is a decision and not a change.

## What a supervised execution experiment would need

Named here so the decision is about a list rather than a feeling:

1. ~~A real execution authorisation path, separate from confirmation.~~ **Built**: per action,
   per observation, expiring, and never inferred. [[ADR-188-performing-a-saved-workflow]]
2. ~~A Director command that performs a reloaded workflow step by step.~~ **Built**:
   `director perform-workflow`, refusing where the readiness report refuses and in four more
   places only a live press needs.
3. Post-action evidence good enough to tell arriving-where-expected from the press having caused
   it — or an explicit decision that destination-established is the bar, stated as such. **Still
   open**, and the honest state is that destination-established is what the code claims.
4. A live run of the two-speed perception path, which has never happened. **Still open**, and now
   the only thing standing between this and a supervised experiment.
5. A decision about the 6-questions-per-route cost, which is a product question and not a
   technical one. **Still open**, and performing adds two more questions per step — one to agree
   to it and one to permit it.
