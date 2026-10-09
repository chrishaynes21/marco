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
if "%SECS%"=="" set "SECS=120"

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
echo.
director.exe observe-game --application "%APP%" --duration %SECS%s --interval 500ms
if errorlevel 1 (
  echo watch-interface: could not start a session on "%APP%".
  echo                  `director.exe windows` lists what it can see.
  director.exe shutdown >nul 2>&1
  exit /b 1
)
echo.
echo Navigate the menu. Ctrl-C stops watching; the session stops on its own.
echo.
echo   An inference costs about two seconds, so hold each selection for ~5s:
echo     1. sit still on a menu for ~25s        does one candidate survive many looks
echo     2. move the selection 4-5 times slowly is a mark handed between members
echo     3. open a submenu, pause ~20s          does the belief follow a real change
echo     4. leave menus for the last ~60s       is the belief LOST for the right reason
echo.
echo   Nothing is injected. Keyboard and controller are both fine - but note which you
echo   used: the navigation producer hooks keyboard and mouse only, so a gamepad is
echo   invisible to it by construction and reads as a player who pressed nothing.
echo.

set /a LOOPS=%SECS%/2
for /l %%i in (1,1,%LOOPS%) do (
  cls
  director.exe observation-session observe_1
  ping -n 3 127.0.0.1 >nul
)

echo.
director.exe observation-session observe_1 --json > "%WATCH_OUT%\final.json" 2>&1
echo.
echo The run recorded itself:
echo     %MARCO_SHADOW_TRACE%
echo         every slot offered and what became of it
echo     %WATCH_OUT%\final.json
echo         the session's own final answer
echo.
echo The session is over. The Director is still running on this script's isolated home,
echo so run this script again to watch another one, or stop it with:
echo     set MARCO_HOME=%MARCO_HOME%  ^&^&  director.exe shutdown
endlocal
