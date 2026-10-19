---
type: adr
status: accepted
date: 2026-10-18
decision: "ACCEPTED — an action's outcome verdict rests on readings taken on BOTH sides of it, each a distinct reading, over a full settled window, scoped to the window acted on rather than to the screen id it was acted under. A step whose outcome must be verified is not acted on until the screen it stands on has been read at least once, and waiting costs no authorisation. One side of the evidence is UNCONFIRMED, never CONTRADICTED. A transition landing inside the settled window restarts it, bounded. And a command's window selector may be as exact as the operator can make it — `--foreground`, `--pid`, `--window-id` — because choosing what to look at is not permission to act there: the scope is still asked of the window acquired."
supersedes: []
affects:
  - execution
  - perception
source_paths:
  - internal/director/workflowrun/verify.go
  - internal/director/workflowrun/workflowrun.go
  - cmd/director/performworkflow.go
  - cmd/director/capturegamesession.go
  - docs/experiments/Experiment-090-the-first-guarded-pixel-action.md
---

# ADR-238 — a verdict needs both sides, and a window of its own

## The defect

The local outcome verifier was built, measured offline on two recordings, tested, and wrong in
five ways the first time a real press went through it. Every one of the five would have reported
a FALSE verdict about an action that worked, and the only reason they were caught is that the
test surface published what it actually received, in its own window title, where Marco could not
reach it. [[Experiment-090-the-first-guarded-pixel-action]] has the measurements.

The owner's standard is why the witness existed at all:

> the test must confirm an actual input was emitted and the expected result was independently
> observed. A successful plan, matched control, or command exit code isn't enough.

## The decision, in five parts

**A reading is one piece of evidence, however many looks it stands for.** OCR answers about four
times a second against a loop that looks ten times, so the last reading stands for the looks
that follow it. Counting it once per look filled a five-reading window from a single reading and
did it faster than any application could respond: a press was contradicted 572ms after it was
sent, on five copies of the reading taken before it. `Sighting.WordsSeq` is an identity for
which reading the words are — deliberately an identity and not a timestamp, because the question
is whether this is another reading, which the caller knows exactly. Only a reading later than
the one standing at the moment of the action is after-evidence.

**A contradiction needs both sides.** Five readings after and none before is not enough evidence
disagreeing; it is the question not being asked. That reading of it blamed the application for a
look Marco never took, and the difference between "it did not work" and "I did not look" is the
difference between a bug report and a retry. One side is UNCONFIRMED.

**And nothing is acted on until its screen has been read.** Which is what made the above
unanswerable by construction: `--from` established the screen on the first observation and the
press went out on the second, before any reading existed. A step carrying an outcome expectation
now waits for one reading of the screen it stands on, within the same patience every other
not-ready-yet case gets, and the new stage `StepScreenUnread` says which fact is missing —
established and read are different questions and a reader told the wrong one looks in the wrong
place.

The wait goes BEFORE authorisation is asked for, and that position is the whole fix. Placed
after, it waited correctly and the run still sent nothing: the exploration grant is taken when
somebody is asked, and `--max-actions 1` was spent on the observation that waited. **A wait must
cost no permission.**

**The evidence is scoped to the window, not to the screen id.** The after-readings had to carry
the same screen id as the action, so that a later reading of somewhere else could not be
credited to it. The window is what that rule was reaching for. Measured live: the press worked,
the run itself observed `visual_1 -> visual_2`, visual_2's words were the eighteen the step
expected, and not one reached the verifier. The verifier exists for the case where the screen
does NOT change ([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]); a change
large enough to be named must not be the one thing that turns it off. What stops an unrelated
screen being credited is not the label: it is that the words read must overlap the expectation
the step SAVED, by `OutcomeWords` of them.

**A transition inside the settled window restarts it, and a verdict needs the window full.**
`OutcomeSkipReadings = 2` was measured on a recording read at every look; live, a page took
longer than two readings to redraw, the window straddled the change, and the persistence term
correctly called every word transient. A fixed skip cannot know when the response happened, so a
reading bringing `OutcomeWords` words the previous one lacked is the transition, and the window
begins there — asked through `OutcomeBetween` so there is one statement of what "appeared"
means, and bounded, because a screen that keeps changing would restart forever.

The first version of that restart verified on ONE reading, where persistence says nothing at
all: the first reading after a transition always carries every word it carries. A verdict either
way now requires `OutcomeSettleLooks` readings. The same case went from "verified after 774ms on
1 reading" to "verified after 2.495s on 5", which is the term doing its job rather than being
present.

## And the window selector may be exact

`--application` is the weakest selector and the only one that can be ambiguous. On the owner's
machine a controlled test page could neither be captured nor acted on, because their own Edge
was showing a floating `Search bar` — and the refusal pointed at `director windows --window-id`,
a command needing a daemon that was not running, naming a flag neither command had. The advice
was unfollowable and there was no way through at all.

So `capture-game-session` and `perform-workflow` both take `--foreground`, `--pid` and
`--window-id`, resolved in order of exactness, exactly one at a time. This is permitted
precisely because it authorises nothing:

	discovery picks what to look at, the scope decides whether the file may act there, and a
	mismatch between them is a refusal rather than a negotiation

The scope gate is asked of `v.Ref.Application`, the executable of the window actually acquired.
Measured: an `msedge`-scoped artifact run with `--foreground` while Notepad led the desktop was
refused before anything that could press existed, naming both applications, with nothing
pressed.

**And a capture records the application it OBSERVED, not the one it was asked for.** The field
was `*app`, the flag — empty under `--foreground`, and a caller's word otherwise. Everything
downstream treats it as provenance; `perceptionreplay -fixture` makes it the derived workflow's
scope. A scope taken from a flag is a scope taken from whoever typed it. Found the hard way: 20
frames of the owner's game were captured and labelled with it, and the capture was deleted.

## Enforced by

`internal/director/workflowrun/verifylive_test.go`, seven tests over the identified-reading
path, each naming the live measurement it came from. That file exists because the first mutation
round over this work killed only 2 of 9: every test already in `verify_test.go` leaves
`WordsSeq` zero, and therefore exercises the behaviour this ADR replaced rather than the
behaviour it decides. Ten mutations now, all killed.

Plus `TestAScreenChangeAfterTheActionIsStillTheActionsEvidence` and
`TestAnotherWindowsReadingsDoNotVerifyTheAction`. The second replaces a test that asserted the
opposite of the first — that another SCREEN's readings were not an action's evidence — moved to
the level its own stated fear belongs to, which was another WINDOW. And `cmd/director`
`TestPerformWorkflowInstallsItsGates`, whose installed-call list names both halves of the
reading identity and why one without the other fails.

Two survivors are deliberate and are documented at their sites rather than left looking like
gaps: `readsOf`'s per-screen filter and the `asked` term in `verdict` are both unreachable from
`Advance` while the wait in section 4b stands — `noteReading` runs after the usability gate, so
a look Marco could act on is a look Marco read. Each is kept as the statement of what its rule
IS, on the same argument `observe.Independent`'s site filter is kept on.

## See also

- [[Experiment-090-the-first-guarded-pixel-action]]
- [[ADR-237-an-outcome-is-only-an-outcome-of-something]]
- [[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]]
- [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]]
