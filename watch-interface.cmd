@echo off
rem Watch what the Director believes it is looking at, live, while you navigate.
rem
rem   watch-interface.cmd                     trackmania, 2 minutes
rem   watch-interface.cmd rocketleague 180    another application, another length
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
rem THE PROTOCOL BELOW HAS SIX PHASES AND THE LAST ONE HAS TO OUTLIVE A CONSTANT.
rem An interface belief survives interfaceForgetLooks = 10 looks that bring nothing
rem before it is dropped, and a look costs about two seconds, so "the menu is closed"
rem needs 30s+ of looking at nothing to show the belief being LOST rather than merely
rem going stale. 240s divides into the six phases with that one long enough.
if "%SECS%"=="" set "SECS=240"

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
rem thing being tested here is the half a person has to do by hand. These two files are the
rem whole record: every slot and what became of it, and the session's final answer.
set "WATCH_OUT=%LOCALAPPDATA%\marco-liveiface\runs"
if not exist "%WATCH_OUT%" mkdir "%WATCH_OUT%"
set "MARCO_SHADOW_TRACE=%WATCH_OUT%\slots.jsonl"
if exist "%MARCO_SHADOW_TRACE%" del "%MARCO_SHADOW_TRACE%"
rem AND WHEN EACH PHASE BEGAN, which is what makes the two regimes separable.
rem The gate confirms a change on a second agreeing look, so slow navigation and fast
rem navigation are the two cases it must be judged on separately — and a trace of a run
rem that mixed them answers neither. Wall clock here maps onto the trace's at_ms,
rem which is measured from this Director's start: the session line below is the anchor.
set "PHASELOG=%WATCH_OUT%\phases.txt"
if exist "%PHASELOG%" del "%PHASELOG%"
rem AND EVERY TICK'S ANSWER, not just the last one. The loop below already asks the
rem session what it believes every two seconds and prints it to a screen that is then
rem cleared. The final aggregate cannot answer "how many changes were confirmed WHILE
rem the player was moving fast", because it is one number for the whole run; the
rem per-tick answers can. One JSON object per line, and phases.txt says which tick
rem each phase started after, so the two files index each other.
set "TICKLOG=%WATCH_OUT%\ticks.jsonl"
if exist "%TICKLOG%" del "%TICKLOG%"
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
set /a T1=%SECS%*20/100/2
set /a T2=%SECS%*20/100/2
set /a T3=%SECS%*12/100/2
set /a T4=%SECS%*13/100/2
set /a T5=%SECS%*10/100/2
set /a T6=%SECS%*25/100/2

rem ASKS: does one candidate survive many looks as ONE thing, or does it flicker?
call :phase %T1% "1 of 6   HOLD STILL" "Open a menu. Leave the selection alone. Touch nothing."
rem ASKS: is a mark handed between members, and is each move confirmed?
call :phase %T2% "2 of 6   MOVE SLOWLY" "One press about every 5 seconds."
rem ASKS: how many real changes does a two-look gate MISS at speed? The point of the run.
call :phase %T3% "3 of 6   MOVE FAST" "Several presses as fast as you like, then stop."
rem ASKS: is direction observed, or only that something moved?
call :phase %T4% "4 of 6   REVERSE" "Back up slowly, the way you came."
rem ASKS: does a wrap read as one move, or as a jump to a different interface?
call :phase %T5% "5 of 6   WRAP" "Go off the end of the list so it wraps around."
rem ASKS: is the belief LOST, for the right reason, and when?
call :phase %T6% "6 of 6   CLOSE THE MENU" "Close it and leave it closed. Do not navigate."
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
echo The session is over. The Director is still running on this script's isolated home,
echo so run this script again to watch another one, or stop it with:
echo     set MARCO_HOME=%MARCO_HOME%  ^&^&  director.exe shutdown
endlocal
