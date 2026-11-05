---
type: adr
status: accepted
date: 2026-11-05
decision: "ACCEPTED — THE CONTROL CENTRE STOPS ITSELF ONCE ITS PAGE HAS GONE. `marco ui` served until Ctrl+C in a terminal nobody could see when the widget's help started it, so one was found running a day later, holding marco.exe and refusing plugins/studio/build.cmd. The server now counts every request as the page being there, the page says so every 20 seconds (`/api/alive`), and after three minutes with nothing heard and no run it started still going, it shuts down. Its accessibility bridge ends with it, because uia.exe exits when its input closes. `exit` in the widget still quits only the widget."
supersedes: []
affects:
  - interface
source_paths:
  - cmd/marco/idle.go
  - cmd/marco/edit.go
---

# The control centre stops once its page has gone

## What happened

- The owner said `m exit` "didn't close the director process".
- There was no Director: it had stopped as [[ADR-308-opening-the-widget-asks-for-the-director]]
  says it does. What was still running was `marco.exe ui help`, started by the widget's `help`
  at 8:37 PM the day before, and its `uia.exe` child. The shell that launched it was gone.
- The next morning `plugins\studio\build.cmd` refused to build because that process was running
  from `marco.exe`.
- `marco ui` blocked in `http.Serve` until Ctrl+C. Closing the browser tab closed nothing.

## Decision

- **The page's presence keeps the server up.**
  - Every request counts.
  - The page also posts `/api/alive` every 20 seconds, and again when it becomes visible.
- **The server stops after `pageGone` (three minutes) with nothing heard.**
  - It is minutes, not seconds, because a hidden tab's timers are throttled to about once a
    minute.
  - The same allowance covers a browser that never opens the page.
- **Never while a run it started is still going.** The page is waiting for that run's answer;
  once the run has finished there is nobody left to ask.
- **The bridge ends with it.** The `uia.exe` the control centre started reads its input until
  end of file, so it exits when its parent does.
- **The page says when it has been left behind.** It shows *This control centre has closed* and
  how to open it again, rather than failing quietly on the next click.
- **`exit` in the widget is unchanged.** The control centre is not the widget's in any sense a
  person can see, and a server that ends when it is no longer used is the rule the Director
  already follows.

## Verified (Windows only)

- Through `serveEditor`, the server `marco ui` runs, with its real doors:
  - it stops once nothing has been heard for the allowance;
  - it stays up for four times the allowance while `/api/alive` is posted, then stops;
  - it waits for an unfinished run, then stops once the run finishes.
- The page posts to `/api/alive`, which is registered (`TestEveryDoorThePageKnocksOnIsAnswered`),
  and has the closed notice.
- **Mutants:** 5, all killed:
  - never stops;
  - ignores a running run;
  - requests do not count;
  - no alive door;
  - the page never pings.

## Not established

- **Not seen on the desktop.** The rebuilt `marco.exe` has not been run from the widget, and
  throttling in a real browser has not been observed.
- **`runEdit` calling `serveEditor` is not itself under test.** `runEdit` opens a browser, so no
  test enters it.

## Enforced by

- `cmd/marco` `TestTheControlCentreStopsOnceItsPageHasGone`,
  `TestTheControlCentreStaysWhileItsPageIsOpen`,
  `TestTheControlCentreWaitsForARunItStarted`,
  `TestThePageKeepsItsControlCentreAliveAndSaysWhenItHasClosed`,
  `TestEveryDoorThePageKnocksOnIsAnswered`.
