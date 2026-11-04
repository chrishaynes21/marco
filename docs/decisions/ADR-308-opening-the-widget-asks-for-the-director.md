---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — OPENING THE WIDGET ASKS FOR THE DIRECTOR, AND A DIRECTOR STARTED FOR THE WIDGET STOPS ONCE THE WIDGET HAS GONE AND NOTHING IS USING IT. The widget's status reads never start a Director (ADR-267), so until the first request that needed one, the widget said the Director was not running. Opening the widget is a person asking for it, so the widget now runs `marco director start --for=<its pid>` once, off its main thread. A Director that call STARTS is served with `--for`: nothing happens while that process lives; once it has exited, the Director checks once a second whether anything is using it, and that check and the decision to stop are ONE act under the lock that admits connections (Server.ShutdownIfUnused), so a client arriving as it stops is either admitted first and keeps it, or refused as `stopping` before any of its work is accepted. In use means a command running, a question or confirmation outstanding, a client connection open, an observation, or watching. A Director that is ALREADY running is left exactly as it is and is not stopped when the widget closes, because the widget did not start it. The owner is identified by pid AND start time and waited for on a handle, because a pid is not a process. Unchanged: reads still never start a Director, a request that needs one still starts one that runs until shut down, and MARCO_DIRECTOR=off still asks for nothing."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - plugins/overlay/startdirector.go
  - plugins/overlay/main.go
  - cmd/marco/director.go
  - cmd/director/serve.go
  - cmd/director/servefor.go
  - internal/director/service/server.go
  - internal/director/service/client.go
  - internal/gate/exited_windows.go
---

# Opening the widget asks for the Director

## What a person saw

The widget's headline and watch panel read the Director's state every few seconds. Those reads
never start a Director: a question with a side effect is the one thing a read must not be (see
[[ADR-267-director-owns-route-resolution]]). Nothing else started one when the widget opened:
not Open widget, not `overlay.cmd`, not the installer. So a freshly opened widget said the
Director service was not running until somebody typed something that needed it. The message was
true, but the person had opened the widget in order to use it.

## The decision

**Opening the widget counts as asking for the Director.** When it opens, the widget runs
`marco director start --for=<its own pid> --json` once, on its own goroutine, and logs the answer.
It never waits for it. All three launchers start the widget through `overlay.exe`, so this one
call covers all of them.

**A Director started for the widget stops once the widget has gone and nothing is using it.** With
`director serve --for <pid>`:

- nothing happens while that process is alive, however idle the Director is;
- once it has exited, the Director asks once a second whether it may stop, and stops the first
  time nothing is using it;
- **the reading and the stop are one act** (`Server.ShutdownIfUnused`). Both are made under the
  lock that admits a connection, so a client that arrives as the stop is decided is either
  admitted first, counted and keeps the Director, or refused with `stopping` before any request
  of it is read. The client reports that refusal as such, not as a bad token. No work is accepted
  that the stop then cancels;
- in use means a command running, a question or a confirmation still owed an answer, a client
  connection open, an observation session, or watching. The server answers the first four
  (`Server.InUse`); the runtime answers the last two, read under the same lock.

**One that is already running is left alone.** `marco director start` passes `--for` only to a
Director it starts. One that is already running was started by somebody else, so the widget
closing does not stop it, and it is not told whom it is for.

**The owner is an identity, not a number.** The pid and its start time are read once, and the
exit is waited for on a handle whose start time is checked (`gate.Exited`), for the same reason
`gate.End` checks it. An owner that cannot be identified counts as gone, so a Director started for
nobody findable does not run on for ever on its account.

## What did not change

- **Reads never start a Director.** The headline, watch panel, `marco resolve` and the learn
  offer are as ADR-267 left them.
- **A request that needs a Director still starts one,** and that one runs until it is shut down.
  So does one started by hand, or with no `--for`.
- **`MARCO_DIRECTOR=off` asks for nothing.**
- **Studio is unaffected.** Its recording and its Play run single-job `director` commands of their
  own, which claim no home. The dry-run Director behind its feasibility shell (`Backend.reach`)
  still refuses to attach to one it did not start, and the page does not use it to run anything.
  Only one Director serves a home at a time (`homelock.ClaimHome`), and that is still so.

## Consequences

- An idle Director, with vision off by default, takes one look at the screen when it starts
  (`Runtime.Warm`) and otherwise only answers requests. Watching starts only when a person asks
  for it. That one look now happens when the widget opens, instead of on the first request.
- **A question nobody answers keeps it running.** An outstanding clarification or confirmation
  counts as in use, so a Director asked something just before the widget closed stays up until
  the question is answered or withdrawn. That was chosen over stopping in the middle of a
  conversation.

## Verified (Windows only)

