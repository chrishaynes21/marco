@echo off
rem Watch what the Director believes it is looking at, live, while you navigate.
rem
rem   watch-interface.cmd                          trackmania pause menu, 4 minutes
rem   watch-interface.cmd trackmania 150 maps      the MAP-SELECTION screen, 2.5 minutes
rem   watch-interface.cmd rocketleague 180         another application, another length
rem
rem The `maps` protocol is the one to run after Experiment-025: the pause menu does not present
rem as a list to this detector and the map-selection screen does. It expects the list to be
rem ON SCREEN BEFORE YOU START.
rem
rem ---------------------------------------------------------------------------
rem WHAT THIS IS FOR
rem
rem A passive observation session with the experimental detector running, printing the
rem session's own answer to "what interface am I looking at, what is selected, and what
rem changed since the last observation" every two seconds. It injects NOTHING: no key, no
rem click, no controller. You navigate; it watches.
rem
rem WHY IT IS A SCRIPT AND NOT THREE COMMANDS
rem
rem Because the three commands have to agree about four pieces of configuration, and a run
rem whose configuration nobody recorded is a measurement of an unknown system. In particular:
rem
rem   MARCO_SHADOW_UNGATED  the purchase policy declines an inference whenever the primary
rem                         reading claims to be sufficient. On a game it always claims that
rem                         — accessibility exposes a window and a pane — so the only sensor
rem                         that can see the menu is declined. The session report names
rem                         the policy, so no number from this script can be read as if
rem                         it had come from the default one.
rem
rem                         ON THE NUMBERS THAT USED TO BE HERE. This block cited "1
rem                         inference in 78 looks gated, 130 in 322 ungated". The second
rem                         number was wrong, and the switch was not doing what it said:
rem                         the request's own source set is a SECOND gate this variable
rem                         does not touch, so 116 of those 130 were a detector nobody
rem                         asked, 0ms each. ADR-149. Fixed, and now measured at 55
rem                         inferences in 55 slots, 0 unproven, median ~2.0s.
rem
rem   MARCO_HOME            an isolated store. Nothing here should touch the real one.
rem
rem It also shuts down whatever Director is already serving that home first. `--check` style
rem readiness reporting about a process that is not the one you just configured has cost this
rem project several afternoons; see the dogfood stale-director note.
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"

set "APP=%~1"
if "%APP%"=="" set "APP=trackmania"
set "SECS=%~2"
rem WHICH PROTOCOL. `menu` is the pause-menu protocol; `maps` points the same instrument at
rem the map-selection screen.
rem
rem Experiment-025 is why there are two. That run navigated the Trackmania PAUSE MENU for four
rem minutes - 92 directional presses, the menu legible in every phase - and no list was ever
rem identified, because as this detector sees it the pause menu is about five boxes at
rem unrelated positions and sizes. Zero vertical stacks of three or more similar rectangles in
rem 67 navigation looks, and no choice group or menu-like state formed in any of them.
rem
rem The map-selection screen is the opposite, and the run only saw it because the operator
rem exited the map by mistake: 20.8 boxes a look against 4-5, and the only uniform list in the
rem whole run - 8 rows, gaps 0.032-0.034, identical width, height and left edge. Director named
rem it a choice group with nine seconds left on the clock.
rem
rem So the pause-menu protocol is kept, unchanged, because it is the baseline Experiment-025
rem measured and a baseline that gets edited is not one. The maps protocol is where the
rem evidence is.
set "PROTOCOL=%~3"
if "%PROTOCOL%"=="" set "PROTOCOL=menu"
if /i "%PROTOCOL%"=="maps" set "SECSDEFAULT=150"
if /i not "%PROTOCOL%"=="maps" set "SECSDEFAULT=240"
rem THE PROTOCOL BELOW HAS SIX PHASES AND THE LAST ONE HAS TO OUTLIVE A CONSTANT.
rem An interface belief survives interfaceForgetLooks = 10 looks that bring nothing
rem before it is dropped, and a look costs about two seconds, so "the menu is closed"
rem needs 30s+ of looking at nothing to show the belief being LOST rather than merely
rem going stale. 240s divides into the six phases with that one long enough.
if "%SECS%"=="" set "SECS=%SECSDEFAULT%"

