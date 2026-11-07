---
type: adr
status: accepted
date: 2026-11-07
decision: "ACCEPTED — ↑ AND ↓ ON THE WIDGET'S COMMAND LINE RECALL COMMANDS LIKE A SHELL'S HISTORY, AND A ROUTINE COMES BACK WITHOUT ITS VALUES. ↑ puts the most recent command back on the line, editable, and walks back; ↓ walks forward and, past the newest, puts back what was being typed before the first ↑ (with the Director proposal it was open on). Enter on a recalled line is an ordinary Enter: read, reviewed and run. A line the engine reads as a routine is kept as the routine's name (a repeat with its count), so its reading asks for the value again; any other typed line is kept as typed; a spoken line only when it named a routine. In memory only, 100 commands, the same command twice running kept once."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/overlay/recall.go
  - plugins/overlay/line.go
  - plugins/overlay/acts.go
  - plugins/overlay/intake.go
  - plugins/overlay/controller_windows.go
  - plugins/overlay/model.go
---

# The command line recalls commands

Builds on [[ADR-301-the-command-line-reads-back-what-it-will-run]], [[ADR-324-the-history-shows-what-ran]] and [[ADR-325-repeat-a-routine-a-fixed-number-of-times]].

## Why

The owner asked for ↑ and ↓ on the widget's command line to work like zsh: ↑ recalls the most
recent command and walks back, ↓ walks forward and then restores what was being typed. A recalled
routine that takes values should come back as its name with its hints and ask for fresh values,
"so you can quickly reuse `cloud pull` without saving what you said last time".

## Decision

- **Keys.** While the command line is open, ↑ and ↓ are swallowed from the application and queue
  `actRecallOlder` / `actRecallNewer` (the hook thread only pushes; the walk happens on the action
  loop). Opening the line and Enter each end the walk, so the next ↑ starts at the newest.
- **The walk** (`recall.go`). The first ↑ keeps the line as it was and the proposal it was open on;
  a recalled line drops the proposal, which was for the routine on that line and no other; ↓ past
  the newest puts both back. ↑ on the oldest stays there. Edits to a recalled line are not kept
  when the walk moves on.
- **What is kept**, decided where the engine's reading of the line is known (`typedLineFrom`,
  `recallOf`): a routine's name, or `repeat <name> <N> times`, and never a value, so the reading
  of the recalled line shows the missing value (‹resource›) and asks for it. Any other typed line,
  and the widget's own words, as typed. A spoken line only when it named a routine, so what the
  microphone heard in passing is not walked through. A line that had no reading before it ran is
  kept as typed and replaced by the routine it ran as, if it ran as one.
- **Where it lives.** This process only: 100 commands, never written down and never logged.

## Not established

- Not tried live.
- A typed line that had no reading and did not run as a routine keeps what was typed, values
  included if it had any: the engine said nothing about it either way.

## Enforced by

- `plugins/overlay` `TestUpAndDownWalkTheCommandsLikeAShell`, `TestTheRecallKeepsEachCommandOnceAndNotForever`, `TestARecalledRoutineAsksForItsValueAgain`, `TestALineWithNoReadingIsKeptAsTheRoutineItRanAs`, `TestTheArrowKeysRecallOnTheCommandLine`.