**The ordering, deterministically.** Two test hooks force both orders, with no delay changed
(`internal/director/service` tests below). In one, a client connects while the stop is being
decided, and the decision waits until that connection is at admission: it is refused with
`stopping`, and nothing reaches the runtime. In the other, a client admitted first keeps the
Director, and its work is done.

**Real processes, BEFORE the ordering fix.** Temporary builds of `marco.exe` and `director.exe`,
an isolated `MARCO_HOME`, a sleeping PowerShell standing in for the widget. Nothing was typed or
clicked, and no widget was opened. Output:
`docs/experiments/Roadmap-38-output/2026-11-04-director-for-widget.txt`. This ran the earlier
two-readings stop, and was not repeated after the fix.

1. Started for the stand-in: still running 5 seconds later while it lived. About 2 seconds after it
   ended, the Director had exited and its endpoint file was gone.
2. A Director already running, started with no owner: asked again for the stand-in, it answered
   `"started":false`. Six seconds after the stand-in ended it was still running, and was then shut
   down by hand.

**The first version was not atomic,** and a review found it. It read `InUse`, and then called
`Shutdown`, on two readings a second apart. A client accepted between the last reading and the
stop would have had its newly accepted work cancelled. Its comment claimed the second reading
prevented that, which was wrong. It is replaced by `ShutdownIfUnused`.

The first full `cmd/director` run FAILED: `TestEveryValueTakingFlagIsInTheReorderingTable` found
`--for` missing from the `valued` table in `explain.go`. Its output was not kept. The rerun that
named the failure is `2026-11-04-director-for-widget-cmd-director-rerun.txt`; the run after that
fix is `2026-11-04-director-for-widget-cmd-director.txt`.

**Mutants,** each applied alone and the file restored, all killed. The first 16 covered: the
widget no longer asking, ignoring `MARCO_DIRECTOR=off`, asking without `--for`, marco dropping
`--for`, marco not looking for a running Director, each of the three server readings dropped,
the start time not checked, the stop never started, runtime activity ignored, not waiting for
the owner, an unfindable owner never counting as gone, and `--for` left out of `valued`. Two of
them (one idle reading enough, a busy reading not resetting the count) tested the two-readings
rule, which is gone. Seven more for the atomic stop: admission ignoring the decision, the stop
not taking the admission lock (failing because the late connection was admitted, 3 of 3 runs),
the runtime's account not asked, stopping while in use, the client not reporting the refusal,
serve stopping without the check, and serve dropping the runtime's account.

## Not established

- **Not seen on the desktop.** Nobody has yet opened the widget built from this change and watched
  the status change from not running to running, or closed it and seen the Director go.
- The in-use readings are checked against a stand-in server and a scripted sequence, not against
  a person mid-conversation when the widget closes.

## Enforced by

- `plugins/overlay` `TestOpeningTheWidgetAsksForTheDirector`: main runs `go startDirectorForWidget()`,
  it asks once for a Director `--for` its own pid, and with `MARCO_DIRECTOR=off` it asks for
  nothing.
- `cmd/marco` `TestStartingTheDirectorLeavesARunningOneAlone` and
  `TestADirectorStartedForTheWidgetIsToldWhomItIsFor`: a running Director is found and left alone
  without anything being started, and one that is started gets `--for`.
- `cmd/director` `TestADirectorStartedForAProcessStopsWhenItHasGoneAndNothingUsesIt`: it is not
  even asked to stop while its process lives, keeps asking and running while in use, and stops
  once both conditions hold.
- `cmd/director` `TestAnOwnerIsWaitedForUntilItExits` and `internal/gate`
  `TestAnExitIsWaitedForOnTheProcessThatWasIdentified`: a live owner is waited for, its exit is
  seen, a pid naming another instance is refused, and an owner that cannot be found counts as gone.
- `cmd/director` `TestServeForWiresTheStopToTheServiceAndTheRuntime`: `runServe` starts the stop
  with `srv.ShutdownIfUnused(rt.runtimeInUse)`, so the reading and the decision are one act.
- `internal/director/service` `TestTheServiceSaysWhatIsUsingIt`: a running command, an open
  client and an unanswered question each keep it in use, and once they have gone nothing does.
- `internal/director/service` `TestAConnectionArrivingAsTheIdleStopIsDecidedIsRefusedBeforeAnyWork`
  and `TestAConnectionAdmittedBeforeTheIdleStopKeepsTheDirector`: both orders of a client and the
  idle stop, forced by hooks. Late, it is refused with `stopping` and no work reaches the runtime;
  early, it keeps the Director and its work is done. The runtime's account is asked as well.
- `cmd/director` `TestEveryValueTakingFlagIsInTheReorderingTable`: `--for` takes a value, so it is
  in the `valued` table.