if not exist "director.exe" (
  echo watch-interface: no director.exe. Build it first:
  echo     go build -o director.exe ./cmd/director
  exit /b 1
)
if not exist "plugins\vision\vision.exe" (
  echo watch-interface: no vision plugin. The default `go build` in plugins/vision
  echo                  produces a detector that detects NOTHING. Build it with:
  echo     plugins\vision\build.cmd
  exit /b 1
)

set "MARCO_HOME=%LOCALAPPDATA%\marco-liveiface"
rem THE RUN RECORDS ITSELF. A session that has to be watched live to be read is a session
rem whose evidence depends on somebody being at the keyboard at the right moment, and the
rem thing being tested here is the half a person has to do by hand. These files are the
rem whole record: every slot and what became of it, and the session's final answer.
rem
rem EACH RUN GETS ITS OWN DIRECTORY, because the previous version deleted the last run of
rem evidence at the top of this one. The question the next run asks is a COMPARISON - did
rem preserving unreadable geometry change what the detector admits, against the 0 regions on
rem 56 looks this script measured on 2026-10-09 - and a script that destroys the baseline it
rem is to be measured against cannot be used that way twice. Numbered and not timestamped:
rem %DATE% is locale-dependent, and a run directory nobody can sort is barely better than
rem one file. Run 0001 is Experiment-024, moved into this scheme by hand.
set "RUNS=%LOCALAPPDATA%\marco-liveiface\runs"
if not exist "%RUNS%" mkdir "%RUNS%"
set /a RUNN=0
:nextrun
set /a RUNN+=1
set "RUNID=000%RUNN%"
set "RUNID=%RUNID:~-4%"
rem A GLOB, NOT AN EXACT NAME. Run 0001 is on disk as `0001-experiment-024`, because a
rem baseline worth comparing against is worth naming, and an exact test for `0001` does
rem not see it - so this run would be a SECOND 0001 sitting beside it. `for /d` runs its
rem body only for directories that exist, so no match means no skip.
for /d %%d in ("%RUNS%\%RUNID%*") do goto nextrun
set "WATCH_OUT=%RUNS%\%RUNID%"
mkdir "%WATCH_OUT%"
echo This run records itself to %WATCH_OUT%
set "MARCO_SHADOW_TRACE=%WATCH_OUT%\slots.jsonl"
rem AND WHEN EACH PHASE BEGAN, which is what makes the two regimes separable.
rem The gate confirms a change on a second agreeing look, so slow navigation and fast
rem navigation are the two cases it must be judged on separately — and a trace of a run
rem that mixed them answers neither. Wall clock here maps onto the trace's at_ms,
rem which is measured from this Director's start: the session line below is the anchor.
set "PHASELOG=%WATCH_OUT%\phases.txt"
rem AND EVERY TICK'S ANSWER, not just the last one. The loop below already asks the
rem session what it believes every two seconds and prints it to a screen that is then
rem cleared. The final aggregate cannot answer "how many changes were confirmed WHILE
rem the player was moving fast", because it is one number for the whole run; the
rem per-tick answers can. One JSON object per line, and phases.txt says which tick
rem each phase started after, so the two files index each other.
set "TICKLOG=%WATCH_OUT%\ticks.jsonl"
set "TICKTMP=%WATCH_OUT%\tick.tmp"
set /a TICKN=0
set "MARCO_SHADOW_VISION=screenparser"
if "%MARCO_SHADOW_CADENCE%"=="" set "MARCO_SHADOW_CADENCE=1s"
set "MARCO_SHADOW_UNGATED=1"
set "DIRECTOR_VISION=%CD%\plugins\vision\vision.exe"
if not exist "%MARCO_HOME%" mkdir "%MARCO_HOME%"

rem THE OLD ONE GOES FIRST. A Director already serving this home was started with whatever
rem configuration it was started with, and it would answer every question below.
director.exe shutdown >nul 2>&1

echo Starting a Director on %MARCO_HOME% ...
start "marco-watch-interface" /min director.exe serve
rem The endpoint file appears when it is listening. Polling for it beats sleeping for a
rem guess, and a guess that was too short reads as "the Director is not running".
set /a TRIES=0
:wait
set /a TRIES+=1
director.exe status >nul 2>&1
if not errorlevel 1 goto ready
if %TRIES% GEQ 30 (
  echo watch-interface: the Director did not come up.
  exit /b 1
)
rem `timeout` refuses when stdin is redirected, which is how a harness runs this.
ping -n 2 127.0.0.1 >nul
goto wait

:ready
rem THE TRACE'S CLOCK STARTS WITH THIS PROCESS, NOT WITH THE SESSION. slots.jsonl stamps
rem at_ms from the Director's own start, and the phases below are wall clock, so one of the
rem two needs an anchor or they cannot be laid over each other. This is the anchor: the
rem readiness poll just succeeded, so the serve process is at most one poll old.
>>"%PHASELOG%" echo serve-ready -- %DATE% %TIME%
echo.
director.exe observe-game --application "%APP%" --duration %SECS%s --interval 500ms
if errorlevel 1 (
  echo watch-interface: could not start a session on "%APP%".
  echo                  `director.exe windows` lists what it can see.
  director.exe shutdown >nul 2>&1
  exit /b 1
)
>>"%PHASELOG%" echo session -- %DATE% %TIME%
echo.
echo Nothing is injected. Keyboard and CONTROLLER are both observed now, and every
echo observation says which device it came from. If a device was not watched the report
echo says so BY NAME - it will never report quiet as yours.
echo.
rem THE ONE THING THE OPERATOR CAN GET WRONG THAT LOOKS LIKE NOTHING.
rem
rem Capturing by window does not steal focus, and it is not occlusion-safe: Windows can hand
rem back the pixels of whatever is covering the target, UNDER THE TARGET'S OWN NAME. No
rem identity guard sees it. Experiment-025 captured the Windows taskbar on one look of 107 with
rem provenance clean, the right window, and target_losses 0.
rem
rem So the Director now asks which window is in front and refuses a look taken behind another
rem one. That is the correct behaviour and it has a cost the operator has to know about: if you
rem click this window to read the banner, those looks are REFUSED, and a run spent reading the
rem banner collects nothing. The report says how many, by name, so it can never be mistaken for
rem a quiet screen - but the looks are still gone.
rem
rem Visible is fine. In front is not. Second monitor, or beside a windowed game.
echo KEEP THE GAME IN FRONT. This window may be visible, but a look taken while
echo something else is in front is refused: a window capture can return the covering
echo window's pixels under the target's name. Click the game once, then read this
echo window without clicking it. The report counts refused looks BY NAME.
echo.
echo This run is a protocol, not a browse. The script tells you what to do and for how
echo long, and writes down when each phase began, because the confirmation gate has to
echo be judged separately on slow navigation and on fast navigation - a run that mixed
echo them answers neither question.
echo.
echo   Follow the banner, and nothing else. What the session believes is written to a
echo   file rather than to this window - a forty-line report that changes every two
echo   seconds is unreadable by somebody who is also playing a game.
echo.
ping -n 6 127.0.0.1 >nul

rem Each loop is one ~2s tick. The shares are of the whole run, so any --duration keeps
rem the same shape; only the last phase has a floor to defend (see the note above).
rem NAVIGATION STARTS IMMEDIATELY, AND HOLDING STILL MOVED TO THE END.
rem
rem The first version of this protocol opened with 48 seconds of HOLD STILL. Experiment-024
rem ran it and found two things that together make that ordering unable to succeed:
rem
rem   1. The menu was legible for slots 1-10 only, 0-43s. Over the next 150s the detector
rem      admitted nothing on 56 looks of 72. So the legible window and the hold-still phase
rem      were the same stretch of the run.
rem
rem   2. A list is identified by being seen HANDING A MARK BETWEEN ITS MEMBERS. The rule is
rem      in selection/active.go and wants, of one candidate: 10 judged frames, 8 of them
rem      with exactly one member marked, at least 2 transfers, across at least 2 distinct
rem      holders. A screen nobody touches produces zero transfers.
rem
rem Holding still cannot identify an interface. It is not a slow start, it is a phase that is
rem structurally incapable of the thing the rest of the run depends on, and it was spent on
rem the only looks that could have afforded it. `group_1` came out of that run with 24
rem members, 16 buttons and episodes: 0 — a list, watched, never used.
rem
rem So MOVE SLOWLY is first and gets the largest share. At Experiment-024's measured 2.6s
rem per inference, 25% of 240s is ~23 looks, against the 10 the rule needs; one press every
rem five seconds puts a transfer in roughly every second look, against the 2 it needs. Both
rem margins are deliberate, because the cadence is the thing least under this script's
rem control and ADR-152's tracking path may have changed it in either direction.
rem
rem HOLD STILL survives as phase 5, where it asks a question it could not ask as phase 1: an
rem interface that HAS been identified, and a mark nobody is moving — does the belief hold,
rem and does the selection stay where it was put? As the opening phase it could only ask
rem about candidate flicker, and Experiment-024 already answered that (the gate refused five
rem of six wobbles on an unchanging screen).
if /i "%PROTOCOL%"=="maps" goto protomaps
set /a T1=%SECS%*25/100/2
set /a T2=%SECS%*12/100/2
set /a T3=%SECS%*13/100/2
set /a T4=%SECS%*10/100/2
set /a T5=%SECS%*10/100/2
set /a T6=%SECS%*30/100/2

rem ASKS: is a mark handed between members, and is each move confirmed? THE phase the run
rem      depends on: nothing after it can be read if no list was identified here.
call :phase %T1% "1 of 6   MOVE SLOWLY" "Open the pause menu, then one press about every 5 seconds."
rem ASKS: how many real changes does the gate MISS at speed?
call :phase %T2% "2 of 6   MOVE FAST" "Several presses as fast as you like, then stop."
rem ASKS: is direction observed, or only that something moved?
call :phase %T3% "3 of 6   REVERSE" "Back up slowly, the way you came."
rem ASKS: does a wrap read as one move, or as a jump to a different interface?
call :phase %T4% "4 of 6   WRAP" "Go off the end of the list so it wraps around."
rem ASKS: does an IDENTIFIED belief hold, and the selection stay put, with nobody moving?
call :phase %T5% "5 of 6   HOLD STILL" "Leave the selection alone. Touch nothing."
rem ASKS: is the belief LOST, for the right reason, and when?
call :phase %T6% "6 of 6   CLOSE THE MENU" "Close it and leave it closed. Do not navigate."
goto done

:protomaps
rem ---------------------------------------------------------------------------
rem THE MAP-SELECTION PROTOCOL. Four phases, and it STARTS WITH THE LIST ALREADY VISIBLE.
rem
rem Experiment-025's protocol opened by asking the operator to open the pause menu, which spent
rem its first looks on a screen transition. This one does not: the list is up before the
rem Director is, so phase 1's looks are all looks at the thing being tested.
rem
rem THE ORDER IS NOT ARBITRARY, and HOLD cannot come first. A member's baseline is built only
rem from frames in which that member was NOT marked, so a row that is lit from the very first
rem look never establishes one and is skipped from the judgement entirely. A mark that has
rem never moved is invisible to this tracker by construction - which is the second, independent
rem reason Experiment-024's opening HOLD STILL phase could not have worked, and the reason HOLD
rem sits after SLOW here: by then the baselines exist.
rem
rem The arithmetic, at Experiment-025's measured 2.26s per inference. The rule wants, of one
rem candidate: 10 judged frames, 8 with exactly one member marked, 2 transfers, 2 holders.
rem
rem   SLOW     40% of 150s = 60s = ~26 looks, ~12 presses   against 10 judged and 2 transfers
rem   HOLD     17%         = 25s = ~11 looks
rem   REVERSE  23%         = 34s = ~15 looks
rem   FAST     20%         = 30s = ~13 looks
rem
rem Every margin is deliberate. The cadence is the thing least under this script's control.
set /a M1=%SECS%*40/100/2
set /a M2=%SECS%*17/100/2
set /a M3=%SECS%*23/100/2
set /a M4=%SECS%*20/100/2

rem ASKS: is a mark handed between members, is this list IDENTIFIED, and after how many looks?
rem      THE phase the run depends on - nothing after it can be read if no list is named here.
call :phase %M1% "1 of 4   MOVE SLOWLY" "One press about every 5 seconds. Stay in this list."
rem ASKS: does an IDENTIFIED belief hold, and the selection stay put, with nobody moving?
rem      Distinguishes a belief that survives quiet from one that only existed while moving.
call :phase %M2% "2 of 4   HOLD A SELECTION" "Leave the selection where it is. Touch nothing."
rem ASKS: is DIRECTION observed, or only that something moved?
call :phase %M3% "3 of 4   REVERSE" "Go back up the list, slowly, the way you came."
rem ASKS: how many real changes does the confirmation gate MISS at speed?
call :phase %M4% "4 of 4   MOVE FAST" "Several presses as fast as you like, then stop."
goto done

:phase
rem %1 ticks  %2 banner  %3 instruction
rem TICKN is the tick count so far, so the phase is pinned to a line number in ticks.jsonl
rem rather than to a wall clock somebody has to trust.
>>"%PHASELOG%" echo %~2 -- %DATE% %TIME% -- begins after tick %TICKN%
rem ONE TICK PER CALL, because the countdown has to be read back inside the loop and a
rem for-block cannot do that without delayed expansion — which would eat an exclamation mark
rem in any instruction somebody writes here later.
for /l %%i in (1,1,%~1) do call :tick %%i %~1 "%~2" "%~3"
exit /b 0

:tick
rem %1 this tick  %2 ticks in the phase  %3 banner  %4 instruction
rem
rem THE SESSION REPORT IS NOT PRINTED HERE, DELIBERATELY. It is forty lines, it changes every
rem two seconds, and in the first run of this protocol it pushed the only thing the operator
rem needed — what to do next — off the top of the screen. The operator's job and the
rem diagnostic's output are two audiences; the diagnostic goes to a file, where it can be read
rem by someone who is not simultaneously playing a game.
set /a LEFT=(%~2-%~1+1)*2
cls
echo.
echo.
echo        ##################################################################
echo        ##                                                              ##
echo        ##   %~3
echo        ##                                                              ##
echo        ##   %~4
echo        ##                                                              ##
echo        ##   about %LEFT%s left in this phase
echo        ##                                                              ##
echo        ##################################################################
echo.
echo.
echo          use the CONTROLLER - nothing is injected, Marco only watches
echo          tick %TICKN% recorded
director.exe observation-session observe_1 --json > "%TICKTMP%" 2>nul
type "%TICKTMP%" >> "%TICKLOG%"
set /a TICKN+=1
ping -n 3 127.0.0.1 >nul
exit /b 0

:done
>>"%PHASELOG%" echo end -- %DATE% %TIME%

echo.
director.exe observation-session observe_1 --json > "%WATCH_OUT%\final.json" 2>&1
rem AND THE ONE NUMBER ADR-152 IS MISSING: what a window capture costs, apart from the
rem inference. `director vision` prints capture and detect separately, and this is the only
rem moment a real menu is in front of the machine. One look, read-only.
director.exe vision --application "%APP%" > "%WATCH_OUT%\capture-cost.txt" 2>&1
if exist "%TICKTMP%" del "%TICKTMP%"
echo.
echo The run recorded itself:
echo     %MARCO_SHADOW_TRACE%
echo         every slot offered and what became of it
echo     %WATCH_OUT%\final.json
echo         the session's own final answer
echo     %WATCH_OUT%\capture-cost.txt
echo         what a capture costs apart from the inference - see ADR-152
echo     %TICKLOG%
echo         what the session believed at every tick, one JSON object per line
echo     %PHASELOG%
echo         when each phase began, so slow and fast navigation can be read apart
echo.
rem AND HOW TO READ THE FIVE OF THEM TOGETHER. The seven questions this run is asked -
rem the selected-entry sequence, its timestamps, missed and false transitions, the
rem cadence, what Director believed throughout, and whether a selection was never seen
rem or seen and declined - each need two or three of these files laid over each other.
rem Done by eye that is an afternoon and a guess; the reader does it and shows its
rem arithmetic. It also knows the one join that looks right and is wrong: a press
rem carries an INFERENCE count and a belief moment carries a SAMPLE number, and fast
rem tracking decouples those two on purpose.
echo To read it:
echo     py -I tools\run-report\read_run.py "%WATCH_OUT%"
echo.
echo The session is over. The Director is still running on this script's isolated home,
echo so run this script again to watch another one, or stop it with:
echo     set MARCO_HOME=%MARCO_HOME%  ^&^&  director.exe shutdown
endlocal
